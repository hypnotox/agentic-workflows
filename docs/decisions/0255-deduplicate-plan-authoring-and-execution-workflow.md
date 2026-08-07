---
format: current-state-v4
slug: deduplicate-plan-authoring-and-execution-workflow
status: Implemented
date: 2026-08-07
---
# ADR-0255: Deduplicate Plan Authoring and Execution Workflow


## Context

awf independently selects plans when sequencing, coordination, or resumability materially helps.
Once selected, however, the plan contract carries both change-specific implementation direction and
generic workflow protocol. The writing guidance asks authors to predict exact symbols, paths,
commands, branches, ordering, failure behavior, tests, semantic-review choreography, staging, gates,
commit handling, and checkpoint behavior. Execution skills and the implementer contract then state
many of those same mechanics again. Plans consequently become expensive to author, review, and keep
current even when repository authority can determine the local implementation detail safely.

Recent work makes the cost visible. A warm-cache `./x gate timings` run took about 55 seconds, with
40 seconds in the Go suite and 8 seconds in the Pi runtime smoke. Recent feature histories also carry
separate plan proposal and correction commits, ADR review and resync transactions, phase review,
terminal implementation review, lifecycle closure, and broad generated-output updates. The history
does not prove exact wall-clock or model-token cost, but it does show repeated transactions and
repeated proof of the same properties.

The separate plan-to-ADR resync skill was introduced by ADR-0028 to expose a real convergence step.
The current model now provides stronger authority boundaries: plan-v2 links typed ADR Decisions,
plan review reads current-state authority, ADR changes remain inside the ADR lifecycle, and
implementation autonomy forbids changing approved outcomes or durable boundaries. A plan must never
resolve a contradiction by departing from a linked ADR. An always-on second plan review is therefore
not the only way to preserve convergence; freshness can be owned by ordinary plan review whenever a
linked ADR changes.

The simplification must retain one workflow, not add lightweight and governed profiles or any other
workflow-depth knob. It must also preserve phase-level green transactions, every-commit gates,
report-only independent assurance when risk warrants it, current-state authority, documentation
currency, and append-only ADR history. Existing plan-v1 and plan-v2 documents remain valid; this
change narrows what new plans must repeat rather than invalidating historical plans.

## Decision

1. `decision: change-specific-plan-ownership` A plan owns only change-specific execution facts:
   approved outcome and architecture boundary, ordered phases, per-phase execution ownership,
   observable task results, governing Decision references, scope confinement where ambiguity or
   helper ownership requires it, focused checks that produce new evidence, phase commit subjects,
   and cross-phase outcome ownership. Workflow skills, reviewer and implementer agents, and
   repository authority own generic context loading, staging, gates, clean-tree handling, commits,
   review routing, recovery, checkpoints, and model selection. Generated authoring guidance states
   each generic rule in its semantic home rather than requiring plans to copy it. Every affected
   template and generated target retains coherent missingkey-zero rendering under empty variables
   without `<no value>` or another unresolved token.

2. `decision: authority-resolved-local-detail` A fresh phase owner receives a self-contained
   change-specific plan projection plus repository and current-state authority, not a speculative
   transcript of every local implementation choice. A commit-capable phase owner may resolve local
   symbols, helper structure, test arrangement, and necessary omitted paths when the resolution
   preserves the approved outcome, material scope, durable boundaries, dependency direction, and
   verification strength. Commit-disabled helpers remain confined to assigned paths. A spike remains
   appropriate for uncertainty that can change those boundaries, but authority-determined local
   detail does not require a separate spike or plan amendment merely because it was unknown at
   authoring time.

3. `decision: proportionate-plan-fields` Keep structured plan fields whose values drive typed
   projection, authority resolution, phase ownership, outcome ownership, or path confinement.
   `Applying`, `Context`, execution mode, phase close, `Advances`, `Completes`, and necessary `Paths`
   retain those roles. Treat `Latitude`, batch kind, representative and edge examples, generic gate
   prose, and generic semantic-review choreography as optional authoring aids rather than mandatory
   data for every qualifying task. Historical plan grammar remains accepted. A globbed or otherwise
   ambiguous population still carries a deterministic focused check that proves its terminal state;
   the simplification removes duplicated protocol, not evidence needed to bound a bulk change. The
   implementation phase owner is the single owner of focused generated-prose meaning review and
   records the inspected output boundaries and result in its completion evidence. A plan supplies
   change-specific examples and expected readings only when they are load-bearing; plan and code
   reviewers inspect the requirement and its implementation evidence rather than restating the
   execution choreography.

