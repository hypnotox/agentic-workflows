---
format: current-state-v2
status: Implemented
date: 2026-07-22
---
# ADR-0145: Pi Fresh-Session Workflow Handoffs


## Context

awf already persists an in-flight effort's design brief, phase, next action, and handoff log under `.awf/memory/`. That convention lets an agent recover after session death or context compaction, but the normal workflow still keeps one main Pi session alive across brainstorming, ADR authoring, planning, implementation, review, and retrospective. The session therefore accumulates conversational noise and model-input cost even when the durable facts needed by the next phase already exist in working memory and repository sources.

A workflow checkpoint and a session handoff are different operations. The checkpoint persists state and gives the user a visible intervention point. A handoff may then replace the active session, but must not be the only way the checkpoint becomes durable. Long implementation phases also need intermediate safe checkpoints rather than waiting for the next named chain phase.

Pi is the project's primary supported interactive harness and exposes a parent-linked session replacement API. The Pi version previously required by awf, 0.80.9, could not safely complete the desired automatic flow: a model-callable tool could send a user message, but command expansion is deliberately disabled on that path, and no supported API could privately queue an extension command to run after the agent settled. Reaching into Pi internals or weakening the flow to require the user to type a slash command would make the extension depend on an undocumented boundary or abandon automatic continuation.

The missing general Pi capability has now been implemented upstream as `pi.queueCommand` in commit `50d88261`. It accepts only a registered extension command, validates before queueing, runs it with a fresh `ExtensionCommandContext` after the agent fully settles, preserves the command queue's FIFO and cancellation behavior, and creates no model-visible user message. awf can depend on the Pi release containing that API and keep session replacement out of the subagent child runner.

Pi session replacement is not fully transactional. Validation and countdown cancellation can leave the old session active, and normal replacement preserves its history, but some runtime failures after Pi has begun disposing the old active session can terminate the process. The public contract must state that boundary rather than promise preservation that the harness cannot guarantee.

## Decision

1. Every workflow phase boundary first completes a durable working-memory checkpoint in one tool batch, then displays a concise checkpoint summary that names the completed phase, immediate next action, and memory path. This visible summary is the user's intervention opportunity. Long phases, especially implementation, may create the same kind of intermediate safe checkpoint whenever the next action is independently resumable.

2. On the Pi target, workflow guidance invokes a fresh-session handoff automatically after each safe checkpoint by default. The handoff starts only after the visible summary and gives the user a five-second cancellation window. A user may also request the same handoff at any safe checkpoint. Other targets retain the checkpoint and automatic workflow continuation but make no unsupported session-replacement claim.

3. Pi renders a separate extension at `.pi/extensions/awf-handoff/index.ts`, authored from `templates/pi/awf-handoff/index.ts.tmpl`. The existing `.pi/extensions/awf-subagents/` extension and its child runner remain responsible only for isolated child processes. The handoff extension owns its model tool, private continuation command, pending request, countdown and cancellation UI, session replacement, and kickoff.

4. The extension registers `handoff_session` with exactly two required string arguments: `memoryPath` and `kickoff`. `memoryPath` is an exact repository-relative path confined beneath `.awf/memory/`. Absolute paths, traversal, the memory directory itself, directories, missing files, paths outside that root, and any path containing a symlink component are rejected. The extension resolves the repository and validates the path without reading or interpreting the memory file's content.

5. `kickoff` is trimmed for validation and must contain non-whitespace content. Its public TypeBox schema carries `maxLength: 1000`, and execution additionally requires JavaScript `kickoff.length <= 1000` so the effective boundary is exactly 1,000 UTF-16 code units even though JSON Schema length counts Unicode code points. The original accepted string is carried into the kickoff wrapper. The wrapper names the exact memory path and kickoff, instructs the new agent to read that memory file first, and states that repository sources and current-state documentation are authoritative over the checkpoint.

