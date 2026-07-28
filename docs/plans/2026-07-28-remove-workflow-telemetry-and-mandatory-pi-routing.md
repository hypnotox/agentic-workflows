---
date: 2026-07-28
adrs: [167]
status: Proposed
---
# Plan: Remove workflow telemetry and mandatory Pi routing

## Goal

Implement ADR-0167 in one atomic implementation commit. The cutover removes telemetry, Pi-session assignment, `/awf-effort`, `awf_workflow`, the hidden Pi workflow bodies, and mandatory workflow-transition prose. It retains efforts, optional memory, managed worktrees, direct effort commands, native Pi skills, and Pi handoff. Generation 21 deletes the disposable `.awf/metrics` and `.awf/assignments` resident roots.

This plan deliberately has one phase and one commit. Registering generation 21 makes a schema-20 adopted tree fail the binary/schema gate immediately, and ADR operation 1 precedes every catalog operation. A migration-only commit would therefore strand root and Sundial, while an intermediate catalog/compatibility commit would retain contradictory telemetry/router behavior. Do not add a compatibility extension, legacy router, dual guide, bridge command, or temporary resident-root preservation.

## Architecture summary

### Exact cutover contract

`catalog.WorkflowProfile` replaces `WorkflowMapping`, `SkillSpec.Chain`, and `SkillSpec.Trigger`:

```go
type WorkflowProfile struct {
    Kind            WorkflowKind
    Purpose         string
    Trigger         string
    UsuallyFollows  []string
    CommonFollowUps []string
}
```

`SkillSpec.Profile WorkflowProfile` is required for every standard and synthesized local skill. Every standard `SkillSpec.RequiresSkills` is `[]string{}`; `AgentSpec.RequiresSkills` remains the structural artifact dependency contract. Profile neighbors are advisory only: each name must exist, must not name itself, and must be unique within its own list; they preserve declaration order, do not require the neighbor to be enabled, and never contribute an enablement/closure edge.

Use this complete standard catalog (the text is the required literal data target for `internal/catalog/standard.go`):

| skill | kind | concise purpose | invocation trigger | usually follows | common follow-ups |
|---|---|---|---|---|---|
| brainstorming | chain | Clarify an outcome and settle a grounded design. | Use for non-trivial work before deciding its design. | - | proposing-adr, writing-plans, executing-direct |
| writing-plans | chain | Turn an approved design into an executable plan. | Use when implementation needs a durable, reviewable plan. | brainstorming, proposing-adr | reviewing-plan |
| executing-direct | chain | Implement a small approved change directly. | Use when the change is understood and does not need a plan. | brainstorming | reviewing-impl |
| executing-plans | chain | Implement an accepted plan. | Use when a plan is ready for implementation. | writing-plans, reviewing-plan | reviewing-impl, retrospective |
| subagent-driven-development | chain | Implement a plan through reviewed subagent tasks. | Use when plan execution benefits from delegated implementation tasks. | writing-plans, reviewing-plan | reviewing-impl, retrospective |
| tdd | support | Drive a change from a failing test. | Use when writing the failing test before the implementation change. | - | executing-direct, executing-plans |
| debugging | task | Investigate a defect before changing it. | Use when investigating a bug or unexpected behaviour before any fix. | - | bugfix, executing-direct |
| exploring | support | Explore repository facts without polluting the main context. | Use for fresh-context repository exploration when inline search would pollute the parent context. | - | brainstorming, debugging, refactor-coupling-audit |
| proposing-adr | chain | Author a decision record for a material design choice. | Use when a durable architectural or workflow decision is needed. | brainstorming | reviewing-adr, writing-plans |
| adr-lifecycle | support | Apply an ADR lifecycle transition correctly. | Use when transitioning an ADR between lifecycle states. | proposing-adr, reviewing-adr | executing-plans, writing-plans |
| bugfix | task | Apply a fix with a known root cause. | Use when applying a fix whose root cause is already known. | debugging | reviewing-impl |
| reviewing-plan | chain | Independently review an implementation plan. | Use when a written plan needs review before execution. | writing-plans | reviewing-plan-resync, executing-plans |
| reviewing-plan-resync | chain | Reconcile a plan after review findings. | Use when review findings require a plan revision and re-review. | reviewing-plan, reviewing-adr | executing-plans, subagent-driven-development |
| reviewing-adr | chain | Independently review an ADR. | Use when a proposed ADR needs decision-quality review. | proposing-adr | reviewing-plan-resync, writing-plans |
| reviewing-impl | chain | Independently review an implementation. | Use when an implementation commit or series needs review. | executing-direct, executing-plans, subagent-driven-development | retrospective |
| retrospective | chain | Capture and promote lessons from completed work. | Use after implementation review when a recurrence or improvement is worth recording. | reviewing-impl | - |
| refactor-coupling-audit | support | Scope dependency and test coupling before a refactor. | Use when scoping a refactor that moves files between packages or inverts dependencies. | exploring | brainstorming, writing-plans |
| roadmap-graduation | support | Move a shipped roadmap item out of the roadmap. | Use when graduating a shipped roadmap item out of the roadmap doc. | reviewing-impl | - |

A synthesized local skill is always `task`, has no neighbors or structural requirements, and obtains both `Purpose` and `Trigger` from `strings.TrimSpace(sidecar.data.description)` only when that value is a nonempty string. Otherwise both fields are exactly `A project-local skill.` and `Use when the project-local skill's rendered description fits the current work.` respectively.

The full guide emits one declaration-order row for every enabled standard and synthesized local skill, exactly:

```text
- `<prefix>-<name>` (<kind>): <purpose>. Trigger: <trigger>. Usually follows: <name>, <name>. Common follow-ups: <name>, <name>.
```

`Trigger: <trigger>.` is distinct from and immediately follows the purpose sentence. Omit the complete ` Usually follows: ...` and/or ` Common follow-ups: ...` clause when its list is empty. Escape no missing value into the output: unset/missing `prefix`, name, kind, purpose, trigger, or list data must render coherent generic prose and never an unresolved-value token. Standard rows assert the catalog's exact literal purpose and trigger; synthesized local rows assert the trimmed description for both purpose and trigger, or their distinct specified fallbacks when the description is missing, non-string, empty, or whitespace-only. The guide also says: `Any enabled skill may be used whenever its purpose fits the current work; the listed relationships are recommendations, not prerequisites or required next steps.`

### Exact current-state mutation contract

Apply these ADR-0167 operations in declaration order. For an update, retain `Origin`, append `ADR-0167` once to `Revised-by`, and replace the whole claim body with the text below. For a remove, delete the entire claim block and every matching invariant proof marker.

