---
format: plan-v2
date: 2026-08-29
adrs:
  - delegate-relevance-discovery-to-codegraph
status: Proposed
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
Paths: ["internal/migrate", "internal/upgrade", "internal/config", "internal/configspec", "internal/manifest", "internal/pitfall", "internal/publisher", "internal/project", "internal/projectstate", "internal/testsupport", "cmd/awf", "glob:cmd/awf/testdata/**", "glob:internal/**/testdata/**", "glob:examples/**/.awf/**", "docs/config-reference.md"]
Representative: ["top-level config `tags`", "top-level `contextIgnore`", "pitfall `tags:` frontmatter"]
Edge: ["flow and block YAML forms", "absent and empty fields", "pitfalls without domains", "interrupted upgrade recovery", "staged lock and manifest regeneration", "frozen legacy ADR tags"]
Post-check: "Observe migration fixtures from the current schema fail against the narrowed live parser before the migration is registered, then pass after implementation. Require upgrade to remove all three live surfaces before strict decoding, preserve comments and unrelated formatting where the established migration contract does, leave frozen legacy ADR parsing intact, regenerate outputs and lock once, recover through the existing journal after injected interruption, and reject retired fields in a post-upgrade live config or pitfall source."

Do not scan or rewrite arbitrary adopter production source. The migration owns config-tree and authored pitfall inputs only.

### Task 4.2: Remove tag policy and rendering while preserving pitfall domains
Kind: batch
Applying: ["delegate-relevance-discovery-to-codegraph:relevance-metadata-retirement", "delegate-relevance-discovery-to-codegraph:governance-core-retained"]
Paths: ["internal/vocabularycheck", "internal/pitfall", "internal/pitfallcheck", "internal/publisher", "internal/repositorycheck", "internal/configcheck", "internal/config", "internal/configspec", "internal/project", "internal/projectstate", "internal/testsupport", "templates/docs/pitfalls.md.tmpl", "templates/pitfalls", ".awf/docs/pitfalls", ".awf/config.yaml", "docs/pitfalls.md", "glob:docs/pitfalls/*.md"]
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

### Task 4.4: Apply the schema and marker authority batch
Kind: batch
Applying: ["delegate-relevance-discovery-to-codegraph:relevance-metadata-retirement", "delegate-relevance-discovery-to-codegraph:clean-cutover"]
Paths: [".awf/topics/parts/config/configuration/current-state.md", ".awf/topics/parts/config/validation/current-state.md", ".awf/topics/parts/rendering/doc-outputs/current-state.md", ".awf/topics/parts/invariants/topics-and-markers/current-state.md", ".awf/topics/parts/invariants/current-state-authority/current-state.md", ".awf/topics/parts/tooling/upgrade-runtime/current-state.md", "docs/decisions/delegate-relevance-discovery-to-codegraph.md", "docs/decisions/INDEX.md", "glob:docs/topics/{config,rendering,invariants,tooling}/*.md", ".awf/awf.lock"]
Post-check: "Apply exactly the removals `config/configuration:{tag-coverage-note,tag-frequency-note,tag-vocabulary-governed}`, `config/validation:tag-not-domain-name`, and `invariants/topics-and-markers:{relevance-markers-only-narrow,touches-marker-advisory}`; the adds `config/configuration:no-active-tag-system`, `invariants/current-state-authority:domain-owned-coverage-no-ignore`, and `invariants/topics-and-markers:proof-only-marker-grammar`; and the updates `rendering/doc-outputs:pitfall-corpus-validated`, `invariants/topics-and-markers:{claim-id-qualified,invariant-marker-close-token,invariants-three-state,rendered-applicability-selectors-only}`, and `tooling/upgrade-runtime:upgraded-runtime-has-one-authority-engine`. Render and require staged checking to accept exactly the batch and ADR progress to report no Remaining operation while status stays Implementing."

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
Paths: ["README.md", "changelog/CHANGELOG.md", ".awf/awf.lock", "docs/architecture.md", "docs/config-reference.md", "docs/debugging.md", "docs/doc-standard.md", "docs/glossary.md", "docs/pitfalls.md", "docs/testing.md", "docs/working-with-awf.md", "docs/workflow.md", "docs/decisions/INDEX.md", "docs/decisions/delegate-relevance-discovery-to-codegraph.md", "docs/plans/2026-08-29-implement-codegraph-relevance-delegation.md", "glob:docs/domains/*.md", "glob:docs/topics/**/*.md", "glob:docs/pitfalls/*.md", "glob:.pi/**", "glob:.claude/**", "glob:examples/**"]
Post-check: "Run `./x render`, inspect every changed generated boundary for the approved reading, and require `./x check` to finish with no context-era or tag-health finding. Run checked active-policy sweeps for the deleted command, packages, spill protocol, tags, contextIgnore, state/touches markers, obsolete topic identity, and old topic command grammar; classify and document only frozen ADR history or inert adopter-source compatibility residue. Exercise representative read topic, read ADR, path resolution, explicit none, and whole-repository uncovered outputs. Then run affected-package feedback, `go test ./...`, `./x gate full`, and staged checks with the stable Go toolchain; all terminal gates pass and ADR progress has no Remaining operation."

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
