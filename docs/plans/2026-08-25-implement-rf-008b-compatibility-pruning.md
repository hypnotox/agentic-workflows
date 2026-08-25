---
format: plan-v2
date: 2026-08-25
adrs: [297, separate-live-upgrade-support-from-historical-audit-decoding]
status: Proposed
---
# Plan: Implement RF-008B Compatibility Pruning

## Goal

Make schema 46 the enforced live-source floor, isolate schemas 3 through 46 behind audit-only
historical decoding, and remove every final-census-proven live compatibility path below that floor.
Preserve current-plus-one installed releases, represented authored formats, schema-2 effort residents,
generic upgrade recovery, and deferred current inputs. Do not perform RF-014B residue cleanup,
RF-010 historical-comment cleanup, `project.Open` retirement, punctuation normalization, or lock
initialization-provenance normalization.

## Architecture summary

Separate live source classification and supported migration execution from audit history before
removing their shared compatibility machinery. Live classification recognizes the current `.awf/`
layout, treats schemas 46 through the binary current schema as supported, lets only upgrade execute a
required supported migration, and gives every unsupported schema or retired layout a typed refusal
before authority dispatch or mutation. Working and staged operations never call historical decoding;
a staged pre-adoption side is empty only when its selected tree genuinely contains no `.awf`
authority. Keep one ordered migration seam beginning at schema 46, schema-ahead and binary-version
checks, lock-last publication, and generic journal rollback and recovery.

Move historical config forward decoding and lock-shape interpretation under audit ownership. The
audit boundary accepts actual managed schemas 3 through the explicit tracked upper horizon 46,
retains represented pre-31 top-level ADR routing fields and schema-era ADR activation data, and
rejects malformed or out-of-horizon `.awf` authority without widening live support. Pre-`.awf`
revisions remain empty audit universes. Advance the tracked upper horizon only after a later managed
inventory proves that a newer binary-supported schema entered reachable history.

The final checked managed-corpus census found all ten primary tips clean at release 0.39.2 and schema
46, with no live old layout, bridge or cutover residue, schema-1 resident, legacy four-line active
memory, or plan-v2 ordinal Decision reference. No reachable managed lock contains a bridge
attestation. Reachable locks still span schemas 3 through 46 and include pre-31 routing fields. Current managed inputs still omit `initializedWithVersion` or retain inert punctuation exemptions;
those inputs remain supported and deferred. Task 1.1 reproduces one comprehensive final census and records the exact reachable-ref and primary-tip
snapshot that atomically authorizes the complete candidate set. Re-run it before a deletion only when
one of those recorded refs or tips changes; record commands, success sentinels, repository tips, and
dispositions in Notes without freezing population counts as future policy.

Remove compatibility by semantic owner after the live and historical split is green: below-floor
filesystem migrations and layout readers; live bridge authority, cutover dispatch, approval and
digest handling, and the release sentinel; schema-1 resident retirement and legacy effort-memory
conversion; then plan-v2 `#N` selectors only. Keep markerless and V1 through V4 ADRs, markerless and
plan-v1 plans, stable V4 Decision slugs, canonical effort memory, current worktree and archive
semantics, missing initialization provenance, inert punctuation inputs, factual history, and the
journal safety mechanism. Each implementation transaction applies only its matching successor-ADR
State changes and renders their generated topic and index outputs. Before the first application
transaction, move the reviewed successor ADR through Accepted to Implementing; it and this plan
remain nonterminal through implementation assurance.

**Plan flexibility.**

The protected-contract rule in the workflow document governs what a plan may not change. The plan records the best known route at authoring time, not a binding implementation choreography. A commit-capable owner may merge, split, reorder, add, remove, or replace recorded route detail while the protected contract holds. A path omitted from the plan is not alone a reason to stop, and a stale listed path need not be touched. Reapproval is required only when the protected contract would change or an unresolved material decision appears.

Reconcile a Proposed plan only when another phase or reviewer could rely on stale material instructions. Inconsequential and independently local edits require no deviation record. A delegated owner reports material cross-owner revisions for parent reconciliation. A helper remains confined to its assigned paths and gains no scope, commit, review, checkpoint, handoff, or outcome authority from route flexibility.

## Phase 1: Isolate historical audit decoding from live source operations

**Execution mode: subagent-driven.**

Completes: ["live-history-separation", "historical-audit-horizon"]

### Task 1.1: Reproduce and preserve the final managed-corpus disposition evidence
Applying: ["separate-live-upgrade-support-from-historical-audit-decoding:remove-unrepresented-compatibility", "0297:managed-removal-gate", "0297:represented-authored-formats", "0297:actual-managed-history-horizon"]
Paths: ["docs/decisions/0297-bound-compatibility-support-to-managed-reality.md", "docs/audit-remediation-program.md", "docs/plans/2026-08-25-implement-rf-008b-compatibility-pruning.md"]
Post-check: "Run one `set -euo pipefail` read-only census over exactly aeonseed, agentic-workflows, fleet, go-php, jugend-im-zentrum, nouris, pi-science, pi-tools, remote_pi, and sudoku-solver under `/home/hypno/Projects`. Require explicit success sentinels after checking clean primary tips, 0.39.2/schema-46 locks, current old layouts, bridge and cutover files, active effort and worktree residents, plan-v2 Applying and Context selectors, deferred initialization and punctuation inputs, every reachable lock schema and routing shape, and authority-presence classification at every reachable commit. The terminal disposition has no live dependency for each authorized removal, retains schemas 3 through 46 and pre-31 routing fields only in audit, preserves genuinely pre-`.awf` empty history, and identifies no uninspected repository. Record commands, tip identities, sentinels, and any changed disposition in Notes; a changed protected disposition stops implementation rather than being normalized into the plan."

Use only read operations such as `rev-list`, `ls-tree`, `cat-file`, `show`, and status inspection. Do
not fetch, checkout, clean, or mutate an adopter. Search locally reachable refs because that is the
managed audit universe at execution time. A historical approval file is evidence to classify, not a
bridge lock by implication. A missing expected config or lock is malformed authority; a tree with no
`.awf` authority is pre-adoption only when the complete selected tree proves absence.

