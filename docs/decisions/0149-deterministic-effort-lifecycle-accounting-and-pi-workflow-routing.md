---
format: current-state-v2
status: Proposed
date: 2026-07-23
---
# ADR-0149: Deterministic Effort Lifecycle Accounting and Pi Workflow Routing


## Context

ADR-0146 established an append-only workflow ledger, explicit lifecycle operations, canonical metrics and diagnostics, and a generated Pi dashboard. Post-implementation review found that these pieces do not yet form a usable accounting system. The Pi workflow skills do not invoke lifecycle operations, so normal work has no effort association or phase data. The widget therefore cannot show meaningful workflow state, and direct skill loading can always bypass any lifecycle instructions added only as prose.

The local widget also maintains a partial counter model. It omits restored history, summaries and compactions, nested subagent usage returned outside Pi's top-level usage shape, and some current context usage. Its placement and presentation differ from the Pi footer. Passive appends do not refresh an open overlay, older canonical refreshes can overwrite newer local state, and repair and waiver actions can use an empty causal frontier, an ineligible reason, or a finding other than the selected effort's finding.

Effort identity has a bootstrapping problem. A fresh root session must attribute observations made before the agent selects a workflow, but durable creation at session start would leave a spurious effort whenever the user explicitly resumes an existing effort. Conversely, waiting until after selection loses the first-turn accounting. Continuation also cannot depend on a working-memory file: small efforts intentionally need none, while a memory-backed complex effort needs a deterministic way to name its effort without making the effort depend on the file.

Pi discovers skills from rendered files and exposes those skill bodies for direct loading. Lifecycle correctness therefore cannot be achieved by adding another instruction to each body. Pi needs one discoverable semantic entry point and an enforcing load boundary, while other targets must retain their native individually rendered workflow artifacts.

No protocol-1 resident production data exists. Preserving protocol-1 migration code would add compatibility branches for data that can be deleted before this correction lands.

## Decision

1. Telemetry protocol major 2 replaces protocol major 1 without an in-place protocol-1 migration. Protocol 2 removes `checkpointId` from effort metadata, lifecycle requests, privacy exceptions, fixtures, and generated types. Implementation deletes or resets the unused local protocol-1 resident data and fixtures. An unsupported protocol major remains an explicit incompatibility.

2. A fresh root Pi session begins with a provisional candidate effort held only in bounded process memory. The dashboard displays `[awf:init]` and buffers privacy-filtered observations until the first workflow selection or explicit resume operation resolves identity. The buffer holds at most 256 observations and 1 MiB measured as each observation's canonical UTF-8 protocol encoding; an individually oversized observation triggers overflow settlement before it is appended directly.

3. Settlement is a retryable state machine with one candidate ID and deterministic idempotency keys derived from candidate, session, and observation IDs. Its ledger commit point is the durable `effort_created` event. After that point, retries inspect the ledger, persist the matching Pi custom association entry, and idempotently flush observations in original order; before that point no durable effort exists. Pi custom-entry failure leaves the committed effort recoverable from the in-process settlement record and visibly degraded, never reported as associated. Restart restoration trusts only a persisted custom entry or explicit resume; it does not infer a partially settled candidate.

4. Workflow selection settles by creating the candidate, associating it, flushing the buffer, and then applying the workflow transition. Explicit resume before the creation commit discards the candidate and flushes its observations to the selected existing effort. Exceeding either buffer bound first settles the candidate as an init effort and warns. A later resume closes that overflow effort as abandoned with reason `provisional-overflow-resume`, detaches the session, associates the selected effort, and affects only subsequent observations; already committed observations are never reassigned. Normal session replacement and graceful shutdown attempt candidate settlement. A process crash before the creation commit may lose only the bounded provisional window.

5. `/resume` restores the active-branch association directly from the persisted custom entry. A fresh `/new` continuation uses `/awf-resume-effort <effort-id>`, whose generated command validates one bounded ID and queues private replacement setup carrying the closed resume request before the new session starts. Discovery and active efforts may be associated directly. Completed efforts require the existing explicit reopen operation and a new trajectory before association. Abandoned or pruned efforts cannot be resumed; derived work creates a new effort with ADR-0146 lineage. The runtime does not mine prompts, filenames, arbitrary prose, session ancestry, or repository state for identity.

6. Working memory is optional and one-way. Creating an effort does not create a memory file and effort metadata stores no memory path. When a workflow later creates or updates `.awf/memory/<slug>.md`, the file carries `Effort: <id>`. A structured resume may use that field after validating the memory file, but the ledger never requires the file to exist.

7. `handoff_session` remains available only for memory-backed efforts. It requires a validated memory file carrying `Effort: <id>`, copies the association before child startup independently of dashboard factory load order, restores it before kickoff, and refuses an effort mismatch. Checkpoint-less efforts continue in the same session or through explicit resume rather than synthesizing a handoff checkpoint.

