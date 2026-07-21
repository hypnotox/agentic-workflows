---
date: 2026-07-21
adrs: [141]
status: Proposed
---
# Plan: Pi Subagent Concurrency and Model Routing

## Goal

Implement [ADR-0141](../decisions/0141-govern-pi-parallel-exploration-and-implementation-batch-exclusivity.md): all four Pi subagent roles gain strict optional model routing, independent exploration gains an abort-aware FIFO limit of ten active children, and mixed implementation batches are blocked at Pi's real event seam.

Non-goals: no aggregate batch tool, configurable concurrency, cross-runtime model API, session dashboard, handoff command, or phase-sensitive tool activation.

## Architecture summary

Extend the generated Pi entrypoint, not the subprocess runner's public request shape: resolve each optional canonical model into the existing `{provider, id}` request before queueing, and otherwise use the active parent model. Add one extension-instance semaphore around exploration execution while retaining implementation's existing queue. Add a `tool_call` preflight handler that reconstructs the current assistant batch from the session leaf and blocks every member of a reconstructable batch containing implementation plus siblings.

The behavior, invariant claim mutations, generated outputs, and ADR/plan Implemented transitions form one coupled commit. They cannot be split into independently authoritative commits because ADR-0141's updates and additions must describe behavior that exists in the same staged transaction.

## File structure

- **Created:** none.
- **Modified (authored behavior and tests):** `templates/pi/awf-subagents/index.ts.tmpl`, `tools/pi-extension-test/tests/index.test.ts`, `internal/project/target_test.go`.
- **Modified (authored workflow and docs):** `templates/skills/brainstorming/SKILL.md.tmpl`, `templates/skills/exploring/SKILL.md.tmpl`, `templates/skills/reviewing-adr/SKILL.md.tmpl`, `templates/skills/reviewing-impl/SKILL.md.tmpl`, `templates/skills/reviewing-plan-resync/SKILL.md.tmpl`, `templates/skills/reviewing-plan/SKILL.md.tmpl`, `templates/skills/subagent-driven-development/SKILL.md.tmpl`, `templates/agents-doc/AGENTS.md.tmpl`, `templates/docs/working-with-awf.md.tmpl`, `.awf/parts/agents-doc/identity.md`, `.awf/docs/parts/architecture/components.md`, `.awf/docs/parts/testing/layout.md`, `.awf/docs/parts/releasing/content.md`, `.awf/docs/parts/roadmap/ideas.md`, `README.md`, `changelog/CHANGELOG.md`.
- **Modified (authority and lifecycle):** `.awf/topics/parts/rendering/templates/current-state.md`, `.awf/topics/parts/rendering/catalog-and-targets/current-state.md`, `docs/decisions/0141-govern-pi-parallel-exploration-and-implementation-batch-exclusivity.md`, `docs/plans/2026-07-21-pi-subagent-concurrency-and-model-routing.md`.
- **Modified (generated):** every path reported changed by `./x sync`, including dogfooded and Sundial Pi extensions and skills, `AGENTS.md`, managed docs/topics, `docs/decisions/INDEX.md`, and `.awf/awf.lock`; never edit these outputs directly.
- **Deleted:** none.

## Phase 1: Coupled behavior, authority, and lifecycle transaction

