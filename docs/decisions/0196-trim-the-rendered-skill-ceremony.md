---
format: current-state-v2
status: Proposed
date: 2026-07-31
---
# ADR-0196: Trim the rendered skill ceremony

## Context

The rendered workflow skills carry several protocol blocks inlined in full at every
applicable site. The 2026-07-31 ceremony survey measured the repetition in this repository's
rendered corpus: the `AWF_CONTEXT_SPILL_V1` recovery paragraph is included at 14 template
sites, the routine-checkpoint block (roughly 150-200 words) at 10 sites plus 2
approval-variant sites, and the coverage-regression reminder at 3 sites. All of these are
already template partials: template-level maintenance is single-sourced, so the cost is not
drift risk but rendered context weight, paid on every skill invocation in every adopter
repository. The trade this decision makes is explicit: in-context availability of rarely
exercised or deterministically backstopped prose is exchanged for a smaller always-loaded
surface, with each displaced protocol keeping exactly one canonical rendered home.

Review ceremony has an analogous unconditional cost. The four reviewing skills
(reviewing-adr, reviewing-plan, reviewing-plan-resync, reviewing-impl) dispatch a verify
pass after every fix application, doubling every reviewer dispatch even when the applied
fixes were a single mechanical one-liner. The recorded firing history of verify passes in
this repository's decision corpus shows real catches (a wrongly applied reviewer nit, two
marked-but-unproven invariants), and every recorded catch followed a reasoned or structural
fix; no recorded catch followed a solely-mechanical fix round. Separately, the
working-memory protocol demands an indented verbatim `Record:` block for every
user-provenance decision-log entry regardless of materiality, so a naming preference costs
the same authoring and reviewer-brief ceremony as a package-boundary ruling.

Constraints and prior art:

- ADR-0190 already compressed the model-selection guidance at dispatch sites to the
  compressed tier rule plus a pointer, pinned by
  `rendering/workflow-skill-templates:deliberate-subagent-model-selection`. Compressing
  those sites further is a non-goal of this decision; the one residue it does take is
  `templates/docs/working-with-awf.md.tmpl` hand-inlining the partial's text instead of
  including `templates/partials/model-selection.md`.