### Task 1.2: Establish red tests for strict live, staged, and audit boundaries
Applying: ["separate-live-upgrade-support-from-historical-audit-decoding:separate-live-and-historical-compatibility", "separate-live-upgrade-support-from-historical-audit-decoding:explicit-managed-history-decoder"]
Paths: ["internal/audit/history.go", "internal/audit/history_test.go", "internal/currentstatecoord/currentstate.go", "internal/currentstatecoord/currentstate_substrate_test.go", "internal/project/staged_drift_compat_test.go", "internal/migrate/migrate.go", "internal/migrate/migrate_test.go", "internal/manifest/manifest.go", "internal/manifest/manifest_test.go"]
Post-check: "Before production changes, focused tests fail for the new boundary rather than a fixture defect. The matrix covers supported schema 46, a future registered migration above 46, live and staged schemas below 46, schema ahead of the binary, genuinely empty staged HEAD, config-without-lock and lock-without-config shapes, pre-`.awf` audit history, audit schemas 3 and 46, below-3 and above-46 locks, malformed schema and JSON, represented pre-31 routing fields, unknown historical lock fields, and config/lock shape mismatch. Working and staged tests prove no historical forward decoder call; audit tests prove no live migration or mutation call."

Preserve typed errors and presentation ownership: classifiers return semantic unsupported-source or
historical-horizon outcomes, while command or audit presentation supplies the supported floor and
recovery direction. Do not create a production fault hook. Use existing snapshot and filesystem test
seams.

### Task 1.3: Give audit its historical values and make live classification strict
Applying: ["separate-live-upgrade-support-from-historical-audit-decoding:separate-live-and-historical-compatibility", "separate-live-upgrade-support-from-historical-audit-decoding:explicit-managed-history-decoder", "0297:live-source-schema-floor", "0297:actual-managed-history-horizon", "0297:unsupported-boundaries"]
Paths: ["internal/audit/history.go", "internal/audit/history_test.go", "internal/currentstatecoord/currentstate.go", "internal/currentstatecoord/currentstate_substrate_test.go", "internal/project/staged_drift_compat_test.go", "internal/migrate/migrate.go", "internal/migrate/migrate_test.go", "internal/manifest/manifest.go", "internal/manifest/manifest_test.go", "internal/project/project.go", "internal/project/version_test.go", ".awf/topics/parts/config/migrations-and-locks/current-state.md", ".awf/topics/parts/tooling/audit-and-snapshots/current-state.md", "docs/decisions/separate-live-upgrade-support-from-historical-audit-decoding.md"]
Post-check: "Focused audit, current-state coordination, migrate, manifest, and project tests exit zero. Source dependency checks prove `internal/currentstatecoord` and ordinary project operations do not call audit historical decoding or config forward-porting; audit owns the only schemas-3-through-46 decoder and explicit upper-horizon value; historical parsing performs no writes; and live classification rejects below-floor and partial authority before dispatch. `./awf check staged` passes for an empty pre-adoption HEAD plus schema-46 index and refuses a below-46 nonempty side with the floor and recovery action."

Move only compatibility representations required by audit; do not move live manifest authority or
filesystem migration operations into audit. Historical lock decoding accepts the represented
pre-31 top-level routing fields but does not promote them into live authority. Keep schema-era ADR
format activation data with its existing semantic owner. Preserve legitimate lockless state created
inside first-adoption publication, while a user-invoked operation on an existing partial `.awf`
authority refuses with concrete recovery.

In the same transaction, move the successor ADR through Accepted to Implementing and apply exactly
these claim operations with matching topic mutations: update
`config/migrations-and-locks:retired-keys-forward-ported`, update
`config/migrations-and-locks:upgrade-gate`, update
`config/migrations-and-locks:live-source-compatibility-floor`, and update
`tooling/audit-and-snapshots:managed-history-decode-horizon`. Render the topic pages, ADR index, and
lock from their `.awf/` sources.

### Phase close

Land the audit-only historical boundary and strict live and staged classifiers with the old
below-floor migration implementations still present but unreachable from live operation dispatch.

```commit
feat(awf): separate live and historical compatibility
```

## Phase 2: Prune below-floor migration and layout machinery

**Execution mode: subagent-driven.**

Advances: ["legacy-effort-pruning"]
Completes: ["below-floor-migration-pruning"]

### Task 2.1: Complete the supported-floor refusal regression matrix
Applying: ["separate-live-upgrade-support-from-historical-audit-decoding:retain-supported-floor-migration-seam", "separate-live-upgrade-support-from-historical-audit-decoding:remove-unrepresented-compatibility"]
Paths: ["internal/migrate/migrate_test.go", "internal/upgrade/operation_test.go", "cmd/awf/upgrade_test.go", "cmd/awf/check_test.go"]
Post-check: "Extend the already-green Phase 1 refusal oracles to cover legacy single-file, `.claude/awf/`, and every below-floor classification edge before deleting their implementations. Each shape is recognized as an existing unsupported project, refuses before any config, lock, resident, or rendered byte changes, names schema 46 or the retired layout boundary, and gives a recovery action. Separate tests keep schema-ahead refusal, schema-46 no-op upgrade, future supported migration ordering, lock-last publication, and `--recover` behavior. The later production deletion preserves these results without claiming a second red state."

Do not turn a retired project layout into a fresh-adoption absence. Do not preserve the old parser
merely to improve its refusal: recognition may inspect confined path presence and a lock schema, but
must not decode or mutate the retired config representation.

