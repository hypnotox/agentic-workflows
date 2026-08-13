Run `./x` at the repository root with no argument for usage. Use rendered `./awf` for awf verbs: it runs source through `.awf/runner/parts/runner-body.md`, not a stale installed binary.

| Command | Purpose |
|---|---|
| `./x gate [timings]` | Pre-commit gate: staged documentation-only transactions skip both test lanes; Pi-only changes run Pi smoke only; Go-only changes run Go tests and coverage only; overlapping or uncertain changes run both. Vet, builds, lint, dead code, and pin checks always run. `timings` reports only executed stages. Prose and memory scans are hook/CI checks, not gate steps. |
| `./x test [args]` | `go test ./...`, passing arguments through, without Docker. It names `./x pi-test run` and `./x gate` for the skipped lane and complete transaction. |
| `./x pi-test run\|reset` | Run Pi tests in a throwaway Docker container, or remove lane images. The dependency-keyed image is shared; each run leaves no container or volume. |
| `./x clean-test-tmp [--all]` | Remove managed Linux/macOS test homes older than 24 hours, or all homes after warning. Partial cleanup exits nonzero. |
| `./x lint` / `./x fmt` | Run `golangci-lint run` / `golangci-lint fmt`. |
| `./x deadcode` | Run the dead-code check (ADR-0063). |
| `./x mutants [pkg]` | Advisory mutation triage against `main` or one package (ADR-0066). |
| `./x audit-local <range>` | Advisory repository conformance audit via `cmd/repoaudit`; requires `<base>..<head>` and reports missing `[Unreleased]` entries and changed production `coverage-ignore` directives (ADR-0073). |
| `./awf check commit-policy <revision-or-range>...` | Preview author, committer, and SSH-signature provenance. |
| `./x build` / `./x install` | Build `bin/awf` / install `./cmd/awf`. |
| `./awf effort ...` | Create, list, show, or finish schema-2 efforts; worktree commands remain separate and stateless. No standalone memory, lifecycle repair, manual integration, or force-discard command exists. |
