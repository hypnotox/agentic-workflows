---
format: current-state-v2
status: Proposed
date: 2026-07-21
---
# ADR-0144: Agent-Oriented Current-State Context and CLI Navigation


## Context

The current-state system can validate authority, provenance, backing, path coverage, and ADR operation application, but its main orientation command is not shaped for an agent deciding what to read next. `awf context` unions all selected paths into one topic set, loses the attribution from each requested path to each result, and returns every claim in an applicable topic when no exact `state:` marker exists. A large topic can therefore dominate the answer even when only one production site is directly relevant.

The navigation information already exists but is fragmented. `awf topic --coverage` exposes declared paths and marker sites, topic and domain documents are rendered, `touches-state:` markers identify advisory implementation sites, the output plan and lock identify managed outputs, and the ADR corpus now exposes operation-level Applied, Remaining, and Canceled progress under ADR-0143. The default context projection does not connect these surfaces or explain that a topic path and its owning domain path must both match. Its effective-selector output currently presents a Cartesian product that can look like each pair is known to intersect even when no symbolic glob intersection was computed.

Path eligibility and path identity are also conflated. Directory queries expand only to eligible descendants, while explicit ignored, generated, nested-adopter, absent, outside-repository, and eligible-but-unowned paths collapse into omission or the same unowned result. Known awf authoring and generated artifacts are especially hard to navigate because `contextIgnore` correctly excludes many of them from whole-tree coverage. Executable surfaces such as `.pi/extensions/**` and `internal/testsupport/**` should still have current-state ownership even when generated-output or coverage policy excludes them from an eligibility scan.

The managed adopter runner has a separate discoverability gap. Its forwarded awf verbs are hand-written in `templates/runner/x.tmpl`; `topic` was added to the CLI but not to that list, and this repository's source-running `./x` drifted the same way. `internal/clispec` already owns the command table, so command availability should be declared there rather than copied into runners.

ADR-0143 removes the former requirement that all of this decision's claim operations land in one transaction. The implementation can apply coherent operation batches while this ADR is Implementing, with each batch and matching claim mutation checked independently.

## Decision

1. `awf context` becomes a path-attributed orientation query. Its semantic result preserves normalized requested queries separately from effective expanded paths, groups each effective path under every query that selected it, and reports domains, topics, directly relevant claims, navigation, pending changes, and classification per path rather than unioning those facts into one repository-wide bucket. Duplicate effective paths are represented once with deterministic request attribution.

2. A directory query remains a request rather than being silently replaced by its descendants. Working-tree and `--staged` modes expand it against their existing single-universe snapshot rules and retain the requested directory plus its sorted effective descendants in human and JSON output. Request nodes carry an expansion status rather than an aggregate primary classification; each effective path is classified independently, so mixed descendants are not collapsed into a misleading directory status. An explicit file is both the request and its one effective path. `--range` continues to supply changed paths as implicit requested queries.

3. Every effective path receives exactly one primary classification: `covered`, `eligible-unowned`, `context-ignored`, `generated-output`, `nested-adopter`, `symlink`, `not-found`, or `outside-repository`. `covered` means at least one configured domain owns an eligible path; topic coverage is reported separately so domain ownership and claim-bearing scoped-topic coverage are not conflated. Classification uses this precedence: lexically or canonically outside the repository, beneath a nested-adopter boundary, present in the deterministic managed-output plan, a symlink, matched by `contextIgnore`, absent from the selected snapshot, then covered or eligible-unowned. The output-plan check therefore recognizes an absent planned output as `generated-output`; an ignored ancestor does not conceal a nested adopter. A nested adopter's navigation root is its descendant `.awf/config.yaml` boundary. A symlink is never followed for expansion or authority resolution, and reports whether its target remains inside the repository without allowing the query to escape through cleaning, an absolute path, or link traversal.

