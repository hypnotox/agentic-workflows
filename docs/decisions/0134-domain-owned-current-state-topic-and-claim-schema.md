---
status: Proposed
date: 2026-07-19
tags: [anchored-globs, context-query, invariant-backing, sidecar-derived-doc]
related: [14, 64, 77, 92, 102, 104, 105, 106, 109, 110, 112, 114, 133]
domains: [adr-system, invariants, rendering, tooling]
---
# ADR-0134: Domain-Owned Current-State Topic and Claim Schema

## Context

ADR-0133 chooses constrained Markdown current-state topics as the sole authority for current project reality. That boundary is useful only if topics remain smaller and more directly retrievable than the ADR graph they replace. The topic schema therefore needs to make ownership, applicability, claim identity, provenance, and invariant backing explicit without turning current-state prose into a generated database dump.

Domains already provide coarse path ownership through sidecars and generated documentation. Their current-state sections, however, are single narratives, and their generated ADR indexes grow with the historical corpus. The replacement needs multiple focused topics per domain, with a compact generated index and independent path selectors.

The closed `.awf/` tree does not currently recognize a topic producer family. Domain artifacts use metadata and authored parts as inputs to generated documentation. Following that convention keeps canonical authored state inside `.awf/`, makes rendered output drift-checkable, and avoids a second central registry.

Retrieval also needs a stable address smaller than an ADR. A topic is still too coarse when one file governs several rules, while one current-state file per rule would recreate artifact explosion. Individually addressable claims inside focused topics provide the intermediate unit.

## Decision

1. awf will add topics as a first-class rendered producer family. Topic metadata lives at `.awf/topics/<domain>/<topic>.yaml`; constrained authored content lives at `.awf/topics/parts/<domain>/<topic>/current-state.md`. The path-derived `<domain>` and `<topic>` are authoritative identities. The named domain must exist. `awf new topic <domain> "<Title>"` creates both inputs, derives a collision-free topic slug, and never registers a duplicate central list.

2. A topic sidecar requires `title` and a one-line `summary`. It declares exactly one applicability form: anchored repository-relative `paths`, or `applies: global`. For a path-scoped topic, effective scope is the intersection of its selectors with its parent domain's ownership; selectors cannot extend domain ownership. A global topic remains stored, rendered, maintained, and indexed under its one natural owning domain but is the explicit exception to path bounding.

3. awf will render each topic to `docs/topics/<domain>/<topic>.md` and generate `docs/topics/<domain>/index.md` from titles and summaries. The generated domain document links or embeds a compact topic list instead of a per-domain ADR index. Topic inputs participate in closed-tree claiming, output planning, the lock manifest, sync, pruning, and regenerate-and-compare drift checks like other rendered families.

4. A topic part contains explanatory prose and one `## Claims` section. Each claim starts at a level-three heading whose code span is either `rule: <local-slug>` or `invariant: <local-slug>` and continues until the next level-three heading. A claim requires normative prose and `Origin: ADR-NNNN`. Optional `Revised-by: ADR-NNNN, ...` and `References: <qualified-id>, ...` lines are unique and ordered. A full claim identity is `<domain>/<topic>:<local-slug>` and is globally unique by construction. Claim and topic slugs use lower-case ASCII letters, digits, and hyphens.

5. A rule is normative without claiming mechanical proof. An invariant additionally declares exactly one backing form. `Backing: test` requires a proof marker in a configured test-backing path. `Backing: unbacked` requires a concrete `Verify:` line and forbids a proof marker. Proof markers, advisory touches markers, and the standalone invariant report use the qualified current-state claim identity; Implemented ADR declarations no longer create active invariant obligations.

6. An optional relevance marker `state: <qualified-id>` may narrow normal context to a particular rule or invariant within a topic already applicable to the queried path. It never expands topic scope; an unknown or out-of-scope relevance marker is an error. Proof markers express evidence rather than applicability and may live outside the production topic scope when they satisfy configured test-backing paths.

7. `awf context <paths>` resolves global topics and domain-bounded path topics, then uses relevance markers when present to narrow claim selection. Directory arguments expand to their eligible descendant files before anchored-glob matching. Normal output contains concise topic summaries and active claims, not Implemented ADRs, tag unions, supersession annotations, or historical plan expansion.