- [ ] **Task 1.1: Add failing TypeScript contract tests before production edits.** In `tools/pi-extension-test/tests/index.test.ts`, extend `harness` with an exact model registry fixture (`find(provider, id)` and `hasConfiguredAuth(model)`), configurable runner promises, and a branch-aware `sessionManager` fixture whose current leaf can be an assistant message with `toolCall` content. Preserve the existing parent model fixture.

  Add tests with these exact acceptance assertions:
  - every role schema accepts its existing required fields plus optional `model: "cheap/model/with/slash"`, still rejects unknown fields, and still requires the old role fields;
  - omission sends `{provider: "test", id: "parent"}`; each of the four roles sends an exact selected `{provider, id}`, the inherited thinking level, and requested/actual model details;
  - malformed references, unknown models, and models without configured auth reject before `runner.run`, reviewer loading side effects, exploration slot acquisition, or implementation git snapshots;
  - ten exploration runner promises start, later calls remain queued in invocation order, releasing slots starts queued calls FIFO, aborting a queued call never starts it, and success/failure/abort releases a slot;
  - a singleton implementation batch passes; for a reconstructable assistant batch containing implementation and any sibling, invoking the `tool_call` handler for every id returns `{block: true}` for every member; a stale id and malformed/non-assistant leaf block only implementation and leave unrelated calls unblocked.

  Run `./x pi-test run`; expect the new assertions to fail against the old extension for missing schemas, routing, limiter, and batch guard while all pre-existing tests remain green up to those new failures. Do not commit this red state.

- [ ] **Task 1.2: Implement strict role-wide model resolution.** In `templates/pi/awf-subagents/index.ts.tmpl`, make these exact structural changes:
  - add `requestedModel?: string` to `SubagentDetails` and a shared optional `model` property to all four closed `Type.Object` parameter schemas;
  - add a resolver with the signature `resolveChildModel(ctx: any, requested: string | undefined): { provider: string; id: string; requested?: string }`;
  - on omission, require `ctx.model` and return its provider/id; on an explicit value, split only at the first `/`, require non-empty provider and remaining model id, call `ctx.modelRegistry.find(provider, id)`, require `ctx.modelRegistry.hasConfiguredAuth(found)`, and throw an actionable error for every rejected shape;
  - do not fuzzy-match and do not fall back; use the registry object's canonical provider/id for `RunRequest.model`;
  - resolve before reviewer-file loading, queue acquisition, git snapshots, or runner execution; pass the resolved model into the shared `run` function instead of reading `ctx.model` there;
  - keep `pi.getThinkingLevel()` forwarding unchanged so child Pi owns capability clamping;
  - include the requested model in partial/final details and render the requested and actual values without replacing existing actual-model usage output;
  - update every tool description and prompt guideline to say `model` is optional and omission inherits the parent.

  Run `./x pi-test run`; expect all model-schema/routing tests and all earlier Pi tests to pass, with limiter and batch tests still failing.

- [ ] **Task 1.3: Add the abort-aware FIFO exploration limiter.** In `templates/pi/awf-subagents/index.ts.tmpl`, define `MAX_EXPLORATION_CONCURRENCY = 10` and an extension-instance limiter whose waiters retain resolve/reject, signal, and an abort listener. Immediate acquisitions increment active count. Queued abort removes that waiter, removes its listener, rejects without spawning, and preserves FIFO order. Release is idempotent, decrements active count, skips cancelled waiters, promotes the oldest live waiter, and removes its listener. Wrap only `subagent_explore` child execution in `try/finally`; grounding/review remain unbounded and implementation keeps `implementationTail`.

  Run `./x pi-test run`; expect limiter, routing, and prior tests to pass, with only batch-guard tests still failing.

- [ ] **Task 1.4: Enforce implementation-alone batches at the real Pi preflight seam.** In `templates/pi/awf-subagents/index.ts.tmpl`, register a `pi.on("tool_call", ...)` handler before result middleware. Read `ctx.sessionManager.getLeafEntry()`; accept only the current leaf `type: "message"` with `message.role: "assistant"` and array content. Select `toolCall` blocks, require one whose `id` equals `event.toolCallId` and whose `name` equals `event.toolName`, and classify the full sibling set. For a reconstructable batch containing `subagent_implement` plus any sibling, return `{block: true, reason: "...retry subagent_implement alone..."}` for every event in that batch. If correlation or shape fails, return a block only when `event.toolName === "subagent_implement"`; otherwise return `undefined`. Never scan older entries.

  Run `./x pi-test run`; expect the complete TypeScript suite and its 100% statement/branch/function/line coverage check to pass.