8. Pi exposes exactly one discoverable workflow skill at `.pi/skills/awf-workflow/SKILL.md`. It contains a concise catalog that lets the agent choose a governed chain or task skill by semantic name. Reviewer agents remain separately rendered and discoverable. Existing individually discoverable Pi workflow and task skill files are removed.

9. `awf sync` still pre-renders every governed Pi workflow and task skill body at generation time, but writes the bodies to a managed non-discovered tree under `.pi/awf-workflows/`. They carry normal provenance and drift ownership and are never template-rendered, interpolated, or assembled at runtime. Non-Pi targets continue to render their ordinary individually discoverable skills and equivalent target-native lifecycle guidance.

10. A generated `awf_workflow` Pi tool is the only supported loader for governed hidden bodies. Its input is a closed enum derived from enabled catalog metadata, and it must run alone in its tool batch. The same metadata maps each semantic skill to route selection or change, phase transition, activity, implementation mode, and terminal behavior. Deterministic idempotency keys bind session, current lifecycle frontier, and semantic skill. The tool validates the request, settles or resumes provisional identity, applies the mapped lifecycle mutation, and only then returns the fixed pre-rendered body; competing requests from one frontier produce one winner and a visible stale-frontier failure.

11. Governed bodies are non-discovered and the enforcing loader is the exclusive supported access path; a filesystem path is never accepted as loader input. The runtime does not claim to prevent arbitrary same-user shell or external filesystem access to repository files. The router may describe skill selection but may not return a hidden body itself. Catalog and render checks prove that every enabled governed skill has exactly one hidden body and one lifecycle mapping, and that no stale individually discoverable Pi copy remains.

12. Loading brainstorming settles the candidate and starts brainstorming while the effort remains in discovery. Loading the selected successor after brainstorming chooses `direct`, `adr`, `plan`, `adr-plan`, `bugfix`, or `investigation-only` and transactionally enters its mapped phase. Loading a downstream chain skill without the causally required predecessor is rejected. A later semantic scope change uses the existing explicit route-change rules. Investigation and task-skill activities may occur in discovery without prematurely choosing a route.

13. Chain handoffs use one protocol-2 `phase_transitioned` event whose payload names the causally visible unmatched start, closing phase, successor phase, route effect when any, and current predecessor frontier. One durable append is the commit point and has both projection effects. Identical retries are idempotent; a conflicting transition from the same frontier is rejected or retained as concurrent-state evidence under the existing reader rules. Because one event carries both effects, crash recovery either observes the transition or retries it and cannot expose a committed gap.

14. Loading a workflow skill is the enforcement point, not a second set of agent instructions. When retrospective is loaded, the runtime records its tool call and phase start. The first subsequent uncancelled `agent_end` with no failed tool result after that call is the deterministic settled-success acknowledgment and appends completion idempotently; cancellation, extension shutdown before settlement, or a failed tool leaves retrospective open for retry. Abandonment remains an explicit terminal action. The badge states are `[awf:init]`, `[awf:<phase>]`, `[awf:done]`, and `[awf:abandoned]`; unassociated or incompatible states are exceptional and carry a compact degraded suffix rather than masquerading as init.

15. Local lifecycle state updates the badge immediately after a validated append. Canonical metrics and doctor refresh asynchronously at startup, overlay open, lifecycle settlement, bounded passive-observation boundaries, and explicit refresh. Refreshes are coalesced, and a monotonic generation or epoch prevents an older completion from overwriting newer local lifecycle or projection state. An open overlay receives the same refresh notifications.

16. The compact widget renders below the editor in a muted style. Its information shape follows the Pi footer: input, output, cache-read, cache-hit percentage, permitted cost, subagent marker, and context percentage and window. It uses Pi session entries plus `ctx.getContextUsage` so restored history, summaries, compactions, and current context match footer accounting. It shows subscription or automatic-context indicators only when Pi exposes them through a public and safe API; otherwise those fields are omitted rather than inferred from internals.

17. Subagent tool results return Pi's top-level `usage` shape in addition to bounded details, so Pi's own totals include nested work. The telemetry observation retains only the protocol-approved aggregate fields and never the subagent task, report, transcript, tool arguments, or diagnostics text.

18. Canonical findings carry their owning `effortId`. Repair and waiver actions resolve the selected finding from the selected effort, use the current causal predecessor frontier, validate the rule-authorized reason and scope, and reject stale or cross-effort actions. They never manufacture an empty frontier merely because the UI lacks cached event IDs.

19. Complete effort-wide metrics aggregate every associated session and trajectory under the explicit effort ID. Session replacement, manual structured resume, handoff, tree restoration, and subagent observations preserve that identity. The current-path projection remains trajectory-aware, while all-work totals retain discarded branches without double-counting.

