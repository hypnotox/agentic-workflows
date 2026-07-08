---
status: Implemented
date: 2026-06-27
supersedes: []
superseded_by: ""
tags: [config, tooling, adoption, scaffold]
related: [4, 11]
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
   (`adr-lifecycle` is core because ADR status flips happen mid-chain — `proposing-adr`,
   `executing-plans`/`subagent-driven-development`, and the review skills all drive lifecycle
   transitions — so a fresh adopter running the chain needs it, even though AGENTS.md frames it
   among the as-needed task skills.)

2. **Scaffold only core targets.** `ScaffoldConfig` enables exactly the catalog's core skills and
   emits a `docs:` section listing the core docs. Agents and hooks remain enabled wholesale — all
   three review agents back core review skills and both hooks enforce the green-gate invariant, so
   every agent and hook is workflow-essential. Non-core skills/docs are omitted from the generated
   config.

3. **Keep opt-in friction-free.** The enable mechanism is unchanged: an adopter opts a non-core
   target in by adding its name to the relevant config array (`awf add <skill>` for skills). To make
   that safe, `ScaffoldConfig` continues to seed **every** template-referenced var — not only those
   used by core targets — so a later `awf add` of an opt-in skill renders without a `<no value>`
   failure. Because docs now appear in the scaffold output, `ScaffoldConfig`'s var-collection
   (`collectVars`) is extended to also walk the catalog's doc templates, not just its
   skill/agent/hook templates. Doc templates reference no vars today, but the extension keeps the
   seeding genuinely complete if one ever does.

4. **`core` governs only the init default, never availability.** The full catalog stays renderable
   and listable; `awf list` continues to show every skill with its enabled/available state. `core`
   is consulted solely by `ScaffoldConfig`. (`awf list` and `awf add` operate on skills only; a
   non-core doc is opted in by adding its name to the `docs:` array directly, as docs have been
   since ADR-0004.)

## Invariants

- `inv: scaffold-core-only` — the config `ScaffoldConfig` generates enables exactly the catalog's
  core skills and core docs (plus all agents and all hooks), and no non-core skill or doc.
- `inv: scaffold-seeds-all-vars` — `ScaffoldConfig` seeds every var referenced by any catalog
  skill, agent, hook, or doc template, independent of whether its target is core, so opt-in
  additions render cleanly. The backing test derives its expected var set directly from those
  template families (rather than mirroring `collectVars`'s inputs), so it fails if a future
  doc/skill var is ever left unseeded.

## Consequences

- A fresh `awf init` yields a focused, immediately-coherent workflow setup (the brainstorm → plan →
  ADR → implement → review chain and its review agents) instead of the full fifteen-skill surface.
  Adopters grow the set deliberately with `awf add` / config edits.
- The change is confined to `internal/catalog` (schema) and `internal/project/scaffold.go`
  (filtering + a new `docs:` emit). No render-pipeline, lock, or config-tree-layout change.
- No config-tree schema bump: `core` lives in the embedded `templates/catalog.yaml` (parsed by
  `catalog.Load`, a non-strict decode that accepts the new field without change), not in the
  adopter's `.awf/config.yaml` schema. `migrate.Current()` is unchanged, so existing adopters need
  no `awf upgrade` and the schema-version gate in `awf check` is unaffected. Adopters who already
  ran the enable-all `init` keep their fully-enabled config; only newly-scaffolded configs differ.
- The `core` flag is added to skills and docs only, not to agents or hooks: every current agent
  backs a core review skill and both hooks enforce the green-gate invariant, so there is no
  non-essential agent or hook to exclude and a flag on them would be unused mechanism. If a
  non-essential agent or hook is ever added to the catalog, extend the `core` marker to
  `TargetSpec` and the hook list then; until that day `scaffold-core-only` encodes "all agents,
  all hooks" directly.
- awf's own committed `.awf/config.yaml` is unaffected: it was hand-curated to opt into the full set
  it dogfoods, and `core` does not regenerate it. Only newly-scaffolded adopter configs change.
- `TestScaffoldEnablesAllCatalogSkills` inverts to assert the core skill set; the agent and hook
  tests (`...Agents`/`...Hooks`) are unchanged and still assert the full set; new assertions cover
  the emitted `docs:` section. The vars-seeding behaviour is extended to doc templates (see
  Decision item 3) and stays covered.
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
