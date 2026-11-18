# Requirements and release scope

Status: proposed. IDs below connect scope to the [roadmap](roadmap.md) and [test plan](testing.md). Product versions, pipeline schema versions, and runner protocol versions are independent.

## Core: v0.1 release requirements

| ID | Requirement | Acceptance evidence / phase |
|---|---|---|
| C01 | Install one control plane with PostgreSQL and persistent local blobs; enroll separate Linux Docker agents | Clean Linux and Windows/WSL2 setup walkthrough; P1/P4/P7 |
| C02 | Single workspace, projects, repository identity, admin/operator/viewer roles; deny anonymous control APIs | Authorization matrix and session/token tests; P1 |
| C03 | Strict versioned `shipra.yml`, static DAG, `needs`, parallel jobs, sequential steps, bounded resources and `when` predicates | Parser fixtures, graph invariants, location-aware errors; P2 |
| C04 | Immutable source/config snapshot and execution plan; checkout, shell commands, upload artifacts and explicit dependent-job downloads | Commit changes after admission do not change a run; P3/P5/P7 |
| C05 | Durable queue, runner capabilities, per-project fairness, leases/fencing, deadlines and cancellation | Restart/partition/duplicate claim exercises; P4 |
| C06 | GitHub push integration and manual trigger; webhook verification/deduplication, commit status reporting | Signed duplicate delivery creates one trigger; external outages leave retryable outbox records; P5 |
| C07 | Authenticated logs with reconnect, run/job/attempt history, visible queue and failure reasons | UI/CLI failure investigation without database access; P6/P7 |
| C08 | Scoped encrypted secrets, secret-use policy, token revocation, audit trail | Policy-denial and redaction adversarial tests; baseline P1/P3, release P7 |
| C09 | Isolated disposable workspaces, resource/size/time limits, cleanup and authenticated artifact access | No workspace cross-read; path traversal/unsafe archive tests; P3/P7 |
| C10 | API and CLI for validate, trigger, status, cancel, logs, artifact download | Contract and end-to-end tests; P2/P4/P6/P7 |
| C11 | Structured operational logs, metrics, trace propagation, readiness, backup/restore, documented upgrade | Failure diagnosis and isolated restore drill; P1/P7 |
| C12 | Versioned documentation, supported environment matrix and transparent limitations | P7 release checklist and changelog |

Workflow denotes a reusable graph concept; in v0.1 one pipeline configuration produces one graph and one PipelineRun. Stages are optional future presentation groupings, never hidden ordering rules. See the [pipeline definitions](pipeline-engine.md).

## Advanced: after v0.1

| ID | Capability | Planned phase |
|---|---|---|
| A01 | Project-level team permissions, OIDC identity federation, external secret stores, short-lived workload identity | P8 |
| A02 | Matrices, policy-managed reusable workflows/templates, bounded expression evaluation, trusted caches | P8 |
| A03 | Environment approvals, immutable artifact promotion, deployment locks, health evaluation, rollback/reconciliation and GitOps | P9 |
| A04 | Multiple control-plane instances, shared object storage, runner autoscaling and load-tested fair scheduling | P10 |
| A05 | SCM/notification/storage/deployment/identity integrations through a versioned plugin SDK | P11 |
| A06 | Operational hardening, tested SLOs/DR, security assessment, provenance and supply-chain policy | P12; foundations start P1 |
| A07 | Kubernetes executor and cloud ephemeral runner pools | P13 |

## Future

| ID | Capability | Entry condition |
|---|---|---|
| F01 | AI generation, diagnosis, optimization and remediation proposals | P14; redacted data policy, evaluation and authorization gates |
| F02 | Visual pipeline designer | Canonical schema round-trip without dropping unsupported fields; scheduling semantics unchanged |
| F03 | Native Windows/macOS executors; hostile workload VM/microVM pools | OS-specific cancellation/isolation tests and dedicated trust model |
| F04 | Plugin catalog, federated control planes, multi-region scheduling | Demonstrated demand and independent architecture reviews |

## Explicit v0.1 exclusions

No public/fork pull-request execution, arbitrary repository URLs, anonymous builds, hostile tenants, Jenkinsfile/Groovy compatibility, Jenkins plugins, privileged builds, Docker socket exposure to jobs, or building container images through a privileged host daemon. No native Windows/macOS jobs, SSH execution backend, Kubernetes dependency, autoscaling, HA, Redis or message broker dependency. No native deployment engine, production credentials, manual environment approval workflow, rollback, GitOps reconciler, marketplace, or AI.

No matrices, user-programmable conditional language, reusable templates, shared build caches, scheduled triggers, service containers, submodules, Git LFS, or guaranteed `.git` metadata in the initial source checkout. GitHub source archives are the initial checkout model. Pipeline artifacts are bounded files, not a general artifact/package registry. Teams requiring those features must wait for the appropriate gate or retain existing tooling.

## Proposed validation envelope

Target a single workspace with 10 projects, 10 enrolled agents and at most 10 concurrently executing attempts, one per dedicated agent host. This is a proposed test workload, not a shipping limit or performance result. On a documented baseline of 4 vCPU/8 GiB for the control plane and 2 vCPU/4 GiB for PostgreSQL (build machines separate), measure 1,000 queued jobs, 10 concurrent jobs each emitting 100 KiB/s logs, and a burst of 100 webhook deliveries/minute.

Proposed gates: 95th-percentile control API latency below 500 ms excluding uploads/long polls; eligible queue-to-assignment below 5 s when a compatible slot is idle; durable acknowledged state survives process restart; no duplicate active logical assignment; no lost acknowledged artifact/log chunks; bounded memory and disk under overload. Set thresholds before tests; publish actual hardware/results and adjust supported capacity honestly. A local single-host pilot has planned maintenance and no availability SLA.

For growth, test 1,000 concurrent jobs and 10,000 queued jobs in P10 before advertising organization scale. Job logs/storage/network may dominate far before graph scheduling. Capacity must be expressed with workload and retention, not only pipeline count.

## Assumptions requiring validation

Confirmed by the user on 2026-09-29: own repositories, Linux/Docker runners, a Windows-friendly development setup, and difficult setup/maintenance/plugin management as the first improvement priority. WSL2 is the proposed Windows implementation of that preference. GitHub remains the proposed first provider, pending confirmation. Critical unanswered questions, ownership and phase gates are recorded in [risks](risks.md).
