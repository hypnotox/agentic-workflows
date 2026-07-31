---
format: current-state-v2
status: Implemented
date: 2026-07-31
---
# ADR-0197: Trim the rendered skill ceremony

## Context

The rendered workflow skills carry several protocol blocks inlined in full at every
applicable site. The 2026-07-31 ceremony survey measured the repetition in this repository's
rendered corpus: the `AWF_CONTEXT_SPILL_V1` recovery paragraph is included at 14 template
sites, the routine-checkpoint block (roughly 380 words, with a roughly 300-word
approval variant) at 10 sites plus 2 approval-variant sites, and the
coverage-regression reminder at 3 sites. All of these are
already template partials: template-level maintenance is single-sourced, so the cost is not
drift risk but rendered context weight, paid on every skill invocation in every adopter
repository. The trade this decision makes is explicit: in-context availability of rarely
exercised or deterministically backstopped prose is exchanged for a smaller always-loaded
surface, with each displaced protocol keeping exactly one canonical rendered home.

Review ceremony has an analogous unconditional cost. ADR-0074 conditioned the verify pass
on fixes having been applied: the four reviewing skills (reviewing-adr, reviewing-plan,
reviewing-plan-resync, reviewing-impl) render it as the step after fix application, so
every review that applied fixes dispatches one, doubling the reviewer dispatch even when
the applied fixes were a single mechanical one-liner. This decision narrows that contract
rather than replacing it. The recorded firing history of verify passes in
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
- Three further `Backing: test` claims have proofs that pin literals on surfaces this
  decision touches while their claim prose survives unchanged, so no operation is declared
  for them and their proofs are rewritten in the same transaction:
  `rendering/workflow-skill-templates:orienting-single-home` and
  `rendering/guide-and-doc-templates:working-memory-single-home` (both pin the orienting
  resume-revalidation literal touched by item 4's co-edits) and
  `rendering/workflow-skill-templates:implementer-role-contract` (its unset-fallback proof
  pins the reviewing skills' scope-fallback literal item 5 rewords). Likewise
  `tooling/effort-management:memory-skeleton-purpose-partition` survives item 4 unchanged:
  the scaffolded skeleton's placeholder text is a co-edited surface, not a claim change.
- The Conventional-Commits scope list is restated in the four reviewing skill templates
  while `awf check commit` already rejects a wrong scope deterministically at commit time;
  the coverage-regression reminder is restated in three task skills while the 100%-coverage
  gate already fails the build on any regression. Repetition that has no deterministic
  backing (the never-`--amend` rule, doc-currency reminders) is deliberately kept.

## Decision

1. The `AWF_CONTEXT_SPILL_V1` recovery contract moves to one canonical rendered home: a new
   named `### Context spill notices` subsection under `## Commands` in
   `templates/docs/working-with-awf.md.tmpl`, created by this decision (the template edit
   is a same-commit surface; the subsection lives inside the doc's `commands` part, not as
   a part of its own).
   Every managed context-calling skill and the grounding-checker agent body replace the
   expanded paragraph with a single sentence naming the spill notice and pointing at that
   home. The spill partial becomes the pointer sentence; the full contract text renders
   only in the doc.
2. The routine-checkpoint partial compresses to a four-step digest: classify the outcome
   (a minimal simple fix uses no effort, reaching a checkpoint never creates one); validate
   the one immutable slug and owned `.awf/efforts/<slug>/memory.md` path (confirming the
   `Effort: <slug>` first line, continuing in the effort's managed worktree when one
   exists) and update the memory file in its own writer-owned batch, setting `Phase:` and
   `Next:`, appending one `## Handoff log` line and any unrecorded settled decision and
   observation, and refreshing `Updated:`; decide whether user attention is required using
   the existing trigger list; then raise a check-in and stop, or state a continuity notice
   and continue. Element dispositions are explicit: the field set, slug confirmation, and
   worktree continuation stay in the digest; the continuity notice keeps its exact
   slug-and-owned-path clause, and the closing "mechanical corrections and
   authority-determined implementation details stay autonomous" sentence stays in the
   digest's final step (both are behaviour, not ceremony); repository-authority precedence
   and the one-writer/report-only-child contract move to the pointer (the workflow doc's
   working-memory section already states both); the Pi handoff branch renders unchanged. The digest points at the workflow doc's working-memory
   section for the file skeleton, ground rules, and the full protocol, which remain
   rendered there in full. The approval-variant partial compresses the same way while
   keeping its stop-is-the-protocol clause and its full step 4 (rejection-revision loop,
   post-approval persistence, continuation) in compressed form.
3. The four reviewing skills dispatch the verify pass conditionally, narrowing ADR-0074's
   after-fixes rule: a review that applied no fixes leaves the verify-pass trigger unfired,
   which this decision makes explicit in the rendered step; a fix round whose applied fixes
   are all classified `mechanical` skips the verify pass and records the skip and its
   ground in the summary the skill presents at its check-in; a fix round with at least one
   fix classified `reasoned`, or applied under a `user-decision` ruling, dispatches exactly
   one verify pass as today. Finding classification vocabulary is unchanged.
4. The working-memory protocol narrows the `Record:` requirement to material decisions: an
   entry whose decision changes scope, design, authority, or previously-approved output
   carries the indented verbatim `Record:` block; any other user-provenance entry is a
   plain decision-log entry without one. Reviewer briefs paste user entries verbatim
   including whatever `Record:` blocks exist. The co-edited surfaces are the workflow doc
   template, the review-spine-head partial, the three reviewer-skill paste instructions,
   the orienting skill's resume-revalidation enumeration, and the binary-scaffolded memory
   skeleton's `## Decision log` placeholder in `internal/effort/memory.go` with its golden
   test, so a fresh effort's first surface states the narrowed rule.
5. Gate-duplicated prose is deleted: the four reviewing skills' restated scope list is
   replaced by a publication-safe reference to the project's commit scope conventions, and
   the coverage-regression reminder partial and its three inclusions are removed.
   Repetition without deterministic backing (never `--amend`, doc-currency reminders) is
   kept unchanged.
6. `templates/docs/working-with-awf.md.tmpl` includes `templates/partials/model-selection.md`
   instead of hand-inlining its text; the rendered prose stays semantically identical
   within the same paragraph, with source line wrapping reflowed by the include, so golden
   updates are expected rather than a surprise.
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
- The item 5 deletions ship to every adopter, including one whose repository wires neither
  a commit-message gate nor a coverage gate; such an adopter loses a reminder, not a
  control, since nothing enforced either rule there before. Named and accepted by user
  decision; the replacement scope reference stays publication-safe and asserts no gate.
- An adopter that overrides the working-with-awf doc's `commands` part replaces the spill
  contract's only rendered home while every context-calling surface still points at it;
  such an override must carry the contract itself, the same responsibility ADR-0157
  established for full-replacement chain-section parts.
- The four updated claims narrow with their proofs across the reviewed implementation
  series: the spine and chain tests that expand the spill contract and enumerate checkpoint
  elements are rewritten against the digest and pointer forms in the batches that change
  those surfaces, and the final operation lands with the terminal status flip.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Configurable verbosity (a var or per-target profile selecting expanded or pointer form) | Two rendered shapes to test and keep publication-safe for every affected surface; a doubled drift and review surface for a one-time trim |
| Compress the model-selection dispatch sites further | Reverts ADR-0190's week-old pinned design; the compressed-rule-plus-pointer shape is the settled optimum |
| Move the spill contract into the agent guide instead of the doc | The guide is the always-loaded surface ADR-0157 deliberately slimmed; a rare-path contract belongs in an on-demand doc |
| Drop the verify pass entirely | Its recorded catch history on reasoned fixes is real; conditioning keeps the protection where it fires |
| Skip the verify pass by diff size instead of fix classification | Size is a poor proxy (a one-line reasoned fix can regress); the classification already exists and names the risk directly |
| Drop the `Record:` block entirely | Verbatim wording is the consensus-adherence anchor for material decisions; the materiality threshold keeps it where reviewers use it |
| Condition the two reviewing-impl audits as well | They are deterministic commands, not dispatches; their cost is seconds and their catch history is real (changelog currency) |

## Status history

- 2026-07-31: Proposed
- 2026-07-31: Implementing; content-sha256: c03278186c4502f274e5bc028a33c0c39206de39be58e1f6a236d68301afb80e
- 2026-07-31: Applied; operations: update `rendering/workflow-skill-templates:implementer-context-grounding`, update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`, update `rendering/workflow-skill-templates:unified-effort-workflow-coverage`
- 2026-07-31: Applied; operations: update `rendering/workflow-skill-templates:memory-log-consumer-coverage`
- 2026-07-31: Implemented; content-sha256: c03278186c4502f274e5bc028a33c0c39206de39be58e1f6a236d68301afb80e
