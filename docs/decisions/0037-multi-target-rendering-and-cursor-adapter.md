---
status: Proposed
date: 2026-06-29
supersedes: []
retires_invariants: [target-output-paths]
superseded_by: ""
tags: [rendering, adapter, cursor, config]
related: [0014, 0016, 0018, 0024]
domains: [rendering, config, tooling]
---
# ADR-0037: Multi-Target Rendering and the Cursor Adapter

## Context

ADR-0016 built a `Target` seam (`internal/project/target.go`) so adapter artifacts (skills,
agents, the bridge file) get their output paths from a named runtime descriptor rather than
literals, and split artifacts into **neutral** (AGENTS.md, docs, domain docs, ADR infra — fixed
paths) versus **adapter** (target-placed). The seam was explicitly designed to admit a second
runtime, but `claudeTarget` remains the sole built-in: `Open` hardwires `Target: claudeTarget`
(project.go:41), `Project.Target` is a single value, and there is no user-facing way to select or
add a runtime. awf positions itself as a multi-tool agentic-workflow renderer, yet it renders only
for Claude Code.

Adding **Cursor** as a second adapter realises that positioning and, by being the first concrete
second runtime, validates how far the seam must stretch. A web check of Cursor's current (2026)
conventions settled the shape:

- Cursor **Skills** are the same `SKILL.md` open standard as Claude skills — a per-skill folder
  containing `SKILL.md` with `name`/`description` frontmatter, progressive disclosure, on-demand
  description-based activation. awf skills map to Cursor Skills with **identical body and
  frontmatter**.
- Cursor **subagents** (2.4, Jan 2026) are flat markdown files in `.cursor/agents/<name>.md`,
  matching Claude's `.claude/agents/<name>.md` shape.
- Cursor reads **AGENTS.md natively** at repo root, so it needs **no bridge file**.

Because both runtimes share the SKILL.md/AGENTS.md standards, the same rendered body and frontmatter
are written to two paths — no per-adapter frontmatter transform or per-adapter template set is
needed. The grounding check confirmed the lock is keyed by output path (project.go:138) and
`targetConfigHash` does not fold the path (confighash.go), so two entries for one skill carry
identical hashes under distinct keys and cannot collide. It also confirmed the prune in Sync
(project.go:144) already drops files no longer produced, and `PlannedOutputs`/`RenderAll` are the
single source of truth for the output set — so multi-target drift, collision-guarding, and pruning
fall out of one render-loop change rather than bespoke per-target bookkeeping.

Three frictions surfaced that this ADR must address head-on:

1. **The render loop is not cleanly split.** Skills (adapter), agents (adapter), and docs (neutral)
   share one `renderKind` driver (render.go:115), and the bridge is nested inside the AGENTS.md
   block (render.go:169). Rendering docs/AGENTS.md once while rendering skills/agents/bridge
   per-target requires actually separating the neutral and adapter passes.
2. **Pruning is shallow.** `os.Remove(filepath.Dir(file))` clears only the immediate parent
   (project.go:150), so removing a target would leave empty `.cursor/`, `.cursor/skills/` behind.
   `Uninstall` already walks all empty ancestors (install.go:96) — that pattern must be reused.
3. **A term clash.** ADR-0016's `Target` means *adapter*, but `renderTarget`/`targetConfigHash`/
   `planSections`/`partRel`/`config.PartPath` and the orphan loop use "target" to mean *the managed
   artifact's name*. With `Targets` now plural this is actively confusing; a third, unrelated sense
   (`catalog.VarDescriptor.Target` in `internal/initspec`) must be left untouched.

This ADR also carries a second, separable commitment (Decision 6): the shipped skill prose names
Claude-specific tools ("the `Agent` tool", "`subagent_type: Explore`", "the `Skill` tool",
"`AskUserQuestion`") in ~10 templates. Shipping the same body to Cursor surfaces these verbatim, so
the rendered standard should name **actions, not a runtime's tools** — an extension of the ADR-0018
documentation authoring standard. It is recorded here because it is motivated by and lands with the
Cursor adapter, but the reviewer may split it into its own ADR; nothing downstream depends on the
two staying joined.

## Decision

1. **`targets` config enable array.** Add `Targets []string` to `internal/config.Config`. When the
   `targets:` key is absent, `Load` injects `["claude"]` (mirroring the `DocsDir` default at
   config.go:94), so a pre-existing config renders byte-identical with no schema-version bump and no
   `internal/migrate` entry — the backward-safe optional-field precedent set by `Domains` (ADR-0014).
   `Validate` rejects an empty list and any name not in the known-adapter set.