- [ ] **Task 1.5: Update generated-output proofs and Pi-only guidance as a batch.** In `internal/project/target_test.go`:
  - revise `TestPiStructuredExplorationContract` exact schema strings for optional `model`, assert the ten-slot limiter and model resolver, and keep the exact four registration assertion;
  - add `// invariant: rendering/templates:pi-subagent-model-routing` to a test that asserts all four schemas, first-slash parsing, exact registry/auth checks, no silent fallback, and inherited thinking;
  - extend `TestBoundedExplorationReporting` under its existing proof marker with independent-parallel versus sequential-refinement wording and the ten-active/FIFO boundary;
  - revise `TestPiSubagentToolBoundaries` so the updated catalog claim proves inherited thinking, explicit-or-inherited model choice, fixed allowlists, recursion exclusion, and retained-output bounds;
  - add `TestPiImplementationBatchExclusivity` with `// invariant: rendering/templates:pi-implementation-batch-exclusivity`, asserting leaf/id correlation, whole-batch blocking, actionable retry wording, and narrow malformed-context fallback;
  - extend cross-runtime rendering assertions so Pi output contains concurrency/model guidance and every non-Pi output remains generic and contains neither Pi tool names nor Pi model-argument syntax.

  Apply the same target-conditional prose shape to this affected-site set: `templates/skills/brainstorming/SKILL.md.tmpl`, `templates/skills/exploring/SKILL.md.tmpl`, `templates/skills/reviewing-adr/SKILL.md.tmpl`, `templates/skills/reviewing-impl/SKILL.md.tmpl`, `templates/skills/reviewing-plan-resync/SKILL.md.tmpl`, `templates/skills/reviewing-plan/SKILL.md.tmpl`, and `templates/skills/subagent-driven-development/SKILL.md.tmpl`. Representative Pi diff: after the existing tool invocation instruction, add that independent exploration may be sibling-dispatched up to the extension's ten-active ceiling and that any role may specify an exact optional `model` to avoid inheriting the parent. Edge site: implementation guidance must retain "alone in its parent tool batch" and may mention optional model, but must never suggest parallel implementation. The affected-site command is `rg -l 'subagent_(grounding|explore|review|implement)' templates/skills | sort`; its output must equal the paths listed above. Post-check: `go test ./internal/project -run 'Test(PiStructuredExplorationContract|CrossRuntimeExplorationDispatch|BoundedExplorationReporting|PiSubagentToolBoundaries|PiImplementationBatchExclusivity)'` passes.

- [ ] **Task 1.6: Update authored documentation and deferred candidates.** Make these exact content changes:
  - `templates/agents-doc/AGENTS.md.tmpl`: state that independent fresh-context exploration may run concurrently where supported, refinement stays sequential, lower-cost child models may be selected deliberately, and shared-checkout implementation stays alone;
  - `.awf/parts/agents-doc/identity.md`: describe the four Pi tools as optionally role-routed and exploration as bounded-parallel without changing the language-agnostic product identity;
  - `templates/docs/working-with-awf.md.tmpl`: document optional canonical `model` on all four schemas, strict registry/auth rejection with no fallback, inherited default and thinking, ten-active FIFO/abort behavior, and whole-batch implementation blocking plus narrow malformed fallback;
  - `.awf/docs/parts/architecture/components.md`: describe resolver-before-queue flow, the session-local exploration limiter, and leaf-correlated tool-call preflight;
  - `.awf/docs/parts/testing/layout.md`: name model-routing, limiter lifecycle, real event-seam, and cross-target publication-safety coverage;
  - `.awf/docs/parts/releasing/content.md`: add real-Pi smoke cases for one explicit lower-cost child, more than ten exploration calls proving queue progress, and a rejected mixed implementation batch;
  - `.awf/docs/parts/roadmap/ideas.md`: replace "No other roadmap ideas are recorded" with separate concise candidates for a session dashboard, a fresh-session handoff command, and phase-sensitive tool activation; do not list model routing;
  - `README.md` and `changelog/CHANGELOG.md`: summarize the optional all-role model field, ten-active exploration queue, and mechanically blocked implementation batches.

  Run `./x sync`; expect it to finish cleanly and update only derived outputs consistent with the authored inputs. Run `./x check`; expect `awf check: clean`. Run `rg -n 'session dashboard|fresh-session handoff|phase-sensitive tool' .awf/docs/parts/roadmap/ideas.md docs/roadmap.md`; expect each candidate in both authored and rendered roadmap surfaces.

