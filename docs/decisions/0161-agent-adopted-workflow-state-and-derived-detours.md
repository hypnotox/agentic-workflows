---
format: current-state-v2
status: Implemented
date: 2026-07-26
---
# ADR-0161: Agent-adopted workflow state and derived detours


## Context

The protocol-2 Pi workflow router assumes that every in-flight effort began inside Pi and
therefore already has a resident effort, selected route, and causally open phase. That
assumption fails when another runtime creates a working-memory checkpoint before Pi has
recorded any metrics. `/awf-resume-effort` requires an existing effort ID and rejects an
absent projection, while `awf_workflow` can only move through the phase history already in
the ledger. Pi therefore cannot continue truthful middle-of-chain work from working memory.

The router also encodes a stricter linear chain than its generated skill bodies promise.
Loading debugging opens investigation, but debugging's required brainstorming successor is
mapped as a phase start and rejects the still-open investigation phase with `workflow start
requires no open phase`. A workflow already open at the requested phase cannot simply load
its body either: start mappings reject it, while current-phase task mappings manufacture a
`phase_transitioned` event from a phase to itself.

Real work is not wholly linear. Implementation can expose a blocker that warrants a new ADR,
planning can require an investigation, and a derived decision can itself encounter another
blocker. Allowing arbitrary phase jumps inside one effort would make route conformance and
phase accounting meaningless. Refusing every deviation instead makes the workflow unusable
when its own purpose is to surface new information.

Finally, the Pi `awf_metrics` and `awf_doctor` tools currently place the complete canonical
JSON projection in both model-visible content and tool details. One session-scoped query
returned long event-ID arrays and repository-wide `ambiguous-anchor` integrity records.
Eventless integrity notices survive selector filtering, so a narrow selector cannot bound the
result. The dashboard needs the full projection, but an agent does not.

The lifecycle protocol is append-only and already has resident protocol-2 data. The new
semantics can be expressed as new event kinds without changing the meaning of an existing
event. The current reader reports and skips an unknown kind, however, and can therefore
project a misleading partial effort rather than rejecting the whole effort. Protocol evolution
must close that unsafe partial-projection behavior as it adds the new kinds.

## Decision

1. Telemetry protocol 2.1 adds explicit agent-adoption, phase-continuation, derived-detour,
   and detour-return semantics. Existing protocol-2.0 events retain their meaning and require
   no migration or rewrite. A 2.1 reader accepts existing 2.0 history and suppresses lifecycle,
   metrics, diagnostics, and retention projection for an entire effort when any record carries
   an unsupported required kind or protocol interpretation; it reports one bounded
   schema-compatibility result instead of projecting the remaining records. A 2.0 reader does
   not have that effort-wide guard and must not be used after a 2.1 writer has appended new
   kinds. The pinned runtime handshake therefore requires protocol 2.1 before registration,
   and render/runtime advancement publishes the compatible reader before the writer. No
   dual-major projection or automatic resident-data migration is introduced.

   The closed additions are:

   | Event / action | Closed request and payload fields | Source, projection, and legality |
   |---|---|---|
   | `effort_adopted` / `adopt` | Request base plus optional `route`, required `phase`, bounded-category `workflow`, `trajectoryId`, and `anchorId`. Payload adds fixed `creationMode: adopted` and `associationOrigin: manual`; omitted route means unselected and the payload never carries an `unselected` route value. | Absent effort and empty predecessors. Alternative atomic creation record establishes adopted metadata, discovery when route is absent or active when present, one open phase whose start is this event, one trajectory, and the current session association. Metadata must match effort ID, timestamp, and adopted mode and has no origin. Lifecycle idempotency is the conflict key; repairable. |
   | `phase_continued` / `continue-phase` | Request base and payload require `phase`, `startEventId`, and bounded-category `workflow`; `activity` and bounded-category `implementationMode` are independently optional. Omission explicitly clears the corresponding current attribution rather than preserving it. | Discovery or active effort with exactly the named causally visible open phase/start and the current frontier. Preserves the unmatched start and interval and replaces workflow/activity/mode attribution. Lifecycle idempotency is the conflict key; repairable. |
   | `detour_started` / `start-detour` | The request base's `effortId` is the explicit child ID and its `sessionId` is the return session. Request and payload require fixed `creationMode: derived`, `origin` with `effortId`, `trajectoryId`, and `anchorId`, `returnPhase`, `returnPhaseStartEventId`, child `trajectoryId`, child `anchorId`, and fixed `workflow: brainstorming`; payload adds fixed `associationOrigin: detour`. | Absent child, empty child predecessors, and independently validated active parent named by origin. Alternative atomic child creation establishes derived metadata matching ID, timestamp, mode, and origin; metadata also stores return session, phase, and phase-start ID. Projection opens child brainstorming at this event, creates the child trajectory and association, and marks return pending. Lifecycle idempotency is the conflict key; repairable. |
   | `detour_returned` / `mark-detour-returned` | Request base and payload require `terminalOutcome` (`completed` or `abandoned`) and `parentAssociationEventId`. | Completed or abandoned pending-return child, current terminal child frontier, matching durable parent association event, and outcome matching the terminal event. The only new post-terminal effect marks return settled without changing terminal outcome. Lifecycle idempotency is the conflict key; not repairable. |

   `CreationMode` adds `adopted`; detours retain `derived`. `AssociationOrigin` adds
   `detour`. The descriptor retains one event kind for every lifecycle request action. Go and TypeScript
   readers generalize the creation-kind set to exactly `effort_created`, `effort_adopted`, and
   `detour_started`: exactly one valid creation-kind record must be the first record of one
   stream and match immutable metadata. Atomic create, identical retry, append rejection,
   projection, repair vocabulary, and reader integrity checks use that same set. All other new
   lifecycle events require the exact current causal frontier. Only `detour_returned`, repair,
   and waiver may follow a terminal event.