### Task 2.2: Delete live schema 1 through 45 mutation implementations and fixtures
Kind: batch
Applying: ["separate-live-upgrade-support-from-historical-audit-decoding:retain-supported-floor-migration-seam", "separate-live-upgrade-support-from-historical-audit-decoding:remove-unrepresented-compatibility", "0297:live-source-schema-floor", "0297:managed-removal-gate"]
Paths: ["glob:internal/migrate/**", "internal/project/project.go", "internal/project/version_test.go", "internal/project/resident_migration_sync_test.go", "internal/upgrade/operation.go", "internal/upgrade/operation_test.go", "cmd/awf/upgrade.go", "cmd/awf/upgrade_test.go"]
Representative: "A schema-45 current-layout project and either retired layout receive the same typed below-floor refusal without invoking a parser or apply function; a schema-46 project remains a clean no-op upgrade."
Edge: "Historical schemas 3 through 45 continue to decode only through Phase 1 audit ownership, and schema-era ADR format activations remain available to stale-merge audit replay."
Post-check: "A checked production-source census reports no registered migration target below the supported floor, no legacy single-file reader, no `.claude/awf/` relocation, no schema-1 resident classifier, and no filesystem apply function from generations 1 through 45. The retained migrate surface has one explicit ordered seam beginning at schema 46, current and ahead classification, typed unsupported recognition, and no dependency on audit. Focused migrate, upgrade, project, audit, and command tests exit zero; an audit fixture at every represented schema class still decodes read-only; an unsupported live fixture leaves a before/after tree digest identical."

Delete mutation-only tests and fixtures only after the replacement refusal and retained audit tests
cover their surviving contract. Historical config transformations needed by audit were moved in
Phase 1 and are not copied back. Narrow schema minimum-version authority to supported live schemas
without changing current release support or the Pi runtime floor. Preserve `schemaVersion` and
`awfVersion` as independent lock facts.

### Task 2.3: Retire migration-specific current claims and update upgrade guidance
Applying: ["separate-live-upgrade-support-from-historical-audit-decoding:retain-supported-floor-migration-seam", "separate-live-upgrade-support-from-historical-audit-decoding:remove-unrepresented-compatibility"]
Paths: [".awf/topics/parts/config/migrations-and-locks/current-state.md", ".awf/topics/parts/config/configuration/current-state.md", ".awf/topics/parts/adr-system/adr-lifecycle/current-state.md", ".awf/topics/parts/rendering/singletons-and-payloads/current-state.md", ".awf/topics/parts/tooling/upgrade-runtime/current-state.md", ".awf/parts/working-with-awf", ".awf/docs/parts/debugging", "templates/bootstrap/awf-upgrade.sh.tmpl", "docs/decisions/separate-live-upgrade-support-from-historical-audit-decoding.md"]
Post-check: "Rendered config and upgrade documentation states schema 46 as the live floor, distinguishes supported upgrade from audit history, gives below-floor recovery without claiming legacy mutation, and retains journal recovery. `rg` over current generated guidance finds no promise that schemas 1 through 45 or either retired layout migrate; factual ADR, plan, and changelog history remains unchanged. Render and drift checks exit zero."

Apply the successor ADR removals for the migration-specific claims declared under
`config/migrations-and-locks`; update `lock-atomic-save`, `migration-ordering`, `schema-min-version`,
and `schema-version-lock`; and append the matching Applied event in the same authored transaction.
The updated claims narrow ordering to supported sources, make current-schema ownership independent of
removed historical registrations, and state atomicity through the retained lock-last journal seam.
Also update `config/configuration:awf-config-root` to remove the legacy-reader exception,
`adr-system/adr-lifecycle:corpus-raw-access-enumerated` to remove migration raw-access callers, and
`rendering/singletons-and-payloads:resident-output-preservation` to retain only current resident-root
preservation. This includes removal of the schema-1 unified effort resident migration claim, while
Phase 4 still owns the independent legacy memory reader. Preserve `corrupt-lock-refuses` and every
unaffected nonmigration current claim.

### Phase close

Land the supported-floor migration seam, obsolete migration deletion, matching claim removals, and
current upgrade guidance while audit history and generic recovery remain green.

```commit
refactor(awf): prune below-floor migrations
```

## Phase 3: Remove bridge and cutover compatibility

**Execution mode: subagent-driven.**

Completes: ["bridge-cutover-pruning"]

### Task 3.1: Establish permanent-lock and recovery oracles before bridge removal
Applying: ["separate-live-upgrade-support-from-historical-audit-decoding:remove-unrepresented-compatibility"]
Paths: ["internal/manifest/manifest_test.go", "internal/upgrade/operation_test.go", "internal/upgrade/journal_test.go", "internal/initop/init_test.go", "internal/publisher/sync_fault_test.go", "internal/publisher/sync_presentation_test.go", "cmd/awf/initrender_test.go", "cmd/awf/upgrade_test.go", "cmd/releasecheck/main_test.go"]
Post-check: "Before production deletion, tests fail because the live manifest and dispatch still accept bridge authority or the release check still exposes its sentinel. Replacement tests require current permanent locks, including absent `initializedWithVersion`, to parse and publish; any live `bridgeAttestation` field to refuse as unsupported; pre-31 historical routing to remain audit-only; schema-46 upgrade and interrupted generic journal recovery to preserve lock-last, rollback, quarantine, and postcommit cleanup semantics; and release checking to have no bridge branch."

Do not weaken unknown-field validation to remove the bridge model. The audit decoder does not accept a
bridge shape because the final reachable-history census found none. Keep the three current permanent
locks without initialization provenance valid.

