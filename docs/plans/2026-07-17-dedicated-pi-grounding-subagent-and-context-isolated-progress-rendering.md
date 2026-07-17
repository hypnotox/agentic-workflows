---
date: 2026-07-17
adrs: [125]
status: Proposed
---
# Plan: Dedicated Pi Grounding Subagent and Context-Isolated Progress Rendering

## Goal

Implement [ADR-0125](../decisions/0125-dedicated-pi-grounding-subagent-and-context-isolated-progress-rendering.md): add the dedicated Pi grounding tool and show bounded live child activity inline for every awf subagent tool without adding intermediate activity to parent model context.

Non-goals: OS-level filesystem sandboxing, token-by-token child prose, full child transcript retention, widgets or standalone session entries, parallel implementation, and changes to non-Pi runtime dispatch semantics.

## Architecture summary

First replace the runner's text summaries with the closed `DisplayEvent` model and make post-start failures return structured outcomes. Thread that model through the existing three tool wrappers and use Pi's `tool_result` middleware to restore error signaling without losing details. Then add the fourth grounding role and one shared call/result renderer for all four tools. Finally bind Pi brainstorming to grounding, strengthen all-target and invariant-ledger tests, update authored documentation, sync every generated copy, and flip ADR-0125 and this plan to Implemented.

Phases 1 and 2 are ordered: the renderer consumes Phase 1's event and failure-detail contract. Phase 3 depends on the fourth tool existing. Every phase is independently gated and committed.

## File structure

- **Created:** none beyond this plan.
- **Modified:**
  - `templates/pi/awf-subagents/runner.ts.tmpl`: structured retained events, omission accounting, and structured child failure outcomes.
  - `templates/pi/awf-subagents/index.ts.tmpl`: failure middleware, dedicated grounding tool, context-isolated update details, and shared TUI renderers.
  - `tools/pi-extension-test/tests/runner.test.ts`: event/order/bounds and failure-result tests.
  - `tools/pi-extension-test/tests/index.test.ts`: four-tool, grounding, middleware, context-isolation, and renderer tests.
  - `tools/pi-extension-test/fixtures/fake-pi.mjs`: tool start/end plus assistant completion fixture events.
  - `tools/pi-extension-test/package.json` and `package-lock.json`: direct minimum-version Pi TUI test dependency.
  - `templates/skills/brainstorming/SKILL.md.tmpl`: Pi grounding dispatch.
  - `internal/project/target_test.go`: revised ADR-0123 proof markers, six ADR-0125 proof markers, all-target dispatch and source-contract checks.
  - `.awf/parts/agents-doc/identity.md`, `.awf/docs/parts/architecture/components.md`, `.awf/docs/parts/testing/layout.md`, `.awf/domains/parts/rendering/current-state.md`, `.awf/domains/parts/tooling/current-state.md`: repository-owned current-state sources.
  - `templates/docs/working-with-awf.md.tmpl`, `README.md`, and `changelog/CHANGELOG.md`: adopter-facing contract and release prose.
  - `docs/decisions/0125-dedicated-pi-grounding-subagent-and-context-isolated-progress-rendering.md` and this plan: final lifecycle flips.
  - Generated outputs from `./x sync`: `.pi/extensions/awf-subagents/{index.ts,runner.ts}`, `.pi/skills/awf-brainstorming/SKILL.md`, `AGENTS.md`, `docs/{architecture,testing,working-with-awf}.md`, `docs/decisions/ACTIVE.md`, `docs/domains/{rendering,tooling}.md`, `.awf/awf.lock`, and the corresponding Sundial files `examples/sundial/.pi/extensions/awf-subagents/{index.ts,runner.ts}`, `examples/sundial/.pi/skills/sundial-brainstorming/SKILL.md`, `examples/sundial/docs/working-with-awf.md`, and `examples/sundial/.awf/awf.lock`.
- **Deleted:** none.

## Phase 1: Structured progress and failure preservation

