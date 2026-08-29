---
format: plan-v2
date: 2026-08-29
adrs:
  - delegate-relevance-discovery-to-codegraph
status: Implemented
---
# Plan: Implement CodeGraph relevance delegation

## Goal

Replace the general context and active tag systems with focused awf authority reads and resolution, while preserving current-state enforcement, publication, drift, historical parsing, and migration safety. Do not redesign domains, topics, proof backing, ADR lifecycle, or generated-output ownership beyond the approved boundary.

## Architecture summary

CodeGraph becomes the expected source-navigation owner and Git remains the changed-path owner. `internal/currentstatecoord` assembles the working authority universe; focused operation owners expose topic reads, ADR reads, exact lexical path resolution, and the whole-repository unowned census through verb-subject CLI leaves. `awf check` remains the only enforcement owner. Publisher and staged drift retain only their independent output-plan inputs before context-specific preparation and observation models are removed.

The cutover is additive before subtractive: land replacement authority commands, move generated workflow consumers to CodeGraph plus those commands, then remove context. A schema migration removes live tags, `contextIgnore`, and pitfall tag frontmatter before strict decoding; marker parsing narrows to proof markers without rewriting arbitrary adopter source. Each phase applies only the matching ADR claim transaction and closes green.

**Plan flexibility.**

The protected-contract rule in the workflow document governs what a plan may not change. The plan records the best known route at authoring time, not a binding implementation choreography. A commit-capable owner may merge, split, reorder, add, remove, or replace recorded route detail while the protected contract holds. A path omitted from the plan is not alone a reason to stop, and a stale listed path need not be touched. Reapproval is required only when the protected contract would change or an unresolved material decision appears.

Reconcile a Proposed plan only when another phase or reviewer could rely on stale material instructions. Inconsequential and independently local edits require no deviation record. A delegated owner reports material cross-owner revisions for parent reconciliation. A helper remains confined to its assigned paths and gains no scope, commit, review, checkpoint, handoff, or outcome authority from route flexibility.

## Phase 1: Add focused authority operations

**Execution mode: subagent-driven.**

Completes: ["authority-commands"]

### Task 1.1: Establish command contracts and focused operation owners
Kind: batch
Applying: ["delegate-relevance-discovery-to-codegraph:focused-authority-commands", "delegate-relevance-discovery-to-codegraph:governance-core-retained"]
Paths: ["cmd/awf", "internal/clispec", "internal/currentstatecoord", "internal/topicop", "internal/topic", "internal/adr", "internal/plan", "internal/plancheck", "internal/presentation", "internal/testsupport", "README.md"]
Representative: ["`awf read topic <domain>/<topic>[:<claim>]` preserves the current topic query projections", "`awf read adr <identity>` reports status, canonical Applied/Remaining/Canceled progress, and parsed linked plans", "`awf resolve topic <path>...` reports per-path domains and applicable topics or explicit none"]
Edge: ["normalized repository-relative proposed paths that do not exist", "global topics outside bounded ownership", "duplicate paths", "repository-external or malformed paths", "`--uncovered` combined with any positional path"]
Post-check: "Run focused command, clispec, topic, ADR, plan-link, and current-state coordinator tests. Require exact verb-subject help and dispatch, a regenerated clispec-derived README command block, successful explicit-none results, deterministic per-path attribution, a whole-repository-only informational uncovered census with topmost-directory collapse, ordinary presentation output, and unchanged topic history, references, coverage, proof-site, and ADR parser semantics. The legacy `awf topic` and `awf context` routes remain available only until their later removal phases."

Add durable tests before or with each consumer so every new command is exercised through its semantic owner and the real command boundary. Keep path resolution lexical and read-only: it may consult the working authority corpus and inventory required by the uncovered census, but it must not import publisher coordination, render artifacts, Git selection modes, or context representations.

### Task 1.2: Apply the replacement-command authority batch
Kind: batch
Applying: ["delegate-relevance-discovery-to-codegraph:focused-authority-commands", "delegate-relevance-discovery-to-codegraph:governance-core-retained"]
Paths: [".awf/topics/parts/tooling/authority-queries/current-state.md", ".awf/topics/parts/tooling/cli/current-state.md", ".awf/topics/parts/invariants/current-state-authority/current-state.md", "docs/decisions/delegate-relevance-discovery-to-codegraph.md", "docs/decisions/INDEX.md", "glob:docs/topics/{tooling,invariants}/*.md", ".awf/awf.lock"]
Post-check: "Enter Implementing and apply exactly the adds `tooling/authority-queries:{authority-read-projections,path-topic-resolution,unowned-path-census,authority-query-read-only,authority-query-full-profile-only}`, `tooling/cli:init-describe-read-only`, and `invariants/current-state-authority:production-packages-domain-owned`. Render, then require `awf check staged` to accept the claim mutations and require the ADR progress projection to report exactly this batch Applied while every other declaration remains Remaining."

Write claims against the focused owners and existing Full-profile boundary. Do not yet claim the CodeGraph workflow cutover or remove any context claim.

### Phase close

The phase owner closes one independently green additive transaction.

```commit
feat(tooling): add focused authority queries
```

## Phase 2: Move workflow navigation to CodeGraph and focused authority reads

**Execution mode: subagent-driven.**

Completes: ["codegraph-workflow"]

### Task 2.1: Rewrite the canonical guidance and generated consumers
Kind: batch
Applying: ["delegate-relevance-discovery-to-codegraph:external-navigation-boundary", "delegate-relevance-discovery-to-codegraph:focused-authority-commands", "delegate-relevance-discovery-to-codegraph:governance-core-retained"]
Paths: ["README.md", "templates/agents-doc", "templates/skills", "templates/agents", "templates/partials", ".awf/skills", ".awf/parts", ".awf/docs/parts", "internal/catalog", "internal/evals", "internal/project", "glob:.pi/skills/awf-*/SKILL.md", "glob:.claude/skills/awf-*/SKILL.md", "glob:.pi/agents/*.md", "glob:.claude/agents/*.md", "AGENTS.md", "docs/workflow.md", "docs/working-with-awf.md", "docs/pi-runtime-reference.md", "docs/debugging.md", "examples"]
Representative: ["orientation uses CodeGraph for code discovery, then `awf resolve topic` and `awf read topic` for normative authority", "ADR lifecycle and review use `awf read adr` rather than context facets", "Git supplies staged or range path selection"]
Edge: ["Core guidance where current-state commands are unavailable", "configured and unset template variables", "review flows with no linked plans", "non-code exact-file orientation where CodeGraph is unnecessary"]
Post-check: "Run focused project, catalog, eval, and generated-consumer contract tests across Full and Core fixtures, configured and empty variables, and Pi and Claude outputs. Require one CodeGraph expectation in the canonical reader guidance, action-first routing to the focused awf commands, no active `awf context` invocation, no context-spill recovery clause, coherent Core degradation, and no unresolved or `<no value>` token. Review representative rendered orientation, ADR, plan, review, and debugging prose for equivalent normative authority handling without a parallel navigation fallback."

Retain exact-known-file and trivial lookup behavior; the external navigation boundary does not force CodeGraph where no discovery or structural question exists. Remove only context-era instructions and spill mechanics, not generic fresh-context subagent or ordinary runtime context concepts.

