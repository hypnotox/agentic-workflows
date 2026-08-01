---
date: 2026-08-02
adrs:
  - context-aware-discretionary-pi-handoffs
status: Proposed
---
# Plan: Context-Aware Discretionary Pi Handoffs

## Goal

Implement [ADR-context-aware-discretionary-pi-handoffs](../decisions/context-aware-discretionary-pi-handoffs.md): keep durable checkpoints mandatory, let Pi choose session replacement only at safe resumable boundaries, reduce `handoff_session` to bounded prose, and inject current model-window and active-branch compaction facts before every Pi model request.

This plan does not add a pressure threshold, trigger compaction or handoff automatically, persist context-usage telemetry, alter non-Pi continuation semantics, or move effort-memory policy back into the handoff runtime.

## Architecture summary

The workflow templates remain the sole owner of checkpoint, approval, safe-point, and handoff-log policy. `templates/pi/awf-handoff/index.ts.tmpl` becomes a session-replacement mechanism whose only public data is unchanged bounded kickoff prose. A new independent `templates/pi/awf-context-usage/index.ts.tmpl` observes `ctx.getContextUsage()` and `ctx.sessionManager.getBranch()` in Pi's per-model-call `context` event, formats one neutral transient user message, and performs no persistence, UI, telemetry, warning, compaction, or replacement action. `internal/project/target.go` remains the single descriptor home that projects both generated entrypoints into output planning, drift, pruning, provenance, and adopter rendering. Deterministic TypeScript tests own runtime behavior, Go tests own generated-target and invariant-ledger coverage, and the pinned in-memory Pi runtime proves actual request delivery without session persistence.

Phases 1 through 3 apply the linked V3 ADR's first seven State changes in declaration order and stage each matching current-state claim mutation and proof marker in the same transaction. Phase 4 lands and reviews the final runtime behavior while operation 8 remains pending; deferred Phase 5 applies that final claim update immediately before the terminal status event. Every behavior phase updates the owning `.awf/` documentation source and runs `./x render` so root and Sundial generated outputs travel with their causes.

## File structure

- **Created:** `templates/pi/awf-context-usage/index.ts.tmpl`, `tools/pi-extension-test/tests/context-usage.test.ts`, `.pi/extensions/awf-context-usage/index.ts`, and `examples/sundial/.pi/extensions/awf-context-usage/index.ts`.
- **Modified:** `templates/partials/{checkpoint-routine,checkpoint-approval}.md`; `templates/pi/awf-handoff/index.ts.tmpl`; `templates/agents-doc/AGENTS.md.tmpl`; `templates/docs/{workflow,working-with-awf}.md.tmpl`; `internal/evals/chain_test.go`; `internal/project/{target.go,target_test.go,output_plan_test.go,project_test.go,example_wiring_test.go}`; `internal/contextq/adapter_outputs_test.go`; `tools/pi-extension-test/{container.sh,tests/handoff.test.ts,tests/runtime.test.ts}`; the linked ADR lifecycle/history; `changelog/CHANGELOG.md`; and the authored documentation/current-state paths listed by phase below.
- **Deleted:** no files. Memory-validation functions, types, dependencies, imports, and tests are deleted from the handoff template and handoff test file, but the handoff entrypoint itself remains.

All paths are relative to `/home/hypno/Projects/agentic-workflows/.awf/worktrees/context-aware-handoffs`. The exhaustive authored documentation/configuration set is:

- **Phase 1:** `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`, `.awf/parts/agents-doc/working-memory.md`, `templates/agents-doc/AGENTS.md.tmpl`, `templates/docs/workflow.md.tmpl`, `.awf/docs/glossary.yaml`, `.awf/domains/parts/rendering/current-state.md`, and `changelog/CHANGELOG.md`.
- **Phase 2:** `.awf/topics/parts/rendering/pi-workflows/current-state.md`, `.awf/parts/working-with-awf/commands.md`, `.awf/docs/parts/architecture/{overview,components,data-flow}.md`, `.awf/docs/parts/testing/{gate,layout}.md`, `.awf/docs/pitfalls.yaml`, `templates/docs/{workflow,working-with-awf}.md.tmpl`, `.awf/docs/glossary.yaml`, `.awf/domains/parts/rendering/current-state.md`, and `changelog/CHANGELOG.md`.
- **Phase 3:** `.awf/topics/parts/rendering/pi-runtime/current-state.md`, `.awf/docs/parts/architecture/{components,dependencies}.md`, `.awf/docs/parts/testing/{gate,layout}.md`, `.awf/topics/parts/rendering/adapter-outputs/current-state.md`, `.awf/topics/parts/rendering/project-output-plan/current-state.md`, `.awf/topics/parts/tooling/quality-gates/current-state.md`, `.awf/docs/glossary.yaml`, `.awf/domains/parts/rendering/current-state.md`, `templates/docs/working-with-awf.md.tmpl`, and `changelog/CHANGELOG.md`.
- **Phase 4:** `.awf/docs/parts/architecture/{overview,components,data-flow,dependencies}.md`, `.awf/docs/parts/testing/{gate,layout,tiers}.md`, `.awf/docs/parts/releasing/content.md`, `.awf/parts/agents-doc/{identity,working-memory}.md`, `.awf/parts/working-with-awf/commands.md`, `.awf/docs/glossary.yaml`, `.awf/domains/parts/rendering/current-state.md`, and `templates/docs/{workflow,working-with-awf}.md.tmpl`.
- **Phase 5:** `.awf/topics/parts/rendering/pi-runtime/current-state.md`, the linked ADR, this plan, and their generated lifecycle/topic/lock outputs.

