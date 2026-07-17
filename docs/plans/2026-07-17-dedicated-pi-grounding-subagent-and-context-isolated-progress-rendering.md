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

Phase 1 replaces text summaries with the closed `DisplayEvent` model, preserves bounded details across post-start failures, and proves Pi 0.80.9's partial-update/result-middleware behavior through a real in-memory `AgentSession`. Phase 2 adds the grounding role, one shared renderer, Pi-only workflow binding, all six successor invariant proofs, and the user-facing documentation in the same behavior commit. Phase 3 records deviations, freezes the ADR and plan, regenerates indexes, and runs final review checks.

Phases are ordered and each closes with a separately green gate. All commands run from `/home/hypno/Projects/agentic-workflows`. File paths in prose are absolute; command-local paths may be repository-relative only after an explicit `cd /home/hypno/Projects/agentic-workflows`.

## File structure

- **Created:** `/home/hypno/Projects/agentic-workflows/tools/pi-extension-test/tests/runtime.test.ts`.
- **Modified:**
  - `/home/hypno/Projects/agentic-workflows/templates/pi/awf-subagents/runner.ts.tmpl`
  - `/home/hypno/Projects/agentic-workflows/templates/pi/awf-subagents/index.ts.tmpl`
  - `/home/hypno/Projects/agentic-workflows/tools/pi-extension-test/tests/runner.test.ts`
  - `/home/hypno/Projects/agentic-workflows/tools/pi-extension-test/tests/index.test.ts`
  - `/home/hypno/Projects/agentic-workflows/tools/pi-extension-test/fixtures/fake-pi.mjs`
  - `/home/hypno/Projects/agentic-workflows/tools/pi-extension-test/package.json`
  - `/home/hypno/Projects/agentic-workflows/tools/pi-extension-test/package-lock.json`
  - `/home/hypno/Projects/agentic-workflows/templates/skills/brainstorming/SKILL.md.tmpl`
  - `/home/hypno/Projects/agentic-workflows/internal/project/target_test.go`
  - `/home/hypno/Projects/agentic-workflows/.awf/parts/agents-doc/identity.md`
  - `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/architecture/components.md`
  - `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/architecture/dependencies.md`
  - `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/testing/layout.md`
  - `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/rendering/current-state.md`
  - `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/tooling/current-state.md`
  - `/home/hypno/Projects/agentic-workflows/templates/docs/working-with-awf.md.tmpl`
  - `/home/hypno/Projects/agentic-workflows/README.md`
  - `/home/hypno/Projects/agentic-workflows/changelog/CHANGELOG.md`
  - `/home/hypno/Projects/agentic-workflows/docs/decisions/0125-dedicated-pi-grounding-subagent-and-context-isolated-progress-rendering.md`
  - `/home/hypno/Projects/agentic-workflows/docs/plans/2026-07-17-dedicated-pi-grounding-subagent-and-context-isolated-progress-rendering.md`
  - generated repository and Sundial copies enumerated in each phase's staging allowlist.
- **Deleted:** none.

## Phase 1: Structured progress and failure preservation

- [ ] **Task 1.1: Snapshot and protect the unrelated dirty baseline.** From `/home/hypno/Projects/agentic-workflows`, establish this exact pre-existing unstaged set:

  ```text
  internal/adr/adr.go
  internal/adr/adr_test.go
  internal/migrate/pitfalls.go
  internal/migrate/pitfalls_test.go
  internal/migrate/retirementtokens_test.go
  ```

  Run:

  ```sh
  cd /home/hypno/Projects/agentic-workflows
  test -z "$(git diff --cached --name-only)"
  printf '%s\n' \
    internal/adr/adr.go \
    internal/adr/adr_test.go \
    internal/migrate/pitfalls.go \
    internal/migrate/pitfalls_test.go \
    internal/migrate/retirementtokens_test.go > /tmp/awf-0125-unrelated.paths
  git diff -- $(cat /tmp/awf-0125-unrelated.paths) > /tmp/awf-0125-unrelated.patch
  git diff --name-only -- $(cat /tmp/awf-0125-unrelated.paths) | sort > /tmp/awf-0125-unrelated.actual
  cmp /tmp/awf-0125-unrelated.paths /tmp/awf-0125-unrelated.actual
  ./x gate
  ```

  Expect both `test`/`cmp` commands and the gate to exit 0. Before and after every phase commit, rerun `git diff -- $(cat /tmp/awf-0125-unrelated.paths) | cmp - /tmp/awf-0125-unrelated.patch`; any mismatch is a stop condition requiring user triage, not a change to stage.

