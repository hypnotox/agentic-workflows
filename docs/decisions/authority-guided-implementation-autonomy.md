---
format: current-state-v4
slug: authority-guided-implementation-autonomy
status: Proposed
date: 2026-08-06
---
# ADR-authority-guided-implementation-autonomy: Authority-Guided Implementation Autonomy


## Context

ADR-0232 established a proportionate simplicity contract so users retain control over material
scope and design while agents retain autonomy over equivalent mechanical choices. Its execution
rule nevertheless treats every newly discovered effect on behavior, scope, structure, dependencies,
patterns, checks, or testing strategy as an approval boundary. The rendered implementation guidance
projects that broad stop across direct changes, test-driven work, bug fixes, plan execution, and
fresh-context implementers.

That rule conflates invalidating an approved outcome or durable boundary with correcting an
implementation detail. Source inspection routinely reveals stale paths, inaccurate diffs, unsuitable
local mechanics, missing edge handling, or a better authority-preserving way to realize the same
approved design. Review and verification likewise surface work to resolve, but their mere failure or
severity does not create a design fork. Requiring approval for every such finding interrupts
implementation without transferring a meaningful choice to the user.

The existing authorities already provide a stronger distinction. ADRs and current-state claims bind
durable decisions and active rules. A grounded design binds the approved outcome, material scope,
and settled structural boundaries. Proposed plans remain mutable through implementation under
ADR-0097, with execution findings recorded before they freeze. Required verification remains
non-negotiable. Together these authorities can determine many reasoned corrections without treating
the implementer as a second planner or permitting unrelated cleanup.

Delegation adds an ownership constraint. An inline owner can correct a mutable plan as soon as source
reality invalidates a detail. A fresh-context implementer owns its dispatched implementation
transaction, not the parent's plan or working memory. It needs permission to finish a compliant
reasoned deviation and a precise reporting duty so the parent can reconcile the plan before later
execution consumes stale instructions.

## Decision

1. `decision: authority-guided-autonomy` Make authority-guided resolution the default throughout
   implementation. An agent continues autonomously when a reasoned change complies with applicable
   ADRs, current-state claims, and repository authority; preserves the approved outcome, material
   scope, and settled durable boundaries; carries a defensible rationale; and can satisfy required
   verification. This rule applies to direct execution, test-driven work, bug fixes, plan execution,
   delegated implementation, review remediation, and implementation checkpoints.

2. `decision: narrow-escalation-boundary` Require user escalation only when applicable authorities
   conflict or must change, the approved outcome or material scope must change, a genuine design fork
   has no authority-determined answer, safe or correct completion is impossible within the approved
   boundary, or required verification remains unreachable after reasonable diagnosis and
   remediation. Discovery of a source contradiction, correctness or safety concern, review finding,
   blocker symptom, or failed check is not itself an escalation condition.

3. `decision: reasoned-detail-deviation` Permit an implementation owner to depart from plan details
   when the departure satisfies the authority-guided boundary. This autonomy does not permit
   replanning the approved outcome, broadening material scope, overturning a settled structural
   choice, weakening an oracle, or performing unrelated cleanup. Reasoned judgment, rather than only
   equivalent mechanical choice, is autonomous inside those limits.

4. `decision: truthful-plan-reconciliation` Keep a mutable plan truthful according to ownership. An
   inline plan owner corrects a stale instruction, records the reasoned deviation in the plan's
   Notes section, and continues. A delegated implementer may finish without editing the plan but
   reports every deviation with its rationale, governing authority, and verification. The parent
   supplies that report to phase review, then reconciles the plan and review findings in a focused
   settlement commit before checkpointing or later execution. This preserves one delegated
   phase-closing commit while preventing subsequent work from relying on stale instructions.

5. `decision: issue-resolution-before-escalation` Treat correctness, safety, review, recovery, and
   verification findings as implementation work whenever existing authority determines a compliant
   remedy. Mechanical and reasoned review findings remain autonomously actionable regardless of
   severity. Dirty-state recovery remains explicit and safe. User attention is required only when
   remediation reaches the narrow escalation boundary.

6. `decision: judgment-without-policy-schema` Keep the classification in one shared prose partial
   included directly by each applicable implementation consumer, so one semantic home prevents
   cross-path drift without introducing nested composition. Do not add a policy schema, deviation
   ledger, approval artifact, command, linter, or automated semantic classifier. Retain the existing
   mandatory approvals for effort creation, grounded design, and settled ADR review, as well as plan
   freezing, phase ownership, report-only review, and required verification. Back the shared rule
   with an invariant proof in the existing template-contract test suite that verifies projection to
   every intended consumer. Affected templates retain missingkey=zero behavior and coherent generic
   rendering with empty variables, without unresolved no-value tokens.

## State changes

- add `rendering/workflow-skill-templates:authority-guided-implementation-autonomy`
- update `rendering/workflow-skill-templates:mandatory-approval-boundaries`
- update `rendering/workflow-skill-templates:maintainable-code-subagent-contract`
- update `rendering/workflow-skill-templates:implementer-role-contract`

## Consequences

Agents can correct stale plan details, follow source-backed implementation reality, and resolve
review or verification findings without repeatedly asking users to approve choices already bounded
by repository authority. Users keep control over durable decisions, the approved outcome, and
material scope rather than becoming an approval queue for ordinary implementation judgment.

Plans become more truthful during execution. Inline correction happens before dependent work.
Delegated correction remains compatible with a fresh-context owner's narrow artifact authority: the
child reports deviations, phase review sees them, and the parent reconciles the plan in settlement
before the checkpoint or next phase. This accepts additional ceremony for delegated deviations:
the child's structured report becomes review input, and the parent must land a focused settlement
transaction before continuing.

The boundary remains judgment-based. An agent may misclassify a material design change as a detail,
or escalate too cautiously. Explicit preservation criteria, structured deviation reports,
independent review, and required verification mitigate that risk. Consensus review continues to
escalate actual deviations from settled user decisions; plan-detail latitude does not erase the
approved design.

Some implementation findings will still stop work, but only after diagnosis establishes that no
safe authority-preserving path remains. Gates, assertions, goldens, fixtures, and other oracles stay
non-negotiable. The decision changes when user input is needed, not what correctness requires.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep approval for every affected design category | It is more conservative against an agent misclassifying material drift, but treats stale implementation detail and authority changes alike. Explicit authority criteria, deviation reporting, review, and verification provide proportionate control with fewer meaningless interruptions. |
| Permit only equivalent mechanical deviations | It remains too narrow for source-backed corrections that require judgment while preserving every approved boundary. |
| Let delegated implementers amend the parent plan directly | It expands fresh-context ownership and couples the phase transaction to a parent-owned durable artifact unnecessarily. |
| Formal deviation taxonomy or machine classifier | Semantic materiality is contextual; a schema adds ceremony and false precision without deciding the hard cases. |
| Reconcile delegated deviations in a separate parent commit before review | It keeps the plan current for review, but splits reconciliation from review findings and makes the reviewer inspect a parent interpretation instead of the child's original deviation report. One post-review settlement handles both coherently. |
| Reconcile delegated deviations before review by rewriting the child commit | It preserves one commit but obscures delegated ownership and discards the useful review-before-settlement sequence. |

## Status history

- 2026-08-06: Proposed