2. **Plural targets and the Cursor descriptor.** `Project.Target Target` becomes
   `Project.Targets []Target`, resolved in `Open` from `Cfg.Targets` through a new name→`Target`
   registry. Add `cursorTarget{Name:"cursor", SkillDir:".cursor/skills", AgentDir:".cursor/agents",
   BridgeFile:""}`. An empty `BridgeFile` emits no bridge (the existing render.go:169 guard), so
   Cursor relies on native AGENTS.md.

3. **Split the render passes.** `RenderAll` renders neutral artifacts (docs, AGENTS.md, domain docs,
   ADR readme/template, plans-readme, ACTIVE.md) **exactly once**, and adapter artifacts (skills,
   agents, bridge) **once per enabled target**, with each artifact's output path taken from the
   loop's `Target`, not from `p`-wide state. The same skill/agent body and frontmatter is written to
   every target's path unchanged.

4. **Prune empty ancestors.** When a target is dropped from `targets:`, Sync removes its rendered
   files (existing prune) and walks **all** now-empty ancestor directories up toward the repo root,
   reusing `Uninstall`'s ancestor-walk, so no empty adapter directory tree lingers.

5. **`awf add/remove/list target`.** Provide a bespoke CLI path for the `targets` array, validated
   against the known-adapter set, that edits `.awf/config.yaml`. `targets` is **not** a
   `kindDescriptor` and does not enter the catalog/sidecar/parts/orphan machinery; `orphans()` stays
   scoped to skills/agents/docs/domains. Hand-editing `targets:` in `config.yaml` remains supported.

6. **Tool-agnostic skill/agent prose** (extends ADR-0018). Rendered skill and agent prose names
   actions, not a specific runtime's tools. Neutralise the Claude-tool vocabulary in the affected
   skill templates (`brainstorming`, the four `reviewing-*`, `refactor-coupling-audit`,
   `executing-plans`, `proposing-adr`, `writing-plans`, `subagent-driven-development`):
   "dispatch a fresh-context `<name>` subagent" / "ask a multiple-choice question" rather than naming
   the `Agent`/`Skill`/`AskUserQuestion` tools. Record the rule in `doc-standard.md`.

7. **Rename the artifact-sense "target" to "artifact"** across `renderTarget`, `targetConfigHash`,
   `planSections`, `consumedParts`, `partRel`, `config.PartPath`, and the orphan-loop vocabulary —
   a mechanical, behaviour-preserving rename that leaves `catalog.VarDescriptor.Target` untouched.

## Invariants

Tagged slugs are backed by tests landing with implementation (enforced by `awf check` once this ADR
is `Implemented`); untagged bullets are textual contracts. This ADR retires ADR-0016's
`target-output-paths` (singular `Target`); `multi-target-render` is its generalised successor. It
also realises the user-facing `targets:` key ADR-0016 Decision item 2 deliberately deferred ("the
key lands with the second adapter"), so introducing `targets:` extends rather than contradicts
ADR-0016.

- `inv: multi-target-render` — with `targets` enabling N adapters, each adapter artifact (skill,
  agent) renders once per enabled target to that target's paths (for `claude`,
  `.claude/skills/<prefix>-<name>/SKILL.md` and `.claude/agents/<name>.md`; for `cursor`,
  `.cursor/skills/<prefix>-<name>/SKILL.md` and `.cursor/agents/<name>.md`), with paths produced by
  the `Target` descriptor, not render-loop literals; neutral artifacts render exactly once
  regardless of N.
- `inv: targets-default-claude` — a config with no `targets:` key loads as `["claude"]`; `Validate`
  rejects an empty `targets` list and any unknown adapter name.
- `inv: cursor-no-bridge` — the `cursor` target has an empty `BridgeFile` and emits no bridge file;
  its rendered skill and agent files are byte-identical in body and frontmatter to the `claude`
  target's at their respective paths.
- `inv: target-prune-ancestors` — removing a target from `targets:` and re-syncing deletes that
  target's rendered files and every resulting empty ancestor directory, not only the immediate
  parent.
- `inv: target-cli` — `awf add target` / `awf remove target` mutate the config `targets` array
  against the known-adapter set, and `awf list target` reads it, all without routing through the
  `kindDescriptor` machinery. This adds a sixth CLI kind token alongside ADR-0024's five;
  `inv: cli-config-kinds` (which covers the five `kindDescriptor`-backed arrays) is **extended, not
  contradicted** — `targets` is the bespoke path Decision 5 keeps outside that machinery, and
  ADR-0024's `cli-config-kinds` backing test is updated for the added token in the same commit that
  flips this ADR to `Implemented`.
