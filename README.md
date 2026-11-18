# Shipra

Shipra is a proposed self-hosted CI/CD and software-delivery platform for personal development first, with a path toward larger engineering teams. Its goal is broad Jenkins capability coverage with simpler operation, explicit security boundaries, durable execution, and clearer diagnostics.

**Current status: Phase 1 foundation in progress.** The first code checkpoint is a server configuration validator. There is no runnable CI/CD service, installer, runner, pipeline parser, or tested performance claim yet. The architecture remains subject to each phase's acceptance gate.

Start with the [Phase 0 Architecture Review](docs/phase-0-review.md), then the [scope and requirements](docs/requirements.md) and [phased roadmap](docs/roadmap.md). All technology selections remain reviewable.

The initial `shipra-server -check-config -config PATH` command validates a JSON file against the [example](config.example.json). It does not start a server or create credentials. A Go 1.27 toolchain is needed to build it.

## Proposed first release

One Go control-plane process, PostgreSQL, local blob storage, and separately installed Linux Docker runners. A versioned `shipra.yml` describes a static job graph. GitHub push events or manual requests create durable runs; a small web dashboard and CLI show jobs, logs, and artifacts. Authentication, least privilege, audit events, cancellation, deadlines, and recovery are release requirements.

v0.1 targets your own trusted repositories in one workspace. It does not claim Jenkins parity, safe execution of public pull requests, high availability, production deployment orchestration, or a plugin marketplace. Windows development is planned through WSL2/Linux containers; native Windows execution is deferred.

## Architecture documentation

| Topic | Document |
|---|---|
| Product and scope | [Vision](docs/vision.md), [requirements](docs/requirements.md) |
| Existing systems | [Research and source comparison](docs/research.md) |
| System and choices | [Architecture](docs/architecture.md), [technology stack](docs/technology-stack.md), [decision index](docs/architecture-decisions.md) |
| Execution | [Pipeline engine](docs/pipeline-engine.md), [pipeline specification](docs/pipeline-spec.md), [scheduler](docs/scheduler.md), [runners](docs/runners.md) |
| Security | [Security architecture](docs/security.md), [threat model](docs/threat-model.md) |
| Contracts and persistence | [Data model](docs/data-model.md), [API boundaries](docs/api.md) |
| Operation | [Reliability](docs/reliability.md), [observability](docs/observability.md) |
| Future capabilities | [Deployments](docs/deployments.md), [plugins](docs/plugins.md) |
| Engineering | [Development and repository layout](docs/development.md), [testing](docs/testing.md) |
| Delivery and review | [Roadmap](docs/roadmap.md), [risks and assumptions](docs/risks.md), [Phase 0 review](docs/phase-0-review.md) |

Research date: 2026-09-29. Official sources are linked beside factual claims. Architectural judgments and proposed targets are labeled separately. No external deployment or GitHub repository has been created.