- [ ] **Task 1.1: Write runner regression tests first.** In `tools/pi-extension-test/tests/runner.test.ts`, replace summary assertions with the exact ADR-0125 event contract before changing production code:
  - import `MAX_DISPLAY_EVENTS` and `MAX_DISPLAY_EVENT_BYTES` instead of `MAX_SUMMARIES`;
  - add JSON helpers for `tool_execution_start` and `tool_execution_end` carrying `toolCallId`, `toolName`, arguments, and `isError`;
  - change `runner streams fragmented JSON, summaries, usage, and cleans up` to `runner streams ordered display events, usage, and cleans up` and assert this exact ordered projection:

    ```ts
    [
      { sequence: 1, kind: "tool-start", toolCallId: "call-1", toolName: "read", argsPreview: "{}" },
      { sequence: 2, kind: "tool-end", toolCallId: "call-1", toolName: "read", isError: false },
      { sequence: 3, kind: "assistant", text: "done" },
    ]
    ```

  - assert every partial update contains only the current `events` window and `omittedEvents`, and sequence numbers follow observation order across fragmented lines;
  - replace the repeated-summary test with 25 oversized tool-start events followed by an assistant event; assert `events.length === MAX_DISPLAY_EVENTS`, `omittedEvents === 6`, retained sequence numbers are the last twenty, every `Buffer.byteLength(JSON.stringify(event), "utf8") <= MAX_DISPLAY_EVENT_BYTES`, and retained oversized fields include `[event truncated]`;
  - assert an unmatched tool completion is retained unchanged rather than synthesized or reordered;
  - change malformed JSON, non-zero exit, `error`/`aborted` stop reason, and post-start abort expectations from rejected promises to `RunResult` values with `failed === true`, bounded `failureMessage`, all progress emitted before failure, and temporary-directory cleanup;
  - retain rejection tests only for pre-abort and spawn/setup errors where no useful child state exists.

  Run `./x pi-test run`; expect a non-zero test result because the current runner still exposes summaries and rejects post-start failures. Do not commit the red state.

- [ ] **Task 1.2: Implement the runner event model.** In `templates/pi/awf-subagents/runner.ts.tmpl`, make these exact public type/constant changes:

  ```ts
  export type Role = "explore" | "review" | "implement";
  export const MAX_DISPLAY_EVENTS = 20;
  export const MAX_DISPLAY_EVENT_BYTES = 2 * 1024;

  export type DisplayEvent =
    | { sequence: number; kind: "assistant"; text: string }
    | { sequence: number; kind: "tool-start"; toolCallId: string; toolName: string; argsPreview: string }
    | { sequence: number; kind: "tool-end"; toolCallId: string; toolName: string; isError: boolean };

  export interface RunUpdate {
    events: readonly DisplayEvent[];
    omittedEvents: number;
  }

  export interface RunResult {
    output: string;
    stderr: string;
    events: readonly DisplayEvent[];
    omittedEvents: number;
    usage: Usage;
    model?: string;
    stopReason?: string;
    failed: boolean;
    failureMessage?: string;
  }
  ```

  Replace `EventSummary`, `addSummary`, and summary constants. Add these implementation rules:
  - `appendDisplayEvent` assigns a strictly increasing sequence, bounds tool-call ID and tool name to 256 UTF-8 bytes, then shrinks the event's variable payload (`text` or `argsPreview`) until the complete serialized event including `\n[event truncated]` is at most `MAX_DISPLAY_EVENT_BYTES`;
  - append to the retained ring, remove oldest entries beyond `MAX_DISPLAY_EVENTS`, and increment `omittedEvents` for every removed event;
  - `message_end` assistant text appends `kind: "assistant"`; `tool_execution_start` appends the start variant using `JSON.stringify(args ?? {})`; `tool_execution_end` appends the completion variant and never retains result content;
  - every processed display event calls `onUpdate({events: [...events], omittedEvents})`; unknown events remain inert;
  - normal completion returns `failed: false`; malformed JSON, non-zero child exit, and stop reasons `error`/`aborted` return `failed: true` with bounded `failureMessage` after terminating when needed, rather than rejecting;
  - spawn errors and pre-start abort remain thrown setup errors; every path preserves listener/timer cleanup and `await deps.rm`.

  Update `tools/pi-extension-test/fixtures/fake-pi.mjs` to emit, in order, one `tool_execution_start`, matching `tool_execution_end`, and the existing assistant `message_end`; Task 1.1's production-fixture test must assert all three retained events.

  Run `./x sync` so both dogfooded runner copies update, then run `./x pi-test run`; runner tests pass, but TypeScript may still fail until Task 1.3 updates the wrapper in the same phase. Do not commit between coupled tasks 1.2 and 1.3.

