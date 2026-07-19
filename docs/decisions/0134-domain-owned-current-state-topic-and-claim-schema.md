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

1. awf will add topics as a first-class rendered producer family. Topic metadata lives at `.awf/topics/metadata/<domain>/<topic>.yaml`; constrained authored content lives at `.awf/topics/parts/<domain>/<topic>/current-state.md`. Separating `metadata` and `parts` prevents either name from colliding with a domain key. The path-derived `<domain>` and `<topic>` are authoritative identities. A topic-owning domain key, topic slug, and local claim slug each match `[a-z0-9]+(?:-[a-z0-9]+)*`; incompatible legacy domain keys must be renamed before upgrade. The named domain must exist. `awf new topic <domain> "<Title>"` creates both inputs, derives a collision-free topic slug, and never registers a duplicate central list.

2. A topic sidecar accepts only `title`, `summary`, `paths`, and `applies`. `title` and the one-line `summary` are required and nonempty. A path-scoped sidecar has a nonempty, duplicate-free `paths` array of valid anchored repository-relative globs. A global sidecar has exactly `applies: global`. The two applicability forms are mutually exclusive; unknown fields, including generic `data`, `sections`, and `local`, fail. For a path-scoped topic, effective scope is the intersection of its selectors with its parent domain's ownership. A global topic remains stored, rendered, maintained, and indexed under its one natural owning domain but is the explicit exception to path bounding.

3. awf will render each topic to `<docsDir>/topics/<domain>/<topic>.md` and generate `<docsDir>/topics/<domain>/index.md` from titles and summaries. The generated domain document links or embeds a compact topic list instead of a per-domain ADR index. Topic inputs participate in closed-tree claiming, output planning, the lock manifest, sync, pruning, and regenerate-and-compare drift checks like other rendered families. Their templates use the existing publication-safe rendering path and cannot emit a no-value token when authored or configured values are empty. This config-tree and configuration change bumps the schema generation; config parsing and reference generation, output planning, manifest hashing and pruning, and migration all consume the new schema under ADR-0133's project-atomic upgrade gate.

4. A topic part contains explanatory prose followed by exactly one final `## Claims` section. A claim heading has exactly the form ``### `rule: <local-slug>` `` or ``### `invariant: <local-slug>` `` with no other heading content. A claim ends at the next claim heading or end of file; another level-one through level-three heading inside `## Claims` fails. At least one nonblank, non-metadata prose line is required. A contiguous metadata block ends the claim and uses canonical order: required `Origin: ADR-NNNN`; optional `Revised-by: ADR-NNNN, ...`; optional `References: <qualified-id>, ...`; and invariant-only backing metadata. Duplicate, empty, out-of-order, or unknown reserved metadata fails.

5. Claim metadata is manually authored rather than generated from the sidecar or ADR. `Origin` names exactly one existing Implemented ADR. `Revised-by` is duplicate-free and lists existing Implemented ADRs in application order; each named ADR declares the corresponding update. Migration normalizes legacy `Superseded` statuses to `Implemented`, so provenance has no status exception. `References` contains duplicate-free direct qualified IDs of active claims. Dangling and self references fail; reference cycles are allowed and are never traversed implicitly. A full identity is `<domain>/<topic>:<local-slug>` and is globally unique by construction.

6. A rule is normative without claiming mechanical proof. An invariant declares `Backing: test`, or `Backing: unbacked` followed by one nonempty `Verify:` line. Test backing requires a proof marker; unbacked verification forbids one. The `currentState` config replaces the legacy invariant scanner configuration. Each optional `sources` entry has exactly `globs`, `marker`, and optional `close`: `globs` is a nonempty duplicate-free array of valid anchored path globs, `marker` is a nonempty opening comment token, and `close` is a nonempty closing token when present; unknown or duplicate fields fail. `testGlobs` is a nonempty duplicate-free array of valid anchored proof-eligible globs when any test-backed claim exists. `sources` may be absent when no markers are used, but any test-backed claim requires nonempty `sources` and `testGlobs`, and every proof marker must match both regardless of the invariant topic's production scope. The standalone invariant report consumes qualified topic claims only.

7. Scanned markers open their own comment line and have exactly one of three forms: `state: <qualified-id>` narrows either claim type; `invariant: <qualified-id>` proves a test-backed invariant; and `touches-state: <qualified-id> - <nonempty-note>` records an advisory site for either claim type. Relevance and touches markers are visible only through configured `currentState.sources`. A relevance marker never expands topic scope; unknown or out-of-scope markers fail. Proof markers express evidence rather than applicability and may live outside production topic scope only when they match both a configured source entry and `testGlobs`.