2. External working-memory continuation is agent-driven and adds no TUI. An agent pointed at
   a working-memory file reads it, establishes any missing identity, determines the route,
   phase, and semantic workflow, and normalizes the file's leading fields before adoption.
   Runtime code never infers identity from a filename, mines arbitrary prose, or chooses a
   route or phase for the agent.

3. A memory file eligible for adoption is a confined regular UTF-8 file below
   `.awf/memory/`, at most 1 MiB, with no symlink traversal. Within its first 16 KiB and before
   the first level-two heading it has exactly one line each, in this order, for `Effort:`,
   `Route:`, `Phase:`, `Workflow:`, and `Next:`; each line is at most 512 UTF-8 bytes, accepts
   LF or CRLF, and has one nonempty value after one ASCII space. Duplicate, reordered,
   malformed, invalid-UTF-8, or over-bound fields reject adoption. Effort is one bounded
   identifier not carried by any resident effort, tombstone, or other memory file. Route is
   one protocol route or the literal `unselected`. Phase is one protocol phase, Workflow is
   one enabled governed semantic skill, and Next is nonempty. The sibling uniqueness scan
   examines at most 256 confined regular `.md` files, rejects an unsafe entry or an exceeded
   bound, and ignores an unrelated sibling only when its bounded header contains no Effort
   field. Before invoking adoption, the agent updates legacy freeform headers into this
   normalized form and surfaces any materially uncertain interpretation to the user.

4. Pi exposes an exclusive `awf_adopt_effort` tool with a closed schema containing the
   memory path, effort ID, route or `unselected`, phase, and semantic workflow. Its preflight
   uses the same trustworthy single-tool-batch rule as `awf_workflow` and
   `handoff_session`. The runtime re-reads and confines the file, requires every submitted
   field to match its normalized header, and rejects an identity collision, an existing
   resident effort, a disabled or phase-incompatible workflow, or an invalid route/phase
   combination.

5. Successful adoption appends one idempotent `effort_adopted` lifecycle event as the effort
   creation commit. Its payload explicitly carries the selected route when any, current
   phase, governed workflow, new trajectory and anchor identity, and current session
   association. Projection treats all history before this event as unknown rather than as
   omitted required phases, opens exactly the asserted phase, and validates route order only
   from the adoption boundary forward. The tool persists the active-branch association and
   returns the fixed pre-rendered workflow body only after that event is durable.