- [ ] **Task 1.2: Write failing runner tests before production changes.** Modify `/home/hypno/Projects/agentic-workflows/tools/pi-extension-test/tests/runner.test.ts` with these exact test contracts:
  - replace `MAX_SUMMARIES` imports/assertions with `MAX_DISPLAY_EVENTS` and `MAX_DISPLAY_EVENT_BYTES`;
  - add `toolStart(id, name, args)` and `toolEnd(id, name, isError)` JSON helpers;
  - replace the summary-stream test's event writes and expected projection with:

    ```ts
    h.process.stdout.write(toolStart("call-1", "read", {}) + "\n");
    h.process.stdout.write(toolEnd("call-1", "read", false) + "\n");
    // then write the existing fragmented assistant message

    assert.deepEqual(result.events, [
      { sequence: 1, kind: "tool-start", toolCallId: "call-1", toolName: "read", argsPreview: "{}" },
      { sequence: 2, kind: "tool-end", toolCallId: "call-1", toolName: "read", isError: false },
      { sequence: 3, kind: "assistant", text: "done" },
    ]);
    assert.equal(result.omittedEvents, 0);
    ```

  - add `runner preserves unmatched completions in observation order`: emit an end before its start and assert sequence 1 remains the end and sequence 2 remains the start;
  - add `runner bounds every event field and counts omissions`: emit oversized assistant text, tool-call ID, tool name, and start args. After each oversized event, assert its marker from the latest captured partial-update snapshot: ID truncation contains `[toolCallId truncated]`, name truncation contains `[toolName truncated]`, and payload truncation contains `[event truncated]`. Then emit 25 further starts and one end. Assert every complete event in every captured update is at most 2,048 UTF-8 bytes, the final retained count is 20, and `omittedEvents` equals total appended minus 20;
  - change malformed JSON, non-zero exit, model stop reason `error`, model stop reason `aborted`, and post-start signal abort tests to resolved results with the state matrix in Task 1.3;
  - add `runner cleans setup failures`: injected `writeFile` rejection and process `error` before any JSON both reject and each call `rm` exactly once. Retain pre-abort rejection and assert it creates no temporary directory.

  Run `cd /home/hypno/Projects/agentic-workflows && ./x pi-test run`; expect non-zero with failures naming missing `events`, `omittedEvents`, and `failed` fields. Do not commit the red state.