- `inv: skill-prose-tool-agnostic` — rendered skill and agent bodies contain no Claude-specific tool
  token. The backing check matches the vocabulary Decision 6 neutralises **case-insensitively**,
  covering at least `subagent_type` / "subagent type", "Agent tool" / "the agent tool", "Skill tool",
  and "AskUserQuestion", plus runtime-specific subagent-type names where they appear as a tool
  argument — so the three `reviewing-*` skills that say "the agent tool with subagent type" are
  caught, not only the templates carrying the capitalised tokens.
- Neutral artifacts keep their existing single paths; only adapter artifacts multiply across
  targets. (Textual.)
- The known-adapter set is `{claude, cursor}`; adding a third adapter is a new `Target` value plus
  registry entry, not a render-loop change. (Textual.)

## Consequences

- awf renders to Claude Code and Cursor from one config; the seam is proven by a real second runtime
  rather than a hypothetical, making a third adapter (Codex, Copilot, Gemini) a `Target` literal plus
  a registry entry.
- ADR-0016 stays `Implemented`; its `target-output-paths` invariant is retired here (per ADR-0031)
  and its backing comment/test is removed in favour of `multi-target-render`, whose backing test
  covers both `claude` and `cursor` paths. ADR-0016's narrative is unchanged (append-only).
- The render loop is restructured (neutral vs adapter passes separated); this is the only sizeable
  refactor. Drift, collision-guarding (`PlannedOutputs`), and pruning need no new bookkeeping beyond
  the ancestor-walk because they already derive from `RenderAll`.
- awf dogfoods `targets: [claude, cursor]`: the repo gains a committed, drift-checked `.cursor/` tree
  and a `!.cursor/` `.gitignore` negation (global ignores commonly hide `.cursor`, which would
  otherwise silently break the CI drift gate).
- Local skills/agents (none shipped today) would resolve to N output paths under multiple targets;
  `localOutPath`/`checkLocalFrontmatter` take a single `Target`, so the implementation plan must
  resolve their multi-target behaviour (e.g. validate each target's path, or scope local artifacts
  to the first/`claude` target and document the limitation) — this ADR commits to resolving it in the
  plan rather than leaving it open.
- Decision 6 makes the standard genuinely portable but is separable; if the reviewer splits it, the
  rename (Decision 7) and the mechanism (Decisions 1–5) still stand alone.

Doc-currency obligations the implementing commit(s) must satisfy:

- `docs/architecture.md` documents the plural-`Targets` seam, the separated neutral/adapter render
  passes, and the Cursor adapter (no bridge); the `rendering`, `config`, and `tooling` domain docs'
  current-state narratives are refreshed.
- The agent guide's "Working with awf" section and the README command table gain the
  `awf add/remove/list target` grammar, extending the ADR-0024 CLI surface.
- `doc-standard.md` records the tool-agnostic-prose rule (Decision 6) — if Decision 6 lands in this
  ADR rather than a split successor.
- The dogfood commit adds the committed, drift-checked `.cursor/` tree and the `!.cursor/`
  `.gitignore` negation.
- In the commit that flips this ADR to `Implemented`: ADR-0016's `target-output-paths` backing
  test is removed and `multi-target-render`'s backing test (covering both `claude` and `cursor`
  paths) lands; ADR-0024's `cli-config-kinds` backing test is updated for the `target` token; and
  `docs/decisions/ACTIVE.md` is regenerated via `./x sync`. No `docs/decisions/README.md` ADR-index
  row is owed (the README is a how-to guide; `ACTIVE.md` is the generated index — ADR-0005).

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Single switchable `target:` (one runtime per repo) | Forecloses awf's multi-tool positioning; the field is converging on shared SKILL.md/AGENTS.md standards, so rendering several at once is the higher-value default. |
| Map awf skills to Cursor **Rules** (`.cursor/rules/*.mdc`) | Worse semantic fit (rules are always-on/glob-scoped project knowledge; awf skills are on-demand procedures) and forces a frontmatter re-dialect that Skills make unnecessary. |
| Emit a thin always-apply Cursor rule pointing at AGENTS.md | Redundant — Cursor reads AGENTS.md natively; extra rendered output to maintain for no gain. |
| Build a frontmatter-transform / container-rename capability now | Speculative: Claude and Cursor both use identity under the shared standard; the capability earns its place only when a genuinely divergent adapter (e.g. single-file Copilot) arrives. |
| Make `targets` a fifth `kindDescriptor` | `targets` has no catalog pool, sidecars, parts, or orphan semantics; forcing it into the kind machinery drags it into `orphans()` and render dispatch for no benefit. A bespoke CLI path is cleaner. |
| `awf check` lint forbidding Claude-tool tokens | Heavier than warranted; a golden test backing `skill-prose-tool-agnostic` already guards regressions without a new drift kind. |
