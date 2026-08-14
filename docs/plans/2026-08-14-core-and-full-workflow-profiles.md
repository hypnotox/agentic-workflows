---
format: plan-v2
date: 2026-08-14
adrs: [introduce-core-and-full-workflow-profiles]
status: Proposed
---
# Plan: Core and Full workflow profiles

## Goal

Ship two closed awf workflow profiles: Core as the default for new adopters and Full as the governance layer and migration destination for existing adopters. Core retains the brainstorm -> implement/test -> review discipline, operational skills and agents, efforts, worktrees, rendering, and quality tooling without ADR, plan, current-state, context, or workflow-audit machinery. Non-goals are arbitrary artifact selection, a second binary, deletion of authored historical ADR or plan files, and weakening Full's existing governance semantics.

## Architecture summary

Execution accepts the reviewed ADR, then Phase 2 introduces only a behavior-preserving immutable view of the complete catalog and threads it through Project without profile membership or filtered behavior. Phase 3 atomically adds Core/Full membership, profile-aware outputs and checks, Core-neutral templates with Full-only governance additions, the required configuration fact, default-Core initialization, existing-repository Full migration, explicit command capabilities, every current-state mutation, and end-to-end transition proofs.

The complete catalog remains the single authority. Catalog membership projects a closed Core subset and a Full superset; Project derives the selected immutable view once and threads it to render, layout, generated producers, hashes, pruning, checks, and command decisions. Core artifacts may depend only on Core artifacts and ordinary repository documentation, source, tests, and history. Shared workflow prose is Core-neutral at its semantic home, with Full additions selected by the same view. Existing lock membership remains the only pruning authority, and authored historical ADR and plan leaves never enter Core's managed output set.

## Phase 1: Authorize the profile decision

**Execution mode: inline.**

Completes: ["decision-authorized"]