- [ ] **Task 1.3: Replace runner summaries with the exact event/result model.** In `/home/hypno/Projects/agentic-workflows/templates/pi/awf-subagents/runner.ts.tmpl`, replace the old declarations with:

  ```ts
  export const MAX_DISPLAY_EVENTS = 20;
  export const MAX_DISPLAY_EVENT_BYTES = 2 * 1024;
  export const MAX_FAILURE_BYTES = 2 * 1024;

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

  Replace `EventSummary`, `addSummary`, and summary constants with these named helpers and exact semantics:
  - `truncateField(value, maximum, marker)` returns valid UTF-8, includes its supplied marker inside the maximum, and is used with 256-byte maxima and markers `[toolCallId truncated]` and `[toolName truncated]`;
  - `fitDisplayEvent(event)` first truncates ID/name, then shrinks only `text` or `argsPreview` until `Buffer.byteLength(JSON.stringify(event), "utf8") <= MAX_DISPLAY_EVENT_BYTES`; a changed payload ends in `[event truncated]`, included inside 2,048 bytes;
  - `appendDisplayEvent(eventWithoutSequence)` assigns a monotonic sequence in JSON observation order, retains only the last 20, and increments a cumulative omission counter for every removed event;
  - `failure(message)` returns valid UTF-8 capped at `MAX_FAILURE_BYTES` including `[failure truncated]`.

  Process only these variants: assistant `message_end` adds assistant text; `tool_execution_start` adds ID/name/`JSON.stringify(args ?? {})`; `tool_execution_end` adds ID/name/`Boolean(isError)` and no result. Unknown events stay inert. Every appended event calls `onUpdate({events: [...events], omittedEvents})`.

  Implement this exact state matrix at the runner boundary:

  | Condition | Promise | `failed` | `stopReason` | failure content |
  |---|---|---:|---|---|
  | normal close 0 | resolves | false | child value | absent |
  | malformed JSON after spawn | resolves after TERM | true | `error` | bounded malformed-event diagnostic |
  | non-zero/null exit | resolves | true | `error` | bounded exit plus retained stderr diagnostic |
  | child stop reason `error` | resolves | true | `error` | bounded child output |
  | child stop reason `aborted` | resolves | true | `aborted` | bounded child output |
  | parent signal after spawn | resolves after TERM/KILL | true | `aborted` | `Subagent was aborted` |
  | pre-aborted signal | rejects | n/a | n/a | no child state |
  | temp write or spawn error before JSON | rejects | n/a | n/a | no useful child state |

  Every created temporary directory is removed once; timers/listeners are removed on every resolved/rejected path.

  Modify `/home/hypno/Projects/agentic-workflows/tools/pi-extension-test/fixtures/fake-pi.mjs` to emit start `call-1/read/{}`, matching successful end, then the existing assistant completion. The production-fixture test must assert all three events.

- [ ] **Task 1.4: Preserve details and error state through the existing three wrappers.** In `/home/hypno/Projects/agentic-workflows/templates/pi/awf-subagents/index.ts.tmpl`, replace summary-shaped details with:

  ```ts
  export type SubagentState = "running" | "completed" | "failed" | "aborted";
  export interface SubagentDetails {
    role: RunRequest["role"];
    task: string;
    state: SubagentState;
    events: readonly DisplayEvent[];
    omittedEvents: number;
    stderr?: string;
    usage?: Usage;
    model?: string;
    stopReason?: string;
    awfFailure?: true;
    [roleSpecific: string]: unknown;
  }
  ```

  Use this wrapper state mapping: successful result is completed; failed result with stop reason `aborted` is aborted; every other failed result and commit-policy violation is failed. Partial updates always return content `"(running...)"` and details only; success returns only `result.output` as content; failure returns only `result.failureMessage` as content and sets `awfFailure: true`. Convert the `allowCommits=false` HEAD change from throw to a marked failed result preserving before/after evidence.

  Register exactly one `pi.on("tool_result", ...)` middleware. It returns `{isError: true}` only when the tool name is in the closed awf subagent-name set and `event.details.awfFailure === true`; all other events return `undefined`.

  In `/home/hypno/Projects/agentic-workflows/tools/pi-extension-test/tests/index.test.ts`, update fixtures and assert partial content never contains child text, final content never contains display events/stderr, marked failures preserve details, middleware marks only eligible results, and queue release still works.

- [ ] **Task 1.5: Add the real Pi 0.80.9 runtime test.** Create `/home/hypno/Projects/agentic-workflows/tools/pi-extension-test/tests/runtime.test.ts`. Use only lock-pinned imports from `@earendil-works/pi-ai`, `@earendil-works/pi-coding-agent`, `typebox`, and Node. The test must:
  1. create an in-memory `DefaultResourceLoader` extension factory registering a `runtime_probe` tool whose execute callback emits one partial result with `details.events`, then returns final content/details carrying `awfFailure: true`;
  2. register the same narrowly scoped `tool_result` patch to `isError: true`;
  3. create an in-memory `AgentSession` via `createAgentSession` with only `runtime_probe`, then replace `session.agent.streamFn` with a deterministic two-turn `createAssistantMessageEventStream`: first turn emits an assistant tool call for `runtime_probe`, second turn emits final assistant text;
  4. subscribe to actual `tool_execution_update` and `tool_execution_end` session events;
  5. call `session.prompt("run probe")`, then assert the update preserved partial details, the end event preserved final content/details and has `isError === true`, and the stored parent tool-result message contains only final content in its model-visible content field;
  6. dispose the session in `finally`.

  This is the minimum-runtime proof; a direct call to a registered handler is not sufficient.

- [ ] **Task 1.6: Travel technical documentation with Phase 1.** Apply one documentation batch:
  - representative `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/rendering/current-state.md`: after ADR-0123's extension paragraph, add that the runner retains the last 20 structured assistant/tool lifecycle events with whole-event 2 KiB limits, cumulative omissions, and context-isolated details;
  - edge `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/tooling/current-state.md`: extend the Pi lane paragraph to name real 0.80.9 runtime coverage for partial details and result middleware;
  - `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/testing/layout.md`: extend the Pi-extension test paragraph with event ordering/bounds, setup cleanup, and runtime-middleware coverage;
  - `/home/hypno/Projects/agentic-workflows/changelog/CHANGELOG.md`: under `[Unreleased]` Bug fixes, add that Pi child failures now retain bounded progress/diagnostics while preserving error status.

  Post-check:

  ```sh
  cd /home/hypno/Projects/agentic-workflows
  rg -n '20 structured|partial details|runtime middleware|retain bounded progress' \
    .awf/domains/parts/rendering/current-state.md \
    .awf/domains/parts/tooling/current-state.md \
    .awf/docs/parts/testing/layout.md \
    changelog/CHANGELOG.md
  ```

  Expect one current-state match in each file.

- [ ] **Task 1.7: Sync, width-independent gate, stage from an allowlist, and commit.** Run `cd /home/hypno/Projects/agentic-workflows && ./x sync && ./x check && ./x gate`. Expect clean checks and 100% Go and Pi statement/branch/function/line coverage.

  Create and apply this literal exhaustive Phase 1 allowlist:

  ```sh
  cat > /tmp/awf-0125-phase1.allow <<'EOF'
  .awf/awf.lock
  .awf/docs/parts/testing/layout.md
  .awf/domains/parts/rendering/current-state.md
  .awf/domains/parts/tooling/current-state.md
  .pi/extensions/awf-subagents/index.ts
  .pi/extensions/awf-subagents/runner.ts
  changelog/CHANGELOG.md
  docs/domains/rendering.md
  docs/domains/tooling.md
  docs/testing.md
  examples/sundial/.awf/awf.lock
  examples/sundial/.pi/extensions/awf-subagents/index.ts
  examples/sundial/.pi/extensions/awf-subagents/runner.ts
  templates/pi/awf-subagents/index.ts.tmpl
  templates/pi/awf-subagents/runner.ts.tmpl
  tools/pi-extension-test/fixtures/fake-pi.mjs
  tools/pi-extension-test/tests/index.test.ts
  tools/pi-extension-test/tests/runner.test.ts
  tools/pi-extension-test/tests/runtime.test.ts
  EOF
  sed -i 's/^  //' /tmp/awf-0125-phase1.allow
  sort -o /tmp/awf-0125-phase1.allow /tmp/awf-0125-phase1.allow
  while IFS= read -r path; do git add -- "/home/hypno/Projects/agentic-workflows/$path"; done < /tmp/awf-0125-phase1.allow
  git diff --cached --name-only | sort > /tmp/awf-0125-phase1.actual
  cmp /tmp/awf-0125-phase1.allow /tmp/awf-0125-phase1.actual
  git diff -- $(cat /tmp/awf-0125-unrelated.paths) | cmp - /tmp/awf-0125-unrelated.patch
  ```

  Commit:

  ```commit
  feat(rendering): preserve Pi subagent progress details
  ```

## Phase 2: Grounding, inline rendering, and Pi workflow binding

- [ ] **Task 2.1: Add direct Pi TUI dependency without host npm.** In `/home/hypno/Projects/agentic-workflows/tools/pi-extension-test/package.json`, add `"@earendil-works/pi-tui": "0.80.9"` to `devDependencies`. Regenerate `/home/hypno/Projects/agentic-workflows/tools/pi-extension-test/package-lock.json` with:

  ```sh
  cd /home/hypno/Projects/agentic-workflows
  docker run --rm --user "$(id -u):$(id -g)" -e HOME=/tmp \
    -v "$PWD/tools/pi-extension-test:/work" -w /work --entrypoint npm \
    node:22.22.0-alpine@sha256:e4bf2a82ad0a4037d28035ae71529873c069b13eb0455466ae0bc13363826e34 \
    install --package-lock-only --ignore-scripts
  test ! -d tools/pi-extension-test/node_modules
  ./x pi-test reset
  ```

  Expect exit 0 and a root lock entry resolving exactly 0.80.9. Do not run the red suite until Task 2.2 has installed its failing tests.

- [ ] **Task 2.2: Write failing grounding, schema, renderer, and width tests.** In `/home/hypno/Projects/agentic-workflows/tools/pi-extension-test/tests/index.test.ts`:
  - change exact registration order to grounding, explore, review, implement;
  - validate grounding parameters with TypeBox's compiled/check seam: missing task, empty task, and additional property fail; `{task: "ground"}` passes; schema has `minLength: 1` and `additionalProperties: false`;
  - execute grounding and assert role `grounding`, exact read-only tools, all grounding duties, exact finding keys/kinds/confidences, all three definitions (verified is mechanically confirmed against source, interpreted requires judgment, and unverified could not be confirmed), and no public subagent name in any child allowlist;
  - add shared renderer cases for each role and running/completed/failed/aborted/missing-details states.

  Renderer assertions use `visibleWidth` from `@earendil-works/pi-tui`. Render representative collapsed, expanded, malformed, and oversized states at widths 24 and 120; for every line assert `visibleWidth(line) <= width`. Also assert configured expansion hint text, omission count, recent-event collapse, task, final Markdown text, present-only stderr/usage, and fallback behavior.

  In `/home/hypno/Projects/agentic-workflows/internal/project/target_test.go`, move the public-contract marker now, before the gate that sees four tools: rename `TestPiSubagentPublicContract` to `TestPiSubagentFourToolContract`, replace its marker with `// invariant: pi-subagent-four-tool-contract`, assert exactly four registrations, and inspect each registration block for its exact TypeBox schema and role mapping. Run `./x pi-test run`; expect non-zero only for missing grounding/renderer behavior. Do not commit red state.

