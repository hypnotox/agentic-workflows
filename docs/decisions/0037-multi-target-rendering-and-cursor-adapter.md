---
status: Implemented
date: 2026-06-29
tags: [multi-target, target-seam]
related: [14, 16, 24]
domains: [rendering, config, tooling]
---
# ADR-0037: Multi-Target Rendering and the Cursor Adapter

## Context

ADR-0016 built a `Target` seam (`internal/project/target.go`) so adapter artifacts (skills,
agents, the bridge file) get their output paths from a named runtime descriptor rather than
literals, and split artifacts into **neutral** (AGENTS.md, docs, domain docs, ADR infra: fixed
paths) versus **adapter** (target-placed). The seam was explicitly designed to admit a second
runtime, but `claudeTarget` remains the sole built-in: `Open` hardwires `Target: claudeTarget`
(project.go:41), `Project.Target` is a single value, and there is no user-facing way to select or
add a runtime. awf positions itself as a multi-tool agentic-workflow renderer, yet it renders only
for Claude Code.

Adding **Cursor** as a second adapter realises that positioning and, by being the first concrete
second runtime, validates how far the seam must stretch. A web check of Cursor's current (2026)
conventions settled the shape:

- Cursor **Skills** are the same `SKILL.md` open standard as Claude skills: a per-skill folder
  containing `SKILL.md` with `name`/`description` frontmatter, progressive disclosure, on-demand
  description-based activation. awf skills map to Cursor Skills with **identical body and
  frontmatter**.
- Cursor **subagents** (2.4, Jan 2026) are flat markdown files in `.cursor/agents/<name>.md`,
  matching Claude's `.claude/agents/<name>.md` shape.
- Cursor reads **AGENTS.md natively** at repo root, so it needs **no bridge file**.

Because both runtimes share the SKILL.md/AGENTS.md standards, the same rendered body and frontmatter
are written to two paths; no per-adapter frontmatter transform or per-adapter template set is
needed. The grounding check confirmed the lock is keyed by output path (project.go:138) and
`targetConfigHash` does not fold the path (confighash.go), so two entries for one skill carry
identical hashes under distinct keys and cannot collide. It also confirmed the prune in Sync
(project.go:144) already drops files no longer produced, and `PlannedOutputs`/`RenderAll` are the
single source of truth for the output set, so multi-target drift, collision-guarding, and pruning
fall out of one render-loop change rather than bespoke per-target bookkeeping.

Three frictions surfaced that this ADR must address head-on:

1. **The render loop is not cleanly split.** Skills (adapter), agents (adapter), and docs (neutral)
   share one `renderKind` driver (render.go:115), and the bridge is nested inside the AGENTS.md
   block (render.go:169). Rendering docs/AGENTS.md once while rendering skills/agents/bridge
   per-target requires actually separating the neutral and adapter passes.
2. **Pruning is shallow.** `os.Remove(filepath.Dir(file))` clears only the immediate parent
   (project.go:150), so removing a target would leave empty `.cursor/`, `.cursor/skills/` behind.
   `Uninstall` already walks all empty ancestors (install.go:96); that pattern must be reused.
3. **A term clash.** ADR-0016's `Target` means *adapter*, but `renderTarget`/`targetConfigHash`/
   `planSections`/`partRel`/`config.PartPath` and the orphan loop use "target" to mean *the managed
   artifact's name*. With `Targets` now plural this is actively confusing; a third, unrelated sense
   (`catalog.VarDescriptor.Target` in `internal/initspec`) must be left untouched.