- [ ] **Task 1.7: Apply ADR-0141's claim transaction and lifecycle flips.** Edit only authored topic parts:
  - in `.awf/topics/parts/rendering/templates/current-state.md`, update `bounded-exploration-reporting` to include independent concurrent dispatch, sequential refinement, and the Pi ten-active FIFO/abort-aware limit while preserving `Origin: ADR-0132` and appending `Revised-by: ADR-0141`;
  - update `pi-structured-exploration-contract` to retain exactly four tools, add optional model to every closed role schema, retain exploration's required task/breadth/detail, and cover the ten-active limiter while preserving `Origin: ADR-0132` and appending `Revised-by: ADR-0141`;
  - add test-backed `pi-subagent-model-routing` with `Origin: ADR-0141`, covering exact canonical resolution, configured auth, inheritance on omission, inherited/clamped thinking, no fallback, and requested/actual diagnostics;
  - add test-backed `pi-implementation-batch-exclusivity` with `Origin: ADR-0141`, covering current-leaf/id correlation, whole reconstructable mixed-batch blocking, and narrow fail-closed fallback;
  - in `.awf/topics/parts/rendering/catalog-and-targets/current-state.md`, update `pi-child-tool-boundaries` to replace unconditional parent-model inheritance with explicit validated selection or parent inheritance while retaining thinking, closed allowlists, recursion exclusion, and output bounds; preserve `Origin: ADR-0123` and append `Revised-by: ADR-0141`.

  Flip ADR-0141 and this plan from `Proposed` to `Implemented`. Append the plan freeze through its status only. For the ADR, first stage the complete behavior and claim transaction with a provisional terminal history entry, run `./x check --staged`, and use its reported expected `content-sha256` and `state-sequence` in the append-only Implemented line. Run `./x sync` again and stage every regenerated output, including `docs/decisions/INDEX.md` and `.awf/awf.lock`.

- [ ] **Task 1.8: Verify and commit the atomic transaction.** Run `git diff --check`; expect no output. Run `./x check`; expect clean drift. Stage the explicit authored paths listed in File structure plus every path printed as changed by `./x sync`; do not use `git add -A`. Run `./x check --staged`; expect a clean ADR-0141 five-operation transition with matching proofs and no uncovered staged path. Run `./x gate`; expect all Go, Pi-extension, coverage, lint, dead-code, pin, and prose gates to pass. Inspect `git diff --cached --name-status` and confirm every staged path belongs to this plan and every generated path has an authored cause. Commit:

```commit
feat(rendering): route and govern Pi subagents (implements 0141)
```

## Verification

After the commit, run `git status --short`; expect no output. Run `./x check`; expect `awf check: clean`. Run `./x gate`; expect the full normal gate to pass. Query `./x topic rendering/templates:pi-subagent-model-routing` and `./x topic rendering/templates:pi-implementation-batch-exclusivity`; expect both Implemented claims with ADR-0141 provenance and test backing. Inspect `docs/decisions/INDEX.md`; ADR-0141 must be in History rather than In flight.

Then invoke `awf-reviewing-impl` against the implementation commit and finish with `awf-retrospective` after review findings are resolved.

## Notes

The session dashboard, fresh-session handoff command, and phase-sensitive tool activation remain roadmap candidates. The fixed concurrency ceiling is ten and is not a new config key. Explicit model selection is per call and Pi-only; omission preserves current parent-model behavior.