- [ ] **Task 2.3: Add grounding and shared renderers.** Modify `/home/hypno/Projects/agentic-workflows/templates/pi/awf-subagents/runner.ts.tmpl` so `Role` is exactly `"grounding" | "explore" | "review" | "implement"`. In `/home/hypno/Projects/agentic-workflows/templates/pi/awf-subagents/index.ts.tmpl`:
  - export `GROUNDING_TOOLS = EXPLORE_TOOLS`;
  - register `subagent_grounding` before exploration with required non-empty task schema and no additional properties;
  - use a fixed no-mutation grounding prompt containing premise verification, assumptions/edge cases, altitude, ADR/invariant fit, and exact ADR-0125 finding/confidence schema;
  - include grounding in failure middleware and never include any public subagent name in a child allowlist.

  Add exact display constants:

  ```ts
  const COLLAPSED_EVENT_COUNT = 10;
  const MAX_TASK_PREVIEW_BYTES = 512;
  const MAX_FALLBACK_BYTES = 2 * 1024;
  const TASK_TRUNCATION = "[task truncated]";
  const DISPLAY_TRUNCATION = "[display truncated]";
  ```

  Import `getMarkdownTheme` and `keyHint` from coding-agent and `Container`, `Markdown`, `Spacer`, `Text`, `truncateToWidth`, and `wrapTextWithAnsi` from Pi TUI. Every tool delegates to shared `renderSubagentCall` and `renderSubagentResult`.

  State/render matrix:

  | State | Collapsed | Expanded |
  |---|---|---|
  | running | role, running state, last 10 events, omission state, available usage | task, all retained events, present diagnostics, available usage |
  | completed | role, success state, last 10 events, omission state, available usage, key hint | task, all events, Markdown final report, present diagnostics, available usage |
  | failed | role, failed state, last 10 events, omission state, available usage | task, all events, bounded final failure, present diagnostics, available usage |
  | aborted | role, aborted state, last 10 events, omission state, available usage | task, all events, bounded abort message, present diagnostics, available usage |
  | malformed/missing details | role plus bounded fallback content | role plus the same bounded fallback content |

  Task previews are valid UTF-8 capped at 512 bytes including `[task truncated]`; fallback text is capped at 2 KiB including `[display truncated]`; stderr is already runner-capped at 50 KiB with its existing marker; final output remains capped at 50 KiB/2,000 lines. All renderer lines use TUI wrapping/truncation so visible width never exceeds the supplied width. The expansion hint is `keyHint("app.tools.expand", "to expand")`, never literal Ctrl+O.