- [ ] **Task 1.3: Thread details through the existing tools and preserve error state.** In `templates/pi/awf-subagents/index.ts.tmpl`:
  - replace summary-shaped updates/results with one exported `SubagentDetails` shape containing `role`, `task`, `state: "running" | "completed" | "failed" | "aborted"`, `events`, `omittedEvents`, optional bounded `stderr`, usage, model, stop reason, existing role-specific metadata, and private `awfFailure?: true`;
  - make partial `content` exactly `"(running...)"`; place all intermediate events only in partial `details`;
  - make final success `content` exactly `result.output`; keep events, omissions, diagnostics, and usage only in final `details`;
  - for `result.failed`, return bounded `failureMessage` as content and `awfFailure: true` in details without throwing;
  - convert the `allowCommits=false` HEAD-change violation from a throw to the same marked result while preserving `before`, `after`, and `commitVerification`;
  - register one `pi.on("tool_result", ...)` handler. It returns `{isError: true}` only when `event.toolName` is one of the currently registered awf subagent tools and `event.details.awfFailure === true`; all other results return `undefined`.

  In `tools/pi-extension-test/tests/index.test.ts`, update the fake `RunResult` and update callback to the event model. Add assertions that:
  - partial content is only `"(running...)"`, while child activity is present only in `details.events`;
  - final content is only `"child output"`, while events and diagnostics remain details;
  - the registered `tool_result` handler patches marked awf results, ignores unmarked awf results, and ignores marked results for unrelated tools;
  - failed child and commit-policy results return details and become `isError: true` through the handler;
  - setup validation still rejects and the implementation queue still releases after queued abort.

  This test uses the lock-pinned `@earendil-works/pi-coding-agent` 0.80.9 types/runtime and is the minimum-version proof required by ADR-0125.

- [ ] **Task 1.4: Verify and commit the structured boundary.** Run:

  ```sh
  ./x sync
  ./x check
  ./x gate
  ```

  Expect `awf check: clean`, the Pi TAP suite to pass, TypeScript and c8 to report 100% statements/branches/functions/lines, and Go coverage to report `100.0%`. Explicitly stage only the Phase 1 templates, tests, fixture, generated extension copies, and lock files reported by sync. Commit:

  ```commit
  feat(rendering): preserve Pi subagent progress details
  ```

## Phase 2: Dedicated grounding and shared inline rendering

- [ ] **Task 2.1: Add red grounding and renderer tests.** In `tools/pi-extension-test/tests/index.test.ts`:
  - change both exact tool-order assertions to `subagent_grounding`, `subagent_explore`, `subagent_review`, `subagent_implement`;
  - add a grounding execution test asserting the same read-only allowlist as exploration and a fixed system prompt containing the five grounding duties, the exact finding keys, both finding kinds, all three confidence values, and their verified/interpreted/unverified meanings;
  - assert no role allowlist contains any of the four `subagent_` tool names;
  - invoke each registered tool's `renderCall` and `renderResult`, call the returned component's `render(120)`, and assert:
    - calls show role plus a bounded task preview;
    - partial collapsed output shows running state, recent tool/assistant events, and omission count;
    - completed collapsed output shows status, recent events, available usage, and the configured expansion hint text;
    - failed and aborted output use distinct status text and expose no raw ANSI-independent content beyond bounded details;
    - expanded output includes task, retained calls/completions, final Markdown text, present stderr, and usage;
    - absent optional diagnostics/usage render no empty diagnostic or usage headings;
    - fallback rendering handles missing/malformed details without throwing.

  Run `./x pi-test run`; expect failure because the fourth tool and renderers do not exist. Do not commit the red state.

