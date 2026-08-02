Pi workflow contracts: governed subagent tools, session handoff, native skills, and structured exploration dispatch.

## Claims

### `invariant: pi-session-handoff-lifecycle`

Pi handoff retains its model-tool batch exclusivity, supported persisted-TUI check, single-use pending request, private FIFO queued command, terminating tool result, five-second countdown, cancellation, parent-linked session creation, old-history preservation, prepared-child cleanup, pre- and post-replacement failure boundary, automatic kickoff, editor fallback, visible recovery notice, and no-silent-retry behavior. Post-countdown revalidation covers the matching pending request and active persisted-session state; the runtime does not infer, read, validate, mutate, or mention effort memory.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0152, ADR-0164, ADR-0167, ADR-0175, ADR-0209, ADR-0218, ADR-0219
Backing: test

### `invariant: pi-dedicated-grounding-dispatch`

In the generated Pi extension and skills, brainstorming's grounding check dispatches through the dedicated grounding tool while general exploration and coupling audits use the exploration tool, and no non-Pi target's rendered output contains either Pi subagent tool name.
Origin: ADR-0148
Backing: test

### `invariant: pi-extension-editor-quiet-strip`

Every governed Pi extension file carries the ts-nocheck directive on the line immediately after the provenance banner, and the container test harness deterministically strips that exact directive from every extension TypeScript file in its ephemeral copy after source copy and before running the TypeScript compiler.
Origin: ADR-0148
Backing: test

### `invariant: pi-implementation-batch-exclusivity`

Pi correlates each tool preflight with the current leaf assistant tool-call id, blocks every member of a reconstructable batch that mixes implementation with siblings, and blocks only implementation when trustworthy batch context is unavailable.
Origin: ADR-0148
Backing: test

### `invariant: pi-session-handoff-public-contract`

Pi handoff exposes exactly one required `kickoff` string property with no additional properties. It trims kickoff only to establish nonempty content, retains the public `maxLength: 1000` schema bound and execution-time 1,000-UTF-16-code-unit check, and otherwise carries the prose unchanged into the replacement session, automatic submission, editor fallback, and recovery path. It accepts no memory path or other repository, filesystem, effort, ownership, link, size, encoding, header, or identity input.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0162, ADR-0164, ADR-0167, ADR-0175, ADR-0189, ADR-0209, ADR-0218, ADR-0219
Backing: test

### `invariant: pi-session-handoff-workflow`

Pi workflow guidance keeps checkpoint persistence mandatory and permits session replacement only after a completed formal phase checkpoint, after explicit approval and its next action are persisted, or after an additional safe resumable checkpoint. At each eligible point the agent chooses continuation or handoff from currently available context and compaction evidence, retained-context relevance, and upcoming work, with no fixed threshold; declining handoff is autonomous continuation, not a check-in. A replacement session appends the handoff-log boundary as its first memory update before substantive continuation, while cancellation or failure that leaves the old session active appends none. Callers carry any effort-memory reorientation instruction across the replacement boundary; workflow guidance remains the owner of checkpoint eligibility, reorientation, and boundary logging. A fresh phase or task owner may consume `awf read plan`'s executable closure, but projection never creates a handoff boundary. Pi never creates standalone memory, requires selection or telemetry lifecycle state, adopts a checkpoint, or treats heading-identified tasks and helper returns as routine handoff boundaries; repository authority remains primary and report-only children do not edit memory.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0152, ADR-0164, ADR-0166, ADR-0167, ADR-0175, ADR-0209, ADR-0213
Backing: test

### `invariant: pi-native-workflow-skills`

Pi renders every selected standard and local catalog skill at `.pi/skills/<prefix>-<name>/SKILL.md`; disabled skills or Pi disablement prune those paths, and no router or hidden workflow-body output remains. Selected `effort-workflow` additionally derives the Pi-target-owned `using-effort` skill at the same native skill path without making it a second catalog selection; deselecting `effort-workflow` or disabling Pi prunes that companion and its extension.
Origin: ADR-0167
Revised-by: ADR-0218
Backing: test

### `invariant: pi-effort-session-association`

When selected `effort-workflow` and an enabled Pi target render the generated `using-effort` skill and `using_effort` tool, they explicitly associate one running session with at most one effort, resolve managed or recorded/explicit receiving checkout through the awf binary, and queue command-only `changeCwd` before owner-checked activity commit; they never infer an effort, guess a receiving checkout, follow topology, create a conversation, or write residents directly. Immutable process-local snapshots and a one-shot runtime-replacement coordinator preserve the same-session association across a successful CWD rebind but never across process restart; missing capability visibly changes no CWD/activity/memory axis. Activity takeover warns and proceeds, heartbeat/metadata/name failures remain advisory, stale age never changes permission, Remote Pi publication uses complete `awf` metadata replacement plus negotiated transient name override/replay, and detach/restart restore the base identity without turning presence into authority or a lock.
Origin: ADR-0218
Backing: test

