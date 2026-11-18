# Phase 0 architecture review

The owner requested development to start on 2026-11-18. This authorizes the proposed initial direction: a Go control plane, PostgreSQL, local blobs and separate Linux Docker runners for owned repositories. It does not establish that implementation or security acceptance tests have passed.

## Approved working boundary

- One workspace and trusted owned repositories in v0.1.
- A strict `shipra.yml` graph; no Jenkinsfile execution or arbitrary plugins.
- Authentication, authorization and audit before any real job execution.
- Durable PostgreSQL state and runner-issued outbound requests; no build commands on the control plane.
- Phase gates and GitHub checkpoints in [the roadmap](roadmap.md).

## Decisions before dependent implementation

GitHub is the first proposed provider, but installation mode and repository policy require review before P5. Deployment target selection is required before P9. Capacity and security claims require measured evidence, never an architecture assertion. See [risks](risks.md) and [decision index](architecture-decisions.md).