6. The ledger and effort metadata store no memory path, filename, `Next` text, file content,
   or other repository path. The memory path is an ephemeral validation input only. An
   adoption failure before the event commit creates no effort; a failure after commit is
   recovered idempotently from the explicit effort ID and deterministic request identity.

7. Existing discovery or active ledger efforts continue through structured resume and are
   never re-adopted. Completed efforts require reopen, abandoned or pruned efforts remain
   non-resumable, and repair does not serve as an adoption shortcut. External adoption is
   the only operation that may establish an arbitrary truthful initial route and phase
   without a prior resident phase history.

8. Catalog workflow mappings separately declare an entry target, allowed entry predecessors,
   allowed continuation phases, route effect, activity, and implementation mode. Chain
   mappings continue only at their entry target. Support mappings may enter their dedicated
   phase when no phase is open and may list other phases in which they are lightweight
   current-phase activities. A support action with an independent outcome uses a detour
   instead. Adoption requires its asserted phase to be either the selected workflow's entry
   target or one of its explicit continuation phases. The router plans one of three effects:
   start the entry target when no phase is open and entry is legal; transition from an allowed
   predecessor; or continue an allowed already-open phase. Every other state fails with a
   bounded message naming the current phase, requested workflow, and legal next action.

   The revised standard mappings are:

   | Workflow | Entry phase and predecessors | Continuation phases | Route / activity / mode |
   |---|---|---|---|
   | brainstorming | brainstorming from no phase or investigation | brainstorming | none |
   | bugfix | brainstorming from no phase | brainstorming | select bugfix |
   | debugging | investigation from no phase | every protocol phase | debugging |
   | exploring | no entry | every protocol phase | exploration |
   | tdd | no entry | implementation | tdd |
   | refactor-coupling-audit | no entry | brainstorming | refactor-coupling-audit |
   | adr-lifecycle, roadmap-graduation | no entry | every protocol phase | their existing activity |
   | proposing-adr | adr-authoring from brainstorming | adr-authoring | select adr |
   | writing-plans | planning from brainstorming or adr-review | planning | existing ADR-plan promotion |
   | reviewing-adr | adr-review from adr-authoring | adr-review | none |
   | reviewing-plan | plan-review from planning | plan-review | none |
   | reviewing-plan-resync | adr-plan-resync from plan-review | adr-plan-resync | none |
   | executing-direct | implementation from brainstorming | implementation | select direct / inline-execution |
   | executing-plans | implementation from plan-review or adr-plan-resync | implementation | inline-execution |
   | subagent-driven-development | implementation from plan-review or adr-plan-resync | implementation | subagent-driven-development |
   | reviewing-impl | implementation-review from implementation | implementation-review | none |
   | retrospective | retrospective from implementation-review or investigation | retrospective | select investigation-only when unrouted / arm completion |

9. Protocol 2.1 adds `phase_continued`. It names the causally visible open phase and its
   unmatched start event and may set or clear the current activity and implementation mode.
   It does not finish, restart, or shorten the phase interval. The router uses it when a chain
   body is loaded at its already-open target and when debugging, exploration, TDD, or another
   support activity begins inside the current phase. A later transition or continuation
   updates attribution explicitly; no same-phase `phase_transitioned` event is emitted.

10. Brainstorming permits investigation as an allowed predecessor. A debugging-to-
    brainstorming handoff therefore appends one ordinary transactional transition from
    investigation to brainstorming. Loading brainstorming when brainstorming is already open
    appends `phase_continued`; loading it with no open phase starts it as before.

11. Normal route and phase conformance remains strict within one effort. Lightweight support
    work that does not own an independent outcome is an activity in the current phase. Work
    that has its own decision, plan, implementation, or terminal outcome is represented as a
    derived detour effort rather than an arbitrary phase jump in its parent.

12. Pi exposes an exclusive `awf_detour` tool whose closed input requires an explicit bounded
    `childEffortId` and selects the enabled governed reason workflow. Initially it admits
    brainstorming as the child entry workflow; later entries require their own reviewed mapping
    rather than bypassing brainstorming. The agent establishes and surfaces the child identity
    before the call. The tool requires one active associated parent, exactly one open parent
    phase, one active parent trajectory, and a valid current anchor. It rejects every resident
    effort, tombstone, or other pending detour with the child identity.

