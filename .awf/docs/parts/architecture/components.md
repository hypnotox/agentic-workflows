## Components

- **`cmd/awf/`** — CLI entry point; `init`, `sync`, `check`, `list`, `add`, `setup`, `upgrade`
  subcommands. `sync`/`check` gate on the schema generation before opening the project.
- **`internal/config/`** — loads `.claude/awf/config.yaml` plus keyed sidecars; owns the config schema.
- **`internal/catalog/`** — reads `templates/catalog.yaml`; declares the available skills, agents,
  hooks, docs, and their sections.
- **`internal/render/`** — Go `text/template` rendering with `missingkey=zero`; assembles section
  overlays (sidecar overrides + convention parts) then executes the template.
- **`internal/manifest/`** — reads and writes `.claude/awf/awf.lock` (schema-versioned); drives
  drift detection for `awf check`.
- **`internal/migrate/`** — ordered schema-migration registry (ADR-0010); the `tree-layout`
  migration and the frozen legacy reader; powers `awf upgrade` and the sync/check version gate.
- **`internal/project/`** — orchestrates config + catalog + render + manifest into `Sync()` and
  `Check()`; golden tests live here.
- **`internal/frontmatter/`** — the single parser for `---`-delimited YAML frontmatter; used by
  `internal/adr` and skill/agent validation.
- **`internal/adr/`** — parses ADRs and regenerates `docs/decisions/ACTIVE.md` from their
  frontmatter; invoked by `awf sync` (`./x sync`).
- **`templates/`** — embedded skill, agent, hook, doc, and agent-guide templates; the catalog
  lives at `templates/catalog.yaml`.
