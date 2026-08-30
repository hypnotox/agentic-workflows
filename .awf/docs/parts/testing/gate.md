`./x gate` is the fast commit tier: version validation, one native build, blocking lint including govet, and workflow pin validation. Use focused tests while editing and `./x test-affected` for fail-closed behavioral feedback. It selects changed owners, production reverse dependents, test-only importers, and a small global smoke package set. Shared or uncertain inputs widen or refuse visibly.

Hosted CI runs exhaustive Go behavior, the fast gate, current-state and drift checks, strict Pi behavior, and targeted macOS safety. The aggregate `CI / gate` job is the definitive repository verdict. Release-only snapshot and archive validation stays in the release workflow.

| Command | Purpose |
|---|---|
| `./x gate [timings]` | Fast static commit feedback. |
| `./x test [args]` | Complete Go behavior without the Pi host lane. |
| `./x test-affected [--staged\|--range <base>..<head>]` | Report and run focused affected-package feedback. |
| `./x pi-test run` | Strict Pi extension behavior. |
| `./x lint` | Blocking then advisory lint. |
| `./x deadcode` | Optional whole-program dead-code analysis. |

Coverage percentages may be reported by external services but are informational. Tests are retained for behavioral and safety value rather than exact source-line representation.
