---
status: Proposed
date: 2026-06-27
supersedes: []
superseded_by: ""
tags: [config, tooling, adoption, scaffold]
related: [0004, 0011]
domains: [config, tooling]
---
# ADR-0022: Curated Init Default — Workflow-Core Targets

## Context

`project.ScaffoldConfig` (internal/project/scaffold.go) generates the `.awf/config.yaml` that
`awf init` writes. Today it enables **every** catalog skill, **every** agent, and **every** hook,
and emits no `docs:` section at all. A fresh adopter therefore inherits all fifteen skills, all
review agents, and both hooks on the first run, and must *subtract* the pieces they do not want by
hand-editing the enable arrays.

Docs already follow the opposite, opt-in shape: ADR-0004 established that docs are not scaffolded
and are added deliberately. The skill set never inherited that discipline, so adoption starts
maximally invasive — the audit ahead of the first real adoption flagged "init enables the entire
catalog" as the single largest first-run friction. The catalog is the full standard awf *offers*;
it should not also be the default an adopter is forced to start from.

The grounding-check confirmed: only `roadmap-graduation` is doc-gated (`requiresDoc: roadmap`) and it
is an opt-in skill, so no workflow-core skill depends on an opt-in doc; the `workflow-ref-fallback`
and `layout-docs-enabled-only` invariants remain sound when the `workflow` doc is enabled by
default; and `ScaffoldConfig` already seeds vars from every template, so opt-in additions render
without a `<no value>` failure.

## Decision

1. **Introduce a `core` marker in the catalog.** Add a `Core bool` field (`yaml:"core"`) to
   `catalog.SkillSpec` and `catalog.DocSpec`. In `templates/catalog.yaml`, mark `core: true` on the
   ten workflow-chain skills — `brainstorming`, `writing-plans`, `reviewing-plan`,
   `reviewing-plan-resync`, `proposing-adr`, `reviewing-adr`, `adr-lifecycle`, `executing-plans`,
   `subagent-driven-development`, `reviewing-impl` — and on the three workflow docs — `workflow`,
   `doc-standard`, `agents-md-standard`. All other skills and docs are non-core (opt-in).

2. **Scaffold only core targets.** `ScaffoldConfig` enables exactly the catalog's core skills and
   emits a `docs:` section listing the core docs. Agents and hooks remain enabled wholesale — all
   three review agents back core review skills and both hooks enforce the green-gate invariant, so
   every agent and hook is workflow-essential. Non-core skills/docs are omitted from the generated
   config.

3. **Keep opt-in friction-free.** The enable mechanism is unchanged: an adopter opts a non-core
   target in by adding its name to the relevant config array (`awf add <skill>` for skills). To make
   that safe, `ScaffoldConfig` continues to seed **every** template-referenced var — not only those
   used by core targets — so a later `awf add` of an opt-in skill renders without a `<no value>`
   failure.

4. **`core` governs only the init default, never availability.** The full catalog stays renderable
   and listable; `awf list` continues to show every skill with its enabled/available state. `core`
   is consulted solely by `ScaffoldConfig`.

## Invariants

- `inv: scaffold-core-only` — the config `ScaffoldConfig` generates enables exactly the catalog's
  core skills and core docs (plus all agents and all hooks), and no non-core skill or doc.
- `inv: scaffold-seeds-all-vars` — `ScaffoldConfig` seeds every var referenced by any catalog
  template, independent of whether its target is core, so opt-in additions render cleanly.

## Consequences

- A fresh `awf init` yields a focused, immediately-coherent workflow setup (the brainstorm → plan →
  ADR → implement → review chain and its review agents) instead of the full fifteen-skill surface.
  Adopters grow the set deliberately with `awf add` / config edits.
- The change is confined to `internal/catalog` (schema) and `internal/project/scaffold.go`
  (filtering + a new `docs:` emit). No render-pipeline, lock, or config-tree-layout change.
- awf's own committed `.awf/config.yaml` is unaffected: it was hand-curated to opt into the full set
  it dogfoods, and `core` does not regenerate it. Only newly-scaffolded adopter configs change.
- The scaffold tests that assert "enables all skills/agents/hooks" invert to assert the core set;
  new assertions cover the emitted `docs:` section. The vars-seeding behaviour is unchanged and
  stays covered.
- Enabling the `workflow` doc by default makes `.layout.workflowRef` resolve to `docs/workflow.md`
  rather than the AGENTS.md fallback for fresh adopters; both are valid targets, and the
  `workflow-ref-fallback` invariant still covers the doc-absent case.

Doc-currency obligations the implementing commit(s) must satisfy:

- The `config` and `tooling` domain narratives gain the curated-default behaviour of `init`.
- The status flip to `Implemented` regenerates `docs/decisions/ACTIVE.md` via `./x sync`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep enable-all (status quo) | Forces every adopter to subtract; the audit named it the top first-run friction. |
| Interactive per-skill prompts at init | Breaks non-interactive/scriptable init; a fixed core set is simpler and deterministic. |
| Multiple named profiles (minimal/standard/full) | Over-engineered for a pre-1.0 tool; one core set plus opt-in covers the need, and `--all` can be added later if demand appears. |
| Hardcode the core list in `ScaffoldConfig` | Splits the source of truth from the catalog; a `core` field keeps the declaration next to each target's other metadata. |
