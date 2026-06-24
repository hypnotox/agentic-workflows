Conventions every agent must respect:

- **TDD first.** Write a failing test before the implementation change. Use `go test -run TestX -v ./...` to confirm the test fails for the right reason before fixing.
- **Gate before every commit.** Run `./x gate` (≈15s; runs tests, vet, and golangci-lint). Never commit with a failing gate; never skip with `--no-verify`.
- **Publication-safe templates.** All templates render with `missingkey=zero`. A template must never produce a no-value token when a var is an empty string — wrap optional content in a conditional block. Run `awf check` after any sync to verify.
- **`awf check` is the drift oracle.** After editing `.claude/awf.yaml` or any part file, run `./x sync && ./x check`. A clean check means on-disk rendered files match the lock. Commit rendered files alongside config changes.
- **ADRs live under `docs/decisions/`.** Follow the template at `docs/decisions/template.md`. Regenerate `docs/decisions/ACTIVE.md` via `go test ./internal/adrtools/` after any ADR status change. Never hand-edit `ACTIVE.md`.
- **Plans live under `docs/the skills framework/plans/`.** Use `awf-writing-plans` to write them; `awf-reviewing-plan` to review them before execution.
- **Conventional Commits with `awf` scope** for tool and workflow changes: `feat(awf):`, `fix(awf):`, `docs(adr):`, `refactor(awf):`. Subject lines ≤72 chars, imperative mood.
- **One concern per commit.** No incidental refactors riding alongside a feature. No `git add -A` — stage files explicitly.
- **Do not hand-edit rendered files.** Edit `.claude/awf.yaml` or part files under `.claude/awf/parts/`, then re-sync. Rendered files are committed as generated artifacts.
- **`.the skills framework/` files are never committed.** They are session-local tooling artifacts.
