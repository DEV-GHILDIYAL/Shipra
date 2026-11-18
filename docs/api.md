# API boundaries

Status: proposed contracts, not implemented endpoints or a generated OpenAPI file. P1-P4 produce reviewed machine-readable contracts. Runner wire details and normative lease behavior live in [runners](runners.md).

## Protocol choice

| Boundary | Protocol | Reason and cost |
|---|---|---|
| Dashboard / CLI to control plane | HTTPS REST JSON under `/api/v1` | Familiar clients, visible authorization and idempotency; requires explicit pagination/versioning |
| Dashboard logs/status | Authenticated SSE under `/api/v1/runs/{id}/events` plus ranged log reads | Unidirectional updates, reconnect cursor; proxies must avoid buffering and clients may lag |
| Runner to control plane | Outbound HTTPS JSON under `/api/runner/v1`, long polls and bounded uploads | Works across NAT, inspectable, one network stack; explicit acknowledgments, lease and sequence handling |
| GitHub to ingress | HTTPS webhook `/hooks/github/{integration_id}` | Verify raw-body signature before accepting bounded data; integration ID is not an authenticator |
| Control-plane modules | Typed in-process calls | No network/microservice overhead; enforce imports and transaction ownership |
| Future plugins | Authenticated local/private RPC, version handshake | Crash/capability isolation; additional process supervision |

Do not require WebSocket or gRPC in v0.1. Evaluate gRPC if measured high-volume agent traffic merits streaming/generated binary contracts; changing transport must preserve leases, sequence acknowledgment and authorization semantics. WebSocket is justified only by a future bidirectional UI feature; interactive shell access is not implied.

## Public resource surface

| Operation | Proposed endpoints | Authorization / behavior |
|---|---|---|
| Login/logout and current identity | `POST /api/v1/session`, `DELETE /api/v1/session`, `GET /api/v1/me` | Rate limits, secure session cookie, CSRF protection for mutations |
| Projects / repositories | `GET/POST /api/v1/projects`; scoped repository configuration subresources | Viewer read; admin configuration; stable provider IDs |
| Validate | `POST /api/v1/projects/{id}/pipeline-validations` | Authenticated size-bounded text; syntax/graph/policy diagnostics; no execution or secret reads |
| Trigger run | `POST /api/v1/projects/{id}/runs` | Operator/admin; repository ID+ref or commit within configured policy; resolves immutable revision; Idempotency-Key required |
| Trigger admission | `GET /api/v1/triggers/{id}` | Returns pending/admitted/rejected and eventual run URL; no fake run on rejected config |
| Run/job/attempt history | `GET /api/v1/runs/{id}` and project run collection | Scoped reads; includes reason, plan digest, attempt history, queue/capacity explanation |
| Cancel / rerun | `POST /api/v1/runs/{id}/cancel`, `POST /api/v1/runs/{id}/reruns` | Audited operator action; cancel is intent until stop confirmed; rerun creates a new run |
| Logs and artifacts | `GET /api/v1/attempts/{id}/logs`, `GET /api/v1/artifacts/{id}/content` | Verify workspace/project access for each request; cursor/byte ranges; safe download headers |
| Runners | `GET /api/v1/runners`; enrollment-token, drain/revoke actions | Admin manages; operator observes; no job can enroll an agent |
| Secrets | Project secret create/update/delete and metadata list | Admin only; never a general plaintext GET endpoint; policy-aware delivery only to assigned attempts |
| Audit and health | `GET /api/v1/audit-events`, `/health/live`, `/health/ready`, `/metrics` | Audit admin; health minimal unauthenticated info; metrics authenticated/private network |

A new trigger returns `202` with trigger ID only after durable inbox commit; accepted triggers subsequently produce a run or durable rejection. Return `503` if durable acceptance cannot occur. Do not acknowledge a webhook before recording it. Duplicate accepted delivery returns the existing receipt without a new run. Provider status failures are outbox state, not build state.

## Runner contract constraints

Enrollment consumes a one-use pool-scoped token. An enrolled runner authenticates heartbeat, claim and control requests with a revocable agent credential. Execution-specific writes require attempt/assignment/epoch binding; server-issued upload/secret grants expire with the lease and authorize specific objects only. Agent credentials never enter containers.

Long-poll calls release database transactions while waiting. Claim request IDs make a lost response safely repeatable. Heartbeats renew only current eligible leases; expired/fenced assignments cannot resurrect. Upload/event replay repeats the original sequence and digest; a different payload for the same sequence is rejected. Result acceptance checks assignment ownership and expected state atomically. See exact routes, response fields and time budgets in [runner protocol](runners.md).

## Common contract rules

Use opaque IDs, UTC RFC3339 timestamps, finite byte/time limits and cursor pagination (default 50, max 200 proposed). Error envelopes include stable `code`, readable `message`, safe field diagnostics, `request_id` and retryability; omit stack traces and credentials. Use 400 malformed input, 401 unauthenticated, 403 forbidden, 404 scoped absence, 409 stale/conflicting version, 422 invalid pipeline, 429 quota, 503 unavailable authority.

POST actions use `Idempotency-Key` scoped to principal and operation, payload hash and stored outcome. Same key/different payload is conflict. Policy/config updates require an expected resource version (`If-Match`); concurrent edits cannot silently overwrite. Unknown input properties fail validation, with explicit API versioning for breaking changes. Additive response fields must be tolerated.

SSE event IDs are durable cursors. Reconnect replays retained events after the cursor; an expired cursor returns a documented reset response and clients refresh the snapshot. Server may disconnect slow clients; it must not keep an unbounded buffer. Events notify readers of persisted log offsets; readers use authenticated log APIs for historical chunks. Token-bearing URLs and secret-bearing event payloads are prohibited.

CLI tokens have expiry/scopes and use an OS credential store or permission-restricted config, not shell history. Browser sessions use secure, HttpOnly, SameSite cookies, origin checks and CSRF tokens; bearer tokens are distinct from cookie authentication. Every object lookup checks ownership, not merely knowledge of an ID. HTTP does not imply trust between local processes.
