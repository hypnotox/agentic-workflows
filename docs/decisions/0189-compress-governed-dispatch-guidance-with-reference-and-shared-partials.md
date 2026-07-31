---
format: current-state-v2
status: Proposed
date: 2026-07-31
---
# ADR-0189: Compress governed dispatch guidance with reference and shared partials

## Context

The 2026-07-30 skill and agent prose audit
(docs/research/skill-agent-prose-audit-2026-07-30.md) found roughly 150+ template lines of
restatement across the workflow skill and agent templates. The largest single item is the
governed model-selection paragraph: the ~75-word "smallest model expected to complete
reliably" block with its small/standard/large tier glosses appears 26 times across 8 skill
templates at HEAD 142dba0c (brainstorming, executing-plans, exploring, reviewing-adr,
reviewing-impl, reviewing-plan, reviewing-plan-resync, subagent-driven-development; two to
four copies per file across sections and Pi/non-Pi branches), plus once in the agent-guide
template, and every adopter's rendered guide carries it again.

Constraints verified against the repository:

- The paragraph is claim-governed. `deliberate-subagent-model-selection` (Origin
  ADR-0173, Backing: test) is a behavioural claim: every final governed subagent dispatch
  chooses the smallest reliable tier and reconsiders escalation, with a Pi routing rule and
  a non-Pi explicit-selection rule. The claim text names the tiers but does not mandate the
  per-site definitional glosses; the verbatim-at-every-site restatement is the proof shape
  chosen by `internal/project/subagent_model_selection_test.go`, which pins five common
  clauses plus the branch rule at each governed dispatch section. Changing the rendered
  text is therefore a deliberate decision about the claim's proof shape, not drift repair.
- A dispatched subagent may not have the agent guide in context: Pi loads only the agent
  artifact per the role-contract loader (ADR-0179). Full deferral of the rule to the guide
  would leave such a child without the selection rule. The guide itself cannot be disabled
  by any adopter (`agents-doc` is Mandatory and ADR-0061 keeps mandatory docs out of the
  toggleable pool), so guide presence is guaranteed for the parent, not the child.
- There is no rendered-identical extraction tier. `awf:include` directives are
  line-anchored (the directive occupies a whole line and is replaced wholesale), and the
  model-selection block sits mid-line at every site (numbered list items in six skills,
  prose paragraphs in exploring and executing-plans), so no site is line-anchored and any
  extraction changes rendered line structure.
  Includes expand before section parsing and template execution, so one partial can carry
  both the Pi and non-Pi branches as template actions; partials admit no nesting and no
  section markers.
- The same audit quantified a wider duplication class the deferred reviewer-spine-dedup
  backlog item already owns: ~45 duplicated template lines across the review skills and
  ~10 across the reviewer agents (intro sentences, memory preambles, route-findings and
  verify-tail blocks), beyond the existing review-spine-head/tail partials.
- ADR-0187 set the precedent this decision generalises: `templates/partials/`
  already shares the orientation ladder between a skill body and an agent contract.

## Decision

1. Rendered governed-dispatch guidance follows a compress-with-reference policy: each
   governed dispatch site keeps a one-line rule that names the small, standard, and large
   tiers and the escalation trigger, and states the target's branch rule (Pi routing or
   explicit native selection). The full tier definitions have exactly one rendered home
   per target, the agent guide's workflow section, sourced from a single shared
   model-selection partial in `templates/partials/` whose first consumer is the
   agent-guide template; no skill site includes the full form. Full deferral to the
   guide alone is ruled out because a dispatched child may lack the guide; unchanged
   per-site verbatim restatement is ruled out as the duplication this decision removes.
