# Scheduler, durable queue and fairness

Status: **Phase 0 proposal — awaiting architecture approval.** PostgreSQL is the sole durable scheduling store. v0.1 runs one control-plane instance and supports multiple outbound agents. The design includes fencing and transactions to preserve correctness, but control-plane horizontal scaling is not an advertised v0.1 feature.

## Why a database-backed queue

The first system needs durable job eligibility, cancellation, retry timing, capacity reservations and state transitions more than it needs a separate broker. Keeping them in PostgreSQL allows one transaction to connect the queue entry, Attempt, lease, audit record and outbox. In-memory channels or timers can accelerate wakeups but cannot be the only record of work. Redis, Kafka, RabbitMQ and NATS are not required infrastructure in v0.1.

PostgreSQL explicitly describes `SKIP LOCKED` as useful for avoiding contention in queue-like tables, while warning that skipped rows give an inconsistent view. Shipra uses row locking for claims, and separately defines fairness/order; `SKIP LOCKED` by itself is neither a scheduler nor a fairness guarantee. [PostgreSQL SELECT locking documentation](https://www.postgresql.org/docs/current/sql-select.html#SQL-FOR-UPDATE-SHARE)

## Work classes

| Durable work | Producer | Consumer and acknowledgment |
|---|---|---|
| Trigger inbox | Verified webhook/manual API | Admission worker; claim lease and idempotent admission result |
| DAG evaluation work | Run creation or terminal JobRun | Pipeline engine; transaction evaluates terminal dependencies and updates eligibility |
| Ready JobRun | Pipeline engine/retry timer | Runner claim handler; atomic Attempt creation and capacity reservation |
| Scheduled retry | Accepted eligible failed Attempt | Scheduler; durable `not_before`, no in-memory-only timer |
| Expiry/reconciliation work | Time and lease state | Reaper; compare expected version/epoch and expire only current active work |
| Outbox delivery | State-changing transaction | Provider/UI notification workers; idempotent or duplicate-tolerant receiver behavior |
| Blob/retention cleanup | Tombstones and retention policy | Bounded janitor; repeated deletion is safe and progress is durable |

Separate work classes prevent a provider outage or slow artifact deletion from consuming all claim-processing capacity. No transaction remains open while waiting on an agent, GitHub, the blob store or a long poll. A long poll waits outside a transaction and rechecks eligibility when woken or periodically. Wakeup loss delays work at most until the next database scan while the server is healthy; it cannot erase work.

## Eligibility and placement

The runner requests one assignment per free slot. The server verifies current runner identity/session, protocol compatibility, approved labels, executor/architecture support, healthy non-quarantined capacity, and project/pool authorization. Self-reported free resources may lower availability but cannot increase the configured capacity ceiling.

An eligible JobRun must be queued, past its durable `not_before`, before both queue and run deadline, without a run stop intent, with all frozen constraints satisfied. A label matches only exact strings; v0.1 provides conjunction of labels, not a custom expression language. CPU and memory reservations are checked against configured runner budgets and enforced again by the agent runtime. Jobs do not share workspace, credentials or implicit local affinity.

Per-run, project and global concurrency limits are checked in the claim transaction. Reservations include preparation, execution, uploading and canceling, because each occupies real worker capacity. Retry waits and dependency waits occupy no runner slot. A terminal result or lease expiry releases the database reservation exactly once; an unconfirmed physical cleanup additionally quarantines the original runner slot. A different slot may accept other work, but Shipra never treats the suspect slot as clean based only on a timeout.

## Proposed fairness rule

Use equal-weight durable round robin across projects, then active runs within a project, then FIFO ready jobs within a run. Project and run records store the global dispatch sequence of their last successful assignment. On each claim:

1. Identify projects having at least one job eligible for this runner and spare project/global capacity.
2. Choose the least recently served eligible project; tie-break by project ID.
3. Within it, choose the least recently served eligible run with spare run capacity; tie-break by run admission order and Run ID.
4. Within that run, choose the earliest ready sequence, then JobRun ID.
5. Atomically increment the durable dispatch sequence, record the chosen project/run service position, reserve capacity, increment the job epoch and create its Attempt.

Only a successful assignment advances service position. A retry re-enters the ready queue with a new sequence after its backoff, so it cannot repeatedly jump ahead. There are no user-configurable priority levels or preemption in v0.1. The smallest implementation can serialize this short decision with a durable scheduler-coordinate row; the performance cost is acceptable only if confirmed by the target workload benchmark. A later distributed scheduler must preserve the semantics or publish an explicit policy change.

Fairness is conditional on eligibility, available slots and jobs finishing within their bounds. A job requiring an offline specialized runner cannot receive capacity from an incompatible host. Heterogeneous runner pools can produce different service rates; equal dispatches do not mean equal CPU time. A long job is not preempted. Surface each wait reason: dependencies, retry delay, no matching runner, unhealthy runner, project/run cap, insufficient resources, queue limit or stopped run.

The queue-wait deadline prevents indefinite queued jobs; it does not prove starvation freedom. Fairness tests must show that continually arriving eligible jobs from one project/run cannot permanently exclude another continuously eligible project/run under available capacity. This design targets intelligibility and bounded starvation rather than maximum utilization or billing fairness.

## Atomic claim and result boundaries

The claim transaction checks stop/deadline state again under lock, locks the JobRun and capacity records in a defined order, allocates an Attempt ID and increasing job epoch, commits the lease/reservation, and persists the claim key. The reply is sent only after commit. A crash before commit leaves no assignment; a crash after commit leaves an assignment that the same claim key can retrieve. A different claim cannot consume that JobRun while its Attempt is authorized.

The result transaction verifies the authenticated runner, current Attempt/epoch, lease and allowed transition. It atomically records the result, releases capacity, chooses retry/terminal JobRun state, creates DAG evaluation work and emits audit/outbox events. The artifact service must already have committed required output manifests. A stale runner's result never wins because its HTTP request arrived first; only the database-authorized epoch can mutate state.

Within the single control-plane instance, concurrent HTTP handlers and background workers still race. Transaction constraints and compare-and-set versions are required even before multiple controllers exist. A process mutex alone does not protect against restart or lost responses. Per-JobRun epoch values are monotonic and never reused across attempts; manual reruns use new JobRun IDs.

## Bounded operating envelope

| Bound | Proposed v0.1 value | Action at limit |
|---|---:|---|
| Enrolled agents | 10 initial qualification target | Larger installations are unqualified until measured |
| Global concurrent Attempts | 20 qualification target; development default 2 | Preserve queued work and expose capacity wait |
| Per-project concurrency | Default 4, administrator-controlled within global cap | Round robin chooses another eligible project |
| Per-run concurrency | Default 4, administrator-controlled within project cap | Preserve ready nodes without claiming |
| Agent slots | Default 1; proposed maximum 8 with measured host budget | Agent and scheduler both enforce reservation |
| Nonterminal PipelineRuns | 100 globally | Reject new manual admission with retryable capacity error; retain bounded trigger inbox processing status |
| Outstanding JobRuns | 10,000 globally | Admission fails atomically before creating partial DAGs |
| Pending trigger inbox | 10,000 entries and body-size quota | Reject before durable acceptance with retryable overload; never acknowledge and silently discard |
| Queue wait | 30 minutes per ready interval | JobRun timed_out with `queue_timeout`; run normally fails |
| Reconciliation batch | 100 rows per short transaction | Continue next batch without monopolizing claims |

The qualification baseline proposed for load experiments is a 4-vCPU/8-GiB control-plane host and a 2-vCPU/4-GiB PostgreSQL host, with worker hardware sized separately. This is a test profile, not a published minimum or a throughput promise. Blob storage and source/artifact/log sizes materially affect the result. The total job limit is checked against the entire compiled DAG at admission, preventing partially admitted pipelines.

Inbox overload must consider provider retry behavior explicitly. An authenticated duplicate already present can return its original success even when the new-inbox limit is full. A new request that cannot be persisted receives an error and an operator-visible overload signal; operators can inspect/redeliver missing GitHub events. Automatic recovery of events the provider never delivered is not assumed.

## Recovery and scaling triggers

On server startup: establish exclusive v0.1 controller ownership; reconcile expired claims/leases, stop intents and retry timers in bounded batches; scan for missing DAG work from terminal transitions; then admit new claims. Existing unexpired leases remain valid. The controller never bulk-requeues running work solely because the process restarted. Database time is authoritative for lease comparisons.

A periodic consistency scan checks no active job has multiple authorized Attempts, reservations correspond to active Attempts, terminal nodes have no ready entries, every queued node has discoverable work, and outbox cursors progress. Repair is a version-checked domain transition with audit evidence, not arbitrary row editing. Contradictory terminal outcomes raise an incident rather than being guessed from logs.

Measure queue latency by wait reason, scheduling transaction latency/lock waits, jobs per second, fairness by project/run, expired leases, database growth and outbox lag. Consider a separate queue or scheduler service only after profiling shows these bounds are inadequate and the new failure modes have a clear design. Thousands of simultaneous jobs remain a long-term target that requires a later capacity plan, not an assertion about this first implementation.