13. Starting a detour appends one idempotent `detour_started` creation event under the explicit
    child ID in a new derived child effort. Identical retries use the same child ID and lifecycle
    idempotency key and recover the matching child; a mismatched origin, return target, workflow,
    or creation payload is a visible identity conflict. The event carries immutable origin effort,
    trajectory, and anchor lineage plus the
    parent effort, parent trajectory, parent phase, parent phase-start event, return session,
    child trajectory, child anchor, and initial brainstorming association. The parent phase
    remains open and receives no fabricated transition. After durable child creation the tool
    persists the child association and returns the pre-rendered brainstorming body.

14. A detour child follows the ordinary strict workflow and may start another derived detour,
    producing a durable nested parent chain rather than a process-memory stack. Ordinary
    metrics continue to account parent and child separately, while canonical projection and
    dashboard presentation expose their lineage and terminal return status.

15. Successful child completion and explicit child abandonment both arm deterministic parent
    return settlement. Settlement validates that the recorded parent still exists, is active,
    has the named phase start open, retains the recorded session association, and owns the
    recorded active trajectory. The child never closed or mutated that parent trajectory, so
    return requires no fabricated trajectory resume. Settlement derives one parent
    session-association event and idempotency key from the child effort ID and terminal epoch,
    reads the parent's current frontier, and appends the association against that whole frontier
    with origin `detour`. If the frontier races before the append, no identity is reserved and a
    retry revalidates and joins the new whole frontier. Once the deterministic association exists,
    later parent events do not invalidate it and retries recognize it by idempotency key instead
    of requiring it to remain at the frontier. Settlement then appends a `detour_returned`
    marker to the terminal child naming that parent association event and terminal outcome and
    persists the restored parent custom association. Completion does not reopen or restart the
    parent phase. A terminal detour child remains pending-return and is excluded
    from retention selection and prune rechecks until `detour_returned` is durable; an invalid
    parent therefore cannot be pruned merely because automatic return is blocked.

16. Parent return is a retryable cross-effort state machine, not a claimed atomic write across
    two ledgers. The child terminal event is its commit boundary. A crash before the parent
    events retries them; a crash after parent association but before the child marker or custom
    entry detects the deterministic events and finishes settlement. Until settlement succeeds,
    the terminal child remains visibly selected and recoverable. An invalid or no-longer-
    resumable parent fails visibly and is never replaced by a guessed association.

17. Retrospective's automatic completion runs detour return settlement after the child
    terminal event. An explicit `awf_lifecycle abandon` does the same when the active effort
    is a detour child. Repeated terminal acknowledgments or startup recovery produce at most
    one logical parent return. A normal independent or non-detour derived effort retains its
    existing terminal behavior.

18. The dashboard renders the active effort and compact parent or child relationship and
    exposes full adoption-boundary, detour lineage, pending return, and returned state in its
    existing overlay. The below-editor badge remains phase-oriented and does not grow an
    unbounded ancestry display.

19. Pi continues to invoke canonical `awf metrics --json` and `awf doctor --json` internally
    for the dashboard. Those parsed objects may remain in private in-process dashboard state,
    but raw canonical JSON or an equivalent full object never appears in model-visible tool
    content or tool details.

20. `awf_metrics` returns a plain-text compact summary containing selected effort count,
    current effort state, route and phase when available, and aggregate usage and counters.
    It includes no event IDs, per-session/phase/trajectory event sets, evidence, raw integrity
    notices, or repository paths. Multiple efforts are sorted deterministically and capped at
    exactly eight rows. Every displayed identifier remains within the protocol's 128-byte
    bound, the normalized selector summary is capped at 256 UTF-8 bytes, and the complete
    model-visible response is capped at 4096 UTF-8 bytes.