20. Protocol-1 cleanup is confined to repository development data and generated fixtures during implementation. A preflight scans resident efforts; no data is deleted automatically when any protocol-1 effort exists, and the implementation stops with the existing confirmed purge path. Fixture replacement is ordinary tracked-file deletion. Protocol-2 startup never rewrites or guesses protocol-1 state.

21. Tests cover provisional commit, discard, overflow, graceful settlement, and crash-loss boundaries; explicit resume and active-branch restoration; optional memory and mismatch refusal; router discovery and hidden-body non-discovery; lifecycle mapping completeness and transactional phase handoff recovery; footer-parity accounting across restored history, compaction, current context, and nested subagents; widget placement and muted rendering; refresh generation ordering; finding-owned repair and waiver validation; protocol-2 privacy and no protocol-1 compatibility path; non-Pi target preservation; and real Pi smoke coverage for routing, replacement, accounting, and recovery. Affected templates preserve `missingkey=zero`, and empty-string render tests reject every no-value or unresolved token.

22. Each implementation batch updates the matching authored current-state claims and provenance, affected AGENTS.md convention parts and user documentation, generated topic and domain docs, and `docs/decisions/INDEX.md` in the same checked transaction. Every ADR status transition runs `./x sync`, stages the exact transaction, and passes `awf check --staged` and `./x gate`.

## State changes

- update `tooling/workflow-telemetry:event-protocol-and-ledger`
- update `tooling/workflow-telemetry:effort-lifecycle-and-routes`
- update `tooling/workflow-telemetry:privacy-integrity-and-retention`
- update `tooling/workflow-telemetry:canonical-projections-and-diagnostics`
- update `rendering/pi-workflows:pi-session-handoff-lifecycle`
- update `rendering/pi-workflows:pi-session-handoff-public-contract`
- update `rendering/pi-workflows:pi-session-handoff-workflow`
- update `rendering/pi-workflows:pi-workflow-dashboard-public-contract`
- add `rendering/pi-workflows:pi-lifecycle-enforcing-workflow-router`
- update `rendering/pi-runtime:pi-real-runtime-smoke`
- update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- update `rendering/adapter-outputs:pi-workflow-dashboard-runtime`
- update `rendering/singletons-and-payloads:workflow-telemetry-governed-outputs-and-resident-data`

## Consequences

Normal Pi workflow use produces deterministic effort, route, phase, activity, and terminal data without relying on an agent to remember a parallel lifecycle instruction. First-turn usage is retained without creating durable abandoned candidates for explicit resumes. Small work remains free of working-memory ceremony, while complex handoffs have a validated identity bridge.

Pi loses direct discovery of each individual workflow skill. The single router adds one selection step, but makes the enforcing boundary visible and keeps the full bodies fixed, generated, inspectable, and drift-checked. Target rendering and catalog metadata become responsible for lifecycle-mapping completeness.

Protocol 2 deliberately discards compatibility with unused protocol-1 data. This simplifies the permanent contract but requires local fixture and resident-data reset during implementation. The bounded provisional window accepts a narrow crash-loss risk in exchange for avoiding false durable efforts.

Footer-parity accounting depends only on Pi public runtime state. Subscription and automatic-context labels may remain absent until Pi exposes them safely. Canonical refresh remains asynchronous, but generation ordering and immediate validated local state prevent visibly regressing the workflow badge.

Repairs and waivers become stricter and may reject actions that the current UI attempts. That failure is preferable to appending a correction against the wrong effort, evidence, scope, reason, or frontier.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Add lifecycle instructions to every existing Pi skill | Direct loading and agent omission would still bypass accounting, so the ledger would remain advisory rather than deterministic. |
| Render skill templates dynamically inside the loader | Runtime interpolation would create a second render engine and weaken publication, drift, and provenance guarantees. |
| Create an effort and memory file at every fresh session start | Explicit resumes would leave spurious efforts, and small efforts would inherit unnecessary checkpoint ceremony. |
| Infer continuation from prompts or memory filenames | Heuristic identity can silently merge unrelated work and contradicts the explicit lifecycle authority established by ADR-0146. |
| Persist provisional observations to a spool | A spool adds another recovery and privacy-sensitive durable protocol for a deliberately narrow initialization window. |
| Keep protocol-1 compatibility and migrate resident ledgers | There is no resident production data to preserve, so the compatibility surface would have cost without value. |
| Reimplement footer accounting from dashboard-local counters | Local counters already diverged from restored history, compaction, context, and nested usage; Pi's public accounting sources are the authority. |
| Keep independent finish and start calls at handoffs | A crash or partial failure can leave a gap or overlap that no instruction can repair deterministically. |

## Status history

- 2026-07-23: Proposed