### `invariant: using-effort-skill`

The Pi target alone derives the target-owned `using-effort` skill and `awf-effort` extension from selected `effort-workflow`; neither artifact is independently selectable, and no non-Pi target renders or refers to either one. The skill documents the explicit tool arguments, validated managed/receiving switches, first-receiving-path repair, takeover warning, advisory heartbeat/metadata/name behavior, explicit detach, and the rule that activity is neither authority nor a lock.
Origin: ADR-0218
Backing: test

### `invariant: pi-structured-exploration-contract`

The generated Pi extension exposes exactly four closed-schema roles, each with optional exact model routing; exploration retains required task, breadth, and detail and runs through the ten-active FIFO limiter without changing the other process boundaries.
Origin: ADR-0148
Backing: test

### `invariant: pi-subagent-failure-details`

In the generated Pi extension, expected failures that occur after a child process has started return a marked error result that preserves bounded progress and diagnostics through a tool_result middleware hook instead of throwing, while retaining cancellation, cleanup, and implementation-commit-policy behavior.
Origin: ADR-0148
Backing: test

### `invariant: pi-subagent-model-preferences`

The generated Pi extension merges user-global and gitignored project-local preferences per field for the shared default, every grounding, exploration, review, and implementation role, and the small, standard, and large tiers. Completeness requires every field explicitly after merging; missing fields remain valid and visible, while any malformed, overlong, unregistered, unauthenticated, unavailable, or unreadable configured field blocks all implicit routing and leaves valid explicit calls usable. Preference and registry state reloads at preflight and again immediately before child startup.
Origin: ADR-0151
Revised-by: ADR-0173
Backing: test

### `invariant: pi-subagent-model-routing`

Every Pi subagent role accepts only omission or an exact registry-valid provider/model-id of at most 256 printable-ASCII characters, excluding space and DEL. The tool schemas and preference parsing derive that form from one shared pattern constant, so the two layers cannot diverge, and within the permitted charset the bound is the same count whether measured in code points, UTF-16 units, or UTF-8 bytes. Omission alone requests configured role routing and parent fallback, and is displayed with a label the shared form check rejects, so a displayed value can never be copied back as a usable argument. Default, auto, inherit parent, and other sentinel values reject with an omit-the-field repair and are never normalized, and an overlong reference reports overlong before any form rejection. Queue acquisition is followed by preference and registry revalidation immediately before child startup, failures never fall through, thinking remains inherited for child clamping, and diagnostics report requested, resolved, and actual models with routing source.
Origin: ADR-0148
Revised-by: ADR-0151, ADR-0173, ADR-0176
Backing: test

### `invariant: pi-subagent-model-wizard`

The /awf-subagent-models command is a TUI-only atomic wizard for the shared default, all four explicit role defaults, and the small, standard, and large tiers. It preserves scope and error display, complete cancellation without writes, live registry-gated Luna/Terra/Sol preset selection, informed manual selectors, project-local gitignore enforcement, owner-only sibling-temp replacement, stale-writer detection, cleanup, and in-memory refresh, and it writes roles and tiers together as one preference transaction.
Origin: ADR-0151
Revised-by: ADR-0173
Backing: test

### `invariant: pi-implement-role-artifact`

The generated Pi extension builds the implementation child's role prompt from the rendered implementer agent at its `.pi/agents/` path, prepending the commit-authority role line for the call's mode. The before-and-after git snapshot fails a commit-capable implementation call whose HEAD is unchanged, naming the required stopped inventory, and retains the existing commit-forbidden violation, its message, cancellation, cleanup, and bounded-diagnostic reporting.
Origin: ADR-0177
Revised-by: ADR-0179
Backing: test

### `invariant: pi-subagent-progress-bounds`

The generated Pi extension retains at most 20 display events of at most 2 KiB each, reports cumulative omitted-event counts and truncation explicitly, and never keeps a second raw child-transcript store.
Origin: ADR-0148
Backing: test

### `invariant: pi-subagent-progress-context-isolation`

The generated Pi extension carries intermediate child activity only in bounded tool details, never appending it to parent model-visible content or custom session messages, and a subagent tool's final content contains only the child report or a bounded failure summary.
Origin: ADR-0148
Backing: test

### `invariant: pi-subagent-progress-rendering`

In the generated Pi extension, every public subagent tool's collapsed view renders status, recent bounded activity, omission state, and available usage, and its expanded view additionally renders the task, retained activity, the final report, present diagnostics, and available usage from the same structured details without changing execution.
Origin: ADR-0148
Backing: test

### `invariant: pi-role-contract-loader`

The generated Pi extension loads every dispatched role's contract from its rendered agent artifact through one shared loader that reads the file, strips frontmatter, prepends the role's per-call authority line, and fails with an actionable enable-and-render repair naming that role on a missing file or an empty instruction body. No dispatched role's prose remains inline in the extension.
Origin: ADR-0179
Backing: test