1. **update `config/migrations-and-locks:workflow-telemetry-config-migration`**

   Schema generation 21 removes only `.awf/metrics` and `.awf/assignments` from the primary control root. It reports each root in that order as removed or already absent, refuses symlinks and non-directories without removal, and permits retry after partial failure while ordinary schema, lock, render, drift, discovery, sweep, and uninstall ownership excludes both roots.

   Origin: ADR-0146
   Revised-by: ADR-0164, ADR-0167
   Backing: test
   Proof: `internal/migrate/remove_workflow_residents_test.go:TestRemoveWorkflowResidentsMigration`, `// invariant: config/migrations-and-locks:workflow-telemetry-config-migration`.
2. **update `rendering/catalog-and-targets:enabled-set-closed`**

   Every enabled non-local artifact's direct structural catalog requirements (required skills, agents, and docs) must themselves be enabled; advisory workflow-profile neighbors do not create enablement edges, and an unmet structural requirement fails project open with a repair hint.

   Origin: ADR-0081
   Revised-by: ADR-0167
   Backing: test
   Proof: `internal/project/project_test.go:TestOpenRefusesUnclosedEnabledSet`, `// invariant: rendering/catalog-and-targets:enabled-set-closed`.
3. **remove `rendering/catalog-and-targets:exploration-skill-closure`**

   Delete the entire claim block and every `// invariant: rendering/catalog-and-targets:exploration-skill-closure` marker.
4. **update `rendering/catalog-and-targets:requires-skills-exact`**

   Every standard skill has an empty `RequiresSkills`; workflow-profile neighbors are advisory only. Artifact requirements, including reviewing agents' structural `RequiresSkills`, remain exact declared dependencies rather than workflow edges.

   Origin: ADR-0080
   Revised-by: ADR-0167
   Backing: test
   Proof: `internal/catalog/workflow_test.go:TestStandardSkillRequirementsAreEmpty`, `// invariant: rendering/catalog-and-targets:requires-skills-exact`.
5. **update `tooling/effort-management:effort-record-authority`**

   The awf binary alone allocates lowercase UUIDv4 effort IDs and owns schema-1 repository-local effort records and optional memory and managed-worktree state. Records contain no Pi-session assignment; creation, rename, lifecycle, and repair retain their existing resident-state authority without replacing Git-tracked project truth.

   Origin: ADR-0164
   Revised-by: ADR-0167
   Backing: test
   Proof: `cmd/awf/effort_test.go:TestEffortCommandAcceptsInstalledMemoryDirectory`, `// invariant: tooling/effort-management:effort-record-authority`.
6. **remove `tooling/effort-management:session-effort-assignment`**

   Delete the entire claim block and every `// invariant: tooling/effort-management:session-effort-assignment` marker.
7. **remove `tooling/workflow-telemetry:event-protocol-and-ledger`**

   Delete the entire claim block and every `// invariant: tooling/workflow-telemetry:event-protocol-and-ledger` marker.
8. **remove `tooling/workflow-telemetry:privacy-integrity-and-retention`**

   Delete the entire claim block and every `// invariant: tooling/workflow-telemetry:privacy-integrity-and-retention` marker.
9. **remove `tooling/workflow-telemetry:canonical-projections-and-diagnostics`**

   Delete the entire claim block and every `// invariant: tooling/workflow-telemetry:canonical-projections-and-diagnostics` marker.
10. **update `tooling/cli:effort-command-contract`**

    `awf effort` owns creation, memory, rename, terminal state, repair, and managed-worktree attachment, integration, and removal. It has no assignment command or Pi-session concept; creation defaults to memory, worktrees are opt-in, recoverable worktree risks require paired force and reason, and JSON replies carry schema version 1.

    Origin: ADR-0164
    Revised-by: ADR-0167
    Backing: test
    Proof: `cmd/awf/effort_test.go:TestEffortCommandContractProof`, `// invariant: tooling/cli:effort-command-contract`.
11. **remove `tooling/cli:metrics-command-contract`**

    Delete the entire claim block and every `// invariant: tooling/cli:metrics-command-contract` marker.
12. **update `rendering/singletons-and-payloads:memory-gitignore-always-on`**

    Every render declares a self-ignoring `.gitignore` for exactly the three repository-wide resident roots: efforts, memory, and worktrees. Only each root ignore file is governed; dynamic descendants are preserved.

    Origin: ADR-0148
    Revised-by: ADR-0159, ADR-0164, ADR-0167
    Backing: test
    Proof: `internal/project/memory_test.go:TestMemoryGitignoreAlwaysOn`, `// invariant: rendering/singletons-and-payloads:memory-gitignore-always-on`.
13. **remove `rendering/singletons-and-payloads:workflow-telemetry-governed-outputs-and-resident-data`**

    Delete the entire claim block and every `// invariant: rendering/singletons-and-payloads:workflow-telemetry-governed-outputs-and-resident-data` marker.
14. **add `rendering/singletons-and-payloads:resident-output-preservation`**

    The output plan preserves exactly the effort, memory, and managed-worktree resident roots and their dynamic descendants at the primary control root. Generation 21 destructively removes only the obsolete metrics and assignments roots.

    Origin: ADR-0167
    Backing: test
    Proof: `internal/effort/paths_test.go:TestEffortPathsClosedResidentRoots`, `// invariant: rendering/singletons-and-payloads:resident-output-preservation`.
15. **update `rendering/project-output-plan:output-plan-complete`**

    The deterministic output plan contains catalog and local artifacts, bridge files, generated documentation, reservations, and exactly three resident-root self-ignoring outputs: efforts, memory, and worktrees. Resident dynamic descendants are not plan nodes and resolve at the primary root while tracked authority remains invoking-checkout authority.

    Origin: ADR-0124
    Revised-by: ADR-0164, ADR-0167
    Backing: test
    Proof: `internal/project/output_plan_test.go:TestOutputPlanContainsWritesGeneratedNodesAndReservations`, `// invariant: rendering/project-output-plan:output-plan-complete`.
16. **update `rendering/guide-and-doc-templates:guide-entry-point-routing`**

    The rendered guide lists every enabled standard and local skill in declaration order with kind, purpose, a distinct trigger sentence, and optional advisory neighbors. Missing values render coherent generic prose, and the guide says that any enabled skill may be used when its purpose fits rather than routing or requiring transitions.

    Origin: ADR-0157
    Revised-by: ADR-0167
    Backing: test
    Proof: `internal/project/guide_scopes_test.go:TestGuideCatalogRowsAreCompleteSafeAndAdvisory`, `// invariant: rendering/guide-and-doc-templates:guide-entry-point-routing`.
