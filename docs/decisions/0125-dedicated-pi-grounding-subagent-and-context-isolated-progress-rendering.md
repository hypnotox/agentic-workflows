---
status: Proposed
date: 2026-07-17
supersedes: []
superseded_by: ""
tags: [multi-target, subagent-dispatch, workflow-chain]
related: [123]
domains: [rendering, tooling]
---
# ADR-0125: Dedicated Pi Grounding Subagent and Context-Isolated Progress Rendering

## Context

ADR-0123 gave Pi adopters three workflow-focused child-process tools. It bound
brainstorming's grounding check to the generic `subagent_explore` role because
grounding and general investigation shared a no-mutation tool boundary. That
made the mechanism available, but not the role: the extension does not identify
grounding as a distinct workflow operation or supply its stable verification
instructions independently of each dispatch brief.

The extension already parses child JSON events and sends partial tool updates.
It retains completed assistant text and started tool calls as bounded summaries,
but the registered tools define no custom renderer. A user therefore cannot
meaningfully monitor the retained activity in the parent TUI. Moving progress
into ordinary conversation messages would make it visible, but would also
pollute the parent model's context and session history with a child transcript.
A separate widget would avoid that context cost while detaching progress from
the tool invocation that owns it.

Pi's custom-tool contract provides the required boundary: tool `content` is the
model-visible result, while `details` carries UI and log metadata and is passed
to `renderResult`, including partial updates. Pi's maintained subagent example
uses that split for inline progress. The installed 0.80.10 source confirms the
behavior, but awf supports Pi 0.80.9, so the deterministic extension lane must
prove the same partial-details seam at the declared minimum.

Failure handling needs special care. A custom tool that throws loses its custom
details when Pi constructs the error result. If progress details are to remain
visible after a child fails or is aborted, the extension must return a failed
tool result carrying those details rather than throw after execution begins.
The current twenty-summary ring also removes old entries silently; an expanded
view needs an explicit cumulative omitted-event count to describe its bounded
history honestly.

The design touches the stable public tool set, workflow dispatch, child event
retention, custom TUI rendering, and target-specific generated guidance. It is
load-bearing and requires a partial successor to ADR-0123 rather than an
in-place change to that Implemented record.

## Decision

1. Expand the extension's closed public contract to exactly four tools by adding
   `subagent_grounding {task: string}` beside `subagent_explore`,
   `subagent_review`, and `subagent_implement`. The task remains required and
   non-empty. Grounding is a fixed extension role, like exploration, not a
   catalog-toggleable Markdown reviewer and not a new `subagent_review` kind.
   Existing tool names and parameter schemas remain unchanged.
   `supersedes: ADR-0123#2`
   `supersedes-invariant: ADR-0123#pi-subagent-public-contract`

2. Give `subagent_grounding` the exploration role's exact closed tool allowlist:
   `read`, `grep`, `find`, `ls`, and `bash`. Its fixed system prompt makes it a
   no-mutation grounding checker: verify the supplied design premises against
   source, identify unstated assumptions and edge cases, assess ADR/plan/effort
   altitude, check Accepted or Implemented ADR and invariant fit, and return
   grounded confidence-classified findings. Pi brainstorming dispatches its
   single grounding check through this tool. General investigations and large
   coupling audits continue to use `subagent_explore`; all non-Pi targets keep
   their target-native grounding wording.
   `supersedes: ADR-0123#5`
   `supersedes-invariant: ADR-0123#pi-explicit-workflow-dispatch`

3. Preserve ADR-0123's child-process architecture: each invocation starts the
   current Pi executable in JSON mode with no session, inherits the parent
   provider/model and thinking level, passes an explicit closed role allowlist,
   and uses a mode-0600 temporary role prompt. Final model-visible output stays
   capped at 50 KiB or 2,000 lines and stderr retains its last 50 KiB. Replace
   the old summary representation with at most twenty structured display events
   of at most 2 KiB each plus a cumulative count of older omitted events. Every
   bounded surface reports truncation or omission explicitly; no complete raw
   child transcript is retained elsewhere. Diagnostics, usage, model, stop
   reason, temporary-state cleanup, and TERM-to-KILL cancellation remain part
   of the runner contract.
   `supersedes: ADR-0123#3`

4. Retained display events cover completed child assistant turns and child tool
   call starts and completions. They do not stream token-by-token assistant
   prose and do not retain full tool results. This supplies useful stable
   progress without excessive redraws or an unbounded data channel. The runner
   emits the structured event window and omission count through partial tool
   `details`. On completion, only the final child report becomes tool
   `content`; retained activity, diagnostics, and usage stay in final
   `details`. The extension never appends progress as custom session messages.

