---
format: plan-v2
date: 2026-08-03
adrs: [simplify-pi-effort-association-around-fixed-repository-paths]
status: Proposed
---
# Plan: Simplify Pi effort association around fixed repository paths

## Goal

Implement the linked ADR by replacing checkout-aware activity with protocol v2, making Pi effort association direct and CWD-neutral, supplying fixed relative effort paths in transient context, retaining advisory Remote Pi naming and metadata, and updating every behavior-facing artifact. Do not infer effort identity, validate unsupported checkout topology, preserve a protocol-v1 execution path, add local TUI presentation, or apply current-state claims before terminal implementation review settles.

## Architecture summary

`internal/effort` remains the safe-resident and activity-policy owner; `cmd/awf` exposes only the three JSON protocol operations. The generated client is the strict protocol-v2 transport boundary, while the generated Pi index owns one serialized process-local association, fixed-root directory inspection, transient context injection, heartbeat and shutdown cleanup, and optional Remote Pi translation. Authoring inputs and their rendered outputs land with the behavior they describe. Two independently green implementation-with-docs transactions precede terminal implementation review; only after that review settles does one inline direct transaction apply all eight current-state updates and co-freeze the ADR and this plan.

## Phase 1: Replace checkout-aware activity with protocol v2

**Execution mode: subagent-driven.**

Advances: ["rendered-doc-currency", "repository-green"]
Completes: ["protocol-v2"]

### Task 1.1: Test and implement the protocol-v2 activity model and recovery boundary
Latitude: exact
Applying: ["simplify-pi-effort-association-around-fixed-repository-paths:activity-protocol-v2", "simplify-pi-effort-association-around-fixed-repository-paths:recovery-attachment", "simplify-pi-effort-association-around-fixed-repository-paths:fixed-layout-dependency-boundary", "simplify-pi-effort-association-around-fixed-repository-paths:compatibility-and-verification"]
Paths: ["internal/effort/activity.go", "internal/effort/activity_test.go", "internal/effort/service.go", "internal/effort/service_test.go", "internal/effort/wiring_test.go", "internal/effort/git_context_test.go", "internal/effort/safety_test.go", "internal/effort/durability_test.go", "internal/effort/repair_test.go", "internal/effort/store_test.go", "internal/effort/types.go", "internal/effort/types_test.go"]

Start only from this clean/green baseline: the Proposed ADR and this plan are committed, `git status --short` prints nothing, and `./x check` plus `./x gate` pass. The phase owner changes no file under `.awf/topics/parts/` and does not change either lifecycle status.

Write failing table-driven tests before production changes, then replace `activitySchemaVersion`, `Activity`, `ActivityReply`, `ActionableOutcome`, activity conditions, operations, validation, and `Service` activity methods with the exact protocol-v2 contract from the ADR. A resident activity fact has exactly `schemaVersion: 2`, lowercase UUIDv4 owner, nonzero UTC `attachedAt`, and nonzero UTC `heartbeatAt`; remove checkout role, destination, CWD, receiving-checkout, prior-claim, `ready`, `checkout-updated`, `repository-mismatch`, and `changedCwd` concepts rather than retaining inert compatibility fields. Successful `attached`, `taken-over`, and `heartbeat` replies contain exactly schema version, condition, effort, fully validated memory, and activity; `detached` contains only schema version and condition. Each `not-owner`, `missing`, `invalid-memory`, or `unsafe-resident` reply contains exactly schema version, condition, and one outcome. The outcome contains category `operation`, bounded present-tense observed `condition`, the real `changedActivity` axis, ordered independently executable `nextActions`, and `cause` exactly when a mechanism call failed. Keep malformed invocation and failures that observe no managed state on the nonzero, empty-stdout, bounded-stderr path.

Under the existing per-effort activity lock, make explicit attach read only the bounded no-follow regular-file identity needed for conditional replacement. Absence creates v2 and reports `attached`; any safe existing file, including valid v1, malformed JSON, or another safe version, is replaced without decoding and reports `taken-over`. Symlink, non-regular, oversized, permission/storage, and identity-race cases refuse as `unsafe-resident`. Heartbeat and detach strictly decode a valid owned v2 resident; heartbeat updates only `heartbeatAt`; detach removes only the matching identity. Prove an old owner cannot update or remove a successor. Preserve optional advisory behavior: show, list, memory, worktree, integrate, finish, render, check, and unrelated commands never decode or gate on activity, and finish consumes it only within already-proven tombstone deletion.

