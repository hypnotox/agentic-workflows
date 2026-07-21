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

Repository root for every path below: `/home/hypno/Projects/agentic-workflows`. All task paths are absolute against this root; commands run from this root and therefore use the displayed relative suffixes.

- **Created:** none.
- **Modified (authored behavior and tests):** `/home/hypno/Projects/agentic-workflows/templates/pi/awf-subagents/index.ts.tmpl`, `/home/hypno/Projects/agentic-workflows/tools/pi-extension-test/tests/index.test.ts`, `/home/hypno/Projects/agentic-workflows/tools/pi-extension-test/tests/runtime.test.ts`, `/home/hypno/Projects/agentic-workflows/internal/project/target_test.go`.
- **Modified (authored workflow and docs):** the seven absolute paths under `/home/hypno/Projects/agentic-workflows/templates/skills/` named `brainstorming/SKILL.md.tmpl`, `exploring/SKILL.md.tmpl`, `reviewing-adr/SKILL.md.tmpl`, `reviewing-impl/SKILL.md.tmpl`, `reviewing-plan-resync/SKILL.md.tmpl`, `reviewing-plan/SKILL.md.tmpl`, and `subagent-driven-development/SKILL.md.tmpl`; plus `/home/hypno/Projects/agentic-workflows/templates/agents-doc/AGENTS.md.tmpl`, `/home/hypno/Projects/agentic-workflows/templates/docs/working-with-awf.md.tmpl`, `/home/hypno/Projects/agentic-workflows/.awf/parts/agents-doc/identity.md`, `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/architecture/components.md`, `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/testing/layout.md`, `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/releasing/content.md`, `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/roadmap/ideas.md`, `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/rendering/current-state.md`, `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/tooling/current-state.md`, `/home/hypno/Projects/agentic-workflows/README.md`, and `/home/hypno/Projects/agentic-workflows/changelog/CHANGELOG.md`.
- **Modified (authority and lifecycle):** `/home/hypno/Projects/agentic-workflows/.awf/topics/parts/rendering/templates/current-state.md`, `/home/hypno/Projects/agentic-workflows/.awf/topics/parts/rendering/catalog-and-targets/current-state.md`, `/home/hypno/Projects/agentic-workflows/docs/decisions/0141-govern-pi-parallel-exploration-and-implementation-batch-exclusivity.md`, and `/home/hypno/Projects/agentic-workflows/docs/plans/2026-07-21-pi-subagent-concurrency-and-model-routing.md`.
- **Modified (generated, exhaustive expected set):** `/home/hypno/Projects/agentic-workflows/.pi/extensions/awf-subagents/index.ts`; `/home/hypno/Projects/agentic-workflows/.pi/skills/awf-brainstorming/SKILL.md`; `/home/hypno/Projects/agentic-workflows/.pi/skills/awf-exploring/SKILL.md`; `/home/hypno/Projects/agentic-workflows/.pi/skills/awf-reviewing-adr/SKILL.md`; `/home/hypno/Projects/agentic-workflows/.pi/skills/awf-reviewing-impl/SKILL.md`; `/home/hypno/Projects/agentic-workflows/.pi/skills/awf-reviewing-plan-resync/SKILL.md`; `/home/hypno/Projects/agentic-workflows/.pi/skills/awf-reviewing-plan/SKILL.md`; `/home/hypno/Projects/agentic-workflows/.pi/skills/awf-subagent-driven-development/SKILL.md`; `/home/hypno/Projects/agentic-workflows/AGENTS.md`; `/home/hypno/Projects/agentic-workflows/docs/architecture.md`; `/home/hypno/Projects/agentic-workflows/docs/releasing.md`; `/home/hypno/Projects/agentic-workflows/docs/roadmap.md`; `/home/hypno/Projects/agentic-workflows/docs/testing.md`; `/home/hypno/Projects/agentic-workflows/docs/working-with-awf.md`; `/home/hypno/Projects/agentic-workflows/docs/domains/rendering.md`; `/home/hypno/Projects/agentic-workflows/docs/domains/tooling.md`; `/home/hypno/Projects/agentic-workflows/docs/topics/rendering/catalog-and-targets.md`; `/home/hypno/Projects/agentic-workflows/docs/topics/rendering/templates.md`; `/home/hypno/Projects/agentic-workflows/docs/decisions/INDEX.md`; `/home/hypno/Projects/agentic-workflows/.awf/awf.lock`; `/home/hypno/Projects/agentic-workflows/examples/sundial/.pi/extensions/awf-subagents/index.ts`; `/home/hypno/Projects/agentic-workflows/examples/sundial/.pi/skills/sundial-brainstorming/SKILL.md`; `/home/hypno/Projects/agentic-workflows/examples/sundial/.pi/skills/sundial-exploring/SKILL.md`; `/home/hypno/Projects/agentic-workflows/examples/sundial/.pi/skills/sundial-reviewing-adr/SKILL.md`; `/home/hypno/Projects/agentic-workflows/examples/sundial/.pi/skills/sundial-reviewing-impl/SKILL.md`; `/home/hypno/Projects/agentic-workflows/examples/sundial/.pi/skills/sundial-reviewing-plan-resync/SKILL.md`; `/home/hypno/Projects/agentic-workflows/examples/sundial/.pi/skills/sundial-reviewing-plan/SKILL.md`; `/home/hypno/Projects/agentic-workflows/examples/sundial/.pi/skills/sundial-subagent-driven-development/SKILL.md`; `/home/hypno/Projects/agentic-workflows/examples/sundial/AGENTS.md`; `/home/hypno/Projects/agentic-workflows/examples/sundial/docs/working-with-awf.md`; and `/home/hypno/Projects/agentic-workflows/examples/sundial/.awf/awf.lock`. If `./x sync` reports an additional generated path, stop and add its authored cause and absolute path to this plan before staging it.
- **Deleted:** none.

