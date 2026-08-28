Run `./x` at the repository root with no argument for usage. Use rendered `./awf` for awf verbs: it runs source through `.awf/runner/parts/runner-body.md`, not a stale installed binary.

| Command | Purpose |
|---|---|
| `./x gate [timings]` | Fast commit gate: version validation, one native build, blocking lint (including govet), and pin validation. |
| `./x gate full [timings] [--range <base> <head>]...` | Terminal exhaustive verification: the commit tier plus complete native Go/coverage and Pi suites, vet, advisory lint, dead code, four Linux/Darwin release cross-builds, and exact-universe-selected mutation. Local calls use the staged candidate; ranges union remote evidence. |
| `./x test [args]` | `go test ./...`, passing arguments through, without the host Pi lane. It names `./x pi-test run` and `./x gate` for the skipped lane and complete transaction. |
| `./x pi-test run` | Run Pi tests on the pinned host Node runtime. The lane lock serializes each checkout; every run uses a narrow throwaway workspace. |
| `./x clean-test-tmp [--all]` | Remove managed Linux/macOS test homes older than 24 hours, or all homes after warning. Partial cleanup exits nonzero. |
| `./x lint` / `./x fmt` | Run `golangci-lint run` / `golangci-lint fmt`. |
| `./x deadcode` | Run the dead-code check (ADR-0063). |
| `./x mutants [pkg]` | Advisory mutation triage against `main` or one package (ADR-0066). |
| `./x covercheck-mutants [--select-staged\|--select-range <base> <head>]` | Run the pinned, hermetic `cmd/covercheck` mutation blocker. Selection skips only a proven non-owned change set and runs on uncertainty. |
| `./x audit-local <range>` | Advisory repository conformance audit via `cmd/repoaudit`; requires `<base>..<head>` and reports missing `[Unreleased]` entries, changed production `coverage-ignore` directives, and added or moved raw coverage-baseline identities (ADR-0073). |
| `./awf check commit-policy <revision-or-range>...` | Preview author, committer, and SSH-signature provenance. |
| `./x build` / `./x install` | Build `bin/awf` / install `./cmd/awf`. |
| `./awf effort ...` | Create, list, show, or finish schema-2 efforts; worktree commands remain separate and stateless. No standalone memory, lifecycle repair, manual integration, or force-discard command exists. |