4. `decision: precommit-plan-review` Review a newly written plan before its first commit. Apply
   mechanical findings directly without a durable review ledger; record substantive reasoned or
   user-decided findings and their dispositions in plan Notes, then create one settled initial plan
   commit. Later substantive corrections remain new commits. The reviewer stays report-only and the
   single verify-pass bound remains. This preserves meaningful review evidence without manufacturing
   correction commits solely because the draft was committed before its first independent reading.

5. `decision: linked-plan-review-freshness` Remove the separate plan-resync skill and chain node.
   Full plan review verifies every ADR resolved from the plan's typed `adrs:` links. A substantive
   amendment or review correction to an ADR invalidates prior review of every linked Proposed plan;
   review the ADR first, then review each affected plan against the updated decision. A plan finding
   that would contradict an ADR returns to ADR amendment and review before the plan changes. If
   implementation has begun, reassess completed affected phases and renew implementation assurance
   where the changed decision can affect landed work. `internal/project` owns reverse ADR-to-plan
   association over the already-parsed plan and ADR corpora; `awf context` exposes the deterministic
   linked-plan set for an explicit ADR request so review workflows consume typed identities without
   reparsing Markdown or guessing from modification time or session implication. Removing the
   standard resync skill also adds one schema migration that removes its selection from existing
   configs before catalog validation, reports the removal, and preserves every unrelated config
   value. This freshness rule replaces always-on resync without permitting plan-to-ADR drift.

6. `decision: freshness-scoped-assurance-reuse` Make delegated phase review explicit assurance
   evidence scoped to the reviewed phase commit and supplied deviation report. A later terminal
   review does not repeat already-covered phase correctness: for a single-phase implementation it
   covers unreviewed settlement or integration changes, and for multi-phase work it focuses on
   cross-phase, settlement, and integration consequences. Required range audits still run over the
   final implementation range, and divergence, post-review reasoned fixes, changed authority, or
   other material later mutations renew the affected assurance. Phase review never silently
   substitutes for evidence its contract did not collect.

7. `decision: one-workflow-no-depth-controls` Ship the simplification as the one awf workflow. Do
   not add profiles, workflow-depth configuration, classifiers, routers, or runtime policy knobs.
   Existing universal authority, documentation, verification, commit, approval, and lifecycle
   obligations remain. Every actual commit still passes its configured staged checks and full gate;
   plans omit repeated generic gate instructions but do not weaken the gate boundary. Back every
   invariant added or updated by this decision with named proof annotations in the implementing
   transaction's tests. Every `add` operation below declares an invariant with `Backing: test`.

## State changes

- update `rendering/workflow-skill-templates:implementer-context-grounding`
- update `rendering/workflow-skill-templates:authority-guided-review-remediation`
- update `rendering/workflow-skill-templates:memory-log-consumer-coverage`
- update `rendering/workflow-skill-templates:phase-transaction-ownership`
- update `rendering/workflow-skill-templates:plan-task-detail-modes`
- remove `rendering/workflow-skill-templates:workflow-chain-surfaces-resync`
- update `rendering/workflow-skill-templates:semantic-rendering-review`
- add `rendering/workflow-skill-templates:plan-review-before-first-commit`
- add `rendering/workflow-skill-templates:linked-plan-review-freshness`
- add `rendering/workflow-skill-templates:single-workflow-no-depth-controls`
- add `config/migrations-and-locks:retired-plan-resync-selection-migration`
- add `tooling/context-and-topic:adr-linked-plan-references`

## Consequences

Plans become shorter and cheaper to write because they state the facts unique to the change rather
than reproducing the execution platform. Fresh owners still receive governing Decisions, current
state, explicit phase ownership, bounded scope where needed, and observable outcomes. Local source
reality can correct stale detail without turning the plan into a second implementation language.