### Task 3.2: Delete live bridge authority, cutover dispatch, approval, digest, and sentinel paths
Kind: batch
Applying: ["separate-live-upgrade-support-from-historical-audit-decoding:remove-unrepresented-compatibility", "0297:managed-removal-gate", "0297:represented-authored-formats"]
Paths: ["internal/manifest/manifest.go", "internal/manifest/manifest_test.go", "internal/upgrade/operation.go", "internal/upgrade/operation_test.go", "internal/upgrade/upgrade.go", "internal/upgrade/upgrade_test.go", "internal/upgrade/digest.go", "internal/upgrade/digest_test.go", "internal/upgrade/journal.go", "internal/upgrade/journal_test.go", "internal/initop/init.go", "internal/initop/init_test.go", "internal/publisher/sync.go", "internal/publisher/sync_fault_test.go", "internal/publisher/sync_presentation_test.go", "internal/currentstate/legacy_absent_test.go", "internal/currentstatecoord/currentstate_owner_test.go", "internal/contextinput/input_test.go", "internal/contextop/context_preparation_test.go", "internal/contextq/render_test.go", "internal/project/project.go", "internal/project/currentstate_test.go", "internal/project/scaffold_test.go", "internal/clispec/clispec.go", "internal/clispec/clispec_test.go", "cmd/awf/main.go", "cmd/awf/checkgroup_test.go", "cmd/awf/check_test.go", "cmd/awf/context_test.go", "cmd/awf/upgrade.go", "cmd/awf/upgrade_test.go", "cmd/releasecheck/main.go", "cmd/releasecheck/main_test.go", "templates/bootstrap/awf-upgrade.sh.tmpl", "templates/docs/debugging.md.tmpl"]
Representative: "A permanent schema-46 lock follows the ordinary supported path, while a lock containing the retired bridge field receives one unsupported-format error before init, publication, or upgrade mutation."
Edge: "A current journal in any valid precommit or postcommit phase remains recoverable even though bridge finalization and migration-approval deletion no longer exist."
Post-check: "A production-source and template census reports no BridgeAttestation value, AuthorityBridge state, FinalUpgrade, approval parser, attestation digest, current-state migration path, BridgeTrancheComplete sentinel, or bridge-specific command help. The generic journal model, file and tree operations, quarantine ownership, lock commit point, rollback, recovery command, command blocking, and diagnostics remain reachable and fully tested. Focused manifest, upgrade, init, publisher, CLI-spec, command, releasecheck, audit, and project tests exit zero; default and self-hosted debugging and help output meaning-review cleanly; then `go test ./...` exits zero."

Remove bridge-only fields from the live lock representation and collapse live authority to permanent
lock validation without changing initialization-provenance compatibility. Remove only journal
operations whose sole semantic purpose is bridge approval or cutover; retain general transactional
file, tree, lock, quarantine, rollback, and recovery behavior used by supported upgrades. Read back
all generated bootstrap and command help after rendering.

### Task 3.3: Retire bridge current claims and publish the unsupported-format boundary
Applying: ["separate-live-upgrade-support-from-historical-audit-decoding:remove-unrepresented-compatibility"]
Paths: [".awf/topics/parts/tooling/upgrade-runtime/current-state.md", ".awf/topics/parts/tooling/cli/current-state.md", ".awf/topics/parts/config/migrations-and-locks/current-state.md", ".awf/topics/parts/code-design/dependency-composition/current-state.md", ".awf/domains/parts/tooling/current-state.md", ".awf/docs/parts/releasing", ".awf/docs/parts/debugging", "docs/decisions/separate-live-upgrade-support-from-historical-audit-decoding.md"]
Post-check: "Apply exactly the successor ADR bridge and cutover claim removals, remove `code-design/dependency-composition:upgrade-attestation-filesystem-wiring`, and update `tooling/upgrade-runtime:managed-cutover-format-support` plus `tooling/cli:group-child-project-guard-exemption` in the same transaction and Applied event. Retire the tooling-domain bridge paragraph. Rendered topics, release guidance, default and self-hosted debugging guidance, upgrade help, bootstrap output, ADR index, and lock describe permanent authority plus generic journal recovery without an attestation guard, bridge consumption promise, or approval-file instruction. Render, drift, staged transition, and focused prose checks exit zero."

Retain `upgrade-failure-is-recoverable`, `initial-adoption-version-immutable`, and
`upgraded-runtime-has-one-authority-engine`. Historical ADRs and plans remain factual records; broad
tranche-comment cleanup stays assigned to RF-010.

### Phase close

Land permanent-only live lock authority and remove bridge and cutover compatibility while retaining
historical routing decode and generic transaction recovery.

```commit
refactor(awf): retire bridge cutover compatibility
```

## Phase 4: Remove legacy effort-memory compatibility

**Execution mode: subagent-driven.**

Completes: ["legacy-effort-pruning"]

### Task 4.1: Replace legacy update and preview fixtures with canonical-only refusal coverage
Applying: ["separate-live-upgrade-support-from-historical-audit-decoding:remove-unrepresented-compatibility"]
Paths: ["internal/effort/memory_metadata_test.go", "internal/effort/memory_test.go", "internal/effort/memory_operations_test.go", "internal/effort/activity_test.go", "internal/effort/store_test.go"]
Post-check: "Before production changes, focused tests fail because an exact legacy four-line memory still reads, previews, or converts. Replacement fixtures require canonical YAML identity and metadata for read, body edit, structured update, preview, store selection, and activity association; malformed or legacy documents refuse without publication and direct the user to recover or recreate canonical memory. Existing canonical pagination, edit atomicity, preview bounding, owner advisory, and publication-uncertainty tests remain unchanged and green after implementation."

Do not change the immutable effort identity, schema-2 state, activity protocol 2, managed worktree
lifecycle, scratch opacity, or archive move. The user-managed effort executing this plan remains a
live canonical fixture and must not be modified by tests.

### Task 4.2: Delete the four-line reader, conversion, sentinel, and legacy preview representation
Applying: ["separate-live-upgrade-support-from-historical-audit-decoding:remove-unrepresented-compatibility", "0297:represented-authored-formats", "0297:managed-removal-gate"]
Paths: ["internal/effort/memory_metadata.go", "internal/effort/memory_operations.go", "internal/effort/store.go", "internal/effort/activity.go", "internal/effort/types.go", "internal/effort/memory_metadata_test.go", "internal/effort/memory_test.go", "internal/effort/memory_operations_test.go", "internal/effort/activity_test.go", "internal/effort/store_test.go", "internal/effort/presentation_test.go", "internal/project/plan_execution_workflow_template_test.go", "cmd/awf/effort_test.go", "tools/pi-extension-test/tests/using-effort.test.ts", ".awf/skills/parts/retrospective/procedure.md", "templates/skills/orienting/SKILL.md.tmpl", "templates/skills/subagent-driven-development/SKILL.md.tmpl", "templates/skills/executing-plans/SKILL.md.tmpl", "templates/skills/effort-workflow/SKILL.md.tmpl", "templates/skills/roadmap-graduation/SKILL.md.tmpl", "templates/skills/retrospective/SKILL.md.tmpl", "templates/skills/using-effort/SKILL.md.tmpl", "templates/partials/checkpoint-approval.md", "templates/partials/checkpoint-routine.md", "templates/pi/awf-effort/client.ts.tmpl", "templates/docs/workflow.md.tmpl", ".awf/topics/parts/tooling/effort-management/current-state.md", ".awf/topics/parts/rendering/pi-runtime/current-state.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", "docs/decisions/separate-live-upgrade-support-from-historical-audit-decoding.md"]
Post-check: "A production and template census reports no legacy four-line parser, `Not yet updated.` timestamp sentinel, legacy conversion, legacy source offset, or promise of dual-format memory acceptance. Canonical memory read, edit, update, preview, activity, store, worktree, finish, archive, rendered workflow, checkpoint, using-effort projection, Pi handoff, and Pi effort-client protocol tests exit zero. Apply updates to `tooling/effort-management:memory-skeleton-purpose-partition`, `tooling/effort-management:managed-effort-format-support`, `rendering/pi-runtime:pi-session-handoff-workflow`, and `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage` with the matching ADR Applied event; meaning-review generated consumers, then require render and drift checks to exit zero."

