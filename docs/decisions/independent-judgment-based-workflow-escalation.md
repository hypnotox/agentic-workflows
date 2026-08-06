---
format: current-state-v4
slug: independent-judgment-based-workflow-escalation
status: Implementing
date: 2026-08-06
---
# ADR-independent-judgment-based-workflow-escalation: Independent Judgment-Based Workflow Escalation


## Context

awf currently treats a broad minimal-versus-non-trivial distinction as a proxy for several
different workflow needs. Brainstorming is a hard prerequisite for almost every non-trivial
change. Brainstorming then requires an effort and a fresh-context grounding check. Implementation
paths normally require terminal review, and that review owns effort integration and removal. The
result is coherent for intricate work but makes an easily reasoned localized code, configuration,
documentation, or upgrade change pay for the full chain.

Those mechanisms solve different problems. Brainstorming settles material choices. Efforts preserve
continuity. Grounding validates broad or uncertain repository premises. ADRs retain load-bearing
decisions. Plans make sequencing, coordination, or resumability executable. Independent review
adds assurance when breadth, risk, or non-obvious reasoning makes a second context valuable. Change
size and artifact type are poor substitutes for those needs: a one-line change can embody an
unsettled product choice, while a multi-file mechanical change can be locally obvious and directly
verified.

The existing proportional-simplicity decision already rejects speculative process and a mechanical
simplicity classifier, but stage routing still bundles the process itself. Grounding verified three
additional couplings. Pi's grounding guidance restricts the dedicated tool to post-brainstorm use;
implementation review currently requires effort memory and owns integration, divergence handling,
pending artifact transitions, removal, and retrospective routing; and a docs-only shortcut skips
review by artifact type rather than risk. Repository inspection also showed that effort creation
cannot become independent while brainstorming, debugging, and roadmap graduation own first
creation and the effort support skill accepts existing efforts only.

The replacement must keep repository authority, documentation currency, required verification,
commit gates, ADR and plan review, report-only subagents, and effort-state formats intact. It must
remain a judgment boundary rather than add a classifier, manifest, checklist, or runtime workflow
engine. Upgrades must preserve project-local artifacts rather than silently assign them new standard
semantics.

## Decision

1. `decision: independent-escalation-triggers` Replace the bundled minimal-versus-non-trivial
   routing threshold with independent, judgment-based escalation triggers. Each mechanism is used
   only when its own purpose applies, while repository authority, documentation currency, required
   verification, and commit gates continue to bind every change. Re-evaluate an individual trigger
   when material facts change; activate the newly warranted mechanism before further mutation
   without retroactively invalidating prior valid work.

2. `decision: need-specific-routing` Use brainstorming only for a material choice or clarification
   about behavior, scope, structure, dependencies, patterns, checks, or testing. Use an effort only
   when durable continuity materially helps through multi-step work, likely continuation,
   coordination or delegation, or preservation of settled decisions and observations. Use an ADR
   only for a load-bearing post-implementation commitment or active claim change, and use a plan
   only when sequencing, coordination, or resumability materially helps. No trigger follows merely
   from line count, file count, artifact type, another mechanism firing, or the label non-trivial.

3. `decision: grounding-support-skill` Make grounding a core, reusable support skill callable from
   any workflow when correctness depends on broad or uncertain repository facts, hidden coupling,
   cross-domain effects, unfamiliar architecture, or high-consequence assumptions. It is advisory,
   report-only, single-pass, effort-noncreating, and never a chain prerequisite. It owns the
   self-contained brief, target-native dispatch, spill discipline, and finding classification;
   callers resolve findings. Move the existing grounding-checker pairing and Pi dispatch guidance
   from brainstorming to this skill while preserving the checker, dedicated runtime tool, and
   orienting's ownership of topic orientation and resume revalidation.

4. `decision: effort-lifecycle-single-home` Make the effort support skill the single workflow owner
   of continuity-trigger evaluation, first-creation confirmation and creation, resume, checkpoints,
   managed-worktree context, pending artifact transitions, integration, divergence handling,
   topology removal, retrospective routing, and finish. Other workflows invoke it only when the
   effort trigger fires and may remain effort-free otherwise. Effort-backed finalization always
   returns to this owner after implementation review is either settled or explicitly skipped; a
   divergent integration activates review before removal because it introduces meaningful breadth
   and uncertainty.

5. `decision: risk-based-implementation-review` Make implementation review optional only for a
   locally obvious, low-risk, directly verified change where an independent reviewer is unlikely to
   uncover meaningful hidden consequences. Require it for meaningful breadth, non-obvious logic,
   contract or compatibility effects, migrations, security, concurrency, data-loss risk, or
   verification requiring substantive judgment; uncertainty resolves toward review. Remove the
   docs-only shortcut. Keep ADR and plan artifact review mandatory when those artifacts exist.
   Reviewing implementation owns assurance rather than effort finalization and accepts either
   effort-backed memory evidence or an effort-free self-contained brief containing the outcome,
   user constraints, implementation summary, commit range, and verification results. Effort-free
   review creates no memory, checkpoint, retrospective, or topology work.

