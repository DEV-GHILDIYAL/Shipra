# Security architecture

Status: proposed Phase 0 design, 2026-09-29. These are requirements for future implementation, not claims that controls already exist. See [threat model](threat-model.md), [runners](runners.md), [secret ADR](adr/ADR-007-secret-management.md), and [plugin ADR](adr/ADR-006-plugin-model.md).

## Security contract and limits

Shipra treats repository contents, pipeline configuration, shell scripts, images, dependencies, logs, artifacts, webhook payloads, and agent responses as untrusted input. The control plane never runs repository code. It authorizes work; a separate agent host executes it. Authentication, authorization, audit events, encrypted secret storage, and execution restrictions are prerequisites for the first real job, rather than features postponed until the end of the roadmap.

The v0.1 operating envelope is one workspace, trusted administrators, owned private repositories, and dedicated Linux Docker agent hosts. Public repository execution, fork pull requests, mutually distrustful tenants, production deployment credentials, and arbitrary plugins are excluded. Trusted repository ownership does not make every dependency or pipeline safe. It bounds who is permitted to submit code and who accepts residual risk.

Docker shares the host kernel. A hardened container reduces exposure but is not the promised security boundary for hostile multitenant execution. Dedicated disposable VMs or a separately evaluated stronger runtime are required before that scope expands. Docker daemon access is a privileged host capability; Docker itself advises restricting it to trusted users. Rootless mode mitigates some daemon/runtime risks but does not make malicious builds harmless. [Docker security](https://docs.docker.com/engine/security/), [rootless mode](https://docs.docker.com/engine/security/rootless/).

A job given a secret can transmit, encode, persist, or otherwise reveal it. No log filter can prevent that. A compromised runner can fabricate results and steal secrets it receives; leases and TLS cannot establish that the claimed commands ran honestly. Host compromise, compromised administrators, control-plane code execution, and combined database/root-key compromise remain high-impact risks.

## Identity and authorization

Authorization is server-side, deny by default, and applied to every API, streamed log subscription, artifact download, and agent operation. Object IDs and runner labels never confer permission. Workspace roles apply across the single workspace in v0.1; project membership isolation is a later feature. A viewer can therefore see every project's logs and artifacts in that workspace. Deploy separate instances when that visibility is unacceptable.

| Operation | Viewer | Operator | Administrator |
|---|---|---|---|
| Read projects, runs, sanitized logs, artifacts | Yes | Yes | Yes |
| Trigger or cancel a run under existing project policy | No | Yes | Yes |
| Retry eligible jobs with the original immutable plan | No | Yes | Yes |
| Configure repositories, trusted refs, limits, runner eligibility | No | No | Yes |
| Add/rotate/delete secrets and edit secret policy | No | No | Yes |
| Reveal a stored plaintext secret through UI/API | No | No | No |
| Enroll, revoke, or change grants for agents | No | No | Yes |
| Manage users, roles, tokens, instance settings; read security audit | No | No | Yes |

The no-reveal API reduces accidental exposure; it does not stop an administrator who can change a pipeline or policy from arranging secret use. Likewise, repository writers on an authorized secret-bearing ref must be treated as able to obtain those secrets.

- Local identities use a vetted password hashing implementation, with Argon2id parameters benchmarked for the supported host. No default password, public signup, or credentials checked into seed data. Bootstrap creates one administrator through a local administrative command and a one-use expiring setup credential.
- Browser authentication uses opaque server-side sessions, rotated on login and privilege change, revoked on logout or account disablement. Use host-only `Secure`, `HttpOnly`, `SameSite` cookies, CSRF tokens on mutations, origin validation, and restrictive CORS. No credentials in local storage or query strings. Proposed defaults are 30 minutes idle and 12 hours absolute expiry, configurable downward.
- CLI/API tokens are high-entropy random opaque tokens with visible identifiers, bounded scopes, expiry, and revocation. Store only a cryptographic digest of the random secret part. Their effective privileges cannot exceed the current owner's role; demotion and disablement take effect on the next request. Passwords use a slow password hash, not the token digest scheme.
- Login, recovery, token creation, enrollment, and mutation APIs have rate limits. Errors avoid account and secret enumeration. Single-instance v0.1 can use bounded local request counters; this is not a distributed rate-limiting guarantee.
- TLS is required for remote access; agent certificates validate the configured control-plane identity. Never offer a production `skip-verify` switch. Disposable loopback development exceptions must be explicit and reject remote binding.

Cookie protections supplement CSRF validation; they do not replace it. The browser design follows the [OWASP session guidance](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html) and [CSRF guidance](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html).

## Repository and webhook trust

An administrator connects a GitHub repository by stable provider repository ID and explicitly grants allowed event types, refs, and agent eligibility. A manual trigger uses the same policy as a webhook. A caller cannot supply an arbitrary clone URL, substitute another repository, override the runner security profile, or impersonate a different triggering identity.

Verify the GitHub HMAC-SHA256 signature over the bounded raw request body before parsing it. Use the provider's documented algorithm through the standard library and constant-time comparison. Persist a uniqueness key for provider connection plus delivery ID with the accepted event transaction. Signature verification proves possession of the webhook secret, not that an event is fresh or that its code is safe. Duplicate suppression and event/ref validation are separate controls. [GitHub webhook validation](https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries), [delivery headers](https://docs.github.com/en/webhooks/webhook-events-and-payloads).

Resolve source and configuration to an immutable commit SHA from the connected repository, record their digests and schema version, and execute that snapshot. Do not trust payload-supplied URLs. Fetch configuration as data, with strict byte/depth limits; never execute configuration on the control plane. Reject unknown executable features rather than treating them as shell extensions. Untrusted event values must enter scripts as data through environment variables or argument arrays, never by string substitution into command source.

The control plane fetches a bounded source archive from GitHub's authenticated archive endpoint for the exact repository and commit. It validates redirect destinations, stores the immutable bytes and digest, and never extracts or runs source on the control-plane host. Provider credentials stay on the control plane and are not forwarded to unapproved redirect hosts. The agent fetches the authorized snapshot and extracts it into a fresh managed workspace with path, symlink, file-count, expansion-size, and permission checks. No Git credential, `.git` metadata guarantee, submodules, Git hooks, or Git LFS support is provided in v0.1. Any future full Git checkout requires a separate credential and destination review.

## Agent and job isolation

The agent makes outbound authenticated HTTPS calls for registration, work leases, heartbeats, logs, artifacts, and completion. It exposes no public inbound job-control port. An administrator creates a one-use, expiring enrollment token scoped to the workspace, runner pool, and intended agent grants; only its digest is stored. Successful redemption atomically consumes it and issues a separate revocable, expiring agent credential. Replay, concurrent redemption, wrong workspace/pool, and expired token must fail. Rotation overlaps only for a short configured grace period and is audited.

The agent's credential lives in a permissions-restricted host file and is never mounted into a build. Each API request binds agent identity to its server-assigned attempt ID, lease generation/fencing token, and allowed operation. Self-reported capabilities help compatibility selection; they are not trusted authorization grants. The server controls eligible projects and resource ceilings. Revocation prevents new work and secret fetches immediately; already disclosed material cannot be recalled.

Attempt grants are high-entropy random opaque credentials whose digests and scopes are stored server-side, not self-signed JWTs. They bind agent identity/session, attempt and assignment ID, lease generation, operation, permitted object or secret version, and expiry no later than the current lease. Every use still checks live lease and revocation state. A replayed claim ID for the same active assignment may receive a fresh equivalent grant after identity/session validation; it must never create a second attempt or reopen a closed assignment. Expired/stale generations cannot obtain replacement grants. Grants are agent-only material and never enter the job environment.

Required Docker execution profile:

- Dedicated agent host with no personal credentials or production network access; do not colocate it with PostgreSQL, blob storage, or the control plane outside disposable development.
- Non-root container user, dropped Linux capabilities, `no-new-privileges`, default or stricter seccomp and AppArmor/SELinux policy, and a read-only root filesystem where the documented image contract permits it. Prefer a tested rootless Docker configuration; verify resource enforcement on supported Linux/cgroup versions.
- No privileged containers, host network/PID/IPC namespaces, device access, Docker socket, arbitrary host bind mounts, or administrator-supplied agent credentials inside jobs. The engine creates only a fresh per-attempt managed workspace volume and bounded temporary storage. No pipeline-configurable container runtime flags.
- Finite CPU, memory, process count, disk/log/artifact budgets, job timeout, and cancellation grace period. Unsupported resource controls fail admission for that profile instead of silently running without limits.
- Initially one active attempt per agent host. Parallel pipelines use multiple agents. Job steps may share their own job workspace; separate jobs do not share writable volumes. Teardown removes the job container and workspace, and failed cleanup quarantines the agent from more work.
- Agent control-plane egress is separate from job egress. Enforce network restrictions on the host/network plane, not inside a container that pipeline code controls. Jobs cannot reach the control plane, database, blob backend, agent management endpoints, loopback services on the host, cloud metadata, or unrelated private networks. Dependency traffic is restricted by an administrator-controlled egress policy/proxy; unrestricted internet egress cannot provide secret-exfiltration prevention.

The first supported Linux deployment profile must demonstrate these network rules, including Docker firewall behavior and IPv6. A configuration flag labelled “isolated” is insufficient evidence. These requirements deliberately limit images and workflows supported by v0.1. Container image building through host Docker or privileged Docker-in-Docker is excluded; a future isolated build service requires a separate design review.

If a lease is lost, an honest agent stops the container and withholds completion. Fencing rejects stale control-plane mutations. Neither can stop a malicious or partitioned machine from continuing arbitrary external side effects. v0.1 therefore promises no exactly-once shell execution and carries no production deployment engine.

## Secret storage, policy, and delivery

Use a maintained AEAD library, with the proposed Go choice being Tink, and no hand-written cipher, nonce scheme, token format cryptography, or key derivation. Persist versioned ciphertext and key metadata in PostgreSQL. Keep the root keyset outside PostgreSQL and backups, preferably in a mounted read-only key file available only to the control-plane account. A development environment variable is an explicit convenience with process-environment exposure risk. Associated data binds ciphertext to workspace, project, secret identity, and secret version. See [ADR-007](adr/ADR-007-secret-management.md) for rotation and recovery. Tink provides authenticated-encryption primitives and keyset handling; application policy still remains Shipra's responsibility. [Tink AEAD usage](https://developers.google.com/tink/encrypt-data).

Secret records carry an owner, project, purpose, provider, version, expiry if known, creation/rotation times, and administrator-managed usage policy. A pipeline may request named secrets but cannot grant itself access. v0.1 policy is a project-level allowlist of approved refs, with protected-ref verification through the configured GitHub connection. The resolved repository identity, commit SHA, configuration digest, triggering identity, and policy revision are recorded with the run. This administrator approval of policy is distinct from a future per-run human approval workflow.

At dispatch, re-evaluate current policy, user/repository/agent grants, secret status, and ref trust. Verify the immutable commit belongs to the authorized event/ref under that policy; do not replace it with the ref's current tip. If protection evidence is unavailable, revoked, or ambiguous, fail closed for secret-bearing execution. Secretless runs may proceed only if their separate execution policy permits them. Checks narrow the race window; provider administrators remain trusted and can change branch policy outside Shipra.

Resolve only the secrets explicitly requested for the attempt after a valid lease has been accepted. Delivery is over authenticated TLS to that agent and attempt; no secret values enter the queue, run plan, webhook record, ordinary API response, or trace. The agent supplies values only to the intended job through a narrow environment/file injection mechanism; prefer temporary read-only secret files when tools support them, backed by volatile storage and removed at teardown. Avoid command-line arguments. Agent memory and process inspection by a privileged host user remain exposures.

At run admission, resolve and snapshot authorized secret version IDs without decrypting values. Every attempt re-authorizes the pinned versions under current policy and audits use; retries within a run never silently switch versions. Rotated versions apply to new runs, including explicit reruns. Disabled/revoked versions or unavailable decryption keys fail closed at dispatch. Revocation blocks further retrieval and attempts, requests cancellation of affected running jobs, and requires upstream credential revocation when urgent. A fetched password cannot be made unknown to the job.

No production deployment credentials are allowed in the v0.1 supported profile. Build secrets should be read-only where possible and narrowly scoped to an individual registry/package repository. Short-lived provider-issued credentials and workload identity are future additions; long-lived credentials still require owner-driven upstream rotation. Lifecycle expectations follow [OWASP secrets management](https://cheatsheetseries.owasp.org/cheatsheets/Secrets_Management_Cheat_Sheet.html).

## Logs, artifacts, and data exposure

The trusted agent filters log chunks before sending them, and the server filters again before durable storage/streaming using that attempt's authorized mask set. Matching must handle secrets split across chunks and multiline byte sequences; overlap buffers are bounded by a documented maximum secret size. Do not persist unfiltered logs in a spool, crash dump, debug trace, or retry queue. Redaction failure blocks unsafe log persistence and records an explicit log-delivery failure. This second filter provides defense in depth for ordinary raw leakage, not proof that a malicious agent sent safe output.

Redaction is best effort. It cannot recognize arbitrary encodings, partial secrets, derived credentials, steganography, or deliberately chosen low-entropy values. Shipra makes no claim that artifacts are sanitized. GitHub documents the same fundamental limitation for transformed secret values. Preventing unauthorized jobs from receiving secrets is the primary control; access restriction and short credential lifetimes limit damage after authorized use. [GitHub secure workflow guidance](https://docs.github.com/en/actions/reference/security/secure-use).

Render logs as escaped text with restricted ANSI handling; ignore OSC/control sequences, HTML, and job-generated instructions that claim to change platform state. Job stdout is never an API channel for arbitrary secret grants, output-to-command execution, or admin actions. Raw downloads are authenticated and served as inert text/attachments with safe content types and `nosniff`.

Artifact uploads are scoped to the assigned attempt, size-limited, hashed, and finalized only after server verification. Store immutable metadata linking artifact digest to repository commit, plan digest, job attempt, and authenticated agent. A digest verifies bytes; it does not prove the build was honest. Reject path traversal, symlinks escaping the workspace, special files, and cross-attempt object writes. Never extract an untrusted archive on the control plane. Cache sharing and restored cache trust require a separate design before caching is enabled.

Retention applies to logs, artifacts, partial uploads, audit records, credentials, and backups. Authorization is checked at every read, including SSE reconnect and ranged downloads. Private blob storage has no public listing; future signed URLs require short expiry, bounded operation, and audit linkage. Deleting a visible log does not erase historical backups; exposure response must rotate the upstream credential.

## SSRF and outbound integrations

v0.1 supports fixed GitHub service endpoints and administrator-configured storage/control-plane endpoints, not arbitrary pipeline-defined callbacks or clone addresses. Restrict URL schemes and authorities, reject embedded credentials and unusual IP literals, revalidate every redirect, validate resolved IPv4/IPv6 addresses, and bind validated resolution to the connection to avoid DNS rebinding. Explicit administrator-configured private endpoints must be narrowly allowlisted; the default denies loopback, link-local, metadata, and private destinations for untrusted URLs. Network policy is an independent backstop. [OWASP SSRF prevention](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html).

Dependency downloads performed by a job are arbitrary code egress, not simply a control-plane SSRF issue. Allowlisting destination domains does not make their content trustworthy and cannot prevent exfiltration to an attacker-controlled endpoint on an approved service.

## Audit, incident response, and release controls

Write security audit events for authentication, role/policy changes, token lifecycle, repository/ref trust decisions, enrollment/revocation, secret version use, run triggers/cancellation, artifact reads, and future approvals/deployments/plugin grants. Include actor, action, target, timestamp, request/run/attempt correlation, policy revision, outcome, and reason, excluding raw credentials and request bodies. Configuration mutations and their audit record commit together. Audit table access is limited to the service's append path and administrative read API; a database administrator can still modify it. External immutable export is a later hardening feature, not a current tamper-proof claim.

An exposure playbook must: disable the affected project/agent; revoke Shipra credentials and upstream secrets; quarantine the host without reusing its workspace; preserve restricted forensic records; locate affected attempts, artifacts, and downstream consumers; replace the host; restore access only after the cause is addressed. Losing a root key makes its ciphertext unrecoverable; suspected root-key exposure requires both at-rest key rotation and upstream secret rotation.

Shipra releases must use pinned build dependencies, vulnerability scanning, SBOMs, checksums, and signed release provenance once the release pipeline exists. Verify runner release identity before upgrade; no unauthenticated self-update download. These measures identify expected software but do not prove it has no vulnerabilities. Dependency and container-image security review applies to Shipra itself as well as user pipelines.

## Release gates and open validation

Before v0.1 can execute real repositories, the [threat-model tests](threat-model.md#required-adversarial-tests) must verify authorization, replay handling, secret isolation, network containment, resource limits, fenced completion, safe artifact handling, and cleanup. The root-key backup/restore and credential-revocation drills must pass. The threat model is reviewed whenever execution scope, auth, plugins, deployment privileges, or a new provider is added.

Open validation includes rootless Docker/cgroup/network behavior on the supported Linux versions; the exact GitHub permissions needed to verify protected refs; a maintained Tink Go release and key-rotation operational procedure; and whether the initial supported package registries can be used through a constrained egress policy. These are explicit implementation acceptance questions, not grounds for weakening the supported security contract silently.