Preserve the canonical YAML frontmatter fields and body contract exactly. Update workflow and
orientation guidance to require canonical identity without changing effort association, one-writer,
handoff, checkpoint, or worktree confinement semantics.

### Phase close

Land canonical-only effort memory and the final removal of legacy effort compatibility with all
schema-2 lifecycle behavior preserved.

```commit
refactor(awf): remove legacy effort memory support
```

## Phase 5: Remove plan-v2 ordinal Decision selectors

**Execution mode: inline.**

Completes: ["plan-selector-pruning"]

### Task 5.1: Prove stable-slug-only plan-v2 selection before narrowing resolution
Applying: ["separate-live-upgrade-support-from-historical-audit-decoding:remove-unrepresented-compatibility", "0297:represented-authored-formats", "0297:managed-removal-gate"]
Paths: ["internal/plan/structure.go", "internal/plan/structure_test.go", "internal/plancheck/plancheck.go", "internal/plancheck/plancheck_test.go", "internal/adr/decision.go", "internal/adr/decision_test.go"]
Post-check: "Before production changes, a new plan-v2 fixture expecting `ADR:#N` rejection fails because ordinal compatibility still resolves. After implementation, malformed and canonical positive ordinal selectors both fail with stable-slug guidance; V4 slug selectors, retained pending and numeric ADR identities, Applying and Context membership, markerless plans, plan-v1 bytes, and pre-V4 ADR Decision enumeration outside plan-v2 resolution remain green. Focused plan, plancheck, ADR, project validation, and executable-plan tests exit zero."

Remove only plan-v2 ordinal reference syntax and resolution. Do not delete frozen pre-V4 ADR parsing,
Decision enumeration, historical navigation, markerless or plan-v1 parsing, numeric ADR links, or
retained-slug links.

### Task 5.2: Update plan authority and authoring guidance
Applying: ["separate-live-upgrade-support-from-historical-audit-decoding:remove-unrepresented-compatibility"]
Paths: [".awf/topics/parts/adr-system/plan-artifacts/current-state.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", ".awf/domains/parts/adr-system/current-state.md", ".awf/parts/adr-readme", "templates/plans-readme/README.md.tmpl", "templates/plans-template/template.md.tmpl", "templates/skills/writing-plans/SKILL.md.tmpl", "templates/skills/reviewing-plan/SKILL.md.tmpl", "templates/skills/adr-lifecycle/SKILL.md.tmpl", "internal/project/plan_execution_workflow_template_test.go", "docs/plans/template.md", "docs/decisions/separate-live-upgrade-support-from-historical-audit-decoding.md"]
Post-check: "Apply updates to `adr-system/plan-artifacts:plan-v2-decision-references`, `adr-system/plan-artifacts:managed-plan-format-support`, and `rendering/workflow-skill-templates:plan-task-detail-modes` with the matching ADR Applied event. Narrow the ADR-system domain plan-reference sentence to stable V4 Decision slugs while retaining ordinals only for frozen ADR history. Rendered ADR, plan, writing-plan, reviewing-plan, ADR-lifecycle, and plan-template guidance contains no current plan-v2 `#N` authoring instruction. Meaning-review every generated consumer, then require render, drift, plan-corpus validation, and staged checks to exit zero."

Keep factual historical plan text unchanged. Do not rewrite existing markerless or plan-v1 records.

### Phase close

Land stable-slug-only plan-v2 Decision references and matching active authority without changing
represented historical artifact parsing.

```commit
refactor(adr-system): remove ordinal plan decision selectors
```

## Phase 6: Prove managed upgrades and close RF-008B currency

**Execution mode: inline.**

Completes: ["managed-compatibility-proof", "current-authority-release"]

### Task 6.1: Exercise the candidate binary against disposable copies of every managed repository
Applying: ["separate-live-upgrade-support-from-historical-audit-decoding:separate-live-and-historical-compatibility", "separate-live-upgrade-support-from-historical-audit-decoding:retain-supported-floor-migration-seam", "separate-live-upgrade-support-from-historical-audit-decoding:explicit-managed-history-decoder", "separate-live-upgrade-support-from-historical-audit-decoding:remove-unrepresented-compatibility", "0297:rolling-installed-release-floor", "0297:managed-removal-gate", "0297:unsupported-boundaries"]
Paths: ["cmd/awf", "internal/audit", "internal/migrate", "internal/upgrade", "internal/effort", "internal/plan", "internal/plancheck"]
Post-check: "Build one candidate `awf` binary from the exact staged implementation tree. For each of the ten recorded clean primary tips, create a uniquely owned disposable local clone outside every source repository, run candidate `upgrade`, candidate `check`, and the repository-required gate, then confirm the source repository tip and status are unchanged and remove only the proven disposable root. Every supported schema-46 copy succeeds. Separate confined fixtures prove retired layouts and schemas below 46 refuse before mutation with floor and recovery text, schemas above current refuse, audit schemas 3 through 46 succeed read-only, and out-of-horizon or malformed audit authority refuses. End with explicit success sentinels and record commands, candidate digest, source tips, outcomes, and cleanup evidence in Notes."

