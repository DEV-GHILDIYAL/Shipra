# Development environment and repository design

Status: plan only. None of the commands, binaries, scripts, containers or source directories below has been implemented. Current repository contents are README and architecture documents.

The user confirmed Windows-friendly development and Linux/Docker execution. Proposed supported path: Windows editor/browser plus WSL2 Linux checkout/runtime and a Linux Docker engine. Native PowerShell commands may drive the tools, but do not imply Windows job execution. Verify WSL2 networking, filesystem permissions, path handling and resource limits on the user's machine before choosing exact setup instructions. Remote Linux runners require outbound HTTPS and should live outside the developer's control-plane host.

## Planned repository tree

```text
shipra/
  README.md
  docs/                  # architecture, operations and ADRs
    adr/
  cmd/
    shipra-server/       # composition root: API + workers + embedded web
    shipra-runner/       # independent agent composition root
    shipra/              # CLI client
  internal/
    identity/ policy/ projects/ scm/ webhooks/
    pipeline/ execution/ scheduler/ runners/
    secrets/ artifacts/ logs/ audit/ outbox/
    storage/ transport/ telemetry/
  api/
    openapi/             # public and runner API specifications
    schema/              # pipeline JSON Schema and compatibility fixtures
  web/                   # TypeScript dashboard, generated API client
  migrations/            # ordered reviewed PostgreSQL migrations
  deployments/
    compose/             # minimal dev and personal deployment profiles
    examples/            # TLS/config/monitoring examples without secrets
  scripts/               # reproducible dev, verification, backup tooling
  tests/
    fixtures/ integration/ security/ recovery/ e2e/ load/
  go.mod
  go.sum
```

Use one Go module initially; do not create a `services/` directory suggesting independent deployment where none exists. Keep internal modules private until a stable public SDK exists. The web client only consumes API contracts. Separate executor adapters from orchestration to test scheduling without starting arbitrary processes. Introduce deployment/plugin packages only in their approved phases, not as empty scaffolding now.

## Reproducible environment contract

The future quickstart should require Git, a supported Go toolchain, a pinned Node LTS for frontend work, and Docker/Compose. Publish a tested version matrix rather than assume tools installed on this workstation. Runtime releases need only server/runner binaries or images, PostgreSQL, blob storage and Docker on runner hosts; the Node build toolchain must not be required for ordinary operators.

One documented future dev command starts PostgreSQL plus server and a deliberately marked development runner profile. It waits for readiness, applies reviewed migrations, creates non-sensitive fixture metadata and explains bootstrap enrollment. It must not silently expose a Docker socket to jobs, change firewall settings, install background services, bind all interfaces, or download unpinned installers. No working quickstart command is claimed in Phase 0.

Control-plane/DB/blob volumes are persistent and named. Temporary test volumes use unique test prefixes and are cleaned by ownership labels; never delete a developer's ordinary data. A reset command must name its target and show what will be lost. Isolated test instances get a fresh database and blob root. No tests share a production Docker daemon or credentials.

## Configuration and migrations

Use a documented server configuration file plus explicit environment overrides and secret file references. Validate at startup: unknown keys, missing durable storage, invalid TLS/auth settings and unsafe non-loopback configuration fail with actionable errors. Settings cover listen/base URL, DB DSN reference, blob root/quota/retention, root key file, logging, provider installation, runner pools and size/time caps. Print a redacted configuration summary, never DSNs/passwords.

Bootstrap creates the first admin using a local explicit flow with a single-use short-lived secret. No default admin password or sample production key. Seed data includes projects/fake runs only; a fixture runner is not a fake success implementation. TLS is mandatory for remote access. Provider credentials use secret storage or restricted bootstrap files, never the tracked `.env` template.

Migrations run as an explicit admin operation with a singleton lock and schema compatibility check. Back up before destructive changes. Prefer expand/migrate/contract for changes spanning releases; binary downgrade is allowed only when schema compatible, otherwise restore the coordinated backup. Test upgrades from every supported prior release. Root key material is backed up separately with restricted access.

## Verification and CI for Shipra

Plan formatting (`gofmt`, frontend formatter), Go static analysis/vet, TypeScript strict checking/linting, unit/property/fuzz corpus tests, DB integration tests, API compatibility and build packaging. Pin dependency lockfiles, use a dependency vulnerability scan, produce an SBOM and verify source-to-release provenance. Exact tools are selected in P1 with supported versions and license review.

Initially run CI on an independent provider (proposed GitHub Actions) with no privileged runner available to untrusted PRs. Separate public code checks from trusted release workflows and keep release credentials out of test jobs. Do not make Shipra self-hosting a prerequisite for fixing Shipra. Later dogfood representative pipelines with a documented independent rescue build path.

Build release binaries/images from clean source, sign or attest through a maintained standard toolchain when the release policy is introduced, and verify archive/container digests. Developer convenience flags must not appear in the recommended deployment configuration. Each phase updates the quickstart and its verification record.