- Four `Backing: test` claims pin prose this decision changes:
  `implementer-context-grounding` (every managed context-calling skill "consumes the exact
  spill notice ... and the projection-pinning spine test ... expands the shared spill
  contract"), `memory-checkpoint-chain-coverage` (the per-site checkpoint element set),
  `unified-effort-workflow-coverage` (each path's carried contract set), and
  `memory-log-consumer-coverage` (reviewer briefs "paste user entries verbatim with their
  `Record:` blocks"). Each needs an update operation; the claims stay, narrowed.
- No current-state claim pins the verify pass in any reviewing skill (repository sweep of
  `docs/topics/` returned zero hits), so the verify-pass condition is a template and test
  change only.
- The Conventional-Commits scope list is restated in the four reviewing skill templates
  while `awf check commit` already rejects a wrong scope deterministically at commit time;
  the coverage-regression reminder is restated in three task skills while the 100%-coverage
  gate already fails the build on any regression. Repetition that has no deterministic
  backing (the never-`--amend` rule, doc-currency reminders) is deliberately kept.

## Decision

1. The `AWF_CONTEXT_SPILL_V1` recovery contract moves to one canonical rendered home: the
   managed-context section of the working-with-awf doc. Every managed context-calling skill
   replaces the expanded paragraph with a single sentence naming the spill notice and
   pointing at that home. The spill partial becomes the pointer sentence; the full contract
   text renders only in the doc.
2. The routine-checkpoint partial compresses to a four-step digest: classify the outcome
   (a minimal simple fix uses no effort, reaching a checkpoint never creates one); validate
   the one immutable slug and owned `.awf/efforts/<slug>/memory.md` path and update the
   memory file in its own writer-owned batch; decide whether user attention is required
   using the existing trigger list; then raise a check-in and stop, or state a continuity
   notice and continue. The digest points at the workflow doc's working-memory section for
   the file skeleton, ground rules, and the full protocol, which remain rendered there in
   full. The approval-variant partial compresses the same way while keeping its
   stop-is-the-protocol clause.
3. The four reviewing skills dispatch the verify pass conditionally: it is dispatched when
   at least one applied fix is classified reasoned or user-decision-driven, and a fix round
   consisting solely of mechanical fixes skips it, recording the skip and its ground in the
   review summary. Finding classification vocabulary is unchanged.
4. The working-memory protocol narrows the `Record:` requirement to material decisions: an
   entry whose decision changes scope, design, authority, or previously-approved output
   carries the indented verbatim `Record:` block; any other user-provenance entry is a
   plain decision-log entry without one. Reviewer briefs paste user entries verbatim
   including whatever `Record:` blocks exist. The co-edited surfaces are the workflow doc
   template, the review-spine-head partial, the three reviewer-skill paste instructions,
   and the orienting skill's resume-revalidation enumeration.
5. Gate-duplicated prose is deleted: the four reviewing skills' restated scope list becomes
   "a Conventional-Commits scope the commit gate accepts", the coverage-regression reminder
   partial and its three inclusions are removed, and proposing-adr's duplicated INDEX.md
   hand-edit warning collapses to one mention per rendered skill. Repetition without
   deterministic backing (never `--amend`, doc-currency reminders) is kept unchanged.
6. `templates/docs/working-with-awf.md.tmpl` includes `templates/partials/model-selection.md`
   instead of hand-inlining its text; rendered output stays value-identical for a project
   whose override does not replace that section.
7. Explicitly declined relaxations, recorded here so they are not re-litigated: the two
   audits in reviewing-impl stay unconditional (deterministic and cheap is the kind of
   check this effort keeps), and the two mandatory approval stops at brainstorming end and
   settled ADR review stay unconditional.

## State changes

- update `rendering/workflow-skill-templates:implementer-context-grounding`
- update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- update `rendering/workflow-skill-templates:unified-effort-workflow-coverage`
- update `rendering/workflow-skill-templates:memory-log-consumer-coverage`

## Consequences

- Every skill invocation loads a materially smaller rendered body in every adopter
  repository; the displaced protocols keep exactly one rendered home each, so nothing
  becomes unwritten.
- A skill is no longer fully self-contained for the spill recovery path: an agent that hits
  a spill notice must read the working-with-awf doc's contract before consuming the packet.
  Accepted because the path is rare and deterministic to follow.
- A checkpoint site carries the operative digest rather than the full protocol; an agent
  that needs the skeleton or ground rules must follow the pointer to the workflow doc.
  Accepted for the same reason, and the digest retains every step that mutates state.
- A regression introduced by a solely-mechanical fix now reaches the next deterministic
  layer unverified instead of being caught by an immediate verify pass. Named and accepted:
  the recorded catch history clusters entirely on reasoned fixes, and the skip is recorded
  in the review summary so the trail stays auditable.
- Small user decisions lose their verbatim evidence block; a reviewer checking consensus
  adherence on an immaterial decision has the entry's paraphrase only. Named and accepted;
  material decisions keep the full block.
- The four updated claims narrow with their proofs in the same transaction; the spine and
  chain tests that expand the spill contract and enumerate checkpoint elements are
  rewritten against the digest and pointer forms.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Compress the model-selection dispatch sites further | Reverts ADR-0190's week-old pinned design; the compressed-rule-plus-pointer shape is the settled optimum |
| Move the spill contract into the agent guide instead of the doc | The guide is the always-loaded surface ADR-0157 deliberately slimmed; a rare-path contract belongs in an on-demand doc |
| Drop the verify pass entirely | Its recorded catch history on reasoned fixes is real; conditioning keeps the protection where it fires |
| Skip the verify pass by diff size instead of fix classification | Size is a poor proxy (a one-line reasoned fix can regress); the classification already exists and names the risk directly |
| Drop the `Record:` block entirely | Verbatim wording is the consensus-adherence anchor for material decisions; the materiality threshold keeps it where reviewers use it |
| Condition the two reviewing-impl audits as well | They are deterministic commands, not dispatches; their cost is seconds and their catch history is real (changelog currency) |

## Status history

- 2026-07-31: Proposed