6. The working-memory update must finish before `handoff_session` is invoked, and `handoff_session` must be the only tool call in its assistant tool-call batch. Pi's tool preflight correlates the call with the current leaf assistant message and fails closed when it cannot establish exclusivity. Checkpoint freshness and content quality remain instructional because the extension intentionally has no dependency on the memory file's prose.

7. Initial support is limited to an interactive Pi TUI with a persisted session capable of parent-linked replacement. Print, JSON, RPC, ephemeral, and no-session invocations reject before queuing and do not change session state. The public tool remains registered in supported TUI sessions so it is available at intermediate safe checkpoints, not only at terminal skill handoffs.

8. A valid tool call creates one pending request and queues a correlated private continuation command through `pi.queueCommand`. Pending requests are single-use. A second tool call cannot replace an existing request, and the continuation command rejects invocation without the matching validated request. Queue failure clears the request and returns an error without changing sessions. A successfully queued `handoff_session` result sets `terminate: true`, preventing another model turn before the agent settles and the command starts.

9. The continuation command begins only after the calling agent has fully settled. It presents a visible five-second countdown with a documented cancellation control. Cancellation consumes the request and leaves the old session active. Immediately before replacement, the command repeats repository confinement, existence, file-type, and all-component no-symlink validation so a change during the pending window fails closed.

10. After the countdown, the command creates a full persisted session whose parent is the old session and makes it active through Pi's supported replacement context. Normal replacement never deletes or rewrites the old session history. Neither successful nor failed handoff deletes the working-memory file; the workflow's existing retrospective cleanup remains its only automatic deletion.

11. Once replacement succeeds, the extension submits the mechanical kickoff wrapper through the replacement-session context. If automatic kickoff submission fails, the new session remains active and the exact wrapper is placed in the editor for manual submission. Failures before Pi commits to replacement preserve the old active session. Failures after Pi begins its non-transactional disposal boundary may terminate the runtime, but still do not intentionally delete either persisted session or the memory file.

12. awf raises the generated extension's minimum and pinned Pi version from 0.80.9 to 0.81.1, the first project-approved build containing `pi.queueCommand`. The compatibility check happens before either Pi extension partially registers its public tools. Documentation and release smoke instructions name the same minimum.

13. The Pi output descriptor, output plan, manifest, cleanup, drift check, and target-sensitive render hash govern all three extension files: the two existing `awf-subagents` files and the new `awf-handoff` file. A target set without Pi renders none of them. The generated awf checkout and Sundial example carry the same governed extension output after sync.

14. Deterministic tests cover strict schema and path boundaries, the explicit UTF-16 kickoff limit, symlink and pending-window revalidation, batch exclusivity, the single-use pending state machine, unsupported modes, countdown cancellation, termination without an intervening model turn, replacement and parent lineage against the pinned Pi API, pre- and post-replacement failure behavior, editor fallback, output planning and cleanup, generated awf and Sundial files, and target-specific workflow guidance. The Pi container lane copies and type-checks every governed Pi extension file rather than retaining a hard-coded two-file assumption.

15. Implementation updates the workflow and working-memory guidance, architecture, working-with-awf and testing documentation, generated agent guide, target and template claims, and relevant examples in the same checked batches as their behavior. Templates remain publication-safe for non-Pi targets and for unset project variables. Every ADR lifecycle transition runs `./x sync` and stages the regenerated `docs/decisions/INDEX.md` in the same commit.

## State changes

- update `rendering/templates:memory-checkpoint-chain-coverage`
- update `rendering/templates:pi-extension-editor-quiet-strip`
- add `rendering/templates:pi-session-handoff-public-contract`
- add `rendering/templates:pi-session-handoff-workflow`
- update `rendering/catalog-and-targets:pi-extension-target-render`
- update `rendering/catalog-and-targets:pi-minimum-runtime`
- update `rendering/catalog-and-targets:pi-real-runtime-smoke`
- add `rendering/catalog-and-targets:pi-session-handoff-lifecycle`

## Consequences

