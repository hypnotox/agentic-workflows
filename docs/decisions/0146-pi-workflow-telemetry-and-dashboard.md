---
format: current-state-v2
status: Implementing
date: 2026-07-22
---
# ADR-0146: Pi Workflow Telemetry and Dashboard


## Context

awf guides an effort through named workflow phases, but it has no durable, structured account of which route the effort took, how work and model usage were distributed, where handoffs or forks occurred, or which deviations may indicate a workflow problem. Pi exposes useful per-session model, token, cache, cost, tool, and failure data, and the subagent extension already reports a subset of it, but those observations end with the session and do not share a stable effort identity. Working-memory files cannot be the authority: they are prose, are intentionally deleted after retrospective, and record a checkpoint rather than an event history.

A useful dashboard must span the parent-linked sessions that now form one workflow effort without collecting conversational content. It must distinguish current-path work from work on discarded Pi tree branches, distinguish a resumed effort from a new effort derived from prior analysis, and diagnose both exact workflow-order violations and weaker statistical signals. The same interpretation must serve the TUI, automation, and offline analysis; duplicating aggregation or diagnosis in TypeScript would allow those surfaces to disagree.

The generated Pi extensions are executable standard artifacts installed into adopter repositories. The design must therefore cover output governance, runtime compatibility, ignored resident data, concurrent session writers, corrupt or newer event data, retention, and privacy rather than treating the feature as a repository-local visualization. Passive observation must not interrupt work, while explicit lifecycle changes must not pretend to succeed when they were not durably recorded.

## Decision

1. awf will ship workflow telemetry and its Pi dashboard as standard functionality for Pi-enabled adopter projects. The implementation introduces a Go telemetry package as the canonical protocol, reader, projection, selection, aggregation, retention, and diagnostic engine, and a generated `.pi/extensions/awf-dashboard/index.ts` extension as the Pi runtime writer and UI surface. TypeScript writes are conforming protocol implementations, not a second projection or diagnostic engine.

2. The authoritative history is an append-only, versioned JSONL event ledger under `.awf/metrics/efforts/<effort-id>/sessions/`, with one stream per Pi session to avoid cross-session writer contention. Optional aggregate caches are disposable and never authoritative. `.awf/metrics/**` is an explicit ignored dynamic-resident closed-tree exemption. Only its generated `.gitignore` is governed output; runtime descendants never enter the manifest, drift set, or cleanup ownership.

3. A single machine-readable protocol descriptor owned by the Go telemetry package is normative for protocol versions, event kinds, routes, phases, activities, bounded categories, identifier limits, and payload shapes. Go validation consumes that descriptor, and Pi target rendering projects it into TypeScript constants, types, and validators. The descriptor participates in render input attribution and hashing, with cross-language golden tests preventing protocol drift.

4. Long-lived events are privacy-minimal. They may contain timestamps, opaque bounded effort, session, Pi entry, and trajectory identifiers, model and tool names, durations, token, cache, and cost totals, categorized outcomes and errors, and bounded workflow counters such as gates and commits. They never contain prompts, assistant text, tool arguments, command output, conversational waiver prose, or repository paths other than the bounded effort checkpoint identifier. Arbitrary identifiers and category-like values are length-bounded and validated.

5. Effort creation writes immutable `effort.json` metadata containing identity, creation time, checkpoint identifier, and optional immutable origin metadata. All mutable effort, route, phase, association, repair, waiver, and terminal state is represented by events; no mutable snapshot rewrites history.

6. An effort begins in an undecided discovery state. A closed route is selected only when evidence supports one of `direct`, `adr`, `plan`, `adr-plan`, `bugfix`, or `investigation-only`; `investigation-only` is valid only when investigation concludes without requested implementation. Later scope changes append `route_changed`. Abandonment is terminal from any route.

7. The top-level phase vocabulary is investigation, brainstorming, ADR authoring, ADR review, planning, plan review, ADR-plan resync, implementation, implementation review, and retrospective. Debugging is an investigation activity, TDD is an implementation activity, exploration belongs to the phase that requested it, bugfix is a route spanning implementation, review, and retrospective, and plan execution styles are implementation modes rather than phases.

8. Stable effort identity and exact phase transitions are changed only through explicit lifecycle operations. The system does not infer them from working-memory prose, prompts, filenames, or ambient session activity. Explicit lifecycle writes fail visibly if durable append fails; passive telemetry failures are non-blocking and surface degraded status and diagnostics.