The exact generated families to inspect and stage when changed by those sources are:

- Root and Sundial Pi extensions: `.pi/extensions/awf-{handoff,context-usage}/index.ts`, `.pi/extensions/awf-subagents/{index,model-routing,runner}.ts`, and their `examples/sundial/` counterparts. The existing subagent files are an inspection set for shared-guard parity and are not expected to differ.
- Root Pi checkpoint-bearing skills: `.pi/skills/awf-{brainstorming,bugfix,debugging,executing-direct,executing-plans,proposing-adr,reviewing-adr,reviewing-impl,reviewing-plan,reviewing-plan-resync,subagent-driven-development,writing-plans}/SKILL.md`, plus the corresponding `examples/sundial/.pi/skills/sundial-*/SKILL.md` set.
- Root docs and guide: `AGENTS.md`, `docs/{architecture,glossary,pitfalls,releasing,testing,workflow,working-with-awf}.md`, `docs/domains/rendering.md`, `docs/topics/rendering/{adapter-outputs,pi-runtime,pi-workflows,project-output-plan,workflow-skill-templates}.md`, and `docs/topics/tooling/quality-gates.md`.
- Sundial guide/docs: `examples/sundial/AGENTS.md` and `examples/sundial/docs/{architecture,glossary,pitfalls,releasing,testing,workflow,working-with-awf}.md` when their enabled source sections change.
- Lifecycle/render state: `docs/decisions/INDEX.md`, `.awf/awf.lock`, and `examples/sundial/.awf/awf.lock`.

`./x render` decides which members of these generated families actually differ. In every phase, `git diff --name-only` must be a subset of that phase's authored files, production/tests, linked ADR, locks, and the generated families above; an unexpected path is investigated rather than staged blindly.

## Literal claim mutations

The executor lands these blocks verbatim in the phase named below. `ADR-context-aware-discretionary-pi-handoffs` is the pending record's exact provenance token until integration assigns a number; governed numbering may substitute that number mechanically.

**Phase 1 update `memory-checkpoint-chain-coverage`:**

```markdown
### `invariant: memory-checkpoint-chain-coverage`

Checkpoint guidance renders the four-step digest: it creates no effort for a minimal simple fix or merely because a boundary was reached, and once the outcome is concrete and non-minimal it validates exactly one immutable slug and `.awf/efforts/<slug>/memory.md`, confirms `Effort: <slug>`, carries continuation in the effort's managed worktree when one exists with the owned path spelled primary-root-relative, and updates phase, next action, time, any unrecorded settled decision, and any observation in one writer-owned batch. It appends a handoff-log entry only after a fresh-session boundary actually exists. Routine implementation checkpoints remain after the phase closing commit and settled report-only review, never after checkbox tasks or helper returns; an additional checkpoint is permitted at any safe point whose next action is independently resumable, and every checkpoint points at the workflow doc's working-memory section for authority precedence, the one-writer contract, the skeleton, and the full protocol.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0152, ADR-0160, ADR-0164, ADR-0166, ADR-0167, ADR-0175, ADR-0186, ADR-0189, ADR-0197, ADR-context-aware-discretionary-pi-handoffs
Backing: test
```

**Phase 2 update `pi-session-handoff-lifecycle`:**

```markdown
### `invariant: pi-session-handoff-lifecycle`

Pi handoff retains its model-tool batch exclusivity, supported persisted-TUI check, single-use pending request, private FIFO queued command, terminating tool result, five-second countdown, cancellation, parent-linked session creation, old-history preservation, prepared-child cleanup, pre- and post-replacement failure boundary, automatic kickoff, editor fallback, visible recovery notice, and no-silent-retry behavior. Post-countdown revalidation covers only the pending request and active persisted-session state; the runtime does not infer, read, validate, mutate, or mention effort memory.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0152, ADR-0164, ADR-0167, ADR-0175, ADR-context-aware-discretionary-pi-handoffs
Backing: test
```

**Phase 2 update `pi-session-handoff-public-contract`:**