### Task 2.2: Apply the workflow boundary claims
Kind: batch
Applying: ["delegate-relevance-discovery-to-codegraph:external-navigation-boundary", "delegate-relevance-discovery-to-codegraph:focused-authority-commands"]
Paths: [".awf/topics/parts/tooling/authority-queries/current-state.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", "docs/decisions/delegate-relevance-discovery-to-codegraph.md", "docs/decisions/INDEX.md", "docs/topics/tooling/authority-queries.md", "docs/topics/rendering/workflow-skill-templates.md", ".awf/awf.lock"]
Post-check: "Apply exactly the add `tooling/authority-queries:codegraph-navigation-boundary` and updates `rendering/workflow-skill-templates:{implementer-context-grounding,implementer-role-contract,explorer-and-grounding-role-contracts,orienting-single-home,closed-workflow-profiles}`. Render and require staged current-state checks to accept the batch, generated-prose contract tests to pass, and ADR progress to leave only undeclared later batches Remaining."

### Phase close

```commit
docs(rendering): route navigation through CodeGraph
```

## Phase 3: Remove the context subsystem

**Execution mode: subagent-driven.**

Completes: ["context-removed"]

### Task 3.1: Separate publication and drift inputs from context preparation
Kind: batch
Applying: ["delegate-relevance-discovery-to-codegraph:governance-core-retained", "delegate-relevance-discovery-to-codegraph:clean-cutover"]
Paths: ["internal/publisher", "internal/outputplan", "internal/project", "internal/checkop", "internal/currentstatecoord", "internal/projectstate", "internal/generatedcheck", "internal/testsupport"]
Post-check: "Run publisher, output-plan, project, check-operation, current-state coordinator, and staged drift suites over working and index universes. Require render and drift to retain one selected-universe route, complete declaration and collision behavior, source and template hashes, lock membership, and generated-path classification without importing or constructing a context snapshot. Prove every removed observation field has no non-context consumer before deletion."

Make the minimal enabling refactor first. Do not move publication or drift policy into the replacement queries, and do not preserve context-shaped compatibility values after their final consumer is gone.

### Task 3.2: Delete the command, packages, delivery protocol, and observability
Kind: batch
Applying: ["delegate-relevance-discovery-to-codegraph:external-navigation-boundary", "delegate-relevance-discovery-to-codegraph:clean-cutover"]
Paths: ["cmd/awf", "cmd/contextspilllog", "internal/contextdelivery", "internal/contextinput", "internal/contextop", "internal/contextq", "internal/contextspill", "internal/currentstatecoord", "internal/project", "internal/publisher", "internal/resident", "internal/clispec", "x", ".gitignore", ".githooks", ".github", "internal/testsupport", "README.md"]
Representative: ["CLI membership and help", "focused working and staged preparation", "spill delivery and logging", "artifact relationship projection", "context-only output observation"]
Edge: ["outside-adoption fallback", "oversize output", "staged and range selection", "directory request grouping", "artifact provenance facets"]
Post-check: "Run a checked repository-wide source and command-spec probe and require no production `awf context` command, context package import, spill notice/logger, context cache/log path, facet, staged/range context selector, directory census, classification, or artifact-navigation projection. Require old command forms to fail as unknown usage, while Phase 1 read and resolve commands and all publication, drift, check, and output-plan suites pass."

Delete obsolete packages and tests rather than moving them under a new name. Retain similarly named Go `context.Context` helpers and generic runtime context prose; the residue probe must distinguish those unrelated meanings.

### Task 3.3: Retire context authority and reconcile surviving claims
Kind: batch
Applying: ["delegate-relevance-discovery-to-codegraph:focused-authority-commands", "delegate-relevance-discovery-to-codegraph:clean-cutover"]
Paths: [".awf/topics/metadata/tooling/context-and-topic.yaml", ".awf/topics/parts/tooling/context-and-topic/current-state.md", ".awf/topics/parts/invariants/current-state-authority/current-state.md", ".awf/topics/parts/adr-system/adr-lifecycle/current-state.md", ".awf/topics/parts/tooling/cli/current-state.md", ".awf/topics/parts/rendering/sync-and-drift/current-state.md", ".awf/topics/parts/code-design/dependency-composition/current-state.md", ".awf/topics/parts/code-design/state-ownership/current-state.md", "docs/decisions/delegate-relevance-discovery-to-codegraph.md", "docs/decisions/INDEX.md", "glob:docs/topics/{tooling,invariants,adr-system,rendering,code-design}/*.md", "glob:docs/domains/*.md", ".awf/awf.lock"]
Post-check: "Remove every declared `tooling/context-and-topic:*` claim and its now-empty topic inputs; apply the updates `invariants/current-state-authority:{accepted-authority-is-pending-only,accepted-does-not-override-current,current-state-sole-active-authority,historical-rationale-is-explicit}`, `adr-system/adr-lifecycle:corpus-owns-status-literals`, `tooling/cli:{explicit-output-bypasses,check-severity-by-protected-property}`, `rendering/sync-and-drift:managed-output-attribution`, `code-design/dependency-composition:repository-layer-direction`, and `code-design/state-ownership:project-derived-state-ownership`; remove `invariants/current-state-authority:uncovered-lists-unowned-unignored`, add and correct `invariants/current-state-authority:uncovered-lists-unowned`, and strengthen the surviving proof sites. Render and require staged checking to accept the complete batch, no generated link or dependency fixture to the removed context subsystem, and ADR progress to show these operations Applied."

### Phase close

```commit
feat(tooling): remove context subsystem
```

## Phase 4: Retire active tags, context exclusions, and navigation markers

**Execution mode: subagent-driven.**

Completes: ["relevance-retired", "migration-compatible"]

### Task 4.1: Add the schema migration before narrowing live decoding
Kind: batch
Applying: ["delegate-relevance-discovery-to-codegraph:relevance-metadata-retirement", "delegate-relevance-discovery-to-codegraph:clean-cutover"]
Paths: ["internal/migrate", "internal/upgrade", "internal/audit", "internal/config", "internal/configspec", "internal/manifest", "internal/pitfall", "internal/publisher", "internal/project", "internal/projectstate", "internal/testsupport", "cmd/awf", "glob:cmd/awf/testdata/**", "glob:internal/**/testdata/**", "glob:examples/**/.awf/**", "docs/config-reference.md"]
Representative: ["top-level config `tags`", "top-level `contextIgnore`", "pitfall `tags:` frontmatter"]
Edge: ["flow and block YAML forms", "absent and empty fields", "pitfalls without domains", "interrupted upgrade recovery", "staged lock and manifest regeneration", "frozen legacy ADR tags"]
Post-check: "Observe migration fixtures from the current schema fail against the narrowed live parser before the migration is registered, then pass after implementation. Require upgrade to remove all three live surfaces before strict decoding, preserve comments and unrelated formatting where the established migration contract does, leave frozen legacy ADR parsing intact, regenerate outputs and lock once, recover through the existing journal after injected interruption, and reject retired fields in a post-upgrade live config or pitfall source."

Do not scan or rewrite arbitrary adopter production source. The migration owns config-tree and authored pitfall inputs only.

