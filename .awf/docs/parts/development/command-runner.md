## Command runner

`./x` at the repo root carries the repo-local project verbs; run it with no argument
for the usage line. awf verbs go through the rendered `./awf` wrapper, which runs awf
from source (`awfInvokeCmd: go run ./cmd/awf`) so the dogfooded render always matches
the tree, never a stale installed binary.

| Command | What it does |
|---|---|
| `./x gate` | The pre-commit gate: profiled Go tests (writing the durable `coverage.out`, ADR-0196), the 100% statement-coverage floor (`cmd/covercheck`), containerized Pi-extension type checks and 100% line/function/branch coverage, `go vet`, `golangci-lint`, the whole-program dead-code check (`cmd/deadcodecheck`), and the workflow-pin check (`cmd/pincheck`, ADR-0079). The prose and memory scans are not gate steps; the pre-commit hook payload runs them locally and CI backstops them (ADR-0196). Focused development may exercise schema-2 efforts with `./awf effort new`, `list`, `show`, `finish`, and the separate stateless worktree commands; no standalone memory, lifecycle repair, manual integration, or awf force-discard command remains. `./x gate full` runs the identical steps; `full` is accepted only as a no-op legacy argument, and no rendered artifact passes it. |
| `./x test [args]` | `go test ./...`, passing extra args through. |
| `./x lint` / `./x fmt` | `golangci-lint run` / `golangci-lint fmt`. |
| `./x deadcode` | The dead-code check on its own (ADR-0063). |
| `./x pi-test run|reset` | Run the Pi-extension tests in a throwaway Docker container, or remove the lane's images along with whatever the superseded path-keyed design left behind. Each run copies the source the suite compiles into a fresh container that exits with it, so the lane leaves no container and no volume. The image is keyed by dependency content alone, so every checkout, worktree, and clone shares one. |
| `awf` commands via `./awf` | The rendered pure wrapper `./awf` forwards every awf CLI verb verbatim, running awf from source through `awfInvokeCmd: go run ./cmd/awf`. Repository-special `./x render` additionally re-renders `examples/sundial`, and `./x check` additionally gates its drift, invariants, advisory notes, and tests. |
| `./x mutants [pkg]` | Advisory mutation triage (ADR-0066): the production diff vs `main` by default, or one package with a path argument. Never part of the gate. |
| `./x audit-local <range>` | Repo-local conformance audit (ADR-0073) via `cmd/repoaudit`: over a required `<base>..<head>`, judged from the range's merge base (a moved base neither blames upstream files nor masks a missing entry), it flags an adopter-facing change with no CHANGELOG `[Unreleased]` entry (Error) and each added-or-touched `coverage-ignore` directive in a production Go file (Warning: re-evaluate the reachability claim). Repo-specific, not rules in the shipped `awf audit`; never gated. |
| `./x build` / `./x install` | `go build -o bin/awf ./cmd/awf` / `go install ./cmd/awf`. |
