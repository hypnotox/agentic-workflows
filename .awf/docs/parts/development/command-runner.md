## Command runner

`./x` at the repo root carries the repo-local project verbs; run it with no argument
for the usage line. awf verbs go through the rendered `./awf` wrapper, which runs awf
from source (`awfInvokeCmd: go run ./cmd/awf`) so the dogfooded render always matches
the tree, never a stale installed binary.

| Command | What it does |
|---|---|
| `./x gate` | The pre-commit gate: profiled Go tests, the 100% statement-coverage floor (`cmd/covercheck`), containerized Pi-extension type checks and 100% line/function/branch coverage, `go vet`, `golangci-lint`, the whole-program dead-code check (`cmd/deadcodecheck`), the workflow-pin check (`cmd/pincheck`, ADR-0079), and the plain-punctuation scan (`awf prose-gate`, ADR-0119, opt-in for adopters and enabled here). `./x gate full` runs the identical steps; the argument exists only for pre-push hook compatibility. |
| `./x test [args]` | `go test ./...`, passing extra args through. |
| `./x lint` / `./x fmt` | `golangci-lint run` / `golangci-lint fmt`. |
| `./x deadcode` | The dead-code check on its own (ADR-0063). |
| `./x pi-test run|stop|reset` | Run the Pi-extension tests in the persistent Docker container, stop it while retaining cached dependencies, or remove the repo-keyed container, volume, and image. Ordinary gate runs reuse the live container and snapshot current source inside it. |
| `./x dashboard-awf-path` | Resolve `refs/awf/dashboard-runtime`, initializing an absent ref to `HEAD`, and print only the immutable cached launcher path to standard output. Build and initialization diagnostics go to standard error. |
| `./x dashboard-awf-advance [commit]` | Build and validate the named commit, or `HEAD` when omitted, then compare-and-swap the local runtime ref and report old commit, new commit, and launcher path. Run only after staged checks, the gate, and implementation review; existing Pi sessions keep their captured launcher. |
| `awf` commands via `./awf` | The rendered pure wrapper `./awf` forwards every awf CLI verb verbatim, running awf from source through `awfInvokeCmd: go run ./cmd/awf`. Repository-special `./x sync` additionally re-renders `examples/sundial`, and `./x check` additionally gates its drift, invariants, advisory notes, and tests. |
| `./x mutants [pkg]` | Advisory mutation triage (ADR-0066): the production diff vs `main` by default, or one package with a path argument. Never part of the gate. |
| `./x audit-local <range>` | Repo-local conformance audit (ADR-0073) via `cmd/repoaudit`: over a required `<base>..<head>`, judged from the range's merge base (a moved base neither blames upstream files nor masks a missing entry), it flags an adopter-facing change with no CHANGELOG `[Unreleased]` entry (Error) and each added-or-touched `coverage-ignore` directive in a production Go file (Warning: re-evaluate the reachability claim). Repo-specific, not rules in the shipped `awf audit`; never gated. |
| `./x build` / `./x install` | `go build -o bin/awf ./cmd/awf` / `go install ./cmd/awf`. |

The dashboard runtime cache lives under `${XDG_CACHE_HOME:-$HOME/.cache}/awf/dashboard-runtime/v1` and is immutable and content-addressed. Its local Git ref and cache are operational state, never staged outputs. The generic rendered runner deliberately omits both dashboard commands; generated Pi code discovers only an advertised command and assumes no repository source path.
