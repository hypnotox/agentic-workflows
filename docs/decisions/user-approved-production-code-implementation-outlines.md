---
format: current-state-v4
slug: user-approved-production-code-implementation-outlines
status: Implementing
date: 2026-08-11
---
# ADR-user-approved-production-code-implementation-outlines: User-Approved Production-Code Implementation Outlines


## Context

ADR-0243 made workflow triggers independent and judgment-based: brainstorming fires for a material
choice, while an understood direct change may proceed without it. That boundary preserves
proportionality, but it also lets a straightforward production-code task reach implementation
without the user seeing the intended structure, dependencies, or verification approach. A change
can therefore begin as a one-shot implementation even though a concise outline would have exposed
an unsuitable boundary before code or preparatory tests made it concrete.

ADR-0232 already requires brainstorming to settle a proportionate simplicity contract when it
fires, and the brainstorming workflow scales that presentation from a few sentences to a fuller
comparison. The missing behavior is not another design owner. It is a stronger intake rule for
hand-authored production code, using the existing brainstorming owner and its grounded-design
approval boundary.

ADR-0152 established two routine approval stops, after grounded design and after ADR review.
ADR-0240 preserved both while making implementation autonomous inside approved authority. Requiring
an implementation outline before ADR authoring changes that ordering: the outline must carry the
user's design authority into later artifacts, and artifact authoring and review must not become a
second routine consent stop for the same boundary. ADR-0264 separately requires every ADR
commitment to have prior user acceptance; moving the outline first supplies that consent rather
than weakening it.

## Decision

1. `decision: approve-production-code-outline` Before any hand-authored production-code change,
   present a proportionate implementation outline and obtain explicit user approval. This includes
   mechanical production refactors and tests written to prepare the production change. Scale the
   outline to the work rather than treating a straightforward change as a reason to omit it.

2. `decision: preserve-nonproduction-autonomy` Keep documentation-only work, test-only maintenance
   that does not prepare a production change, generated-output-only work, and non-code mechanical
   work autonomous unless another independently evaluated workflow trigger requires user input.

3. `decision: approve-before-artifacts-once` Complete outline approval before authoring an ADR or
   plan for the production change. That approval is the sole routine user checkpoint for the
   approved boundary: ADR authoring and review, plan authoring and review, implementation, and
   implementation review continue autonomously afterward. Return to the user only for a new
   material decision, a change to the approved boundary, or a blocker or safety or correctness
   concern that cannot be resolved within existing authority.

4. `decision: keep-brainstorming-as-single-owner` Broaden the existing brainstorming workflow to
   own both concise implementation outlines and fuller material-choice design work. Do not add a
   parallel outline skill. The existing proportionate design contract remains the semantic shape
   of the approved boundary.

5. `decision: retain-outline-approval-evidence` Accept retained conversation, a user-provenance
   effort Decision-log entry, or an explicit request to execute a named existing plan whose
   Architecture summary supplies the outline as approval evidence. A delegated implementation
   owner receives the approved boundary from its parent rather than asking for approval again.

## State changes

- update `rendering/workflow-skill-templates:independent-workflow-escalation`
- update `rendering/workflow-skill-templates:mandatory-approval-boundaries`
- update `rendering/workflow-skill-templates:authority-guided-review-remediation`

## Consequences

Users see the intended production-code shape before implementation artifacts or preparatory tests
narrow the solution. Straightforward changes incur a concise approval round, while complex changes
retain the fuller design comparison already owned by brainstorming. Mechanical production refactors
are no longer an exception, because they can still encode structural choices.

The workflow loses settled-ADR approval as a routine stop. ADR review still checks consent,
minimum-sufficient scope, and active authority, but it resolves authority-preserving findings
without asking the user to approve the artifact again. A genuinely new decision or changed boundary
returns through brainstorming before it can enter the ADR, preserving ADR-0264's prior-consent rule.

Nonproduction maintenance retains its current autonomy. The distinction between test-only
maintenance and a test that prepares a production change requires contextual judgment, as does the
size of a proportionate outline. Existing review, authority, and verification obligations mitigate
misclassification; no policy schema or runtime classifier is added.

Approval evidence remains lightweight and compatible with effort-free work. An explicit request to
execute a named plan can reuse its Architecture summary, while delegated owners reuse the boundary
their parent supplies instead of introducing an unreachable approval interaction.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Add a dedicated implementation-outline skill | It would duplicate brainstorming's design ownership and introduce another workflow stage for the same semantic boundary. |
| Distribute the checkpoint across planning and execution workflows | It would duplicate design policy and approval semantics across several owners instead of keeping one proportionate design boundary. |
| Approve the outline after ADR authoring | Higher-altitude design choices could already be embedded in the ADR before the user accepts them, conflicting with ADR-0264. |
| Keep mandatory settled-ADR approval as well | It would create two routine approvals for one already approved boundary rather than making later artifact work autonomous. |
| Exempt mechanical production refactors | Mechanical edits can still commit the repository to an unsuitable structure, so their implementation shape must be visible first. |

## Status history

- 2026-08-11: Proposed
- 2026-08-11: Accepted; content-sha256: 2f9db939ea4405e0e24591e689d2ec4e0fe18e628db4383e8234a03c43c0669f
- 2026-08-11: Implementing; content-sha256: 2f9db939ea4405e0e24591e689d2ec4e0fe18e628db4383e8234a03c43c0669f
- 2026-08-11: Applied; operations: update `rendering/workflow-skill-templates:independent-workflow-escalation`
- 2026-08-11: Applied; operations: update `rendering/workflow-skill-templates:mandatory-approval-boundaries`
- 2026-08-11: Applied; operations: update `rendering/workflow-skill-templates:authority-guided-review-remediation`
