---
format: current-state-v4
slug: user-approved-adr-decision-boundaries
status: Implementing
date: 2026-08-11
---
# ADR-0264: User-Approved ADR Decision Boundaries


## Context

ADR-0224 established that Decision items contain durable commitments rather than implementation
directives, but its post-implementation and counterfactual tests do not prevent unnecessary semantic
strengthening. A plausible output shape, guarantee, ownership refinement, or constraint can survive
implementation and still be an additional decision that the user never made. An agent can therefore
turn three approved choices into six related commitments, leaving review approval to detect and
unwind the additions.

ADR-0245 reserves user-decision review findings for corrections that would change settled authority
and otherwise permits authority-preserving reasoned remediation. Its consensus-adherence boundary
assumes a deviation from recorded user authority, but does not distinguish removing an unapproved
surplus commitment from changing an approved one. Review dispatch also carries user-provenance
Decision-log evidence when an effort exists but leaves the check idle for effort-free approved
designs.

The missing boundary is consent, not another prose classifier. Repository facts can establish what
is true or constrained, and engineering judgment can select delegated implementation details, but
neither establishes that the user accepted another load-bearing commitment.

## Decision

1. `decision: require-prior-user-acceptance` Put a load-bearing commitment in an ADR only after the user has explicitly accepted that decision. The approved decision set is closed: relatedness, usefulness, repository facts, and architectural reasoning do not authorize additions. An agent may propose another decision outside the artifact, but may not insert it and rely on later ADR approval for retroactive consent.

2. `decision: keep-minimum-sufficient-semantics` State the narrowest durable commitment that preserves the accepted semantics. Do not strengthen it with a representation, guarantee, mechanism, constraint, exclusion, or other detail that could vary without violating the accepted decision. Route such implementation detail to a plan or direct execution; retain a mechanism in the ADR only when the user accepted the mechanism itself as load-bearing.

3. `decision: ground-adherence-in-consent-evidence` Ground decision adherence in the effort memory's user-provenance Decision-log entries and especially their `Record:` transcript evidence when an effort exists. For effort-free work, use the explicitly approved conversational design summary. Treat missing or insufficient evidence as no basis to infer consent, while keeping efforts optional.

4. `decision: review-adherence-and-scope-separately` Give universal ADR review distinct decision-adherence and ADR-scope lenses. Adherence compares every commitment with consent evidence and detects contradictions, semantic strengthening, and new decisions. Scope applies the minimum-sufficient boundary and detects incidental implementation choices or unnecessary constraints.

5. `decision: remove-surplus-and-disclose-refinements` Treat removal of an unauthorized surplus commitment as an authority-preserving reasoned correction, disclose the removal in the settled-ADR approval summary, and raise any worthwhile replacement only as a separate suggestion requiring acceptance before insertion. Keep a correction that would change accepted semantics classified as a user decision. Permit semantics-preserving reasoned wording refinements and disclose them in the same summary.

## State changes

- update `rendering/templates:decision-artifact-routing`
- update `rendering/workflow-skill-templates:authority-guided-review-remediation`
- update `rendering/workflow-skill-templates:memory-log-consumer-coverage`

## Consequences

ADR authors gain a pre-mutation boundary: either a commitment is supported by explicit consent
evidence or it stays outside the record. Users can still invite suggestions without having to find
and unwind speculative decisions embedded in a completed draft.

ADRs remain semantic rather than exhaustive. Plans and direct execution retain delegated detail,
and effort-free work remains valid. Review must carry the approved conversational summary when no
effort memory exists, which makes the dispatch brief more important but does not create a new
durable artifact.

The boundary remains semantic and cannot be proven by natural-language linting. Independent review,
verbatim consent evidence, and mandatory disclosure make deviations visible. Reviewers must
separate removal of authority-free surplus from a correction that changes approved authority; the
shared classification rule keeps that distinction consistent across initial and residual findings.

Reasoned refinements may improve accuracy without another approval interruption, but the final
summary makes them inspectable. A useful surplus idea incurs an explicit suggestion-and-approval
round before entering the ADR rather than gaining authority by appearing in a draft.

Every affected template preserves `missingkey=zero` behavior and renders coherent, token-free prose
when optional variables are empty.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Rely on final ADR approval to accept added commitments | It makes consent retroactive and requires the user to detect decisions they never asked the agent to make. |
| Extend only the existing post-implementation and counterfactual tests | A needlessly specific commitment can pass both tests; durability does not establish consent or minimum scope. |
| Require an effort for every ADR | It would create continuity machinery for work that does not need it; an explicitly approved conversational summary supplies the effort-free evidence. |
| Mechanically classify Decision prose | Syntax and keywords cannot distinguish a load-bearing semantic boundary from an incidental implementation choice. |

## Status history

- 2026-08-11: Proposed
- 2026-08-11: Implementing; content-sha256: 2eddf90dc55c84c541fb5f85aa842fa95bcc4d2b4c03c884ed9b5e62077147f1
- 2026-08-11: Applied; operations: update `rendering/templates:decision-artifact-routing`
- 2026-08-11: Reapplied; operations: update `rendering/templates:decision-artifact-routing`
- 2026-08-11: Applied; operations: update `rendering/workflow-skill-templates:authority-guided-review-remediation`, update `rendering/workflow-skill-templates:memory-log-consumer-coverage`
- 2026-08-11: Reapplied; operations: update `rendering/workflow-skill-templates:memory-log-consumer-coverage`
