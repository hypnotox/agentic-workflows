awf ties a per-project `.claude/awf.yaml` config to an embedded template catalog, renders the
standard's skills/agents/hooks/docs/agent-guide, and drift-checks them against a lock file.

Key directories:

- **`cmd/awf/`** — CLI entry point; `init`, `sync`, `check`, `list`, `add`, `setup` subcommands.
- **`internal/config/`** — parses and validates `.claude/awf.yaml`; owns the config schema.
- **`internal/catalog/`** — reads `templates/catalog.yaml`; declares available skills/agents/hooks/docs/sections.
- **`internal/render/`** — Go `text/template` rendering with `missingkey=zero`; applies `data`, `sections` (drop / replaceWith), and per-template part injection.
- **`internal/manifest/`** — writes and reads `.claude/awf.lock`; drives drift detection for `awf check`.
- **`internal/project/`** — orchestrates config + catalog + render + manifest into `Sync()` and `Check()`; golden tests live here.
- **`internal/adrtools/`** — regenerates `docs/decisions/ACTIVE.md` from ADR frontmatter; run via `go test ./internal/adrtools/`.
- **`templates/`** — embedded skill, agent, hook, doc, and agents-doc templates; catalog lives at `templates/catalog.yaml`.
- **`docs/decisions/`** — ADRs; `ACTIVE.md` is auto-generated; `README.md` is the human index.
- **`docs/plans/`** — implementation plans written by `awf-writing-plans`.
- **`.claude/skills/awf-*/`**, **`.claude/agents/`**, **`.githooks/`** — rendered artifacts (committed; do not hand-edit; re-sync from config and parts).
