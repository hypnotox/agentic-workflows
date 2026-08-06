---
format: current-state-v4
slug: authority-guided-review-remediation
status: Implementing
date: 2026-08-06
---
# ADR-0245: Authority-Guided Review Remediation


## Context

Review is intended to improve an artifact or implementation without transferring ordinary
engineering judgment back to the user. The current shared review spine partly supports that intent:
it classifies direct corrections as mechanical, corrections requiring judgment as reasoned, and
only a genuine fork or unresolved ambiguity as a user decision. Consensus adherence separately
requires an actual deviation from a recorded user decision to be escalated.

The dispatching plan, plan-resync, and ADR review skills weaken that boundary after their single
fresh verification pass. They automatically promote every residual structural finding to a
`user-decision`, regardless of whether repository authority determines a clean correction. The
shared classifier is also broad enough for a reviewer to treat ambiguity itself as an approval
request even when no settled choice or active rule is affected. Structural character, severity,
and survival of one verification pass describe a finding; they do not establish that the user owns
a choice.

ADR-0074 deliberately introduced the automatic residual escalation as a stopping rule for the
single-pass review architecture. ADR-0240 later established authority-guided autonomy for
implementation and implementation-review remediation: findings are diagnosed first, and user
attention is reserved for an authority, outcome, scope, safety, or persistent-verification boundary.
Its active claim is implementation-specific, so it does not by itself replace ADR-0074 for plan and
ADR review. The newer direction should extend uniformly across the shared review classification
without weakening independent review, required verification, consensus adherence, or the mandatory
grounded-design and settled-ADR approvals.

Plan resync has one further asymmetry. An initial finding that shows the plan is right and a
still-Proposed ADR is wrong takes the ADR amendment and review return edge, while the same discovery
in verification is automatically escalated. Discovery timing should not change who owns an
authority-preserving correction.

## Decision

1. `decision: authority-deviation-boundary` Reserve `user-decision` review findings for cases where
   every viable correct remediation would contradict or change a settled user-approved design or
   decision, or would require an unauthorized change to an active current-state claim. A newly
   material load-bearing choice outside approved durable boundaries is not escalated merely because
   it is ambiguous: the dispatcher routes it through the existing grounded-design or ADR workflow
   and pauses only at that workflow's mandatory approval boundary before adopting the new authority.
   A reviewer classifying a finding as `user-decision` must cite the affected authority and explain
   the required deviation. When no such authority is affected, the finding is not a user decision.

2. `decision: autonomous-review-judgment` Keep the shared review spine as the single semantic home
   for finding classification. It classifies an unambiguous authority-preserving correction as
   mechanical and a correction requiring judgment as reasoned; dispatching workflows route those
   classifications rather than redefining them. The dispatching workflow applies both autonomously,
   with a concise rationale for reasoned work. Ambiguity, competing clean options, severity,
   structural character, or the fact that a finding survived a prior correction does not
   independently transfer the choice to the user. An inability to complete correctly without
   changing settled authority reaches the authority-deviation boundary.

3. `decision: uniform-residual-routing` Retain exactly one fresh verify-pass dispatch after a review
   applies reasoned fixes or a user-approved ruling. Diagnose every residual finding under the same
   authority-deviation boundary rather than promoting structural findings automatically. Apply an
   authority-preserving mechanical or reasoned residual correction, run the applicable verification,
   and report its disposition without dispatching another same-artifact review loop. Stop only for a
   residual finding that remains a true user decision.

4. `decision: resync-return-edge` Keep plan-resync's ADR amendment and review return edge available
   for both initial and residual findings while the implicated ADR remains amendable. This governed
   edge is an exception to the same-artifact no-loop rule: residual resync ends, the ADR is amended
   and independently reviewed, and a new resync invocation follows under its own one-verify-pass
   bound. Discovery during verification does not create a separate approval boundary.

5. `decision: preserve-review-controls` Keep reviewers report-only, classification rather than
   severity as the routing axis, consensus deviations as user decisions, required gates and
   verification, and the existing mandatory approvals for grounded design and settled ADR review.
   An ADR that intentionally declares an active-claim change is not an unauthorized deviation merely
   because its proposed future state differs from current state; review instead checks it against
   the settled design and its declared State changes. Back the cross-template invariant with an
   annotated `internal/project` test that proves the shared classifier and all four reviewing-skill
   projections retain authority-guided initial and residual routing. Every affected template retains
   missingkey=zero behavior and renders coherently with empty variables without a `<no value>` token.

## State changes

- add `rendering/workflow-skill-templates:authority-guided-review-remediation`

## Consequences

Plan, ADR, resync, and implementation review share one meaningful escalation boundary. Reviewers and
dispatching skills can select and apply careful corrections when the user has not retained the
choice, including after the single verification pass. Users are asked only when proceeding correctly
would change settled authority, rather than because a finding is difficult, structural, ambiguous,
or late.

The fresh verify pass and report-only reviewer boundary remain intact. Outside the governed ADR
amendment and review return edge, a residual correction receives no second independent reviewer
dispatch, so the main thread carries responsibility for diagnosing, applying, verifying, and
reporting it. Required gates and checks mitigate that risk, while the single-pass limit continues to
prevent an unbounded review loop.

The rule remains judgment-based. An agent can mistake a material authority change for an ordinary
reasoned correction. Requiring a cited authority for `user-decision`, retaining consensus-adherence
review, preserving mandatory design and ADR approvals, and independently verifying applied fixes
make the boundary inspectable without introducing a policy schema or automated classifier.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Change only plan review | The classification lives in a deliberately shared spine, and the same authority distinction applies to ADR and implementation findings. Per-reviewer semantics would recreate drift. |
| Keep automatic residual escalation | Verification timing and structural character do not establish user ownership; this is the direct source of unnecessary approval questions. |
| Escalate every apparent design fork | Options inside approved durable boundaries remain delegated detail that the workflow can resolve with rationale; a newly material load-bearing choice outside those boundaries still returns through the existing approval workflow. |
| Add another verify pass after residual correction | It restores a review loop and additional fresh-context cost. The existing single pass plus required verification is the chosen bounded assurance model. |

## Status history

- 2026-08-06: Proposed
- 2026-08-06: Implementing; content-sha256: eb887604b5932abda1cbdee2858805464965927344ef1fcc9b97cfcd7d520e77
- 2026-08-06: Applied; operations: add `rendering/workflow-skill-templates:authority-guided-review-remediation`