## Phase 1: Coupled behavior, authority, and lifecycle transaction

- [ ] **Task 1.1: Add failing TypeScript contract tests before production edits.** In `/home/hypno/Projects/agentic-workflows/tools/pi-extension-test/tests/index.test.ts`, extend `harness` with an exact model registry fixture (`find(provider, id)` and `hasConfiguredAuth(model)`), configurable runner promises, and a branch-aware `sessionManager` fixture whose current leaf can be an assistant message with `toolCall` content. Preserve the existing parent model fixture.

  Add tests with these exact acceptance assertions:
  - every role schema accepts its existing required fields plus optional `model: "cheap/model/with/slash"`, still rejects unknown fields, and still requires the old role fields;
  - omission sends `{provider: "test", id: "parent"}`; each of the four roles sends an exact selected `{provider, id}`, the inherited thinking level, and requested/actual model details;
  - malformed references, unknown models, and models without configured auth reject before `runner.run`, reviewer loading side effects, exploration slot acquisition, or implementation git snapshots;
  - ten exploration runner promises start, later calls remain queued in invocation order, releasing slots starts queued calls FIFO, aborting a queued call never starts it, and success, child failure, abort, and runner setup rejection each release a slot and start the next live waiter;
  - the unit harness proves handler classification for a singleton implementation, a reconstructable mixed batch, stale ids, and malformed/non-assistant leaves;
  - `/home/hypno/Projects/agentic-workflows/tools/pi-extension-test/tests/runtime.test.ts` adds a real in-memory `createAgentSession` regression whose streamed assistant messages exercise singleton implementation, whole mixed-batch blocking, stale or malformed correlation, and the narrow implementation-only fallback through Pi's actual parallel preflight and persisted session leaf.

  Run `./x pi-test run`; expect the new assertions to fail against the old extension for missing schemas, routing, limiter, and batch guard while all pre-existing tests remain green up to those new failures. Do not commit this red state.