```markdown
### `invariant: pi-session-handoff-public-contract`

Pi handoff exposes exactly one required `kickoff` string property with no additional properties. It trims kickoff only to establish nonempty content, retains the public `maxLength: 1000` schema bound and execution-time 1,000-UTF-16-code-unit check, and otherwise carries the prose unchanged into the replacement session, automatic submission, editor fallback, and recovery path. It accepts no memory path or other repository, filesystem, effort, ownership, link, size, encoding, header, or identity input.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0162, ADR-0164, ADR-0167, ADR-0175, ADR-0189, ADR-context-aware-discretionary-pi-handoffs
Backing: test
```

**Phase 2 update `pi-session-handoff-workflow`:**

```markdown
### `invariant: pi-session-handoff-workflow`

Pi workflow guidance keeps checkpoint persistence mandatory and permits session replacement only after a completed formal phase checkpoint, after explicit approval and its next action are persisted, or after an additional safe resumable checkpoint. At each eligible point the agent chooses continuation or handoff from current context facts, active-branch compaction history, retained-context relevance, and upcoming work, with no fixed threshold; declining handoff is autonomous continuation, not a check-in. A replacement session appends the handoff-log boundary as its first memory update before substantive continuation, while cancellation or failure that leaves the old session active appends none. Callers carry any effort-memory reorientation instruction inside bounded kickoff prose; the handoff runtime remains effort-agnostic.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0152, ADR-0164, ADR-0166, ADR-0167, ADR-0175, ADR-context-aware-discretionary-pi-handoffs
Backing: test
```

**Phase 3 add `pi-context-usage-injection`:**

```markdown
### `invariant: pi-context-usage-injection`

Before every Pi model call, including tool-follow-up calls, the standalone context-usage extension appends exactly one non-persisted model-facing line reporting current tokens against the active model window and the compaction count from the active session branch. It formats finite values below 1,000 as rounded integers, values from 1,000 in trimmed one-decimal base-1,000 `k` units, and values from 1,000,000 in trimmed one-decimal base-1,000,000 `m` units; computes percentage by rounding `tokens / contextWindow * 100`; and emits the deterministic unknown-token or unavailable-window form. The extension never persists a message or entry, writes a file or telemetry record, changes UI, triggers a model turn, compaction, warning, or handoff, or recommends a pressure threshold.
Origin: ADR-context-aware-discretionary-pi-handoffs
Backing: test
```

**Phase 3 update `pi-extension-target-render`:**

```markdown
### `invariant: pi-extension-target-render`

Enabling Pi renders the standalone context-usage and handoff entrypoints plus the subagent index, bounded model-routing module, and runner with provenance. Context usage owns transient per-model-call fact injection, handoff owns parent-linked main-session replacement, model routing owns pure preference policy, and the subagent entrypoint retains tool registration, queueing, process lifecycle, and runtime integration. No telemetry or workflow-router output renders, and every file follows normal output-plan, drift, cleanup, target-sensitive hash, generated-checkout, adopter-example, editor-quiet, and container-coverage semantics; a target set without Pi renders none of them.
Origin: ADR-0148
Revised-by: ADR-0162, ADR-0164, ADR-0167, ADR-0173, ADR-context-aware-discretionary-pi-handoffs
Backing: test
```

**Phase 3 update `pi-minimum-runtime`:**

```markdown
### `invariant: pi-minimum-runtime`

Every generated Pi extension entrypoint requires the minimum Pi runtime APIs used by its retained contract, reports the shared single actionable incompatibility notice, and fails before registering functional hooks when required APIs are absent. Supported context-usage, handoff, and subagent operation emits no compatibility, pressure, or handoff warning.
Origin: ADR-0148
Revised-by: ADR-0162, ADR-0167, ADR-context-aware-discretionary-pi-handoffs
Backing: test
```

**Phase 5 update `pi-real-runtime-smoke` (deferred until terminal review has settled):**

```markdown
### `invariant: pi-real-runtime-smoke`

Pinned Pi runtime smoke covers generated TypeScript loading, native Pi skill discovery, prose-only effort-independent handoff, before-agent-start routing-card delivery, and transient context-usage delivery into actual model requests with refresh after an active-branch compaction. Routing and context facts do not persist as session messages, and telemetry, workflow-router, selection, context-usage UI, and automatic pressure-action surfaces remain absent.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0161, ADR-0162, ADR-0164, ADR-0167, ADR-0173, ADR-context-aware-discretionary-pi-handoffs
Backing: unbacked
Verify: Run `./x pi-test run` to exercise native Pi skill discovery, prose-only effort-independent handoff, routing-card delivery, and per-request context-usage refresh after compaction in the pinned Pi runtime without persisted routing or context messages, telemetry, workflow routing, selection, context-usage UI, or automatic pressure actions.
```

## Phase 1: Separate mandatory checkpoints from discretionary replacement