Do not run candidate upgrade against a managed source checkout. A disposable copy may render new
outputs, but its post-upgrade check and repository gate must use the candidate-compatible tree. A
repository gate failure is blocking even when candidate `awf check` passes. Cached historical
binaries are not support evidence.

### Task 6.2: Update release, upgrade, and audit-program current truth
Applying: ["separate-live-upgrade-support-from-historical-audit-decoding:separate-live-and-historical-compatibility", "separate-live-upgrade-support-from-historical-audit-decoding:retain-supported-floor-migration-seam", "separate-live-upgrade-support-from-historical-audit-decoding:explicit-managed-history-decoder", "separate-live-upgrade-support-from-historical-audit-decoding:remove-unrepresented-compatibility"]
Paths: ["changelog/CHANGELOG.md", "docs/audit-remediation-program.md", ".awf/parts/working-with-awf", ".awf/docs/parts/releasing", ".awf/docs/parts/debugging", ".awf/topics/parts/config/migrations-and-locks/current-state.md", ".awf/topics/parts/tooling/audit-and-snapshots/current-state.md", ".awf/topics/parts/tooling/effort-management/current-state.md", ".awf/topics/parts/tooling/upgrade-runtime/current-state.md", ".awf/topics/parts/adr-system/plan-artifacts/current-state.md", "docs/plans/2026-08-25-implement-rf-008b-compatibility-pruning.md"]
Post-check: "Release notes state the schema-46 live floor, retired-layout and below-floor refusal, audit-only schemas-3-through-46 horizon, removed bridge, effort, and ordinal compatibility, and retained recovery and represented formats. The audit program records RF-008B range, candidate census, protected contract, verification, deviations, residual deferred inputs, and RF-014B applicability without changing another issue's status prematurely. Rendered current docs agree with command help and contain no transitional constant without a stated live owner or removal condition. Render and drift checks, plan and ADR validation, and prose gates exit zero."

Retain factual historical changelog entries. Record RF-014B candidates as applicable or inapplicable
from the actual remaining residue rather than forcing a deletion. Do not perform RF-014B or RF-010
in this transaction.

### Task 6.3: Run final repository and release assurance
Applying: ["separate-live-upgrade-support-from-historical-audit-decoding:separate-live-and-historical-compatibility", "separate-live-upgrade-support-from-historical-audit-decoding:retain-supported-floor-migration-seam", "separate-live-upgrade-support-from-historical-audit-decoding:explicit-managed-history-decoder", "separate-live-upgrade-support-from-historical-audit-decoding:remove-unrepresented-compatibility"]
Paths: ["pathspec::(top)**", "docs/decisions/separate-live-upgrade-support-from-historical-audit-decoding.md", "docs/plans/2026-08-25-implement-rf-008b-compatibility-pruning.md"]
Post-check: "From a clean candidate tree, run focused owner tests, `go test ./...`, `./x render`, `./x check`, `./awf check staged`, `./x gate`, release checks, local workflow audit over the implementation range, production reachability, binary-version validation, and the coverage identity and targeted mutation gates selected by the changed paths. All blocking checks exit zero; generated diff is empty; raw coverage has no unauthorized identity change; no removed compatibility symbol has a production caller; and only explicitly nonblocking pre-existing advisories remain. Inspect the full range to prove no RF-014B, RF-010, `project.Open`, punctuation-normalization, initialization-provenance, represented-parser, generic-journal, or historical-record cleanup entered the implementation. Record exact commands, ranges, outcomes, and advisory dispositions in Notes."

The successor ADR remains Implementing after its final explicit Applied batch. Independent
implementation assurance and effort workflow own the later status-only ADR and plan terminal closure,
integration, worktree removal, audit-program topology confirmation, retrospective, and effort finish.

### Phase close

Land release and program currency plus final verification evidence without terminally closing the ADR,
plan, or effort before independent assurance.

```commit
docs(awf): record RF-008B compatibility pruning
```

## Definition of done

- `dod: live-history-separation` Live and staged operations use strict schema-floor classification and never call audit historical decoding; only upgrade executes a supported migration.
- `dod: historical-audit-horizon` Audit alone decodes represented schemas 3 through the explicit managed upper horizon 46, retains pre-31 routing shapes, treats pre-`.awf` history as empty, and refuses malformed or out-of-horizon authority read-only.
- `dod: below-floor-migration-pruning` No live schema 1 through 45 mutation, legacy single-file reader, retired tree relocation, or schema-1 resident reset path remains; one schema-46-supported future migration seam and all current safety gates remain.
- `dod: bridge-cutover-pruning` Live manifest, upgrade, command, release, template, and current-authority surfaces contain no bridge attestation, approval, adjudication, marker cleanup, digest, cutover dispatch, or release sentinel, while generic journal rollback and recovery remain.
- `dod: legacy-effort-pruning` Effort memory accepts canonical YAML only, and schema-2 residents, activity, worktrees, scratch, updates, previews, and archive moves retain their current contracts.
- `dod: plan-selector-pruning` Plan-v2 accepts stable V4 Decision slugs but no frozen ordinal selector, while every represented ADR and plan format outside that selector remains supported.
- `dod: managed-compatibility-proof` The final census has no candidate dependency, every supported managed repository succeeds in a disposable candidate upgrade and check, unsupported inputs fail before mutation with recovery guidance, and source repositories remain unchanged.
- `dod: current-authority-release` Matching ADR State changes, current-state sources, rendered docs, help, release notes, audit-program evidence, and plan Notes describe the implemented boundary; full gates and independent assurance remain at least as strong as before.

## Notes

Apply the plan-flexibility rule above when recording deviations. Delegated owners report material cross-owner revisions rather than editing the plan; the parent supplies the report to phase review and reconciles required plan changes with findings in one focused post-review settlement commit before checkpointing or later execution. Record the final managed-corpus census commands and sentinels, each explicit ADR application batch, disposable-adopter verification, review findings, deviations, release evidence, and residual RF-014B disposition here as execution proceeds.