21. `awf_doctor` returns plain-text counts grouped by severity and rule code, integrity counts
    grouped by integrity code, and at most five actionable findings. Each displayed finding
    contains only a protocol-bounded effort ID and code plus explanation and next action fields
    separately capped at 512 UTF-8 bytes. It includes no event IDs, counter IDs, evidence
    arrays, per-scope integrity records, repository paths, or raw JSON, and the complete
    model-visible response is capped at 4096 UTF-8 bytes.

22. Query-tool details contain only a versioned compact-result marker, a normalized selector
    summary capped at 256 UTF-8 bytes, truncation flag, and displayed counts. Their serialized
    encoding is independently capped at 1024 UTF-8 bytes and never retains the raw parsed
    projection. A truncated or detail-bearing summary directs the operator to
    `/awf-dashboard`; the Pi tools add no pagination or full-detail flag. CLI JSON remains
    available for implementations and scripts and is not an agent-context presentation format.

23. Compact serialization constructs output from an allowlist rather than redacting a raw
    object. It strips control characters and path-shaped substrings from canonical explanation,
    action, selector, and error text before deterministic ordering, applies per-field and item
    limits, then applies its byte ceiling with Unicode-safe truncation while reserving space for
    an explicit truncation notice. It never includes raw stdout or stderr. A malformed or
    incompatible canonical response produces one bounded degraded error through the same
    allowlist rather than falling back to process output.

24. Tests first reproduce the absent-effort resume rejection, debugging-to-brainstorming
    failure, mapped-phase continuation failure, partial projection after an unsupported record,
    and raw query flood. Protocol and projection tests then cover the complete request/event
    table, alternative creation invariants, 2.0 preservation, whole-effort incompatible-kind
    suppression, honest adoption boundaries, phase-start preservation, detour nesting, pending
    return retention exclusion, and invalid parents. Fault tests interrupt completion and
    abandonment before and after the child terminal event, each parent event, the child marker,
    and the custom association. Pi tests cover bounded memory grammar, confinement, tombstones,
    sibling identity matching, exclusive batches, idempotent recovery, compact allowlists and
    byte ceilings, dashboard detail retention, generated output, and the minimum supported real
    Pi runtime.

25. Authored changes land in `internal/telemetry`, `internal/catalog`, Pi and workflow
    templates, tests, and `.awf/` current-state sources. The two added current-state claims use
    `Backing: test`, with proof markers on the adoption confinement/idempotency and detour
    return/recovery suites. Generated `.pi` artifacts, rendered docs, indexes, and `AGENTS.md`
    change only through `./x render`; the matching authored `.awf/` agent-guide convention
    source gains the external-adoption and detour routing rule in the same implementation batch.
    Every ADR status transition regenerates and stages `docs/decisions/INDEX.md`; every
    implementation batch updates docs and current-state authority in the same checked
    transaction and does not edit resident metrics.

## State changes

- update `tooling/workflow-telemetry:event-protocol-and-ledger`
- update `tooling/workflow-telemetry:effort-lifecycle-and-routes`
- update `tooling/workflow-telemetry:trajectory-and-derived-effort-model`
- add `tooling/workflow-telemetry:external-adoption-boundary`
- add `tooling/workflow-telemetry:derived-detour-return`
- update `tooling/workflow-telemetry:privacy-integrity-and-retention`
- update `tooling/workflow-telemetry:canonical-projections-and-diagnostics`
- update `rendering/pi-workflows:pi-lifecycle-enforcing-workflow-router`
- update `rendering/pi-workflows:pi-workflow-dashboard-public-contract`
- update `rendering/guide-and-doc-templates:working-memory-single-home`
- update `rendering/adapter-outputs:pi-workflow-dashboard-runtime`
- update `rendering/pi-runtime:pi-real-runtime-smoke`

## Consequences

A Pi agent can continue an explicitly identified external checkpoint without asking an
operator to reconstruct telemetry state or pretending earlier phases occurred. Governed
workflow bodies become resumable at their current phase, and the debugging handoff matches
its own instructions. Strictness remains meaningful because ordinary deviations become
separate derived efforts with explicit lineage and return behavior.

The protocol and extension become more complex. Adoption creates a trusted boundary from an
agent assertion, mitigated by normalized memory headers, exact runtime matching, an exclusive
tool call, and visible unknown prior history. Parent return spans two append-only ledgers and
cannot be physically atomic, mitigated by deterministic identities, a terminal-child commit
boundary, durable return metadata, and startup recovery.

