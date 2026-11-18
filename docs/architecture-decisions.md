# Architecture decision index

The current design selects a Go modular control plane, PostgreSQL authoritative state, local blobs for the personal release, and a separately installed Linux Docker runner. The first client protocol is JSON over HTTPS. These choices are recorded in [architecture](architecture.md), [technology stack](technology-stack.md), [runner design](runners.md) and [scheduler design](scheduler.md). Change a choice through an explicit review before dependent implementation.

- [ADR-006: isolated extension model](adr/ADR-006-plugin-model.md) applies from P11.
- [ADR-007: secret management](adr/ADR-007-secret-management.md) applies before secret-bearing execution.