8. `awf topic <domain>/<topic>` and `awf topic <qualified-id>` inspect canonical state directly. Default output remains bounded. `--history`, `--references`, `--coverage`, and `--json` opt into provenance operations, reference edges, applicability diagnostics, and structured output. Removed-claim history comes from ADR state-impact operations; exact former prose remains available through Git rather than active tombstones.

9. `awf context --uncovered` distinguishes unowned paths, domain-owned paths with no scoped topic, duplicate or excessive topic fan-out, and invalid claim markers. Interactive coverage scans tracked and non-ignored untracked working-tree files. Staged validation scans the index. Generated outputs, ignored and deleted paths, nested repositories or adopted projects, and configured `contextIgnore` paths are excluded. Global topics do not satisfy scoped topic coverage. Topic-coverage and overlap severity is configurable as `error`, `warn`, or `off`, with `error` the default unless an adopter explicitly weakens it.

## Invariants

- `unbacked-invariant: topic-identity-path-derived`: A topic has one path-derived domain and topic identity and no second registry can disagree with it. **Verify:** create a topic fixture, inspect its parsed identity and rendered paths, then rename either directory component and confirm every qualified claim identity and output path changes consistently or fails on unresolved references.
- `unbacked-invariant: topic-output-complete`: Every valid topic input has one rendered topic document and participates in its domain's generated topic index, output plan, lock manifest, drift check, and prune behavior. **Verify:** create and remove a topic in a render fixture and compare `awf sync`, `awf check`, output-plan, lock, index, and stale-output results.
- `unbacked-invariant: claim-id-qualified`: Every parsed rule and invariant has the unique identity `<domain>/<topic>:<local-slug>`, and all provenance, reference, relevance, and proof links resolve through that identity. **Verify:** exercise duplicate local slugs across different and identical topics plus valid and dangling references in topic corpus tests.
- `unbacked-invariant: topic-scope-cannot-expand-domain`: A path-scoped topic applies only where its selectors and its parent domain both match; only `applies: global` bypasses path bounding. **Verify:** query fixtures inside and outside the domain intersection and confirm only the intersection matches, then confirm a global topic stored under the same domain applies to both.
- `unbacked-invariant: relevance-markers-only-narrow`: A relevance marker selects claims only from an already applicable topic and never repairs uncovered topic metadata. **Verify:** place the same qualified marker inside and outside topic scope and confirm the first narrows output while the second fails and remains uncovered.
- `unbacked-invariant: context-default-excludes-history`: Normal path context returns active current-state claims without expanding Implemented ADRs or historical plans. **Verify:** use a fixture with claim provenance, ADR tags and relations, and linked plans; compare bounded default output with explicit `awf topic <claim-id> --history` output.

## Consequences

Current authority becomes directly readable from bounded files while retaining stable machine-checkable identities. A domain may grow by adding focused topics rather than lengthening one narrative. Topic summaries provide cheap navigation, and claim markers allow adopters to improve precision without annotating every ordinary rule or letting annotations conceal scope gaps.

The feature is a new producer family, not merely a parser. It touches closed-tree validation, rendering, output planning, locking, pruning, config reference generation, CLI dispatch, domain documentation, context assembly, invariant scanning, and adopter fixtures. The schema ADR deliberately fixes these external contracts while leaving package decomposition to implementation planning.

Qualified markers are a breaking source annotation change. Existing invariant slugs and proof or touches markers must be reconciled during the project-atomic migration rather than accepted as ambiguous aliases afterward.

Global applicability is explicit and intentionally exceptional. A global topic still has one maintenance owner and one rendered domain location, but its claims add a small fixed baseline to every context query. Review and coverage checks must resist using global scope as a shortcut for missing focused topics.

Topic overlap can be legitimate because several aspects may govern one file. awf reports duplication and excessive fan-out without pretending every overlap is an error. The configured severity lets adopters choose a stricter budget while the default keeps gaps visible.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Store authored topic Markdown directly under `docs/` | It would bypass the existing config-tree source, rendering, and drift model. |
| Keep one current-state document per domain | Large domains would again accumulate monolithic prose and coarse retrieval. |
| Store one file per claim | File count and navigation would recreate the scaling problem at a smaller unit. |
| Register every topic in `.awf/config.yaml` | A central registry would duplicate file discovery and create another drift edge. |
| Let source markers expand topic coverage | Distributed annotations would conceal incomplete metadata and make `--uncovered` unreliable. |
| Encode claims as YAML records rendered to prose | It would improve parsing at the cost of making canonical state unpleasant to author and read. |