The chain loses one always-on fresh-context dispatch. ADR-to-plan convergence instead becomes a
freshness property of ordinary plan review. This is simpler in the common case but makes linked-plan
discovery and post-start reassessment load-bearing; typed links, explicit invalidation, and renewed
assurance mitigate the risk of an amended ADR leaving already-reviewed or already-implemented work
stale.

Initial plan history becomes less noisy because review precedes the first commit. Mechanical draft
corrections are no longer observable as separate commits. Substantive findings remain visible in
Notes, and all later corrections retain ordinary commit history.

Removing a standard skill affects more than its template. The config consumer removes an existing
selection through the schema migration before validation and preserves unrelated values; the render
consumer omits the retired artifact and its dependency edges; the manifest consumer prunes its
tracked outputs through the ordinary regenerated output plan; and the migration consumer performs
and reports the one idempotent retirement step. The catalog, workflow tests, generated Claude and Pi
outputs, source parts, glossary and current-state prose, and lock manifest move together. Projects
without the retired selection and all unrelated configuration remain backward compatible. No adopter
chooses between old and new workflows.

Assurance becomes freshness-scoped rather than layer-counted. Phase evidence can prevent a duplicate
terminal reading, but audits and review of unreviewed settlement, cross-phase, integration, or
post-review changes remain. The workflow must define and test those coverage boundaries explicitly;
otherwise reuse would be an assurance reduction rather than deduplication.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the current plan contract and only shorten its prose | The duplicated responsibilities and speculative authoring requirements would remain even if expressed in fewer words. |
| Keep resync but make it conditional | Ordinary plan review can own linked-ADR freshness directly; retaining a second skill preserves catalog, migration, routing, and cognitive overhead without unique authority. |
| Remove resync with no replacement freshness rule | An ADR amended after plan review could leave the plan or completed phases stale. Typed linked-plan invalidation preserves the necessary guarantee. |
| Skip plan review entirely | Independent review finds real scope, authority, and verification gaps. Reviewing the draft before its first commit removes churn without discarding assurance. |
| Let phase review replace all terminal assurance | Current phase review does not necessarily cover range audits, settlement commits, cross-phase composition, or integration effects. Reuse must remain explicit and freshness-scoped. |
| Add lightweight and governed workflow profiles | Multiple operating modes create configuration, documentation, migration, and user-choice burden. awf owns one workflow and improves it directly. |

## Status history

- 2026-08-07: Proposed
- 2026-08-07: Implementing; content-sha256: 053c394ffd48cb926a2a125babb491a46da7bc393e88b96871e39f2e2da6ff1e
- 2026-08-07: Applied; operations: add `tooling/context-and-topic:adr-linked-plan-references`
- 2026-08-07: Applied; operations: update `rendering/workflow-skill-templates:phase-transaction-ownership`, update `rendering/workflow-skill-templates:plan-task-detail-modes`, update `rendering/workflow-skill-templates:semantic-rendering-review`, add `rendering/workflow-skill-templates:plan-review-before-first-commit`
- 2026-08-07: Reapplied; operations: update `rendering/workflow-skill-templates:semantic-rendering-review`
- 2026-08-07: Applied; operations: update `rendering/workflow-skill-templates:implementer-context-grounding`, update `rendering/workflow-skill-templates:authority-guided-review-remediation`, update `rendering/workflow-skill-templates:memory-log-consumer-coverage`, remove `rendering/workflow-skill-templates:workflow-chain-surfaces-resync`, add `rendering/workflow-skill-templates:linked-plan-review-freshness`, add `rendering/workflow-skill-templates:single-workflow-no-depth-controls`, add `config/migrations-and-locks:retired-plan-resync-selection-migration`
- 2026-08-07: Reapplied; operations: add `config/migrations-and-locks:retired-plan-resync-selection-migration`
- 2026-08-07: Reapplied; operations: add `config/migrations-and-locks:retired-plan-resync-selection-migration`
- 2026-08-07: Implemented; content-sha256: 053c394ffd48cb926a2a125babb491a46da7bc393e88b96871e39f2e2da6ff1e