- [ ] **Task 1.2: Implement strict role-wide model resolution.** In `/home/hypno/Projects/agentic-workflows/templates/pi/awf-subagents/index.ts.tmpl`, make these exact structural changes. The production resolver body must be this literal shape (type names may be imported rather than inlined, but no fuzzy or fallback branch may be added):

  ```ts
  function resolveChildModel(ctx: any, requested: string | undefined) {
    if (requested === undefined) {
      if (!ctx.model) throw new Error("Cannot start a subagent without an active parent model");
      return { model: { provider: ctx.model.provider, id: ctx.model.id }, requested: undefined };
    }
    const slash = requested.indexOf("/");
    if (slash <= 0 || slash === requested.length - 1) throw new Error("Subagent model must be an exact provider/model-id reference");
    const provider = requested.slice(0, slash);
    const id = requested.slice(slash + 1);
    const found = ctx.modelRegistry.find(provider, id);
    if (!found) throw new Error(`Subagent model ${requested} is not registered`);
    if (!ctx.modelRegistry.hasConfiguredAuth(found)) throw new Error(`Subagent model ${requested} has no configured authentication`);
    return { model: { provider: found.provider, id: found.id }, requested };
  }
  ```
  - add `requestedModel?: string` to `SubagentDetails` and a shared optional `model` property to all four closed `Type.Object` parameter schemas;
  - add a resolver with the signature `resolveChildModel(ctx: any, requested: string | undefined): { provider: string; id: string; requested?: string }`;
  - on omission, require `ctx.model` and return its provider/id; on an explicit value, split only at the first `/`, require non-empty provider and remaining model id, call `ctx.modelRegistry.find(provider, id)`, require `ctx.modelRegistry.hasConfiguredAuth(found)`, and throw an actionable error for every rejected shape;
  - do not fuzzy-match and do not fall back; use the registry object's canonical provider/id for `RunRequest.model`;
  - resolve before reviewer-file loading, queue acquisition, git snapshots, or runner execution; pass the resolved model into the shared `run` function instead of reading `ctx.model` there;
  - keep `pi.getThinkingLevel()` forwarding unchanged so child Pi owns capability clamping;
  - include the requested model in partial/final details and render the requested and actual values without replacing existing actual-model usage output;
  - update every tool description and prompt guideline to say `model` is optional and omission inherits the parent.

  Run `./x pi-test run`; expect all model-schema/routing tests and all earlier Pi tests to pass, with limiter and batch tests still failing.

- [ ] **Task 1.3: Add the abort-aware FIFO exploration limiter.** In `/home/hypno/Projects/agentic-workflows/templates/pi/awf-subagents/index.ts.tmpl`, define `MAX_EXPLORATION_CONCURRENCY = 10` and `createLimiter(limit)` with an `acquire(signal): Promise<() => void>` API. Its exact state transition is: immediate acquire increments `active`; queued acquire appends one waiter; queued abort splices that waiter, removes its listener, and rejects; release is idempotent, decrements `active`, shifts cancelled waiters, increments for and resolves the oldest live waiter, and removes its listener. The exploration registration must use this exact control shape:

  ```ts
  const release = await explorationLimiter.acquire(signal);
  try {
    return toolResult("explore", params.task, await run(/* resolved model and existing arguments */));
  } finally {
    release();
  }
  ```

  Grounding/review remain outside the limiter and implementation keeps `implementationTail`.

  Run `./x pi-test run`; expect limiter, routing, and prior tests to pass, with only batch-guard tests still failing.