Initial plan review dispositions: mechanically assigned every ADR-0297 Decision through substantive Applying tasks and replaced Phase 2's unreachable second red claim with behavior-preserving deletion evidence. Reasoned corrections made Task 1.1's checked ref snapshot the atomic authorization for the complete candidate set, added successor-ADR updates for the retained migration claims whose wording changes, and added the workflow-template claim and all authored plan guidance needed to remove `#N` instructions without weakening historical ADR navigation.

Fresh review after ADR amendment found additional current-authority and generated-consumer closure. Reasoned correction adds updates for ADR raw-access enumeration, config-root ownership, and resident preservation in Phase 2. Mechanical corrections remove the attestation filesystem-wiring claim in Phase 3, update checkpoint-chain identity authority and every generated legacy-memory consumer in Phase 4, and add default debugging, CLI help, and ADR-lifecycle plan-reference sources with focused tests and meaning review. Final mechanical closure also updates the CLI group-child guard and tooling-domain prose, Pi handoff authority and using-effort projection, and ADR-system domain guidance in their owning phases. Executability verification added every direct staged-forwarding, resident-migration, bridge-type, legacy-timestamp, and plan-guidance test consumer found by deterministic source census to its owning phase.

Post-review settlement at `ae3e51763`: Phase 1 now refuses below-floor and incomplete live authority before bridge or migration dispatch, keeps the bridge path only as a Phase 2 deletion candidate, and treats every `.awf/**` residue without the complete control pair as historical partial authority. Audit owns the read-only schema-3-through-46 decoder and accepts unknown historical lock fields without importing live migration or manifest decoding. Semantic classifiers carry facts only; CLI and audit presentation boundaries render recovery direction.

Verification completed after settlement: focused `go test ./cmd/awf ./internal/project ./internal/upgrade ./internal/audit ./internal/migrate`, full `go test ./...`, `./awf check` (warnings only: advisory verification-discipline tag concentration), and `./x gate` all passed. The whole-module coverage profile was regenerated through `cmd/covercheck --generate-policy`; `coverage-baseline.json` records the measured 96.8% raw and 99.9% filtered profile. `coverage-review.json` admits only exact uncovered defensive error-formatting and legacy package-only migration identities: decoder malformed-formatting arms, command/transaction closed lowering arms, replay/current-state fault-invariant arms, and schemas retired from the schema-46 live route. Reachable partial authority, horizon, malformed decoder, unknown-field, and no-write paths are covered rather than ignored.

Renewed Phase 1 settlement after review: command admission now rejects below-floor, retired-layout,
and partial authority before bridge or AuthorityState dispatch; staged HEAD emptiness examines the
complete `.awf/**` tree; gate and upgrade presentation share migration-owned live classification;
and audit owns its refusal presentation. Red-first command, audit, migrate, upgrade, current-state,
and staged-tree tests cover every supported or input-reachable branch. The obsolete archive-root
migration claim was removed in this settlement because the strict floor made its live-upgrade
promise false; the retained bridge and migration implementations remain Phase 2 scope.

Coverage reconciliation added direct tests for audit passthrough, future supported migration,
retired-layout and partial-authority refusal, staged retired authority, and init loader-open failure.
Only shifted closed-invariant branches, concurrent filesystem disappearance, and migration paths
retained until Phase 2 remain reviewed misses. `go test ./...` passed before reconciliation, the
focused init test passed afterward, and the regenerated policy reports 96.8% raw and 99.9% filtered
coverage.

Final renewed-review settlement makes live schema admission precede current authority decoding in
manifest, command, staged, migrate, and upgrade paths. Red-first tests proved that malformed
below-floor bridge bytes previously reached `AuthorityState`; replacement tests also cover lock
absence after presence observation and both current control-path stat failures. The command guard now
checks lock presence before using the loaded value and preserves non-absence stat errors. Focused
owner tests and `go test ./...` pass. Coverage reconciliation directly covers the new reachable
branches, retains one reviewed command arm for the future supported-migration seam, and regenerates
the 96.8% raw and 99.9% filtered baseline.

Final verify settlement after `a9afe1fd2` closes two mechanical upgrade-owner findings. Current-pair
filesystem failures now propagate before dispatch, and the operation revalidates the pair after
schema classification so a disappeared or faulting lock cannot reach migration, gate, or sync.
The disappearance regression failed red with the prior nil dereference, and temporary propagation
falsification made the config-stat regression fail as expected. Focused upgrade tests and a fresh
whole-module coverage run pass; the regenerated profile reports 96.8% raw and 99.9% filtered
coverage without the obsolete upgrade stat exclusion.

The renewed verify found one further mechanical stale-value path: presence revalidation alone did not
replace the originally loaded lock after schema classification. The operation now reloads the live
lock and uses that value for authority dispatch, while the loader itself proves lock presence and
errors and a separate config probe proves the complete current pair. A replacement regression failed
red against the stale lock, and focused plus whole-module coverage tests pass with no new exclusion.

The final Phase 1 verify settlement moves Full-only capability interpretation behind live authority
admission and reads the staged config from the index for staged commands. Red-first command tests
proved that malformed working config could precede schema-45 refusal and that a working Core profile
could override a staged Full profile. The replacement also preserves an empty pre-adoption staged
universe. Focused and full tests pass; coverage adds only the reviewed concurrent repository-loss
arm and otherwise shifts existing closed-state identities.

The settlement review found that `context --staged` remained outside the command-path-only staged
selector. A mechanical follow-up gives command admission and capability interpretation one shared
selected-universe predicate. Red-first inverse regressions prove that staged Full overrides working
Core and staged Core overrides working Full for context queries. Focused and full tests pass, and the
coverage policy only shifts the already-reviewed command-boundary identities.

