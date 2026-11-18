# Reliability, idempotency and recovery

Status: **Phase 0 proposal — awaiting architecture approval.** No fault tolerance, availability or recovery number in this document has yet been demonstrated. The initial deployment has a single control-plane process and PostgreSQL durability; it does not provide control-plane high availability.

## Guarantees to build and limits to expose

PostgreSQL owns triggers, run/job/Attempt state, leases, retry timing, authorization references, audit and outbox metadata. The blob store owns immutable source, log chunks and artifacts whose committed manifests are tracked in PostgreSQL. In-memory notifications, open long polls, browser SSE connections and process logs are disposable. A database plus blob backup is required; neither alone reconstructs all run evidence.

The target guarantees are durable accepted work, idempotent state mutation, at most one currently authorized Attempt per JobRun, monotonic terminal outcomes and explicit ambiguous execution. Delivery of claims/events/outbox messages can repeat. A command can perform external effects and then crash before acknowledgment, so **exactly-once execution of arbitrary code is not promised**. Fencing prevents stale writes to Shipra; it does not fence an external database or cloud API unless that target participates in its own idempotency protocol.

Automatic job retries default to zero and only opt into observed nonzero command failures. Ambiguous `lost` Attempts never auto-retry in v0.1. A manual rerun is a new run, records lineage and warns that external effects may already have occurred. Idempotent pipeline authoring remains useful even in trusted repositories.

## Failure behavior matrix

| Failure | Durable interpretation | Recovery and user-visible result |
|---|---|---|
| Runner dies before/after claim response | Committed Attempt may exist even if receipt is unknown | Lease expires; Attempt/JobRun lost; original slot quarantined pending cleanup; no blind requeue |
| Runner crashes during command | Command effects unknown | Agent restart stops owned containers; report abandonment; server closes active Attempt lost; manual rerun permitted as a separate run |
| Control plane restarts | Accepted database state survives; open HTTP connections end | Agents reconnect within lease bounds; controller scans durable state; no resetting active jobs to queued |
| Control plane unavailable longer than lease | Agents cannot renew authority | Agents stop by local bound; server later records lost unless it already has accepted terminal evidence |
| PostgreSQL unavailable | Cannot safely admit, claim, renew or finish work | Fail closed with retryable API errors; do not acknowledge uncommitted mutation; agents stop when existing authority runs out |
| Queue worker crashes | Queue state is PostgreSQL state | Work lease/reaper and durable scans resume; there is no independent broker whose memory could lose assignments |
| Database restored from backup | State may predate previously issued leases/tokens | Isolate controller, revoke/rotate runner authority, terminate/reconcile old agents, then resume; never allow old workers to reconnect into restored epochs blindly |
| Agent/control-plane network partition | Neither side can assume the other's physical state | Agent stops before local lease bound; server marks expired Attempt lost; stale results rejected; no automatic retry |
| Duplicate webhook | Repeated delivery identity | Unique inbox identity returns original receipt; only one admission for that delivery; a deliberate new delivery is separately tracked |
| Duplicate claim/result/log/finalize | Request may already have committed | Return original/equivalent committed result for identical key/body; reject conflicts; do not repeat execution or publication |
| Job exceeds command/attempt deadline | Deadline intent and process status | Stop/escalate; confirmed termination timed_out, unconfirmed lost; pipeline deadline separately bounds whole run |
| Queue wait expires | No Attempt was created | JobRun timed_out with queue_timeout reason; dependents evaluate final outcome; capacity diagnostic remains visible |
| User cancellation | Durable run stop intent | Suppress new work immediately; stop active attempts; canceled run may include termination_unconfirmed if a runner is lost |
| Required artifact upload fails | Commands may have succeeded; job contract incomplete | Retry transfer within same deadline; final failure is artifact_upload, no rerun of commands |
| Optional artifact upload fails | Command outcome remains known | Warning and unavailable output; run may still succeed; dependent download of that output fails explicitly |
| Log upload fails or quota fills | Logs incomplete, command result independent | Bounded spool, truncation marker and byte counters; continue draining; execution only while lease valid |
| Storage full/unavailable | Cannot commit new blobs | Stop accepting new source snapshots; required outputs fail; logs degrade visibly; never publish a manifest for unverified content |
| Notification/provider status outage | Terminal run result already committed | Durable outbox retry, visible delivery failure/dead letter after bounded policy; run is not changed back to running |
| Cleanup fails | Physical slot not demonstrably reusable | Quarantine slot, retain evidence and bounded retry; operator action if cleanup remains stuck |
| Runner disconnects during a deployment-like shell command | External side effect may have occurred | v0.1 records lost; it cannot infer target state or automatically roll back; future deployment engine requires uncertain/reconciliation states |

