# Shipra delivery roadmap

Status: working plan. A phase advances only after its acceptance evidence is recorded. Phase 0-14 follow the existing requirements; Phase 15-17 assign gates to previously unnumbered future work. Later phases are conditional on demonstrated need.

| Phase | Deliverable | Exit evidence |
|---|---|---|
| P0 | Architecture, scope, threat model, decisions, risks and release gates | Broken references resolved; review decisions and open questions recorded |
| P1 | Go project, reproducible setup, PostgreSQL migrations, configuration, identity, roles, audit and safe bootstrap | Clean install; denied anonymous control calls; role matrix; migration and restart tests |
| P2 | Strict versioned `shipra.yml` compiler, static DAG and validation API/CLI | Fixture, graph, bounds and location-aware error tests |
| P3 | Immutable source/config plan, secret policy and local blob foundations | Changed ref cannot change admitted inputs; denied secret and unsafe path tests |
| P4 | Durable queue, scheduler, Linux Docker runner, lease/fencing, cancellation | Real PostgreSQL/Docker failure and duplicate-assignment tests |
| P5 | GitHub installation, signed webhook, manual trigger, status outbox | Duplicate delivery admits once; status outage retries |
| P6 | Run/history/log/artifact API, CLI and dashboard | Authenticated failure investigation from UI/CLI |
| P7 | v0.1 packaging, security, recovery, operations and docs | Isolated end-to-end, restore and release checklist on supported hosts |
| P8 | Team access, OIDC, external secrets, matrices, templates and trusted cache | Policy and compatibility tests per feature |
| P9 | Audited approvals and immutable artifact promotion to one selected target | Staging deployment, reconciliation and rollback drill |
| P10 | Multi-controller HA, shared blobs, autoscaling | Failover and published load results |
| P11 | Isolated versioned plugin SDK and representative integrations | Conformance, permission and crash-isolation tests |
| P12 | Operational SLO/DR, security assessment and supply-chain policy | Rehearsed recovery and independent findings closed |
| P13 | Kubernetes executor and ephemeral cloud runners | Isolation, lifecycle and cancellation tests |
| P14 | Optional AI assistance | Redaction, evaluation and explicit-action gates |
| P15 | Visual pipeline designer | Lossless schema round-trip and unchanged scheduling semantics |
| P16 | Native Windows/macOS runners and stronger hostile-workload isolation | Platform-specific execution and security reviews |
| P17 | Plugin catalog, federation and multi-region scheduling | Demand, architecture review and cross-region failure tests |

## Checkpoint rule

Each phase is split into reviewable features. A feature is committed and pushed after its focused checks pass; a phase is tagged only after all exit evidence passes. Never describe an untested feature as complete. Keep the independent GitHub build path available while Shipra is under development. Credentials, local databases and generated artifacts are never committed. If GitHub is unavailable, retain the local commit and report the unpushed state explicitly.