9. Pi effort and session association is persisted only in a TUI custom session entry. The handoff extension copies the association in `newSession.setup`, and restoration reads only entries on the active branch. The runtime never guesses an association from session ancestry, working-memory prose, or repository state. Dashboard controls may explicitly associate, detach, or repair state.

10. The ledger models Pi tree navigation with trajectories. Events carry an opaque Pi anchor entry ID, trajectory ID, parent trajectory ID, and fork anchor where applicable. `/tree`, `/fork`, and `/clone` close the current trajectory segment and either resume the selected existing active-branch trajectory or append `trajectory_forked`; selecting an anchor before association detaches rather than guessing.

11. Current-path aggregates follow the active trajectory ancestry, while effort totals include every trajectory and retain discarded-branch work. Neither branch navigation nor repair rewrites historical events. Dashboard projections make the distinction visible so discarded work is neither lost nor silently charged twice to the active path.

12. Continuing work from a terminal effort is explicit. The default for tackling a new issue from completed analysis is a new `derived` effort with immutable opaque origin effort, trajectory, and anchor metadata and independent lifecycle and totals. `independent` starts unrelated work. `reopen` is allowed only for an unpruned completed effort and explicitly creates a new trajectory in that effort; abandoned and pruned efforts can only be origins for a new effort. History may group derived families but never double-count parent analysis cost.

13. Writers use a serialized append queue per stream and drain it during extension shutdown. Appends are line-oriented and flush safely. Readers ignore and report only a corrupt final partial line, report other integrity failures, and reject unsupported protocol interpretations explicitly rather than guessing. Retention coordinates with late writers to terminal efforts so pruning cannot race an active append into deleted state.

14. Repairs append narrowly scoped corrective events rather than editing or deleting history. Diagnostic results may carry typed remediation proposals, but the doctor remains read-only. An explicit lifecycle repair action applies a selected proposal, and destructive cleanup requires confirmation.

15. Retention applies deterministic configurable maximum-age and maximum-count limits to completed efforts while preserving active efforts. Pruning order and tie-breaking are stable, late-writer safety is enforced, and pruning is invoked by the dashboard runtime and explicit metrics maintenance surfaces. Retention never depends on retrospective deleting working memory.

16. `awf metrics` is the canonical query, aggregation, export, and retention command family. `awf doctor` is the canonical read-only diagnostic command. Both support human and `--json` output and share storage, selectors, projections, and rule machinery. Common structured selectors cover effort, session, phase, and time windows; metrics and doctor remain separate concepts and commands.

17. Diagnostics have two tiers. Exact rules identify workflow-order and data-contract violations. Heuristic rules expose potential inefficiency or unusual behavior using configurable absolute defaults and historical comparable-effort baselines only after the configured minimum sample size is met. There is no opaque composite health score.

18. Initial exact diagnostics cover phase ordering, required reviews and ADR-plan resync, terminal implementation review and retrospective, phase overlap, handoff association, event integrity, and schema compatibility. Initial heuristics cover phase reentry, duration and usage, compaction and handoff density, tool failures, gate churn, cache reuse, subagent queue wait, and implementation rework.

19. Every finding has a stable versioned rule code, rule type, severity, scope, event or counter evidence, threshold or baseline, confidence, explanation, next action, and optional typed reconciliation proposal. Exact violations and heuristic signals remain distinguishable in both human and JSON projections.

20. Diagnostics are non-blocking by default. Human-approved exceptions append narrowly scoped typed waiver events, such as `benign-followup-no-rereview`, without free-form conversational prose; waived deviations remain informational. Any future non-zero diagnostic policy is explicit opt-in, such as `--fail-on violation`, and the dashboard never blocks workflow execution.

21. The Pi extension registers structured lifecycle, metrics, and `awf_doctor` tools rather than accepting arbitrary CLI strings. `awf_doctor` consumes the same canonical doctor result and exposes structured selectors. Shell-derived workflow counters use a narrow allowlist classifier; ambiguous commands remain unclassified rather than being interpreted heuristically.

22. The Pi UI consists of a compact always-visible active-effort widget plus an on-demand `/awf-dashboard` interactive overlay for current effort, trajectory, phase usage, history, findings, retention state, and explicit repair or association controls. It does not replace Pi's footer.

