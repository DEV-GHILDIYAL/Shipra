# Testing and acceptance strategy

Status: planned tests. No application or runtime tests have been executed in Phase 0. Documentation checks do not establish executor safety or performance.

## Test layers

| Layer | Required scenarios and oracle | Gate |
|---|---|---|
| Unit | Pure DAG readiness, predicates, deadline/retry arithmetic, RBAC decisions, safe error envelopes | P1-P4; terminal state never reopens |
| Parser/schema | YAML scalar behavior, duplicate keys, forbidden tags/aliases, cycles, missing needs, unknown version/fields, bounded nesting/size; valid examples | P2; normalized plans match fixtures and diagnostics identify location |
| Property/fuzz | Arbitrary config, webhook/log/archive input and event interleavings | P2 onward; no panic/unbounded memory, no invalid transition or path escape |
| Database integration | Real PostgreSQL locks, rollback, unique dedupe, concurrent claims, capacity reservations, outbox atomicity, migration upgrade | P1/P4; no duplicate active assignment or acknowledged missing trigger |
| API contract | Authentication scopes, every role/object combination, pagination, expected versions, idempotency mismatch, SSE cursor reset | P1/P6; no unauthorized cross-object read/write |
| Executor/runner | Fresh workspace, image pinning, one host slot, process-group stop, lease expiry, drain/revoke, reconnect and disk limits | P3/P4; no next job before prior execution cleanup/quarantine |
| Storage | Chunk replay/conflict, hash mismatch, partial upload, symlink/archive traversal, artifact download from direct dependency | P3/P7; never publish a partial or wrong artifact |
| Security/adversarial | Secret policy changes, leak attempts, control endpoint/metadata SSRF, webhook tampering, malicious HTML/ANSI, resource exhaustion, compromised agent scope | P3-P7; deny forbidden capability and document residual risk |
| End-to-end | Push/manual -> validation -> parallel DAG -> logs/artifacts -> final status; invalid config, cancel and rerun | P5-P7; deterministic fixture expectations |
| Load/soak | Defined small-workspace load, slow readers, saturated agents, burst ingestion, full disk | P7; measured latency/resource bounds in requirements |
| Recovery/chaos | Crash injection at each durable handoff, partition, DB outage, server/agent restart, coordinated restore | P4/P7/P10; invariants and documented unknown outcomes |

## Critical failure experiments

1. Deliver one webhook 100 times concurrently, including a response loss after inbox commit. Expect one trigger/run association; repeated invalid signatures never create one.
2. Crash server immediately after assignment commit but before HTTP reply; retry the same claim key. Expect the same attempt identity or explicit closed assignment, never fresh work under that key.
3. Partition runner after command start. Let lease expire, then deliver delayed success/log/artifact writes. State remains lost/fenced, stale writes rejected; no automatic replay of potentially side-effectful lost work.
4. Race cancellation, deadline and successful completion under randomized ordering. Exactly one terminal outcome wins according to the documented transaction rules. Unconfirmed process termination remains visible.
5. Kill control plane between blob upload and metadata finalize. Artifact remains unpublished until checks complete; garbage collection eventually removes unreferenced bytes without deleting live manifests.
6. Revoke policy or a secret version after admission and before dispatch/retry. No grant issued. Ensure plaintext never entered graph/event/audit/database query logs.
7. Stop PostgreSQL while a runner is active. Admission fails durably; leases cannot renew; runner terminates conservatively; recovery does not revive expired ownership.
8. Restore an older database plus matching blobs in an isolated environment. Invalidate old credentials/leases and reconcile active state before admitting work; never execute restored assignments blindly.
9. Send more logs than quotas and attach slow SSE consumers. Memory/disk stay bounded, discarded ranges are explicit, scheduler/control heartbeats remain responsive.

Every test records state assertions, not just HTTP success. Distinguish network-message replay from command re-execution. Model-check or property-test small scheduler state spaces before relying on randomized stress alone.

## Safety of the test system

Run adversarial jobs only inside disposable, dedicated Linux VMs with no real credentials and restricted networks. A Docker container alone is insufficient as the test laboratory boundary. Test fixtures use canary secrets and inert target services; never probe public cloud metadata or real deployment targets. Fault injection may kill only resources labeled for the test. Do not run destructive payloads on this workstation during Phase 0.

Meaningful release evidence includes supported OS/runtime/database versions, CPU/RAM/storage, git revision, workload, duration, results, unresolved defects, and cleanup confirmation. Use a controlled clock for state-machine unit tests and real time/network for lease integration tests. Capture artifacts from failures without leaking canary values into published reports.

P7 requires a 24-hour bounded soak, at least 100 iterations of each critical race/crash scenario, all core requirement tests passing, an independently followed quickstart and a successful backup/restore exercise. These are proposed gates; no results exist yet. Larger P10 load and P12 security assessment must add evidence, not simply repeat an unchanged small test suite.

See [security tests](threat-model.md), [capacity targets](requirements.md), and per-phase [acceptance criteria](roadmap.md).
