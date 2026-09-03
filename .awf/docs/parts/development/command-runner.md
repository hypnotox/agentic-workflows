| Command | Purpose |
|---|---|
| `./x gate [timings]` | Fast commit feedback: version, build, blocking lint, and workflow pins. |
| `./x test [args]` | Complete Go behavior. |
| `./x test-affected [--staged\|--range <base>..<head>]` | Run changed owners, reverse dependents, test importers, and smoke packages; uncertain inputs widen or refuse. |
| `./x lint [args]` | Blocking and advisory lint. |
| `./x deadcode` | Optional whole-program dead-code analysis. |
| `./x audit-local <base>..<head>` | Advisory repository conformance audit. |
