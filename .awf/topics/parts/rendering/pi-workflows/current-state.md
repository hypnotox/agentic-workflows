Pi workflow contracts: governed subagent tools, session handoff, native skills, and structured exploration dispatch.

## Claims

### `invariant: pi-session-handoff-lifecycle`

Pi handoff retains its model-tool batch exclusivity, supported persisted-TUI check, single-use pending request, private FIFO queued command, terminating tool result, five-second countdown, cancellation, parent-linked session creation, old-history preservation, prepared-child cleanup, pre- and post-replacement failure boundary, automatic kickoff, editor fallback, visible recovery notice, and no-silent-retry behavior. A matched replacement submits one visible default-rendered `agent-handoff` custom transcript message through replacement-bound `sendMessage` with `triggerTurn:true`; its exact ownership prefix occurs once and editor and recovery paths use the same envelope. Post-countdown revalidation covers the matching pending request and active persisted-session state; the runtime does not infer, read, validate, mutate, or mention effort memory.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0152, ADR-0164, ADR-0167, ADR-0175, ADR-0209, ADR-0218, ADR-0219, ADR-0231
Backing: test

### `invariant: pi-dedicated-grounding-dispatch`

In the generated Pi extension and skills, the reusable grounding support skill dispatches through the existing dedicated grounding tool whenever its independent repository-premise trigger fires, while general exploration and coupling audits use the exploration tool, and no non-Pi target's rendered output contains either Pi subagent tool name.
Origin: ADR-0148
Revised-by: ADR-0243
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

Pi handoff's entire public input is exactly one required `kickoff` string property with no additional properties. It trims kickoff only to establish nonempty content, retains the public `maxLength: 1000` schema bound and execution-time 1,000-UTF-16-code-unit check, and preserves the accepted prose byte-for-byte after the two newlines in the `Agent-authored handoff context; this is not user input:` envelope. The persisted transcript role is custom with custom type `agent-handoff` and visible default rendering, while Pi's current provider adapter still converts that content to a user-role message. It accepts no memory path or other repository, filesystem, effort, ownership, link, size, encoding, header, or identity input.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0162, ADR-0164, ADR-0167, ADR-0175, ADR-0189, ADR-0209, ADR-0218, ADR-0219, ADR-0231
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

When selected `effort-workflow` and an enabled Pi target render the generated `using-effort` skill and `using_effort` tool, they explicitly associate one running process with at most one effort through direct non-terminating attach or detach at the repository root. A switch conservatively detaches the prior owner before attaching the next effort; immutable process-local snapshots, owner-checked heartbeat, restart-detached behavior, and ownership-loss cleanup keep advisory state bounded. Successful attachment activates only the three associated memory tools alongside unrelated active tools; detach, restart, missing activity, or ownership loss removes only those names. Every attached model call receives the fixed relative memory path and, when cached directory presence confirms it, the fixed managed-worktree path; failures silently omit unverified advisory facts. The runtime never infers an effort, resolves a checkout, changes CWD, queues a command, replaces or creates a conversation, writes residents directly, or uses local TUI presentation. Complete `awf` metadata remains independent. Optional presentation publishes only `{value:<canonical slug>}` or `{value:null}` through the capability-gated display-suffix event family; complete capability snapshots authoritatively withdraw support, replay is synchronous, and detach, restart, ownership loss, and shutdown clear explicitly. awf never reads or composes a name and display suffix is never routing authority.
Origin: ADR-0218
Revised-by: ADR-0225, ADR-0231, ADR-0239
Backing: test

### `invariant: using-effort-skill`

The Pi target alone derives the target-owned `using-effort` skill and `awf-effort` extension from selected `effort-workflow`; neither artifact is independently selectable, and no non-Pi target renders or refers to either one. The skill documents exactly direct `{effort:"<canonical-slug>"}` attachment and `{detach:true}` detachment from the repository root, use of the supplied fixed relative memory and optional managed-worktree paths, explicit-only association, restart-detached state, and the display-only suffix that is never routing input; activity is neither authority nor a lock. While associated it directs agents to prefer pathless memory reads, separate exact body edits from `phase` or `next` metadata updates, and rely on automatic timestamps, without forbidding generic file tools or direct commands.
Origin: ADR-0218
Revised-by: ADR-0225, ADR-0231, ADR-0239
Backing: test

### `invariant: pi-effort-memory-tools`

The Pi target alone derives three closed-schema pathless associated-memory tools from selected `effort-workflow`: paginated complete-document read, exact Markdown-body edit, and mutable `phase` or `next` update. They are registered but inactive while detached, activate only with a successful association, preserve unrelated active tools, carry active-only preference guidance, and clear on detached lifecycle boundaries or advisory ownership loss. The generated client uses bounded invocation and strict protocol decoding, while the index serializes complete operations and puts mutations on Pi's shared real-path file queue; these conveniences neither prohibit direct file access nor make activity an authority or lock.
Both mutation tools additionally run one owner-scoped binary preview before execution. The client selects and strictly decodes the operation-specific closed `previewed` envelope, requiring `replacementCount` for edit preview, forbidding it for update preview, forbidding any memory fact in either, and refusing a preview envelope for a normal invocation or a normal success envelope for a preview one. Preview inserts only `--preview` into otherwise unchanged owner-scoped argv and leaves normal mutation validation, publication, and durability semantics untouched. Any preview refusal or preview transport failure becomes the thrown tool error and no mutation runs. One shared self-rendered mutation surface built from Pi's public `renderDiff` and TUI exports serves both tools: it keys one preview per tool call, immutable association, and exact arguments, redraws only the still-current key, and always replaces preview state with the authoritative result, refusal, or error, so no refusal reads as success and a bounded diff shows its truncation warning. Model-visible mutation content stays compact while structured details keep the protocol reply. Preview joins association serialization only; the file mutation queue stays reserved for the mutations themselves.
Origin: ADR-0239
Revised-by: ADR-0244
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