23. The TypeScript extension obtains canonical projections by invoking the verified `awf metrics --json` and `awf doctor --json` surfaces at startup, overlay open or refresh, and relevant state changes, then caches results in memory. It resolves the binary through `.awf/bootstrap.sh` when present and otherwise through `awf` on `PATH`; it never depends on optional `./x`. A protocol handshake precedes projection calls. A missing or incompatible binary leaves direct conforming lifecycle writes registered but disables canonical query and doctor tools with an explicit degraded result. Rendering never spawns a process. The widget may merge current-session counters into the last canonical projection and clearly marks stale or degraded refreshes; TypeScript does not reimplement historical aggregation or diagnosis, and no daemon is introduced.

24. The dashboard extension owns local event writing, passive parent-session telemetry, lifecycle and doctor tools, dashboard commands and overlay, the active widget, retention invocation, shutdown drain, and association restoration. Existing subagent and handoff extensions communicate through versioned `pi.events` contracts and never import dashboard modules or own the ledger or projections. The dashboard attaches the active association to privacy-filtered producer observations. Handoff synchronously requests a validated association copy during the handoff and appends it through `newSession.setup`; absence or version mismatch degrades without blocking handoff.

25. Retention limits, widget behavior, diagnostic thresholds, baseline sample size, baseline percentile, and heuristic enablement live in the exact tracked `workflowTelemetry` config shape below, with strict validation, schema migration, configspec descriptions, generated reference state, hashing, and lock participation. No ignored local settings file becomes a second authority.

26. Pi target output planning, rendering, manifest ownership, cleanup, drift checking, generated checkout fixtures, and example projects govern exactly two new Pi target outputs, `.pi/extensions/awf-dashboard/index.ts` and `.pi/extensions/awf-dashboard/protocol.ts`, taking the Pi target from three to five extension files. The neutral generated `.awf/metrics/.gitignore` is the third new governed output overall. Current-state metadata adds `internal/telemetry/**` to tooling domain territory and adds `internal/project/target.go` and `internal/project/target_test.go` to the catalog-and-targets topic; the Pi runtime contract belongs to rendering topics rather than the Go telemetry topic. Tests cover target exclusion, protocol parity, runtime registration, publication-safe empty values, privacy exclusions, association and handoff propagation, trajectories and derived efforts, append integrity and races, deterministic retention, selector and JSON parity, diagnostic evidence, UI refresh boundaries, and degraded passive telemetry.

27. Implementation updates architecture, workflow, working-with-awf, configuration, testing, privacy, generated artifact documentation, and the authored AGENTS.md convention parts in the same checked batches as behavior. Every Applied batch carries exactly its matching current-state claim mutations and provenance. Every config or lifecycle transition runs `./x sync`, stages generated documentation including `docs/decisions/INDEX.md`, and passes the staged check and gate. All new templates preserve `missingkey=zero` publication safety and have deterministic empty-data tests. The doctor is documented as an expansion point for later repository adherence and quality diagnostics, but this decision does not authorize automatic reconciliation or collection of conversational content.

### Writer, protocol, and extension contracts

The descriptor defines the complete event envelope and payload union. Every event has a protocol version, bounded unique event ID, bounded idempotency key, effort and session IDs, trajectory and Pi anchor IDs when associated, timestamp, event kind, and a causal predecessor frontier. A lifecycle retry with the same idempotency key and identical payload is a success without a second logical mutation; reuse with a different payload is an integrity violation. Passive observations may be duplicated physically after a crash, but their observation ID makes aggregation deduplicate them deterministically.

The dashboard's per-stream queue validates against generated `protocol.ts`, appends one complete JSON line, flushes it, and acknowledges an explicit lifecycle operation only after durable completion. The Go reader validates against the same descriptor and remains normative when projections disagree. Protocol evolution may add event kinds or fields only under declared compatibility rules; an unsupported major version is rejected, while unknown optional fields in a compatible version are preserved but not interpreted.

The inter-extension event names are versioned. Subagents publish only a bounded envelope of role, requested and resolved model, thinking level, queue and run duration, usage totals, categorized outcome and stop reason, tool count, and tool-failure count. They never publish their task, output, stderr, display text, tool arguments, or argument previews to telemetry. The handoff association request carries a callback and accepts exactly one validated response. Because the request occurs during an actual handoff after extension loading, it is independent of factory load order; no response means no copied association. Shutdown ordering is irrelevant to producers because published observations are best effort, while the dashboard owns its queue drain.