- [ ] **Task 2.2: Add the TUI peer test dependency.** In `tools/pi-extension-test/package.json`, add the direct dev dependency:

  ```json
  "@earendil-works/pi-tui": "0.80.9"
  ```

  Regenerate `tools/pi-extension-test/package-lock.json` without host npm by running the same digest-pinned Node image as the test Dockerfile:

  ```sh
  docker run --rm --user "$(id -u):$(id -g)" -e HOME=/tmp \
    -v "$PWD/tools/pi-extension-test:/work" -w /work \
    --entrypoint npm \
    node:22.22.0-alpine@sha256:e4bf2a82ad0a4037d28035ae71529873c069b13eb0455466ae0bc13363826e34 \
    install --package-lock-only --ignore-scripts
  ```

  Expect exit 0, no `node_modules/` in the checkout, and a lock resolving the direct exact 0.80.9 package with no floated root version. Then run `./x pi-test reset && ./x pi-test run` so the dependency fingerprint rebuilds. This is a test/runtime peer API already supplied by Pi, not a Go binary dependency.

- [ ] **Task 2.3: Register the dedicated grounding role.** In `templates/pi/awf-subagents/runner.ts.tmpl`, extend `Role` to include `"grounding"`. In `templates/pi/awf-subagents/index.ts.tmpl`:
  - export `GROUNDING_TOOLS = EXPLORE_TOOLS`;
  - add a fixed grounding prompt that says no edits or commits, lists premise verification, assumptions/edge cases, altitude, and ADR/invariant fit, and requires exactly:

    ```text
    {kind: open-question | possible-issue, topic, detail, grounding, confidence: verified | interpreted | unverified}
    ```

    with the ADR-0125 confidence semantics;
  - register `subagent_grounding` before exploration with schema `{task: Type.String({minLength: 1})}`, label `Grounding Subagent`, grounding-specific description/snippet/guideline, grounding role, and `GROUNDING_TOOLS`;
  - expand the failure middleware's closed tool-name set to all four tools.

- [ ] **Task 2.4: Implement one renderer shared by all four registrations.** In `templates/pi/awf-subagents/index.ts.tmpl`:
  - import `getMarkdownTheme` and `keyHint` from `@earendil-works/pi-coding-agent`, and `Container`, `Markdown`, `Spacer`, and `Text` from `@earendil-works/pi-tui`;
  - define shared `renderSubagentCall(role, args, theme)` and `renderSubagentResult(role, result, {expanded, isPartial}, theme)` helpers; every registration delegates its `renderCall` and `renderResult` to them;
  - render tool starts as a muted arrow plus tool name and argument preview, tool ends as matched success/error status, and assistant completions as bounded tool output;
  - collapsed output shows only the last ten retained events, prefixes `... N earlier retained events` when appropriate, adds the cumulative omitted count, and uses `keyHint("app.tools.expand", "to expand")` when expansion is useful;
  - expanded output uses `Container`, task/status headings, every retained event, `Markdown` for final successful report content, bounded diagnostics only when present, and formatted turns/tokens/cache/cost/model usage only when available;
  - use `isPartial` plus details state for running display; never derive model-visible content in the renderer; malformed/missing details falls back to bounded text content;
  - keep all lines component-wrapped and theme-derived; do not hardcode Ctrl+O or a theme singleton.

  Complete Task 2.1's assertions and add branch cases until the Pi lane remains at 100% statement/branch/function/line coverage.

- [ ] **Task 2.5: Verify and commit grounding plus rendering.** Run:

  ```sh
  ./x sync
  ./x check
  ./x gate
  ```

  Expect the exact clean/100% outputs from Task 1.4 and four passing public-tool registrations. Stage the Phase 2 source, package lock, tests, generated extension copies, and managed lock files only. Commit:

  ```commit
  feat(rendering): add Pi grounding and progress renderer
  ```

## Phase 3: Bind the Pi workflow and invariant ledger

- [ ] **Task 3.1: Change only Pi's grounding dispatch.** In `templates/skills/brainstorming/SKILL.md.tmpl`, replace the Pi branch's two `subagent_explore` references in step 6 with `subagent_grounding`; leave the non-Pi branch and the complete grounding brief/output instructions unchanged. Do not change `templates/skills/refactor-coupling-audit/SKILL.md.tmpl`, whose large-scope branch must still name `subagent_explore`.