### Task 4.2: Remove tag policy and rendering while preserving pitfall domains
Kind: batch
Applying: ["delegate-relevance-discovery-to-codegraph:relevance-metadata-retirement", "delegate-relevance-discovery-to-codegraph:governance-core-retained"]
Paths: ["internal/vocabularycheck", "internal/glossarycheck", "internal/pitfall", "internal/pitfallcheck", "internal/publisher", "internal/repositorycheck", "internal/configcheck", "internal/config", "internal/configspec", "internal/project", "internal/projectstate", "internal/testsupport", "templates/docs/pitfalls.md.tmpl", "templates/pitfalls", ".awf/docs/pitfalls", ".awf/config.yaml", ".awf/domains/rendering.yaml", ".awf/topics/metadata/rendering/doc-outputs.yaml", "docs/pitfalls.md", "glob:docs/pitfalls/*.md", "docs/glossary.md", "x", "changelog/CHANGELOG.md"]
Representative: ["remove vocabulary membership, collision, missing-tag, and frequency findings", "remove tag columns and leaf metadata", "retain optional validated domains, related ADRs, and generated By-domain grouping"]
Edge: ["no-domain pitfalls", "multiple domains", "legacy ADR fixtures with tags", "glossary checks formerly sharing the vocabulary owner"]
Post-check: "Run focused pitfall parse, render, check aggregation, glossary, configuration, and project suites. Require strict authored pitfall metadata to admit only the retained fields, generated index and leaves to contain no tag label or value, domain grouping and ADR links to remain deterministic, glossary behavior to retain an appropriate single owner, and repository checks to emit no tag property, warning, or information. A checked literal and paraphrase sweep over active config, templates, code, tests, and generated docs has no live tag-vocabulary contract while frozen legacy ADR fixtures remain."

Rename or dissolve a tag-oriented owner only when required for a truthful surviving glossary boundary; do not preserve an empty compatibility checker.

### Task 4.3: Narrow marker parsing to proof and remove the coverage escape hatch
Kind: batch
Applying: ["delegate-relevance-discovery-to-codegraph:relevance-metadata-retirement", "delegate-relevance-discovery-to-codegraph:governance-core-retained"]
Paths: ["internal/topic", "internal/currentstate", "internal/currentstatecoord", "internal/config", "internal/configspec", "internal/project", "internal/testsupport", ".awf/config.yaml", "glob:**/*.go", "glob:templates/**", "glob:.awf/**/parts/**/*.md", "docs/doc-standard.md"]
Representative: ["`invariant:` proof markers continue to require named in-scope tests", "`state:` and `touches-state:` become unrecognized inert comments", "domain-owned paths receive coverage without `contextIgnore` filtering"]
Edge: ["configured line and block comment families", "close-token validation", "unbacked claims", "proposed or untracked paths", "generated outputs and nested adopters"]
Post-check: "Run topic marker, current-state, config, project, and staged-universe suites. Require proof-marker missing, scope, name, close-token, and unbacked-refusal cases to retain behavior; state and touches payloads create no relationship or validation result; currentState source families used only for retired authoring comments are removed; every ordinary domain-owned eligible path participates in coverage; generated outputs and nested adopters retain their independent exclusions. Sweep the repository-owned source and template population and require no active state/touches marker after confirming the probe ran."

### Task 4.4: Apply the schema, marker, and reviewed settlement batches
Kind: batch
Applying: ["delegate-relevance-discovery-to-codegraph:relevance-metadata-retirement", "delegate-relevance-discovery-to-codegraph:governance-core-retained", "delegate-relevance-discovery-to-codegraph:clean-cutover"]
Paths: [".awf/topics/parts/config/configuration/current-state.md", ".awf/topics/parts/config/validation/current-state.md", ".awf/topics/parts/rendering/doc-outputs/current-state.md", ".awf/topics/parts/invariants/topics-and-markers/current-state.md", ".awf/topics/parts/invariants/current-state-authority/current-state.md", ".awf/topics/parts/tooling/upgrade-runtime/current-state.md", ".awf/topics/parts/code-design/dependency-composition/current-state.md", ".awf/topics/parts/tooling/audit-and-snapshots/current-state.md", "docs/decisions/delegate-relevance-discovery-to-codegraph.md", "docs/decisions/INDEX.md", "docs/topics/code-design/dependency-composition.md", "glob:docs/topics/{config,rendering,invariants,tooling}/*.md", ".awf/awf.lock"]
Post-check: "Preserve the landed Applied batch containing exactly the removals `config/configuration:{tag-coverage-note,tag-frequency-note,tag-vocabulary-governed}`, `config/validation:tag-not-domain-name`, and `invariants/topics-and-markers:{relevance-markers-only-narrow,touches-marker-advisory}`; the adds `config/configuration:no-active-tag-system`, `invariants/current-state-authority:domain-owned-coverage-no-ignore`, and `invariants/topics-and-markers:proof-only-marker-grammar`; and the updates `rendering/doc-outputs:pitfall-corpus-validated`, `invariants/topics-and-markers:{claim-id-qualified,invariant-marker-close-token,invariants-three-state}`, and `tooling/upgrade-runtime:upgraded-runtime-has-one-authority-engine`. In the later reviewed settlement batch, update `code-design/dependency-composition:repository-extraction-owners` from `VocabularyChecker` to `GlossaryChecker` ownership and `tooling/audit-and-snapshots:managed-history-decode-horizon` from horizon 46 to 47. Render atomically with each claim mutation and append the matching Applied event. Require staged checking and ADR progress to report no Remaining operation while status stays Implementing."

### Phase close

```commit
feat(config): retire active relevance metadata
```

## Phase 5: Reconcile documentation and terminal assurance

**Execution mode: inline.**

Completes: ["terminal-assurance"]

### Task 5.1: Render, document, and prove the clean cutover
Kind: batch
Applying: ["delegate-relevance-discovery-to-codegraph:external-navigation-boundary", "delegate-relevance-discovery-to-codegraph:focused-authority-commands", "delegate-relevance-discovery-to-codegraph:relevance-metadata-retirement", "delegate-relevance-discovery-to-codegraph:governance-core-retained", "delegate-relevance-discovery-to-codegraph:clean-cutover"]
Paths: ["README.md", "changelog/CHANGELOG.md", ".awf/awf.lock", ".awf/topics/parts/rendering/project-output-plan/current-state.md", "docs/architecture.md", "docs/config-reference.md", "docs/debugging.md", "docs/doc-standard.md", "docs/glossary.md", "docs/pitfalls.md", "docs/testing.md", "docs/working-with-awf.md", "docs/workflow.md", "docs/decisions/INDEX.md", "docs/decisions/delegate-relevance-discovery-to-codegraph.md", "docs/plans/2026-08-29-implement-codegraph-relevance-delegation.md", "glob:docs/domains/*.md", "glob:docs/topics/**/*.md", "glob:docs/pitfalls/*.md", "glob:.pi/**", "glob:.claude/**", "glob:examples/**"]
Post-check: "Remove the obsolete explicit-context clause from `rendering/project-output-plan:check-report-single-plan`, append `ADR-delegate-relevance-discovery-to-codegraph` to its `Revised-by`, render, and atomically append an Applied event for exactly `update rendering/project-output-plan:check-report-single-plan`. Run `./x render`, inspect every changed generated boundary for the approved reading, and require `./x check` to finish with no context-era or tag-health finding. Run checked active-policy sweeps for the deleted command, packages, spill protocol, tags, contextIgnore, state/touches markers, obsolete topic identity, and old topic command grammar; classify and document only frozen ADR history or inert adopter-source compatibility residue. Exercise representative read topic, read ADR, path resolution, explicit none, and whole-repository uncovered outputs. Then run affected-package feedback, `go test ./...`, `./x gate full`, and staged checks with the stable Go toolchain; all terminal gates pass and ADR progress has no Remaining operation."