- [ ] **Task 2.4: Bind Pi workflow and all successor proofs in the same gateable phase.** In `/home/hypno/Projects/agentic-workflows/templates/skills/brainstorming/SKILL.md.tmpl`, change only the Pi branch's step-6 tool name from `subagent_explore` to `subagent_grounding`; leave the full brief and non-Pi branch unchanged. Leave `/home/hypno/Projects/agentic-workflows/templates/skills/refactor-coupling-audit/SKILL.md.tmpl` on exploration.

  In `/home/hypno/Projects/agentic-workflows/internal/project/target_test.go`, add these exact test names and one opening proof marker each:
  - `TestPiSubagentFourToolContract`: `pi-subagent-four-tool-contract`;
  - `TestPiDedicatedGroundingDispatch`: `pi-dedicated-grounding-dispatch`, covering Pi grounding/exploration and absence of `subagent_` in Claude, Codex, Copilot, Cursor, Gemini;
  - `TestPiSubagentProgressContextIsolation`: `pi-subagent-progress-context-isolation`, checking partial/final content separation and absence of `appendEntry`, `appendMessage`, and `sendMessage`;
  - `TestPiSubagentProgressRendering`: `pi-subagent-progress-rendering`, checking all four shared render hooks and configurable key hint;
  - `TestPiSubagentFailureDetails`: `pi-subagent-failure-details`, checking private marker and scoped result middleware;
  - `TestPiSubagentProgressBounds`: `pi-subagent-progress-bounds`, checking all event/display constants, omission counter, whole-event bound, and no raw transcript store.

  Remove only retired markers `pi-subagent-public-contract` and `pi-explicit-workflow-dispatch`; retain all other ADR-0123 markers. Run:

  ```sh
  cd /home/hypno/Projects/agentic-workflows
  go test ./internal/project -run '^(TestPiSubagentFourToolContract|TestPiDedicatedGroundingDispatch|TestPiSubagentProgressContextIsolation|TestPiSubagentProgressRendering|TestPiSubagentFailureDetails|TestPiSubagentProgressBounds)$'
  ./x invariants
  ```

  Expect six passing tests and `awf invariants: clean`.

