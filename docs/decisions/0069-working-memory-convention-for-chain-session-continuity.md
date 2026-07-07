---
status: Implemented
date: 2026-07-07
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [workflow, memory, context, chain, skills, rendering]
related: [15, 20, 34, 45, 46, 47, 48, 52, 54, 60, 61, 67]
domains: [rendering, tooling]
---
# ADR-0069: Working-memory convention for chain session continuity

## Context

The workflow chain's durable state is asymmetric. From the plan phase onward every
phase leaves recoverable artifacts on disk: the plan file carries checkbox tasks
executable "with no prior conversation context", the ADR file carries the decision, and
commits carry progress. The pre-plan phase carries nothing: `awf-brainstorming` holds
the negotiated design — user constraints, rejected approaches, agreed decisions —
only in conversation context, and its `grounding-check-output-format` section
explicitly forbids writing the brief to a file. A session death or context compaction
mid-brainstorm destroys the entire negotiation. Chain *position* is equally volatile:
nothing on disk says which phase an in-flight effort is in or which skill fires next.

This is the deferred second half of "the two loops" from the 2026-07-06 deep-analysis
gap map (the first half — the review-finding feedback loop — shipped as ADR-0067). The
field consensus ([the research doc](../research/agentic-workflow-landscape-and-awf-standing-2026-07.md),
Pillar 1) is note-taking/working-memory plus
just-in-time retrieval: hold lightweight identifiers, persist working state outside
the context window, re-read on demand.

Mechanical facts established by a grounding pass against source:

- Bootstrap and hooks are **not** `DocEntry`s; they render as hardcoded blocks at the
  tail of `RenderAll` (`internal/project/render.go`), gated on config enablement.
  Everything `RenderAll` returns is automatically written, lock-tracked,
  drift-checked, pruned when dropped, collision-probed at init, and removed by
  uninstall — a new render unit needs no bespoke lifecycle wiring.
- A `DocEntry` cannot express an output path outside `docsDir` (`plainSingletons`
  hardcodes `l.DocsDir + "/" + e.Path`; the only escape is the `AgentsDoc` root flag),
  so a `.awf/`-tree file does not fit the ADR-0060/0061 doc collection without
  reworking its path derivation.
- `injectBanner` (`internal/project/banner.go`) knows three shapes: `#`-comment after
  a shebang, HTML comment after frontmatter, HTML comment as first line. A rendered
  `.gitignore` matches none — the default branch would prepend an HTML comment as a
  junk ignore pattern.
- `isManagedMarkdown` (`internal/project/check.go`) scopes the dead-link and
  dead-skill-reference scans; it excludes only bridge/bootstrap/hooks TIDs, so a new
  TID is scanned by default.
- The agents-doc template's `awf:section` markers are parity-checked by **nothing**:
  the skills/agents parity test (ADR-0054) covers skills and agents, the docs parity
  test skips `Mandatory` entries, and the singleton parity test iterates
  `plainSingletons`, which excludes the `AgentsDoc` entry. A guide section added to
  the template but not the catalog (or vice versa) renders green with a broken
  override path — exactly the ADR-0054 failure class, on the one artifact class it
  missed.
- A shared partial spliced into many skills multiplies the ADR-0046
  dead-skill-reference risk: an unconditional skill-name mention trips drift for every
  adopter who trims that one skill.

## Decision

1. **Always-on memory render unit.** `awf sync` unconditionally renders
   `.awf/memory/.gitignore` ignoring everything in the directory except itself
   (patterns `*` and `!.gitignore`), from a new embedded template
   `templates/memory/gitignore.tmpl` (not dot-prefixed — the `go:embed` pitfall). It
   renders once, neutrally (outside the per-target loop), via the same `RenderAll`
   tail-block mechanism as bootstrap and hooks, minus the config gate. It is
   deliberately non-configurable: no enable array, no CLI kind, no sidecar, no
   convention parts, no `local:` escape. Lock tracking, drift checking, prune,
   init-collision probing, and uninstall removal all follow automatically from being a
   `RenderAll` output.

