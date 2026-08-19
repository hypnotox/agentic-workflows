---
format: current-state-v4
slug: make-review-enforce-concrete-maintainability-risks
status: Proposed
date: 2026-08-19
---
# ADR-make-review-enforce-concrete-maintainability-risks: Make Review Enforce Concrete Maintainability Risks


## Context

AF-003 established that review protects outcomes and durable authority rather than recorded route
choreography. AF-004 then made clean integration operative in plan and implementation review while
deliberately leaving finding severity and blocking policy unchanged. The resulting reviewers apply
useful ownership, dependency, representation, obsolete-path, and residual-debt lenses, but no shared
contract distinguishes an actionable maintainability defect from a preferred pattern or style.

That omission creates two opposite risks. A reviewer can demand aesthetic restructuring without
naming harm, or it can treat duplicated policy and parallel ownership as optional taste even when
they can diverge. The existing review result model does not solve this distinction: severity is
informational, while mechanical, reasoned, and user-decision classification determines remediation
ownership. Adding an advisory disposition now would broaden that model and preempt AF-013's separate
work on finding severity.

The maintainable-code-design guide remains canonical doctrine. Implementation review and plan review
need one operative threshold for applying that doctrine. ADR review remains outside this decision:
its bounded structural lens evaluates whether a proposed durable choice is coherent, rather than
whether implementation or plan detail creates a concrete maintainability risk.

## Decision

1. `decision: concrete-risk-admissibility` Admit a maintainability concern to the actionable plan-
   or implementation-review findings digest only when it names the implicated semantic owner, the
   affected location, a concrete maintainability risk, the smallest clean remediation, and the
   existing mechanical, reasoned, or user-decision classification. Concrete risk includes future
   divergence, ambiguous ownership, hidden parallel policy, inappropriate dependency,
   representation leakage, a workaround around the wrong model, unbounded debt, or reduced
   verification strength. Pure aesthetic, stylistic, or pattern preference without such risk does
   not become an actionable finding.

2. `decision: finding-evidence-mapping` Preserve the six-field finding schema and informational
   severity. Record the affected location in `location`, the semantic owner and concrete risk in
   `issue`, the smallest clean remediation in `suggested_fix`, and remediation ownership in
   `classification`. Here semantic owner means the owner of the implicated behavior, policy, state,
   representation, dependency, or test seam, not the reviewer or the finding author.

3. `decision: risk-grounded-remediation` Require implementation- and plan-review dispatchers to
   validate maintainability findings against the concrete-risk threshold before acting. Reject a
   risk-free preference as non-admissible rather than creating a new no-action classification;
   choose autonomously among clean local remedies inside approved boundaries; and route a genuinely
   new material choice or changed approved boundary through brainstorming independently of finding
   severity. Only a true authority deviation remains a user-decision finding.

4. `decision: single-operative-contract` Keep one shared operative contract as the semantic owner of
   maintainability-review admissibility and remediation evidence, consumed by the plan reviewer,
   implementation reviewer, and their dispatching skills. This single home is load-bearing because
   duplicated thresholds could make reviewer output and dispatcher behavior disagree across Core,
   Full, Pi, and Claude projections. Keep the maintainable-code-design guide as doctrine owner.

5. `decision: preserve-review-boundaries` Preserve report-only reviewer agents, the existing
   classification model, autonomous authority-preserving remediation, and the single bounded verify
   pass. Do not extend this decision to ADR review or introduce AF-013's later severity separation.

6. `decision: deterministic-contract-proof` Back the shared contract with deterministic tests that
   prove Core and Full plus Pi and Claude projection parity, coherent missingkey=zero rendering with
   empty variables and no `<no value>` token, the accepted evidence mapping, classification autonomy,
   dispatcher rejection of risk-free preferences, and outcome scenarios for concrete risk, competing
   clean local options, and the material-decision boundary.

## State changes

- add `rendering/workflow-skill-templates:concrete-maintainability-review`
- update `rendering/workflow-skill-templates:maintainable-code-review-lenses`
- update `rendering/workflow-skill-templates:authority-guided-review-remediation`

## Consequences

Plan and implementation review can block dual ownership, duplicated policy, and other concrete
maintainability hazards with inspectable reasoning. A reviewer cannot force a preferred local shape
without identifying the risk it prevents. Dispatchers retain engineering autonomy for competing
clean remedies and still stop at the existing material-decision boundary when the protected contract
would change.

The unchanged schema avoids a cross-reviewer migration and keeps severity policy outside this issue,
but it encodes several evidence elements inside prose fields rather than dedicated keys. Deterministic
projection tests and outcome-oriented reviewer scenarios must therefore prove both the field mapping
and its semantic interpretation. A malformed risk-free maintainability finding can still be emitted;
the dispatcher defense rejects it, while reviewer evaluation and review assurance limit recurrence.

One more shared partial and four consumers add rendered-prose surface area. Single-home assertions,
Core and Full plus Pi and Claude parity checks, and generated-output meaning review mitigate drift.
ADR structural review continues under its existing trigger and is intentionally not governed by this
implementation-and-plan risk threshold.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Add owner and risk keys to every finding schema | Retaining the schema avoids a migration across all reviewer contracts, but makes evidence conformance prose-based and therefore dependent on semantic tests and dispatcher validation. |
| Add a severity-routed advisory finding disposition | Severity is intentionally informational today, and changing that model belongs to AF-013. |
| Let risk-free preferences remain as nits | Dispatchers apply mechanical and reasoned findings regardless of severity, so the preference would still demand remediation. |
| Repeat the threshold in each reviewer and dispatcher | Parallel wording could diverge and recreate ambiguous policy ownership. |
| Apply the threshold to ADR structural review | ADR review evaluates proposed durable authority and has a narrower structural trigger; AF-005 explicitly targets implementation and plan assurance. |

## Status history

- 2026-08-19: Proposed