Binary resolution starts at the project root derived from the extension path. When `.awf/bootstrap.sh` exists, the extension runs it with `bash`, requires a zero exit and exactly one non-empty stdout path, and uses that binary. Otherwise it resolves `awf` through `PATH`. Before a canonical read it invokes `awf metrics protocol --json` and requires a binary allowed by the existing project version gate plus an exactly supported telemetry protocol major. Resolution, handshake, or projection failure is cached as a visible degraded state and retried only at the controlled refresh boundaries.

### Lifecycle and causal-order contract

Effort lifecycle state is `discovery`, `active`, `completed`, or `abandoned`. Creation starts `discovery`; route selection moves it to `active`; route change remains `active`; completion and abandonment are terminal events. Reopen moves only an unpruned `completed` effort back to `active`, starts a new trajectory and terminal epoch, and preserves all earlier totals. An abandoned effort is never reopened. A missing pruned origin can be referenced opaquely by a new derived effort but cannot be reconstructed or reopened.

The legal lifecycle mutations and transitions are closed:

| Mutation | Allowed source | Effect |
|---|---|---|
| create | absent | atomically create metadata and first event; enter `discovery` |
| associate or detach session | `discovery`, `active` | change only the named session association |
| select route | `discovery` | set the first route; enter `active` |
| change route | `active` | replace the effective route and retain route history |
| start or finish phase | `discovery`, `active` | open or close one named phase interval |
| start, resume, or close trajectory | `discovery`, `active` | change trajectory projection without changing effort state |
| complete | `active` | require route and freshness rules; enter `completed` |
| abandon | `discovery`, `active` | enter `abandoned` with no route-completion requirement |
| reopen | `completed` | create a terminal epoch and trajectory; enter `active` |
| waive | any existing state, including terminal | reference one eligible finding and change only its presentation |
| repair | any existing state, including terminal | append one typed correction to named evidence |

A phase finish must name its causally visible unmatched start. A start must observe no other open top-level phase in its frontier. Duplicate identical retries are idempotent. Explicit lifecycle tools validate before append and fail without writing an invalid transition. Readers retain a structurally valid event from any other writer even when its transition is invalid, exclude its claimed state effect, and emit the exact transition finding. A finish without its start, a second competing start from the same frontier, an unlisted transition, and causally ordered phase overlap are exact violations. Waiver and repair are the only legal post-terminal mutations other than reopen; they do not reopen an effort. Repairs name the rejected or superseded event and append a typed correction; they never make the source event disappear.

Per-session order plus predecessor frontiers, explicit handoff links, trajectory parent and fork anchors, and transitive closure define a partial order. No wall-clock total order is invented across sessions. Exact ordering rules evaluate causally comparable events. Competing mutations from a shared frontier are reported as concurrent-state violations rather than ordered by timestamp. Timestamps remain authoritative only for durations when both endpoints are in one causally linked interval; negative or implausible clock results are integrity findings and are excluded from duration baselines.

The route requirements are:

| Route | Required ordered phases before completion |
|---|---|
| `direct` | brainstorming, implementation, implementation review, retrospective |
| `adr` | brainstorming, ADR authoring, ADR review, implementation, implementation review, retrospective |
| `plan` | brainstorming, planning, plan review, implementation, implementation review, retrospective |
| `adr-plan` | brainstorming, ADR authoring, ADR review, planning, plan review, ADR-plan resync, implementation, implementation review, retrospective |
| `bugfix` | brainstorming, implementation, implementation review, retrospective |
| `investigation-only` | investigation, retrospective |

Investigation is optional before any implementation route. Reentry is legal and counted, but it invalidates stale downstream evidence: ADR authoring after ADR review requires another ADR review; planning after plan review requires another plan review; either change after ADR-plan resync requires another resync; implementation after implementation review requires another implementation review. Completion requires the latest terminal epoch to satisfy its route and freshness rules. `investigation-only` may be selected only when no implementation phase exists. Route changes cause the final route's requirements to apply to the full causally ordered history.

Exact diagnostic rule version 1 is closed:

| Code | Severity | Predicate and evidence | Scope | Waiver or remediation |
|---|---|---|---|---|
| `WFV1-LIFECYCLE-TRANSITION` | violation | event source state or effect is not in the transition table; event and frontier IDs | effort | typed repair only |
| `WFV1-PHASE-ORDER` | violation | causally ordered phase sequence contradicts the effective route table; route and phase event IDs | terminal epoch | `approved-route-deviation` waiver or typed phase repair |
| `WFV1-ADR-REVIEW` | violation | ADR authoring required by the route has no later fresh ADR review; phase event IDs | terminal epoch | `benign-followup-no-rereview` waiver or add/correct review evidence |
| `WFV1-PLAN-REVIEW` | violation | planning required by the route has no later fresh plan review; phase event IDs | terminal epoch | `benign-followup-no-rereview` waiver or add/correct review evidence |
| `WFV1-ADR-PLAN-RESYNC` | violation | `adr-plan` lacks resync after the final ADR review and plan review; phase event IDs | terminal epoch | `approved-route-deviation` waiver or add/correct resync evidence |
| `WFV1-IMPLEMENTATION-REVIEW` | violation | implementation has no later fresh implementation review before completion; phase event IDs | terminal epoch | `benign-followup-no-rereview` waiver or add/correct review evidence |
| `WFV1-RETROSPECTIVE` | violation | the effective route lacks its required retrospective before completion; phase event IDs | terminal epoch | `approved-route-deviation` waiver or add/correct retrospective evidence |
| `WFV1-PHASE-OVERLAP` | violation | one start observes an open phase or causally ordered intervals overlap; interval event IDs | trajectory | `approved-phase-overlap` waiver or typed phase repair |
| `WFV1-CONCURRENT-STATE` | violation | competing lifecycle mutations share a frontier without causal order; competing event IDs | effort | typed repair selecting or superseding evidence |
| `WFV1-HANDOFF-ASSOCIATION` | warning | a handoff link has no validated association copy or changes effort without an explicit association event; handoff and association IDs | session link | `approved-missing-handoff-association` waiver or associate/detach repair |
| `WFV1-EVENT-INTEGRITY` | violation | malformed complete line, duplicate conflict, unsafe path, broken causal reference, or invalid payload; file, line, and available event IDs | stream or effort | no waiver; repair when an event remains appendable, otherwise explicit confirmed cleanup |
| `WFV1-SCHEMA-COMPATIBILITY` | violation | protocol major is unsupported or a required interpretation is unknown; version and stream evidence | stream or effort | no waiver; compatible binary/protocol migration only |
| `WFV1-CLOCK-INTEGRITY` | warning | linked interval has negative, non-finite, or protocol-bound-exceeding duration; endpoint IDs and timestamps | interval | `approved-clock-skew` waiver; exclude from duration baselines |

A waiver has one of the reason codes named in this table, the exact rule code, matching scope, and referenced evidence. It changes only that finding to informational and does not change lifecycle state. A rule not naming a waiver is never waivable.

### Integrity, retention, and resident-data contract

The threat model covers accidental truncation or alteration, incompatible writers, path traversal, unsafe file types, and symlink or reparse-point redirection by repository content. It does not defend against a hostile process running as the same user, and the ledger has no cryptographic tamper-evidence guarantee. Readers distinguish a final partial line from malformed complete lines, invalid schemas, broken causal references, duplicate conflicts, and unsupported versions. All identifiers are single path components after validation. On POSIX, directories and new files use owner-only permissions, existing paths must be owned by the current user, and ledger, metadata, lease, cache, and pruning targets must be confined regular files or directories with no symlink traversal. Platforms without POSIX ownership enforce their closest regular-file and no-reparse equivalent. Explicit mutations fail closed on an unsafe path; passive telemetry degrades and reports it.

Creation first takes an effort-ID lease outside the not-yet-created effort and rejects an existing effort or tombstone. It writes `effort.json` and the first session stream containing `effort_created` into an owner-only staging directory, flushes both files and their directory, and atomically renames the staging directory into `efforts/<effort-id>` before syncing the efforts directory. The rename is the creation commit point. A retry with identical creation idempotency key, immutable metadata, and first event succeeds; any same-ID difference is a collision. Recovery removes an expired uncommitted staging directory, preserves a committed effort, and never synthesizes missing metadata or a create event.