**Execution mode: subagent-driven.** Start only from the committed and reviewed plan with `git status --short` producing no output, `./x check` finishing clean, and `./x gate` passing. This phase is one independently green workflow-policy transaction and applies State change 1 only.

- [ ] **Task 1.1: Write the checkpoint-policy regressions first.** In `internal/evals/chain_test.go`, update `TestUnifiedEffortWorkflowCoverage` and the approval-boundary cases so every rendered target still contains the four-step checkpoint digest and mandatory approval stop, while Pi no longer contains an unconditional successor invocation. Assert exact ordering: persistence precedes the eligibility decision; explicit approval precedes persisted approval and any eligible handoff choice; phase review settlement precedes the routine checkpoint; and checkbox tasks/helper returns remain ineligible. Add cases for an additional safe independently-resumable checkpoint, autonomous continue-in-session wording, and truthful handoff logging: ordinary continuation/cancellation/old-session failure cannot append `## Handoff log`, while replacement-session kickoff must instruct the new session to append the boundary before substantive work. Keep non-Pi output free of `handoff_session` and replacement claims. Run `go test ./internal/evals -run 'TestUnifiedEffortWorkflowCoverage|Test.*Approval'`; the new expectations must fail before template edits.
- [ ] **Task 1.2: Rewrite the two checkpoint partials without weakening persistence.** In `templates/partials/checkpoint-routine.md`, retain classification, exact effort/path validation, writer-owned memory update, and check-in routing. Replace unconditional replacement with this closed choice: after the checkpoint, Pi considers the injected context facts, active-branch compactions, relevance of retained context, and successor work; at a formal phase boundary or other safe resumable point it either continues immediately or invokes `handoff_session` alone with prose instructing the new session to read the effort checkpoint and append the actual boundary first. State that no fixed threshold controls the choice and that continuing is not a check-in. In `templates/partials/checkpoint-approval.md`, preserve the hard stop before approval; only after approval plus next action are persisted may the same discretionary choice occur. In both partials, remove every instruction that appends the handoff log before replacement, and specify that cancellation or failure leaving the old session active logs nothing. Preserve the target-native non-Pi branch without unsupported replacement language.
- [ ] **Task 1.3: Apply the checkpoint claim and synchronize workflow prose.** Change the linked ADR to `Implementing` and append its canonical content stamp, then append an Applied event naming only `update rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`. Land the Phase 1 literal claim block and keep its existing proof markers on `TestUnifiedEffortWorkflowCoverage`, `TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion`, and `TestCheckpointDigestShape`; update those tests where their handoff-log assertions changed. Update every Phase 1 authored documentation path: define a routine checkpoint as mandatory persistence plus a discretionary eligible replacement choice, define safe resumability and truthful log timing, preserve mandatory approval semantics, and make all generic prose coherent when target variables are empty. Add an Unreleased feature entry describing mandatory checkpoints with discretionary Pi replacement. Do not describe the context-usage line as shipped until Phase 3.
- [ ] **Task 1.4: Render and bound the generated change.** Run `./x render`; inspect all checkpoint-bearing root and Sundial Pi skills, `AGENTS.md`, `docs/workflow.md`, glossary/domain/topic output, both locks, and `docs/decisions/INDEX.md`. Assert `rg -n 'invoke `handoff_session` alone with the exact|updates phase, next action, time, and handoff log' templates/partials templates/docs .awf/parts .awf/topics/parts/rendering` returns no stale unconditional/log-before-boundary contract. Run `./x check` and require clean drift and no Sundial notes.
- [ ] **Phase-close: stage, check, gate, and commit.** Run `go test ./internal/evals ./internal/project ./internal/contextq`, `git diff --check`, `./x render`, and `./x check`; all must succeed. Stage only this transaction, require `./awf check --staged` and `./x gate` to pass, then commit:

```commit
feat(rendering): make Pi handoff discretionary
```

## Phase 2: Reduce handoff to bounded prose

**Execution mode: subagent-driven.** Start with `git status --short` empty, `git log -1 --format=%s` equal to `feat(rendering): make Pi handoff discretionary`, `./x check` clean, and `./x gate` passing. This phase is one independently green runtime-contract transaction and applies State changes 2, 3, and 4 in their declaration order.