Phase 2 application batch: removed the schema-1-through-45 live migration registry, filesystem mutation implementations, retired-layout readers and relocation, schema-1 resident reset entry point, and their mutation fixtures. The retained live seam starts at schema 46; focused refusal tests prove legacy single-file, retired-tree, below-floor, and ahead inputs leave fixture bytes unchanged, while schema 46 is a no-op and an injected next supported migration remains ordered. Audit historical decoding remains isolated. Verification: `go test ./internal/migrate ./internal/upgrade ./cmd/awf ./internal/project`, `./x render && ./x check` (one existing advisory warning), and `go test ./...` passed. The route additionally updated source-census tests that previously enumerated removed migration-only corpus and pitfall consumers; this is a bounded consequence of deleting their production seams. Parent completion after the delegated owner stopped before coverage reconciliation removed helper APIs that became unreachable with the migration implementations, retained topology coverage through the repository-owned worktree list, and removed `config/validation:glob-migration-anchored` because its only behavior and proof were the deleted schema-7 migration. These omitted paths are direct dead-code and claim-closure consequences of the approved pruning boundary; `./x deadcode` passes after their removal.

Coverage reconciliation removed reviews for deleted migration code, relocated only source-preserved defensive identities, and directly covered supported classifier branches, future migration failure, schema minimum refusal, manifest lock-read failure, initial upgrade lock-stat failure, and repository worktree-list refusals. The remaining new admissions are three adjacent filesystem-race propagation arms in command and upgrade admission. The unused detached-worktree fixture helper was removed after its migration-only callers disappeared. A fresh canonical profile reports 96.9% raw and 99.9% filtered coverage.

Phase 2 review settlement removed the relocated-lock injection and second migration dispatch, made the supported migration registry validate and plan plural ascending steps without filesystem mutation, and mapped replacement and removal plans into the retained upgrade journal with the current-schema lock last. Focused journal tests prove committed replacement and removal, pre-lock rollback, lock stamping, and ordinary recovery ownership. The current config root now has no migration-reader exemption, while retired layouts remain presence-only refusals and non-absence stat failures propagate. The resident-root proof now syncs a schema-46 project twice and preserves every owned root descendant byte and mode. Focused and full tests pass; the reconciled canonical profile reports 96.9% raw and 99.9% filtered coverage.

Renewed Phase 2 review settlement gives ordered future migration steps a read-only proposed-tree overlay and coalesces repeated path plans before journal construction. The generic journal now routes all reads, preimages, replacements, resident-tree renames, cleanup, recovery, and journal publication through its root-confined filesystem handle; the handle gained confined rename and recursive removal operations. Project presence preserves non-absence stat failures. Focused tests prove later migration steps observe earlier plans, physical lock-last application, symlink-ancestor refusal without outside mutation, retired-layout presence-only production ownership, and presence-error propagation. The attestation wiring census excludes the independently confined generic journal by purpose rather than treating its handle as attestation state. Full tests, render and drift checks, dead-code analysis, canonical 96.9% raw and 99.9% filtered coverage policy, and the full gate pass.

The single Phase 2 verify pass found one reasoned transaction-lifetime defect and five mechanical proof or seam gaps. The settlement binds each production journal transaction and recovery to one opened confined handle so a repository-root rebind cannot split durable work across trees; this follows the root-confined and lock-atomicity authorities without changing the migration contract. Migration and cutover preimages use the same injected operation, parent-creation failures are covered rather than excluded, the retired-layout census admits only enumerated presence operations, rename proves destination confinement, and migration planning joins handle-close failures. Red tests reproduced the root-rebind split and bypassed migration preimage seam; mutation checks falsified the retired-layout and rename proofs. Focused and full tests, render and drift checks, dead-code analysis, canonical 96.9% raw and 99.9% filtered coverage, and the full gate pass.

Phase 3 application: permanent schema-46 locks, including locks without `initializedWithVersion`, remain accepted. A bridge-shaped live lock now refuses as an unsupported field before command dispatch or mutation. Removed the bridge model, attestation digest and approval/cutover transaction, command guard, release sentinel, and generated guidance; retained the generic root-confined journal and recovery. Applied the successor ADR removals and retained-claim updates, then rendered current outputs. Red evidence: the replacement manifest bridge-field oracle failed before the lock model deletion because the live parser accepted the field. The phase owner reported no deviations or omitted in-boundary consumers.

Phase 3 review settlement addressed four mechanical findings: releasing guidance and upgrade-runtime metadata and introduction now describe permanent authority and generic journal recovery, the staged commit-child proof stages a journal and pairs its success with a nonexempt refusal control, and the orphaned release-sentinel comment is gone. Temporarily clearing the child's `StateExempt` flag made the backing test fail. The settlement census additionally found and corrected stale bridge-era package guidance and recovery diagnostics in the journal, init, publisher, command guard, and adjacent tests; these omitted paths are current-currency consequences inside the approved bridge-removal boundary, not RF-010 historical-comment cleanup. Generated releasing and upgrade-topic boundaries meaning-review as permanent-lock and supported-migration guidance. Focused and full tests, render and drift checks, dead-code analysis, regenerated canonical coverage at 96.9% raw and 99.8% filtered, and the full gate pass.

The renewed Phase 3 review found only mechanical cleanup after the bridge-removal behavior settled. The settlement removes the now test-only Git HEAD hash entrypoint and helper, drops the deleted digest from the presentation allowlist, makes command admission commentary permanent-lock specific, turns the empty migration-approval seam inventory into a direct absence proof, and names the ordinary rendered-output failure fixture by its actual role. Focused and full tests, render and drift checks, dead-code analysis, canonical 96.9% raw and 99.8% filtered coverage, and the full gate pass.

The final fresh Phase 3 verify review found one mechanical audit-boundary residue: the historical lock decoder still ignored the unrepresented retired `bridgeAttestation` field. A complete-pair audit regression failed red because the bridge-shaped lock was accepted, then passed after the audit-owned decoder rejected that exact field while preserving other represented unknown historical fields. Focused and full tests, render and drift checks, dead-code analysis, canonical 96.9% raw and 99.8% filtered coverage, and the full gate pass.

Phase 4 application: red-first legacy-memory update, identity, read, and preview tests failed while the exact four-line resident remained accepted. Canonical-only parsing now refuses legacy bytes without publication; the sentinel, conversion, and preview offset path are removed. Applied the four declared current-state updates and successor-ADR event, then rendered generated guidance and Pi client output.