Every later append and prune operation takes a short cross-process effort lease using atomic creation, a random nonce, owner identity, a 30-second expiry, and a heartbeat every 10 seconds. A contender may recover an expired lease only after a further 30-second grace interval and a compare-before-remove check on its nonce. Under the lease, an append revalidates the effort path, appends and flushes, then releases. A pruner revalidates terminal state, writes and flushes a nonce-bearing pending tombstone, atomically renames the effort into a private trash directory, syncs both parent directories, atomically promotes and syncs the tombstone as committed, and then releases before recursive deletion. The effort rename is the append/prune linearization point: a later writer observes the missing effort or pending or committed tombstone and refuses rather than recreating it. Recovery removes a pending tombstone when the effort was never renamed, commits it when the matching trash entry exists, completes deletion for a committed tombstone, and reports any nonce or path mismatch as ambiguous. No recursive deletion begins before the committed tombstone and its directory entry are durable. Retry is idempotent by prune nonce.

Only completed and abandoned efforts are retention candidates; active and discovery efforts are preserved. The `maxCompletedEffort*` config names use completed in this retention sense to include either terminal outcome. Age uses the latest terminal event timestamp. Count applies repository-wide to terminal efforts. Zero disables that retention dimension. An effort is selected if it exceeds enabled maximum age or falls beyond enabled maximum count after sorting newest first by terminal timestamp, creation timestamp, then effort ID. Pruning executes oldest candidates first with the inverse stable keys. Reopening must win the effort lease and append before candidate selection can remain valid; pruning rechecks terminal state under the same lease.

The generated `.awf/metrics/.gitignore` self-ignores the resident tree. Project open, sync, check, and nested-adopter discovery skip runtime descendants without claiming them. Uninstall preserves the ignore file whenever resident metrics remain and removes it only with an empty directory or an explicitly confirmed metrics purge. Ordinary cleanup never deletes ledger, cache, trash, tombstone, or lease state.

### Configuration and heuristic contract

Schema migration and fresh scaffolds materialize this complete default block:

```yaml
workflowTelemetry:
  retention:
    maxCompletedEffortAgeDays: 90
    maxCompletedEffortCount: 100
  widget:
    enabled: true
    showCost: true
  diagnostics:
    heuristicsEnabled: true
    minimumBaselineSamples: 10
    baselinePercentile: 95
    thresholds:
      phaseReentryCount: 2
      phaseDurationSeconds: 14400
      phaseTokens: 200000
      compactionCount: 3
      handoffCount: 3
      toolFailureCount: 3
      gateFailureCount: 2
      cacheReadPercentBelow: 10
      subagentQueueWaitSeconds: 60
      implementationReworkCount: 2
```

Retention zero values disable their individual dimension. Other counts and durations are positive integers; `baselinePercentile` is an integer from 1 through 100, and percentage thresholds are integers from 0 through 100. Disabling the widget hides only the widget. Disabling heuristics leaves exact diagnostics, collection, tools, and the overlay active. `showCost` suppresses cost display but not collection or export. Migration from every supported earlier schema adds this block without changing any unrelated configured value.

Initial heuristic rule version 1 evaluates: reentries per named phase; duration and tokens per phase interval; compactions and handoffs per effort; categorized tool failures and failed gates per implementation segment; cache-read percentage as `cacheReadTokens / (inputTokens + cacheReadTokens) * 100` when the denominator is positive; queue wait per subagent invocation; and implementation rework as each return to implementation after an implementation review. The narrow shell classifier counts a gate only for an exact supported awf gate command shape; ambiguity is unclassified.

An absolute signal fires when a value reaches an upper threshold or falls to the cache lower threshold. A historical signal is available only from at least `minimumBaselineSamples` completed comparable efforts with the same route, rule version, and available metric. Its baseline is the configured nearest-rank percentile; cache reuse uses the complementary lower percentile. Historical and absolute evidence are reported independently. Confidence is `medium` for one applicable trigger and `high` when both trigger; insufficient sample produces no historical trigger and is stated in evidence. Exact rules have confidence `certain`. Thresholds, cohort keys, sample count, percentile, observed value, and contributing event or counter IDs are present in every finding. No values from a newer or incompatible protocol enter a cohort.

## State changes

