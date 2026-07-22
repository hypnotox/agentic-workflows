---
format: current-state-v2
status: Proposed
date: 2026-07-22
---
# ADR-0146: Pi Workflow Telemetry and Dashboard


## Context

awf guides an effort through named workflow phases, but it has no durable, structured account of which route the effort took, how work and model usage were distributed, where handoffs or forks occurred, or which deviations may indicate a workflow problem. Pi exposes useful per-session model, token, cache, cost, tool, and failure data, and the subagent extension already reports a subset of it, but those observations end with the session and do not share a stable effort identity. Working-memory files cannot be the authority: they are prose, are intentionally deleted after retrospective, and record a checkpoint rather than an event history.

A useful dashboard must span the parent-linked sessions that now form one workflow effort without collecting conversational content. It must distinguish current-path work from work on discarded Pi tree branches, distinguish a resumed effort from a new effort derived from prior analysis, and diagnose both exact workflow-order violations and weaker statistical signals. The same interpretation must serve the TUI, automation, and offline analysis; duplicating aggregation or diagnosis in TypeScript would allow those surfaces to disagree.

The generated Pi extensions are executable standard artifacts installed into adopter repositories. The design must therefore cover output governance, runtime compatibility, ignored resident data, concurrent session writers, corrupt or newer event data, retention, and privacy rather than treating the feature as a repository-local visualization. Passive observation must not interrupt work, while explicit lifecycle changes must not pretend to succeed when they were not durably recorded.

## Decision

1. awf will ship workflow telemetry and its Pi dashboard as standard functionality for Pi-enabled adopter projects. The implementation introduces a Go telemetry package as the canonical storage, validation, projection, selection, aggregation, retention, and diagnostic engine, and a generated `.pi/extensions/awf-dashboard/index.ts` extension as the Pi runtime surface.

2. The authoritative history is an append-only, versioned JSONL event ledger under `.awf/metrics/efforts/<effort-id>/sessions/`, with one stream per Pi session to avoid cross-session writer contention. Optional aggregate caches are disposable and never authoritative. `.awf/metrics/**` is ignored resident runtime data and receives an explicit closed-tree ownership or exemption model rather than being mistaken for undeclared config-tree input.

3. A single machine-readable protocol descriptor owned by the Go telemetry package is normative for protocol versions, event kinds, routes, phases, activities, bounded categories, identifier limits, and payload shapes. Go validation consumes that descriptor, and Pi target rendering projects it into TypeScript constants, types, and validators. The descriptor participates in render input attribution and hashing, with cross-language golden tests preventing protocol drift.

4. Long-lived events are privacy-minimal. They may contain timestamps, opaque bounded effort, session, Pi entry, and trajectory identifiers, model and tool names, durations, token, cache, and cost totals, categorized outcomes and errors, and bounded workflow counters such as gates and commits. They never contain prompts, assistant text, tool arguments, command output, conversational waiver prose, or repository paths other than the bounded effort checkpoint identifier. Arbitrary identifiers and category-like values are length-bounded and validated.

5. Effort creation writes immutable `effort.json` metadata containing identity, creation time, checkpoint identifier, and optional immutable origin metadata. All mutable effort, route, phase, association, repair, waiver, and terminal state is represented by events; no mutable snapshot rewrites history.

6. An effort begins in an undecided discovery state. A closed route is selected only when evidence supports one of `direct`, `adr`, `plan`, `adr-plan`, `bugfix`, or `investigation-only`; `investigation-only` is valid only when investigation concludes without requested implementation. Later scope changes append `route_changed`. Abandonment is terminal from any route.

7. The top-level phase vocabulary is investigation, brainstorming, ADR authoring, ADR review, planning, plan review, ADR-plan resync, implementation, implementation review, and retrospective. Debugging is an investigation activity, TDD is an implementation activity, exploration belongs to the phase that requested it, bugfix is a route spanning implementation, review, and retrospective, and plan execution styles are implementation modes rather than phases.

