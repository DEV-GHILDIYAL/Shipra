# Observability

Status: proposed. Minimum instrumentation is part of P1-P4; P12 hardens it, rather than introducing it for the first time.

Operational logs explain Shipra behavior; pipeline logs contain untrusted build output; audit events record accountable actions. Store and authorize them separately. OpenTelemetry supplies portable signal and context conventions; it is not Shipra's run database. [OTel concepts](https://opentelemetry.io/docs/concepts/).

## Signals and ownership

| Signal | Proposed fields/measures | Reader and persistence |
|---|---|---|
| Structured operational logs | timestamp, severity, service/version, request/trace ID, component, event code, safe reason, run/attempt ID | Operators; bounded JSON stdout/log sink |
| Pipeline logs | Attempt, stream, sequence, byte offset, observed time, redaction/truncation markers | Authorized project users; blob chunks with DB manifests |
| Audit events | Actor, action, object, policy revision, decision, request ID, safe changes | Admin; append-only DB with export in later phases |
| Traces | Ingest, source fetch, compile, admission, queue assignment, execution start/end, uploads, result, outbox | Optional OTLP exporter; link long-lived async spans by correlation IDs |
| Metrics | HTTP latency/errors, queue age, state transitions, runner capacity, DB pool, upload/storage pressure | Private `/metrics`; optional Prometheus/Grafana |

Do not create one continuously open trace per multi-hour build if it exhausts backend buffers. Use bounded spans and links across durable handoffs. A failed exporter cannot block scheduling; dropped export batches are counted. Do not export raw commands, repository content, secret values, credential URLs, or complete webhook payloads.

## Metric contract

| Metric | Type / useful labels |
|---|---|
| `shipra_http_request_duration_seconds` | Histogram; route template, method, status class |
| `shipra_queue_ready_jobs`, `shipra_queue_oldest_age_seconds` | Gauge; bounded pool and readiness reason |
| `shipra_assignment_latency_seconds` | Histogram; pool; ready-to-assigned excludes dependency-blocked time |
| `shipra_attempts_total` | Counter; terminal state and bounded reason code |
| `shipra_attempt_duration_seconds` | Histogram; executor and outcome |
| `shipra_runner_slots`, `shipra_runner_leases_lost_total` | Gauge/counter; pool/executor; per-runner detail via API |
| `shipra_log_ingest_bytes_total`, `shipra_log_dropped_bytes_total` | Counter; reason, not raw job identifiers |
| `shipra_blob_bytes`, `shipra_upload_failures_total` | Gauge/counter; blob kind and reason |
| `shipra_outbox_oldest_age_seconds`, `shipra_outbox_retries_total` | Gauge/counter; destination class |
| `shipra_reconcile_duration_seconds`, `shipra_db_pool_wait_seconds` | Histogram; operation class |

Keep IDs, commit SHAs, branch names, usernames and full URLs out of metric labels. High-cardinality labels multiply time series; this follows [Prometheus naming and label guidance](https://prometheus.io/docs/practices/naming/). Detailed per-run data belongs in API/logs/traces.

## Dashboards and alerts

First dashboard: ready jobs and oldest age versus compatible idle slots, assignment latency, runner connectivity, build failure versus infrastructure-loss rate, log lag, disk headroom, database latency and outbox backlog. Show deadlines and unavailable capability reasons to developers. Later add environment deployment histories and desired/observed state without conflating deployment health with build success.

Proposed alerts: eligible jobs waiting over 60 seconds while matching capacity is idle; reconciliation loop stalled for 30 seconds; outbox age over 5 minutes; disk over 80% warning/90% admission halt; repeated auth rejection; any unexpected fenced result; elevated runner lease losses. Tune thresholds from P7 measurements. Health readiness fails if DB authority or required storage checks fail; liveness only reports process responsiveness and must not cause destructive restart loops during database outages.

Track platform availability separately from user build success. User test failures should not burn platform error budgets. P7 validates the [initial latency envelope](requirements.md); P12 proposes an operator-approved SLO with real measurements. v0.1 promises no uptime SLA.

## Privacy and completeness

Apply redaction before durable ingestion and repeat at the server as defense in depth, with explicit limitations in [security](security.md). Remove terminal escape sequences dangerous to log viewers and render as text. Mark missing ranges; never show a partial log as complete. Treat artifact names and trace attributes as untrusted strings. Authorize historical log access exactly like live access and include telemetry in retention/export policy.