- [ ] **Task 1.4: Enforce implementation-alone batches at the real Pi preflight seam.** In `/home/hypno/Projects/agentic-workflows/templates/pi/awf-subagents/index.ts.tmpl`, register a `pi.on("tool_call", ...)` handler before result middleware. Read only `ctx.sessionManager.getLeafEntry()`; accept only the current leaf `type: "message"` with `message.role: "assistant"` and array content. Select `toolCall` blocks, require one whose `id` equals `event.toolCallId` and whose `name` equals `event.toolName`, and classify the full sibling set. Its terminal branches must be exactly:

  ```ts
  if (!correlated) return event.toolName === "subagent_implement"
    ? { block: true, reason: "Cannot verify the current tool batch; retry subagent_implement alone." }
    : undefined;
  if (calls.length > 1 && calls.some((call) => call.name === "subagent_implement"))
    return { block: true, reason: "A batch containing subagent_implement cannot contain siblings; retry subagent_implement alone." };
  return undefined;
  ```

  Never scan older entries. In `/home/hypno/Projects/agentic-workflows/tools/pi-extension-test/tests/runtime.test.ts`, construct the extension through `DefaultResourceLoader` and `createAgentSession`, stream assistant `toolCall` batches, and assert `tool_execution_end`/stored `toolResult` errors prove the registered handler ran at the actual Pi preflight seam rather than calling the handler function directly.

  Run `./x pi-test run`; expect the complete TypeScript suite and its 100% statement/branch/function/line coverage check to pass.

- [ ] **Task 1.5: Update generated-output proofs and Pi-only guidance as a batch.** In `/home/hypno/Projects/agentic-workflows/internal/project/target_test.go`:
  - revise `TestPiStructuredExplorationContract` exact schema strings for optional `model`, assert the ten-slot limiter and model resolver, and keep the exact four registration assertion;
  - add `// invariant: rendering/templates:pi-subagent-model-routing` to a test that asserts all four schemas, first-slash parsing, exact registry/auth checks, no silent fallback, and inherited thinking;
  - extend `TestBoundedExplorationReporting` under its existing proof marker with independent-parallel versus sequential-refinement wording and the ten-active/FIFO boundary;
  - revise `TestPiSubagentToolBoundaries` so the updated catalog claim proves inherited thinking, explicit-or-inherited model choice, fixed allowlists, recursion exclusion, and retained-output bounds;
  - add `TestPiImplementationBatchExclusivity` with `// invariant: rendering/templates:pi-implementation-batch-exclusivity`, asserting leaf/id correlation, whole-batch blocking, actionable retry wording, and narrow malformed-context fallback;
  - extend cross-runtime rendering assertions so Pi output contains concurrency/model guidance and every non-Pi output remains generic and contains neither Pi tool names nor Pi model-argument syntax.

  Apply the same target-conditional prose shape to the seven absolute skill-template paths in File structure. Representative exact diff in `/home/hypno/Projects/agentic-workflows/templates/skills/exploring/SKILL.md.tmpl`:

  ```diff
  -Construct one self-contained task. {{ if .targetSubagentTools }}Call `subagent_explore` exactly once with required task, breadth, and detail.{{ else }}Dispatch one target-native fresh-context exploration subagent with task, breadth, detail, boundary, outcome, and report contracts in its brief.{{ end }}
  +Construct one self-contained task. {{ if .targetSubagentTools }}Call `subagent_explore` with required task, breadth, and detail. Independent information needs may be sibling-dispatched; Pi runs at most ten exploration children and queues the rest FIFO. Optionally set exact `model` as `provider/model-id`; omission inherits the parent.{{ else }}Dispatch one target-native fresh-context exploration subagent with task, breadth, detail, boundary, outcome, and report contracts in its brief.{{ end }}
  ```

  Edge exact diff in `/home/hypno/Projects/agentic-workflows/templates/skills/subagent-driven-development/SKILL.md.tmpl`:

  ```diff
  -{{ if .targetSubagentTools }}4. **Per task, call `subagent_implement` alone in its parent tool batch.** Put the complete fresh-context task in `task`, set `allowCommits` explicitly according to whether this task is authorized to create its planned commit, and wait for it before any other tool call or delegation. Never dispatch tasks in parallel. Bake these conventions into `task` verbatim:
  +{{ if .targetSubagentTools }}4. **Per task, call `subagent_implement` alone in its parent tool batch.** Put the complete fresh-context task in `task`, set `allowCommits` explicitly, optionally set exact `model` as `provider/model-id` (omission inherits the parent), and wait for it before any other tool call or delegation. Never dispatch implementation tasks in parallel. Bake these conventions into `task` verbatim:
  ```

  The other five sites receive only the same Pi-conditional optional-model sentence beside their existing call instruction; their non-Pi `else` text is byte-unchanged. The affected-site command is `rg -l 'subagent_(grounding|explore|review|implement)' templates/skills | sort`; its output must equal the seven paths in File structure. Post-check: `go test ./internal/project -run 'Test(PiStructuredExplorationContract|CrossRuntimeExplorationDispatch|BoundedExplorationReporting|PiSubagentToolBoundaries|PiImplementationBatchExclusivity)'` passes.