## Idempotency namespaces and ordering

| Operation | Identity | Conflict/expiry rule |
|---|---|---|
| GitHub ingress | Provider, configured integration/repository and delivery ID | Same identity and payload digest deduplicates; mismatched digest is an incident/validation failure |
| Manual trigger | Project, caller and supplied idempotency key | Retain for at least 24 hours; identical body returns original Run/trigger; different body conflicts |
| Claim | Runner ID, process session ID and claim key | One committed assignment identity; expired/closed assignment never silently yields different work for that key |
| Attempt observations/result | Attempt ID, epoch and idempotency/event key | Identity/body digest recorded with state transition; stale unseen mutations rejected |
| Logs | Attempt ID and contiguous batch sequence | Duplicate digest accepted; mismatch rejected; gaps request retransmission from expected sequence |
| Artifact content/finalize | Attempt ID, output name and upload ID/content digest | Content immutable after commit; incomplete staging cannot be a dependency input |
| Outbox | Event ID and destination | Repeated delivery may occur; use receiver idempotency when offered; delivery history is separate from run result |
| Cleanup | Owned resource ID and generation | Repeated deletion safe; absent resource succeeds; ownership mismatch never deleted |

Deduplication retention is a product contract, not an unbounded table. Proposed ingress identity retention is 30 days; manual keys at least 24 hours; Attempt/claim/result identities are retained with run metadata for 90 days by default. A key outside its supported retention window cannot promise deduplication. Protected audit/retention requirements may extend these values. Pipeline reruns always use new identities instead of reusing keys deliberately.

Mutating APIs acknowledge only after their authoritative transaction or verified blob-plus-manifest commit. If the reply is lost, clients retransmit the same key. Same-key result replay can return a prior success after a lease expires because it creates no new state; a different/previously unseen result after expiry cannot. Logs are observations, never proof that a command or cleanup succeeded.

## Blob commit and cleanup ordering

PostgreSQL and filesystem/S3 storage do not form one atomic transaction. The proposed protocol makes this visible:

1. Allocate an upload record with server-chosen staging key and bounded grant.
2. Write content to staging, verify declared size/digest and manifest, and durably commit immutable content in the blob store.
3. In a short database transaction, publish the manifest/reference and its audit/outbox event.
4. Return the committed manifest to the agent; only then may it reference the output in a final successful result.

A crash before step 3 leaves an unreferenced blob, which a grace-period janitor can delete. A crash after step 3 but before the response leaves a committed artifact that an idempotent finalize retrieves. A missing/corrupt blob behind a committed manifest is a storage-integrity incident: hide that artifact from consumption, record the defect, and fail any new dependent retrieval. Do not rewrite a historical terminal run to make this inconsistency disappear.

Filesystem storage requires durable file write and atomic publication within the supported filesystem, with errors propagated. The selected implementation must test crash/fsync behavior on the qualified Linux storage profile. An eventual S3 adapter must satisfy the same verified-content and immutable-publication interface, with explicit multipart abort/orphan cleanup; portability is an interface objective, not a claim every S3-compatible product has identical semantics.

Retention first tombstones database-visible availability, prevents new reads, waits for active readers/pins to expire, then deletes content, then records cleanup completion. Runs in progress, their snapshots, required successful ancestor artifacts and active investigations are pinned. Objects may be deleted only after no live reference needs them; temporary objects also have a safe grace interval longer than any upload grant. Proposed staging/orphan grace is 24 hours. Backup retention and lifecycle policies must not race these rules.

## Recovery procedures to design and test

### Normal control-plane restart

