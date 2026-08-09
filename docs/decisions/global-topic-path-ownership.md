---
format: current-state-v4
slug: global-topic-path-ownership
status: Proposed
date: 2026-08-09
---
# ADR-global-topic-path-ownership: Global Topic Path Ownership


## Context

A topic currently declares exactly one applicability form: path selectors or `applies: global`.
Path-scoped topics can satisfy coverage for paths owned by their parent domain. Global topics supply
authority everywhere, but coverage and fan-out skip them outright. The two concerns - where an
authority applies and which paths it owns - are therefore coupled in the metadata representation.

That coupling leaves shared-pattern packages without a natural authority owner. A package that
exists solely to carry a cross-cutting model belongs with the global topic describing that model,
not with whichever scoped topic happens to be nearby. The ADR-0183 rollout encountered this with a
presentation package and widened an unrelated scoped topic selector to preserve coverage. Source
markers cannot solve the mismatch: they select or evidence claims only after topic applicability is
established and are not ownership declarations.

The metadata grammar is strict, and older binaries reject a topic that combines `applies: global`
with `paths`. Adopting the combined form therefore requires a schema-generation advance and its
minimum-binary-version mapping.

## Decision

1. `decision: separate-global-applicability-and-ownership` Topic metadata may combine
   `applies: global` with nonempty anchored `paths`. Global applicability continues to make the
   topic authoritative for every repository path; the selectors separately declare paths the topic
   owns. A topic may still declare only `paths` for scoped applicability or only `applies: global`
   for global authority without path ownership.
2. `decision: bound-global-ownership-by-domain` A global topic's ownership selector matches only
   where both that selector and its owning domain's selector match. It grants topic-level ownership
   and never creates domain ownership for an otherwise unowned path.
3. `decision: global-ownership-satisfies-coverage` A matching path-owning global topic satisfies its
   owning domain's coverage requirement when the topic has at least one claim. Authority owned by a
   different domain never closes the gap.
4. `decision: global-ownership-counts-in-fanout` A global topic counts toward the fixed fan-out
   budget only on paths matched by its bounded ownership selectors. As with scoped topics, fan-out
   counts the matching topic even when it currently has no claims; repository-wide applicability
   outside those selectors contributes nothing to fan-out.
5. `decision: distinguish-applicability-from-ownership-evidence` Matching and reporting preserve
   separate concepts for global applicability and bounded path ownership. Context and markers keep
   using global applicability, while coverage and fan-out use ownership matches. Topic coverage and
   context selector output expose declared ownership selectors and ownership witnesses without
   presenting them as bounds on global authority.
6. `decision: selectors-are-the-only-ownership-declaration` Claim references, relevance markers,
   touches markers, proof markers, generated indexes, and ADR attribution neither grant nor expand
   path ownership.
7. `decision: activate-combined-metadata-safely` The combined metadata form advances the config
   schema generation and records the corresponding minimum binary version so an older strict parser
   cannot govern a project using global path ownership.

## State changes

- update `invariants/topics-and-markers:fan-out-budget-fixed`
- add `invariants/topics-and-markers:global-topic-path-ownership`
- update `invariants/topics-and-markers:topic-scope-cannot-expand-domain`
- update `invariants/topics-and-markers:topic-scope-is-domain-bounded`
- update `invariants/topics-and-markers:rendered-applicability-selectors-only`
- update `tooling/context-and-topic:context-applicability-navigation`

## Consequences

A global topic can become the truthful path authority for a shared-pattern package while remaining
available everywhere. Coverage no longer forces an unrelated scoped topic to claim that package.
Domain sidecars remain the sole source of domain ownership, and topic selectors remain explicit,
reviewable ownership evidence.

The implementation needs distinct applicability and ownership matching helpers and distinct query
witnesses; reusing the existing global applicability helper for ownership would accidentally narrow
context and marker validity. A matching global owner consumes one fan-out slot, making overlap cost
honest without counting every globally applicable topic on every path.

The schema advance intentionally requires adopters using the combined form to run a compatible
binary. Existing path-only and global-only metadata remains valid. Documentation, fixtures, coverage
queries, context projections, rendering, the glossary, and the recorded global-topic coverage pitfall
must distinguish applicability from ownership.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Infer ownership from claim references or source markers | Those relationships carry evidence and navigation semantics, not an explicit path-ownership commitment. |
| Make a global topic's selectors bound its applicability | That would contradict global authority and reject valid context and markers outside the owned paths. |
| Let global ownership escape its parent domain | Topic selectors would become a second, conflicting domain-ownership registry. |
| Exclude path-owning globals from fan-out | Selective ownership would consume no overlap budget despite participating in the same path authority set. |

## Status history

- 2026-08-09: Proposed