4. Coverage eligibility remains unchanged in principle. Generated outputs, `contextIgnore` matches, symlinks, deletions, and nested adopters stay out of whole-tree coverage and directory expansion. Explicit context queries classify these paths instead of forcing them into coverage or dropping them. Domain ownership, topic applicability, and authority resolution are computed independently of eligibility when a classified path can safely be matched without traversal, so an owned ignored or generated path still receives its applicable claims, pending operations, and navigation. `contextIgnore` remains the adopter's deliberate mechanism for excluding repository surfaces that need no domain ownership floor.

5. Known awf artifacts receive an additive attribution block even when their primary classification is ignored or generated. Attribution records use the closed, ordered role set `config`, `lock`, `manifest`, `template`, `convention-part`, `authored-data`, `topic-metadata`, `claim-part`, `decision-record`, and `managed-output`. A path may carry multiple roles when it genuinely serves more than one role, including an in-place authored source and output; records retain this order in human and JSON projections. Recognition comes only from the loaded config tree, layout, catalog, output plan, manifest, topic layout, and ADR corpus. Each record identifies authored source paths, generated output paths, owning catalog or singleton identity where applicable, and useful config or documentation navigation. No second persisted attribution ledger or heuristic lookalike classification is introduced.

6. Managed-output attribution uses the deterministic output plan as its source of truth and the manifest only for snapshot identity and drift metadata. A generated path links back to the template, convention part or authored data source that produced it when that relationship exists. An authored sidecar or part links forward to every enabled output it influences. Local reservations and unmanaged lookalikes are never labeled as generated artifacts.

7. An explicit path matching `docs/decisions/NNNN-*.md` is a first-class ADR artifact query. The concise projection reports number, title, lifecycle status, frozen or mutable state, and the authority role: ADR prose is decision history or pending intent, never active current-state authority. Its operations are partitioned with ADR-0143's canonical operation progress into Proposed, Remaining, Applied, and Canceled as applicable; Applied operations carry their batch state sequence.

8. ADR artifact operation entries link to their qualified topic and, when present, active claim. They distinguish an active current claim, a historically removed claim, and a not-yet-current add without inventing a tombstone. Proposed operations are mutable intent, Accepted operations are Remaining, Implementing operations partition into Applied and Remaining, Implemented operations are Applied, and a partially Abandoned ADR partitions into Applied and Canceled. Abandonment never relabels an applied operation as canceled.

9. Default context is concise. For an ordinary covered path it includes only claims directly selected by an exact `state:` marker, identified by a `touches-state:` marker on that path, or backed by a proof marker on that path. It still identifies every applicable topic and reports the count of omitted topic-wide claims with an `awf topic <id>` drilldown, so omission is explicit rather than a weakening of authority. `touches-state:` remains advisory navigation and never becomes backing or authority; a proof marker selects its backed invariant because it is direct evidence for that claim.

10. `awf context --full` returns the complete applicable authority packet for every effective path. It includes every current claim in each applicable topic, invariant backing and proof sites, direct marker and implementation sites, topic and domain scopes, current matched paths, pending ADR operations, artifact attribution, and sorted direct incoming and outgoing claim-reference IDs. Referenced claim bodies remain explicit `awf topic` drilldowns rather than recursively expanding the packet. Full output is never truncated by the concise projection or the topic-size advisory.

11. For an explicit ADR artifact, `--full` adds only operation-linked current or historical claim details, including direct provenance, backing, marker sites, and removal history where available. It does not expand unrelated ADR history, plans, tags, or relations. Ordinary path context likewise continues to exclude unrelated ADR history.

12. `--json` changes serialization only. Default `--json` serializes the concise semantic projection; `--full --json` serializes the full projection. Human and JSON rendering consume the same result model in both cases, and field collections are deterministic and non-null where an empty collection is semantically meaningful.

13. `--full` is valid with explicit paths, `--staged`, and `--range`. It is rejected with `--uncovered`, because that mode is a coverage-gap report rather than a context projection. Outside an adopted tree, concise and full invocations both retain the successful static fallback and state that live classification and authority require an adopted project.

