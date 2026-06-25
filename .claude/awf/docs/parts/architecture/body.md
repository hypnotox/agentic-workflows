awf ties a per-project `.claude/awf/` config tree to an embedded template catalog, renders the
standard's skills/agents/hooks/docs/agent-guide, and drift-checks them against a lock file.

The config tree (ADR-0009) lives under a single `.claude/awf/` root:

- **`config.yaml`** — the skeleton: `prefix`, `vars`, `invariants`, `docsDir`, and flat enable
  arrays (`skills`, `agents`, `docs`, `hooks` — presence of a name enables that target).
- **`<kind>/<target>.yaml`** — optional per-target sidecars holding everything non-prose: a
  target's structured `data`, its `sections` overrides (`drop` / `replaceWith`), and its `local`
  flag. An enabled target with no sidecar renders from template defaults.
- **`<kind>/parts/<target>/<section>.md`** — convention parts: if present, the file replaces that
  section's body, no `replaceWith` pointer needed. Per-section precedence is
  `drop > explicit replaceWith > convention part > template default`.
- **`agents-doc.yaml`** + **`parts/agents-doc/<section>.md`** — the always-on agent-guide singleton.
- **`awf.lock`** — the relocated lock; each entry's `ConfigHash` is a per-target projection over
  exactly that file's inputs (skeleton fields it reads, its sidecar, its consumed parts), so a
  sidecar/part edit reflags only the affected target.

Key directories:

- **`cmd/awf/`** — CLI entry point; `init`, `sync`, `check`, `list`, `add`, `setup`, `upgrade`
  subcommands. `sync`/`check` gate on the schema generation before opening the project.
- **`internal/config/`** — loads `.claude/awf/config.yaml` plus keyed sidecars; owns the config schema.
- **`internal/catalog/`** — reads `templates/catalog.yaml`; declares available skills/agents/hooks/docs/sections.
- **`internal/render/`** — Go `text/template` rendering with `missingkey=zero`; assembles section overlays (sidecar overrides + convention parts) then executes the template.
- **`internal/manifest/`** — writes and reads `.claude/awf/awf.lock` (with a `schemaVersion`); drives drift detection for `awf check`.
- **`internal/migrate/`** — ordered schema-migration registry (ADR-0010); the `tree-layout` migration and a frozen legacy reader of `.claude/awf.yaml`; powers `awf upgrade` and the sync/check version gate.
- **`internal/project/`** — orchestrates config + catalog + render + manifest into `Sync()` and `Check()`; golden tests live here.
- **`internal/frontmatter/`** — the single parser for `---`-delimited YAML frontmatter; used by `internal/adr` and skill/agent validation.
- **`internal/adr/`** — parses ADRs and regenerates `docs/decisions/ACTIVE.md` from their frontmatter; invoked by `awf sync` (`./x sync`).
- **`templates/`** — embedded skill, agent, hook, doc, and agents-doc templates; catalog lives at `templates/catalog.yaml`.
- **`docs/decisions/`** — ADRs; `ACTIVE.md` is auto-generated; `README.md` is the human index.
- **`docs/plans/`** — implementation plans written by `awf-writing-plans`.
- **`.claude/skills/awf-*/`**, **`.claude/agents/`**, **`.githooks/`** — rendered artifacts (committed; do not hand-edit; re-sync from config and parts).

When a breaking config-format change ships, `awf upgrade` runs the registered migrations from the
project's recorded generation up to current and re-syncs; until then `awf sync`/`check` hard-fail
with a "run `awf upgrade`" gate (a no-op version gap auto-bumps on the next sync).