- [ ] **Task 2.5: Travel all four-tool/render/dispatch documentation in this behavior commit.** Apply a documentation batch with these exact old-to-new contracts.

  Representative `/home/hypno/Projects/agentic-workflows/templates/docs/working-with-awf.md.tmpl`:

  ```diff
  -The Pi target requires Pi 0.80.9 or newer and renders executable project-extension code that Pi
  -loads only after project trust. It registers exactly three tools: `subagent_explore` takes a
  -required `task`; `subagent_review` takes required `kind` (`adr`, `plan`, or `code`) and `task`;
  -and `subagent_implement` takes required `task` and `allowCommits`. Exploration and review follow a
  -no-mutation prompt policy and can use `bash`; they are not OS-sandboxed. Implementation shares the
  -parent checkout, must run alone in its parent tool batch and sequentially, and may commit only when
  -the orchestrator sets `allowCommits: true`. Missing or modified extension files are `awf check`
  -drift; run `awf sync` to repair them.
  +The Pi target requires Pi 0.80.9 or newer and renders executable project-extension code that Pi
  +loads only after project trust. It registers exactly four tools: `subagent_grounding` takes a
  +required `task` for the workflow's premise and altitude check; `subagent_explore` takes a required
  +`task` for general investigation; `subagent_review` takes required `kind` (`adr`, `plan`, or
  +`code`) and `task`; and `subagent_implement` takes required `task` and `allowCommits`. Grounding,
  +exploration, and review follow a no-mutation prompt policy and can use `bash`; they are not
  +OS-sandboxed. Implementation shares the parent checkout, must run alone in its parent tool batch
  +and sequentially, and may commit only when the orchestrator sets `allowCommits: true`.
  +
  +All four tools render bounded recent activity inline. The expanded tool view shows the retained
  +task, events, report, present diagnostics, and available usage. Intermediate activity remains in
  +tool details; only the final report or bounded failure summary enters parent model content.
  +Brainstorming uses grounding, while large coupling audits retain exploration. Missing or modified
  +extension files are `awf check` drift; run `awf sync` to repair them.
  ```

  Edge `/home/hypno/Projects/agentic-workflows/.awf/parts/agents-doc/identity.md`:

  ```diff
  -Pi receives three generated project-extension tools that run isolated no-session child processes for exploration, governed review, and sequential implementation;
  +Pi receives four generated project-extension tools (`subagent_grounding`, `subagent_explore`, `subagent_review`, and `subagent_implement`) that run isolated no-session child processes for grounding, exploration, governed review, and sequential implementation, with bounded inline progress kept out of parent model content;
  ```

  Apply the same contract, not placeholder prose, to this exhaustive authored set:
  - `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/architecture/components.md`: four names plus structured details/shared renderer/content boundary;
  - `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/architecture/dependencies.md`: name coding-agent and Pi TUI 0.80.9 as runtime peer APIs and the direct test pin;
  - `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/testing/layout.md`: add renderer state/width/schema coverage;
  - `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/rendering/current-state.md`: add dedicated Pi dispatch and shared renderer;
  - `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/tooling/current-state.md`: add schema, width, context-isolation, and runtime-middleware coverage;
  - `/home/hypno/Projects/agentic-workflows/README.md`: list four tools and context-isolated progress;
  - `/home/hypno/Projects/agentic-workflows/changelog/CHANGELOG.md`: under `[Unreleased]` Features add ADR-0125, and update only `[Unreleased]` ADR-0123 three-tool wording to four.

  Post-check all authored and generated prose after sync with multiline search:

  ```sh
  cd /home/hypno/Projects/agentic-workflows
  ./x sync
  ! rg -U --multiline-dotall -n 'exactly three|three generated project-extension tools|three governed extension tools|subagent_explore.{0,160}subagent_review.{0,160}subagent_implement' \
    .awf/parts/agents-doc/identity.md \
    .awf/docs/parts/architecture/components.md \
    templates/docs/working-with-awf.md.tmpl \
    README.md changelog/CHANGELOG.md AGENTS.md \
    docs/architecture.md docs/working-with-awf.md \
    examples/sundial/docs/working-with-awf.md
  rg -l 'subagent_grounding' \
    templates/docs/working-with-awf.md.tmpl README.md changelog/CHANGELOG.md \
    AGENTS.md docs/architecture.md docs/working-with-awf.md \
    examples/sundial/docs/working-with-awf.md
  ```

  Expect the negative search to exit 0 and the positive search to print every listed path.

