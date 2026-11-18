# Extension and integration design

Status: proposed future architecture, 2026-09-29. No plugin SDK, marketplace, dynamic loading, or third-party plugin installation is part of v0.1. See [ADR-006](adr/ADR-006-plugin-model.md) and the [threat model](threat-model.md).

## Initial boundary

The modular control plane has narrow interfaces for Git providers, secret providers, artifact storage, and notifications. v0.1 implements only the providers required by its approved scope as reviewed, compiled-in adapters shipped with Shipra. These are trusted application code with normal release review, not a runtime plugin sandbox. A module interface is not itself a public stable SDK.

Pipeline steps are declarative built-ins and shell commands within the job container. An arbitrary downloaded “action” or executable is not a privileged Shipra extension. No repository-specified plugin can install itself, alter scheduler state, receive global credentials, run on the control-plane host, or bypass the executor's isolation profile.

Keep policy evaluation, scheduling, run state transitions, identity, audit, and secret authorization in the core. Extensions can implement bounded provider operations; they cannot redefine the security decisions governing those operations.

## Proposed extension classes

| Extension point | Capability examples | Boundary and restrictions |
|---|---|---|
| Source control | Fetch a file/commit, verify repository/ref, post a status | One configured provider connection; fixed destination policy; no global repository enumeration |
| Notifications | Send a run summary to an approved destination | Sanitized event envelope, destination grant, no default source/log/secret access |
| Artifact storage | Put/get/delete a named object under an assigned prefix | Per-workspace/project prefix and size/expiry policy; no arbitrary filesystem path |
| Secret provider | Resolve a specific secret reference under policy | High-trust extension class; no list-all grant by default; values only on protected runtime delivery path |
| Authentication | Validate a provider assertion and return claims | Cannot directly choose Shipra roles or bypass local admission policy; mapping remains administrator-controlled |
| Cloud/deployment target | Plan/apply/status/cancel a target-specific operation | Future deployment approval and environment identity required; no arbitrary control-plane shell |

These are candidate interfaces, not guaranteed v0.1 API contracts. Stabilize them only after at least two representative implementations demonstrate that the abstraction is useful.

## Process and protocol model

Future third-party plugins run as supervised isolated processes and communicate through versioned RPC. Prefer a local Unix socket with explicit peer/session authentication for Linux-local plugins; use authenticated TLS for any future remote plugin transport. Select the concrete RPC library and serialization at the SDK phase; typed Protobuf/gRPC is a candidate, not an additional initial dependency. HashiCorp's Vault provides a relevant example of authenticated RPC between a host and plugins. Shipra should adopt the boundary principle without copying Vault's complete lifecycle. [Vault plugin architecture](https://developer.hashicorp.com/vault/docs/plugins/plugin-architecture).

The supervisor launches only administrator-installed, digest-pinned packages with a manifest declaring plugin identity/version, protocol-major range, extension kinds, resource needs, and requested capabilities. Installation and enablement are separate operations. The administrator sees the exact grants before enabling; a new version cannot silently broaden them. Package signatures and provenance establish an expected publisher and bytes, not a guarantee of safety.

RPC envelopes carry plugin identity, invocation ID, workspace/project/connection scope, operation, deadline, idempotency key where applicable, and trace context. Core authorization constructs these fields; repository input cannot replace them. Plugin responses are untrusted: enforce size/schema limits, validate returned URLs/IDs, and map failures to stable errors. No direct database connection, shared in-memory core objects, arbitrary callbacks, or core transaction handle crosses this boundary.

Process separation provides crash isolation, not adequate malicious-code containment. A plugin running under the same host user could otherwise read key files, signal the core, and make arbitrary network calls. Supported third-party execution therefore also requires a distinct OS identity or hardened container/VM, minimal read-only files, a private temporary directory, denied ambient credentials, constrained egress, resource limits, and no Docker socket. If the target platform cannot enforce the required isolation, third-party plugins remain disabled there.

## Capability and credential model

Grants name operations and scope: for example “post commit status for provider connection X and repository Y” or “send notification to destination Z.” A generic `network:*`, `secrets:*`, or filesystem-root grant is not the default. The host brokers operations wherever possible, so it can enforce destination policy and audit use. Network restrictions must also exist outside the plugin process to stop direct bypass.

Secret values are not a general RPC context field. A plugin requesting a provider credential receives only the connection-scoped material required for that operation, preferably a short-lived provider token, or asks a host broker to perform the credentialed action. A secret-provider plugin necessarily enters a more privileged trust boundary and requires a separate review. It never receives the Shipra root key, all database rows, agent credentials, or unrelated secrets.

Emit audit records for install, enable/disable, package/version/grant changes, credential access, failed authorization, and privileged invocations. Denial and revocation apply to new calls immediately. Already disclosed credentials and in-flight provider side effects require their own upstream revocation/reconciliation.

## Lifecycle and compatibility

1. Install a pinned package, verify expected digest/provenance, inspect its manifest, and record it as disabled.
2. Validate protocol compatibility and perform a health/handshake check inside its intended sandbox, without production credentials.
3. Administrator grants specific capabilities and enables a version for a connection.
4. Core invokes it with deadlines and bounded concurrency. Failures produce typed errors; repeated crashes trip a circuit breaker and surface a connection health problem.
5. Upgrade by testing the new pinned version, reviewing changed grants, draining old invocations, and switching the connection. Preserve the previous package for an explicit rollback.
6. Disable/revoke, stop processes, remove temporary state, and apply retention to audit records. Uninstallation does not erase evidence or silently abandon a deployment operation.

Use independent semantic versions for the host RPC protocol and each extension contract. Reject unsupported major versions during handshake; add optional fields in compatible minor releases. Publish a host/plugin compatibility matrix and conformance suite when the SDK becomes public. Never permit a plugin to migrate Shipra's core schema. Plugin-owned state needs a bounded namespaced API and explicit migration/rollback contract before that feature is enabled.

Read-only idempotent operations may retry with bounded backoff. Non-idempotent operations require a provider idempotency key or reconciliation method; a timeout is an unknown outcome, not permission to send again. Notification delivery is at least once when the destination offers no dedupe. A plugin crash must not crash the core or cause an unapproved deployment retry.

## Alternatives and deferred work

Native in-process loading offers low overhead but couples ABI, dependencies, failure, and privileges to the core. It is rejected for arbitrary third-party code. Separate services over HTTP are appropriate for some remote integrations later, with additional deployment and authentication cost. RPC does not require every integration to become a separately deployed service initially.

WebAssembly is worth evaluating for constrained, mostly computational extensions. Wasmtime documents capability-based WASI filesystem access, but the host's granted imports, network access, resource limits, and runtime vulnerabilities still determine the effective boundary. No automatic claim that every WASM module is safe is justified. Evaluate API/toolchain fit and sandbox tests before selecting it. [Wasmtime security](https://docs.wasmtime.dev/security.html).

The SDK phase must deliver the manifest schema, protocol specification, isolated supervisor, two sample adapters, capability UI/API, compatibility tests, malicious-plugin test fixtures, packaging/signing guide, operational diagnostics, and security review. Marketplace discovery, automatic upgrades, global plugin dependencies, and unreviewed community installation remain separate scope decisions.
