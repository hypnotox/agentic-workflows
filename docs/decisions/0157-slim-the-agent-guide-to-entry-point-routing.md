---
format: current-state-v2
status: Proposed
date: 2026-07-23
---
# ADR-0157: Slim the agent guide to entry-point routing

## Context

The rendered agent guide has drifted against its own authoring standard. `docs/agents-md-standard.md` holds the guide to an "extra-terse bar" because it "pays its word cost on every task", and instructs authors to "push anything else not every-session-critical into a doc reached through the document map". The guide this repository renders is 23,737 bytes, and the template defaults every adopter renders carry the same weight: the workflow section alone embeds the full chain diagram, warrant definitions, the plan exact/pseudocode contract, V2 batch application semantics, and the exploration/subagent policy in one paragraph, and the working-memory section embeds the complete checkpoint protocol.

Nearly all of that prose is duplicated authority. The chain is self-routing: every chain skill's terminal step names its successor, and brainstorming's terminal step routes by the warrant outcome. The checkpoint and approval protocols render into every chain skill through the shared partials (proof-backed by `mandatory-approval-boundaries` and `memory-checkpoint-chain-coverage`). The plan contract renders in the plan-authoring skill, the plan reviewer, and the plans README (`plan-task-detail-modes`). V2 batch semantics render in the adr-lifecycle and both plan-execution skills. `docs/workflow.md` carries the chain rules; `docs/working-with-awf.md` carries usage and command detail. What an agent loses by not reading the guide's copies is nothing; what every session pays for them is roughly two thirds of the guide.

A verified inventory (fresh-context grounding check, 2026-07-23) established the residuals and couplings:

- The only guide-unique workflow prose is the "select a lower-cost child model" sentence and the concurrency/refinement sentence of the exploration policy; the sequential-implementation-subagent guidance already renders in the working-with-awf Pi section.
- The guide's closing "Conventional Commits; one concern per commit." is the only unconditional commit-discipline mention on an empty config; the invariants-section scopes bullet is data-driven and absent on empty init. The gate sentence, by contrast, is unconditional in the invariants section, so the workflow section's copy is a safe dedup.
- Two shared checkpoint partials (`templates/partials/checkpoint-routine.md`, `checkpoint-approval.md`), the brainstorming skill body, and `workflow.md.tmpl`'s chain section all point at "the agent guide's working-memory section" for the file skeleton and ground rules; slimming that section requires re-pointing them, which changes rendered skill bodies for every target.
- Two backed claims name the guide as a carrying surface: `workflow-chain-adr-before-plan` ("The rendered AGENTS.md and workflow.md workflow-chain string...") and `plan-task-detail-modes` ("...plans README, and agent guide accept..."). Their proofs assert guide-render strings (`internal/project/spine_test.go`, `internal/project/plan_detail_modes_test.go`).
- The guide's task-skill sentence is deliberately catalog-derived (`taskSkills` render key) so a newly added task skill cannot be dropped by a forgotten template edit; a hand-written replacement would reintroduce that failure mode.
- ADR-0155 (implementer-side grounding) landed before this decision; ADR-0156 (rendered awf wrapper) is in flight and edits the guide's awf-setup runner sentence, so implementation ordering against 0156 is settled at plan time.

## Decision

