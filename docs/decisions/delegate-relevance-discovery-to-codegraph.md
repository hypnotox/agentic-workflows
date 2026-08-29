---
format: current-state-v4
slug: delegate-relevance-discovery-to-codegraph
status: Proposed
date: 2026-08-29
---
# ADR-delegate-relevance-discovery-to-codegraph: Delegate relevance discovery to CodeGraph


## Context

`awf context` combines two different concerns. It performs source-tree discovery, path census and grouping, impact-style relationship projection, and artifact navigation while also exposing awf-specific current-state and ADR authority. The first concern now duplicates the expected CodeGraph workflow and has accumulated separate input snapshots, focused preparation, delivery limits, spill files and logging, staged and range modes, facets, classifications, and publisher observation fields. The second concern remains necessary because CodeGraph cannot infer authored domain and topic applicability, current claim authority, ADR operation progress, or parsed plan links.

The governed tag vocabulary is another manual relevance index. Its live consumers are authored pitfalls, validation, heuristic warnings, and rendered taxonomy; it does not drive current-state authority. `state:` and `touches-state:` markers likewise provide navigation rather than proof. In contrast, `invariant:` markers express the authored assertion that a named test backs a specific invariant and remain outside CodeGraph's descriptive source graph.

The current context exclusion list serves the removed navigation census and also exempts paths from authority coverage. Domain and topic selectors already define the authority boundary, generated outputs have independent treatment, and a domain-owned path should not silently escape coverage.

## Decision

1. `decision: external-navigation-boundary` CodeGraph is the expected and documented owner of source discovery, architecture, callers, dependencies, and impact analysis; Git owns changed-path selection. awf will not retain a parallel general context or relevance engine.
2. `decision: focused-authority-commands` awf will expose only focused read-only authority operations: verb-subject topic and ADR reads, exact path-to-topic resolution that reports owning domains and applicable topics or explicit absence, and one whole-repository unowned-path census. Query absence succeeds, while `awf check` remains the enforcement owner.
3. `decision: relevance-metadata-retirement` awf will remove the active tag vocabulary and pitfall tags while retaining pitfall domains, remove `state:` and `touches-state:` markers while retaining `invariant:` proof markers, and remove the context exclusion surface without replacement. Frozen legacy ADR tags remain historical parser compatibility rather than live vocabulary.
4. `decision: governance-core-retained` Domains, topic selectors, current claims, proof backing, authority coverage and fan-out, ADR-to-claim transitions, parsed plan links, generated-output provenance, publication, and drift remain with their existing semantic owners because CodeGraph does not replace them.
5. `decision: clean-cutover` The obsolete context command, projections, compatibility aliases, packages, spill behavior, and context-only publisher observations will be removed rather than deprecated. Upgrade will migrate live config and pitfall sources before strict decoding; arbitrary retired navigation comments may remain inert in adopter-owned source.

## State changes

- remove `tooling/context-and-topic:context-adr-operation-projection`
- remove `tooling/context-and-topic:adr-linked-plan-references`
- remove `tooling/context-and-topic:context-applicability-navigation`
- remove `tooling/context-and-topic:context-default-excludes-history`
- remove `tooling/context-and-topic:context-concise-projection`
- remove `tooling/context-and-topic:context-full-authority-packet`
- remove `tooling/context-and-topic:context-known-artifact-navigation`
- remove `tooling/context-and-topic:context-path-attribution`
- remove `tooling/context-and-topic:context-path-classification`
- remove `tooling/context-and-topic:context-query-boundary`
- remove `tooling/context-and-topic:context-read-only`
- remove `tooling/context-and-topic:context-static-fallback`
- remove `tooling/context-and-topic:context-summary-projection`
- remove `tooling/context-and-topic:context-terminal-output-cap`
- remove `tooling/context-and-topic:context-spill-observability`
- remove `tooling/context-and-topic:describe-read-only`
- remove `tooling/context-and-topic:production-packages-domain-owned`
- remove `tooling/context-and-topic:uncovered-collapses-directories`
- remove `tooling/context-and-topic:context-full-profile-only`
- add `tooling/authority-queries:codegraph-navigation-boundary`
- add `tooling/authority-queries:authority-read-projections`
- add `tooling/authority-queries:path-topic-resolution`
- add `tooling/authority-queries:unowned-path-census`
- add `tooling/authority-queries:authority-query-read-only`
- add `tooling/authority-queries:authority-query-full-profile-only`
- add `tooling/cli:init-describe-read-only`
- add `invariants/current-state-authority:production-packages-domain-owned`
- add `invariants/current-state-authority:domain-owned-coverage-no-ignore`
- remove `invariants/current-state-authority:uncovered-lists-unowned-unignored`
- add `invariants/current-state-authority:uncovered-lists-unowned`
- update `invariants/current-state-authority:accepted-authority-is-pending-only`
- update `invariants/current-state-authority:current-state-sole-active-authority`
- update `invariants/current-state-authority:historical-rationale-is-explicit`
- update `invariants/topics-and-markers:claim-id-qualified`
- update `invariants/topics-and-markers:invariants-three-state`
- remove `invariants/topics-and-markers:relevance-markers-only-narrow`
- remove `invariants/topics-and-markers:touches-marker-advisory`
- add `invariants/topics-and-markers:proof-only-marker-grammar`
- update `invariants/topics-and-markers:rendered-applicability-selectors-only`
- remove `config/configuration:tag-coverage-note`
- remove `config/configuration:tag-frequency-note`
- remove `config/configuration:tag-vocabulary-governed`
- add `config/configuration:no-active-tag-system`
- remove `config/validation:tag-not-domain-name`
- update `rendering/doc-outputs:pitfall-corpus-validated`
- update `rendering/workflow-skill-templates:implementer-context-grounding`
- update `adr-system/adr-lifecycle:corpus-owns-status-literals`
- update `tooling/cli:explicit-output-bypasses`
- update `tooling/cli:check-severity-by-protected-property`
- update `tooling/upgrade-runtime:upgraded-runtime-has-one-authority-engine`

## Consequences

The general context subsystem, its delivery protocol, and its manual relevance indexes disappear. Authority lookup becomes smaller, attributable, and aligned with existing semantic owners. Code navigation quality depends on users providing CodeGraph, and awf documentation must state that expectation rather than imply a built-in fallback.

Path resolution remains useful for proposed files and therefore reports absence without failing. The whole-repository census retains visibility into paths outside every configured domain without restoring scan-root selection. Coverage remains stricter because a domain-owned path has no ignore escape hatch.

Existing projects require a schema migration that removes config tags, context exclusions, and pitfall tag frontmatter before the new strict parser runs. Retired source markers outside awf's owned config tree are harmless inert comments rather than a reason to mutate arbitrary adopter source. Legacy ADR bytes remain parseable for append-only history.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep `awf context` and add CodeGraph guidance | Preserves duplicated navigation machinery and its conceptual invitation to grow. |
| Shrink `awf context` in place | Retains an overloaded command boundary instead of assigning focused authority operations to their semantic owners. |
| Remove all path-to-authority lookup | CodeGraph cannot infer normative domain and topic applicability. |
| Keep tags as prose taxonomy | Maintains a second governed vocabulary and heuristic checks for navigation that no longer justifies its cost. |
| Remove proof markers with navigation markers | CodeGraph cannot express the authored test-to-invariant backing assertion. |

## Status history

- 2026-08-29: Proposed
