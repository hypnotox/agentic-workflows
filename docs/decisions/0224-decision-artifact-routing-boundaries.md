---
format: current-state-v4
slug: decision-artifact-routing-boundaries
status: Proposed
date: 2026-08-03
---
# ADR-0224: Decision Artifact Routing Boundaries


## Context

The ADR guide already says that a decision record captures a significant, load-bearing choice,
while the plan guide owns step-by-step execution and effort memory owns unsettled working context.
The boundary is not concrete enough at the point of authoring or review. Several historical ADRs,
including ADR-0069, ADR-0135, and ADR-0223, combine durable commitments with rollout inventories,
proof transactions, or file-oriented downstream work. Those records remain valid history, but the
pattern makes it harder to tell which prose must constrain the project after implementation and
which prose only directed one implementation.

The ambiguity also exists in the workflow that should prevent it. ADR review currently asks records
to declare same-commit update obligations and regeneration work, which turns plan tasks into review
requirements for the ADR body. Natural-language linting cannot resolve the distinction reliably:
a package, algorithm, or proof mechanism can be either an incidental implementation choice or the
substance of a durable compatibility, ownership, safety, or reproducibility constraint.

A related drift demonstrates the cost of duplicated authoring authority. The ADR scaffold uses the
activation registry's current format, while the proposing skill names an older literal format. The
scaffold, not remembered instruction prose, must remain the single source for a new record's format.

## Decision

1. `decision: route-by-durability` Route settled content by its authority lifetime: an ADR Decision item owns a durable design, policy, boundary, or constraint that remains meaningful after implementation; current-state topic claims own the active rules and invariants established by applied decisions; plans own implementation execution; and effort memory owns unsettled or transient working context.

2. `decision: test-decision-content` Judge a proposed Decision item with post-implementation and counterfactual tests. The item belongs in the ADR when it continues to constrain the project after delivery and changing it would violate the intended architecture, policy, behavior, ownership, compatibility, safety, or reproducibility boundary. A mechanism is valid Decision content only when the record makes clear why that mechanism itself is load-bearing.

3. `decision: keep-directives-in-plans` Treat affected paths, commands, task order, rollout batches, ordinary test transactions, and comparable executor instructions as plan content rather than Decision commitments. Consequences may identify durable downstream obligations or costs, but do not become an implementation inventory.

4. `decision: review-semantics` Make this boundary a semantic ADR-review responsibility. A misplaced directive is a reasoned finding, not a mechanical format failure. ADR review assesses decision quality, rationale, consequences, and agreement with declared state changes; plan review and lifecycle execution own implementation completeness and same-transaction update work.

5. `decision: mechanize-objective-contracts-only` Limit deterministic enforcement to objective contracts such as rendered guidance presence, scaffold preservation, publication safety, and generated-output fan-out. Do not infer the meaning of arbitrary Decision prose from keywords, paths, or syntax, and do not add Decision-item kinds solely to approximate that inference.

6. `decision: preserve-historical-records` Leave terminal ADR bodies unchanged. Historical directive-heavy patterns may inform generic positive and negative authoring examples, but do not create or maintain a second classification of the ADR corpus.

7. `decision: scaffold-owns-format` Require ADR authoring guidance to preserve the frontmatter emitted by the sanctioned scaffold. The running binary's activation registry owns the current authoring format; workflow prose does not duplicate its literal value.

## State changes

- add `rendering/templates:decision-artifact-routing`

## Consequences

Authors and reviewers gain a concrete distinction that follows the lifetime of authority rather
than superficial wording. Plans become the unambiguous execution record, and ADRs remain useful
after their implementation details age. Existing historical records need no retrofit.

Semantic review remains necessary, so two reviewers can reasonably disagree about whether a
mechanism is load-bearing. The post-implementation and counterfactual tests make that disagreement
explicit and reviewable instead of hiding it behind a brittle gate. Objective rendering tests prove
that the guidance reaches adopters but do not claim to prove the semantic policy.

Removing implementation-completeness checks from ADR review shifts that responsibility to plan
review and lifecycle execution. A project that skips planning must still satisfy its documentation,
state-change, and gate obligations during implementation; the ADR is no longer used as a surrogate
task list.

The current authoring format can advance without updating prose that copied the previous marker.
The sanctioned scaffold becomes more important and remains the only supported creation path.
All affected templates preserve the existing `missingkey=zero` publication-safety constraint and
must render coherent prose without a no-value token when optional data is unset.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Add an Implementation Directives ADR section or a separate durable artifact | It would duplicate plan authority and freeze execution details inside append-only decision history. |
| Add typed Decision-item categories | A category cannot prove that its prose has the claimed semantics, and a new ADR format would impose disproportionate schema and compatibility cost. |
| Lint paths, commands, sequencing terms, or test language in Decision items | The same terms can name either incidental work or a genuine durable constraint, producing both false positives and easy evasions. |
| Clarify documentation without changing review | The current reviewer itself requests implementation inventories, so prose alone would preserve contradictory incentives. |
| Maintain a classification of every historical ADR | The classification would become a drifting second index and imply mutable judgment over frozen records. |

## Status history

- 2026-08-03: Proposed
