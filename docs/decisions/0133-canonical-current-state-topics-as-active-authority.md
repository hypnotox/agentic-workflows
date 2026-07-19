---
status: Proposed
date: 2026-07-19
tags: [adr-lifecycle, context-query, domain-index, invariant-backing]
related: [7, 14, 92, 104, 105, 109, 112, 114, 120, 128, 129, 130, 131]
domains: [adr-system, invariants, rendering, tooling]
---
# ADR-0133: Canonical Current-State Topics as Active Authority

## Context

awf currently asks one growing ADR corpus to serve two different purposes. ADRs preserve the rationale and consequences of past decisions, but their frontmatter, Decision anchors, invariant declarations, tags, relations, and supersession records also determine what guidance is active now. `awf context` must infer current relevance from that historical graph.

That model worked while the corpus was small. At more than 100 ADRs, an agent may need to traverse refinement and retirement chains before it can state the current rule. Broad path queries accumulate invariant governors and then expand through shared tags, relations, historical plans, and pitfalls. Generated indexes have become structurally complete without being concise statements of present truth.

The structural gate cannot close the semantic gap. It can prove that anchors, relations, statuses, and proof-marker slugs agree as a ledger while current prose or a marked test expresses the wrong meaning. Recent work in ADR-0120 and ADR-0128 through ADR-0131 made supersession records substantially more complete, but also demonstrated the cost of reconstructing current authority from historical documents. Domain current-state prose mitigates the problem, yet remains a narrative projection beside the ADR authority rather than the canonical input to checks and retrieval.

The system needs separate artifacts for separate questions:

- An ADR should explain why a consequential change was chosen.
- Current-state documentation should state what is true and required now.

## Decision

1. awf will make constrained Markdown current-state topics the sole source of active project authority. Each topic belongs to exactly one domain, remains focused on one area of current behavior, and contains individually addressable normative claims. Domain documents become overview and navigation surfaces over their topics.

2. ADRs will have temporary authority only while an accepted change is in flight. Once implementation incorporates the decision into code and the affected topic claims, the ADR becomes immutable historical rationale. Later changes update or remove current claims through a new ADR; they do not rewrite or reconcile historical ADR prose.

3. Active rules and invariants will be parsed and checked from topic claims. Invariant proof markers will reference current-state claim identities rather than ADR declarations. ADR provenance remains linked from claims so a reader can request rationale without loading it during ordinary context retrieval.

4. `awf context` will answer which current claims apply to queried paths. It will select topics through domain-bounded topic scopes and optional claim markers, include a distinct bounded notice for Accepted ADRs whose explicit state impacts target those topics, and load historical ADRs only on explicit request. ADR tags, relation graphs, supersession coverage, and historical plan links will not expand active context.

5. awf will provide one authority model, not a permanent compatibility mode. Existing adopters must curate current-state topics to regain effective context and invariant checking. Migration tooling may inventory obligations and expose uncovered paths, but it must not infer authoritative prose from the legacy ADR corpus.

6. Focused successor ADRs will settle the topic and claim schema, the ADR lifecycle and mechanically checked state-impact protocol, and the one-time invariant and adopter migration. Implementation will proceed through phased plans after those decisions are reviewed and reconciled.

## Invariants

- `unbacked-invariant: current-state-sole-active-authority`: Normal context retrieval and invariant enforcement consume current-state topic claims, not Implemented historical ADR prose. **Verify:** inspect the context and invariant corpus assembly paths and confirm historical ADRs enter only explicit history queries or the bounded Accepted-ADR notice.
- `unbacked-invariant: implemented-adrs-are-history`: After an ADR becomes Implemented, changing active guidance requires a new ADR and a current-state claim mutation; the Implemented ADR body is not edited to express the new rule. **Verify:** inspect the lifecycle documentation and transition checks, then confirm current claim provenance points back to immutable ADR files.
- `unbacked-invariant: migration-does-not-infer-authority`: Migration may inventory and validate legacy obligations but never promotes generated summaries of ADR prose into authoritative claims. **Verify:** inspect migration output and ensure topic claim prose requires explicit authored input.
- Every current-state topic has one owning domain, and its effective path scope is bounded by that domain.
- Historical rationale remains reachable from active claim provenance without appearing in normal path-context output.

## Consequences

The active guidance surface becomes smaller, concrete, and directly editable. Refining a rule changes one canonical claim and records one ADR operation instead of requiring agents to reconstruct a supersession chain. Domain ownership, topic coverage, invariant backing, provenance, and retrieval can remain mechanically checked without treating history as present truth.

The change deliberately removes substantial recently built machinery: partial and full ADR supersession, anchor-coverage derivation, relation back-pointers, superseded-anchor annotations, and tag-tiered ADR expansion cease to determine active authority. Their historical encodings can remain readable without remaining an active validation graph.

This is a breaking conceptual migration. Existing projects must classify legacy invariants and author topic claims. awf cannot automate the semantic judgment safely. The upgrade must make missing work visible and prevent invariant obligations from disappearing silently.

The implementation is too broad for one package or one plan. Topic rendering affects the closed config tree and output plan; claim identities affect invariant markers; staged transitions require Git snapshot semantics; context and audit behavior change; generated workflow instructions and adopter documentation must move with the implementation.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep ADRs permanently authoritative and improve retrieval | Better indexes and tags still require current truth to be inferred from a growing historical graph, preserving the central scaling failure. |
| Generate current-state projections from ADRs | A generated projection can be structurally valid while semantically stale, and it retains the inference machinery this decision removes. |
| Replace ADRs with lightweight claim change logs | This would simplify active state but discard valuable proposal review, alternatives, rationale, and consequences for consequential decisions. |
| Keep both legacy ADR authority and topic authority | Two active models would create ambiguity, duplicate checks, and a permanent migration burden. |
