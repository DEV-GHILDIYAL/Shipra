# ADR-007: Early secret policy and vetted authenticated encryption

- Status: Proposed; architecture approval required before implementation.
- Date: 2026-09-29
- Scope: Stored build credentials, provider credentials, webhook secrets, and runtime secret delivery. Production deployment credentials are excluded from v0.1.

## Context

CI code executes arbitrary commands and may be malicious. Encryption at rest does not decide which job should receive a credential, and redaction cannot make an authorized recipient unable to disclose it. Secrets, identity, policy, and audit must therefore exist before real job execution, even though advanced vault/workload-identity integration can come later.

A new solo-operated system should avoid requiring a separate vault service solely to run its first private repository. It must also avoid inventing cryptography or placing the decryption key beside database backups.

## Decision

Use a maintained authenticated-encryption library; the proposed Go implementation is Tink's AEAD and keyset APIs, subject to release/maintenance validation before coding. Use a library-supported modern randomized AEAD configuration such as AES-256-GCM. Let the library manage key generation, ciphertext framing, and nonce creation. Do not write a custom cipher, nonce allocator, or cryptographic envelope protocol. [Tink AEAD usage](https://developers.google.com/tink/encrypt-data).

Keep an external root encryption keyset in a permissions-restricted, read-only mounted file accessible only to the control-plane service. A development environment variable may supply equivalent key material with an explicit process-environment exposure limitation. Never store it in the repository, database, ordinary configuration export, logs, or the same backup archive as ciphertext. A single workspace initially uses one root keyset with versioned keys; per-tenant key hierarchies and KMS envelope encryption are future choices.

PostgreSQL stores immutable secret versions containing ciphertext, format/key identifiers, owner/project/purpose/provider metadata, lifecycle timestamps, and policy references. AEAD associated data uses a fixed, versioned, unambiguous encoding of workspace ID, project ID, secret ID, and secret-version ID. Identifiers are data, not cryptographic secrets. Swapping a ciphertext into another identity context must fail authentication. Secret metadata can still disclose sensitive business names and requires authorized reads.

Administrator-managed project/ref policy governs runtime use. Requests bind to a connected repository, immutable commit and plan digest, trigger identity, current policy, eligible agent, and active attempt lease. Fetch only explicitly requested secrets, only after authorization and lease acceptance, and deliver only over authenticated TLS to that attempt. A denied lookup fails closed without echoing the value. Do not embed plaintext in a durable queue, plan, browser response, trace, or log spool.

Secret version IDs are pinned at run admission without decrypting plaintext; every attempt re-evaluates current policy and the pinned version's enabled/revoked status. Retries within that run retain the same versions. Rotation applies to new runs, including an explicit rerun, and never replaces a value halfway through execution. Revocation blocks new delivery and requests cancellation; revoke the actual upstream credential to invalidate already disclosed material. Missing decryption keys fail closed. Credential-use audit records contain identifiers and versions, never values.

## Rotation and recovery contract

1. Create new library-generated key material, retain the old key for decryption, back up the updated keyset separately, and verify recovery access before activation.
2. Set the new primary key for all new ciphertext writes. Record an audited rotation generation and versioned format; old ciphertext remains readable during migration.
3. Re-encrypt stored secret versions in resumable bounded batches using the library. Verify authentication under the expected associated data and compare operation counts; make progress idempotent across restart. Never log plaintext during migration.
4. Confirm all live ciphertext uses retained keys and perform a restore drill with representative retained backups. Retain old decryption keys until every backup/object version that needs them has expired or been securely re-encrypted.
5. Retire a key only after explicit recorded validation of those conditions. Keep key inventory and restore instructions with restricted operational records.

Root-key backup is independently encrypted/protected and held by the instance owner outside normal data backups. Losing all copies makes the data unrecoverable by design. On root-key compromise, replacing the at-rest key does not revoke stolen upstream passwords/tokens: rotate those credentials as well. Restore tests must cover missing/wrong key, damaged ciphertext, interrupted rotation, and restoration of an older database snapshot with its corresponding retained keyset. The lifecycle and recovery requirement follows [OWASP secrets management](https://cheatsheetseries.owasp.org/cheatsheets/Secrets_Management_Cheat_Sheet.html) and [key management](https://cheatsheetseries.owasp.org/cheatsheets/Key_Management_Cheat_Sheet.html).

The control plane can decrypt secrets and is therefore a high-trust service. A mounted key protects a database-only theft scenario; it does not defeat control-plane remote code execution, host root, memory inspection, or joint theft of backups and key material. Memory zeroization cannot be promised for all Go/runtime/library copies; minimize lifetime and disable sensitive dumps instead of claiming complete erasure.

## Alternatives

| Alternative | Benefit | Trade-off / decision |
|---|---|---|
| Plaintext environment variables/database columns | Easy initial development | Rejected: uncontrolled persistence and weak rotation/audit |
| Custom encryption using low-level primitives | Fewer dependencies | Rejected: unnecessary cryptographic design and nonce/key-management risk |
| Vetted AEAD library plus external keyset | Manageable local deployment; authenticated ciphertext | Selected: key backup/rotation remains operator responsibility; core can decrypt |
| Vault or managed secret provider as mandatory dependency | Centralized policy and operational key management | Valuable later; excessive required infrastructure for initial self-hosted scope |
| Cloud KMS envelope encryption | Separates key service and improves operational controls | Future adapter; adds cloud/provider availability and bootstrap dependencies |
| Workload identity and short-lived provider tokens | Reduces static secret exposure | Preferred where available later; does not cover every initial integration |

## Consequences and acceptance

No API reveals stored plaintext secrets. Authorization is separate from cryptography; a pipeline reference never grants access. Root-key lifecycle, audit, and restore work are required before the first real-secret execution, increasing early foundation work intentionally.

Mask known values before persistent logging, with cross-chunk handling and bounded buffering, and apply a server-side second filter. Filtering is explicitly best effort: encoded, transformed, partial, generated, or deliberately exfiltrated secrets remain possible. GitHub documents that automatic redaction cannot be guaranteed for transformed secret values. [GitHub secure workflow guidance](https://docs.github.com/en/actions/reference/security/secure-use).

Accept implementation only when policy-denial tests, per-attempt isolation, ciphertext authentication/transplant tests, key rotation/restore drills, interrupted migration recovery, and log-persistence tests pass. Verify the selected library version's maintenance and API before adoption. See [security.md](../security.md) and tests A05-A07 in the [threat model](../threat-model.md).