1. The guide template's workflow section becomes an entry-point router. It renders: one sentence establishing that non-trivial work starts with the brainstorming skill, whose chain hands off through review and retrospective via each skill's terminal step; a catalog-derived trigger table in which every enabled entry and task skill appears iff enabled, each with a one-line trigger derived from catalog metadata (preserving the derived-roster guarantee of the `taskSkills` render key, which the table becomes the consumer of); the unconditional closing line "Conventional Commits; one concern per commit." with a pointer to the workflow doc. The chain diagram, warrant definitions, plan-form contract, V2 batch semantics, exploration/subagent policy, and the duplicated gate sentence no longer render in the guide. This shape becomes the new backed invariant `rendering/guide-and-doc-templates:guide-entry-point-routing`: the guide's workflow section renders a catalog-derived entry-skill trigger table in which every catalog entry and task skill appears iff enabled, and none of the evicted prose classes renders: the chain diagram, warrant definitions, the plan-form contract, V2 batch semantics, the exploration/subagent policy, and the duplicated gate sentence.
2. The guide template's working-memory section shrinks to the routing minimum: where the files live, the check-on-resume trigger, never commit or cite the file, delete it when the chain terminates, and a pointer to the canonical protocol.
3. The workflow doc gains a `working-memory` section (catalog `Sections` change for the `workflow` doc descriptor) that becomes the single canonical home of the protocol: the file skeleton, the boundary steps, just-in-time retrieval, and the ground rules, absorbing both the guide's evicted prose and the chain section's existing duplicate protocol sentences. The chain section keeps one pointer to it. The guide-unique exploration-policy fragments (lower-cost child model, concurrent independent exploration with sequential refinement) move into the workflow doc.
4. Every rendered surface that points at "the agent guide's working-memory section" (the two shared checkpoint partials, the brainstorming skill body, the workflow doc chain section) re-points to the workflow doc's working-memory section. Single-home-plus-pointers becomes the new backed invariant `rendering/guide-and-doc-templates:working-memory-single-home`: the file skeleton, ground rules, and just-in-time retrieval prose render canonically in the workflow doc's working-memory section, and the guide, the shared checkpoint partials, and the chain section point to that content rather than carrying copies of it. The boundary-step and approval protocols the chain skills embed per `memory-checkpoint-chain-coverage` and `mandatory-approval-boundaries` are unaffected.
5. The guide template's awf-setup section shrinks to: rendered files are generated and never hand-edited, the edit-config/sync/check loop, and a pointer to the working-with-awf doc; toggle and override detail folds into that doc where not already present.
6. The guide template's Pi conditional branches shrink to the routing minimum: governed workflow entry goes through the `awf_workflow` router and governed bodies are never loaded directly. Telemetry, lifecycle, and handoff detail stay in the surfaces that already carry them.
7. The agents-md-standard doc codifies the shape as the authored contract: the workflow section is entry triggers only, the working-memory section is the resume trigger plus ground-rule one-liners and the canonical-protocol pointer only, and any procedure or protocol prose belongs in a doc reached through the document map.
8. Proofs retarget with the content: `workflow-chain-adr-before-plan` narrows its surface to the workflow doc's chain string and its assertion (with the resync assertion) moves from the guide render to the workflow doc render; `plan-task-detail-modes` drops the agent guide from its surface list and its guide proof case is removed. The two added claims get proofs at implementation: `guide-entry-point-routing` by guide-render assertions in `internal/project/spine_test.go` (trigger table present, evicted prose classes absent, empty-init collapse pinned), `working-memory-single-home` by a workflow-doc render assertion of the canonical section plus pointer-only assertions on the guide and partial renders. Degraded renders stay pinned: the empty-init guide stays coherent with a trigger table that collapses to a coherent minimal sentence when no chain skills are enabled, and the hand-authored fallback case is updated to the new shape.
9. This repository's own convention parts conform in the same effort so awf keeps modeling its standard: the agents-doc commands part's command reference prose moves to the working-with-awf commands part, the identity part drops Pi telemetry detail, the workflow part keeps a one-to-two line Pi router entry rule, the working-memory part shrinks to the Pi resume pointer, the awf-setup part is retrimmed against the slimmed default (ordering with ADR-0156's edit of the same content is settled at plan time), and the workflow chain part (`.awf/parts/workflow/chain.md`) relocates its appended Pi working-memory sentences to the new working-memory section so the rendered chain section conforms to the single-home invariant.
10. The changelog carries an upgrade note: adopters who replaced the workflow doc's chain section with a full-replacement part do not receive the relocated protocol prose and must re-derive their part or adopt the new section.
11. Every status transition of this ADR regenerates `docs/decisions/INDEX.md` via `./x sync` in the same commit.

## State changes

- update `rendering/workflow-skill-templates:workflow-chain-adr-before-plan`
- update `rendering/workflow-skill-templates:plan-task-detail-modes`
- add `rendering/guide-and-doc-templates:guide-entry-point-routing`
- add `rendering/guide-and-doc-templates:working-memory-single-home`

## Consequences

- Every adopter's guide shrinks on next sync; this repository's rendered guide drops from roughly 24KB to an estimated 7-9KB, dominated by the invariants section and document map that remain by design. Every session stops paying for duplicated chain, protocol, and command prose.
- Rendered skill bodies change for every target because the shared checkpoint partials re-point; this is a cross-target render diff with no behavioral change beyond the pointer.
- The relocation is move-not-delete and auditable: the implementation plan carries a per-paragraph disposition table (delete-as-duplicate with the canonical home cited, or move-with-target), with columns for the owning claim and proof site where one exists, so `awf check`'s claim/backing symmetry and the review can mechanically confirm nothing was silently lost.
- Adopters who overrode affected sections keep their overrides (replacement semantics are unchanged); the ones with full-replacement chain-section parts miss the relocated protocol prose until they update their part, which the changelog upgrade note calls out. Sync provenance labels the change "(template)", which understates a content relocation; the note compensates.
- The guide stops being a standalone summary of the workflow: an agent that never invokes a skill and never opens the workflow doc sees triggers, invariants, commands, and the document map only. This is the accepted trade: the chain is self-routing, and the two mandatory approval check-ins plus the memory-file convention cover mid-effort resume.
- Test surface moves: guide-render literals in the spine tests, the plan-detail-modes guide case, the empty-init and fallback pins, and the goldens all change in lockstep with the templates; two claims mutate and two are added, so the staged handshake enforces that tests and claim operations land together.
- Coordination debt: this decision touches the awf-setup content that in-flight ADR-0156 also edits; the plan sequences that file against 0156's landing order.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Chain-summary routing: keep the one-line chain diagram and skill roster with warrant criteria, evict only procedure detail | Roughly three times the words of the trigger table and keeps a drift-prone partial duplicate of what the skills and workflow doc own; the map is one pointer away when needed |
| A new using-awf skill carrying command and usage detail | Overlaps `docs/working-with-awf.md` nearly one-to-one, creating a second canonical home that must be kept in sync; docs already own this content and the document map already routes to it |
| Trim only this repository's parts, leave template defaults | The bloat is in the defaults, so every adopter keeps paying it; the repo would also stop modeling the standard it publishes |
| Slim the guide but leave the working-memory protocol duplicated in the chain section and guide | Two protocol homes is the drift pattern this decision exists to end; a single home with pointers is strictly cheaper to keep correct |

## Status history

- 2026-07-23: Proposed