8. Stable effort identity and exact phase transitions are changed only through explicit lifecycle operations. The system does not infer them from working-memory prose, prompts, filenames, or ambient session activity. Explicit lifecycle writes fail visibly if durable append fails; passive telemetry failures are non-blocking and surface degraded status and diagnostics.

9. Pi effort and session association is persisted only in a TUI custom session entry. The handoff extension copies the association in `newSession.setup`, and restoration reads only entries on the active branch. The runtime never guesses an association from session ancestry, working-memory prose, or repository state. Dashboard controls may explicitly associate, detach, or repair state.

10. The ledger models Pi tree navigation with trajectories. Events carry an opaque Pi anchor entry ID, trajectory ID, parent trajectory ID, and fork anchor where applicable. `/tree`, `/fork`, and `/clone` close the current trajectory segment and either resume the selected existing active-branch trajectory or append `trajectory_forked`; selecting an anchor before association detaches rather than guessing.

11. Current-path aggregates follow the active trajectory ancestry, while effort totals include every trajectory and retain discarded-branch work. Neither branch navigation nor repair rewrites historical events. Dashboard projections make the distinction visible so discarded work is neither lost nor silently charged twice to the active path.

12. Continuing work from a terminal effort is explicit. The default for tackling a new issue from completed analysis is a new `derived` effort with immutable opaque origin effort, trajectory, and anchor metadata and independent lifecycle and totals. `independent` starts unrelated work, while `reopen` explicitly creates a new trajectory in the same terminal effort. History may group derived families but never double-count parent analysis cost.

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

23. The TypeScript extension obtains canonical projections by invoking the verified `awf metrics --json` and `awf doctor --json` surfaces at startup, overlay open or refresh, and relevant state changes, then caches results in memory. Rendering never spawns a process. The widget may merge current-session counters into the last canonical projection and clearly marks stale or degraded refreshes; TypeScript does not reimplement historical aggregation or diagnosis, and no daemon is introduced.

24. The dashboard extension owns local event writing, passive parent-session telemetry, lifecycle and doctor tools, dashboard commands and overlay, the active widget, retention invocation, shutdown drain, and association restoration. Existing subagent and handoff extensions publish and propagate structured versioned telemetry contracts without taking ownership of the ledger or projections.

25. Retention limits, widget behavior, diagnostic thresholds, baseline sample size, and heuristic enablement live in tracked `.awf/config.yaml`, with strict validation, schema migration, configspec descriptions, generated reference state, hashing, and lock participation. No ignored local settings file becomes a second authority.

26. Pi target output planning, rendering, manifest ownership, cleanup, drift checking, generated checkout fixtures, and example projects govern the dashboard extension and projected protocol artifacts alongside the existing Pi extensions. Tests cover target exclusion, protocol parity, runtime registration, privacy exclusions, association and handoff propagation, trajectories and derived efforts, append integrity and races, deterministic retention, selector and JSON parity, diagnostic evidence, UI refresh boundaries, and degraded passive telemetry.

27. Implementation updates architecture, workflow, working-with-awf, configuration, testing, privacy, and generated artifact documentation in the same checked batches as behavior. The doctor is documented as an expansion point for later repository adherence and quality diagnostics, but this decision does not authorize automatic reconciliation or collection of conversational content.

## State changes

- add `tooling/workflow-telemetry:event-protocol-and-ledger`
- add `tooling/workflow-telemetry:effort-lifecycle-and-routes`
- add `tooling/workflow-telemetry:trajectory-and-derived-effort-model`
- add `tooling/workflow-telemetry:privacy-integrity-and-retention`
- add `tooling/workflow-telemetry:canonical-projections-and-diagnostics`
- add `tooling/workflow-telemetry:pi-dashboard-runtime`
- add `tooling/cli:metrics-and-doctor-command-contract`
- add `config/configuration:workflow-telemetry-settings`
- update `rendering/catalog-and-targets:pi-extension-target-render`
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