2. **Banner shape.** `injectBanner` gains a `#`-comment branch for this render unit,
   so the file carries the standard provenance banner as a valid gitignore comment.
   The existing `provenance-banner` invariant's backing test extends to the new
   branch.

3. **Scan scoping.** The new TID joins the `isManagedMarkdown` exclusion list
   (bootstrap/hooks precedent): a gitignore is not managed markdown and must not
   enter the dead-link or dead-skill-reference scans.

4. **Doc-model boundary.** The ADR-0060/0061 doc collection covers *prose documents*;
   **config-tree render units** — bootstrap (ADR-0047), hook payloads (ADR-0048), and
   now the memory gitignore — are a distinct category rendered by dedicated
   `RenderAll` blocks, not `DocEntry`s. The agent guide's unified-doc-model invariant
   wording is updated to draw this boundary explicitly.

5. **Working-memory convention (the prose contract).** A new agent-guide section
   documents the convention: one file per effort at `.awf/memory/<effort-slug>.md`;
   at session start check `.awf/memory/` for in-flight effort files and resume from
   the recorded phase — when several exist or the file matches no in-flight work, ask
   the user rather than silently resuming; while working, hold identifiers (paths,
   ADR numbers) and read sources on demand rather than preloading. The file skeleton
   is a documented *convention, not a schema* (the tool never parses it): a header
   (title, `Phase:`, `Next:`, `Updated:`), `## Brief`, `## Handoff log`,
   `## Scratch`. Ground rules: never committed (the gitignore makes this mechanical),
   never cited by an ADR, plan, or commit message — it is session state, not a design
   artifact, so the no-spec-rule stands — and deleted when the chain terminates.

6. **Agents-doc section parity.** A new parity test asserts the agents-doc template's
   `awf:section` markers match its catalog-declared sections, closing the verified
   ADR-0054 gap for the artifact class this convention's guide section lands in.

7. **Brainstorming checkpoints continuously.** The brainstorming skill instructs
   updating `.awf/memory/<effort-slug>.md` as each decision settles — it is the one
   phase with no other durable state. Its grounding-check section's "Do NOT write the
   brief to a file" is reworded: the grounding *subagent dispatch brief* stays inline
   in the prompt (synthesised, never written to disk); the evolving *design brief*
   lives in the memory file and nowhere else. The `no-spec-rule` section is unchanged.

8. **Chain checkpoint partial.** A shared partial
   `templates/partials/memory-checkpoint.md` (ADR-0052 `awf:include`) instructs
   updating the memory file's phase/next/handoff lines, spliced **inside** the
   declared terminal/hand-off section of the nine non-terminal chain nodes
   (brainstorming `terminal-step`, proposing-adr `terminal-step`, reviewing-adr
   `hand-off-to-resync`, writing-plans `terminal-step`, reviewing-plan `hand-off`,
   reviewing-plan-resync `hand-off-to-impl`, executing-plans `terminal-step`,
   subagent-driven-development `terminal-step`, reviewing-impl `hand-off`) and of the
   task skills **bugfix** and **debugging** (multi-step efforts that lose state the
   same way; roadmap-graduation is self-contained maintenance and excluded). Bugfix
   and debugging declare no terminal/hand-off section today (bugfix ends at
   `oracle-note`, debugging at `red-flags`), so each gains a new declared
   `memory-checkpoint` section — a catalog `Sections` addition, kept honest by the
   ADR-0054 `skill-section-parity` test — as its splice target. Inside
   the section means adopter part-overrides replace it knowingly — consistent with
   every other section body. Any skill-name mention in the partial is guarded with
   `{{ if index .skills "…" }}` and gets an unset-degradation test lock (the ADR-0067
   pitfall).

9. **Retrospective deletes.** The retrospective skill — the chain terminal — gains
   the deletion step for the effort's memory file, the ephemerality endpoint. It
   lands as a final numbered step inside the skill's declared `procedure` section,
   so it is overridable on the same contract as the checkpoints.

10. **Tool boundary.** awf's involvement stops at rendering the gitignore: no memory
    subcommands, no parsing of memory files, no staleness checks. Files orphaned by an
    abandoned session or a disabled retrospective are accepted gitignored residue;
    `awf uninstall` removes the lock-tracked gitignore but leaves a non-empty
    `.awf/memory/` behind, and both facts are documented in the agent guide's
    working-memory section (Decision 5) rather than engineered around.