17. **update `rendering/guide-and-doc-templates:working-memory-single-home`**

    Working-memory guidance has one canonical workflow-doc home. Efforts and memory are optional for durable coordination, memory, or managed worktrees; direct effort commands remain available without Pi selection or session assignment, and durable records never cite a particular memory file.

    Origin: ADR-0157
    Revised-by: ADR-0160, ADR-0161, ADR-0164, ADR-0167
    Backing: test
    Proof: `internal/project/render_test.go:TestWorkingMemoryHasOneHome`, `// invariant: rendering/guide-and-doc-templates:working-memory-single-home`.
18. **update `rendering/workflow-skill-templates:mandatory-approval-boundaries`**

    The rendered brainstorming and ADR-review skills close with the mandatory approval protocol: persist memory, present the completed summary, explicitly request approval, and stop. Continuation and handoff begin only after explicit approval is persisted; no other chain skill renders an approval stop.

    Origin: ADR-0152
    Revised-by: ADR-0160, ADR-0167
    Backing: test
    Proof: `internal/project/render_test.go:TestWorkflowSkillTemplatesKeepApprovalBoundaries`, `// invariant: rendering/workflow-skill-templates:mandatory-approval-boundaries`.
19. **update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`**

    Checkpoint guidance treats memory as optional local effort state and recommends outcome-specific `awf effort` creation when durable coordination, memory, or worktrees warrant it. It contains no selection, assignment, adoption, detour, or telemetry-lifecycle gate.

    Origin: ADR-0148
    Revised-by: ADR-0149, ADR-0152, ADR-0160, ADR-0164, ADR-0167
    Backing: test
    Proof: `internal/project/render_test.go:TestWorkflowSkillTemplatesRetainMemoryCheckpoints`, `// invariant: rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`.
20. **add `rendering/workflow-skill-templates:workflow-transitions-advisory`**

    Rendered workflow skills describe catalog relationships only as recommendations. Any enabled skill may be used when its purpose fits, while controls within a selected skill remain mandatory.

    Origin: ADR-0167
    Backing: test
    Proof: `internal/project/guide_scopes_test.go:TestGuideCatalogRowsAreCompleteSafeAndAdvisory`, `// invariant: rendering/workflow-skill-templates:workflow-transitions-advisory`.
21. **remove `rendering/adapter-outputs:pi-workflow-telemetry-runtime`**

    Delete the entire claim block and every `// invariant: rendering/adapter-outputs:pi-workflow-telemetry-runtime` marker.
22. **update `rendering/pi-workflows:pi-session-handoff-lifecycle`**

    Pi handoff retains its single-use queue, countdown, cancellation, parent link, confined optional regular-file memory validation, kickoff, child cleanup, and editor fallback. It neither selects nor assigns an effort, invokes awf, parses an `Effort:` header, nor appends selection state.

    Origin: ADR-0148
    Revised-by: ADR-0149, ADR-0152, ADR-0164, ADR-0167
    Backing: test
    Proof: `internal/project/target_test.go:TestHandoffLifecycleWithoutEffort`, `// invariant: rendering/pi-workflows:pi-session-handoff-lifecycle`.
23. **update `rendering/pi-workflows:pi-session-handoff-public-contract`**

    Pi handoff accepts an optional confined regular-file memoryPath and bounded kickoff; absent memory is valid. It never selects or assigns an effort, invokes awf, adopts checkpoints, or fabricates history.

    Origin: ADR-0148
    Revised-by: ADR-0149, ADR-0162, ADR-0164, ADR-0167
    Backing: test
    Proof: `internal/project/target_test.go:TestHandoffPublicContractWithoutEffort`, `// invariant: rendering/pi-workflows:pi-session-handoff-public-contract`.
24. **update `rendering/pi-workflows:pi-session-handoff-workflow`**

    Pi checkpoint guidance permits effort-independent handoff after normal persistence, with optional confined memory, and never requires selection, telemetry lifecycle state, adoption, or structured resume.

    Origin: ADR-0148
    Revised-by: ADR-0149, ADR-0152, ADR-0164, ADR-0167
    Backing: test
    Proof: `internal/project/target_test.go:TestHandoffWorkflowWithoutEffort`, `// invariant: rendering/pi-workflows:pi-session-handoff-workflow`.
25. **remove `rendering/pi-workflows:pi-lifecycle-enforcing-workflow-router`**

    Delete the entire claim block and every `// invariant: rendering/pi-workflows:pi-lifecycle-enforcing-workflow-router` marker.
26. **add `rendering/pi-workflows:pi-native-workflow-skills`**

    Pi renders every enabled standard and local skill at `.pi/skills/<prefix>-<name>/SKILL.md`; disabled skills or Pi disablement prune those paths, and no router or hidden workflow-body output remains.

    Origin: ADR-0167
    Backing: test
    Proof: `internal/project/target_test.go:TestNativePiSkillsAreDiscoverableAndPruned`, `// invariant: rendering/pi-workflows:pi-native-workflow-skills`.
27. **remove `rendering/pi-workflows:pi-workflow-telemetry-public-contract`**

    Delete the entire claim block and every `// invariant: rendering/pi-workflows:pi-workflow-telemetry-public-contract` marker.
28. **update `rendering/pi-runtime:pi-extension-target-render`**

    Enabling Pi renders exactly the two subagent extension files and handoff extension with provenance. No telemetry extension or protocol output renders, and all remaining files follow normal render and cleanup semantics.

    Origin: ADR-0148
    Revised-by: ADR-0162, ADR-0164, ADR-0167
    Backing: test
    Proof: `internal/project/target_test.go:TestPiRuntimeTargetRender`, `// invariant: rendering/pi-runtime:pi-extension-target-render`.
29. **update `rendering/pi-runtime:pi-minimum-runtime`**

    Generated Pi extension entrypoints require the minimum Pi runtime APIs used by the retained subagent and handoff contracts, report one actionable incompatibility notice, and fail before registering functional hooks when required APIs are absent.

    Origin: ADR-0148
    Revised-by: ADR-0162, ADR-0167
    Backing: test
    Proof: `internal/project/target_test.go:TestPiMinimumRuntime`, `// invariant: rendering/pi-runtime:pi-minimum-runtime`.
30. **update `rendering/pi-runtime:pi-real-runtime-smoke`**

    Pinned Pi runtime smoke covers generated TypeScript loading, native Pi skill discovery, and effort-independent handoff, and verifies telemetry, router, and selection surfaces are absent.

    Origin: ADR-0148
    Revised-by: ADR-0149, ADR-0161, ADR-0162, ADR-0164, ADR-0167
    Backing: unbacked
    Verify: Run `./x pi-test run` to exercise native Pi skill discovery and effort-independent handoff, with no telemetry, router, or selection.

## File structure

