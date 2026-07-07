## Command runner

`./x` at the repo root is the entry point for every repo task; run it with no argument
for the usage line. awf subcommands run from source (`go run ./cmd/awf`) so the
dogfooded render always matches the tree, never a stale installed binary.

| Command | What it does |
|---|---|
| `./x gate` | The pre-commit gate: profiled tests, the 100% statement-coverage floor (`cmd/covercheck`), `go vet`, `golangci-lint`, and the whole-program dead-code check (`cmd/deadcodecheck`). `./x gate full` runs the identical steps — the argument exists only for pre-push hook compatibility. |
| `./x test [args]` | `go test ./...`, passing extra args through. |
| `./x lint` / `./x fmt` | `golangci-lint run` / `golangci-lint fmt`. |
| `./x deadcode` | The dead-code check on its own (ADR-0063). |
| `./x sync` / `./x check` / `./x invariants` / `./x audit` / `./x commit-gate` / `./x new` | The matching `awf` subcommand, run from source. |
| `./x mutants [pkg]` | Advisory mutation triage (ADR-0066): the production diff vs `main` by default, or one package with a path argument. Never part of the gate. |
| `./x audit-local [range]` | Repo-local changelog-conformance audit (ADR-0073) via `cmd/repoaudit`: over `<base>..<head>` (default `origin/main..HEAD`) it flags an adopter-facing change with no CHANGELOG `[Unreleased]` entry. Repo-specific, not a rule in the shipped `awf audit`; never gated. |
| `./x build` / `./x install` | `go build -o awf ./cmd/awf` / `go install ./cmd/awf`. |
