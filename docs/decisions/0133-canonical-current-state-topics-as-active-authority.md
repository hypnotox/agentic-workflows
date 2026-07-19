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

1. awf will make constrained Markdown current-state topics the sole source of authority for current project reality. Each topic belongs to exactly one maintenance-owning domain, lives and renders under that domain, remains focused on one area of current behavior, and contains individually addressable normative claims. Path-scoped topics are bounded by their owning domain. An explicit `applies: global` topic is the sole path-bounding exception: it remains stored and indexed under its natural owning domain while its intentionally small claim set applies repository-wide. Domain documents become overview and navigation surfaces over their topics.

2. An Accepted ADR is normative only for executing its pending change. It supplements the current-state claims as implementation instruction but does not override what those claims say is true before implementation. Normal context presents it in a separate pending-changes section. Its temporary implementation authority ends when the ADR becomes Implemented or Abandoned. An Implemented ADR is immutable historical rationale; later changes update or remove current claims through a new ADR rather than rewriting historical prose.

3. Active rules and invariants will be parsed and checked from topic claims. Invariant proof markers will reference current-state claim identities rather than ADR declarations. ADR provenance remains linked from claims so a reader can request rationale without loading it during ordinary context retrieval.

4. `awf context` will answer which current claims apply to queried paths. It will select topics through domain-bounded topic scopes and optional claim markers, include only the bounded Accepted-ADR notice described above, and load historical ADRs only on explicit request. ADR tags, relation graphs, supersession coverage, and historical plan links will not expand active context.

5. awf will ship one authority engine, not a permanent or transitional compatibility mode. Upgrade is project-atomic and completes only after every domain-owned path has required topic coverage, every surviving legacy invariant maps to exactly one typed topic claim, every marker resolves under the new model, every existing Proposed or Accepted ADR is Implemented or Abandoned, and the static and staged gates pass. An adopter that has not curated this state remains on the preceding awf release. Migration preflight may inventory obligations and expose uncovered paths, but it must neither operate the legacy authority engine nor infer authoritative prose from the ADR corpus.

6. Existing ADR-derived context, supersession, and invariant contracts remain intact while the replacement is developed. Focused successor ADRs will settle the topic and claim schema, the ADR lifecycle and mechanically checked state-impact protocol, and the invariant and adopter migration. The atomic cutover removes the legacy consumers; historical Decision anchors do not receive a corpus-wide supersession retrofit. The migration gate instead adjudicates each legacy invariant as migrated or retired, and surviving contracts are redeclared as current-state claims. Implementation will proceed through phased plans after those decisions are reviewed and reconciled.

## Invariants

- `unbacked-invariant: current-state-sole-active-authority`: Normal context retrieval and invariant enforcement consume current-state topic claims, not Implemented historical ADR prose. **Verify:** in an adopter fixture containing claim provenance, one topic-declared invariant, and one ADR-only legacy invariant, run `awf context <matching-path>` and confirm it emits the active claim but no historical ADR; run the invariant checker and confirm only the topic declaration creates an active obligation; then run `awf topic <claim-id> --history` and confirm it emits the provenance ADR.
- `unbacked-invariant: accepted-authority-is-pending-only`: An Accepted ADR appears as pending implementation instruction and never replaces a current claim before its state-impact transaction completes. **Verify:** in a fixture with a conflicting current claim and Accepted ADR, run `awf context <matching-path>` and confirm the output keeps the claim under current authority and places the ADR only under pending changes.
- `unbacked-invariant: migration-does-not-infer-authority`: Migration inventories and validates legacy obligations but never writes generated ADR summaries as authoritative claims. **Verify:** run upgrade preflight on a fixture with an unmigrated invariant and no topic claim; confirm it refuses completion, reports the obligation, and leaves the topic authoring tree unchanged.
- `unbacked-invariant: current-state-cutover-is-atomic`: Schema upgrade enables the topic authority engine only after every readiness predicate succeeds and never leaves a partial or compatibility state. **Verify:** use upgrade fixtures that independently fail topic coverage, legacy-invariant mapping, marker resolution, in-flight ADR resolution, and static or staged validation; confirm each leaves the old lock unchanged, then make every predicate pass and confirm the upgraded project runs only topic-based context and invariant checks.
- `unbacked-invariant: topic-scope-is-domain-bounded`: Every topic has one owning domain; path-scoped topics are bounded by that domain, while an explicit globally applicable topic remains stored under its owner and is the only exception. **Verify:** configure a path-scoped topic selector that also matches outside its parent domain and confirm context includes only the domain-owned match, then configure an `applies: global` topic under the same domain and confirm its claims apply repository-wide without changing its rendered domain path.
- `unbacked-invariant: historical-rationale-is-explicit`: Historical rationale remains reachable from active claim provenance without appearing in normal path-context output. **Verify:** run normal context and explicit `awf topic <claim-id> --history` over the same fixture and confirm only the latter follows Implemented provenance.

## Consequences

The active guidance surface becomes smaller, concrete, and directly editable. Refining a rule changes one canonical claim and records one ADR operation instead of requiring agents to reconstruct a supersession chain. Domain ownership, topic coverage, invariant backing, provenance, and retrieval can remain mechanically checked without treating history as present truth.

The change deliberately removes substantial recently built machinery: partial and full ADR supersession, anchor-coverage derivation, relation back-pointers, superseded-anchor annotations, and tag-tiered ADR expansion cease to determine active authority. Their historical encodings can remain readable without remaining an active validation graph. Until the project-atomic cutover, the existing contracts and gates remain in force; there is no partially migrated authority state.

This is a breaking conceptual migration. Existing projects must classify legacy invariants and author topic claims. awf cannot automate the semantic judgment safely. The upgrade must make missing work visible and prevent invariant obligations from disappearing silently.

The implementation is too broad for one package or one plan. Topic rendering affects the closed config tree and output plan; claim identities affect invariant markers; staged transitions require Git snapshot semantics; context and audit behavior change. The commits that change these behaviors must update the `.awf/` sources for AGENTS.md, the ADR lifecycle and README guidance, and the adr-system, invariants, rendering, and tooling current-state surfaces, then run `./x sync` so their rendered outputs and the decision index move atomically with reality.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep ADRs permanently authoritative and improve retrieval | Better indexes and tags still require current truth to be inferred from a growing historical graph, preserving the central scaling failure. |
| Generate current-state projections from ADRs | A generated projection can be structurally valid while semantically stale, and it retains the inference machinery this decision removes. |
| Replace ADRs with lightweight claim change logs | This would simplify active state but discard valuable proposal review, alternatives, rationale, and consequences for consequential decisions. |
| Keep both legacy ADR authority and topic authority | Two active models would create ambiguity, duplicate checks, and a permanent migration burden. |
| Shadow-validate topics while upgrading an adopted project | A staged per-project cutover would require the new release to retain the obsolete authority engine; a project-atomic preflight provides readiness evidence without shipping two engines. |