14. Topic applicability presentation reports owning-domain selectors and topic selectors separately and states that both must match. It does not claim symbolic glob intersection. It adds concrete currently matched paths and configured `state:`, `touches-state:`, and proof marker sites as navigation evidence. `awf topic --coverage` and the context topic blocks share this applicability model.

15. Workflow skills and managed guidance that require the complete authority packet invoke `awf context --full` explicitly. Human-oriented or initial exploratory guidance may use concise context. No caller may assume that bare context contains every topic-wide claim after this decision.

16. The `currentState` config gains a positive `maxClaimsPerTopic` advisory threshold with a default of 20. `awf check` emits one non-failing note per topic strictly above the threshold, naming its claim count, configured limit, and the topic metadata and claim-part paths to consider splitting. The threshold never truncates query output and initially has no error severity. Config parsing, configspec, render hashing, lock and manifest inputs, migration, generated reference, scaffold defaults, and validation expose one consistent value. Omission remains backward-compatible and reads as 20; the schema generation advances, while `awf init` and the corresponding upgrade migration write the explicit default into adopted config.

17. Executable surfaces receive current-state ownership independent of eligibility. This repository creates `rendering/adapter-outputs` with the test-backed `generated-adapter-runtime-ownership` claim and assigns `.pi/extensions/**` to it. It also creates `tooling/test-infrastructure` with the test-backed `test-support-leaf-boundary` claim, removes `internal/testsupport/**` from `contextIgnore`, and assigns that tree to the topic. Topic paths remain bounded by their domain paths. Generated extension outputs remain excluded from coverage through generated-output classification, not through lack of ownership.

18. The CLI command table gains explicit runner-forwarding metadata for every top-level command. A command is either forwarded or excluded with a nonempty reason, and metadata-forwarded names are reserved from adopter project verbs. The initial exclusions are exactly `init`, because it is meaningful before adoption; `upgrade`, because it must cross rather than reuse the pinned bootstrap boundary; and `uninstall`, because runner-mediated self-removal is unsafe. Every other current top-level command, including `topic`, `config`, `list`, `enable`, `disable`, `changelog`, and `version`, is forwarded. Help-only aliases follow their owning command rather than creating a second list.

19. The rendered runner dispatch and usage are generated from the command metadata, not a hand-maintained verb enumeration. Rendering rejects a project-verb collision with a reserved forwarded name. This repository's source-running `./x` declares its project-only verbs and exclusions but has parity tests requiring every metadata-forwarded awf command to delegate through `go run ./cmd/awf`. Adding or changing a command without resolving both managed and repository runner availability fails tests.

20. Context assembly remains read-only and snapshot-consistent. Working queries load one working universe; staged queries load config, lock, corpus, marker index, output attribution inputs, path classifications, and expansions from one immutable index universe. `--range` remains only a changed-path selector and classifies the selected paths against that same single working universe rather than combining authority from its endpoint revisions. The richer result does not write config, lock, output, or caches.

21. Implementation updates CLI help, generated configuration and workflow documentation, the architecture and glossary, relevant skill templates and project convention parts, the authored AGENTS convention source, rendered `AGENTS.md`, and examples. Every lifecycle transition runs `./x sync` so `docs/decisions/INDEX.md` and all affected rendered guidance land in the same transaction. Publication-safe templates retain coherent missing-value output, and rendered files travel with their authored sources.

22. Every new State changes claim in this decision is a test-backed invariant. Updates preserve the existing Origin, prior Revised-by prefix, and backing contract while adding this ADR's revision. Each ordered Applied event lands in the same checked transaction as exactly its matching provenance-preserving claim mutations and proof markers; topic metadata shells exist before the ADR reaches Accepted, and no Remaining operation's claim effect lands early.

## State changes

