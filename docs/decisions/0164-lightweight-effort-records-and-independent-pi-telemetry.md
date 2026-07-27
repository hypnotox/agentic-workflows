---
format: current-state-v2
status: Implementing
date: 2026-07-27
---
# ADR-0164: Lightweight effort records and independent Pi telemetry

## Context

The current protocol-2 telemetry ledger is simultaneously an effort registry, a causal
workflow-enforcement system, a Pi session-association store, a metrics input, and the source of
handoff and adoption validation. ADR-0149 and ADR-0161 made those responsibilities increasingly
rigid: an effort must be created or adopted through Pi, workflow entry mutates lifecycle state,
and an external checkpoint requires a constrained five-field adoption boundary. This makes a
simple local unit of work depend on Pi's private database and on route, phase, trajectory, and
review accounting that are not necessary for managing work.

Git worktrees have a related gap. Agents can create them with Git, but awf has no durable,
repository-owned effort record, no standard managed location, and no safe command surface for
attaching, integrating, retaining, or removing an effort worktree. A checkout-local `.awf/`
root also splits resident state across linked worktrees even though an effort and its optional
resources must survive session replacement and handoff.

The useful concerns are smaller and separate: local orchestration needs a durable effort record;
Pi needs optional selected-effort context; and telemetry needs a privacy-minimal append-only
session stream. The latter must remain useful without asserting that a telemetry event belongs to
the effort that happened to be selected when it was written. Existing resident telemetry must be
preserved rather than rewritten into the new records.

## Decision

1. The awf binary owns lightweight local effort records under `.awf/efforts/<effort-id>.json`.
   It is the sole allocator of stable effort IDs and atomically replaces current-state records.
   A record contains only practical identity and status: a required meaningful nonblank title,
   creation and update times, active, completed, or abandoned state, memory presence, optional
   managed-worktree metadata, integration disposition, and assigned Pi session IDs. Git-tracked
   code, ADRs, plans, and documentation remain durable project truth; records are local
   orchestration state, not a history or an authority over them.

2. The effort, memory, worktree, assignment, and telemetry resident roots are repository-wide and
   resolve from one confined primary control root derived from Git's common directory. Invoking
   worktrees still supply the tracked config and workflow authority. The binary rejects
   unconfinable, symlinked, or foreign-owned paths; those safety failures are never forceable.

3. `awf effort new <title>` creates an effort and memory file by default, with `--no-memory` for
   benign work. It creates no worktree unless `--worktree` is supplied. Agent-oriented commands
   provide list, show, rename, memory creation, worktree add and remove, integration and manual
   integration recording, complete, abandon, reopen, repair, and session assignment management.
   Rename changes display metadata only. Creation, selection, assignment, and resumption are
   ordinary operations at any time: there is no adoption event, fabricated history, or workflow
   precondition.

4. A managed worktree is opt-in and lives at `.awf/worktrees/<effort-id>/`. It branches from the
   caller's HEAD unless an explicit base is supplied, remains after completion and across sessions,
   and is removed only by an explicit ownership, integration, and cleanliness-checked operation.
   Managed integration is explicit: fast-forward when possible, otherwise a normal merge commit;
   manual integration may be recorded. Integration disposition remains on the effort after removal,
   a newly attached worktree resets it to pending, and reopening an effort does not change it.
   Recoverable cleanliness or topology refusal identifies the risk and permits
   `--force --reason <truthful explanation>`; destructive topology operations retain strict checks.
   Native Git commands implement linked-worktree mutations because the existing Go Git dependency
   is read-only for this purpose. If `effort new --worktree` creates the effort but worktree creation
   fails, the effort and its memory remain available, the failure reports the allocated ID, and the
   agent may retry worktree attachment or repair the record; the binary never hides a partial Git
   mutation behind a claimed rollback.

5. Pi may select one optional effort for its session through `/awf-effort new <title>`, `use <id>`,
   `clear`, `show`, and `rename <title>`. Repository-wide current-state assignment permits a Pi
   session to belong to exactly one effort at a time; reassignment and retrospective assignment
   are explicit and atomic. Multiple efforts in one session are unsupported. Pi never implicitly
   creates an effort, and governed workflow entry neither requires a selected effort nor mutates
   effort lifecycle state.

6. Remove Pi's effort database, projector, lifecycle/adoption tools, trajectory and detour model,
   and lifecycle-enforcing router behavior. The router continues to deliver fixed governed
   workflow guidance, but a selected effort is optional context only. Handoffs validate an
   optional selected-effort/memory relationship without adopting a checkpoint or requiring a
   telemetry lifecycle association.

7. Pi telemetry becomes an independent versioned, append-only, session-keyed stream under
   `.awf/metrics/`. Records contain session identity and closed privacy-minimal usage, tool, gate,
   subagent, compaction, and handoff observations, never an effort ID, route, phase, trajectory,
   repository path, command argument, conversational text, or tool input/output. Pi appends them
   directly rather than through the awf binary. The writer holds an exclusive per-session stream
   lock, validates the versioned header and prior records, appends one complete observation carrying
   a stable observation ID, and fsyncs it before reporting success. Retrying that ID is idempotent;
   an unsafe lock, incomplete write, or malformed stream is a refusal that preserves the stream for
   repair rather than silently truncating or guessing. Reports join new session observations to the
   current effort assignment; this intentionally permits a later reassignment to change the
   reporting association. `awf metrics doctor` remains only as deterministic stream-integrity
   diagnosis, not lifecycle or heuristic workflow diagnosis. Existing protocol-1 and protocol-2
   local telemetry remains byte-preserved and read-only and is neither migrated into effort records
   nor deleted by this decision.

