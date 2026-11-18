# Product vision

Status: proposed, Phase 0, 2026-09-29.

Shipra should let a developer connect a repository, understand what will execute, run repeatable builds on owned infrastructure, investigate failures, and eventually promote verified artifacts through controlled environments. The platform should remain understandable to one maintainer while preserving a route to thousands of concurrent jobs. That scale is a future validation goal, not a current capability.

## Problems to address

These are product hypotheses to validate with actual Jenkins users. Jenkins already supports distributed agents, pipelines, credentials, and many integrations; Shipra must demonstrate improvements rather than claim those capabilities are absent. See the [evidence](research.md).

| Pain hypothesis | Shipra response | How to measure |
|---|---|---|
| Maintaining a collection of plugins increases upgrade work | Small built-in core; later versioned isolated integration contracts | Number of mandatory integrations to install; upgrade/restore drill results |
| Pipeline failures hide behind infrastructure failures | Separate command failure, queue timeout, lost runner, and artifact failure reasons | Time to identify injected failure from UI/CLI without server access |
| A controller is hard to operate and protect | No build execution on the control plane; durable relational state | Restart recovery and isolation acceptance tests |
| Build environments drift | Disposable containers, resolved image digests, immutable source/config snapshots | Record complete execution inputs and reproduce controlled fixture builds |
| Pipeline syntax becomes a second programming language | Strict YAML graph with typed built-ins and small condition vocabulary | Validation errors point to source location before a runner starts |
| Delivery needs more accountability | Future immutable promotions, approvals, environment locks, reconciliation | Complete actor/artifact/policy history for a deployment |

Deterministic orchestration means the same validated graph and recorded decisions produce the same scheduling semantics. It does not promise byte-identical builds: dependencies, networks, clocks, and tools need their own reproducibility controls.

## Intended users and progression

The initial user owns the repositories and runner machines. A later small-team release adds finer access control, identity federation, policy-managed templates, and safer credential issuance. Organization-scale operation adds measured HA, runner pools, independent integrations, and stronger isolation. The architecture reserves workspace ownership keys now without advertising hostile multi-tenant hosting.

Jenkins migration will be incremental: a capability inventory, supported translation patterns, and validation reports. Running arbitrary Jenkinsfiles or loading Jenkins plugins is not a compatibility goal. Existing CI may continue running while selected repositories migrate.

## Principles

Keep authoritative state durable; fail closed when authorization or leases cannot be established. Make duplicate delivery safe without claiming exactly-once shell execution. Treat repository content and build output as untrusted input. Expose APIs before automation shortcuts. Prefer a few deployable components over premature services. Add infrastructure only against measured limits.

AI is an optional future consumer of redacted evidence. The deterministic engine must run without any AI service. AI suggestions never acquire a privileged execution path outside normal policy and explicit authorization.