Delete `CheckoutRole`, `CheckoutFacts`, `CheckoutResolutionKind`, `CheckoutResolutionError`, destination resolution, checkout operations, `Dependencies.ResolveCheckout`, `Service.checkoutResolver`, and their composition tests. Keep safe resident publication and fault seams cohesive in `internal/effort`; do not move activity writes into command or TypeScript code. Tests must exhaust exact JSON presence, unknown/trailing fields, v1 recovery replacement, malformed safe bytes, owner mismatch, missing activity and effort, invalid memory, symlink/non-regular/oversized residents, injected storage failures before and after possible publication, conditional-identity races, and unchanged unrelated-command behavior. Preserve every still-valid invariant proof marker and rename its proving test only if the marker's unit name changes with it.

### Task 1.2: Narrow the command grammar and JSON transport to attach, heartbeat, and detach
Latitude: exact
Applying: ["simplify-pi-effort-association-around-fixed-repository-paths:activity-protocol-v2", "simplify-pi-effort-association-around-fixed-repository-paths:fixed-layout-dependency-boundary", "simplify-pi-effort-association-around-fixed-repository-paths:compatibility-and-verification"]
Paths: ["cmd/awf/effort.go", "cmd/awf/effort_test.go", "cmd/awf/gate_test.go", "cmd/awf/effort_worktree_test.go", "internal/clispec/clispec.go", "internal/clispec/clispec_test.go"]

Change `openEffortComposition`, `runEffort`, `runEffortActivity`, `validateEffortActivityGrammar`, `activityRequiredFlags`, and the clispec `effort activity` subtree so the only accepted forms are exactly:

```text
awf effort activity attach <slug> --owner <uuid> --json
awf effort activity heartbeat <slug> --owner <uuid> --json
awf effort activity detach <slug> --owner <uuid> --json
```

Every operation requires one slug, `--owner`, and `--json`; no positional or flag spelling for resolve, checkout, destination, CWD, role, or receiving checkout remains accepted or advertised. Dispatch directly to the three service methods and keep newline-terminated JSON for handled replies. Remove the command-layer Git-to-effort checkout translation seam while retaining ordinary worktree composition. Extend `TestEffortMemoryAndActivityCLIContract`, clispec/help tests, invalid-grammar cases, and the gate probe so exact v2 argv and envelopes pass, every removed action/flag fails through usage with empty stdout, and malformed/pre-state failures preserve bounded stderr. Re-run focused Go tests with `go test ./internal/effort ./cmd/awf ./internal/clispec`; expect success.

