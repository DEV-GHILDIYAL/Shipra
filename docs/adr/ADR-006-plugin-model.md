# ADR-006: Isolated RPC extensions after built-in adapters

- Status: Proposed; architecture approval required. Future extension design, not permission to implement an SDK now.
- Date: 2026-09-29
- Scope: Integration and provider extensions; not arbitrary pipeline shell commands.

## Context

Shipra needs future source-control, notification, cloud, deployment, secret, artifact, and authentication integrations. Loading third-party code inside the control plane would give that code broad memory/credential access and couple its failures and dependencies to scheduling. A solo-developer v0.1 does not need the installation, compatibility, and sandbox machinery of a public ecosystem.

## Decision

Start with narrow internal interfaces and reviewed compiled-in adapters required by v0.1. Do not add runtime third-party loading. Preserve policy, scheduling, state transitions, identity, and audit in the core.

When extensions are approved, use supervised separate processes with versioned, authenticated, bounded RPC, explicit administrator-approved capability grants, and OS-level isolation. Process isolation alone is insufficient: separate credentials/filesystem/network/resource boundaries are required. Plugins have no direct core database/root-key access. Package versions are pinned and updates cannot automatically expand grants.

The concrete RPC library and public SDK contract remain deferred until two real implementations validate the interface. Typed Protobuf/gRPC is a candidate; it is not required infrastructure for v0.1. The full proposal is in [plugins.md](../plugins.md).

## Alternatives

| Alternative | Advantage | Reason not selected initially |
|---|---|---|
| Native in-process plugins | Fast calls; familiar language objects | Shares core privileges/failure domain; ABI/dependency coupling |
| Isolated RPC process | Language-independent boundary; crash containment; explicit contract | Selected future model, but requires real OS sandboxing and lifecycle work |
| WebAssembly/WASI | Portable execution and controllable host capabilities | Evaluate later: provider SDK fit, granted imports and resource limits require validation |
| Independent network services | Strong operational separation and remote ownership | More deployments/authentication than initial integrations justify |
| Compiled-in adapters only forever | Simple to test and operate | Selected initially; insufficient for eventual third-party ecosystem |

## Consequences

The first release stays small and does not inherit a plugin supply-chain surface. Future SDK work can preserve the same logical interfaces but must version public contracts independently. RPC overhead and process supervision are acceptable for integration calls; high-volume inner scheduling loops remain core code.

An extension allowed to use a credential can misuse that credential. Signatures identify approved bytes, not benevolence. A sandbox that cannot enforce its grants is a release blocker for untrusted extensions. Plugin timeouts require reconciliation for side effects, not blind retries.

## Evidence and validation

[Vault plugin architecture](https://developer.hashicorp.com/vault/docs/plugins/plugin-architecture) demonstrates authenticated RPC between a host and plugins. [Wasmtime security](https://docs.wasmtime.dev/security.html) describes capability-based filesystem access and remaining runtime/host considerations. These are reference mechanisms; Shipra's final model is an independent proposal.

Before acceptance of the SDK implementation, demonstrate crash/timeout containment, filesystem and network denial, grant revocation, protocol mismatch rejection, package verification, compatibility tests, and idempotent/reconciled provider behavior. See tests A18 in the [threat model](../threat-model.md).