The implementation modifies these existing Go sources/tests, creates the two migration files, and deletes exactly the listed product files. This is the exhaustive authored-code inventory; no `as applicable` expansion is permitted.

- **Create:** `internal/migrate/remove_workflow_residents.go`, `internal/migrate/remove_workflow_residents_test.go`.
- **Modify:** `internal/catalog/catalog.go`, `internal/catalog/catalog_test.go`, `internal/catalog/graph.go`, `internal/catalog/graph_test.go`, `internal/catalog/standard.go`, `internal/catalog/workflow.go`, `internal/catalog/workflow_test.go`; `internal/project/banner.go`, `internal/project/banner_test.go`, `internal/project/context.go`, `internal/project/local.go`, `internal/project/local_test.go`, `internal/project/project.go`, `internal/project/project_test.go`, `internal/project/render.go`, `internal/project/render_test.go`, `internal/project/target.go`, `internal/project/target_test.go`, `internal/project/confighash.go`, `internal/project/confighash_test.go`, `internal/project/output_plan.go`, `internal/project/output_plan_test.go`, `internal/project/install.go`, `internal/project/install_test.go`, `internal/project/currentstate.go`, `internal/project/currentstate_test.go`, `internal/project/sweep.go`, `internal/project/sweep_test.go`, `internal/project/guide_scopes_test.go`, `internal/project/drift_test.go`, `internal/project/version_test.go`, `internal/project/plan_detail_modes_test.go`, `internal/project/spine_test.go`, `internal/project/scaffold_test.go`, `internal/project/memory_test.go`, and `internal/project/staged_test.go`; `internal/worktree/topology_parity_test.go`; `internal/effort/branches_test.go`, `internal/effort/memory_test.go`, `internal/effort/paths.go`, `internal/effort/paths_test.go`, `internal/effort/service.go`, `internal/effort/service_test.go`, `internal/effort/store.go`, `internal/effort/store_test.go`, `internal/effort/types.go`, `internal/effort/types_test.go`, and `internal/effort/safety_test.go`; `internal/migrate/migrate.go`, `internal/migrate/migrate_test.go`; `internal/git/controlroot.go`, `internal/git/controlroot_test.go`; `internal/clispec/clispec.go`, `internal/clispec/clispec_test.go`; `internal/snapshot/working_test.go`; `internal/evals/chain_test.go`; `cmd/awf/dispatch.go`, `cmd/awf/effort.go`, `cmd/awf/effort_test.go`, `cmd/awf/main.go`, `cmd/awf/main_test.go`, `cmd/awf/uninstall.go`, `cmd/awf/uninstall_test.go`; `templates/embed.go`; `tools/pi-extension-test/tests/handoff.test.ts`, `tools/pi-extension-test/tests/runner.test.ts`, `tools/pi-extension-test/package.json`, `tools/pi-extension-test/package-lock.json`, and `tools/pi-extension-test/tsconfig.json`.
- **Delete:** `internal/telemetry/aggregate.go`, `aggregate_test.go`, `export.go`, `export_render_select_test.go`, `paths.go`, `paths_test.go`, `protocol.go`, `protocol.json`, `protocol_test.go`, `protocol_typescript.go`, `reader.go`, `reader_test.go`, `render.go`, `select.go`, `types.go`; `internal/effort/assignment.go`, `internal/effort/assignment_test.go`; `cmd/awf/metrics.go`, `cmd/awf/metrics_session_test.go`; `templates/pi/awf-telemetry/index.ts.tmpl`, `templates/pi/awf-telemetry/protocol.ts.tmpl`, `templates/pi/awf-workflow/SKILL.md.tmpl`; `tools/pi-extension-test/tests/protocol.test.ts`, `session-v1.test.ts`, `telemetry-registration.test.ts`, `telemetry-writer.test.ts`; `tools/pi-extension-test/fixtures/fake-awf.mjs`; `.awf/topics/metadata/tooling/workflow-telemetry.yaml`; `.awf/topics/parts/tooling/workflow-telemetry/current-state.md`.
- **Modify every skill template:** `templates/skills/adr-lifecycle/SKILL.md.tmpl`, `_base/SKILL.md.tmpl`, `brainstorming/SKILL.md.tmpl`, `bugfix/SKILL.md.tmpl`, `debugging/SKILL.md.tmpl`, `executing-direct/SKILL.md.tmpl`, `executing-plans/SKILL.md.tmpl`, `exploring/SKILL.md.tmpl`, `proposing-adr/SKILL.md.tmpl`, `refactor-coupling-audit/SKILL.md.tmpl`, `retrospective/SKILL.md.tmpl`, `reviewing-adr/SKILL.md.tmpl`, `reviewing-impl/SKILL.md.tmpl`, `reviewing-plan/SKILL.md.tmpl`, `reviewing-plan-resync/SKILL.md.tmpl`, `roadmap-graduation/SKILL.md.tmpl`, `subagent-driven-development/SKILL.md.tmpl`, `tdd/SKILL.md.tmpl`, and `writing-plans/SKILL.md.tmpl`.
- **Modify non-skill templates:** `templates/agents-doc/AGENTS.md.tmpl`, `templates/docs/architecture.md.tmpl`, `templates/docs/releasing.md.tmpl`, `templates/docs/testing.md.tmpl`, `templates/docs/workflow.md.tmpl`, `templates/docs/working-with-awf.md.tmpl`, `templates/pi/awf-handoff/index.ts.tmpl`.
- **Modify authored awf sources:** `.awf/agents-doc.yaml`; `.awf/domains/tooling.yaml`; `.awf/domains/parts/config/current-state.md`, `.awf/domains/parts/rendering/current-state.md`, `.awf/domains/parts/tooling/current-state.md`; `.awf/parts/agents-doc/commands.md`, `.awf/parts/agents-doc/identity.md`, `.awf/parts/agents-doc/working-memory.md`; `.awf/parts/workflow/chain.md`, `.awf/parts/workflow/commit-discipline.md`, `.awf/parts/working-with-awf/commands.md`, `.awf/parts/working-with-awf/config-and-overrides.md`; `.awf/docs/parts/architecture/components.md`, `data-flow.md`, `dependencies.md`, `overview.md`; `.awf/docs/parts/releasing/content.md`; `.awf/docs/parts/testing/gate.md`, `layout.md`, `tiers.md`; `.awf/docs/glossary.yaml`, `.awf/docs/parts/glossary/prepend.md`, `.awf/docs/parts/pitfalls/prepend.md`, `.awf/docs/pitfalls.yaml`; `.awf/topics/parts/config/migrations-and-locks/current-state.md`; `.awf/topics/parts/rendering/catalog-and-targets/current-state.md`, `guide-and-doc-templates/current-state.md`, `adapter-outputs/current-state.md`, `pi-runtime/current-state.md`, `pi-workflows/current-state.md`, `project-output-plan/current-state.md`, `singletons-and-payloads/current-state.md`, `workflow-skill-templates/current-state.md`; `.awf/topics/parts/tooling/cli/current-state.md`, `effort-management/current-state.md`; `.awf/topics/metadata/rendering/pi-workflows.yaml`, `.awf/topics/metadata/rendering/singletons-and-payloads.yaml`, `.awf/topics/metadata/tooling/effort-management.yaml`; and `changelog/CHANGELOG.md`.

