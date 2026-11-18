# Risks and assumptions

| Risk or unknown | Gate |
|---|---|
| Go, PostgreSQL and Docker toolchains may be absent on the developer host | Verify supported versions and a clean build in P1 |
| Windows development and Linux runner networking/filesystem behavior differ | Test WSL2 or a separate Linux host before P4 |
| GitHub provider identity and repo/ref policy are not finalized | Review before P5 |
| Docker does not isolate hostile tenants | Keep v0.1 to trusted owned repositories; revisit in P16 |
| Database and local blob backup must be coordinated | Restore drill before P7 |
| Queue fairness, log throughput and HA are unmeasured | Publish workload and results at P7/P10 |
| Deployment target and rollback contract are unspecified | Select one staging target before P9 |
| Third-party plugins need host isolation beyond process separation | Prove sandbox and revocation before P11 |

Open assumptions are blockers only for their dependent phases. They do not justify claiming a gate passed without evidence.