Start with one controller owner. Check schema compatibility and storage access. Reconcile expired leases, stop intents and incomplete durable work in bounded transactions; retain still-valid assignments. Resume outbox and inbox consumers, then accept new claims. Agents resend their latest known sequences. The existing database is authoritative; an empty memory cache is normal, not a reason to rebuild state from log files.

### Worker host failure or compromise

Revoke its runner identity, fence/expire its active Attempts and quarantine all affected slots. Credentials already handed to repository code may require external rotation. Bring up a clean worker through new enrollment and validated policy. Do not assume deleting the server's token stops a compromised process or retracts a secret. Preserve relevant audit evidence and mark old outcomes lost where appropriate.

### Restore from disaster backup

Freeze ingress and worker connectivity; restore PostgreSQL and blob snapshots to a compatible documented recovery point. Verify manifest/blob integrity and configuration/secret-key references. Invalidate pre-restore runner grants and sessions, reconcile/terminate every old physical worker, and record lost/unknown results where post-backup execution cannot be proven. Re-enroll/re-authorize clean agents before claims resume. Database restore alone must not permit reused fencing epochs to authorize an old worker. A later deployment must provide a restore-generation mechanism and a rehearsed runbook before claiming disaster recovery readiness.

Proposed operational objectives for later qualification: accepted state survives a controller-process crash when PostgreSQL and its disk remain healthy; database/blob backup recovery-point objective of 24 hours for a personal deployment; recovery-time objective of four hours after backups and credentials are available. These are goals to test with a restore drill, not uptime promises or acceptable data loss for all organizations. Production deployments will need a separate durability/backup tier and review.

## Time and duplicate-side-effect reasoning

Database timestamps order server lease expiry and durable deadlines. Agents use conservative monotonic budgets anchored before renewal requests; request retries retain the original time anchor. Clock discontinuities, suspend/resume and unexpectedly delayed responses force safe termination rather than assuming fresh permission. Read [runner deadline policy](runners.md#heartbeats-leases-and-deadlines) for concrete values.

Fencing stops a partitioned runner from committing a stale Shipra result or output. It cannot remove an HTTP request already sent to an external target. Even a confirmed command failure may have created external effects. Therefore explicit retries repeat a whole job under author responsibility, lost attempts require a separate decision, and managed deployment will require target-side operation identities, environment locks and reconciliation. A production deployment feature cannot be implemented as `run: deploy` plus an unconditional retry loop.

## Failure-injection acceptance matrix

Before the execution phases pass, deterministic unit/model tests and real PostgreSQL/Docker integration tests must inject failures at each boundary:

| Experiment | Required evidence |
|---|---|
| Kill controller immediately before/after claim commit | No lost queued job; repeated claim resolves exactly one assignment identity |
| Kill controller immediately before/after result commit | Same terminal result on replay; no duplicate retry or downstream JobRun |
| Lose claim reply, delay acknowledgment and reconnect | No second local container; late revoked assignment cannot start |
| Partition agent longer than lease | Agent stops inside bound on supported host; server records lost; stale result denied |
| Reorder/duplicate heartbeat responses | Authority does not move backward or extend from a stale response |
| Race cancel, run timeout and success | One persisted winner/stop intent; no terminal state reversal |
| OOM/kill Docker daemon and fail cleanup | Known OOM is failed when observed; ambiguous loss is lost; dirty slot quarantined |
| Fill log spool and deny log writes | Command output is drained; logs_incomplete visible; no deadlock or false success inference |
| Crash during artifact staging/publish/finalize | No incomplete artifact consumed; retry finalization safe; orphans collectible |
| Exhaust storage, queue and database connections | Bounded retries and clear overload; no successful acknowledgment of discarded work |
| Flood one project while another remains eligible | Durable fairness progresses across restarts within available capacity |
| Restore old database while old worker exists | Old grants/epochs cannot mutate restored state; incident recorded and worker reconciled |

Observe structured state-transition events, stale-result rejections, lost Attempts, queue age, lease-renewal failures, cancellation latency, cleanup backlog, blob integrity failures and outbox lag. Alert thresholds follow measured baseline behavior; the architecture's provisional timeouts are not proof of actual recovery latency under load.
