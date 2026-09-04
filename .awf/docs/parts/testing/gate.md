`./x gate` is the fast commit tier: version validation, one native build, blocking lint including govet, and workflow pin validation. Use focused tests while editing and `./x test-affected` for fail-closed behavioral feedback. It selects changed owners, production reverse dependents, test-only importers, and a small global smoke package set. Shared or uncertain inputs widen or refuse visibly.

Hosted pull-request CI always runs complete Linux `go` behavior. It consumes one typed JSON v2 selection to run applicable `platform-sensitive`, `release-archive`, and `render-template` behavior without duplicating selection policy. These four identifiers are the complete CI lane inventory. Hosted `main` runs complete source assurance on native Linux/amd64 and Darwin/arm64 targets. The aggregate `CI / gate` job is the definitive repository verdict. Production candidate construction and native archive lifecycle smoke stay in the release workflow.

The authoritative hosted Linux/amd64 lane on the GitHub-hosted `ubuntu-latest` runner invokes `AWF_FULL_LINUX_CEILING=4m ./x test-full-linux budget --artifact "$RUNNER_TEMP/awf-full-linux-timing-v1.json"`. The approved warning ceiling derives from exact push run `33335081550`, whose five sequential all-job attempts measured `168637`, `175509`, `175600`, `170549`, and `163904` milliseconds. Nearest-rank p95 is `175600` milliseconds; adding 25 percent gives `219500` milliseconds, which rounds up to the next whole minute, `240000` milliseconds or `4m`. Exceeding `4m` records a nonfailing warning in the uploaded timing JSON artifact, while three times the ceiling enforces the `12m` hard timeout. Recalibration requires five sequential all-job attempts for one exact pushed revision on the same hosted runner class, preservation of their timing artifacts, and the same nearest-rank p95, 25 percent margin, and whole-minute round-up method before changing the ceiling.

| Command | Purpose |
|---|---|
| `./x gate [timings]` | Fast static commit feedback. |
| `./x test [args]` | Complete Go behavior. |
| `./x test-affected [--staged\|--range <base>..<head>]` | Report and run focused affected-package feedback. |
| `./x lint` | Blocking then advisory lint. |
| `./x deadcode` | Optional whole-program dead-code analysis. |

Coverage percentages may be reported by external services but are informational. Tests are retained for behavioral and safety value rather than exact source-line representation.