- [ ] **Task 3.2: Replace retired proofs and add ADR-0125 contract proofs.** In `internal/project/target_test.go`:
  - replace `// invariant: pi-subagent-public-contract` on `TestPiSubagentPublicContract` with `// invariant: pi-subagent-four-tool-contract`, rename the test accordingly, and assert each of the four registrations occurs once plus no fifth `name: "subagent_` registration exists;
  - replace `// invariant: pi-explicit-workflow-dispatch` with `// invariant: pi-dedicated-grounding-dispatch` and expand the test across `claude`, `codex`, `copilot`, `cursor`, `gemini`, and `pi`. Pi brainstorming must contain `subagent_grounding`, Pi coupling audit must contain `subagent_explore`, and every non-Pi rendered brainstorming/coupling-audit file must contain no `subagent_` token;
  - add one source-contract Go test per remaining ADR-0125 slug, each carrying exactly one proof marker:
    - `// invariant: pi-subagent-progress-context-isolation`: final/partial content literals and details-event path are distinct, with no `appendEntry`, `appendMessage`, or `sendMessage` in the extension;
    - `// invariant: pi-subagent-progress-rendering`: all four registrations delegate both render hooks and source contains `keyHint("app.tools.expand"`;
    - `// invariant: pi-subagent-failure-details`: source contains the private failure marker and scoped `pi.on("tool_result"` patch;
    - `// invariant: pi-subagent-progress-bounds`: runner contains the event count/byte constants, omitted counter, serialized whole-event byte check, and no raw transcript field;
  - retain ADR-0123 markers for child tool boundaries, process safety, implementation state, minimum runtime, target render, and container gate because ADR-0125 does not retire them.

  Use an exact target table with these skill roots: `.claude/skills`, `.agents/skills`, `.github/skills`, `.cursor/skills`, `.gemini/skills`, and `.pi/skills`. Run:

  ```sh
  go test ./internal/project -run 'TestPi(SubagentFourToolContract|SkillsNameGovernedSubagentTools|SubagentProgress)'
  ./x invariants
  ```

  Expect all selected tests to pass and `awf invariants: clean` (ADR-0125 remains Proposed, but every future Implemented obligation is already backed).

- [ ] **Task 3.3: Verify publication and render fan-out.** Run `./x sync`. Confirm the affected-site set is exactly the Pi brainstorming copies plus generated extension/lock changes already expected:

  ```sh
  rg -n 'subagent_grounding|subagent_explore' \
    .pi/skills/awf-brainstorming/SKILL.md \
    examples/sundial/.pi/skills/sundial-brainstorming/SKILL.md \
    .pi/skills/awf-refactor-coupling-audit/SKILL.md \
    examples/sundial/.pi/skills/sundial-refactor-coupling-audit/SKILL.md
  ```

  Expect brainstorming grounding lines to name only `subagent_grounding` and coupling-audit dispatch lines to name only `subagent_explore`. Run the all-target Go test to prove other targets contain no Pi tool token. Run `./x check`; expect clean and no no-value or unresolved-token finding.

- [ ] **Task 3.4: Gate and commit workflow binding.** Run `./x gate`; expect the standard clean/100% result. Explicitly stage the skill template, Go contract tests, Pi rendered skill copies, generated extension copies if sync refreshed provenance, and both managed lock files. Commit:

  ```commit
  feat(rendering): bind Pi grounding workflow
  ```

## Phase 4: Documentation and lifecycle completion