- [ ] **Task 1.6: Update authored documentation and deferred candidates.** Make these exact content changes:
  - `/home/hypno/Projects/agentic-workflows/templates/agents-doc/AGENTS.md.tmpl`: state that independent fresh-context exploration may run concurrently where supported, refinement stays sequential, lower-cost child models may be selected deliberately, and shared-checkout implementation stays alone;
  - `/home/hypno/Projects/agentic-workflows/.awf/parts/agents-doc/identity.md`: describe the four Pi tools as optionally role-routed and exploration as bounded-parallel without changing the language-agnostic product identity;
  - `/home/hypno/Projects/agentic-workflows/templates/docs/working-with-awf.md.tmpl`: document optional canonical `model` on all four schemas, strict registry/auth rejection with no fallback, inherited default and thinking, ten-active FIFO/abort behavior, and whole-batch implementation blocking plus narrow malformed fallback;
  - `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/architecture/components.md`: describe resolver-before-queue flow, the session-local exploration limiter, and leaf-correlated tool-call preflight;
  - `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/testing/layout.md`: name model-routing, limiter lifecycle, real event-seam, and cross-target publication-safety coverage;
  - `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/releasing/content.md`: add real-Pi smoke cases for one explicit lower-cost child, more than ten exploration calls proving queue progress, and a rejected mixed implementation batch;
  - `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/roadmap/ideas.md`: replace "No other roadmap ideas are recorded" with separate concise candidates for a session dashboard, a fresh-session handoff command, and phase-sensitive tool activation; do not list model routing;
  - `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/rendering/current-state.md`: append this exact current-state paragraph after the ADR-0132 paragraph: `ADR-0141 gives every Pi subagent role strict optional per-call model routing, bounds independent exploration at ten active FIFO-scheduled children with abort-aware queueing, and blocks reconstructable mixed implementation batches at the current-leaf tool-call preflight seam.`;
  - `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/tooling/current-state.md`: append this exact sentence to the Pi gate paragraph: `The runtime lane covers exact model routing, limiter lifecycle including setup failure, and whole-batch enforcement through an in-memory Pi session.`;
  - `/home/hypno/Projects/agentic-workflows/README.md` and `/home/hypno/Projects/agentic-workflows/changelog/CHANGELOG.md`: summarize the optional all-role model field, ten-active exploration queue, and mechanically blocked implementation batches.

  Run `./x sync`; expect it to finish cleanly and update only derived outputs consistent with the authored inputs. Run `./x check`; expect `awf check: clean`. Run `rg -n 'session dashboard|fresh-session handoff|phase-sensitive tool' .awf/docs/parts/roadmap/ideas.md docs/roadmap.md`; expect each candidate in both authored and rendered roadmap surfaces.