Historical ADRs and implemented plans are not edited. Generated root files are rendered, not authored. **Modify:** `.awf/awf.lock`, `AGENTS.md`, `docs/architecture.md`, `docs/config-reference.md`, `docs/glossary.md`, `docs/pitfalls.md`, `docs/releasing.md`, `docs/testing.md`, `docs/workflow.md`, `docs/working-with-awf.md`, `docs/domains/config.md`, `docs/domains/rendering.md`, `docs/domains/tooling.md`, `docs/topics/config/migrations-and-locks.md`, `docs/topics/rendering/adapter-outputs.md`, `docs/topics/rendering/catalog-and-targets.md`, `docs/topics/rendering/guide-and-doc-templates.md`, `docs/topics/rendering/index.md`, `docs/topics/rendering/pi-runtime.md`, `docs/topics/rendering/pi-workflows.md`, `docs/topics/rendering/project-output-plan.md`, `docs/topics/rendering/singletons-and-payloads.md`, `docs/topics/rendering/workflow-skill-templates.md`, `docs/topics/tooling/cli.md`, `docs/topics/tooling/effort-management.md`, `docs/topics/tooling/index.md`, `docs/decisions/INDEX.md`, `.claude/skills/{awf-adr-lifecycle,awf-brainstorming,awf-bugfix,awf-debugging,awf-executing-direct,awf-executing-plans,awf-exploring,awf-proposing-adr,awf-refactor-coupling-audit,awf-retrospective,awf-reviewing-adr,awf-reviewing-impl,awf-reviewing-plan,awf-reviewing-plan-resync,awf-subagent-driven-development,awf-tdd,awf-writing-plans}/SKILL.md`, and `.pi/extensions/awf-handoff/index.ts`. **Create:** `.pi/skills/{awf-adr-lifecycle,awf-brainstorming,awf-bugfix,awf-debugging,awf-executing-direct,awf-executing-plans,awf-exploring,awf-proposing-adr,awf-refactor-coupling-audit,awf-retrospective,awf-reviewing-adr,awf-reviewing-impl,awf-reviewing-plan,awf-reviewing-plan-resync,awf-subagent-driven-development,awf-tdd,awf-writing-plans}/SKILL.md`. **Delete:** `.awf/metrics/.gitignore`, `.awf/assignments/.gitignore`, `.pi/extensions/awf-telemetry/index.ts`, `.pi/extensions/awf-telemetry/protocol.ts`, `.pi/skills/awf-workflow/SKILL.md`, `.pi/awf-workflows/{adr-lifecycle,brainstorming,bugfix,debugging,executing-direct,executing-plans,exploring,proposing-adr,refactor-coupling-audit,retrospective,reviewing-adr,reviewing-impl,reviewing-plan,reviewing-plan-resync,subagent-driven-development,tdd,writing-plans}.md`, and `docs/topics/tooling/workflow-telemetry.md`. The tooling domain document and tooling topic index are explicit membership/index outputs changed by removing the telemetry metadata and domain membership.

Sundial has no authored change: its changed paths are generated by the shared cutover. **Modify:** `examples/sundial/.awf/awf.lock`, `examples/sundial/AGENTS.md`, `examples/sundial/docs/architecture.md`, `examples/sundial/docs/config-reference.md`, `examples/sundial/docs/glossary.md`, `examples/sundial/docs/testing.md`, `examples/sundial/docs/workflow.md`, `examples/sundial/docs/working-with-awf.md`, `examples/sundial/.claude/skills/{sundial-adr-lifecycle,sundial-brainstorming,sundial-bugfix,sundial-debugging,sundial-executing-direct,sundial-executing-plans,sundial-exploring,sundial-proposing-adr,sundial-refactor-coupling-audit,sundial-retrospective,sundial-reviewing-adr,sundial-reviewing-impl,sundial-reviewing-plan,sundial-reviewing-plan-resync,sundial-roadmap-graduation,sundial-subagent-driven-development,sundial-tdd,sundial-writing-plans}/SKILL.md`, `examples/sundial/.cursor/skills/{sundial-adr-lifecycle,sundial-brainstorming,sundial-bugfix,sundial-debugging,sundial-executing-direct,sundial-executing-plans,sundial-exploring,sundial-proposing-adr,sundial-refactor-coupling-audit,sundial-retrospective,sundial-reviewing-adr,sundial-reviewing-impl,sundial-reviewing-plan,sundial-reviewing-plan-resync,sundial-roadmap-graduation,sundial-subagent-driven-development,sundial-tdd,sundial-writing-plans}/SKILL.md`, `examples/sundial/.gemini/skills/{sundial-adr-lifecycle,sundial-brainstorming,sundial-bugfix,sundial-debugging,sundial-executing-direct,sundial-executing-plans,sundial-exploring,sundial-proposing-adr,sundial-refactor-coupling-audit,sundial-retrospective,sundial-reviewing-adr,sundial-reviewing-impl,sundial-reviewing-plan,sundial-reviewing-plan-resync,sundial-roadmap-graduation,sundial-subagent-driven-development,sundial-tdd,sundial-writing-plans}/SKILL.md`, and `examples/sundial/.pi/extensions/awf-handoff/index.ts`. **Create:** `examples/sundial/.pi/skills/{sundial-adr-lifecycle,sundial-brainstorming,sundial-bugfix,sundial-debugging,sundial-executing-direct,sundial-executing-plans,sundial-exploring,sundial-proposing-adr,sundial-refactor-coupling-audit,sundial-retrospective,sundial-reviewing-adr,sundial-reviewing-impl,sundial-reviewing-plan,sundial-reviewing-plan-resync,sundial-roadmap-graduation,sundial-subagent-driven-development,sundial-tdd,sundial-writing-plans}/SKILL.md`. **Delete:** `examples/sundial/.awf/metrics/.gitignore`, `examples/sundial/.awf/assignments/.gitignore`, `examples/sundial/.pi/extensions/awf-telemetry/index.ts`, `examples/sundial/.pi/extensions/awf-telemetry/protocol.ts`, `examples/sundial/.pi/skills/awf-workflow/SKILL.md`, and `examples/sundial/.pi/awf-workflows/{adr-lifecycle,brainstorming,bugfix,debugging,executing-direct,executing-plans,exploring,proposing-adr,refactor-coupling-audit,retrospective,reviewing-adr,reviewing-impl,reviewing-plan,reviewing-plan-resync,roadmap-graduation,subagent-driven-development,tdd,writing-plans}.md`.