- [ ] **Task 2.1: Replace memory-validation tests with the prose contract first.** Rewrite `tools/pi-extension-test/tests/handoff.test.ts` imports and harness so no test dependency provides filesystem, path, uid, ownership, link, identity, UTF-8, effort header, or repository-root behavior. Delete the memory path normalization/confinement/revalidation cases. Add assertions that the TypeBox schema has exactly required `kickoff`, rejects missing/unknown properties, retains `maxLength: 1000`, rejects empty/whitespace and 1,001 UTF-16 code units at execution, accepts exactly 1,000 UTF-16 code units including surrogate-pair cases by JavaScript `.length`, and carries nonempty kickoff bytes unchanged (including leading/trailing whitespace) through request details, automatic `sendUserMessage`, editor fallback, and failure recovery. Retain explicit tests for mixed-batch blocking, unverifiable preflight, unsupported mode/persistence, single pending request, FIFO queue token, terminating result, five-second display, Esc cancellation, stale token, lost persisted session after countdown, parent lineage, prepared-child cleanup, queue failure cleanup, timer/key faults, automatic-kickoff failure, cleanup failure, and the shared minimum-runtime guard. Run `./x pi-test run`; it must fail against the old public schema before production edits.
- [ ] **Task 2.2: Simplify the handoff template without disturbing replacement mechanics.** In `templates/pi/awf-handoff/index.ts.tmpl`, delete Node filesystem/path imports, `HandoffStat`, root discovery, ownership/identity helpers, `validateMemoryPath`, `buildKickoffWrapper`, and their dependency fields. Define the tool parameters exactly as `Type.Object({kickoff:Type.String({maxLength:1000})},{additionalProperties:false})`. In `execute`, require `typeof params.kickoff === "string"`, `params.kickoff.trim().length > 0`, and `params.kickoff.length <= 1000`; store exactly the original string. Return no memory detail. In the queued command, use `request.kickoff` unchanged for `sendUserMessage`, editor fallback, and both recovery paths; after countdown, revalidate only the matching pending request and `ctx.sessionManager.getSessionFile()`. Preserve every retained lifecycle branch and keep `handoff_session` batch-exclusive.
- [ ] **Task 2.3: Update the generated contract proofs.** In `internal/project/target_test.go`, rename `TestHandoffPublicOwnedMemoryContract` to `TestHandoffPublicProseContract` and `TestHandoffWorkflowUsesOwnedCheckpoint` to `TestHandoffWorkflowKeepsPolicyOutsideRuntime`; update their proof-marker names in the same file. Assert the exact one-property schema, unchanged kickoff use, explicit UTF-16 runtime bound, retained mechanics, and absence of `memoryPath`, `.awf/efforts/`, filesystem/path imports, validation helpers, wrapper prose, effort selection, telemetry, or lifecycle mutation. Update `internal/project/output_plan_test.go` proof-marker names only where required by renamed tests, while leaving output planning behavior unchanged.
- [ ] **Task 2.4: Apply the three handoff claim updates and documentation.** Append one Applied event naming, in order, `update rendering/pi-workflows:pi-session-handoff-lifecycle`, `update rendering/pi-workflows:pi-session-handoff-public-contract`, and `update rendering/pi-workflows:pi-session-handoff-workflow`; land all three Phase 2 literal blocks with their updated proof markers in the same staged transaction. Update every Phase 2 authored documentation path to remove runtime memory validation and describe the division of ownership: workflow instructions require and reorient from checkpoints, bounded kickoff prose carries that instruction, and the runtime owns replacement mechanics only. Remove or rewrite the `handoff_session memory paths are one effort-owned spelling` pitfall rather than preserving a false warning. Add an Unreleased breaking-change entry that names removal of `memoryPath` and the exact `{kickoff}` replacement schema.
- [ ] **Task 2.5: Render, type-check, and sweep stale policy.** Run `./x render`, then require `rg -n 'validateMemoryPath|buildKickoffWrapper|memoryPath:Type|1 MiB|TextDecoder|sameIdentity' templates/pi/awf-handoff tools/pi-extension-test/tests/handoff.test.ts .pi/extensions/awf-handoff examples/sundial/.pi/extensions/awf-handoff .awf/parts .awf/docs/parts templates/docs` to return no output. Inspect root and Sundial handoff output and generated architecture/testing/workflow/working-with-awf docs. Run `./x check` with clean drift and no adopter notes.
- [ ] **Phase-close: stage, check, gate, and commit.** Run `./x pi-test run`, `go test ./internal/project ./internal/evals ./internal/contextq`, `git diff --check`, `./x render`, and `./x check`; all must succeed. Stage only this transaction, require `./awf check --staged` and `./x gate`, then commit:

```commit
refactor(rendering): reduce Pi handoff to prose
```

## Phase 3: Add transient context usage and govern the fifth output

**Execution mode: subagent-driven.** Start with `git status --short` empty, `git log -1 --format=%s` equal to `refactor(rendering): reduce Pi handoff to prose`, `./x check` clean, and `./x gate` passing. This phase is one independently green extension/output transaction and applies State changes 5, 6, and 7 in order.