- [ ] **Task 1.7: Apply ADR-0141's claim transaction and lifecycle flips.** Edit only authored topic parts:
  - in `/home/hypno/Projects/agentic-workflows/.awf/topics/parts/rendering/templates/current-state.md`, replace only the canonical prose/provenance of `bounded-exploration-reporting` with: `The rendered exploration guidance and Pi's fixed prompt define adaptive breadth and grounded reporting, keep refinement sequential, permit independent information needs to run concurrently, and make Pi queue above ten active children in FIFO and abort-aware order.` Keep `Origin: ADR-0132`, add `Revised-by: ADR-0141`, and keep `Backing: test`;
  - replace only the canonical prose/provenance of `pi-structured-exploration-contract` with: `The generated Pi extension exposes exactly four closed-schema roles, each with optional exact model routing; exploration retains required task, breadth, and detail and runs through the ten-active FIFO limiter without changing the other process boundaries.` Keep `Origin: ADR-0132`, add `Revised-by: ADR-0141`, and keep `Backing: test`;
  - add `pi-subagent-model-routing` with exact prose `Every Pi subagent role accepts an optional exact provider/model-id, inherits the parent on omission, rejects unknown or unauthenticated explicit choices without fallback before queueing, inherits thinking for child clamping, and reports requested and actual models.`, followed by `Origin: ADR-0141` and `Backing: test`;
  - add `pi-implementation-batch-exclusivity` with exact prose `Pi correlates each tool preflight with the current leaf assistant tool-call id, blocks every member of a reconstructable batch that mixes implementation with siblings, and blocks only implementation when trustworthy batch context is unavailable.`, followed by `Origin: ADR-0141` and `Backing: test`;
  - in `/home/hypno/Projects/agentic-workflows/.awf/topics/parts/rendering/catalog-and-targets/current-state.md`, replace only `pi-child-tool-boundaries` prose with: `Pi subagent children use an explicitly selected validated model or inherit the parent, inherit the parent's thinking level, receive fixed role allowlists excluding extension tools, and enforce fixed retained-output limits with explicit truncation diagnostics.` Keep `Origin: ADR-0123`, add `Revised-by: ADR-0141`, and keep `Backing: test`.

  Before freezing, append either concrete implementation deviations or the exact line `- No implementation deviations from ADR-0141 or this plan.` under this plan's Notes. Then flip ADR-0141 and this plan from `Proposed` to `Implemented`. For the ADR, first stage the complete behavior and claim transaction with a provisional terminal history entry, run `./x check --staged`, and use its reported expected `content-sha256` and `state-sequence` in the append-only Implemented line. Run `./x sync` again and stage every regenerated output, including `/home/hypno/Projects/agentic-workflows/docs/decisions/INDEX.md` and `/home/hypno/Projects/agentic-workflows/.awf/awf.lock`.

- [ ] **Task 1.8: Verify and commit the atomic transaction.** Run `git diff --check`; expect no output. Run `./x check`; expect clean drift. Stage the explicit authored paths listed in File structure plus every path printed as changed by `./x sync`; do not use `git add -A`. Run `./x check --staged`; expect a clean ADR-0141 five-operation transition with matching proofs and no uncovered staged path. Run `./x gate`; expect all Go, Pi-extension, coverage, lint, dead-code, pin, and prose gates to pass. Inspect `git diff --cached --name-status` and confirm every staged path belongs to this plan and every generated path has an authored cause. Commit:

```commit
feat(rendering): route and govern Pi subagents (implements 0141)
```

## Verification

After the commit, run `git status --short`; expect no output. Run `./x check`; expect `awf check: clean`. Run `./x gate`; expect the full normal gate to pass. Query `./x topic rendering/templates:pi-subagent-model-routing` and `./x topic rendering/templates:pi-implementation-batch-exclusivity`; expect both Implemented claims with ADR-0141 provenance and test backing. Inspect `/home/hypno/Projects/agentic-workflows/docs/decisions/INDEX.md`; ADR-0141 must be in History rather than In flight.

Then invoke `awf-reviewing-impl` against the implementation commit and finish with `awf-retrospective` after review findings are resolved.

## Notes

The session dashboard, fresh-session handoff command, and phase-sensitive tool activation remain roadmap candidates. The fixed concurrency ceiling is ten and is not a new config key. Explicit model selection is per call and Pi-only; omission preserves current parent-model behavior.