A separable concern that surfaced alongside this work (the shipped skill prose names
Claude-specific tools, which leak verbatim into Cursor's `.cursor/skills/`) is split out as its
own decision in ADR-0038 (tool-agnostic skill/agent prose, extending ADR-0018). The two land in one
plan; nothing in this ADR depends on it.

## Decision

1. **`targets` config enable array.** Add `Targets []string` to `internal/config.Config`. When the
   `targets:` key is absent, `Load` injects `["claude"]` (mirroring the `DocsDir` default at
   config.go:94), so a pre-existing config renders byte-identical with no schema-version bump and no
   `internal/migrate` entry: the backward-safe optional-field precedent set by `Domains` (ADR-0014).
   `config.Validate` rejects an empty list and path-separator names (sanity only: `internal/config`
   stays free of the adapter registry to avoid an import cycle); `project.Open`, via `resolveTargets`,
   rejects any name not in the known-adapter registry, since that is where the registry lives.

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

6. **Rename the artifact-sense "target" to "artifact"** across `renderTarget`, `targetConfigHash`,
   `planSections`, `consumedParts`, `partRel`, `config.PartPath`, and the orphan-loop vocabulary:
   a mechanical, behaviour-preserving rename that leaves `catalog.VarDescriptor.Target` untouched.

7. **Retirement bookkeeping (migrated from retires_invariants by awf upgrade,
   ADR-0120).** This ADR retires `supersedes-invariant: ADR-0016#target-output-paths`.

## Invariants

Tagged slugs are backed by tests landing with implementation (enforced by `awf check` once this ADR
is `Implemented`); untagged bullets are textual contracts. This ADR retires ADR-0016's
`target-output-paths` (singular `Target`); `multi-target-render` is its generalised successor. It
also realises the user-facing `targets:` key ADR-0016 Decision item 2 deliberately deferred ("the
key lands with the second adapter"), so introducing `targets:` extends rather than contradicts
ADR-0016.

- `invariant: multi-target-render`: with `targets` enabling N adapters, each adapter artifact (skill,
  agent) renders once per enabled target to that target's paths (for `claude`,
  `.claude/skills/<prefix>-<name>/SKILL.md` and `.claude/agents/<name>.md`; for `cursor`,
  `.cursor/skills/<prefix>-<name>/SKILL.md` and `.cursor/agents/<name>.md`), with paths produced by
  the `Target` descriptor, not render-loop literals; neutral artifacts render exactly once
  regardless of N.
- `invariant: targets-default-claude`: a config with no `targets:` key loads as `["claude"]`;
  `config.Validate` rejects an empty `targets` list and path-separator names; `project.Open` (via
  `resolveTargets`) rejects any unknown adapter name: config stays registry-free, so the
  unknown-name check lives where the adapter registry does.
- `invariant: cursor-no-bridge`: the `cursor` target has an empty `BridgeFile` and emits no bridge file;
  its rendered skill and agent files are byte-identical in body and frontmatter to the `claude`
  target's at their respective paths.
- `invariant: target-prune-ancestors`: removing a target from `targets:` and re-syncing deletes that
  target's rendered files and every resulting empty ancestor directory, not only the immediate
  parent.
- `invariant: target-cli`: `awf add target` / `awf remove target` mutate the config `targets` array
  against the known-adapter set, and `awf list target` reads it, all without routing through the
  `kindDescriptor` machinery. This adds a CLI kind token (`target`) alongside the
  `kindDescriptor`-backed kinds; `inv: cli-config-kinds` (backed by the marker comment on the
  `kindDescriptors` table plus the `Kinds()` assertion) is **unaffected**: `targets` is the bespoke
  path Decision 5 keeps outside that machinery, so `target` is **not** a `kindDescriptor`, `Kinds()`
  does not change, and no `cli-config-kinds` backing change is required.
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
- Local skills/agents (none shipped today) resolve to N output paths under multiple targets: a
  declared local skill must exist with valid frontmatter at **every** enabled target's path
  (`localOutPath` becomes `localOutPaths`, one path per target; `checkLocalFrontmatter` validates
  each). Chosen over scoping locals to the first/`claude` target so no target carries an unchecked
  hand-authored file.
- The artifact-sense rename (Decision 6) is mechanical and behaviour-preserving; it lands with the
  same change because Decision 2's pluralization is what makes the term clash acute.

Doc-currency obligations the implementing commit(s) must satisfy:

- `docs/architecture.md` documents the plural-`Targets` seam, the separated neutral/adapter render
  passes, and the Cursor adapter (no bridge); the `rendering`, `config`, and `tooling` domain docs'
  current-state narratives are refreshed.
- The agent guide's "Working with awf" section and the README command table gain the
  `awf add/remove/list target` grammar, extending the ADR-0024 CLI surface.
- The dogfood commit adds the committed, drift-checked `.cursor/` tree and the `!.cursor/`
  `.gitignore` negation.
- In the commit that flips this ADR to `Implemented`: ADR-0016's `target-output-paths` backing
  test is removed and `multi-target-render`'s backing test (covering both `claude` and `cursor`
  paths) lands; `docs/decisions/ACTIVE.md` is regenerated via `./x sync`. ADR-0024's
  `cli-config-kinds` backing needs no change (`target` stays outside the `kindDescriptor` machinery).
  No `docs/decisions/README.md` ADR-index row is owed (the README is a how-to guide; `ACTIVE.md` is
  the generated index, ADR-0005).

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Single switchable `target:` (one runtime per repo) | Forecloses awf's multi-tool positioning; the field is converging on shared SKILL.md/AGENTS.md standards, so rendering several at once is the higher-value default. |
| Map awf skills to Cursor **Rules** (`.cursor/rules/*.mdc`) | Worse semantic fit (rules are always-on/glob-scoped project knowledge; awf skills are on-demand procedures) and forces a frontmatter re-dialect that Skills make unnecessary. |
| Emit a thin always-apply Cursor rule pointing at AGENTS.md | Redundant: Cursor reads AGENTS.md natively; extra rendered output to maintain for no gain. |
| Build a frontmatter-transform / container-rename capability now | Speculative: Claude and Cursor both use identity under the shared standard; the capability earns its place only when a genuinely divergent adapter (e.g. single-file Copilot) arrives. |
| Make `targets` a fifth `kindDescriptor` | `targets` has no catalog pool, sidecars, parts, or orphan semantics; forcing it into the kind machinery drags it into `orphans()` and render dispatch for no benefit. A bespoke CLI path is cleaner. |

## Migration history

- 2026-06-29: retired invariant `ADR-0016#target-output-paths`; basis: encoded