8. `awf context <paths>` evaluates each eligible file independently. It loads global topics and the path-scoped topics of every domain owning that file. Within an applicable topic, `state:` markers on that file select only their named claims; without a marker for that topic, all its claims are selected. Selections union and deduplicate across files, so a marker in one file never suppresses another file or topic. Normal output contains topic summaries, active claims, and ADR-0133's separately bounded Accepted-ADR pending-changes notice. It excludes Implemented ADRs, provenance links, tag unions, supersession annotations, and historical plan expansion.

9. A normal explicit file argument is evaluated as named. A directory argument expands tracked and non-ignored untracked working-tree descendants, excluding generated outputs, configured `contextIgnore` paths, nested repositories or adopted projects, symlink traversal, and deleted or missing paths. Interactive `--uncovered` uses that same working-tree universe. `--staged` instead reads configuration, topic inputs, markers, and paths from the Git index, includes staged additions, excludes staged deletions, and never mixes working-tree content into the result.

10. `awf topic <domain>/<topic>` and `awf topic <qualified-id>` are version-gated, read-only queries with a static usage reference outside an adopted tree. Default output shows current summary or claim prose and backing state but hides ADR provenance and claim-reference edges. The independently combinable `--history`, `--references`, and `--coverage` flags add direct ADR operation history, direct incoming and outgoing claim references, and topic scope plus marker sites; no flag performs transitive traversal. `--json` changes only presentation to a deterministic structured form. A removed identity resolves only with `--history`; exact former prose remains in Git rather than an active tombstone.

11. `awf context --uncovered` reports unowned paths and, independently for every domain owning a path, missing scoped-topic coverage. A topic from one owner cannot satisfy another owner's gap, and global topics never satisfy scoped coverage. `awf context --uncovered --staged` is valid, and `awf check --staged` invokes the same index-snapshot coverage and transition assembly; the rendered pre-commit hook runs the latter. Multiple scoped topics may legitimately match. `currentState.topicCoverage` and `currentState.topicFanout` independently accept `error`, `warn`, or `off`; coverage defaults to `error`, fan-out defaults to `warn`. `currentState.maxTopicsPerPath` is a positive integer defaulting to 8 and counts path-scoped topics only. Exceeding it produces the configured fan-out finding; identical scopes have no separate meaning.

## Invariants

- `unbacked-invariant: topic-identity-path-derived`: A topic has one path-derived domain and topic identity and no second registry can disagree with it. **Verify:** rename an unreferenced fixture topic and confirm its identity and output paths change deterministically; in a second fixture retain a reference to the old identity and confirm the rename fails with a dangling-reference diagnostic.
- `unbacked-invariant: topic-output-complete`: Every valid topic input has one rendered topic document and participates in its domain's generated topic index, output plan, lock manifest, drift check, and prune behavior. **Verify:** create and remove a topic in a render fixture and compare `awf sync`, `awf check`, output-plan, lock, index, and stale-output results.
- `unbacked-invariant: claim-id-qualified`: Every parsed rule and invariant has the unique identity `<domain>/<topic>:<local-slug>`; claim references and relevance, touches, and proof markers resolve through that identity, while Origin and Revised-by resolve through the ADR corpus. **Verify:** exercise duplicate local slugs across different and identical topics plus valid and dangling claim and ADR references in topic corpus tests.
- `unbacked-invariant: topic-scope-cannot-expand-domain`: A path-scoped topic applies only where its selectors and its parent domain both match; only `applies: global` bypasses path bounding. **Verify:** query fixtures inside and outside the domain intersection and confirm only the intersection matches, then confirm a global topic stored under the same domain applies to both.
- `unbacked-invariant: relevance-markers-only-narrow`: A relevance marker selects claims only from an already applicable topic and never repairs uncovered topic metadata. **Verify:** place the same qualified marker inside and outside topic scope and confirm the first narrows output while the second fails and remains uncovered.
- `unbacked-invariant: context-default-excludes-history`: Normal path context returns active current-state claims without expanding Implemented ADRs or historical plans. **Verify:** use a fixture with claim provenance, ADR tags and relations, and linked plans; compare bounded default output with explicit `awf topic <claim-id> --history` output.

## Consequences

Current authority becomes directly readable from bounded files while retaining stable machine-checkable identities. A domain may grow by adding focused topics rather than lengthening one narrative. Topic summaries provide cheap navigation, and claim markers allow adopters to improve precision without annotating every ordinary rule or letting annotations conceal scope gaps.

The feature is a new producer family, not merely a parser. It touches closed-tree validation, rendering, output planning, locking, pruning, config parsing and reference generation, CLI dispatch, domain documentation, context assembly, invariant scanning, migration, and adopter fixtures. The schema generation and lock compatibility boundary move with those consumers; no older binary may accept the new config tree. The schema ADR deliberately fixes these external contracts while leaving package decomposition to implementation planning.

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