## Phase 1: one atomic generation-21 cutover and implementation commit

Do every task below in order without an intermediate render, staged check, gate, status event, or commit. In particular, after adding generation 21, do **not** invoke `./awf render`, `./awf check`, `./x render`, or `./x check` from the source checkout until the source cutover is complete and both adopted trees have been upgraded by the cutover binary. The schema-20 root and Sundial trees must remain untouched until task 5.

- [ ] **1. Generation 21 and destructive migration.** Add `applyRemoveWorkflowResidents(root string, out io.Writer) error` in `internal/migrate/remove_workflow_residents.go`; register `{To: 21, Name: "remove-workflow-residents", Apply: applyRemoveWorkflowResidents}` after generation 20 in `internal/migrate/migrate.go`; update `ConfigForCurrentSchema` if its dispatch has an explicit current-generation arm. Set `project.Version` and `minVersionBySchema[21]` to `0.25.0` in `internal/project/project.go`.

  The private helper receives injected `lstat func(string) (fs.FileInfo, error)` and `removeAll func(string) error`; production passes `os.Lstat` and `os.RemoveAll`. Resolve the primary control root through the existing `internal/git` API, never the invoking linked-worktree directory. In literal order `metrics`, `assignments`, Lstat `<primary>/.awf/<name>`: missing prints `remove-workflow-residents: <name> already absent`; symlink or any non-directory returns an identifying error without calling removeAll; a directory calls removeAll and only then prints `remove-workflow-residents: <name> removed`. Stop at a remove failure, preserve the other root's actual state, and allow a later upgrade to retry and report current state. Do not follow a symlink, create a root, retain descendants or `.gitignore`, or touch efforts, memory, or worktrees.

  Add `TestRemoveWorkflowResidentsMigration` in the new test file with subtests `absent`, `nested-roots`, `ordered-output`, `unsafe-symlink`, `non-directory`, `partial-failure-then-retry`, and `linked-worktree-primary-root`. The injected second `removeAll` failure proves deterministic partial failure; retry proves no recreation and schema/lock stamping. Put `// invariant: config/migrations-and-locks:workflow-telemetry-config-migration` immediately above that test. Extend `TestGateBlocksWhenBehind`, `TestUpgradePropagatesMigrationError`, `TestSyncStampsSchemaVersion`, and `TestEffortPathsClosedResidentRoots` for generation/floor refusal and retained roots. The test also proves the migration itself touches only metrics and assignments.

- [ ] **2. Catalog refactor before all remaining ADR operations.** Replace the split fields with the exact `WorkflowProfile` contract above in `internal/catalog/catalog.go`, replace router-specific validation/projection in `internal/catalog/workflow.go`, and update `internal/catalog/standard.go` to the table verbatim. Remove `TestTaskSkillTriggers`, `TestValidateWorkflowMappingsUsesClosedFixedBodyKinds`, `TestWorkflowMappingsForEnabledSkillsRejectsInvalidInputs`, and `TestExploringRequirementsAreOneWay`; replace them with `TestWorkflowProfilesAreCompleteAndAdvisory`, `TestWorkflowProfileRejectsUnknownSelfAndDuplicateNeighbors`, and `TestStandardSkillRequirementsAreEmpty`. The latter carries `// invariant: rendering/catalog-and-targets:requires-skills-exact` immediately above its function and proves all standard skills are empty while the reviewing agents retain their structural `RequiresSkills`.

  Update `internal/project/local.go` and add `TestLocalSkillProfileUsesTrimmedDescriptionOrFallback` in `internal/project/local_test.go` for valid, non-string, empty, and whitespace description values. Update profile consumers in `internal/project/project.go`, `internal/project/render.go`, `internal/project/target.go`, `internal/project/confighash.go`, `internal/project/drift_test.go`, and `internal/evals/chain_test.go`. Put `// invariant: rendering/catalog-and-targets:enabled-set-closed` immediately above `TestOpenRefusesUnclosedEnabledSet`, narrowed to structural artifact requirements only; remove the exploration-closure proof marker completely. Advisory neighbors must validate without closing enablement.

- [ ] **3. Delete telemetry, assignment, router, and selected-effort product behavior.** Delete the exact files listed in File structure. Remove metrics dispatch/grammar/help, effort `assign`, `unassign`, and `assignments`, assignment fields and joins, telemetry output producers/protocol attribution, resident metrics/assignments exceptions, telemetry/assignment sweep and uninstall behavior, and every router/hidden-body symbol (`routesWorkflows`, `HiddenWorkflowPath`, `WorkflowRouterPath`, `workflowRouterData`, and routed-skill splitting). Pi uses `Target.SkillPath` for every enabled standard and local skill, with normal pruning when Pi or a skill is disabled.

  In `templates/pi/awf-handoff/index.ts.tmpl`, retain preflight, queue, countdown/cancel, parent link, confined regular-file optional memory validation, kickoff, cleanup, and editor fallback. Delete `validateMemoryEffort`, selection request/restoration, `runAwf`, child assignment, custom selection entries, and all `Effort:` parsing/comparison. `buildKickoffWrapper` says only to read the optional memory path before the immediate action. In `handoff.test.ts`, add `testHandoffDoesNotInvokeAwfOrSelectEffort`, plus confined regular-file, absent-memory, symlink, directory, traversal, and absolute-path cases. Retain `runner.test.ts`; delete all four telemetry protocol/session/registration/writer tests named above, delete `tools/pi-extension-test/fixtures/fake-awf.mjs`, and remove their fixtures/imports. Update `internal/project/staged_test.go` and `internal/worktree/topology_parity_test.go` for the removed outputs and the resulting root/worktree parity.

  Add `TestNativePiSkillsAreDiscoverableAndPruned` to `internal/project/target_test.go`, with `// invariant: rendering/pi-workflows:pi-native-workflow-skills` immediately above it. It proves normal Pi paths, local paths, disabled-skill/Pi cleanup, no router/hidden output, and target parity. Put `// invariant: rendering/singletons-and-payloads:resident-output-preservation` immediately above `TestEffortPathsClosedResidentRoots`; it proves exactly efforts/memory/worktrees remain and calls the migration fixture for the two removed roots. Replace/remove stale telemetry proof comments rather than leaving an orphan.