### Task 1.1: Accept the reviewed successor ADR
Latitude: exact
Paths: ["docs/decisions/introduce-core-and-full-workflow-profiles.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Post-check: `./awf check` and `./x check` reach clean terminal states, and `./awf context --show pending docs/decisions/introduce-core-and-full-workflow-profiles.md` reports Accepted with every declared operation pending.

Use `awf-adr-lifecycle` to change the linked ADR from Proposed to Accepted without changing its reviewed Decision or State changes. Establish the required content digest mechanically, append the Accepted history event, run `./x render`, and preserve all operations as Remaining.

### Phase close

Stage only the ADR transition, regenerated decision index, and lock, then close the authorization transaction.

```commit
docs(adr): accept core and full workflow profiles
```

## Phase 2: Centralize catalog and project view ownership

**Execution mode: subagent-driven.**

Completes: ["single-view-foundation"]

### Task 2.1: Introduce a behavior-preserving immutable catalog view
Applying: ["introduce-core-and-full-workflow-profiles:single-profile-projection"]
Paths: ["internal/catalog/catalog.go", "internal/catalog/standard.go", "internal/catalog/catalog_test.go"]

Introduce an immutable catalog view over the complete Standard catalog without adding Core/Full membership, selectable profiles, filtered outputs, or new adopter behavior. Preserve every catalog entry and ordering. This is the narrow preparatory seam that lets Project consume one injected view in Task 2.2 without speculative filtering or a second catalog authority.

### Task 2.2: Make Project own one complete-catalog view and remove global catalog bypasses
Applying: ["introduce-core-and-full-workflow-profiles:single-profile-projection"]
Paths: ["internal/project/project.go", "internal/project/contextstate.go", "internal/project/kind.go", "internal/project/layout.go", "internal/project/singleton.go", "internal/project/scaffold.go", "internal/project/configreference.go", "internal/project/output_plan.go", "internal/project/loader_test.go", "internal/project/unified_doc_model_test.go", "internal/project/scaffold_test.go", "internal/project/configreference_test.go"]

Thread an immutable catalog view through Loader, Project, working and staged composition, layout, singleton classification, scaffold variable discovery, output declarations, and config-reference consumers. During this preparatory phase production selection remains Full, so rendered bytes and behavior remain unchanged. Replace `catalog.Standard` reads outside composition roots and explicitly frozen migration authority; add a source-level regression that rejects new bypasses. Do not add a second cache or let individual consumers reconstruct membership.

### Phase close

Prove the Full view is byte- and behavior-equivalent to the pre-profile catalog, all project packages consume the one view, and the gate remains green.

```commit
refactor(code-design): centralize catalog view ownership
```

## Phase 3: Activate Core and Full atomically

**Execution mode: subagent-driven.**

Completes: ["profile-engine", "core-semantic-closure", "profile-activation", "active-state-current"]

### Task 3.1: Project selected membership through every output producer
Applying: ["introduce-core-and-full-workflow-profiles:two-closed-profiles", "introduce-core-and-full-workflow-profiles:closed-profile-transition", "introduce-core-and-full-workflow-profiles:single-profile-projection"]
Paths: ["internal/project/project.go", "internal/project/render.go", "internal/project/output_plan.go", "internal/project/layout.go", "internal/project/target.go", "internal/project/topics.go", "internal/project/configreference.go", "internal/project/pitfalls.go", "internal/project/confighash.go", "internal/project/validate.go", "internal/project/sweep.go", "internal/project/output_plan_test.go", "internal/project/render_test.go", "internal/project/target_test.go", "internal/project/drift_test.go", "internal/project/sweep_test.go", "internal/project/confighash_test.go"]

Make selected membership govern skills, agents, shared and Full-only docs, structural singletons, target outputs, Pi outputs, local docs, pitfalls, ADR index, domain/topic producers, and config reference. Core omits ADR/plan/current-state structural outputs but never claims authored ADR or plan leaves. Sidecars and parts for inactive Full artifacts are orphaned under Core rather than dormant. Existing output-plan membership drives lock changes and pruning; include the selected profile in relevant hashes without introducing parallel prune state.

Add transition fixtures proving Full -> Core removes only lock-managed Full outputs and empty ancestors, preserves authored `docs/decisions/*.md` and `docs/plans/*.md` leaves, and Core -> Full restores structural outputs. Keep Full output parity.

### Task 3.2: Gate corpora, checks, metadata, and references by the selected view
Applying: ["introduce-core-and-full-workflow-profiles:core-operational-workflow", "introduce-core-and-full-workflow-profiles:profile-aware-capabilities"]
Paths: ["internal/project/project.go", "internal/project/check.go", "internal/project/currentstate.go", "internal/project/plan_context.go", "internal/project/plan_read.go", "internal/project/adrnumber.go", "internal/project/staged_drift.go", "internal/project/skillrefs_test.go", "internal/project/check_test.go", "internal/project/currentstate_test.go", "internal/project/plan_context_test.go", "internal/project/staged_drift_test.go"]

Derive operation state once for the selected profile. Core must not load or validate ADR, plan, current-state, topic, domain, or context authority and must skip their governance checks; it retains drift, tracking, commit, prose, memory, rendering, pitfall, effort, and code-quality behavior that does not need Full authority. Dead skill/agent/doc references resolve against the selected view. Full continues to load and validate all current corpora. Profile-specific pitfall and glossary metadata must not silently retain Full-only domain or ADR relationships under Core.

### Task 3.3: Declare command capabilities at the command boundary
Applying: ["introduce-core-and-full-workflow-profiles:profile-aware-capabilities"]
Paths: ["internal/clispec/clispec.go", "internal/clispec/clispec_test.go", "cmd/awf/main.go", "cmd/awf/dispatch.go", "cmd/awf/gate_test.go", "cmd/awf/run_test.go", "internal/project/gatedcommands.go"]

Add explicit Full-only command metadata for workflow audit and ADR, plan, current-state, topic, domain, and context operations. Refuse those commands under an internally selected Core view before handler mutation or incidental generated-file access, with stable typed presentation. Keep operational checks and effort/worktree/render commands available. Prove refusal ordering against schema/state guards and Full success. This task is not staged independently: Tasks 3.8-3.10 bind selection, update current authority, and close the same atomic phase.

### Task 3.4: Neutralize shared workflow skills and reviewer semantics
Kind: batch
Applying: ["introduce-core-and-full-workflow-profiles:core-operational-workflow", "introduce-core-and-full-workflow-profiles:profile-dependency-direction"]
Paths: ["pathspec:templates/skills", "pathspec:templates/agents", "pathspec:templates/partials", "internal/catalog/standard.go", "internal/project/skill_sections_test.go", "internal/project/skillrefs_test.go", "internal/project/catalog_sweep_test.go"]
Representative: Core brainstorming ends at approved direct execution rather than ADR/plan routing, Core orientation and grounding read AGENTS.md, project docs, source, tests, and history rather than current-state topics, and Core implementation review judges the approved boundary without plan or ADR lenses.
Edge: effort-workflow, checkpoints, managed worktrees, handoff, integration, retrospective, implementation autonomy, and code review remain Core; their Full rendering adds deferred ADR/plan closure and authority-specific remediation. `refactor-coupling-audit` remains Core after generic follow-up routing. ADR/plan authoring, review, lifecycle, plan execution, subagent-driven plan development, and roadmap graduation are Full-only.
Post-check: profile-expanded template tests pass for every Core and Full skill/agent, and a selected-Core reference scan reports no ADR, plan, current-state, `awf context`, Full-only skill, or Full-only agent dependency.

Refactor shared partials into Core-neutral semantic homes with profile-selected Full additions rather than copying entire workflows. Preserve mandatory outline approval, TDD, reasoned correction, review classification, continuity, one-writer effort memory, and implementation verification. Full must retain the current governance semantics and ordering.

### Task 3.5: Project profile-specific AGENTS.md and standard documentation
Kind: batch
Applying: ["introduce-core-and-full-workflow-profiles:two-closed-profiles", "introduce-core-and-full-workflow-profiles:profile-dependency-direction"]
Paths: ["templates/agents-doc/AGENTS.md.tmpl", "pathspec:templates/docs", "templates/adr-readme/README.md.tmpl", "templates/adr-template/template.md.tmpl", "templates/plans-readme/README.md.tmpl", "templates/plans-template/template.md.tmpl", "templates/domains/domain.md.tmpl", "templates/topics/index.md.tmpl", "templates/topics/topic.md.tmpl", "pathspec:.awf/parts/agents-doc", "pathspec:.awf/parts/workflow", "pathspec:.awf/parts/working-with-awf", ".awf/parts/workflow/chain.md", "pathspec:.awf/agents", "pathspec:.awf/skills", "internal/catalog/standard.go", "internal/project/agents_doc_budget_test.go", "internal/project/docs_sections_test.go", "internal/project/local_doc_guidance_test.go", "internal/project/unified_doc_model_test.go", "AGENTS.md", "CLAUDE.md", "pathspec:.claude", "pathspec:.pi", "pathspec:docs"]
Representative: Core's guide documents brainstorm -> implement/test -> review, efforts, tools, and ordinary repository authority; Full adds ADR/plan/current-state/context routing and document-map entries. Shared workflow, working-with-awf, documentation, architecture, testing, debugging, development, roadmap, glossary, pitfalls, releasing, code-design, and config-reference docs render profile-appropriate content.
Edge: ADR README/template/index, plan README/template, domain docs, and topic docs are Full-only generated outputs; authored historical ADR and plan leaves are never template outputs. Shared docs must not gain duplicate Core/Full paths.
Post-check: both profile document maps and link scans resolve only emitted outputs, both AGENTS.md renders stay within their budgets, and Core generated prose contains no Full-only command or authority routing.

Edit `.awf/` authoring sources rather than generated outputs wherever this repository overrides standard sections. Run `./x render` and inspect representative Core and Full render fixtures for meaning, contradictory fragments, and literal placeholder intent.

### Task 3.6: Keep Pi operational tools while gating governance review kinds
Kind: batch
Applying: ["introduce-core-and-full-workflow-profiles:core-operational-workflow", "introduce-core-and-full-workflow-profiles:profile-aware-capabilities"]
Paths: ["pathspec:templates/pi", "internal/project/target.go", "internal/project/subagent_model_selection_test.go", "internal/project/repository_wiring_test.go", "pathspec:tools/pi-extension-test"]
Representative: Core keeps grounding, exploration, implementation, code review, effort memory, handoff, context-usage observation, and model routing; ADR and plan review kinds are absent or explicitly unavailable. Full exposes the current complete routing surface.
Edge: runtime API floors, target paths, tool names, session replacement, and effort memory semantics remain unchanged.
Post-check: Pi extension tests pass for both rendered profiles, Core cannot dispatch ADR or plan review, Full can, and no Full reviewer path appears in Core output.

### Task 3.7: Enforce semantic closure and publication safety for both profiles
Kind: batch
Applying: ["introduce-core-and-full-workflow-profiles:profile-dependency-direction"]
Paths: ["internal/project/catalog_sweep_test.go", "internal/project/golden_test.go", "internal/project/repository_awf_invocation_test.go", "internal/project/template_source_marker_test.go", "internal/project/templates_vars_test.go", "internal/evals/chain_test.go", "internal/evals/independent_workflow_escalation_test.go"]
Representative: render the Core brainstorming and code-reviewer artifacts for both targets with empty project data, prove every emitted reference resolves within the Core view, and separately render Full to prove its governance references resolve within Full.
Edge: conceptual references without the awf skill prefix, included shared partials, target-owned Pi outputs, intentional literal placeholders, and profile-specific document links are part of the scanned population rather than exemptions.
Post-check: profile-parameterized catalog, golden, evaluation, link, dead-reference, and empty-data suites pass; each probe emits a success sentinel before asserting an empty residual set.

Pin Full as exactly Core plus the Full layer. Expand templates/includes before scanning dependencies. Render every target and artifact in both profiles with empty project data and reject `<no value>`, unresolved tokens, dead links, dead skill or agent references, contradictory Core/Full fragments, and Full authority concepts in Core. Record focused semantic inspection evidence for representative AGENTS.md, brainstorming, effort, executing-direct, reviewing-impl, code-reviewer, workflow, and working-with-awf outputs.

### Task 3.8: Add the required profile fact, default-Core init, and Full migration
Applying: ["introduce-core-and-full-workflow-profiles:profile-default-and-migration", "introduce-core-and-full-workflow-profiles:single-profile-projection"]
Paths: ["internal/config/config.go", "internal/config/edit.go", "internal/config/config_test.go", "internal/config/edit_test.go", "internal/configspec/spec.go", "internal/configspec/spec_test.go", "internal/initspec/initspec.go", "internal/initspec/initspec_test.go", "internal/migrate/migrate.go", "internal/migrate/migrate_test.go", "internal/migrate/profile.go", "internal/migrate/profile_test.go", "internal/manifest/manifest.go", "cmd/awf/init.go", "cmd/awf/init_test.go", "cmd/awf/initrender_test.go", "cmd/awf/upgrade_test.go", "internal/clispec/clispec.go", ".awf/config.yaml"]

Add required visible `profile: core|full` parsing, validation, skeleton serialization, configspec/reference projection, and project opening. Fresh plain init writes Core; explicit Full init writes Full. Add the next frozen schema migration using the required-field migration pattern: every existing repository missing the field receives `profile: full` before strict current parsing and terminal sync, preserving unrelated bytes and existing outputs; rerun is idempotent. This repository writes `profile: full`. Invalid or absent current-schema values fail closed.

### Task 3.9: Activate selected commands, checks, rendering, and transitions end to end
Applying: ["introduce-core-and-full-workflow-profiles:closed-profile-transition", "introduce-core-and-full-workflow-profiles:full-reactivation", "introduce-core-and-full-workflow-profiles:profile-aware-capabilities"]
Paths: ["cmd/awf/main.go", "cmd/awf/dispatch.go", "cmd/awf/run_test.go", "internal/project/project.go", "internal/project/adopter_fixture_test.go", "internal/project/project_test.go", "internal/project/render_tree_test.go", "internal/project/staged_drift_test.go", "internal/project/check_test.go", "internal/evals/fixture_test.go"]

Bind all preparatory seams to the config-selected view. Add complete temporary adopters proving default Core and explicit Full initialization for Claude and Pi, clean render/check/staged-check cycles, explicit Core refusal for every Full command family, Full command success, and no Full corpus loading under Core. Exercise Full -> Core with required deletion of Full-only `.awf` sources, managed-output pruning, and preservation of altered authored ADR/plan leaves; exercise Core -> Full regeneration and renewed validation failure for deliberately malformed retained history before adopter correction. Verify current Full fixtures retain their behavior after migration.

### Task 3.10: Apply every declared current-state operation with the activation
Kind: batch
Latitude: exact
Applying: ["introduce-core-and-full-workflow-profiles:two-closed-profiles", "introduce-core-and-full-workflow-profiles:core-operational-workflow", "introduce-core-and-full-workflow-profiles:profile-dependency-direction", "introduce-core-and-full-workflow-profiles:profile-default-and-migration", "introduce-core-and-full-workflow-profiles:closed-profile-transition", "introduce-core-and-full-workflow-profiles:full-reactivation", "introduce-core-and-full-workflow-profiles:profile-aware-capabilities", "introduce-core-and-full-workflow-profiles:single-profile-projection"]
Paths: ["docs/decisions/introduce-core-and-full-workflow-profiles.md", "pathspec:.awf", "README.md", "changelog/CHANGELOG.md", "AGENTS.md", "CLAUDE.md", "awf", "pathspec:.githooks", "pathspec:.claude", "pathspec:.pi", "pathspec:docs"]
Representative: replace the retired unconditional full-catalog and single-workflow claims with test-backed profile projection and closed-profile claims while preserving each unaffected neighboring claim and its provenance.
Edge: remove and add pairs land together; every updated claim retains Origin and gains exactly one Revised-by entry; authored `.awf/` sources, never generated topic/domain/docs outputs, own the prose.
Post-check: `./x render && ./x check` are clean; `./awf context --show pending docs/decisions/introduce-core-and-full-workflow-profiles.md` reports Implementing with every declared operation Applied and none Remaining; every added invariant reports `Backing: test`; generated source, docs, guide, target outputs, decision index, config reference, and lock match their authored causes; and the snapshot-scoped historical-leaf command in the task body prints `historical leaves unchanged`.

Update the active documentation and changelog for the public profile contract. Mutate the claims in the linked ADR's exact State changes set, grouped atomically as follows:

- Config: update `config-expresses-repo-facts-only`, `no-artifact-selection-surface`, `configspec-key-parity`, and `schema-version-lock`; add `profile-full-migration`.
- Catalog and outputs: update `target-dialect-render`, `unified-doc-model`, `multi-target-render`, `output-plan-complete`, `scaffold-seeds-all-vars`, `adr-system-singletons-rendered`, `layout-derivation`, `target-prune-ancestors`, `document-map-lists-mandatory-docs`, `guide-entry-point-routing`, `working-memory-single-home`, and `maintainable-code-design-guide`; remove `full-catalog-render` and `layout-docs-full-catalog`; add `profile-dependency-closure`, `profile-projected-render`, and `layout-docs-profile-projection`.
- Drift and checking: update `check-active-md-stale`, `closed-config-tree`, `drift-source-set`, `managed-output-attribution`, `sync-always-writes-active-md`, and `coverage-evaluation-unconditional`; add `profile-config-hash`.
- Workflow semantics: update `independent-workflow-escalation`, `implementer-context-grounding`, `mandatory-approval-boundaries`, `authority-guided-implementation-autonomy`, `authority-guided-review-remediation`, `unified-effort-workflow-coverage`, `effort-workflow`, `orienting-single-home`, and `maintainable-code-stage-coverage`; remove `single-workflow-no-depth-controls`; add `closed-workflow-profiles`.
- Tooling: update `cli-creation-and-inventory`, `invariants-in-check`, `check-universe-groups`, `upgrade-always-syncs`, `init-noninteractive-default`, and `init-prompts-enabled-vars`; add `audit-full-profile-only`, `context-full-profile-only`, and `init-profile-default-core`.

For each update preserve Origin and append `Revised-by: ADR-introduce-core-and-full-workflow-profiles`; each addition carries the focused `Backing: test` established in earlier tasks. Remove each retired claim and add its replacement in the same transaction. Change the ADR to Implementing and append one Applied event containing the complete declared operation membership; do not append Implemented. Render only from `.awf/` sources and stage every generated result.

After staging the complete Phase 3 transaction against the Phase 2 closing `HEAD`, verify historical leaves deterministically:

```bash
unexpected="$(git diff --cached --name-only HEAD -- 'docs/decisions/*.md' 'docs/plans/*.md' | grep -vE '^(docs/decisions/introduce-core-and-full-workflow-profiles\.md|docs/plans/2026-08-14-core-and-full-workflow-profiles\.md)$' || true)"
test -z "$unexpected"
printf '%s\n' 'historical leaves unchanged'
```

The empty changed set after excluding the linked lifecycle records proves that the broad generated-output confinement did not alter adopter-owned history.

### Phase close

Run every Phase 3 focused check plus `go test ./...`, inspect representative Core and Full generated prose, then stage the complete catalog, config, migration, rendering, workflow, command, current-state, documentation, generated-output, decision-history, and lock transaction. The broad generated pathspecs are exhaustive confinement for renderer output; historical ADR and plan leaves other than the linked ADR and this plan must remain byte-identical. The ADR remains Implementing with every operation Applied and the plan remains Proposed pending implementation assurance.

```commit
feat(awf): introduce core and full profiles (applies ADR profile batch)
```

## Definition of done

- `dod: decision-authorized` The independently reviewed successor ADR is Accepted with its reviewed content stamp and every operation pending before behavior changes begin.
- `dod: single-view-foundation` Project consumes one immutable complete-catalog view, production global-catalog bypasses are eliminated, and all externally observable behavior and rendered bytes remain unchanged before profile behavior lands.
- `dod: profile-engine` One selected view governs every output family, generated producer, layout, hash, prune decision, corpus, check, reference, and command capability; internal Core and Full fixtures are independently deterministic.
- `dod: core-semantic-closure` Core's skills, agents, shared partials, AGENTS.md, docs, and Pi tools form a complete brainstorm -> implement/test -> review and effort workflow with no Full-only authority or reference, while Full retains existing governance semantics.
- `dod: profile-activation` Fresh init defaults to Core, explicit Full init works, existing repositories migrate visibly and idempotently to Full, profile changes prune or restore only managed workflow artifacts, authored history is preserved, and unsupported Core commands refuse explicitly.
- `dod: active-state-current` Every linked ADR operation is Applied with test-backed current-state claims and generated documentation matching the shipped profile behavior; the ADR remains Implementing and this plan remains Proposed until assurance settles.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated owners report rather than edit; the parent supplies the report to phase review and reconciles it with findings in one focused post-review settlement commit before checkpointing or later execution.

- Phase 2 is a behavior-preserving preparatory refactor: it introduces one complete-catalog view but no Core/Full type, membership, filtered output, or selectable behavior. Phase 3 is the single activating application transaction because exposing any profile-aware output, check, prose, or migration behavior before the complete closure would contradict active current-state claims and create an incoherent supported profile.
- Full is the self-hosted profile for this repository after activation, so its `.awf/domains`, `.awf/topics`, and Full reviewer overrides remain active sources here. Core closed-tree behavior is proven in temporary adopters, not by deleting this repository's Full authority.
- Historical ADR and plan leaves are preservation fixtures only under Core. Returning to Full intentionally subjects them to Full validation again.
- Plan review reasoned finding: the original Phases 2-4 would have introduced profile behavior before the active one-workflow and full-catalog claims changed. Disposition: restrict Phase 2 to a behavior-preserving complete-catalog view and collapse every profile behavior plus all claim operations into Phase 3.
- Plan review reasoned finding: the new migration was under-confined to the registry and an unrelated required-field precedent. Disposition: give the profile migration dedicated `internal/migrate/profile.go` and `profile_test.go` ownership while keeping the existing integration-branch migration as read-only precedent.
- Plan verify reasoned finding: broad generated-doc confinement did not prove unrelated historical ADR and plan leaves stayed unchanged. Disposition: compare the staged Phase 3 snapshot to the Phase 2 closing HEAD, exclude only the linked ADR and this plan, require an empty residual changed set, and print an explicit success sentinel.
- Phase 2 implementation deviation: the view propagation also touched `internal/project/check.go`, `docs_sections_test.go`, `local_doc_agent_test.go`, `render.go`, `staged_drift.go`, and `validate.go` because those paths consumed the replaced catalog or singleton seam. The added paths preserve the phase's one-view dependency direction and passed the focused and full gates.
- Phase 2 review settlement: the first implementation exposed a mutable Standard alias, retained parallel `Project.Cat` and `Project.view` state, lost singleton ordering, and under-scanned global catalog bypasses. The settlement makes each view own a deep catalog snapshot, removes `Project.Cat`, routes all project consumers through the one view, restores sorted singleton derivation, strengthens bypass falsification, and corrects the misplaced test comment. Repeated focused tests cover the former shared-mutation failure.
- After implementation assurance settles, `effort-workflow` owns one lifecycle-only transaction: reconcile findings and deviations in these Notes, append only the ADR Implemented status event, set this plan to Implemented, run `./x render`, stage the ADR, plan, regenerated decision index, and lock, verify with staged check and gate, and commit them together before integration cleanup and retrospective.