- [ ] **Task 2.6: Verify render fan-out, width, publication safety, stage, and commit.** Run:

  ```sh
  cd /home/hypno/Projects/agentic-workflows
  ./x sync
  ./x check
  ./x gate
  rg -n 'subagent_(grounding|explore)' \
    .pi/skills/awf-brainstorming/SKILL.md \
    .pi/skills/awf-refactor-coupling-audit/SKILL.md \
    examples/sundial/.pi/skills/sundial-brainstorming/SKILL.md \
    examples/sundial/.pi/skills/sundial-refactor-coupling-audit/SKILL.md
  git diff -- $(cat /tmp/awf-0125-unrelated.paths) | cmp - /tmp/awf-0125-unrelated.patch
  ```

  Expect grounding only in brainstorming, exploration only in coupling audit, clean checks, no unresolved/no-value finding, and 100% coverage.

  Create and apply this literal exhaustive Phase 2 allowlist:

  ```sh
  cat > /tmp/awf-0125-phase2.allow <<'EOF'
  .awf/awf.lock
  .awf/docs/parts/architecture/components.md
  .awf/docs/parts/architecture/dependencies.md
  .awf/docs/parts/testing/layout.md
  .awf/domains/parts/rendering/current-state.md
  .awf/domains/parts/tooling/current-state.md
  .awf/parts/agents-doc/identity.md
  .pi/extensions/awf-subagents/index.ts
  .pi/extensions/awf-subagents/runner.ts
  .pi/skills/awf-brainstorming/SKILL.md
  AGENTS.md
  README.md
  changelog/CHANGELOG.md
  docs/architecture.md
  docs/domains/rendering.md
  docs/domains/tooling.md
  docs/testing.md
  docs/working-with-awf.md
  examples/sundial/.awf/awf.lock
  examples/sundial/.pi/extensions/awf-subagents/index.ts
  examples/sundial/.pi/extensions/awf-subagents/runner.ts
  examples/sundial/.pi/skills/sundial-brainstorming/SKILL.md
  examples/sundial/docs/working-with-awf.md
  internal/project/target_test.go
  templates/docs/working-with-awf.md.tmpl
  templates/pi/awf-subagents/index.ts.tmpl
  templates/pi/awf-subagents/runner.ts.tmpl
  templates/skills/brainstorming/SKILL.md.tmpl
  tools/pi-extension-test/package-lock.json
  tools/pi-extension-test/package.json
  tools/pi-extension-test/tests/index.test.ts
  EOF
  sed -i 's/^  //' /tmp/awf-0125-phase2.allow
  sort -o /tmp/awf-0125-phase2.allow /tmp/awf-0125-phase2.allow
  while IFS= read -r path; do git add -- "/home/hypno/Projects/agentic-workflows/$path"; done < /tmp/awf-0125-phase2.allow
  git diff --cached --name-only | sort > /tmp/awf-0125-phase2.actual
  cmp /tmp/awf-0125-phase2.allow /tmp/awf-0125-phase2.actual
  ```

  Commit:

  ```commit
  feat(rendering): add Pi grounding progress
  ```