- [ ] **Task 3.1: Specify formatting and per-call behavior in a new failing TypeScript suite.** Create `tools/pi-extension-test/tests/context-usage.test.ts` around exported pure `formatCount`, `contextUsageLine`, `guardMinimumRuntime`, and `registerContextUsage` seams. Test exact count outputs around integer, `k`, and `m` boundaries; JavaScript `toFixed(1)` rounding with trailing `.0` removed; percentage from `Math.round(tokens / contextWindow * 100)` rather than `usage.percent`; `[session context] unknown/272k; compactions=0` for null/non-finite tokens with a finite positive window; and `[session context] unavailable; compactions=0` for a missing, non-finite, zero, or negative window. Invoke the registered `context` handler repeatedly with mutable usage/model-window and branch fixtures: each call must return a fresh copied message array with exactly one appended hidden custom message containing the current line, count only entries whose `type === "compaction"` on `getBranch()`, refresh after a simulated tool result and model-window change, and leave input messages and branch unchanged. Spy on all Pi registration/mutation/UI surfaces and assert supported operation registers only `context`, produces no notification, status, widget, entry, file, telemetry, command, tool, model turn, compaction, or handoff. Cover the shared compatibility guard's one-notice and no-functional-registration branches. The suite must fail because the template/output do not yet exist.
- [ ] **Task 3.2: Implement the standalone context extension exactly.** Create `templates/pi/awf-context-usage/index.ts.tmpl` with the provenance-compatible `// @ts-nocheck` second line and the shared `pi-minimum-runtime` include. Keep all formatting local to this entrypoint; do not import or change the subagent formatter. `formatCount` accepts a finite number, rounds values below 1,000 to an integer, uses base 1,000 `k` and base 1,000,000 `m` with `toFixed(1).replace(/\.0$/, "")`. `contextUsageLine(ctx)` calls `ctx.getContextUsage()` at event time, validates a positive finite `contextWindow`, handles null/non-finite tokens, computes percentage directly, and counts `ctx.sessionManager.getBranch().filter(entry => entry.type === "compaction")`. `registerContextUsage` requires only the shared runtime APIs it consumes, then registers `pi.on("context", ...)`; the handler returns `{messages:[...event.messages,{role:"custom",customType:"awf-context-usage",content:line,display:false,timestamp:Date.now()}]}` and calls no other API. Default export supplies the pinned package version and delegates to that registration seam.
- [ ] **Task 3.3: Add the output to the one descriptor and every enumerating assertion.** In `internal/project/target.go`, add `.pi/extensions/awf-context-usage/index.ts` with template ID `pi/awf-context-usage/index.ts.tmpl`, `TargetOutputTemplate`, `PlainAgentDialect`, slash-comment provenance, and explicit zero output policy. In `internal/project/target_test.go`, update the exact extension map and registration assertions and add `TestPiContextUsageInjection` with proof marker `// invariant: rendering/pi-runtime:pi-context-usage-injection (TestPiContextUsageInjection)`; assert the context hook, exact sample/unknown/unavailable literals, active-branch access, and absence of persistence/side-effect API calls. Extend `TestPiMinimumRuntime` to all three entrypoints and update `TestPiRuntimeTargetRender` for the new ownership split without asserting a numeric output count. In `internal/project/output_plan_test.go`, add the exact new path/template pair to output-plan completeness, provenance, declarer, target-sensitive hash, and current-tree proof coverage. In `internal/project/project_test.go`, add the new file and directory to Pi render/prune tests. In `internal/project/example_wiring_test.go`, rely on descriptor enumeration for editor-quiet coverage and assert both root and Sundial carry the new file. In `internal/contextq/adapter_outputs_test.go`, assert explicit context selection classifies the new generated extension under adapter-output ownership and keeps generated executable coverage exclusion.
- [ ] **Task 3.4: Render the new output, then put it under strict container coverage.** Run `./x render` after Tasks 3.2 and 3.3 and require both `.pi/extensions/awf-context-usage/index.ts` and `examples/sundial/.pi/extensions/awf-context-usage/index.ts` to exist before invoking the TypeScript lane. Add the root output to `tools/pi-extension-test/container.sh`'s explicit c8 includes; keep the source copy and recursive editor-quiet strip ordered before `tsc`. Do not add a coverage ignore for reachable formatting, unavailable, or runtime-guard branches. Run `./x pi-test run`; TypeScript compilation and 100% line/function/branch coverage must pass with every extension test.
- [ ] **Task 3.5: Apply the context, target, and runtime-floor claim batch.** Append one Applied event naming, in order, `add rendering/pi-runtime:pi-context-usage-injection`, `update rendering/pi-runtime:pi-extension-target-render`, and `update rendering/pi-runtime:pi-minimum-runtime`. Land the three Phase 3 literal blocks and the named Go proof marker together. Update every Phase 3 authored documentation path to describe separate observation/replacement/subagent ownership, five governed Pi TypeScript outputs, Pi-only render/prune behavior, strict container coverage, neutral model-window terminology, active-branch compaction counting, publication-safe unavailable output, and the absence of pressure action or supported-operation warnings. Add exact glossary entries for `session context facts` (the transient model-window/active-branch-compaction line) and `safe resumable point` (a durable checkpoint whose immediate successor can start independently). Add an Unreleased feature entry for the standalone context-usage extension.
- [ ] **Task 3.6: Render both adopters and prove absence on non-Pi targets.** Run `./x render`; inspect the new root and Sundial outputs, target descriptor tests, locks, context ownership docs, testing docs, and the current-state index. Render/scaffold a target set without Pi through the existing project tests and assert it produces none of `.pi/extensions/awf-context-usage`, `.pi/extensions/awf-handoff`, or `.pi/extensions/awf-subagents`. Require `rg -n '<no value>' .pi/extensions/awf-context-usage examples/sundial/.pi/extensions/awf-context-usage` to return no output and `./x check` to be clean with no Sundial notes.
- [ ] **Phase-close: stage, check, gate, and commit.** Run `./x pi-test run`, `go test ./internal/project ./internal/contextq`, `git diff --check`, `./x render`, and `./x check`; all must succeed. Stage only this transaction, require `./awf check --staged` and `./x gate`, then commit:

```commit
feat(rendering): inject Pi session context facts
```

## Phase 4: Prove pinned-runtime refresh and close documentation

**Execution mode: subagent-driven.** Start with `git status --short` empty, `git log -1 --format=%s` equal to `feat(rendering): inject Pi session context facts`, `./x check` clean, and `./x gate` passing. This phase is one independently green real-runtime/documentation transaction. It leaves State change 8 Remaining so the ADR retains a legal `Implementing` state through terminal review.

- [ ] **Task 4.1: Extend the pinned in-memory runtime smoke to inspect actual requests.** In `tools/pi-extension-test/tests/runtime.test.ts`, register both `registerSubagentTools` and `registerContextUsage` in `DefaultResourceLoader`. Change the fake provider capture from system prompts alone to complete request contexts. Keep the supplied `SessionManager.inMemory(cwd)` in a local variable, run one prompt, append an active-branch compaction through the pinned `SessionManager` API, and run a second prompt. Assert the provider's first request contains exactly one hidden custom `[session context] ...; compactions=0` message and the post-compaction request contains exactly one refreshed line ending `compactions=1`; assert both use the pinned model's 4,096-token window form and the later call reflects current usage rather than the earlier line. Preserve the routing-card assertions. Across `session.messages`, manager entries, and serialized request/session projections, assert no context-usage custom message or custom entry persisted, no telemetry/router/selection surface appeared, and no extra model call occurred beyond the explicit prompts. Do not mutate `tools/pi-extension-test/fixtures/fake-pi.mjs` or `term-resistant-pi.mjs`: this smoke uses the in-process pinned provider and session manager, while those fixtures remain owned by subagent process tests.
- [ ] **Task 4.2: Finish behavior documentation while leaving the final operation pending.** Do not append an Applied event and do not mutate `rendering/pi-runtime:pi-real-runtime-smoke` in this phase. Update every Phase 4 authored documentation path so architecture and data flow identify transient context injection, prose-only replacement, and workflow-owned memory; testing describes deterministic unit/container coverage plus the pinned real-request compaction refresh; release instructions exercise the context line, compaction refresh, discretionary cancellation/editor recovery, and no persistence or automatic action; agent identity/working-memory and working-with-awf prose carry the final checkpoint-versus-handoff split. Ensure no current-state doc claims handoff validates memory and no release smoke asks for optional confined memory.
- [ ] **Task 4.3: Render, sweep, and verify the complete behavior.** Run `./x render` and inspect every generated family named in File structure. Run `rg -n 'optional confined memory|memoryPath|validates the exact owned memory|invokes handoff_session alone with the exact|automatic parent-linked continuation' .awf/parts .awf/docs/parts .awf/domains/parts/rendering .awf/topics/parts/rendering templates AGENTS.md docs/architecture.md docs/glossary.md docs/pitfalls.md docs/releasing.md docs/testing.md docs/workflow.md docs/working-with-awf.md docs/domains/rendering.md docs/topics/rendering examples/sundial/AGENTS.md examples/sundial/docs examples/sundial/.pi .pi`; it must return no matches. Historical ADRs, plans, and changelog entries are outside this explicit search set. Run `./x pi-test run`, `go test ./...`, `./x check`, and `git diff --check`; each must finish successfully, with context usage absent from persisted messages and Sundial reporting no notes.
- [ ] **Phase-close: stage, check, gate, and commit.** Stage only this transaction, require `./awf check --staged` and `./x gate` to pass, then commit:

```commit
test(rendering): prove Pi context refresh in runtime
```

## Governed post-phase workflow tail

This tail is not an implementation phase or transaction. After Phase 4 closes green, merge `main` into the managed worktree; resolve or abort any conflict, and require a clean completed merge. Run `./awf adr number context-aware-discretionary-pi-handoffs`, then `./x render`; stage only numbering/render output, require `./awf check --staged && ./x gate`, and commit `docs(adr): number context-aware Pi handoffs` with the printed slug-to-number mapping in the body. Invoke the native `awf-reviewing-impl` workflow over every implementation commit after the committed plan-review baseline; resolve all findings in new green commits until review reports zero unresolved findings.