8. Retire the tracked `workflowTelemetry` configuration block through the next normal schema
   migration. Its retention, widget, baseline, and heuristic thresholds have no truthful consumer
   after the lifecycle projector and phase widget are removed. The migration removes only that
   known block, preserves unrelated configuration and historical resident telemetry, and advances
   the lock generation and minimum binary version in the ordinary upgrade transaction.

## State changes

- add `tooling/effort-management:effort-record-authority`
- add `tooling/effort-management:managed-worktree-lifecycle`
- add `tooling/effort-management:session-effort-assignment`
- remove `config/configuration:workflow-telemetry-settings`
- update `config/migrations-and-locks:workflow-telemetry-config-migration`
- update `tooling/workflow-telemetry:event-protocol-and-ledger`
- remove `tooling/workflow-telemetry:effort-lifecycle-and-routes`
- remove `tooling/workflow-telemetry:trajectory-and-derived-effort-model`
- remove `tooling/workflow-telemetry:external-adoption-boundary`
- remove `tooling/workflow-telemetry:derived-detour-return`
- remove `tooling/workflow-telemetry:anchor-claims-and-location-metadata`
- update `tooling/workflow-telemetry:privacy-integrity-and-retention`
- update `tooling/workflow-telemetry:canonical-projections-and-diagnostics`
- add `tooling/cli:effort-command-contract`
- update `tooling/cli:metrics-command-contract`
- update `rendering/singletons-and-payloads:workflow-telemetry-governed-outputs-and-resident-data`
- update `rendering/singletons-and-payloads:memory-gitignore-always-on`
- update `rendering/project-output-plan:output-plan-complete`
- update `rendering/guide-and-doc-templates:working-memory-single-home`
- update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- update `rendering/adapter-outputs:pi-workflow-telemetry-runtime`
- update `rendering/pi-workflows:pi-session-handoff-lifecycle`
- update `rendering/pi-workflows:pi-session-handoff-public-contract`
- update `rendering/pi-workflows:pi-session-handoff-workflow`
- update `rendering/pi-workflows:pi-lifecycle-enforcing-workflow-router`
- update `rendering/pi-workflows:pi-workflow-telemetry-public-contract`
- update `rendering/pi-runtime:pi-extension-target-render`
- update `rendering/pi-runtime:pi-real-runtime-smoke`

## Consequences

Efforts become useful outside Pi and without a workflow lifecycle, while agents gain a durable,
explicit place for optional memory and managed worktrees. The resident roots are stable across
linked worktrees, but implementation must carefully distinguish the common control root from the
invoking worktree's tracked configuration.

Pi has fewer failure modes and less private state, but it no longer enforces workflow discipline
or provides lifecycle-derived reports. Session reassignment deliberately changes report joins, so
one Pi session cannot accurately represent concurrent efforts. Worktrees consume disk and remain
until explicitly removed; explicit retention is safer than automatic deletion. Removing the
workflowTelemetry block is an adopter-visible config migration, but retaining settings with no
consumer would misrepresent the supported controls.

The scope replaces several load-bearing contracts and crosses the binary, native Git, Pi, rendered
outputs, and documentation. A staged plan and comprehensive safety tests are required before
implementation.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the telemetry causal ledger as the effort registry | Its lifecycle, trajectory, lease, and projection requirements make ordinary local orchestration unnecessarily rigid. |
| Create a worktree for every effort | Many benign efforts need neither a branch nor an isolated checkout; attachment must be opt-in. |
| Keep checkout-local resident roots | Linked worktrees would split one effort's durable resources and assignment state. |
| Retain adoption and workflow-gated effort creation | It forces artificial history and makes valid independent or resumed work fail at an administrative boundary. |
| Put effort IDs in telemetry observations | Session-to-effort assignment can happen later and avoids coupling append availability to effort management. |
| Keep an awf-owned telemetry append boundary | Sending observations through an awf command or service centralizes stream writes, but makes Pi telemetry depend on binary availability, version compatibility, and a new IPC or process-failure boundary. A locked, fsynced direct writer keeps the stream independent while retaining explicit integrity checks. |
| Allow one Pi session to map to several efforts | It makes report attribution and totals ambiguous; explicit reassignment is the simpler operator-controlled model. |
| Preserve the workflowTelemetry settings as inert compatibility fields | Strict accepted configuration implies supported behavior; retaining controls for removed retention, lifecycle diagnosis, and widgets would be misleading. |

## Status history

- 2026-07-27: Proposed
- 2026-07-27: Implementing; content-sha256: 13fe97f744f2a7c14fe59a7d8b2c117f5267d481885409d99c91525d461a70c2
- 2026-07-27: Applied; state-sequence: 61; operations: add `tooling/effort-management:effort-record-authority`
