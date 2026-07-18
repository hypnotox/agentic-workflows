## Command runner

`./x` at the repo root is the entry point for every repo task; run it with no argument
for the usage line. awf subcommands run from source (`go run ./cmd/awf`) so the
dogfooded render always matches the tree, never a stale installed binary.

| Command | What it does |
|---|---|
| `./x gate` | The pre-commit gate: profiled Go tests, the 100% statement-coverage floor (`cmd/covercheck`), containerized Pi-extension type checks and 100% line/function/branch coverage, `go vet`, `golangci-lint`, the whole-program dead-code check (`cmd/deadcodecheck`), the workflow-pin check (`cmd/pincheck`, ADR-0079), and the plain-punctuation scan (`awf prose-gate`, ADR-0119, opt-in for adopters and enabled here). `./x gate full` runs the identical steps; the argument exists only for pre-push hook compatibility. |
| `./x test [args]` | `go test ./...`, passing extra args through. |
| `./x lint` / `./x fmt` | `golangci-lint run` / `golangci-lint fmt`. |
| `./x deadcode` | The dead-code check on its own (ADR-0063). |
| `./x pi-test run|stop|reset` | Run the Pi-extension tests in the persistent Docker container, stop it while retaining cached dependencies, or remove the repo-keyed container, volume, and image. Ordinary gate runs reuse the live container and snapshot current source inside it. |
| `./x sync` / `./x check` / `./x invariants` / `./x audit` / `./x context` / `./x commit-gate` / `./x prose-gate` / `./x new` | The matching `awf` subcommand, run from source. `sync` additionally re-renders the example adopter `examples/sundial` with a source-built binary; `check` additionally gates it: drift, invariants, zero advisory notes, and its module's `go test ./...` (ADR-0090). |
| `./x mutants [pkg]` | Advisory mutation triage (ADR-0066): the production diff vs `main` by default, or one package with a path argument. Never part of the gate. |
| `./x audit-local [range]` | Repo-local conformance audit (ADR-0073) via `cmd/repoaudit`: over `<base>..<head>` (default `origin/main..HEAD`), judged from the range's merge base (a moved base neither blames upstream files nor masks a missing entry), it flags an adopter-facing change with no CHANGELOG `[Unreleased]` entry (Error) and each added-or-touched `coverage-ignore` directive in a production Go file (Warning: re-evaluate the reachability claim). Repo-specific, not rules in the shipped `awf audit`; never gated. |
| `./x build` / `./x install` | `go build -o awf ./cmd/awf` / `go install ./cmd/awf`. |