6. `decision: guarded-grounding-backfill` Select grounding by default for new adopters. Schema
   generation 37, whose minimum binary version is 0.31.0, enables it only when standard
   brainstorming is enabled, leaves project-local brainstorming untouched, and refuses with
   actionable rename-or-adopt-standard guidance when a project-local grounding artifact collides
   with the new standard name. Never overwrite or reinterpret local artifacts, and keep successful
   reruns idempotent. The ordinary upgrade transaction edits the config selection before rendering
   the added skill, enforces the schema minimum before render, and stamps the resulting manifest at
   generation 37; older trees and binaries retain the existing minimum-version and ascending
   migration protections.

7. `decision: judgment-not-classifier` Express these triggers through concise workflow, skill,
   guide, and review contracts backed by semantic rendering tests. Add no policy schema, classifier,
   manifest, checklist, workflow router, effort-state format, or new runtime mechanism. Preserve the
   existing deterministic gates, `missingkey=zero` behavior, and token-free rendering with unset or
   empty-string variables. Use focused tests to prove ownership, routing, and empty-input rendering
   semantics rather than freeze whole prose passages.

## State changes

- add `rendering/workflow-skill-templates:independent-workflow-escalation`
- update `rendering/workflow-skill-templates:implementer-context-grounding`
- update `rendering/workflow-skill-templates:mandatory-approval-boundaries`
- update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- update `rendering/workflow-skill-templates:unified-effort-workflow-coverage`
- update `rendering/workflow-skill-templates:effort-workflow`
- update `rendering/workflow-skill-templates:memory-log-consumer-coverage`
- update `rendering/workflow-skill-templates:workflow-transitions-advisory`
- update `rendering/workflow-skill-templates:phase-transaction-ownership`
- update `rendering/workflow-skill-templates:explorer-and-grounding-role-contracts`
- update `rendering/guide-and-doc-templates:working-memory-single-home`
- update `rendering/pi-workflows:pi-dedicated-grounding-dispatch`
- add `config/migrations-and-locks:grounding-skill-backfill`

## Consequences

Localized work can proceed directly when its outcome, solution shape, affected boundary, and
verification are already clear. Substantial but bounded work can receive review without requiring
an effort, a clear cross-repository change can use grounding without brainstorming, and a localized
design discussion can brainstorm without paying for grounding. Universal correctness obligations do
not weaken: tests, drift checks, documentation, and commit gates remain independent of workflow
ceremony.

Grounding and effort lifecycle each gain one explicit support-skill home. Brainstorming becomes
smaller, implementation review becomes usable without memory, and effort finalization no longer
exists only as a side effect of review. This increases composability and removes the false coupling
between assurance and continuity. It also makes trigger judgment more visible; conservative fallback
rules for grounding and review mitigate under-escalation, while semantic tests guard the ownership
and routing contracts.

Existing adopters with standard brainstorming receive the new grounding support automatically.
Local brainstorming stays local, and a local grounding name collision fails visibly rather than
changing meaning. Annotated migration tests cover standard and local brainstorming, already-enabled
grounding, the local-name collision, missing configuration, and successful reruns. The migration and
the Pi prompt guidance add implementation work, but neither changes effort data nor adds a new
runtime role or public tool.

The change revises several long-lived workflow claims and tests because earlier decisions encoded
the full-chain assumptions at each stage. That breadth is the cost of making the policy coherent
instead of adding another exception list.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Broaden the existing small-change exception list | Artifact and line-count exceptions remain incomplete and keep unrelated mechanisms bundled. |
| Define three mandatory lanes for minimal, substantial-simple, and complex work | Named lanes are easier to teach but recreate one classifier and cannot express unusual valid combinations. |
| Make only grounding conditional inside brainstorming | Fixes the reported symptom but leaves brainstorming, effort creation, and review coupled to the same proxy threshold. |
| Keep finalization in implementation review and add a no-review mode | Makes an assurance skill own unrelated lifecycle work and turns skipped review into a disguised review invocation. |
| Auto-enable grounding for every existing adopter | Overrides intentionally trimmed selections and does not respect project-local name semantics. |

## Status history

- 2026-08-06: Proposed
- 2026-08-06: Implementing; content-sha256: c826fe36816f2ddcec1d269326183e2bbc70d7d0a4d7ee7f4aaa9ab661139848
- 2026-08-06: Applied; operations: add `rendering/workflow-skill-templates:independent-workflow-escalation`, update `rendering/workflow-skill-templates:implementer-context-grounding`, update `rendering/workflow-skill-templates:mandatory-approval-boundaries`, update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`, update `rendering/workflow-skill-templates:unified-effort-workflow-coverage`, update `rendering/workflow-skill-templates:effort-workflow`, update `rendering/workflow-skill-templates:memory-log-consumer-coverage`, update `rendering/workflow-skill-templates:workflow-transitions-advisory`, update `rendering/workflow-skill-templates:phase-transaction-ownership`, update `rendering/workflow-skill-templates:explorer-and-grounding-role-contracts`, update `rendering/guide-and-doc-templates:working-memory-single-home`, update `rendering/pi-workflows:pi-dedicated-grounding-dispatch`, add `config/migrations-and-locks:grounding-skill-backfill`