2. Repeated multi-file spine prose in `templates/` is maintained as shared partials under
   `templates/partials/`, included via `awf:include`, and a partial may carry template
   actions to hold target-conditional branches. This convention authorises rendered-level
   deduplication of the reviewer spine (the intro, memory-preamble, route-findings, and
   verify-tail blocks duplicated across the three reviewer agents and four review skills),
   subsuming the deferred reviewer-spine-dedup backlog item, and gives the audit's other
   cross-file spines a sanctioned home. An extraction proceeds as plan-level work under
   this policy only while it preserves section ids, the catalog `Sections` sets, and
   `awf:edit` overridability, changing line structure and wording of restated contract
   prose alone; a change to a section id or a catalog section set is outside this
   delegation and needs its own decision.
3. The `deliberate-subagent-model-selection` claim is revised to match: its behavioural
   commitments stand, and its canonical prose gains the rendered-shape commitment this
   decision makes, in substance: each governed dispatch section carries the compressed
   tier-and-escalation rule with its target branch rule, and the full tier definitions
   render once per target in the agent guide's workflow section, sourced from the shared
   model-selection partial. This is a canonical claim-prose change, not a provenance-only
   edit; the proof pins the compressed rule at every governed dispatch section and the
   full definitions via the rendered guide.
   `internal/project/subagent_model_selection_test.go` refreshes its pinned literals in
   the same transaction as the claim revision and the template compression, keeping claim
   provenance, backing, and rendered output consistent in one commit.
4. The hand-written convention part `.awf/parts/working-with-awf/config-and-overrides.md`
   restates the full model-selection block and renders into `docs/working-with-awf.md`;
   the same transaction trims it to the compressed rule with a reference to the agent
   guide, so no third rendered home of the full definitional form survives in this repo.

## State changes

- update `rendering/workflow-skill-templates:deliberate-subagent-model-selection`

## Consequences

- Every rendered skill with governed dispatch sections shrinks; the realistic saving is
  bounded (roughly 20 to 40 words per section across 13 governed dispatch sections per
  rendered target, 14 counting the agent guide) because the branch rule is load-bearing
  and stays per-section. Rendered output changes for every enabled target and for the
  sundial example, so the change regenerates and commits all rendered artifacts with the
  templates, and the eventual status flip regenerates `docs/decisions/INDEX.md` via
  `./x render` in the same commit.
- The claim revision, template compression, test-literal refresh, and rendered output must
  land as one transaction; `awf check` validates claim provenance against this ADR's
  operation, so partial landings red the gate. The executing plan must quote the claim
  text it revises.
- The reviewer-spine-dedup backlog item stops being deferred-without-scope: its concrete
  scope is decision 2, and follow-on extraction of the audit's remaining spines (execution
  spine, exploration ladder, effort-memory preambles, reviewer intros) proceeds as
  plan-level work without new decisions.
- Adopters rendering with unset vars keep coherent generic prose: the compressed one-line
  rule and the partial degrade the same way the current blocks do, and publication-safety
  invariants are unaffected.
- A partial imposes a factoring ceiling on the prose it absorbs: it may not nest another
  include and may not contain a section marker, so spine prose moved into a partial
  cannot host an `awf:section` overridable region - directly relevant to the reviewer
  spine, whose sites are section-bearing; the section boundaries stay in the including
  templates. Template readers also accept an indirection cost, following an include to
  see the rendered text.
- Risk: a one-line rule teaches less than the full glosses at the dispatch site. Accepted
  because the guide is always rendered for the parent session, the shared partial serves
  any site needing the full form, and the tier names plus escalation trigger stay
  self-sufficient for a child that sees only the skill body.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep per-site verbatim restatement | 26 copies drift independently; the audit already found micro-drift in sibling spines |
| Full deferral to the agent guide | A dispatched child may not load the guide (Pi loads only the agent artifact, ADR-0179) |
| Source-only partial extraction with unchanged rendered output | Impossible as a distinct tier: includes are line-anchored and the blocks sit mid-sentence in list items, and it would leave the rendered duplication untouched |
| Narrow test-literal refresh without an ADR | The claim text arguably permits compression, but the policy would be undocumented and the partials convention unauthorised; a durable prose-economy rule warrants a record |

## Status history

- 2026-07-31: Proposed