- add `tooling/workflow-telemetry:event-protocol-and-ledger`
- add `tooling/workflow-telemetry:effort-lifecycle-and-routes`
- add `tooling/workflow-telemetry:trajectory-and-derived-effort-model`
- add `tooling/workflow-telemetry:privacy-integrity-and-retention`
- add `tooling/workflow-telemetry:canonical-projections-and-diagnostics`
- add `tooling/cli:metrics-and-doctor-command-contract`
- add `config/configuration:workflow-telemetry-settings`
- add `config/migrations-and-locks:workflow-telemetry-config-migration`
- update `rendering/catalog-and-targets:pi-extension-target-render`
- update `rendering/catalog-and-targets:pi-minimum-runtime`
- update `rendering/catalog-and-targets:pi-real-runtime-smoke`
- add `rendering/project-output-plan:workflow-telemetry-governed-outputs-and-resident-data`
- add `rendering/adapter-outputs:pi-workflow-dashboard-runtime`
- add `rendering/templates:pi-workflow-dashboard-public-contract`

## Consequences

Workflow cost, phase usage, lineage, and deviations become inspectable across the sessions and Pi trajectories that make up an effort. The same canonical engine serves interactive, agent, CLI, and exported views, allowing later analysis and new doctor rules without introducing competing interpretations. Privacy exclusions and typed categories make the store substantially safer than a general trace log.

The feature adds a new persisted protocol, config schema, Go subsystem, CLI family, generated executable extension, and cross-language projection. Compatibility, migration, output governance, ignored-data ownership, corruption handling, and retention all become permanent maintenance obligations. Append-only history also uses more space than a current-state snapshot, mitigated by deterministic retention and disposable caches.

Explicit lifecycle operations can require repair when an agent omits or misorders them. Refusing to infer identity or phase boundaries trades convenience for trustworthy data. Passive observation remains available during partial failure, but projections can be stale and must say so rather than presenting false precision.

Trajectory-aware and derived-effort accounting avoids silently losing discarded work or double-counting analysis, at the cost of a more involved event and aggregation model. Comparable-effort heuristics will intentionally remain unavailable until enough history exists, and even then report evidence and confidence rather than a score or automatic judgment.

The dashboard adds process starts only at controlled refresh boundaries, not during rendering, and no daemon or alternate TypeScript aggregation engine is created. Explicit repairs and waivers preserve auditability, while destructive cleanup remains deliberate.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Build a repository-only dashboard prototype | It would not establish the adopter-facing protocol, output governance, privacy, or compatibility contract required of an awf standard artifact. |
| Parse working-memory files or infer phases from session activity | Working memory is prose and temporary, while inference would make effort identity and phase boundaries non-deterministic. |
| Persist rewrite-only effort snapshots | Rewrites lose lineage and repair history and are unsafe with multiple parent-linked session writers. |
| Use SQLite as the authoritative store | It adds a binary mutable store and coordination burden where append-only per-session streams provide inspectable, mergeable authority. |
| Store full traces or bounded prompt and output snippets | Conversational content and tool payloads are unnecessary for the selected metrics and create unacceptable privacy and retention exposure. |
| Aggregate and diagnose directly in the TypeScript extension | It would duplicate Go semantics, allow CLI and dashboard drift, and make offline querying depend on Pi. |
| Run a background daemon | The required refresh cadence does not justify another lifecycle, process, and synchronization boundary. |
| Guess effort association from parent sessions | Parentage does not prove active-branch intent, especially after tree navigation, clone, fork, or terminal continuation. |
| Treat every continuation as the same effort | A new issue derived from completed analysis would distort lifecycle compliance and double-count parent work. |
| Emit one health score | A composite hides rules, evidence, uncertainty, and remediation, making the diagnosis less actionable and harder to govern. |
| Make diagnostics or repairs automatic | Heuristics can be wrong, and reconciliation changes durable workflow history; both require visible evidence and explicit policy or approval. |
| Keep unlimited history as the only retention policy | Standard artifacts must bound adopter disk usage and provide deterministic cleanup behavior. |

## Status history

- 2026-07-22: Proposed
- 2026-07-22: Implementing; content-sha256: 8fa0a72cf5fc1d2f8c3a999750601cc1bed361ef31160f990653281a2d1dca97
- 2026-07-22: Applied; state-sequence: 15; operations: add `config/configuration:workflow-telemetry-settings`, add `config/migrations-and-locks:workflow-telemetry-config-migration`