- update `tooling/cli:cli-command-spec-single-source`
- update `tooling/cli:context-default-excludes-history`
- update `tooling/cli:context-output-parity`
- update `tooling/cli:context-static-fallback`
- update `tooling/cli:context-read-only`
- add `tooling/cli:context-path-attribution`
- add `tooling/cli:context-path-classification`
- add `tooling/cli:context-known-artifact-navigation`
- add `tooling/cli:context-adr-operation-projection`
- add `tooling/cli:context-full-authority-packet`
- add `tooling/cli:context-applicability-navigation`
- add `tooling/cli:topic-claim-budget-advisory`
- add `tooling/cli:managed-runner-command-parity`
- add `config/configuration:topic-claim-budget-configured`
- add `rendering/project-output-plan:managed-output-attribution`
- add `rendering/adapter-outputs:generated-adapter-runtime-ownership`
- add `tooling/test-infrastructure:test-support-leaf-boundary`

## Consequences

An agent can begin with one concise command, retain which input produced each fact, and drill into full authority, topic detail, code, tests, generated outputs, or an ADR's exact operation progress without mistaking decision prose for current truth. Large topics remain authoritative while becoming visible as an authoring smell, and the full packet stays lossless.

The semantic result and path classifier become substantially richer. Snapshot-aware artifact attribution must reuse existing layout and output-plan authorities instead of accumulating path heuristics. Human output will be longer for multi-path queries, but per-path grouping and concise defaults make it more actionable than the current union.

The default projection intentionally stops printing every claim in an applicable topic. This is safe only because it explicitly reports omitted topic-wide constraints, provides drilldowns, and makes `--full` mandatory for complete-authority callers. Documentation and skills must change in the same implementation batches so agents do not silently rely on the former default.

The topic-size threshold may produce new advisory notes in existing projects, especially this repository's broad historical topics. It does not block adoption or force a migration. Maintainers can split topics over time, tune the positive threshold, or accept the note while preserving all authority.

Runner coverage becomes mechanically complete but requires command authors to make an explicit forwarding decision. Lifecycle commands that cannot safely use the pinned bootstrap remain available as direct `awf` commands with their exclusion reason discoverable in metadata and tests.

The config addition advances the schema generation, but old trees remain readable through the default. Upgrade and scaffold serialization converge them on the explicit value, and every config, render-hash, lock, manifest, validation, and reference consumer observes the same threshold.

Incremental Applied batches allow the command model, artifact attribution, query projections, claim migrations, documentation, and repository ownership changes to land as independently truthful commits. Every test-backed addition and provenance-preserving update is paired atomically with its Applied event. Until the final batch, Remaining operations continue to appear as pending intent under ADR-0143.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the unioned full-claim default and add more fields | It preserves completeness but worsens the noise and still loses which path selected each authority item. |
| Make concise JSON serialize the full model | JSON would no longer mean the same projection as human output, encouraging callers to depend on accidental hidden fields. |
| Treat ignored and generated paths as unowned | It conflates deliberate eligibility policy with missing domain ownership and gives agents the wrong remediation. |
| Remove `contextIgnore` and require ownership of every tracked path | Generated, vendored, metadata, and infrastructure surfaces legitimately need exclusion from whole-tree coverage; explicit classification provides visibility without a universal ownership tax. |
| Compute symbolic intersections of domain and topic globs | The glob language does not provide a reliable simple intersection witness; separate selectors plus concrete current matches are honest and navigable. |
| Persist an artifact-attribution index in the lock | The output plan, catalog, layout, config tree, and ADR corpus already own the relationships; another ledger would introduce drift and migration burden. |
| Keep the runner verb list hand-maintained | It already omitted `topic` in both managed and repository runners, demonstrating that review alone does not preserve parity. |
| Add a dedicated artifact-navigation command or extend only `awf topic` | Agents begin with paths that may be code, config, generated output, topic input, or an ADR. One path-oriented context surface can classify and route all of them without requiring the caller to identify the artifact kind first; topic remains the focused claim drilldown. |
| Make the topic claim threshold an error immediately | Existing broad topics would turn an authoring-quality signal into a disruptive migration gate unrelated to authority correctness. |

## Status history

- 2026-07-21: Proposed
