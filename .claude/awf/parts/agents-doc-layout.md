Key directories:

- **`cmd/awf/`** — CLI entry point; `init`, `sync`, `check`, `list`, `add` subcommands.
- **`internal/config/`** — parses and validates `.claude/awf.yaml`; owns the config schema.
- **`internal/catalog/`** — reads `templates/catalog.yaml`; describes which skills/agents/hooks/sections are available.
- **`internal/render/`** — Go `text/template` rendering with `missingkey=zero`; applies `data`, `sections` (drop / replaceWith), and per-template part injection.
- **`internal/manifest/`** — writes and reads `.claude/awf.lock`; drives drift detection for `awf check`.
- **`internal/project/`** — orchestrates config + catalog + render + manifest into `Sync()` and `Check()`; golden tests live here.
- **`internal/adrtools/`** — regenerates `docs/decisions/ACTIVE.md` from ADR frontmatter; run via `go test ./internal/adrtools/`.
- **`templates/`** — embedded skill, agent, hook, and agents-doc templates; catalog lives at `templates/catalog.yaml`.
- **`docs/decisions/`** — ADRs for this repository; `ACTIVE.md` is auto-generated; `README.md` is the human index.
- **`docs/plans/`** — implementation plans written by `awf-writing-plans`.
- **`.claude/skills/awf-*/`** — rendered skill files (committed; do not hand-edit).
- **`.claude/agents/`** — rendered agent files (committed; do not hand-edit).
- **`.githooks/`** — rendered pre-commit and pre-push hooks (committed; run `awf setup` — or `./x setup` in this repo — to activate via `core.hooksPath`).