Each completed workflow phase can begin its successor with a small context based on durable working memory and current repository authority. The user sees the checkpoint before replacement, can cancel during a predictable window, and retains the old session as navigable history. Intermediate checkpoints extend the same cost control to long implementation work.

Default handoffs add five seconds of latency at every completed checkpoint that is not canceled. They also accumulate persisted parent-linked sessions, increasing storage use and session-navigation clutter. History cleanup remains a deliberate manual action because automatic deletion would weaken recovery and auditability.

Pi receives another executable generated extension and a higher minimum runtime. Output planning, example wiring, container fixtures, documentation, and release smoke checks all widen accordingly. Non-Pi adapters gain clearer visible checkpoints without pretending their harnesses can replace sessions.

The extension deliberately does not summarize the full conversation, parse checkpoint prose, infer a memory file, delete old sessions, or clean memory automatically. Callers must provide the exact durable state path and a bounded immediate kickoff. This makes the handoff mechanical and auditable, but a stale or poor checkpoint can still produce a poor continuation.

Normal validation, cancellation, and queue failures are fail-closed. Pi's non-transactional replacement teardown remains a residual risk after the old active session starts disposal. Preserving both persisted histories and leaving the kickoff in the new editor where possible limits recovery loss without claiming impossible runtime atomicity.

The upstream queued-command API becomes a real awf runtime dependency. Pinning the first approved version avoids undocumented integration, while its general registered-command contract remains reusable by other extensions rather than introducing a handoff-specific Pi primitive.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep one main session and rely on automatic compaction | Compaction is lossy, retains conversation-derived noise, and ignores the already durable workflow checkpoint. |
| Delete or clear the old session | Destroys useful history and recovery evidence without reducing the new session's need to read authoritative repository state. |
| Re-summarize the entire conversation into the new session | Duplicates the working-memory convention, adds model cost, and can elevate conversational details over current repository authority. |
| Require the user to type a continuation slash command | Avoids the Pi API addition but breaks automatic continuation and makes every phase boundary depend on manual action. |
| Send a slash command through `sendUserMessage` | Pi deliberately disables command expansion for extension-sent messages, and changing that security boundary would expose a broader, less controlled mechanism. |
| Reach into Pi command-queue internals | Couples generated project code to undocumented implementation details and cannot support a stable minimum-version contract. |
| Add replacement directly to `awf-subagents` | Mixes main-session lifecycle with isolated child-process orchestration and expands the runner's permission and failure surface. |
| Expose handoff only as an opt-in model tool | Reduces interruption, session creation, and runtime coupling, but leaves the normal phase chain accumulating context unless the user or agent repeatedly elects the optimization; the visible checkpoint and cancellation window make default automation controllable. |
| Make handoff instantaneous | Removes the visible opportunity to stop an automatic replacement after the checkpoint summary. |
| Promise transactional rollback for every replacement failure | Pi can fail after old-session disposal begins; such a promise would not match the pinned runtime's real lifecycle. |

## Status history

- 2026-07-22: Proposed
- 2026-07-22: Implementing; content-sha256: 668f00302378496b92465c023e4ce014020130bbda035ea2a617f3689ff77ed3
- 2026-07-22: Applied; state-sequence: 13; operations: update `rendering/templates:memory-checkpoint-chain-coverage`
- 2026-07-22: Applied; state-sequence: 14; operations: update `rendering/templates:pi-extension-editor-quiet-strip`, add `rendering/templates:pi-session-handoff-public-contract`, add `rendering/templates:pi-session-handoff-workflow`, update `rendering/catalog-and-targets:pi-extension-target-render`, update `rendering/catalog-and-targets:pi-minimum-runtime`, update `rendering/catalog-and-targets:pi-real-runtime-smoke`, add `rendering/catalog-and-targets:pi-session-handoff-lifecycle`
- 2026-07-22: Implemented; content-sha256: 668f00302378496b92465c023e4ce014020130bbda035ea2a617f3689ff77ed3