- [ ] **4. Advisory prose, claims, and documentation cutover.** Apply the full-guide grammar above in `templates/agents-doc/AGENTS.md.tmpl`; provide zero-value-safe template defaults for every interpolated field. Rewrite every template and authored part named in File structure, including every one of the 19 skill templates. Preserve mandatory local procedure controls (approval check-ins, staged check, `./x gate`, and scope-specific rules), but replace workflow routing language. Representative required transformations are:

  ```text
  Before: "In Pi, enter every governed skill through the awf_workflow router with the semantic skill name."
  After:  "In Pi, use any enabled native skill when its purpose fits the current work."

  Before: "The terminal step is mandatory: hand off only to reviewing-impl."
  After:  "A common follow-up is reviewing-impl when independent implementation review is useful."

  Before: "Non-trivial work starts with brainstorming ... then hands off through the chain."
  After:  "For non-trivial work, brainstorming is often useful; select any enabled skill whose purpose fits, and treat catalog relationships as advice."
  ```

  Update the exact current-state sources and topic metadata listed in File structure, including `.awf/topics/metadata/rendering/pi-workflows.yaml`, `.awf/topics/metadata/rendering/singletons-and-payloads.yaml`, and `.awf/topics/metadata/tooling/effort-management.yaml`; update `.awf/docs/parts/glossary/prepend.md` and `.awf/parts/agents-doc/commands.md`; delete the telemetry metadata and part, remove `internal/telemetry/**` from `.awf/domains/tooling.yaml`, and do not leave a telemetry topic shell. Update changelog Unreleased to state generation-21 destructive cleanup, native Pi skills, and removal of `/awf-effort`, `awf_workflow`, metrics, and assignment commands.

  Add these named deterministic test functions and put proof markers immediately above them: `TestGuideCatalogRowsAreCompleteSafeAndAdvisory` in `internal/project/guide_scopes_test.go` for `rendering/guide-and-doc-templates:guide-entry-point-routing` and `rendering/workflow-skill-templates:workflow-transitions-advisory`; `TestWorkingMemoryHasOneHome` in `internal/project/render_test.go` for `rendering/guide-and-doc-templates:working-memory-single-home`; `TestWorkflowSkillTemplatesKeepApprovalBoundaries` and `TestWorkflowSkillTemplatesRetainMemoryCheckpoints` in `internal/project/render_test.go` for the two workflow-template updates; `TestHandoffLifecycleWithoutEffort`, `TestHandoffPublicContractWithoutEffort`, and `TestHandoffWorkflowWithoutEffort` in `internal/project/target_test.go` for the three Pi-handoff claims; and `TestPiRuntimeTargetRender` and `TestPiMinimumRuntime` in `internal/project/target_test.go` for the two Backing:test Pi-runtime claims. Add `TestPiRealRuntimeSmoke` in `internal/project/target_test.go` for the unbacked smoke coverage, but add no invariant proof marker for it; its claim is verified only by the exact Verify line in the mutation contract. The guide test must assert exact rows for the table with their literal `Trigger: <trigger>.` sentence, the local trimmed-description and distinct-fallback purpose/trigger rows, omitted empty clauses, missing-key-safe generic output (including a missing trigger), advisory relationship text, and zero occurrences of `awf_workflow`, `only legal predecessor`, `only legal successor`, `mandatory successor`, `must follow`, and `must be followed by`.

  Run this exhaustive template prose post-check after the rewrites; each command must print no paths and exit zero:

  ```sh
  ! rg -l 'awf_workflow|mandatory successor|mandatory predecessor|only legal (predecessor|successor)|must (be )?follow(ed|ing)|only-entry|only entry' templates/skills/adr-lifecycle/SKILL.md.tmpl templates/skills/_base/SKILL.md.tmpl templates/skills/brainstorming/SKILL.md.tmpl templates/skills/bugfix/SKILL.md.tmpl templates/skills/debugging/SKILL.md.tmpl templates/skills/executing-direct/SKILL.md.tmpl templates/skills/executing-plans/SKILL.md.tmpl templates/skills/exploring/SKILL.md.tmpl templates/skills/proposing-adr/SKILL.md.tmpl templates/skills/refactor-coupling-audit/SKILL.md.tmpl templates/skills/retrospective/SKILL.md.tmpl templates/skills/reviewing-adr/SKILL.md.tmpl templates/skills/reviewing-impl/SKILL.md.tmpl templates/skills/reviewing-plan/SKILL.md.tmpl templates/skills/reviewing-plan-resync/SKILL.md.tmpl templates/skills/roadmap-graduation/SKILL.md.tmpl templates/skills/subagent-driven-development/SKILL.md.tmpl templates/skills/tdd/SKILL.md.tmpl templates/skills/writing-plans/SKILL.md.tmpl templates/agents-doc/AGENTS.md.tmpl templates/docs/architecture.md.tmpl templates/docs/releasing.md.tmpl templates/docs/testing.md.tmpl templates/docs/workflow.md.tmpl templates/docs/working-with-awf.md.tmpl templates/pi/awf-handoff/index.ts.tmpl
  ! rg -l 'awf_workflow|mandatory successor|mandatory predecessor|only legal (predecessor|successor)|must (be )?follow(ed|ing)|only-entry|only entry' .awf/agents-doc.yaml .awf/domains/tooling.yaml .awf/domains/parts/config/current-state.md .awf/domains/parts/rendering/current-state.md .awf/domains/parts/tooling/current-state.md .awf/parts/agents-doc/commands.md .awf/parts/agents-doc/identity.md .awf/parts/agents-doc/working-memory.md .awf/parts/workflow/chain.md .awf/parts/workflow/commit-discipline.md .awf/parts/working-with-awf/commands.md .awf/parts/working-with-awf/config-and-overrides.md .awf/docs/parts/architecture/components.md .awf/docs/parts/architecture/data-flow.md .awf/docs/parts/architecture/dependencies.md .awf/docs/parts/architecture/overview.md .awf/docs/parts/releasing/content.md .awf/docs/parts/testing/gate.md .awf/docs/parts/testing/layout.md .awf/docs/parts/testing/tiers.md .awf/docs/parts/glossary/prepend.md .awf/docs/pitfalls.yaml .awf/topics/metadata/rendering/pi-workflows.yaml .awf/topics/metadata/rendering/singletons-and-payloads.yaml .awf/topics/metadata/tooling/effort-management.yaml .awf/topics/parts/config/migrations-and-locks/current-state.md .awf/topics/parts/rendering/catalog-and-targets/current-state.md .awf/topics/parts/rendering/guide-and-doc-templates/current-state.md .awf/topics/parts/rendering/adapter-outputs/current-state.md .awf/topics/parts/rendering/pi-runtime/current-state.md .awf/topics/parts/rendering/pi-workflows/current-state.md .awf/topics/parts/rendering/project-output-plan/current-state.md .awf/topics/parts/rendering/singletons-and-payloads/current-state.md .awf/topics/parts/rendering/workflow-skill-templates/current-state.md .awf/topics/parts/tooling/cli/current-state.md .awf/topics/parts/tooling/effort-management/current-state.md
  ```

