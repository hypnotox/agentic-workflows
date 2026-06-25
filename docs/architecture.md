# Architecture

## Overview

awf ties a per-project `.claude/awf/` config tree to an embedded template catalog, renders the
standard's skills, agents, hooks, docs, and agent guide, and drift-checks the rendered output
against a lock file. awf is both the tool that publishes the standard and its own first adopter,
so this repo's `.claude/` is rendered by the same engine it ships.

The config tree (ADR-0009) lives under a single `.claude/awf/` root:

- **`config.yaml`** — the skeleton: `prefix`, `vars`, `invariants`, `docsDir`, and flat enable
  arrays (`skills`, `agents`, `docs`, `hooks` — a name's presence enables that target).
- **`<kind>/<target>.yaml`** — optional per-target sidecars holding a target's structured `data`,
  its `sections` overrides (`drop` / `replaceWith`), and its `local` flag.
- **`<kind>/parts/<target>/<section>.md`** — convention parts: if present, the file replaces that
  section's body, no `replaceWith` pointer needed. Per-section precedence is
  `drop > explicit replaceWith > convention part > template default`.
- **`agents-doc.yaml`** + **`parts/agents-doc/<section>.md`** — the always-on agent-guide singleton.
- **`awf.lock`** — the relocated, schema-versioned lock; each entry's `ConfigHash` is a per-target
  projection over exactly that file's inputs, so a sidecar or part edit reflags only that target.


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


## Data flow

A `sync` loads the config tree, resolves each enabled target's sections (sidecar overrides and
convention parts layered over template defaults, precedence
`drop > explicit replaceWith > convention part > template default`), executes `text/template`
under `missingkey=zero`, rejects output carrying an unresolved-variable placeholder, writes the rendered files, and stamps
each one's per-target `ConfigHash` into `.claude/awf/awf.lock`. A `check` re-renders in memory and
compares against the lock — reporting drift, orphaned sidecars/parts, and stale `ACTIVE.md` — while
a stale schema generation hard-fails with a "run `awf upgrade`" gate; `awf upgrade` runs the
registered migrations up to current and re-syncs.


## Key dependencies

- **`gopkg.in/yaml.v3`** — strict (`KnownFields`) parsing of the config tree and ADR frontmatter;
  unknown keys fail fast rather than rendering silently wrong output.
- **`text/template`** (standard library) — the rendering engine, always executed with
  `missingkey=zero` so an unset optional var collapses to empty instead of leaking a token.
- **`golangci-lint`** — pinned as a `go tool` dependency and run by the gate (`./x gate`); this
  repo only, not part of the rendered standard.

