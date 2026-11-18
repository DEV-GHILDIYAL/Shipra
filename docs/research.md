# CI/CD architecture research

Status: Phase 0 proposal; architecture approval is pending. Documentation checked on **2026-09-29**. No application implementation or comparative performance benchmark was performed.

## Method and limits

This review uses official product/project documentation. **Documented behavior** means the linked source describes that capability; **Shipra assessment** means an engineering judgment about its fit for this project. A feature's omission here does not mean the product lacks it. Cloud, enterprise, and self-managed editions can differ; this is an architectural comparison, not a pricing or feature-entitlement comparison. Documentation URLs using `stable`, `latest`, or an unversioned path can change; implementation must pin and revalidate the selected dependency versions.

The comparison includes general CI engines, Kubernetes workflow infrastructure, and deployment controllers. Argo CD and Spinnaker provide lessons for delivery, but their operating models are not equivalent to general-purpose build runners. Public documentation often describes retries without specifying every partition or crash guarantee. No reviewed system is assumed to provide exactly-once external side effects.

## 1. Jenkins

**Documented behavior.** A controller schedules and monitors work on agents; executor slots and labels control placement. Jenkins recommends separating builds from the controller and describes scaling through controller sizing, more agents, and division across multiple controllers. This is already a distributed architecture, not inherently a single-machine build tool. [Agents](https://www.jenkins.io/doc/book/using/using-agents/), [architecting for scale](https://www.jenkins.io/doc/book/scaling/architecting-for-scale/).

Pipelines use a version-controlled Jenkinsfile, with Declarative and Scripted approaches. Plugins extend the platform extensively, but installations acquire explicit and sometimes implied dependencies that administrators must manage. [Jenkinsfile](https://www.jenkins.io/doc/book/pipeline/jenkinsfile/), [plugin management](https://www.jenkins.io/doc/book/managing/plugins/).

Pipeline durability persists execution state to survive interruptions. Jenkins explicitly documents a performance/durability trade-off: reducing persistence work can improve throughput but can lose resumability or visualization after an abrupt shutdown. This does not promise to resume every interrupted external command. [Pipeline scaling and durability](https://www.jenkins.io/doc/book/pipeline/scaling-pipeline/).

Credentials can be referenced by ID and stored encrypted on the controller. Access scope remains essential. Build artifacts can be archived and downloaded, with retention related to build retention; controller storage can be offloaded through artifact integrations. [Credentials](https://www.jenkins.io/doc/book/using/using-credentials/), [credential access guidance](https://www.jenkins.io/doc/book/security/credentials/), [artifact archiving](https://www.jenkins.io/doc/pipeline/steps/core/), [node/storage model](https://www.jenkins.io/doc/book/managing/nodes/).

**Shipra assessment.** Strengths are breadth, heterogeneous agents, expressive automation, and established recovery behavior. Costs for a small operator include plugin compatibility work, controller resource/storage planning, and a large configuration surface. These are trade-offs, not evidence that every Jenkins installation is unstable or insecure. Pipeline-as-code is valuable; a general-purpose controller-side programming runtime is unnecessary for Shipra's first DAG.

**Adopt:** separate execution hosts, repository-owned configuration, visible build history. **Improve for Shipra's intended workflow:** one documented default path, first-party essential features, explicit attempt and infrastructure-failure records. **Avoid initially:** build execution on the controller, third-party code loaded into its process, and promises of transparent shell-process recovery.

## 2. GitHub Actions

**Documented behavior.** YAML workflows define jobs, dependencies, conditions, matrices, permissions, and reusable workflows/actions. Self-hosted placement uses labels and groups; an assigned job that is not picked up within 60 seconds is requeued. Runner scale sets and Actions Runner Controller support autoscaling approaches, with ARC using Kubernetes. [Workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax), [self-hosted runner routing and scaling](https://docs.github.com/en/enterprise-cloud%40latest/actions/reference/runners/self-hosted-runners).

GitHub documents persistent-compromise risks for self-hosted runners, including private repositories. Runner groups restrict access; just-in-time runners consume at most one job, but reused hardware still needs cleaning. Its security guidance recommends narrowly scoped tokens and OIDC for short-lived cloud credentials. Third-party actions should be pinned to full commit SHAs. [Secure use](https://docs.github.com/en/actions/reference/security/secure-use).

Workflow artifacts retain outputs beyond a job; logs are available by workflow/job/step. These are separate products from dependency caches. The documented assignment requeue rule concerns unaccepted work; it does not imply that an interrupted running deployment can be safely replayed. [Artifacts](https://docs.github.com/en/actions/concepts/workflows-and-actions/workflow-artifacts), [workflow logs](https://docs.github.com/en/actions/how-tos/monitor-workflows/use-workflow-run-logs).

**Shipra assessment.** Strengths are repository-centered UX, reusable units, and clear job placement. Trade-offs are GitHub-specific semantics and supply-chain trust in referenced actions; self-hosted runners still need an isolation and patching strategy. Shipra should not promise Actions compatibility merely because it uses similar terms.

**Adopt:** a readable job/step hierarchy, immutable references, placement diagnostics, eventual workload identity. **Improve:** make assignment, acknowledgement, lease loss, and each retry visible. **Avoid initially:** executing untrusted pull requests on reused hosts, importing an action marketplace, and broad implicit secret inheritance.

## 3. GitLab CI/CD

**Documented behavior.** A Go runner requests jobs from GitLab, delegates to an executor, and reports logs/results; runner identity and job identity are distinct. Executors cover shell, containers, and other environments, with concurrency controls and runner tags. The Docker Autoscaler creates capacity through provider plugins and requires dedicated autoscaling resources per configuration. [Runner architecture](https://docs.gitlab.com/runner/), [Docker Autoscaler](https://docs.gitlab.com/runner/executors/docker_autoscaler/).

The YAML specification covers stages, DAG dependencies, rules, includes, and classified retry conditions. Its explicit infrastructure-failure categories are a useful alternative to retrying every nonzero exit. [CI configuration reference](https://docs.gitlab.com/ci/yaml/).

Artifacts are declared job outputs with access/expiry controls. Runners transmit job logs in chunks; standard logging archives completed logs, while optional incremental logging changes the storage path for larger deployments. [Artifacts](https://docs.gitlab.com/ci/jobs/job_artifacts/), [log architecture](https://docs.gitlab.com/administration/cicd/job_logs/).

GitLab warns that masking does not stop malicious scripts from stealing variables. Its runner guidance calls out shell-executor trust, persistent host contamination, privileged containers, shared Git workspaces, and network segmentation. [Variables](https://docs.gitlab.com/ci/variables/), [runner security](https://docs.gitlab.com/runner/security/).

**Shipra assessment.** Strengths are explicit executors, integrated repository/build UX, and mature job semantics. The large configuration vocabulary and coordination with a broader GitLab installation exceed Shipra's starting needs. Autoscaling adds provider ownership and reconciliation work; it is not simply increasing a concurrency integer.

**Adopt:** outbound job acquisition, separate runner/attempt credentials, structured failure reasons, chunked logs. **Improve:** a smaller strict schema and clear effective execution plan. **Avoid initially:** shell execution, shared mutable workspaces across attempts, and a cloud autoscaler before fixed agents are reliable.

## 4. CircleCI

**Documented behavior.** Workflows orchestrate jobs, each containing steps. Its documentation separates workspaces used between jobs in one workflow, reusable caches, and retained artifacts. Job pages expose output, results, artifacts, and resource information. [Jobs and steps](https://circleci.com/docs/guides/orchestrate/jobs-steps/), [data and configuration concepts](https://circleci.com/docs/guides/about-circleci/concepts/).

Self-hosted routing uses resource classes. The container runner polls for work and starts ephemeral Kubernetes pods, while machine runners serve other execution environments. [Runner concepts](https://circleci.com/docs/guides/execution-runner/runner-concepts/), [container runner](https://circleci.com/docs/guides/execution-runner/container-runner/).

Orbs package reusable jobs, commands, and executors; registry orbs use semantic versions. Contexts share and restrict environment variables. CircleCI explicitly warns that secret masking can be bypassed and does not cover secret values placed in artifacts. Workflow recovery includes rerunning failed jobs. [Orbs](https://circleci.com/docs//orbs/use/orb-concepts/), [contexts and masking](https://circleci.com/docs/guides/security/contexts/), [workflow recovery](https://circleci.com/docs/guides/orchestrate/workflows/).

**Shipra assessment.** Strengths are a useful persistence vocabulary and composable configuration. Costs include learning the boundaries among workflow state, workspace contents, caches, and executor capabilities. A pod-per-job design has infrastructure requirements that do not fit a mandatory Kubernetes-free v0.1. Rerun support should not be mistaken for rollback of earlier external effects.

**Adopt:** separate cache/output semantics, explicit required dependencies, versioned reusable configuration later. **Improve:** make artifact incompleteness and log truncation first-class UI states. **Avoid:** correctness that depends on a cache hit, implicit sharing of job directories, and a required cluster just to start Shipra.

## 5. Argo CD

**Documented behavior.** Argo CD is a Kubernetes GitOps deployment controller: an API server handles user operations and authorization, a repository server renders manifests, and an application controller compares desired state with live state. Repository/cluster credentials are stored as Kubernetes Secrets. It does not replace a general-purpose compilation/test runner. [Architecture](https://argo-cd.readthedocs.io/en/stable/operator-manual/architecture/).

Scaling includes repository-server concurrency and application-controller sharding across clusters. Automated synchronization has explicit self-heal, pruning, retry, and rollback interactions; for example, rollback is unavailable while automated sync is enabled. These semantics are policy choices, not a generic promise of automatic recovery for any deployment. [High availability](https://argo-cd.readthedocs.io/en/stable/operator-manual/high_availability/), [automated sync](https://argo-cd.readthedocs.io/en/stable/user-guide/auto_sync/).

Config-management plugins extend manifest generation. Argo CD recommends resolving application secrets at the destination cluster, reducing the controller's need to handle their values. Its central output is desired/live resource state, rather than a CI artifact archive. [Secret management and plugin implications](https://argo-cd.readthedocs.io/en/stable/operator-manual/secret-management/).

**Shipra assessment.** Strengths are continuous reconciliation, drift visibility, and an explicit separation between configuration and observed health. Costs are Kubernetes dependence and privileged access to deployment targets. Application logs and deployment-controller logs must be understood separately from build logs. For Shipra's future delivery layer, an integration may offer more value than rebuilding a Kubernetes reconciler.

**Adopt later:** desired/observed state, health checks, environment ownership, explicit drift policy. **Improve in Shipra's UX:** distinguish build success, deployment acceptance, and verified health. **Avoid:** treating a successful `kubectl` command as proof of a healthy release or exposing production credentials to ordinary build jobs.

## 6. Tekton

**Documented behavior.** Tekton extends Kubernetes with Tasks, TaskRuns, Pipelines, and PipelineRuns. Tasks contain container steps; run objects separate a reusable definition from an execution. Pipeline definitions express dependencies, conditions, retries, and timeouts. Kubernetes provides the underlying pod placement. [Building blocks](https://tekton.dev/docs/pipelines/), [Tasks](https://tekton.dev/docs/pipelines/tasks/), [Pipelines](https://tekton.dev/docs/pipelines/pipelines/).

Workspaces bind shared storage, while results carry values between tasks; they are not interchangeable with a retained binary artifact store. Shared persistent workspaces do not gain filesystem isolation merely because different PipelineRuns use them. Runtime credentials and image-pull credentials have different injection paths. [Workspaces](https://tekton.dev/docs/pipelines/workspaces/), [runtime authentication](https://tekton.dev/docs/pipelines/auth/).

Execution logs originate in task pods. Tekton Results can retain execution history and logs outside the run resources, allowing old objects to be removed from the cluster. Reusable Tasks and Custom Tasks provide extension mechanisms. A retry reruns a Task; it cannot reverse arbitrary commands from an earlier attempt. [Execution logs](https://tekton.dev/docs/pipelines/logs/), [Results](https://tekton.dev/docs/results/), [retry and Custom Task semantics](https://tekton.dev/docs/pipelines/pipelines/).

**Shipra assessment.** Strengths are composable execution primitives and integration with Kubernetes policy/scheduling. Costs are operating a cluster and assembling a broader developer product around the primitives. Kubernetes pod availability alone does not settle job-result or external-side-effect semantics.

**Adopt:** immutable definition snapshots versus run records, explicit inputs/outputs, retention independent of execution. **Improve:** a coherent one-developer installation and default dashboard/API. **Avoid initially:** encoding every domain entity as a CRD or assuming container separation makes shared data safe.

## 7. Buildkite

**Documented behavior.** Its hybrid architecture separates a SaaS control plane from customer-operated agents; hosted agents are also offered. The operator controls execution capacity and environments, while Buildkite handles orchestration and its interface. Agent configuration includes queues and log-output options. [Architecture](https://buildkite.com/docs/pipelines/architecture), [agent configuration](https://buildkite.com/docs/agent/self-hosted/configure).

Pipeline YAML can use plugins that hook into job lifecycle events such as checkout, command execution, artifact handling, and cleanup. Such hooks execute consequential code, and their evaluation order matters. Command-step retries are configurable. [Plugins](https://buildkite.com/docs/pipelines/integrations/plugins), [retries](https://buildkite.com/docs/pipelines/configure/retry).

Secret-storage integrations can retrieve credentials in the execution environment. Artifacts can use Buildkite-managed storage or an operator's storage provider; agents upload directly to storage. Consequently, storage residency depends on configuration, and secret disclosure through job output remains a concern. [Secret management](https://buildkite.com/docs/pipelines/security/secrets/managing), [artifact architecture](https://buildkite.com/docs/pipelines/configure/artifacts).

**Shipra assessment.** Strengths are the clean orchestration/compute boundary and extensible execution environments. A hosted control plane is not the fully self-hosted product the user requested. Agent hooks are powerful but enlarge host trust and make ordering part of pipeline behavior. Automatic retry remains distinct from safely recovering external effects.

**Adopt:** independently operated compute, queue/capability routing, external storage interfaces. **Improve:** a self-hosted control plane with inspectable scheduling reasons. **Avoid initially:** arbitrary agent lifecycle hooks and dynamic graph mutation; approve a static graph before assigning work.

## 8. Drone

**Documented behavior.** Runners poll the server and can be installed on multiple machines. Docker runners execute steps in ephemeral containers and suit stateless workloads; other runner types cover other environments. Routing can match pipeline requirements to runner labels. [Runners](https://docs.drone.io/runner/overview/), [Docker runner](https://docs.drone.io/runner/docker/overview/), [routing](https://docs.drone.io/pipeline/docker/syntax/routing/).

Container-oriented YAML and image-based plugins keep common pipelines concise. Plugins perform operations such as publication and notification without needing an in-process server extension. Repository secrets are provided through named references; pull-request exposure requires deliberate configuration. Logs are retrieved with repository authorization and stage/step identity. [Docker pipelines](https://docs.drone.io/pipeline/docker/overview/), [plugins](https://docs.drone.io/pipeline/docker/syntax/plugins/), [secrets](https://docs.drone.io/secret/repository/), [log API](https://docs.drone.io/api/builds/build_logs/).

**Shipra assessment.** Strengths are a small conceptual surface and container-based integrations. An image is executable third-party code; container packaging does not establish its trustworthiness. Artifact publication through a plugin also needs separate lifecycle, access, and retention rules. The reviewed overview material does not establish a complete lease/fencing or crash-resume contract; Shipra must specify its own rather than infer one.

**Adopt:** straightforward YAML and outbound agents. **Improve:** artifacts and attempt recovery as first-party domain concepts. **Avoid:** allowing publication plugins to inherit all project secrets or advertise exactly-once execution without a defined protocol.

## 9. Woodpecker

**Documented behavior.** The current docs describe configurable agent concurrency, labels, gRPC connectivity, system or per-agent authentication tokens, and an optional one-workflow agent mode. Local workflow execution is available through its CLI. [Agent model](https://woodpecker-ci.org/docs/administration/configuration/agent), [local execution](https://woodpecker-ci.org/docs/intro).

Image-based plugins are steps and share the workspace; they can publish artifacts. Secrets are individually referenced, can be filtered by event/plugin, and are withheld from pull requests by default. External secret values do not receive the same automatic masking as native values. [Plugins](https://woodpecker-ci.org/docs/usage/plugins/overview), [secrets](https://woodpecker-ci.org/docs/usage/secrets).

The server supports SQLite, MySQL/MariaDB, or PostgreSQL. Its documentation assigns database backup to external tooling and notes storage growth from logs. The CLI exposes pipeline history, log retrieval, stop/start, and purge operations. [Server storage and operations](https://woodpecker-ci.org/docs/administration/configuration/server), [CLI](https://woodpecker-ci.org/docs/cli).

**Shipra assessment.** Strengths are small-installation accessibility, container workflows, and local developer feedback. The reviewed pages do not define full recovery semantics for every server/agent crash, so no absence or guarantee is asserted. Shared workspaces and trusted plugin images still require security policy. Shipra should not adopt a source claim that plugin filtering makes disclosure impossible: a compromised allowed image can misuse any secret it receives.

**Adopt:** per-agent credentials, local validation UX, clean execution lifecycles. **Improve:** explicit backup/restore acceptance tests, bounded log retention, lease behavior. **Avoid:** system-wide shared runner credentials and assuming a plugin container is a secret-exfiltration barrier.

## 10. Spinnaker

**Documented behavior.** Spinnaker separates API/UI, orchestration, cloud mutation/state caching, metadata persistence, and other concerns across services. Orca coordinates stages through persistent state and a distributed work queue; scaling depends on both worker and persistence capacity. [Service architecture](https://spinnaker.io/docs/reference/architecture/microservices-overview/), [Orca internals](https://spinnaker.io/docs/community/contributing/code/developer-guides/service-overviews/orca/).

Deployment pipelines support manual judgment, parallel paths, templates, and execution history. A deployment operation may require multiple provider calls followed by polling and failure remediation. Its model is not simply a fleet of CI shell runners. [Pipeline model](https://spinnaker.io/docs/concepts/pipelines/), [deployment lifecycle](https://spinnaker.io/docs/reference/architecture/load/).

Artifacts identify deployable resources from external systems. Secret references can resolve through supported secret engines. Extension points and plugins can add provider or stage behavior. [Artifacts](https://spinnaker.io/docs/reference/artifacts/), [secrets](https://spinnaker.io/docs/reference/secrets/), [extension guides](https://spinnaker.io/docs/guides/user/).

**Shipra assessment.** Strengths are deployment-specific state, approvals, provider integration, and auditability. Its many-service operating model is too much for Shipra v0.1. Persistent orchestration improves node-failure recovery but does not by itself guarantee that a provider mutation happened once; provider status reconciliation is essential. Execution history also serves a different purpose from raw build stdout.

**Adopt later:** environment serialization, immutable release references, policy gates, provider operation identifiers, health verification and compensation. **Improve:** expose uncertain deployment outcomes instead of silently retrying them. **Avoid initially:** copying its microservice topology or coupling production cloud credentials to CI agents.

## Cross-system conclusions

The facts supporting this comparison are linked in the profiles. The following are **Shipra design judgments**, not claims that competitors universally lack these qualities.

| Concern | Lesson from the reviewed models | Proposed Shipra response |
| --- | --- | --- |
| Scheduling | Labels, tags, resource classes, and queues are useful; they must not be treated as authorization by themselves. | Match required capabilities and server-approved scope, enforce capacity, and expose why a job is waiting. Begin with fixed agents and bounded concurrency. |
| Scaling | More execution capacity and more orchestration capacity are different changes. | Scale agents first. Measure database claim latency, queue age, log throughput, and controller load before splitting the control plane. |
| Config and UX | Repository configuration enables review; rich languages and implicit extension order add complexity. | Strict versioned YAML, static bounded DAG, immutable resolved configuration per run, unknown-field errors, local validation. |
| Plugins | A reusable config package, container step, host hook, and server extension have different privileges. | First-party internal provider interfaces first; later versioned process/RPC extensions with explicit capabilities. No third-party in-process core loading. |
| Secrets | Masking reduces accidental exposure; a job receiving a secret can deliberately disclose it. | Authorize secret grants per job; keep control-plane/agent credentials out of jobs; redact before persistence; document residual leak paths. Prefer short-lived credentials later. |
| Artifacts | Retained outputs differ from caches and temporary workspaces. | Immutable output identity, checksum/size metadata, authorization, required-upload finalization and retention. Caches must remain disposable. |
| Logs | Build stdout and platform diagnostics have different readers, volume, and retention. | Separate streams and permissions; bounded chunks with sequence numbers, resumable acknowledgement, quotas and explicit truncation/incompleteness markers. |
| Recovery | A retry creates new execution, while resume and reconciliation are distinct operations. | Durable attempts, leases, fencing tokens, idempotent API receipts, and explicit policy for uncertain outcomes. Never claim exactly-once shell side effects. |
| Trust | Persistent hosts, shared directories, privileged containers and broad network access increase blast radius. | Dedicated Linux agent hosts, fresh per-attempt workspace, unprivileged job containers, resource limits, deny control-plane reachability from jobs where operationally enforceable. |
| Delivery | Build completion is different from a deployment reaching healthy desired state. | v0.1 focuses on CI. A future deployment module owns approvals, environment locks, provider operation state, health checks, and compensation. |

## Jenkins problems translated into testable Shipra goals

Shipra's long-term ambition is Jenkins-class usefulness, not immediate feature parity. The targets below avoid unmeasurable claims such as "faster than Jenkins" or "no maintenance."

| Operator problem | Evidence and qualification | Shipra target to validate |
| --- | --- | --- |
| Core workflows require managing many extension versions. | Jenkins documents dependencies and implied dependencies; actual complexity depends on the installation. [Plugin management](https://www.jenkins.io/doc/book/managing/plugins/) | A fresh supported installation runs checkout/test/artifact/log workflows without installing external plugins; releases test the supported component set together. |
| Controller load and storage grow with pipeline activity. | Jenkins explicitly documents controller sizing and durability-related disk costs. [Scaling](https://www.jenkins.io/doc/book/scaling/architecting-for-scale/), [durability](https://www.jenkins.io/doc/book/pipeline/scaling-pipeline/) | Keep binary/log payloads outside relational run state; measure scheduling under simultaneous uploads; document a measured capacity envelope. |
| Pipeline failures are hard to distinguish from infrastructure failures. | This is a product hypothesis to validate with the user's workflows, not a claim that Jenkins has no diagnostics. | Distinct parser, checkout, capacity, agent-loss, timeout, command, artifact, and platform error reasons; retain each attempt's evidence. |
| Setup and upgrades require too many unrelated decisions. | Jenkins's flexibility serves many environments; that breadth is not required by Shipra's first user. | One supported initial topology with documented migrations, version compatibility, backup/restore, and a reproducible development setup. |
| Build code can threaten control infrastructure. | Jenkins recommends external agents; GitHub/GitLab also document self-hosted execution risk. [Jenkins scaling/security rationale](https://www.jenkins.io/doc/book/scaling/architecting-for-scale/), [GitLab runner security](https://docs.gitlab.com/runner/security/) | No builds in the server process or on its host in the supported operational topology; validate credentials, network access, cleanup, and container restrictions adversarially. |

## Proposed architectural synthesis

Start with **one Go control-plane binary**, **PostgreSQL**, **a local blob/log directory behind a storage interface**, and **one or more Linux Docker agents**. API, authentication, parser, DAG engine, scheduler, log/artifact handling, repository integration, and audit are modules in that binary, not separately operated microservices. The database contains durable execution/queue/outbox records; payload files do not become giant database rows. These are proposals to evaluate in the architecture and ADR documents, not a claim that competitor designs prove a particular throughput.

Agents initiate outbound HTTPS JSON requests, including long-poll claims and heartbeats. Each assignment has an attempt identity, expiring lease, and fencing value. Reports and artifact finalization are conditional on the current attempt, and duplicate receipts are idempotent. Fencing protects Shipra's state; it cannot revoke an already-issued request to an unrelated external service. Expired or disconnected work therefore requires a policy that acknowledges uncertainty.

No Redis, message broker, Kubernetes cluster, SaaS coordinator, or AI service is required initially. A transactional outbox lets status notifications be retried without losing the database commit that created them. Later S3-compatible storage and additional execution backends sit behind explicit interfaces; adding a broker or replicated control plane requires evidence and a new decision record.

The proposed **v0.1 trust scope** is owned, trusted repositories in one workspace, one control plane, and multiple operator-managed agents. Pipeline code is nevertheless treated as potentially hostile for design and adversarial testing: dependencies, compromised accounts, and mistakes can introduce hostile behavior. The scope is an operational restriction, not proof of safety. Public fork workloads and mutually untrusted tenants need stronger isolation and authorization controls before support is claimed.

## Assumptions to validate before implementation commitments

1. **Real workloads:** collect representative pipelines, languages, image sizes, output sizes and desired concurrency. Thousands of future pipelines is a design direction, not a tested capacity claim.
2. **Host support:** confirm Linux agents are acceptable for the first release; native Windows/macOS builds require separate executor design and testing.
3. **Docker constraints:** validate required builds without privileged mode or a job-mounted host Docker socket. Container-image building may need a separately isolated service or executor later.
4. **Durability:** fault-inject controller restart, agent death, delayed heartbeats and duplicate reports; demonstrate that stale attempts cannot finalize the current result.
5. **Storage:** measure filesystem log/blob behavior and backup consistency with PostgreSQL; define the point at which remote object storage becomes necessary.
6. **Performance:** benchmark claims, cancellations and webhook bursts alongside verbose builds. Retention and backpressure must prevent logs from overwhelming scheduling.
7. **Recovery semantics:** decide what the user considers safely rerunnable. Builds may publish packages or mutate test systems even when called "CI"; default automatic retries must reflect that.
8. **Security:** prove secret access checks and log redaction behavior while documenting that transformed secrets, artifacts, and outbound network requests remain possible exfiltration paths after authorized injection.
9. **Product boundaries:** validate whether integrating an existing deployment controller later better serves target environments than implementing every provider operation.
10. **Documentation drift:** recheck referenced upstream behavior and selected versions at each relevant phase. No competitor release number, benchmark, or vulnerability ranking is inferred from this survey.

Phase 0 ends at architecture review. This research authorizes neither production implementation nor the next phase.
