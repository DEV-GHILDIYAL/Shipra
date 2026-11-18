# Technology selection

Status: proposed. Research checked 2026-09-29. Complexity ratings describe Shipra's expected operating burden and are architectural judgment, not product benchmarks. Select exact supported patch versions and lock dependencies during P1; this document intentionally does not prescribe floating `latest` images.

## Language evaluation

| Choice | Benefits for this system | Trade-offs / operational complexity | Decision |
|---|---|---|---|
| Go | Suitable for concurrent HTTP services and an agent/CLI sharing contracts; goroutines simplify bounded I/O workers | Garbage collection and race risks still need profiling/tests; low deployment complexity | Recommended for core/runner/CLI |
| Rust | Ownership/type system catches many concurrency and memory hazards | More design effort for async and API ergonomics; low runtime complexity, higher expected solo-development cost | Revisit for a narrowly measured execution component |
| TypeScript/Node.js | Shared browser/server language and large web ecosystem | Must isolate blocking/CPU work from event loop; runtime/package patching on agents; moderate complexity | Use TypeScript for dashboard only |
| Java | Mature server ecosystem, strong tooling and virtual-thread concurrency | JVM/package/runtime footprint and separate frontend; moderate complexity | Viable alternative if maintainer expertise strongly favors it |

The concurrency mechanisms above are documented by [Go](https://go.dev/doc/effective_go), [Rust](https://doc.rust-lang.org/book/ch16-00-concurrency.html), [Node.js](https://nodejs.org/en/learn/asynchronous-work/dont-block-the-event-loop) and [Java](https://docs.oracle.com/en/java/javase/21/core/virtual-threads.html). None makes durable workflow execution or secure job isolation automatic. Solo-maintainer familiarity remains an explicit validation question.

## Infrastructure and libraries

| Area / choice | Alternatives considered | Reason | Trade-off and complexity |
|---|---|---|---|
| PostgreSQL | SQLite, MySQL, distributed DB | Transactions, relational constraints and row locking for state plus queue | One external DB to operate; moderate; SQLite lacks desired multi-writer concurrency model |
| Database queue + outbox | NATS JetStream, RabbitMQ, Kafka, Redis queues | Atomically couple state and work without another stateful system | Build fair selection/reaper carefully; DB can become bottleneck; low added infrastructure |
| Local blobs via storage interface | S3-compatible service from day one, DB large objects | Minimal personal setup; bytes kept outside relational hot tables | Single-host failure domain and coordinated backups; medium; shared object storage required for HA |
| Docker executor on Linux | Host shell, containerd directly, Kubernetes, VM/microVM | Accessible disposable builds with bounded resources | Docker daemon/host remain trusted; container is not hostile-tenant protection; medium |
| HTTPS JSON long poll | gRPC bidirectional stream, WebSocket, message broker | One transport stack, proxy/NAT friendliness, easy support diagnostics | More HTTP requests and explicit deduplication; modest initial complexity |
| Browser SSE + REST | WebSocket, polling only | One-way log/status delivery with cursor-based reconnection | Slow-client limits and proxy buffering configuration required; low |
| TypeScript + React + Vite | Server-rendered Go HTML, Vue, Svelte | Component model suitable for DAG/log/history views; static output embedded into server | JS build toolchain and browser security burden; moderate; no Node runtime required in server release |
| Explicit SQL via pgx; versioned SQL migrations | ORM, database code generation | Keep queue locks and transaction boundaries reviewable | More handwritten mappings; SQL integration tests mandatory; low-to-medium |
| YAML 1.2 parser + JSON Schema + semantic compiler | Groovy, embedded JS, general workflow engine | Bounded declarative contract, editor validation and graph checks | Must verify chosen parser rejects duplicates/aliases/tags and matches schema semantics; medium |
| Structured JSON logs, OTel traces, Prometheus metrics | Vendor-specific SDK, mandatory full telemetry stack | Portable telemetry and independently optional backends | Cardinality, privacy and exporter limits require design; low in core, medium with full stack |
| Standard library HTTP and vetted crypto packages | Broad framework, custom auth/crypto | Small dependency surface; policies remain explicit | Session/auth review still required; no hand-written cryptographic primitives |

PostgreSQL queue selection is supported by its [locking documentation](https://www.postgresql.org/docs/current/sql-select.html); scheduling fairness remains Shipra's work. Docker describes the daemon and kernel attack surface in its [security documentation](https://docs.docker.com/engine/security/). OpenTelemetry supports telemetry signals without requiring a particular storage backend. [OTel signals](https://opentelemetry.io/docs/concepts/signals/).

Use maintained dependencies with checked licenses, security advisories, compatible support windows and pinned versions. P1 must verify library choices against current releases before lockfiles exist. OpenAPI is a published interface-description standard; pin a tooling-supported version rather than silently follow the latest specification. [OpenAPI specification](https://spec.openapis.org/oas/latest.html).

## Infrastructure introduction gates

S3 becomes mandatory before multi-control-plane HA; a migration/checksum/recovery plan accompanies it. A broker is considered only after database contention and notification demand are measured; it must preserve outbox and idempotency semantics. Kubernetes arrives only with an explicit executor requirement. Prometheus/Grafana/OTel Collector are optional development profiles before production monitoring is required by P12.

No component is chosen simply because Jenkins uses or does not use it. Shipra's expected operator count and failure model drive the initial choices.