In the clean primary checkout, run `./awf effort integrate context-aware-handoffs`. Accept only an already-integrated or fast-forward result, or a divergent result explicitly reported as staged without a commit. For a divergent result run `./awf check --staged && ./x gate`, commit the merge, and invoke `awf-reviewing-impl` again over the combined target history until zero unresolved findings. Stop on conflicts, a red check/gate, or any user-decision finding. The ADR remains `Implementing` and this plan remains `Proposed` throughout this tail. Proceed to Phase 5 only in the integrated primary checkout after terminal implementation review settles.

## Phase 5: Freeze the reviewed ADR and plan

**Execution mode: inline.** This is one deferred, independently green claim/lifecycle transaction in the integrated primary checkout. State change 8 remains unapplied until this transaction.

- [ ] **Task 5.1: Apply the reviewed final claim and record terminal lifecycle state.** Land the Phase 5 `pi-real-runtime-smoke` literal block in `.awf/topics/parts/rendering/pi-runtime/current-state.md`, then append an Applied event naming only `update rendering/pi-runtime:pi-real-runtime-smoke`. In the same staged transaction, append the now-numbered linked ADR's `Implemented` event with the same canonical content stamp used by its `Implementing` event. Change this plan's `status:` to `Implemented` and record actual implementation deviations under Notes without changing the approved design. Run `./x render`; `docs/decisions/INDEX.md` must move the ADR to history, every declared operation must be applied exactly once and in declaration order, and the final claim update must render to `docs/topics/rendering/pi-runtime.md` before staged checking.
- [ ] **Phase-close: stage, check, gate, and commit.** Run `git diff --check`, `./x render`, and `./x check`; stage only the authored/rendered Pi-runtime claim, ADR, plan, index, and lock outputs, require `./awf check --staged` and `./x gate`, then commit:

```commit
feat(rendering): finalize context-aware Pi handoffs
```

After Phase 5 commits, run `./awf effort worktree remove context-aware-handoffs` without force. Require `test ! -e .awf/worktrees/context-aware-handoffs`, `git worktree list --porcelain | rg 'context-aware-handoffs'` to return no output, and `git branch --list awf/context-aware-handoffs` to return no output before retrospective.

## Verification

- `./x gate`, `go test ./...`, `./x pi-test run`, and `./x check` finish successfully; TypeScript retains 100% line/function/branch coverage and Go retains 100% statement coverage.
- `./awf check --staged` accepts each implementation transaction only with its declaration-ordered Applied event, exact claim mutation, and required proof marker staged together.
- Every mandatory checkpoint persists resumable memory, while Pi replacement is offered only after formal phases, persisted approval, or an additional safe resumable checkpoint and is controlled by judgment rather than a numeric threshold.
- `handoff_session` exposes exactly required `{kickoff}`, rejects empty or over-1,000-UTF-16-unit prose, carries all accepted prose unchanged, and preserves exclusivity, queue, countdown, cancellation, lineage, cleanup, fallback, and recovery behavior without any memory or filesystem input.
- Every actual Pi model request receives one current `[session context]` line; tool-follow-up/model-window/compaction changes refresh it, only active-branch compactions count, unavailable forms are deterministic, and neither requests nor session state gain warnings, telemetry, UI, automatic actions, or persisted context-usage messages.
- Pi target render, drift, target-sensitive hash, cleanup, pruning, editor-quiet strip, container coverage, root generated checkout, and Sundial include the context-usage entrypoint; target sets without Pi produce none of the Pi extension tree.
- `rg -n 'validateMemoryPath|buildKickoffWrapper|memoryPath:Type' templates/pi/awf-handoff tools/pi-extension-test/tests/handoff.test.ts .pi/extensions/awf-handoff examples/sundial/.pi/extensions/awf-handoff` returns no output.
- `git diff --exit-code` is clean after commits, the example adopter reports no notes, and release guidance describes the final runtime rather than the retired memory-validation contract.

## Notes

- The context-usage formatter intentionally remains separate from the subagent display formatter: they implement different presentation contracts and sharing would change the approved one-decimal `k`/`m` output.
- `ctx.getContextUsage()` exposes the active model window, not Pi's configured auto-compaction threshold. Documentation and messages must continue to call it the model window and must not infer a threshold.
- Historical ADRs remain frozen. Current-state claims, generated docs, glossary, pitfalls, and changelog correct the retired automatic-handoff and memory-validation contracts forward.
- The plan and ADR stay Proposed throughout plan review. Implementation begins only after user authorization; the first implementation transaction, not this plan commit, moves the ADR to Implementing.