## Phase 3: Lifecycle freeze and final verification

- [ ] **Task 3.1: Record implementation findings before freezing.** Update `/home/hypno/Projects/agentic-workflows/docs/plans/2026-07-17-dedicated-pi-grounding-subagent-and-context-isolated-progress-rendering.md` Notes with every deviation, unexpected generated path, or test/API correction found during Phases 1-2. If none occurred, write `Implementation deviations: none.` Do this while the plan remains Proposed.

- [ ] **Task 3.2: Flip lifecycle state and regenerate indexes.** Change `status: Proposed` to `status: Implemented` in both `/home/hypno/Projects/agentic-workflows/docs/decisions/0125-dedicated-pi-grounding-subagent-and-context-isolated-progress-rendering.md` and this plan. Do not edit ADR-0123 beyond its already-committed reciprocal metadata. Run `cd /home/hypno/Projects/agentic-workflows && ./x sync`; expect `docs/decisions/ACTIVE.md` and both domain indexes to move ADR-0125 from Proposed to Implemented. No `docs/decisions/README.md` row is owed.

- [ ] **Task 3.3: Final gate, exact staging, and commit.** Run:

  ```sh
  cd /home/hypno/Projects/agentic-workflows
  ./x sync
  ./x check
  ./x gate
  ./x audit-local
  git diff -- $(cat /tmp/awf-0125-unrelated.paths) | cmp - /tmp/awf-0125-unrelated.patch
  ```

  Expect clean sync/check/invariants, 100% Go and Pi coverage, no audit Errors, and unchanged unrelated patch. Create and apply the literal six-path allowlist:

  ```sh
  cat > /tmp/awf-0125-phase3.allow <<'EOF'
  .awf/awf.lock
  docs/decisions/0125-dedicated-pi-grounding-subagent-and-context-isolated-progress-rendering.md
  docs/decisions/ACTIVE.md
  docs/domains/rendering.md
  docs/domains/tooling.md
  docs/plans/2026-07-17-dedicated-pi-grounding-subagent-and-context-isolated-progress-rendering.md
  EOF
  sed -i 's/^  //' /tmp/awf-0125-phase3.allow
  sort -o /tmp/awf-0125-phase3.allow /tmp/awf-0125-phase3.allow
  while IFS= read -r path; do git add -- "/home/hypno/Projects/agentic-workflows/$path"; done < /tmp/awf-0125-phase3.allow
  git diff --cached --name-only | sort > /tmp/awf-0125-phase3.actual
  cmp /tmp/awf-0125-phase3.allow /tmp/awf-0125-phase3.actual
  ```

  Commit:

  ```commit
  docs(adr): implement 0125 Pi grounding progress
  ```

## Verification

After the final commit, run:

```sh
cd /home/hypno/Projects/agentic-workflows
./x check
./x gate
rg -n 'name: "subagent_(grounding|explore|review|implement)"' .pi/extensions/awf-subagents/index.ts
rg -n 'subagent_(grounding|explore)' .pi/skills/awf-{brainstorming,refactor-coupling-audit}/SKILL.md
git diff -- $(cat /tmp/awf-0125-unrelated.paths) | cmp - /tmp/awf-0125-unrelated.patch
```

Acceptance criteria:
- exactly four public registrations with closed schemas and role/tool boundaries;
- Pi brainstorming uses grounding, Pi coupling audit uses exploration, and non-Pi skills contain no Pi tool token;
- real Pi 0.80.9 session execution proves partial details and final middleware-preserved content/details/error state;
- renderer tests cover all states at widths 24 and 120 with no over-width ANSI-aware line;
- intermediate child activity remains details-only and final content contains only report/failure summary;
- all six ADR-0125 invariants have exact Go test markers and retired ADR-0123 markers are gone;
- documentation and generated copies describe current behavior in the commit where it lands;
- ADR-0125 and this plan are Implemented, indexes are regenerated, and unrelated dirty diffs are byte-for-byte unchanged.

## Notes

Implementation should run inline with `awf-executing-plans`: the phases are ordered and repeatedly touch the same two extension templates. Fresh subagent-per-task dispatch would add conflict and handoff risk rather than useful isolation.