Update the README with the explicit CodeGraph expectation and the command table from clispec authority. Ensure standard and generated docs describe CodeGraph as the navigation dependency, not as an awf-enforced installation check, and retain no second navigation fallback.

### Phase close

```commit
docs(awf): complete CodeGraph authority cutover
```

## Definition of done

- `dod: authority-commands` Verb-subject topic and ADR reads plus exact path resolution and the whole-repository uncovered census expose deterministic working current-state authority without context representations or enforcement side effects.
- `dod: codegraph-workflow` README and generated workflow guidance expect CodeGraph for structural source navigation, Git for change selection, and focused awf commands for normative authority.
- `dod: context-removed` The context command, packages, facets, selection modes, spill protocol and observability, artifact projection, focused preparation, and context-only publisher observations are absent with no compatibility alias.
- `dod: relevance-retired` Active config and pitfall tags, tag policy and rendering, contextIgnore, and state/touches markers are absent while pitfall domains, proof markers, domains, topics, claims, coverage, fan-out, ADR handshakes, and legacy ADR parsing remain.
- `dod: migration-compatible` Upgrade safely removes retired live schema and authored pitfall fields before strict decoding and converges render, output-plan, manifest, and lock state without rewriting arbitrary adopter source.
- `dod: terminal-assurance` Every ADR operation is Applied with the ADR still Implementing, generated prose and command output match the approved design, residue probes are clean or historically classified, and exhaustive verification passes.

## Notes

Apply the plan-flexibility rule above when recording deviations. Delegated owners report material cross-owner revisions rather than editing the plan; the parent supplies the report to phase review and reconciles required plan changes with findings in one focused post-review settlement commit before checkpointing or later execution. Record review dispositions, migration observations, residue classifications, and implementation deviations here.

- Plan review: classified the Phase 3 close as `feat(tooling)` because deleting the public context command is the transaction's observable subject; the cross-package decoupling remains bounded enabling work rather than an independently shipped refactor.
- Phase 1 review: settled six mechanical findings by canonicalizing ADR identity aliases before reverse-link comparison, rejecting presentation-invalid paths, preserving plan diagnostics, and strengthening progress, ordering, exclusion, and context-ignore-independence tests. The new production-package ownership claim remains truthfully unbacked until the later old-claim removal can relocate its proof without overlapping active claim authority.
- Phase 2 review: settled two mechanical findings by replacing the final legacy `awf topic` reviewer hint and extending the workflow-template oracle to reject that fallback and require focused topic reads. The parent completed the focused settlement inline after the delegated phase owner returned; no transaction or review authority transferred.
- Phase 3 implementation: the delegated owner removed the context subsystem at `0a67a67cf`; the parent restored broad surviving oracles, corrected candidate cleanup with a nested-deletion regression, and committed the independently green phase transaction. Review found stale active authority, weak proof placement, deleted-context dependency residue, and stale documentation. The user approved the omitted claim updates, and the reviewed ADR amendment landed at `dd8e26ed7` before settlement.
- Phase 3 settlement: apply the three amended claim updates and correct the earlier `uncovered-lists-unowned` add through distinct Applied and Reapplied events; relocate proof markers to behavior-proving tests, remove empty proof stubs and the deleted-context dependency fixture, refresh tooling ownership prose and the Unreleased changelog, render, and pass staged checking plus the fast gate. Because the amendment changes review authority and the settlement materially changes Phase 3, renew Phase 3 implementation review before checkpointing.
- Renewed Phase 3 review: restore the non-context legacy-authority denylist oracle, remove stale context owners and flow from architecture and glossary sources, and correct `uncovered-lists-unowned` to the implemented working-tree census of tracked and untracked present paths with lock-listed managed outputs excluded. The claim correction is a reasoned authority-preserving clarification, so it receives a separately observable Reapplied event, focused tracked/untracked/lock-listed command evidence, the fast gate, and the workflow's single verify pass.
- Phase 3 single-verify settlement: remove the residual top-level `awf topic` compatibility route, move its active command tests and generated drilldowns to `awf read topic`, and strengthen the uncovered assertion to exact collapsed entries. Apply `invariants/topics-and-markers:rendered-applicability-selectors-only` early from Phase 4 because the generated drilldown must remain truthful when the old route disappears, and Reapply `invariants/current-state-authority:current-state-sole-active-authority` for its matching Verify correction; Phase 4 retains the remaining proof-only marker work.
- Phase 4 implementation and settlement are complete and independently assured green through `eaa4ae55a`; Phases 1 through 4 require no renewed assurance. The reviewed ADR amendment at `0d2a735b9` adds only the Phase 5 update `rendering/project-output-plan:check-report-single-plan`. Linked-plan freshness review assigned that operation and its authored source to Task 5.1 mechanically. Phase 5 remains unlanded, with its dirty transaction stashed while this reconciliation commits.
- Phase 5 review found four stale ownership descriptions and one weakened staged-pitfall parity oracle. Settlement removes obsolete awf handoff ownership and relevance-marker wording, locates private declarations in Publisher, restores render-input provenance rationale, and compares every surviving immutable pitfall output projection across working and staged universes. The reasoned oracle strengthening preserves the declaration-projection removal while preventing content, identity, hash, policy, or metadata divergence from passing parity.

