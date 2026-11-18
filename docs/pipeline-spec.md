# Proposed pipeline specification

Status: **Phase 0 proposal — awaiting architecture approval.** This is a specification design, not an implemented parser or a published compatibility promise. Defaults and limits are provisional until the parser, runner and failure tests in [testing](testing.md) pass.

## Version and parsing contract

The repository root contains exactly one `shipra.yml`. `version: "1"` selects the first Shipra language version; it is independent of the Shipra application version. v0.1 implements only this language version. Each run stores the raw file, its digest, language version, parser version and compiled DAG digest with the resolved commit. Editing a branch or upgrading the server cannot change an admitted run.

Use a restricted YAML 1.2 document. YAML 1.2 defines mappings and scalar types; Shipra deliberately adds restrictions to make diagnostics and security behavior predictable. Do not rely on a YAML 1.1 parser interpreting `on` correctly. [YAML 1.2.2 specification](https://yaml.org/spec/1.2.2/)

The parser must reject duplicate keys, non-string mapping keys, multiple documents, aliases, anchors, merge keys, explicit/custom tags, unknown fields, invalid enum values, implicit string coercions, and unsupported language versions. An empty document is invalid. Mapping order never controls job scheduling. Steps use list order. Strings such as environment values and the version must be quoted when they could otherwise parse as numbers or booleans.

Admission applies byte, nesting and collection limits before allocating a full graph. Diagnostics include file, line, column, field path, stable error code and an actionable message. Parsing, DAG validation and policy checks execute no repository code and fetch no referenced templates, images or URLs. A future field is rejected today even if its name sounds plausible.

## Complete v0.1 example

This example uses only proposed v0.1 fields. Image tags are accepted for local convenience; the runner records the resolved digest on every attempt. A registry digest should replace the tag when reproducible image identity is needed.

```yaml
version: "1"
pipeline:
  name: application-ci
  timeout: "2h"

on:
  push:
    branches: [main]
  manual: true

env:
  CI: "true"

jobs:
  build:
    runs-on:
      executor: docker
      labels: [linux, amd64]
    image: node:22-bookworm-slim
    timeout: "30m"
    retries: 0
    resources:
      cpu: 1
      memory: "1GiB"
    steps:
      - checkout: {}
      - name: Install
        run: npm ci
      - name: Test
        run: npm test
      - name: Build
        run: npm run build
    artifacts:
      - name: distribution
        paths: [dist]
        when: on_success
        required: true

  inspect:
    needs: [build]
    runs-on:
      executor: docker
      labels: [linux, amd64]
    image: alpine:3.22
    steps:
      - download:
          job: build
          artifact: distribution
          path: input
      - name: Inspect output
        run: find input -type f

  failure-report:
    needs: [build, inspect]
    when: on_failure
    runs-on:
      executor: docker
      labels: [linux, amd64]
    image: alpine:3.22
    steps:
      - run: 'printf "%s\n" "An upstream job failed; inspect its Shipra logs."'
```

`inspect` receives a new empty workspace; it does not inherit the build workspace. `failure-report` does not send an external notification. Shipra provides a failure-reporting execution path; integration delivery belongs to the notification service.

## Fields and defaults

| Location | Accepted fields and meaning |
|---|---|
| Root | Required `version`, `pipeline`, `on`, `jobs`; optional `env` |
| `pipeline` | Required `name` display string, 1–100 characters; optional `timeout` duration, default `2h`, maximum `24h` |
| `on` | At least one of `push` and `manual`; no other trigger types |
| `on.push` | Required nonempty `branches` list of exact branch names, at most 100. No globs, regex, paths or tag filters |
| `on.manual` | Boolean `true` enables authenticated manual triggering; absent or `false` disables it |
| `jobs` | Mapping of 1–100 unique job IDs to job definitions; IDs match `[a-z][a-z0-9_-]{0,62}` |
| Job | Required `runs-on`, `image`, `steps`; optional `name`, `needs`, `when`, `timeout`, `retries`, `env`, `secrets`, `resources`, `artifacts` |
| `job.name` | Display string, 1–100 characters; default job ID |
| `job.needs` | List of distinct job IDs, default empty; no self-reference or missing target; whole graph must be acyclic |
| `job.when` | `on_success` default, `on_failure`, or `always`; evaluated after all direct dependencies are terminal |
| `job.runs-on` | Required `executor: docker`; optional `labels` list, default `[linux]`. All labels must match administrator-approved runner capabilities |
| `job.image` | Nonempty container registry image reference, maximum 512 characters; registry policy is checked separately. No expression evaluation |
| `job.timeout` | Physical attempt duration, default `30m`, maximum `6h`; includes preparation, commands, artifact finalization and termination grace |
| `job.retries` | Integer 0–2, default 0; number of additional physical attempts after eligible command failures, not total attempts |
| `job.resources` | Optional `cpu` number in quarter-core increments, 0.25–8, default 1; `memory` binary size, minimum `128MiB`, maximum `16GiB`, default `1GiB`. Administrator policy may lower these maxima |
| `env` | Root, job and command-step maps from environment variable names to literal string values |
| `job.secrets` | Map from environment variable name to project secret name; names and authorized secret version IDs are snapshotted, values are never part of the config snapshot |
| `job.steps` | Ordered list of 1–100 typed steps; types detailed below |
| `job.artifacts` | Optional list of at most 20 named output declarations; details below |

Durations are a positive integer followed by exactly one of `s`, `m`, `h`; fractional, zero, negative and combined forms are rejected. Job and step timeout minima are `10s`; pipeline timeout minimum is `1m`. Binary sizes use an integer plus `MiB` or `GiB`. These simple grammars are part of version 1, not a permissive language-library duration parser.

Branches use names such as `main`, not `refs/heads/main`. The GitHub adapter compares the event's resolved branch against this exact list. A manual trigger resolves its requested branch or commit once and reads configuration at that same commit. It cannot override pipeline fields or provide arbitrary secrets. Repository authorization and branch policy apply to both trigger paths.

## Step types and process behavior

Every step has exactly one of `checkout`, `run`, or `download`. Each may have optional `name`, `when` and `timeout`. Step `when` defaults to `on_success`, except `checkout` and `download` require `on_success` in v0.1. Step timeout defaults to the remaining attempt budget; a supplied timeout is capped by that budget and has the same `6h` maximum. A step exceeding its timeout times out the attempt and starts termination; no further repository steps run.

| Type | Contract |
|---|---|
| `checkout: {}` | At most once, as the first step. Safely extract the immutable source archive for the admitted commit into the empty workspace. No user-supplied URL, branch, token or path. The archive has no `.git`; Git submodules and Git LFS materialization are deferred |
| `run: string` | Execute the literal script through `/bin/sh -eu` inside the job container. Maximum 64 KiB per script. Optional `working-directory` relative to the workspace, default workspace root; optional `env` map. No implicit `pipefail`, Bash, PowerShell, expression interpolation or host shell |
| `download: {job, artifact, path}` | Download a named output from a job in the current job's direct `needs`. The producer must have finished `succeeded`. Select the artifact only from that producer's successful final attempt in this run. Verify the manifest and content digest, then safely extract into a new empty relative directory. No overwrite, cross-run lookup, `latest`, remote URL or unverified archive |

A run step is a fresh shell process. Files remain within the same job container/workspace between steps; shell variables, `cd` and shell options do not carry over. To persist environment values use declared `env`; dynamic environment files and job outputs are deferred. `/bin/sh` must exist in the selected image. The agent does not install missing utilities implicitly. A zero shell exit code succeeds; a nonzero code sets a failure latch for the attempt. Ordinary later steps then skip, while `on_failure` and `always` command steps may run if there is remaining time and no cancellation. A successful cleanup step does not clear an earlier failure.

Paths are literal relative POSIX paths. Reject absolute paths, `..` components, empty components, NULs and paths resolving outside the workspace. Output and download extraction reject symlinks, hardlinks, device nodes, FIFOs and path traversal. No glob expansion is offered for artifact declarations in v0.1. A download directory cannot overlap source or existing files. Each job starts with a fresh workspace, including every retry.

## Conditions, dependencies and failure

Conditions are three named policies, not a general expression language. All direct dependency JobRuns must first reach terminal state, including exhausting retries. The policies are:

| Policy | Job condition | Command-step condition |
|---|---|---|
| `on_success` | Every direct dependency is `succeeded`; true with no dependencies | No earlier command/helper failure latch |
| `on_failure` | At least one direct dependency is `failed`, `timed_out` or `lost`; invalid on a root job | An earlier command/helper failure latch exists |
| `always` | True after dependencies become terminal | True while attempt remains authorized and within its deadline |

For a job, `skipped` is not success and does not itself count as failure. Thus failure propagates visibly through default jobs as skips, and a report job should list the upstream failure sources it wants to inspect in `needs`. A false condition makes the JobRun `skipped` without creating an Attempt. Cancellation and the pipeline deadline suppress **all** new jobs and command steps, including `always`; runner-owned process termination and filesystem cleanup are separate and always attempted. v0.1 has no individual-job cancellation API, `allow_failure` or `continue_on_error`.

There is no fail-fast cancellation of independent branches in v0.1: they finish subject to run cancellation/deadline. A successful error-handling job does not convert a failed pipeline to success. See [pipeline engine](pipeline-engine.md) for aggregation and terminal states.

## Variables and secrets

Environment names match `[A-Za-z_][A-Za-z0-9_]*`. Root environment is overridden by job environment, then step environment. A job secret injects only the named variables into that job's command containers. Reject a variable defined as both a secret and any applicable plain environment value. `SHIPRA_*` is reserved and immutable; the platform provides `SHIPRA_RUN_ID`, `SHIPRA_JOB_ID`, `SHIPRA_ATTEMPT_ID`, `SHIPRA_COMMIT_SHA` and `SHIPRA_WORKSPACE`. Pipeline text receives no string interpolation; shell expansion such as `$SHIPRA_COMMIT_SHA` happens only in the explicitly requested shell process.

Secret names are validated and permission-checked at admission. Resolve a specific secret version for the run; rotations apply to new runs, while revocation is checked again before dispatch and prevents use of a revoked version. Secret bytes are obtained only for the authorized current attempt, are not placed in command-line arguments or source archives, and are unavailable to unrelated jobs. Revocation cannot remove a value already delivered to a running job. Exact-value masking is a precaution, not protection against code that intentionally encodes or exfiltrates a secret. See [security](security.md).

## Artifact contract

An output declaration accepts exactly `name`, `paths`, `when`, `required`. `name` follows the job-ID grammar and is unique within that job. `paths` is a nonempty list of at most 20 literal files/directories relative to the workspace. `when` is `on_success` (default), `on_failure` or `always`; it is evaluated against the command/helper failure latch before artifact uploads. `required` is a boolean, default `true`.

After command steps complete, the agent selects matching declarations and uploads a deterministic manifest and file content under the Attempt ID. An artifact selected by its condition but missing any declared path or failing upload fails the attempt when required. Optional outputs record a warning and do not change command outcome. A declaration whose condition is false creates no artifact and is not an error. Cancellation, timeout or lease loss does not promise artifact upload; already committed outputs remain inspectable but never imply success.

The control plane marks an artifact available only after size, digest and storage commit are verified. Publication and successful result acceptance are ordered: a job cannot report success with an incomplete required output. Failed-attempt artifacts are retained for diagnosis but are ineligible as dependency inputs. Artifact content is immutable once committed. Download helpers resolve IDs server-side from the current DAG, never from user-controlled object keys. All outputs are subject to project visibility, download authorization, retention and per-attempt byte/file quotas. Proposed limits: 1 GiB total artifact content and 10,000 regular files per attempt, 5 GiB expanded input data per attempt.

## Resource bounds and version evolution

Initial parser limits: 256 KiB raw configuration; depth 20; 100 jobs; 1,000 total steps; 1,000 edges; 100 variables per scope; 16 KiB total literal environment data per job after merge; 32 secret bindings per job. These are intentionally conservative safety bounds, not evidence of tested capacity. [Scheduler](scheduler.md) defines admission and runtime quotas separately.

Future language versions may add matrices, reusable workflows, templates, cache keys, expressions, typed parameters, service containers, environment approvals and deployment jobs. **None of those fields is accepted by version 1 in v0.1.** Unknown keys such as `matrix`, `uses`, `if`, `cache`, `services`, `environment`, `approval`, `deploy`, `stages` and `workflows` produce errors. There is one pipeline and one static DAG per file; stages are a potential UI grouping, not an execution barrier.

Language versions must preserve old semantics for a documented support window. New optional fields require explicit parser support and conformance tests. Breaking semantics require a new language version, a migration guide and a dry-run validator; server upgrades never rewrite repository files. Before approving implementation, validate this syntax with at least one Node application, one Go application and a two-job artifact-consuming pipeline.