Protocol-2.0 readers may skip a 2.1 record and misleadingly project the remaining effort.
The 2.1 reader closes that defect for future unknown required records, and the pinned runtime
requires a 2.1 reader before it registers a writer. Using an older binary against new resident
data is unsupported during stabilization. Existing 2.0 history remains readable by the new
implementation and is not rewritten merely to preserve an immature metrics installation.

Agent-facing query tools become deliberately less expressive. Detailed evidence remains
available in the dashboard and programmatic CLI JSON, while routine model context receives a
small deterministic summary that cannot grow with ledger history or global integrity noise.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Require an operator TUI to select adopted state | The memory checkpoint already gives the agent the required context, and another TUI surface adds friction without improving the explicit runtime validation boundary. |
| Let the extension infer identity, route, or phase from a filename or prose | Heuristic adoption can merge unrelated work and repeats the unsafe identity inference rejected by the existing lifecycle design. |
| Synthesize all phases before the adopted current phase | Fabricated history would corrupt metrics and falsely claim reviews or decisions occurred. |
| Permit arbitrary phase jumps inside one effort | Route conformance and phase accounting would stop distinguishing a disciplined workflow from ad hoc movement. |
| Model blockers as nested phases in one effort | A phase stack entangles independent outcomes and makes nested interruption, review, abandonment, and accounting harder than explicit derived lineage. |
| Keep support activities as same-phase transitions | Closing and reopening the same phase invents an interval boundary and caused the router's transition semantics to carry two meanings. |
| Return a workflow body without a durable continuation event | It would weaken the router's acknowledged-load boundary and lose activity attribution. |
| Bump to protocol 3 | Existing protocol-2 efforts would require dual-major readers, migration, or abandonment even though the new semantics can be isolated in additive event kinds; pinned 2.1 reader-before-writer advancement and whole-effort incompatibility suppression address the unsafe old-reader behavior going forward. |
| Remove the Pi metrics and doctor tools | Compact summaries remain useful to agents; only full canonical projections belong exclusively in the dashboard and programmatic interfaces. |
| Add pagination or a full-output flag to the Pi tools | The dashboard already owns detailed inspection, while another agent-facing expansion path would recreate the context-flood risk. |

## Status history

- 2026-07-26: Proposed
- 2026-07-26: Accepted; content-sha256: ff60f69f266ad7356531e3bd8e1b7b533b77eff7002a2ed3f33ec13b1d6c1164
- 2026-07-26: Implementing; content-sha256: ff60f69f266ad7356531e3bd8e1b7b533b77eff7002a2ed3f33ec13b1d6c1164
- 2026-07-26: Applied; state-sequence: 51; operations: update `tooling/workflow-telemetry:event-protocol-and-ledger`
- 2026-07-26: Applied; state-sequence: 52; operations: update `tooling/workflow-telemetry:effort-lifecycle-and-routes`, update `tooling/workflow-telemetry:trajectory-and-derived-effort-model`
- 2026-07-26: Applied; state-sequence: 53; operations: add `tooling/workflow-telemetry:external-adoption-boundary`
- 2026-07-26: Applied; state-sequence: 54; operations: add `tooling/workflow-telemetry:derived-detour-return`
- 2026-07-26: Applied; state-sequence: 55; operations: update `tooling/workflow-telemetry:privacy-integrity-and-retention`, update `tooling/workflow-telemetry:canonical-projections-and-diagnostics`
- 2026-07-26: Applied; state-sequence: 56; operations: update `rendering/pi-workflows:pi-lifecycle-enforcing-workflow-router`, update `rendering/pi-workflows:pi-workflow-dashboard-public-contract`, update `rendering/guide-and-doc-templates:working-memory-single-home`, update `rendering/adapter-outputs:pi-workflow-dashboard-runtime`, update `rendering/pi-runtime:pi-real-runtime-smoke`
- 2026-07-26: Implemented; content-sha256: ff60f69f266ad7356531e3bd8e1b7b533b77eff7002a2ed3f33ec13b1d6c1164