### Terminal reconciliation
Implementation range: 5a5c87379679ebe5378cacf77c7eef544828af05..9cb81a528bb8e7e6ef50c9167543dbc391c43280
Touched paths:
- ".awf/agents/code-reviewer.yaml"
- ".awf/awf.lock"
- ".awf/config.yaml"
- ".awf/docs/glossary.yaml"
- ".awf/docs/parts/architecture/components.md"
- ".awf/docs/parts/architecture/data-flow.md"
- ".awf/docs/parts/debugging/recipes.md"
- ".awf/docs/parts/debugging/surfaces.md"
- ".awf/docs/parts/roadmap/deferred.md"
- ".awf/docs/parts/testing/gate.md"
- ".awf/docs/pitfalls/a-census-number-is-only-as-good-as-its-stated-query.md"
- ".awf/docs/pitfalls/a-faked-collaborator-makes-both-the-fixtures-and-the-assertion-vocabulary-unfalsifiable.md"
- ".awf/docs/pitfalls/a-future-code-fence-marker-must-account-for-linguist-aliases.md"
- ".awf/docs/pitfalls/a-future-non-catalog-render-singleton-still-has-hand-wired-fan-out.md"
- ".awf/docs/pitfalls/a-milestone-time-check-must-not-double-as-an-every-commit-test.md"
- ".awf/docs/pitfalls/a-new-output-language-needs-an-exercised-real-render-target.md"
- ".awf/docs/pitfalls/a-plan-editing-a-catalog-template-or-default-under-enumerates-the-render-fan-out.md"
- ".awf/docs/pitfalls/a-proof-marker-does-not-prove-every-clause-in-its-invariant-claim.md"
- ".awf/docs/pitfalls/a-prose-contract-test-proves-only-the-clauses-whose-literals-occur-for-one-reason.md"
- ".awf/docs/pitfalls/a-scripted-sweep-over-adr-prose-can-silently-unmake-the-structure-it-edits.md"
- ".awf/docs/pitfalls/a-staged-symlink-fixture-needs-a-real-blob-a-gitlink-does-not.md"
- ".awf/docs/pitfalls/a-token-or-convention-rename-must-sweep-every-rendered-doc-surface.md"
- ".awf/docs/pitfalls/ad-hoc-compound-mutations-still-need-target-read-back.md"
- ".awf/docs/pitfalls/adr-adr-title-already-carries-the-adr-nnnn-prefix.md"
- ".awf/docs/pitfalls/an-ad-hoc-empty-scan-still-needs-proof-that-the-probe-ran.md"
- ".awf/docs/pitfalls/an-ad-hoc-post-check-can-still-overrun-the-change-s-scope.md"
- ".awf/docs/pitfalls/an-attribute-filtered-pinned-set-test-exempts-every-other-attribute-value.md"
- ".awf/docs/pitfalls/an-ordered-phrase-assertion-cannot-reach-a-site-ahead-of-its-first-anchor.md"
- ".awf/docs/pitfalls/an-ordering-proof-written-against-the-log-proves-nothing.md"
- ".awf/docs/pitfalls/check-the-open-boundary-before-ignoring-an-assembly-error.md"
- ".awf/docs/pitfalls/enabled-linters-constrain-api-shape-sketch-signatures-against-them.md"
- ".awf/docs/pitfalls/gofmt-rewrites-double-backticks-in-doc-comments-into-curly-quotes.md"
- ".awf/docs/pitfalls/keep-recovery-ui-writes-non-fatal-after-session-disposal.md"
- ".awf/docs/pitfalls/keep-ssh-alive-through-long-pre-push-gates.md"
- ".awf/docs/pitfalls/link-adrs-by-their-on-disk-filename-never-by-constructing-one-from-the-title.md"
- ".awf/docs/pitfalls/make-custom-staged-slice-hooks-explicit-about-branch-and-cleanup.md"
- ".awf/docs/pitfalls/moving-a-check-earlier-in-the-pipeline-steals-a-later-stage-s-error-branch-coverage.md"
- ".awf/docs/pitfalls/obsoleting-rendered-prose-sweep-parts-and-whole-narratives-not-just-templates.md"
- ".awf/docs/pitfalls/pin-the-go-toolchain-when-preview-compilers-break-lint.md"
- ".awf/docs/pitfalls/port-a-stale-branch-before-merging-a-breaking-marker-grammar.md"
- ".awf/docs/pitfalls/raw-byte-adr-surgery-must-bound-every-scan-to-the-frontmatter-window.md"
- ".awf/docs/pitfalls/raw-byte-offsets-go-stale-the-moment-an-earlier-pass-edits-the-file.md"
- ".awf/docs/pitfalls/recheck-closure-assertions-after-changing-catalog-edges.md"
- ".awf/docs/pitfalls/reconcile-the-exact-mutable-artifact-not-a-similarly-named-predecessor.md"
- ".awf/docs/pitfalls/resolve-review-ranges-from-git-rather-than-transcribing-shas.md"
- ".awf/docs/pitfalls/retiring-a-concept-needs-paraphrase-sweeps-not-just-identifier-greps.md"
- ".awf/docs/pitfalls/reuse-the-repository-boundary-for-new-filesystem-walks.md"
- ".awf/docs/pitfalls/scope-a-claim-to-the-command-s-actual-input.md"
- ".awf/docs/pitfalls/sidecar-data-is-not-placeholder-substituted-drop-awf-escapes-when-converting-a-part.md"
- ".awf/docs/pitfalls/use-absolute-generations-for-historical-migration-shapes.md"
- ".awf/docs/pitfalls/when-retiring-a-config-key-handle-historical-writers.md"
- ".awf/domains/config.yaml"
- ".awf/domains/parts/invariants/current-state.md"
- ".awf/domains/parts/rendering/current-state.md"
- ".awf/domains/parts/tooling/current-state.md"
- ".awf/domains/rendering.yaml"
- ".awf/domains/tooling.yaml"
- ".awf/parts/agents-doc/commands.md"
- ".awf/topics/metadata/config/configuration.yaml"
- ".awf/topics/metadata/config/validation.yaml"
- ".awf/topics/metadata/invariants/topics-and-markers.yaml"
- ".awf/topics/metadata/rendering/doc-outputs.yaml"
- ".awf/topics/metadata/tooling/authority-queries.yaml"
- ".awf/topics/metadata/tooling/context-and-topic.yaml"
- ".awf/topics/parts/adr-system/adr-lifecycle/current-state.md"
- ".awf/topics/parts/code-design/dependency-composition/current-state.md"
- ".awf/topics/parts/code-design/state-ownership/current-state.md"
- ".awf/topics/parts/config/configuration/current-state.md"
- ".awf/topics/parts/config/validation/current-state.md"
- ".awf/topics/parts/invariants/current-state-authority/current-state.md"
- ".awf/topics/parts/invariants/topics-and-markers/current-state.md"
- ".awf/topics/parts/rendering/adapter-outputs/current-state.md"
- ".awf/topics/parts/rendering/doc-outputs/current-state.md"
- ".awf/topics/parts/rendering/pi-runtime/current-state.md"
- ".awf/topics/parts/rendering/pi-workflows/current-state.md"
- ".awf/topics/parts/rendering/project-output-plan/current-state.md"
- ".awf/topics/parts/rendering/sync-and-drift/current-state.md"
- ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md"
- ".awf/topics/parts/tooling/audit-and-snapshots/current-state.md"
- ".awf/topics/parts/tooling/authority-queries/current-state.md"
- ".awf/topics/parts/tooling/cli/current-state.md"
- ".awf/topics/parts/tooling/context-and-topic/current-state.md"
- ".awf/topics/parts/tooling/quality-gates/current-state.md"
- ".awf/topics/parts/tooling/upgrade-runtime/current-state.md"
- ".claude/agents/code-reviewer.md"
- ".claude/agents/grounding-checker.md"
- ".claude/agents/implementer.md"
- ".claude/agents/plan-reviewer.md"
- ".claude/skills/awf-adr-lifecycle/SKILL.md"
- ".claude/skills/awf-brainstorming/SKILL.md"
- ".claude/skills/awf-bugfix/SKILL.md"
- ".claude/skills/awf-debugging/SKILL.md"
- ".claude/skills/awf-executing-direct/SKILL.md"
- ".claude/skills/awf-executing-plans/SKILL.md"
- ".claude/skills/awf-grounding/SKILL.md"
- ".claude/skills/awf-orienting/SKILL.md"
- ".claude/skills/awf-refactor-coupling-audit/SKILL.md"
- ".claude/skills/awf-reviewing-adr/SKILL.md"
- ".claude/skills/awf-reviewing-impl/SKILL.md"
- ".claude/skills/awf-reviewing-plan/SKILL.md"
- ".claude/skills/awf-subagent-driven-development/SKILL.md"
- ".claude/skills/awf-tdd/SKILL.md"
- ".claude/skills/awf-writing-plans/SKILL.md"
- ".githooks/pre-commit"
- ".github/workflows/release.yml"
- ".gitignore"
- ".pi/agents/code-reviewer.md"
- ".pi/agents/grounding-checker.md"
- ".pi/agents/implementer.md"
- ".pi/agents/plan-reviewer.md"
- ".pi/extensions/awf-effort/index.ts"
- ".pi/extensions/awf-subagents/index.ts"
- ".pi/skills/awf-adr-lifecycle/SKILL.md"
- ".pi/skills/awf-brainstorming/SKILL.md"
- ".pi/skills/awf-bugfix/SKILL.md"
- ".pi/skills/awf-debugging/SKILL.md"
- ".pi/skills/awf-executing-direct/SKILL.md"
- ".pi/skills/awf-executing-plans/SKILL.md"
- ".pi/skills/awf-grounding/SKILL.md"
- ".pi/skills/awf-orienting/SKILL.md"
- ".pi/skills/awf-refactor-coupling-audit/SKILL.md"
- ".pi/skills/awf-reviewing-adr/SKILL.md"
- ".pi/skills/awf-reviewing-impl/SKILL.md"
- ".pi/skills/awf-reviewing-plan/SKILL.md"
- ".pi/skills/awf-subagent-driven-development/SKILL.md"
- ".pi/skills/awf-tdd/SKILL.md"
- ".pi/skills/awf-writing-plans/SKILL.md"
- "AGENTS.md"
- "README.md"
- "changelog/CHANGELOG.md"
- "cmd/awf/audit_test.go"
- "cmd/awf/authority.go"
- "cmd/awf/authority_test.go"
- "cmd/awf/check_test.go"
- "cmd/awf/context.go"
- "cmd/awf/context_test.go"
- "cmd/awf/dispatch.go"
- "cmd/awf/dispatch_test.go"
- "cmd/awf/gate.go"
- "cmd/awf/git_context_test.go"
- "cmd/awf/global_help_test.go"
- "cmd/awf/init_test.go"
- "cmd/awf/list_add_test.go"
- "cmd/awf/main.go"
- "cmd/awf/new.go"
- "cmd/awf/presentation_boundary_test.go"
- "cmd/awf/snapshot_test.go"
- "cmd/awf/sync_composition_test.go"
- "cmd/awf/testcontext_test.go"
- "cmd/awf/testdata/help/global.txt"
- "cmd/awf/testdata/init-describe.json"
- "cmd/awf/testdata/presentation-boundary/positive-context-delivery.go"
- "cmd/awf/topic.go"
- "cmd/awf/topic_test.go"
- "cmd/awf/upgrade_test.go"
- "cmd/awf/version.go"
- "cmd/contextspilllog/main.go"
- "cmd/contextspilllog/main_test.go"
- "cmd/releasecheck/main_test.go"
- "cmd/repoaudit/main.go"
- "cmd/repoaudit/main_test.go"
- "coverage-baseline.json"
- "coverage-review.json"
- "docs/architecture.md"
- "docs/config-reference.md"
- "docs/debugging.md"
- "docs/decisions/0317-make-implementation-verification-parent-owned.md"
- "docs/decisions/0318-decompose-repeated-assurance-findings-by-semantic-owner.md"
- "docs/decisions/0319-adopt-the-pi-cockpit-effort-integration-contract.md"
- "docs/decisions/0320-delegate-relevance-discovery-to-codegraph.md"
- "docs/decisions/INDEX.md"
- "docs/decisions/README.md"
- "docs/decisions/adopt-the-pi-cockpit-effort-integration-contract.md"
- "docs/decisions/decompose-repeated-assurance-findings-by-semantic-owner.md"
- "docs/decisions/delegate-relevance-discovery-to-codegraph.md"
- "docs/decisions/make-implementation-verification-parent-owned.md"
- "docs/doc-standard.md"
- "docs/domains/config.md"
- "docs/domains/invariants.md"
- "docs/domains/rendering.md"
- "docs/domains/tooling.md"
- "docs/glossary.md"
- "docs/pi-runtime-reference.md"
- "docs/pitfalls.md"
- "docs/pitfalls/a-census-number-is-only-as-good-as-its-stated-query.md"
- "docs/pitfalls/a-faked-collaborator-makes-both-the-fixtures-and-the-assertion-vocabulary-unfalsifiable.md"
- "docs/pitfalls/a-future-code-fence-marker-must-account-for-linguist-aliases.md"
- "docs/pitfalls/a-future-non-catalog-render-singleton-still-has-hand-wired-fan-out.md"
- "docs/pitfalls/a-milestone-time-check-must-not-double-as-an-every-commit-test.md"
- "docs/pitfalls/a-new-output-language-needs-an-exercised-real-render-target.md"
- "docs/pitfalls/a-plan-editing-a-catalog-template-or-default-under-enumerates-the-render-fan-out.md"
- "docs/pitfalls/a-proof-marker-does-not-prove-every-clause-in-its-invariant-claim.md"
- "docs/pitfalls/a-prose-contract-test-proves-only-the-clauses-whose-literals-occur-for-one-reason.md"
- "docs/pitfalls/a-scripted-sweep-over-adr-prose-can-silently-unmake-the-structure-it-edits.md"
- "docs/pitfalls/a-staged-symlink-fixture-needs-a-real-blob-a-gitlink-does-not.md"
- "docs/pitfalls/a-token-or-convention-rename-must-sweep-every-rendered-doc-surface.md"
- "docs/pitfalls/ad-hoc-compound-mutations-still-need-target-read-back.md"
- "docs/pitfalls/adr-adr-title-already-carries-the-adr-nnnn-prefix.md"
- "docs/pitfalls/an-ad-hoc-empty-scan-still-needs-proof-that-the-probe-ran.md"
- "docs/pitfalls/an-ad-hoc-post-check-can-still-overrun-the-change-s-scope.md"
- "docs/pitfalls/an-attribute-filtered-pinned-set-test-exempts-every-other-attribute-value.md"
- "docs/pitfalls/an-ordered-phrase-assertion-cannot-reach-a-site-ahead-of-its-first-anchor.md"
- "docs/pitfalls/an-ordering-proof-written-against-the-log-proves-nothing.md"
- "docs/pitfalls/check-the-open-boundary-before-ignoring-an-assembly-error.md"
- "docs/pitfalls/enabled-linters-constrain-api-shape-sketch-signatures-against-them.md"
- "docs/pitfalls/gofmt-rewrites-double-backticks-in-doc-comments-into-curly-quotes.md"
- "docs/pitfalls/keep-recovery-ui-writes-non-fatal-after-session-disposal.md"
- "docs/pitfalls/keep-ssh-alive-through-long-pre-push-gates.md"
- "docs/pitfalls/link-adrs-by-their-on-disk-filename-never-by-constructing-one-from-the-title.md"
- "docs/pitfalls/make-custom-staged-slice-hooks-explicit-about-branch-and-cleanup.md"
- "docs/pitfalls/moving-a-check-earlier-in-the-pipeline-steals-a-later-stage-s-error-branch-coverage.md"
- "docs/pitfalls/obsoleting-rendered-prose-sweep-parts-and-whole-narratives-not-just-templates.md"
- "docs/pitfalls/pin-the-go-toolchain-when-preview-compilers-break-lint.md"
- "docs/pitfalls/port-a-stale-branch-before-merging-a-breaking-marker-grammar.md"
- "docs/pitfalls/raw-byte-adr-surgery-must-bound-every-scan-to-the-frontmatter-window.md"
- "docs/pitfalls/raw-byte-offsets-go-stale-the-moment-an-earlier-pass-edits-the-file.md"
- "docs/pitfalls/recheck-closure-assertions-after-changing-catalog-edges.md"
- "docs/pitfalls/reconcile-the-exact-mutable-artifact-not-a-similarly-named-predecessor.md"
- "docs/pitfalls/resolve-review-ranges-from-git-rather-than-transcribing-shas.md"
- "docs/pitfalls/retiring-a-concept-needs-paraphrase-sweeps-not-just-identifier-greps.md"
- "docs/pitfalls/reuse-the-repository-boundary-for-new-filesystem-walks.md"
- "docs/pitfalls/scope-a-claim-to-the-command-s-actual-input.md"
- "docs/pitfalls/sidecar-data-is-not-placeholder-substituted-drop-awf-escapes-when-converting-a-part.md"
- "docs/pitfalls/use-absolute-generations-for-historical-migration-shapes.md"
- "docs/pitfalls/when-retiring-a-config-key-handle-historical-writers.md"
- "docs/plans/2026-08-29-adopt-the-pi-cockpit-effort-integration-contract.md"
- "docs/plans/2026-08-29-implement-codegraph-relevance-delegation.md"
- "docs/plans/2026-08-29-parent-owned-verification-and-semantic-owner-assurance.md"
- "docs/plans/README.md"
- "docs/plans/template.md"
- "docs/roadmap.md"
- "docs/testing.md"
- "docs/topics/adr-system/adr-lifecycle.md"
- "docs/topics/adr-system/frontmatter.md"
- "docs/topics/adr-system/plan-artifacts.md"
- "docs/topics/code-design/dependency-composition.md"
- "docs/topics/code-design/execution-planning.md"
- "docs/topics/code-design/outcome-modeling.md"
- "docs/topics/code-design/package-composition.md"
- "docs/topics/code-design/presentation-ownership.md"
- "docs/topics/code-design/presentation-package.md"
- "docs/topics/code-design/single-home.md"
- "docs/topics/code-design/state-ownership.md"
- "docs/topics/code-design/test-design.md"
- "docs/topics/config/configspec-and-reference.md"
- "docs/topics/config/configuration.md"
- "docs/topics/config/index.md"
- "docs/topics/config/migrations-and-locks.md"
- "docs/topics/config/validation.md"
- "docs/topics/invariants/current-state-authority.md"
- "docs/topics/invariants/index.md"
- "docs/topics/invariants/topics-and-markers.md"
- "docs/topics/rendering/adapter-outputs.md"
- "docs/topics/rendering/catalog-and-targets.md"
- "docs/topics/rendering/companion-scripts.md"
- "docs/topics/rendering/doc-outputs.md"
- "docs/topics/rendering/guide-and-doc-templates.md"
- "docs/topics/rendering/inplace-and-placeholders.md"
- "docs/topics/rendering/pi-runtime.md"
- "docs/topics/rendering/pi-workflows.md"
- "docs/topics/rendering/project-output-plan.md"
- "docs/topics/rendering/render-engine.md"
- "docs/topics/rendering/singletons-and-payloads.md"
- "docs/topics/rendering/sync-and-drift.md"
- "docs/topics/rendering/templates.md"
- "docs/topics/rendering/workflow-skill-templates.md"
- "docs/topics/tooling/audit-and-snapshots.md"
- "docs/topics/tooling/audit-commands.md"
- "docs/topics/tooling/authority-queries.md"
- "docs/topics/tooling/changelog-and-release.md"
- "docs/topics/tooling/cli.md"
- "docs/topics/tooling/commit-policy.md"
- "docs/topics/tooling/context-and-topic.md"
- "docs/topics/tooling/effort-management.md"
- "docs/topics/tooling/evaluations.md"
- "docs/topics/tooling/file-publication.md"
- "docs/topics/tooling/filesystem-access.md"
- "docs/topics/tooling/git-access.md"
- "docs/topics/tooling/index.md"
- "docs/topics/tooling/init-and-enablement.md"
- "docs/topics/tooling/project-license.md"
- "docs/topics/tooling/quality-gates.md"
- "docs/topics/tooling/test-infrastructure.md"
- "docs/topics/tooling/upgrade-runtime.md"
- "docs/workflow.md"
- "internal/adr/adr_test.go"
- "internal/audit/audit.go"
- "internal/audit/history.go"
- "internal/audit/history_test.go"
- "internal/catalog/standard.go"
- "internal/changelog/changelog.go"
- "internal/checkop/publishing.go"
- "internal/clispec/authority_test.go"
- "internal/clispec/clispec.go"
- "internal/clispec/clispec_test.go"
- "internal/config/config.go"
- "internal/config/config_test.go"
- "internal/config/edit.go"
- "internal/configspec/spec.go"
- "internal/configspec/spec_test.go"
- "internal/contextdelivery/delivery.go"
- "internal/contextdelivery/delivery_test.go"
- "internal/contextinput/input.go"
- "internal/contextinput/input_test.go"
- "internal/contextinput/plan_context_test.go"
- "internal/contextop/context.go"
- "internal/contextop/context_benchmark_test.go"
- "internal/contextop/context_preparation_test.go"
- "internal/contextop/context_special_unix_test.go"
- "internal/contextop/context_test.go"
- "internal/contextq/adapter_outputs_test.go"
- "internal/contextq/boundary_test.go"
- "internal/contextq/context.go"
- "internal/contextq/context_adr.go"
- "internal/contextq/context_adr_test.go"
- "internal/contextq/context_artifacts.go"
- "internal/contextq/context_artifacts_test.go"
- "internal/contextq/context_benchmark_test.go"
- "internal/contextq/context_paths.go"
- "internal/contextq/context_paths_test.go"
- "internal/contextq/context_projection.go"
- "internal/contextq/context_projection_test.go"
- "internal/contextq/context_test.go"
- "internal/contextq/render.go"
- "internal/contextq/render_test.go"
- "internal/contextspill/log.go"
- "internal/contextspill/log_fault_test.go"
- "internal/contextspill/log_test.go"
- "internal/currentstate/domainownership_test.go"
- "internal/currentstate/legacy_absent_test.go"
- "internal/currentstate/uncovered_test.go"
- "internal/currentstatecoord/authority.go"
- "internal/currentstatecoord/authority_test.go"
- "internal/currentstatecoord/context.go"
- "internal/currentstatecoord/currentstate.go"
- "internal/currentstatecoord/currentstate_owner_test.go"
- "internal/currentstatecoord/currentstate_substrate_test.go"
- "internal/currentstatecoord/outputstate.go"
- "internal/evals/concrete_maintainability_review_test.go"
- "internal/evals/independent_workflow_escalation_test.go"
- "internal/generatedcheck/generatedcheck_test.go"
- "internal/glossarycheck/glossarycheck.go"
- "internal/glossarycheck/glossarycheck_test.go"
- "internal/initop/init.go"
- "internal/initspec/initspec.go"
- "internal/manifest/manifest.go"
- "internal/migrate/changes.go"
- "internal/migrate/migrate.go"
- "internal/migrate/migrate_test.go"
- "internal/migrate/relevance.go"
- "internal/outputplan/outputplan.go"
- "internal/outputplan/outputplan_test.go"
- "internal/pathglob/pathglob.go"
- "internal/pitfall/pitfall.go"
- "internal/pitfall/pitfall_test.go"
- "internal/plan/plan.go"
- "internal/project/VERSION"
- "internal/project/adr_link_check_test.go"
- "internal/project/agent_template_test.go"
- "internal/project/authoring_workflow_template_test.go"
- "internal/project/check.go"
- "internal/project/context_wrapper_test.go"
- "internal/project/contextstate.go"
- "internal/project/currentstate_test.go"
- "internal/project/documentation_template_test.go"
- "internal/project/export_test.go"
- "internal/project/gatedcommands_test.go"
- "internal/project/hooks_test.go"
- "internal/project/legacy_check_helpers_test.go"
- "internal/project/loader_test.go"
- "internal/project/maintainable_workflow_template_test.go"
- "internal/project/notes_test.go"
- "internal/project/operations.go"
- "internal/project/outputstate.go"
- "internal/project/owner_compat_test.go"
- "internal/project/phase_transaction_ownership_test.go"
- "internal/project/plan_detail_modes_test.go"
- "internal/project/plan_execution_workflow_template_test.go"
- "internal/project/project.go"
- "internal/project/publication_safe_template_test.go"
- "internal/project/publisher_test_helpers_test.go"
- "internal/project/review_workflow_template_test.go"
- "internal/project/scaffold.go"
- "internal/project/staged_drift.go"
- "internal/project/staged_drift_compat_test.go"
- "internal/project/stateownership_test.go"
- "internal/project/surface_coverage_test.go"
- "internal/project/tag_vocabulary_check_test.go"
- "internal/project/tag_vocabulary_dogfood_test.go"
- "internal/project/version_test.go"
- "internal/publisher/adapter_outputs_test.go"
- "internal/publisher/configreference.go"
- "internal/publisher/configreference_test.go"
- "internal/publisher/glossary_test.go"
- "internal/publisher/golden_test.go"
- "internal/publisher/inputs.go"
- "internal/publisher/inputs_test.go"
- "internal/publisher/output_declarations_test.go"
- "internal/publisher/pitfalls.go"
- "internal/publisher/pitfalls_dogfood_test.go"
- "internal/publisher/pitfalls_test.go"
- "internal/publisher/profile_semantics_test.go"
- "internal/publisher/render.go"
- "internal/publisher/render_test.go"
- "internal/publisher/source_marker_test.go"
- "internal/publisher/sync.go"
- "internal/publisher/target_test.go"
- "internal/publisher/template_source_marker_test.go"
- "internal/publisher/test_helpers_test.go"
- "internal/publisher/topics_test.go"
- "internal/referencecheck/referencecheck_test.go"
- "internal/render/render.go"
- "internal/render/section.go"
- "internal/resident/singlehome_test.go"
- "internal/snapshot/selection.go"
- "internal/snapshot/selection_test.go"
- "internal/snapshot/working.go"
- "internal/snapshot/working_test.go"
- "internal/testsupport/check_result_ownership_test.go"
- "internal/testsupport/deps_test.go"
- "internal/testsupport/gitfixture/gitfixture.go"
- "internal/testsupport/gitfixture/gitfixture_test.go"
- "internal/testsupport/gitfixture/native.go"
- "internal/testsupport/publishing_ownership_test.go"
- "internal/testsupport/thin_command_composition_test.go"
- "internal/topic/coverage.go"
- "internal/topic/coverage_test.go"
- "internal/topic/domainownership_test.go"
- "internal/topic/markers.go"
- "internal/topic/markers_test.go"
- "internal/topic/query_test.go"
- "internal/topic/render.go"
- "internal/topic/render_test.go"
- "internal/topic/topic_test.go"
- "internal/vocabularycheck/vocabularycheck.go"
- "internal/vocabularycheck/vocabularycheck_test.go"
- "templates/adr-readme/README.md.tmpl"
- "templates/agents/code-reviewer.md.tmpl"
- "templates/agents/grounding-checker.md.tmpl"
- "templates/agents/implementer.md.tmpl"
- "templates/agents/plan-reviewer.md.tmpl"
- "templates/docs/config-reference.md.tmpl"
- "templates/docs/debugging.md.tmpl"
- "templates/docs/doc-standard.md.tmpl"
- "templates/docs/pi-runtime-reference.md.tmpl"
- "templates/docs/pitfalls.md.tmpl"
- "templates/docs/testing.md.tmpl"
- "templates/docs/workflow.md.tmpl"
- "templates/partials/context-orientation.md"
- "templates/partials/context-spill.md"
- "templates/partials/governance-footprints.md"
- "templates/partials/implementation-autonomy.md"
- "templates/partials/orientation-ladder.md"
- "templates/partials/phase-loop-continuation.md"
- "templates/partials/plan-flexibility.md"
- "templates/partials/semantic-owner-assurance.md"
- "templates/partials/template-source-observation.md"
- "templates/pi/awf-effort/index.ts.tmpl"
- "templates/pi/awf-subagents/index.ts.tmpl"
- "templates/pitfalls/entry.md.tmpl"
- "templates/plans-readme/README.md.tmpl"
- "templates/plans-template/template.md.tmpl"
- "templates/skills/adr-lifecycle/SKILL.md.tmpl"
- "templates/skills/brainstorming/SKILL.md.tmpl"
- "templates/skills/bugfix/SKILL.md.tmpl"
- "templates/skills/debugging/SKILL.md.tmpl"
- "templates/skills/executing-plans/SKILL.md.tmpl"
- "templates/skills/grounding/SKILL.md.tmpl"
- "templates/skills/orienting/SKILL.md.tmpl"
- "templates/skills/refactor-coupling-audit/SKILL.md.tmpl"
- "templates/skills/reviewing-adr/SKILL.md.tmpl"
- "templates/skills/reviewing-impl/SKILL.md.tmpl"
- "templates/skills/reviewing-plan/SKILL.md.tmpl"
- "templates/skills/subagent-driven-development/SKILL.md.tmpl"
- "templates/skills/tdd/SKILL.md.tmpl"
- "templates/skills/using-effort/SKILL.md.tmpl"
- "templates/skills/writing-plans/SKILL.md.tmpl"
- "tools/pi-extension-test/tests/index.test.ts"
- "tools/pi-extension-test/tests/profile-adapter.test.ts"
- "tools/pi-extension-test/tests/using-effort.test.ts"
- "x"
Material deviations:
- Restored broad non-context publication, current-state, project-state, help, and ownership oracles after the Phase 3 owner over-deleted whole test files, and kept staged output preparation in the current-state coordinator because it reuses coordinator-private snapshot helpers.
- Added three Phase 3 authority updates, two Phase 4 authority updates, and the Phase 5 output-plan update through reviewed ADR amendments after implementation or review exposed stale active claims omitted from the original operation set.
- Removed the residual top-level `awf topic` route and applied the rendered-applicability operation early in the Phase 3 verify settlement so generated drilldowns stayed truthful before the remaining proof-marker work.
- Reconciled the coverage policy from a fresh stable-toolchain profile after Phase 4 review exposed deleted context and vocabulary identities, moved lines, and new misses; preserved source-grounded reasons and exact lineage instead of the delegated generic rewrite.
- Removed unreachable output-plan accessors, snapshot live-context inventory helpers, and `TopicsForPath` after the Phase 5 full gate exposed context-era dead code, then regenerated the coverage universe for the smaller reachable surface.
- Integrated divergently with newer main, resolved the changelog, lock, pitfall, and generated-output conflicts, removed retired tags from the concurrent SSH pitfall, and numbered the pending decision as ADR-0320 before renewed integration review.