11. **Rollout.** Existing adopters receive the file at their next `awf sync`. No
    config-shape change occurs, so there is no schema bump and no `minVersionBySchema`
    entry; `awf check` stays green until that sync (a rendered-but-never-locked path
    is flagged by nothing), which is accepted.

## Invariants

- `inv: memory-gitignore-always-on` — every `awf sync` renders `.awf/memory/.gitignore`
  unconditionally (no config gate), lock-tracked, with content that ignores everything
  in the directory except itself and carries a `#`-comment provenance banner.
- `inv: agents-doc-section-parity` — the agents-doc template's `awf:section` markers
  match the catalog-declared section list for the `agents-doc` entry.
- `inv: memory-checkpoint-chain-coverage` — every non-terminal chain-node skill
  template (the nine in Decision 8) plus `bugfix` and `debugging` carries the
  memory-checkpoint reference in its rendered full-catalog output; `retrospective`
  carries the deletion step.
- The tool never reads or parses memory files (textual contract — no code may take a
  dependency on memory-file content).
- Memory files are never cited by an ADR, plan, or commit message (textual contract,
  enforced by the rendered prose).

## Consequences

- A session death or compaction anywhere in the chain now has a durable resume path:
  the design brief survives brainstorming, and chain position survives every phase.
  The always-loaded agent guide is the discovery anchor, so a fresh session finds the
  convention without any skill in context.
- Adopters get the convention automatically at their next sync — no opt-in, no config
  migration.
- A third render-unit category (always-on, config-tree, non-markdown) now exists
  alongside mandatory `DocEntry`s and toggleable config-tree singletons. The doc-model
  boundary is stated (Decision 4) to keep ADR-0060/0061's "adding a mandatory doc is a
  single entry" claim honest — it applies to prose docs.
- An adopter who part-overrides a terminal/hand-off section silently drops that
  skill's checkpoint line; this is the standard section-override contract, accepted
  in exchange for overridability. The eval-suite coverage invariant only protects the
  no-parts render.
- Stale memory files can accumulate invisibly (gitignored, unscanned). Accepted:
  they are harmless residue, and cleanup machinery would violate the tool boundary
  (Decision 10).
- The memory file's quality is entirely probabilistic — awf renders the convention
  but cannot verify a checkpoint was written or is current. This is the same trust
  model as every other rendered instruction.
- Ruled out by this ADR: memory subcommands, staleness/orphan checks, committed
  working memory, toggleability of the gitignore, and any machine-parsed memory
  format.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Toggleable `memory` singleton (bootstrap/hooks precedent), disabling = "track my memory" | The convention prose in skills/guide is unconditional either way, so the toggle governs only the gitignore; the marginal adopter choice was judged not worth the second enablement state. Always-on chosen explicitly. |
| Prose-only: the agent creates the gitignore on first use | Ignoring becomes probabilistic — one forgotten gitignore commits session scratch. The generated file makes ephemerality mechanical. |
| Committed working memory (committed-then-deleted, or a durable `docs/notes/`) | Reintroduces the spec document the no-spec-rule bans; drifts against the ADR/plan mid-effort; pollutes history. |
| Model the gitignore as a `Mandatory` `DocEntry` | `plainSingletons` hardcodes docsDir-rooted paths; forcing a `.awf/` path through the doc collection reworks its projections for one non-prose file. The tail-block mechanism already fits. |
| Splice the checkpoint outside section markers (review-spine precedent) | Non-overridable; inconsistent with the everything-is-a-section contract for skill prose. Inside-section chosen, accepting that overrides drop it knowingly. |
| Brainstorming-only or chain-nodes-only scope | Drops the chain-position handoff note (which compaction also destroys) or leaves bugfix/debugging — real multi-step efforts — stateless. Full scope chosen. |
| `awf check` staleness advisory over `.awf/memory/` | Requires the tool to parse memory files, crossing the render-only boundary set for this feature. |