- [ ] **Task 4.1: Update authored current-state and adopter guidance.** Apply this same contract consistently, with each file's existing voice:
  - `.awf/parts/agents-doc/identity.md`: replace "three generated project-extension tools" with four roles: grounding, exploration, governed review, and sequential implementation; mention context-isolated inline progress.
  - `.awf/docs/parts/architecture/components.md`: list all four tool names and state that structured bounded details drive shared inline rendering while final content alone reaches the parent model.
  - `.awf/docs/parts/testing/layout.md`: extend the Pi-extension paragraph to name renderer, event-bound, failure-middleware, minimum-0.80.9, and context-isolation coverage.
  - `.awf/domains/parts/rendering/current-state.md`: add ADR-0125's four-tool target output, Pi-only grounding dispatch, and generated custom-rendering state.
  - `.awf/domains/parts/tooling/current-state.md`: add the 0.80.9 container lane's event/render/middleware/context-isolation coverage.
  - `templates/docs/working-with-awf.md.tmpl`: change "exactly three" to "exactly four", document `subagent_grounding {task}`, distinguish grounding/exploration policy, and explain collapsed/expanded bounded progress details versus final model-visible content.
  - `README.md`: list all four tools and summarize context-isolated inline progress.
  - `changelog/CHANGELOG.md`: under `[Unreleased]` Features add ADR-0125's dedicated grounding tool and live bounded TUI progress; update the existing ADR-0123 exact-three statements to four without rewriting released history outside `[Unreleased]`.

  Search after edits:

  ```sh
  rg -n 'exactly three|three governed extension tools|Pi receives three|subagent_explore.*, subagent_review.*,.*subagent_implement' \
    .awf templates README.md changelog/CHANGELOG.md
  ```

  Expect zero stale current-contract matches. Historical ADR/plan prose is frozen and excluded from this sweep.

- [ ] **Task 4.2: Sync and inspect every generated copy.** Run `./x sync`, then inspect the exact generated set listed in File structure. Verify:
  - repository and Sundial extension files match their templates apart from provenance;
  - both Pi brainstorming skills name grounding;
  - `AGENTS.md`, `docs/architecture.md`, `docs/testing.md`, and both repository/Sundial `docs/working-with-awf.md` describe four tools and context isolation;
  - domain indexes and current-state prose include ADR-0125;
  - `.awf/awf.lock` and `examples/sundial/.awf/awf.lock` carry the new hashes.

  Run `./x check`; expect `awf check: clean`, `awf invariants: clean`, and clean Sundial checks.

- [ ] **Task 4.3: Flip lifecycle state in the final implementation commit.** Change ADR-0125 and this plan from `status: Proposed` to `status: Implemented`. Do not edit ADR-0123 beyond its already-committed `related: [125]` metadata. Run `./x sync` to regenerate `docs/decisions/ACTIVE.md`, `docs/domains/rendering.md`, and `docs/domains/tooling.md`; there is no `docs/decisions/README.md` index row to add.

- [ ] **Task 4.4: Final verification and commit.** Run:

  ```sh
  ./x sync
  ./x check
  ./x gate
  ./x audit-local
  git status --short
  ```

  Expect sync/check/invariants clean, Go and Pi extension coverage at 100%, no gate errors, and audit-local with no Errors (advisory Warnings remain triage-only). Confirm `git status --short` contains only the explicitly documented Phase 4 files plus any pre-existing unrelated user changes; never stage those unrelated changes. Commit all and only Phase 4 files:

  ```commit
  feat(rendering): complete Pi grounding progress (implements 0125)
  ```

## Verification

After the final commit:

```sh
./x check
./x gate
rg -n 'name: "subagent_(grounding|explore|review|implement)"' .pi/extensions/awf-subagents/index.ts
rg -n 'subagent_(grounding|explore)' .pi/skills/awf-{brainstorming,refactor-coupling-audit}/SKILL.md
```

Acceptance criteria:
- check and gate are clean with 100% Go and Pi-extension coverage;
- the registration search returns exactly one line for each of the four tools and no other public subagent registration;
- brainstorming names grounding and coupling audit names exploration;
- renderer tests prove running, completed, failed, aborted, collapsed, expanded, optional-field, and malformed-detail states;
- intermediate events occur only in details, final content contains only the report/failure summary, and marked failures become Pi error results through middleware;
- ADR-0125 and this plan are Implemented, ACTIVE.md/domain indexes are regenerated, and unrelated checkout changes remain untouched.

## Notes

The checkout contained unrelated migration/parser work before this effort. Every phase stages exact paths and must preserve those changes. Implementation should run inline with `awf-executing-plans`: Phases 1-4 are ordered and repeatedly touch the same two extension templates, so fresh subagent-per-task dispatch would add conflict and handoff risk rather than useful isolation.
