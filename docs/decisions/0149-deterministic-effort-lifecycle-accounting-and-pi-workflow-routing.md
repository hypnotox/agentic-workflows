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

2. A fresh root Pi session begins with a provisional candidate effort held only in bounded process memory. The dashboard displays `[awf:init]` and buffers privacy-filtered observations until the first workflow selection or explicit resume operation resolves identity. Workflow selection atomically creates and associates the candidate, then flushes the buffered observations in order. Explicit resume discards the candidate and flushes the observations to the selected existing effort.

3. The provisional buffer has a fixed item and byte bound. Exceeding either bound durably commits the candidate as an init effort, flushes the buffer, and emits a visible warning. Normal session replacement and graceful shutdown attempt settlement. A process crash before settlement may lose only this bounded provisional window. Append-only observations already committed by overflow are never reassigned by a later resume.

4. `/resume` restores the active-branch association directly from the persisted custom entry. A fresh `/new` continuation uses a closed structured resume-effort operation with an explicit effort ID. The runtime does not mine prompts, filenames, arbitrary prose, session ancestry, or repository state for identity.

5. Working memory is optional and one-way. Creating an effort does not create a memory file and effort metadata stores no memory path. When a workflow later creates or updates `.awf/memory/<slug>.md`, the file carries `Effort: <id>`. A structured resume may use that field after validating the memory file, but the ledger never requires the file to exist.

6. `handoff_session` remains available only for memory-backed efforts. It requires a validated memory file carrying `Effort: <id>`, copies the association before child startup independently of dashboard factory load order, restores it before kickoff, and refuses an effort mismatch. Checkpoint-less efforts continue in the same session or through explicit resume rather than synthesizing a handoff checkpoint.

7. Pi exposes exactly one discoverable workflow skill at `.pi/skills/awf-workflow/SKILL.md`. It contains a concise catalog that lets the agent choose a governed chain or task skill by semantic name. Reviewer agents remain separately rendered and discoverable. Existing individually discoverable Pi workflow and task skill files are removed.

8. `awf sync` still pre-renders every governed Pi workflow and task skill body at generation time, but writes the bodies to a managed non-discovered tree under `.pi/awf-workflows/`. They carry normal provenance and drift ownership and are never template-rendered, interpolated, or assembled at runtime. Non-Pi targets continue to render their ordinary individually discoverable skills and equivalent target-native lifecycle guidance.

9. A generated `awf_workflow` Pi tool is the only supported loader for governed hidden bodies. Its input is a closed enum derived from enabled catalog metadata. The same metadata maps each semantic skill to route selection or change, phase transition, activity, implementation mode, and terminal behavior. The tool validates the requested skill, settles or resumes provisional identity, applies the mapped lifecycle mutation, and only then returns the fixed pre-rendered body.

10. Direct attempts to load a governed hidden body are blocked or redirected to `awf_workflow`; a filesystem path is never accepted as a loader input. The router may describe skill selection but may not return a hidden body itself. Catalog and render checks prove that every enabled governed skill has exactly one hidden body and one lifecycle mapping, and that no stale individually discoverable Pi copy remains.

11. Chain handoffs use one high-level transactional lifecycle operation that closes the causally visible current phase and starts the successor without an intentional open-phase gap. The operation has one idempotency key, current causal frontier, recovery semantics, and a projection effect that is all-or-recoverable rather than two independently acknowledged mutations. Route selection starts the first real phase from init. Task skills record their mapped activities without inventing top-level phase changes.

12. Loading a workflow skill is the enforcement point, not a second set of agent instructions. Retrospective completion is appended mechanically after the successful settled retrospective run. Abandonment is an explicit terminal action. The badge states are `[awf:init]`, `[awf:<phase>]`, `[awf:done]`, and `[awf:abandoned]`; unassociated or incompatible states are exceptional and carry a compact degraded suffix rather than masquerading as init.

13. Local lifecycle state updates the badge immediately after a validated append. Canonical metrics and doctor refresh asynchronously at startup, overlay open, lifecycle settlement, bounded passive-observation boundaries, and explicit refresh. Refreshes are coalesced, and a monotonic generation or epoch prevents an older completion from overwriting newer local lifecycle or projection state. An open overlay receives the same refresh notifications.

14. The compact widget renders below the editor in a muted style. Its information shape follows the Pi footer: input, output, cache-read, cache-hit percentage, permitted cost, subagent marker, and context percentage and window. It uses Pi session entries plus `ctx.getContextUsage` so restored history, summaries, compactions, and current context match footer accounting. It shows subscription or automatic-context indicators only when Pi exposes them through a public and safe API; otherwise those fields are omitted rather than inferred from internals.

15. Subagent tool results return Pi's top-level `usage` shape in addition to bounded details, so Pi's own totals include nested work. The telemetry observation retains only the protocol-approved aggregate fields and never the subagent task, report, transcript, tool arguments, or diagnostics text.

16. Canonical findings carry their owning `effortId`. Repair and waiver actions resolve the selected finding from the selected effort, use the current causal predecessor frontier, validate the rule-authorized reason and scope, and reject stale or cross-effort actions. They never manufacture an empty frontier merely because the UI lacks cached event IDs.

17. Complete effort-wide metrics aggregate every associated session and trajectory under the explicit effort ID. Session replacement, manual structured resume, handoff, tree restoration, and subagent observations preserve that identity. The current-path projection remains trajectory-aware, while all-work totals retain discarded branches without double-counting.

18. Tests cover provisional commit, discard, overflow, graceful settlement, and crash-loss boundaries; explicit resume and active-branch restoration; optional memory and mismatch refusal; router discovery and hidden-body non-discovery; lifecycle mapping completeness and transactional phase handoff recovery; footer-parity accounting across restored history, compaction, current context, and nested subagents; widget placement and muted rendering; refresh generation ordering; finding-owned repair and waiver validation; protocol-2 privacy and no protocol-1 compatibility path; publication-safe empty rendering; and non-Pi target preservation.

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