- [ ] **5. One direct Accepted -> Implemented lifecycle transaction, adopted-tree upgrade, render, stage, and commit.** Mutate all ADR-0167 claims together, in its declared order, beginning with operation 1 and ending with operation 30. Do not append `Implementing` or `Applied` events. Change ADR-0167 frontmatter directly from `Accepted` to `Implemented`; append exactly one `Implemented` status-history event with the checker-derived frozen content SHA-256 digest and the next checker-derived state sequence. Change this plan frontmatter to `Implemented` in the same transaction. Do not alter the ADR body or prior Accepted event.

  First complete formatting, source, template, claim, and status work. Build the cutover binary only after that work is complete, upgrade root and Sundial with it (mandatory sync/render), then run an explicit `./x render`. Only after that render may any Go or Pi test run, so no Pi test can observe stale generated telemetry or handoff output. This is the first source-built gated operation after generation registration:

  ```sh
  gofmt -w internal/catalog/catalog.go internal/catalog/catalog_test.go internal/catalog/graph.go internal/catalog/graph_test.go internal/catalog/standard.go internal/catalog/workflow.go internal/catalog/workflow_test.go internal/project/local.go internal/project/local_test.go internal/project/project.go internal/project/project_test.go internal/project/render.go internal/project/render_test.go internal/project/target.go internal/project/target_test.go internal/project/confighash.go internal/project/confighash_test.go internal/project/output_plan.go internal/project/output_plan_test.go internal/project/install.go internal/project/install_test.go internal/project/currentstate.go internal/project/currentstate_test.go internal/project/sweep.go internal/project/sweep_test.go internal/project/guide_scopes_test.go internal/project/drift_test.go internal/project/version_test.go internal/project/banner.go internal/project/banner_test.go internal/project/context.go internal/project/plan_detail_modes_test.go internal/project/spine_test.go internal/project/scaffold_test.go internal/project/memory_test.go internal/project/staged_test.go internal/worktree/topology_parity_test.go internal/effort/branches_test.go internal/effort/memory_test.go internal/effort/paths.go internal/effort/paths_test.go internal/effort/service.go internal/effort/service_test.go internal/effort/store.go internal/effort/store_test.go internal/effort/types.go internal/effort/types_test.go internal/effort/safety_test.go internal/migrate/migrate.go internal/migrate/migrate_test.go internal/migrate/remove_workflow_residents.go internal/migrate/remove_workflow_residents_test.go internal/git/controlroot.go internal/git/controlroot_test.go internal/clispec/clispec.go internal/clispec/clispec_test.go internal/snapshot/working_test.go internal/evals/chain_test.go cmd/awf/dispatch.go cmd/awf/effort.go cmd/awf/effort_test.go cmd/awf/main.go cmd/awf/main_test.go cmd/awf/uninstall.go cmd/awf/uninstall_test.go
  go build -o /tmp/awf-schema-21 ./cmd/awf
  /tmp/awf-schema-21 upgrade
  (cd examples/sundial && /tmp/awf-schema-21 upgrade)
  ./x render
  go test ./...
  ./x pi-test run
  test ! -e .awf/metrics && test ! -e .awf/assignments
  test ! -e .pi/extensions/awf-telemetry && test ! -e .pi/awf-workflows && test ! -e .pi/skills/awf-workflow
  test ! -e examples/sundial/.awf/metrics && test ! -e examples/sundial/.awf/assignments
  test ! -e examples/sundial/.pi/extensions/awf-telemetry && test ! -e examples/sundial/.pi/awf-workflows && test ! -e examples/sundial/.pi/skills/awf-workflow
  ./awf check
  git diff --check
  ```

  The two upgrades must print metrics before assignments, each as removed or already absent. The cutover render must not recreate either root. `./awf check` and `git diff --check` must exit zero before staging.

  Inspect `git status --short`. Refuse to continue for any path outside the File structure, ADR-0167, this plan, `docs/decisions/INDEX.md`, and the generated root/Sundial paths explicitly enumerated there. Stage modifications with explicit `git add` path arguments from that inventory and each deletion with explicit `git rm` path arguments. In particular, stage `.awf/docs/parts/glossary/prepend.md`, `.awf/parts/agents-doc/commands.md`, `.awf/topics/metadata/rendering/pi-workflows.yaml`, `.awf/topics/metadata/rendering/singletons-and-payloads.yaml`, `.awf/topics/metadata/tooling/effort-management.yaml`, `internal/project/staged_test.go`, `internal/worktree/topology_parity_test.go`, and generated `examples/sundial/docs/glossary.md` with `git add`, and delete `tools/pi-extension-test/fixtures/fake-awf.mjs` with `git rm`. List every generated changed file from `git status --short` as an individual argument after confirming it is in the enumerated generated set. Do not use `git add -A`, `git add .`, a directory argument, a glob, or a catch-all generated-path rule. Run `git status --short` again and refuse any unstaged or unexpected tracked path.

  Finally run exactly once, against the complete transaction:

  ```sh
  ./awf check --staged
  ./x gate
  ```

  Require `./awf check --staged` to exit zero and print `awf check --staged: clean`. The ADR parser/check establishes the direct Accepted -> Implemented transaction validity, declared operation order, active-topic removal, proof-marker ownership, and unresolved-value checks. Commit exactly:

  ```commit
  feat(rendering): remove telemetry and Pi router (implements 0167)
  ```

## Verification

At the final commit, after the cutover binary upgrades root and Sundial and explicit `./x render` refreshes generated output, `go test ./...`, `./x pi-test run`, `./x check`, and `./x gate` exit zero. `awf metrics`, `awf effort assign`, `awf effort unassign`, and `awf effort assignments` fail CLI grammar. A generation-20 tree upgrades to schema 21, removes only the two obsolete resident roots in order, refuses symlinks/non-directories, and retries after partial failure. Root and Sundial have native `.pi/skills/<prefix>-<name>/SKILL.md` files for enabled skills and have no telemetry extension, router skill, hidden body, metrics root, or assignments root. The guide and every skill describe purpose, the distinct `Trigger: <trigger>.` sentence, kind, and advisory neighbors without mandatory transition language; standard rows match their literal catalog purpose/trigger and local rows use the trimmed description or the distinct safe fallbacks.

## Notes

Implementation findings discovered during the atomic cutover must be recorded here before plan status freezes. The one-commit shape is forced by schema gating and declaration order, not convenience.