5. All four public tools use shared `renderCall` and `renderResult` behavior.
   The collapsed inline card shows role, running/completed/failed/aborted state,
   recent retained activity, omission state, and available usage. The expanded
   Ctrl+O view shows the task, all retained child tool calls and completed
   assistant turns, the final report rendered as Markdown, bounded diagnostics,
   and usage. Rendering is presentation-only: non-TUI modes still return the
   ordinary final tool result, and a renderer defect cannot alter child
   execution or model-visible content.

6. Once child execution starts, expected child exit, stop-reason, malformed
   event, and cancellation failures return `isError: true` with bounded final
   content and the retained progress details instead of escaping as thrown tool
   errors. Validation and setup failures that occur before useful child state
   exists may still throw. Implementation commit-policy violations remain hard
   errors, retain their before/after git evidence, and are never auto-reverted.

7. Extend the mandatory Pi-extension lane and Go rendering tests to prove the
   exact four-tool contract, dedicated grounding prompt and allowlist, Pi-only
   workflow binding across every target, unchanged exploration coupling-audit
   binding, minimum-Pi partial-details behavior, structured event ordering and
   bounds, omission counts, collapsed and expanded render states, context
   isolation, failure-detail retention, and all existing process and
   implementation boundaries. Update authored working guidance, identity,
   architecture, testing, changelog, domain current state, and generated adopter
   copies in the implementation commits.

## Invariants

- `invariant: pi-subagent-four-tool-contract`: the generated Pi extension
  exposes exactly grounding, exploration, governed review, and serialized
  implementation tools with their closed parameter schemas and role boundaries.
- `invariant: pi-dedicated-grounding-dispatch`: Pi brainstorming uses the
  dedicated grounding tool, general exploration and coupling audits retain the
  exploration tool, and non-Pi targets contain neither Pi tool name.
- `invariant: pi-subagent-progress-context-isolation`: intermediate child
  activity is carried only in bounded tool details and never in parent
  model-visible content or custom session messages; final content contains only
  the child report or bounded failure summary.
- `invariant: pi-subagent-progress-rendering`: every public subagent tool renders
  bounded live and final inline activity, omission state, status, diagnostics,
  and usage from the same structured details without changing execution.
- `invariant: pi-subagent-failure-details`: expected failures after child start
  preserve bounded progress and diagnostics in an error result while retaining
  cancellation, cleanup, and implementation-policy behavior.
- `invariant: pi-subagent-progress-bounds`: retained display events have fixed
  count and byte limits, report cumulative omissions and truncation, and never
  create a second raw-transcript store.

## Consequences

- Grounding becomes visible as a first-class workflow role, so the model no
  longer has to infer that a generic exploration call carries a stricter
  verification contract.
- Users can monitor every child without paying for intermediate activity in the
  parent context. The final report remains model-visible because the
  orchestrator must act on it.
- Inline rendering keeps progress attached to its invocation and naturally
  supports Pi's collapsed/expanded tool interaction. It adds TUI-specific code
  and renderer tests to the generated extension.
- Completed-turn rather than token-level prose makes progress slightly less
  immediate, but produces stable output and avoids high-frequency redraws.
- The expanded view is deliberately a bounded activity history, not a complete
  transcript. Explicit omission counts prevent it from implying completeness.
- Returning structured error results after child start preserves diagnostics
  but requires the runner and tool wrappers to distinguish setup exceptions
  from expected execution failures.
- Existing Pi adopters receive a fourth model-callable tool and changed
  target-specific brainstorming guidance on sync. Other targets are unchanged.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep grounding on `subagent_explore` | Shares the same capabilities but leaves the workflow role and fixed verification policy implicit. |
| Add `grounding` to `subagent_review.kind` | Grounding validates a brainstorm's premises and altitude; it is exploratory, not governed artifact review. |
| Show progress in a persistent widget | Keeps details out of context but detaches activity from the owning invocation and complicates multiple calls. |
| Append progress messages and filter them in the `context` event | Looks native in the timeline but adds persisted session noise and a context-leakage risk. |
| Stream child prose token by token | More immediate, but unstable partial prose and redraw churn add complexity without improving workflow decisions. |
| Retain the full child transcript in details | Makes expansion complete but violates the fixed bounded-retention and no-secondary-transcript boundary. |