### Task 1.3: Publish protocol-v2 documentation and rendered outputs with the behavior
Kind: batch
Latitude: exact
Applying: ["simplify-pi-effort-association-around-fixed-repository-paths:activity-protocol-v2", "simplify-pi-effort-association-around-fixed-repository-paths:recovery-attachment", "simplify-pi-effort-association-around-fixed-repository-paths:fixed-layout-dependency-boundary", "simplify-pi-effort-association-around-fixed-repository-paths:compatibility-and-verification"]
Paths: ["README.md", ".awf/docs/parts/architecture/overview.md", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", ".awf/docs/parts/architecture/dependencies.md", ".awf/docs/parts/testing/gate.md", ".awf/docs/parts/testing/layout.md", ".awf/docs/parts/testing/tiers.md", ".awf/parts/working-with-awf/overview.md", ".awf/parts/working-with-awf/config-and-overrides.md", ".awf/parts/working-with-awf/commands.md", "templates/docs/architecture.md.tmpl", "templates/docs/testing.md.tmpl", "templates/docs/working-with-awf.md.tmpl", "changelog/CHANGELOG.md", "AGENTS.md", "docs/architecture.md", "docs/testing.md", "docs/working-with-awf.md"]
Representative: "Replace protocol-1 checkout-resolution descriptions with the three-operation protocol-v2 owner/timestamp model, recovery attach, exact refusal outcome ownership, and the rule that unrelated commands never gate on activity."
Edge: "Retain ordinary Git-authoritative managed-worktree operations, memory metadata behavior, and the statement that activity is advisory rather than authority or a lock; do not pre-state the not-yet-landed Pi runtime behavior beyond identifying its protocol boundary."
Post-check: "Run ./x render, then require git diff --check and ./x check to succeed; inspect git diff --name-only and stop for plan correction if a rendered change has no matching authoring input or belongs only to the deferred current-state transaction."

Update the adopter-facing changelog in this transaction with the protocol-v2 break, one-way safe v1 recovery, and removal of checkout/CWD fields and operations. Update behavior-stating documentation at its `.awf/` or template authoring home, then run `./x render` and include every deterministic generated consequence, including `AGENTS.md` when changed. Do not edit generated docs directly. Do not touch the eight claim sources, `docs/decisions/INDEX.md`, or lifecycle statuses.

### Phase close

Run `gofmt` on changed Go files. Run the focused Go commands from Tasks 1.1 and 1.2, then `./x render`, `./x check`, and `./x gate`; all must succeed. Stage the complete protocol, tests, authoring docs, changelog, and rendered consequences explicitly. Require `awf check staged` to report clean and create one commit.

```commit
feat(tooling): replace effort activity with protocol v2
```

## Phase 2: Make Pi association direct, transient, and presentation-silent

**Execution mode: subagent-driven.**

Advances: ["repository-green"]
Completes: ["direct-pi-association", "rendered-doc-currency"]

### Task 2.1: Strictly decode and invoke protocol v2 in the generated client
Latitude: exact
Applying: ["simplify-pi-effort-association-around-fixed-repository-paths:activity-protocol-v2", "simplify-pi-effort-association-around-fixed-repository-paths:direct-using-effort-tool", "simplify-pi-effort-association-around-fixed-repository-paths:fixed-layout-dependency-boundary", "simplify-pi-effort-association-around-fixed-repository-paths:compatibility-and-verification"]
Paths: ["templates/pi/awf-effort/client.ts.tmpl", ".pi/extensions/awf-effort/client.ts", "tools/pi-extension-test/tests/using-effort.test.ts"]

Start only from this clean/green baseline: Phase 1 is committed, `git status --short` prints nothing, `./x check` and `./x gate` pass, and `awf effort activity --help` advertises only attach, heartbeat, and detach. The phase owner changes no file under `.awf/topics/parts/` and leaves the ADR and plan Proposed.

Replace the protocol-v1 client types and `decodeReply` presence matrix with exact protocol-v2 immutable types. Accept only schema version 2, the six retained top-level conditions, exact condition-specific facts, exact v2 activity fields, and refusal outcomes with category `operation`, bounded observed-condition prose, `changedActivity`, a nonempty ordered bounded action list, and cause presence exactly for mechanism failure. Reject unknown keys, optionalized required fields, forbidden facts, invalid UUID/time/memory data, trailing JSON, oversized stdout/stderr, nonzero exit, and cancellation with bounded diagnostics. Remove role, destination, CWD, prior claim, changed-memory, and changed-CWD client concepts. `activity()` must construct only `./awf effort activity <attach|heartbeat|detach> <slug> --owner <uuid> --json`, preserve the injected executor seam and `AbortSignal`, and never read or write residents.

Extend the strict TypeScript suite before changing the template. Assert every accepted presence matrix row, every forbidden extra/missing field, exact argv, cancellation, output bounds, nonzero/invalid JSON behavior, and top-level-condition branching independence from observed-condition text. Render before running the tests so the generated client is the tested artifact.

### Task 2.2: Replace runtime transfer with one direct serialized association
Latitude: exact
Applying: ["simplify-pi-effort-association-around-fixed-repository-paths:associate-without-checkout-replacement", "simplify-pi-effort-association-around-fixed-repository-paths:direct-using-effort-tool", "simplify-pi-effort-association-around-fixed-repository-paths:fixed-relative-effort-context", "simplify-pi-effort-association-around-fixed-repository-paths:advisory-process-lifecycle", "simplify-pi-effort-association-around-fixed-repository-paths:remote-name-without-local-tui", "simplify-pi-effort-association-around-fixed-repository-paths:fixed-layout-dependency-boundary", "simplify-pi-effort-association-around-fixed-repository-paths:compatibility-and-verification"]
Paths: ["templates/pi/awf-effort/index.ts.tmpl", ".pi/extensions/awf-effort/index.ts", "tools/pi-extension-test/tests/using-effort.test.ts", "tools/pi-extension-test/tests/runtime.test.ts", "tools/pi-extension-test/fixtures/fake-pi.mjs", "tools/pi-extension-test/fixtures/term-resistant-pi.mjs", "tools/pi-extension-test/container.sh"]

Write failing runtime tests, then reduce the public `using_effort` schema to a closed object with optional `effort` and `detach` fields whose runtime validation accepts exactly `{effort:"<canonical-slug>"}` or `{detach:true}`. Validation errors throw before invoking the binary. Execute accepted operations directly inside the tool call through one process-local promise chain; tool results never terminate the turn. Repeat attach invokes attach again. A switch detaches the old owner first; a detach refusal preserves the old immutable association, while a successful detach followed by failed/refused attach leaves the process detached without rollback. Expected protocol refusals return bounded actionable model-visible content and branch only on `reply.condition`; cancellation reaches the binary executor.

Use one ephemeral UUID owner and one immutable association snapshot containing slug, validated memory metadata, activity heartbeat, and optional managed-worktree presence. Restart begins detached. After a completed turn, heartbeat the current owner: success refreshes memory metadata and worktree presence; `not-owner` or `missing` clears association and Remote publication; other refusals or mechanism failures silently retain advisory attachment but clear unverified memory and managed-worktree facts. Explicit detach and best-effort shutdown detach clear local state and publication according to owner-checked results; publication and shutdown failures remain silent.

On every Pi `context` event while attached, append exactly one hidden, non-persisted custom message whose content starts `[awf effort] active=<slug> memory=.awf/efforts/<slug>/memory.md`. Directly stat `.awf/worktrees/<slug>` relative to the repository root and append ` managedWorktree=.awf/worktrees/<slug>` only when it is a directory. Missing paths and inspection failures omit that field without blocking association. Refresh this optional fact after heartbeat, inject nothing while detached or after restart, and never copy Phase or Next into the transient line.

Retain Remote Pi capability negotiation, complete `awf` metadata replacement, replay, and temporary effort-slug name override. Metadata contains effort identity, validated memory when available, and activity heartbeat, but no checkout or CWD facts. Detach and ownership loss publish null metadata/name. Remove `EffortTransferCoordinator`, process-global symbols, transfer tokens/timeouts, queued continuation commands, command registration, `changeCwd`, conversation/session creation, termination, destination/role/CWD snapshots, takeover-age presentation, assigned-name diagnostics, and every `ctx.ui` status/notification/cleanup call. Update fake runtime fixtures only where their old APIs existed solely for this extension; retained handoff and subagent runtime contracts must remain green.

Tests must cover attach, repeat attach, switch ordering, each partial failure state, detach, cancellation, concurrent calls serialized in invocation order, restart, heartbeat refresh and cleanup, shutdown, fixed path line with directory present/absent/stat failure, no persistence, tool-follow-up context delivery, Remote metadata/name capability/replay/degradation, and explicit absence of queue/CWD/termination/TUI dependencies. Reach 100% TypeScript statements, branches, functions, and lines without weakening the harness.

### Task 2.3: Align skills, runtime boundaries, render contracts, docs, and changelog
Kind: batch
Latitude: exact
Applying: ["simplify-pi-effort-association-around-fixed-repository-paths:associate-without-checkout-replacement", "simplify-pi-effort-association-around-fixed-repository-paths:direct-using-effort-tool", "simplify-pi-effort-association-around-fixed-repository-paths:fixed-relative-effort-context", "simplify-pi-effort-association-around-fixed-repository-paths:advisory-process-lifecycle", "simplify-pi-effort-association-around-fixed-repository-paths:remote-name-without-local-tui", "simplify-pi-effort-association-around-fixed-repository-paths:fixed-layout-dependency-boundary", "simplify-pi-effort-association-around-fixed-repository-paths:compatibility-and-verification"]
Paths: ["templates/skills/using-effort/SKILL.md.tmpl", "templates/skills/effort-workflow/SKILL.md.tmpl", "templates/partials/pi-minimum-runtime.md", ".awf/parts/workflow/chain.md", ".awf/parts/working-with-awf/overview.md", ".awf/parts/working-with-awf/config-and-overrides.md", ".awf/parts/working-with-awf/commands.md", ".awf/docs/parts/architecture/overview.md", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", ".awf/docs/parts/architecture/dependencies.md", ".awf/docs/parts/testing/gate.md", ".awf/docs/parts/testing/layout.md", ".awf/docs/parts/testing/tiers.md", "templates/docs/workflow.md.tmpl", "templates/docs/working-with-awf.md.tmpl", "templates/docs/architecture.md.tmpl", "templates/docs/testing.md.tmpl", "README.md", "changelog/CHANGELOG.md", "internal/project/target_test.go", "internal/project/project_test.go", "internal/project/output_plan_test.go", "internal/project/spine_test.go", "internal/project/example_wiring_test.go", "internal/evals/chain_test.go", ".pi/skills/awf-using-effort/SKILL.md", ".pi/skills/awf-effort-workflow/SKILL.md", ".claude/skills/awf-effort-workflow/SKILL.md", "AGENTS.md", "docs/architecture.md", "docs/testing.md", "docs/workflow.md", "docs/working-with-awf.md"]
Representative: "Change the Pi companion from destination/rebind instructions to explicit attach/detach at repository root with supplied fixed relative paths, while the target-neutral effort workflow permits a runtime with explicit paths to remain at root and target the managed worktree by command-local path."
Edge: "Non-Pi targets must never name the Pi tool; deselection or Pi disablement must prune the companion and both extension files; empty optional template values must render coherent generic prose with no unresolved token; Remote Pi naming remains documented while every local TUI diagnostic is absent."
Post-check: "Run ./x render && ./x check, focused project/eval tests, and ./x pi-test run; require success, no unexpected generated path, and no checkout/CWD/queue/TUI association reference outside frozen ADRs and historical plans."

Update the `using-effort` skill to describe only the two exact tool forms, repository-root operation, supplied relative memory/worktree paths, explicit detach, advisory activity and Remote naming, and restart-detached behavior. Update target-neutral `effort-workflow` so runtimes without native association still enter the exact managed worktree through native persistent checkout/context tooling, while a runtime that supplies explicit effort paths may remain at root and target the worktree by path; it must not name Pi or `using_effort`. Remove the companion's `changeCwd` minimum-runtime exception while leaving retained handoff and subagent runtime floors unchanged.

Update every behavior-stating authoring doc and the same changelog entry with direct non-terminating association, transient per-model-call fixed paths, directory-presence semantics, silent local lifecycle, and retained Remote naming/metadata. Render all outputs and include deterministic `AGENTS.md`, root docs, skills, and extension consequences. Extend target/output-plan/prune tests and the invariant-bearing `TestPiRuntimeTargetRender`, `TestPiRealRuntimeSmoke`, `TestEffortWorkflowSkillContract`, `TestUnifiedEffortWorkflowCoverage`, and related container-wiring proofs rather than adding a second selection predicate. Prove Pi plus selected `effort-workflow` renders the companion and pair, non-Pi or deselected configurations render none, disablement prunes them without touching unrelated content, and every changed template renders coherently with empty optional values and no `<no value>` or unresolved-value token. Frozen ADRs and implemented historical plans remain unchanged.

### Phase close

Run `./x render`, focused Go render/eval tests, `./x pi-test run`, `./x check`, and `./x gate`; all must succeed with strict TypeScript coverage at the existing floor. Search tracked nonhistorical sources for `EffortTransferCoordinator`, association-owned `changeCwd`, queued `using_effort`, checkout/destination activity fields, `changedCwd`, and `ctx.ui` calls; the search must return no removed association machinery while permitting unrelated handoff/subagent APIs. Stage the complete runtime, tests, authoring inputs, changelog, and rendered outputs explicitly. Require `awf check staged` to report clean and create one commit.

```commit
refactor(rendering): make Pi effort association CWD-neutral
```

## Phase 3: Apply all claims and freeze the reviewed change

**Execution mode: inline.**

Completes: ["deferred-implemented-application", "repository-green"]

### Task 3.1: Directly apply the eight declared claim updates
Kind: batch
Latitude: exact
Applying: ["simplify-pi-effort-association-around-fixed-repository-paths:compatibility-and-verification"]
Paths: [".awf/topics/parts/tooling/cli/current-state.md", ".awf/topics/parts/tooling/effort-management/current-state.md", ".awf/topics/parts/rendering/pi-runtime/current-state.md", ".awf/topics/parts/rendering/pi-workflows/current-state.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md"]
Representative: "Update `tooling/cli:effort-command-contract` and `tooling/effort-management:effort-record-authority` from protocol 1 to the exact three-operation protocol-v2 grammar, v2 resident fields, recovery attach, exact handled outcomes, and unrelated-command non-gating behavior."
Edge: "Update the six rendering claims to remove checkout replacement, CWD capability, queue/transfer, and local TUI claims while preserving target gating, non-Pi absence, target-neutral fallback guidance, transient fixed-path context, Remote Pi name/metadata replay, and all retained handoff/subagent runtime floors."
Post-check: "Run ./awf context --show pending against the ADR and all five claim-source paths; require all eight operations Applied, none Remaining or Canceled, each proof marker resolved to its named live test, and no unrelated claim mutation."

Start this phase only after Phases 1 and 2 are committed, independent terminal implementation review has settled with no unresolved finding, every review fix is committed and re-reviewed as required, `git status --short` is empty, and `./x check` plus `./x gate` pass. This phase contains no production behavior, tests, adopter behavior docs, or changelog changes.

Apply exactly these State changes in declaration order and no others:

1. `tooling/cli:effort-command-contract`: preserve all unrelated effort commands and memory-update prose; state that activity exposes JSON-only v2 `attach|heartbeat|detach`, each with slug, owner, and `--json`; handled replies use the ADR's exact condition-specific envelopes and only `changedActivity`; malformed or pre-state failures keep nonzero/empty-stdout/bounded-stderr behavior; removed checkout actions and flags do not exist.
2. `tooling/effort-management:effort-record-authority`: change optional activity to protocol v2 with exactly owner and attach/heartbeat timestamps; state that explicit attach atomically replaces any safe bounded prior activity without decoding, heartbeat/detach require owned v2, safe resident and race refusals remain, unrelated commands do not gate, and activity remains advisory rather than topology, authority, or a lock.
3. `rendering/pi-runtime:pi-extension-target-render`: retain the existing Pi target/output-plan and prune contract; assign strict v2 invocation/decoding to the client and direct serialized association, fixed-path context, lifecycle, and Remote translation to the index; remove queued live-CWD and transfer ownership.
4. `rendering/pi-runtime:pi-minimum-runtime`: retain the minimum APIs for context-usage, handoff, and subagents; state that `using_effort` needs no `changeCwd` capability or foreign version floor and treats Remote Pi events as optional advisory integration.
5. `rendering/pi-workflows:pi-effort-session-association`: state one explicit process-local association, direct non-terminating attach/detach, conservative switching, fixed memory/worktree context on every model call, restart-detached and owner-loss behavior, silent advisory degradation, no checkout resolution/CWD/queue/conversation/TUI machinery, and retained complete Remote metadata plus negotiated name replay.
6. `rendering/pi-workflows:using-effort-skill`: state the two exact public forms `{effort:"<canonical-slug>"}` and `{detach:true}`, repository-root use of supplied fixed paths, explicit-only association, advisory lifecycle/Remote name, Pi-only derivation, and non-Pi absence.
7. `rendering/workflow-skill-templates:unified-effort-workflow-coverage`: preserve the complete chain-role and ownership coverage while permitting explicit-path runtimes to remain at repository root and target a managed worktree by path; keep non-native runtimes on exact managed-worktree entry and forbid a Pi-tool name in target-neutral output.
8. `rendering/workflow-skill-templates:effort-workflow`: preserve the single selectable cross-target entry guide and lifecycle ordering; direct runtimes without supplied effort paths into the exact managed worktree, permit explicit-path runtimes to operate from root by path, and continue forbidding inference, parallel worktrees, activity authority, and runtime-specific tool names.

For each update preserve `Origin`, preserve the existing `Revised-by` prefix, append `ADR-simplify-pi-effort-association-around-fixed-repository-paths` once, retain `Backing: test`, and keep or relocate each proof marker only with the live test named by that marker. Refresh surrounding current-state prose only where required by the same changed claim. Do not alter the durable deferred-flip pitfall.

### Task 3.2: Record the direct Applied and Implemented transition and freeze the plan
Latitude: exact
Applying: ["simplify-pi-effort-association-around-fixed-repository-paths:compatibility-and-verification"]
Paths: ["docs/decisions/simplify-pi-effort-association-around-fixed-repository-paths.md", "docs/plans/2026-08-03-simplify-pi-effort-association-around-fixed-repository-paths.md", "docs/decisions/INDEX.md", ".awf/awf.lock", "docs/topics/tooling/cli.md", "docs/topics/tooling/effort-management.md", "docs/topics/rendering/pi-runtime.md", "docs/topics/rendering/pi-workflows.md", "docs/topics/rendering/workflow-skill-templates.md", "docs/domains/tooling.md", "docs/domains/rendering.md"]

In the ADR, change `status: Proposed` directly to `status: Implemented`. Append the direct Applied event naming all eight operations in declaration order, then append the Implemented status-history event with the canonical content stamp required by the current ADR format; obtain any required digest from `./awf check` diagnostics rather than hashing raw file bytes. Change this plan's `status: Proposed` to `status: Implemented` without changing its frozen execution content. Run `./x render` to regenerate the affected topic/domain docs, `docs/decisions/INDEX.md`, and `.awf/awf.lock`; never hand-edit generated outputs. Confirm the diff contains claim/lifecycle/render bookkeeping only and no unreviewed behavior.

### Phase close

Stage exactly the five claim sources, ADR, plan, and deterministic render consequences. Run `awf check staged`, `./x check`, and `./x gate`; all must pass. Run `./awf context --show pending docs/decisions/simplify-pi-effort-association-around-fixed-repository-paths.md` and require all eight operations Applied with no Remaining or Canceled operation. Create the one direct terminal transaction.

```commit
refactor(rendering): freeze fixed-path Pi effort association
```

## Definition of done

- `dod: protocol-v2` The binary and CLI expose only exact JSON protocol-v2 attach, heartbeat, and detach; safe explicit attach recovers from any bounded regular prior file; owner, race, refusal-envelope, and unrelated-command behavior are proven.
- `dod: direct-pi-association` Pi attaches and detaches directly without CWD, queue, runtime replacement, conversation, or local TUI machinery; every attached model call receives fixed relative effort paths; heartbeat, restart, switch failure, cancellation, shutdown, ownership loss, and Remote Pi replay/degradation are covered at 100% TypeScript coverage.
- `dod: rendered-doc-currency` Behavior docs, skills, templates, generated outputs, `AGENTS.md`, tests, and changelog describe the same protocol and runtime; target render/prune and unset-value tests pass with no unresolved token.
- `dod: deferred-implemented-application` Independent terminal implementation review settles before one behavior-free direct transaction applies all eight declared claims with preserved provenance and co-flips the ADR and plan to Implemented.
- `dod: repository-green` Every phase closes with explicit staging, `awf check staged`, `./x check`, and `./x gate` green, and the final worktree is clean.

## Notes

- The two pre-review commits deliberately leave all eight current-state claims and both lifecycle statuses unchanged. Review fixes also land before Phase 3 and contain their matching tests, behavior-stating authoring sources, generated outputs, and changelog correction when applicable.
- Frozen terminal ADRs and implemented historical plans that describe protocol v1 remain unchanged; searches for removed machinery exclude those history surfaces rather than rewriting them.
- The final phase preserves the already-recorded durable deferred-flip pitfall and contains no production behavior.
