---
format: current-state-v2
status: Implemented
date: 2026-07-28
---
# ADR-0167: Remove workflow telemetry and mandatory Pi routing

## Context

Workflow telemetry began as low-value, best-effort observation collection. It grew into a
protocol, integrity subsystem, report surface, resident-data contract, Pi extension, and effort
assignment mechanism. Even after ADR-0164 removed lifecycle projection, the direct writer still
implements locking, retry, stable identities, fsync, quarantine, corruption diagnosis, legacy
reading, aggregation, and export. That machinery is disproportionate to the value of its output
and creates more runtime and maintenance failures than useful information.

The Pi telemetry extension also became the host for unrelated effort-selection and workflow-router
behavior. Handoff requests that selection, validates it against a checkpoint header, and assigns it
to a child session. The resulting coupling makes ordinary workflow loading and session replacement
depend on state that neither operation needs.

The router itself is unnecessary. Pi natively discovers project skills, but awf hides its Pi
workflow bodies and exposes a single closed loader. Workflow templates then prescribe mandatory
successors even though agents sometimes need to enter a support or chain workflow from a different
point. A catalog can explain intended use and common relationships without turning those
relationships into legal transition edges.

Effort records, optional memory, and managed worktrees remain independently useful. Session
assignment exists only for the removed Pi selection and metrics join, so it has no remaining
consumer. Existing assignment and telemetry residents are local, obsolete data with no durable
product authority.

## Decision

1. Remove workflow telemetry and dashboard behavior as a product surface. Delete all `awf metrics`
   commands, telemetry readers and reports, protocols, direct writers, retry and integrity logic,
   Pi telemetry outputs, tests, and active documentation. No awf runtime collects, reads, validates,
   repairs, exports, or reports telemetry after this change.

2. Advance the config schema to generation 21 and register its release floor in
   `minVersionBySchema`, so the ordinary schema and binary-version gates reject older binaries.
   The generation-21 upgrade migration recursively removes the obsolete `.awf/metrics/` and
   `.awf/assignments/` roots from the confined primary control root, including their generated
   ignore sentinels and all local descendants. The migration treats those bytes as disposable
   local observations and associations, never follows a symlinked resident root, fails visibly on
   an unsafe root or removal error, and is retryable after partial removal. Upgrade output reports
   each root as removed or already absent before the normal render. Config loading, migration
   dispatch, output planning, the lock manifest, rendering, drift, discovery, sweep, and uninstall
   cease to own or preserve either root after the migration.

3. Remove Pi-session assignment from effort management. Delete `awf effort assign`, `unassign`, and
   `assignments`, the assignment store, assigned-session fields in logical effort output, and every
   assignment join. Retain effort creation, list, show, rename, memory, state transitions, repair,
   and managed-worktree operations without a Pi-session concept.

4. Remove `/awf-effort`, selected-effort session entries and events, selection restoration, and
   selected-effort context from Pi. Pi never creates, selects, assigns, or propagates an effort.
   Agents may still invoke the binary effort commands directly when durable coordination, memory,
   or a managed worktree is useful.

5. Keep fresh-session handoff independent of efforts. Handoff retains its single-tool preflight,
   single-use queue, countdown and cancellation, parent link, optional confined regular-file memory
   path, kickoff, child cleanup, and editor fallback. It does not parse an `Effort:` header, compare
   identities, invoke the awf binary, assign the child session, or append selection state.

6. Remove the Pi `awf_workflow` tool, its discoverable router skill, and hidden
   `.pi/awf-workflows/` bodies. Render every enabled governed workflow through Pi's normal skill
   layout under `.pi/skills/<prefix>-<name>/SKILL.md`, using the same progressive-disclosure model
   as other skill targets. No Pi extension participates in workflow selection.

7. Make the catalog the source of a complete enabled-skill catalog. Every governed skill carries
   a kind (`chain`, `task`, or `support`), a concise purpose and invocation trigger, and optional
   advisory common-predecessor or follow-up information when that relationship helps selection.
   Migrate every standard skill's workflow-to-workflow `RequiresSkills` edge to that advisory
   metadata and leave its `RequiresSkills` empty; requirements needed by non-skill artifacts remain
   structural enablement dependencies rather than workflow edges. The generated agent guide
   presents every enabled skill from the advisory metadata. `chain` describes a skill that commonly
   participates in an end-to-end development sequence; it is not a route, phase, legal-entry rule,
   required enablement edge, or required transition. Native Pi skill and guide templates retain
   coherent generic output under missing-key-zero rendering with unset variables and never emit an
   unresolved-value token.

8. Rewrite workflow skill descriptions, entry guidance, and terminal steps so relationships are
   recommendations rather than mandatory edges. A skill may recommend a useful follow-up and may
   still enforce the procedure within its own scope, but no generated instruction claims that one
   enabled skill is the only legal predecessor or successor. An agent may invoke any enabled skill
   whenever its stated purpose fits the current work.

9. Remove active dashboard, metrics, assignment, selection, router, hidden-body, and retired
   telemetry/workflow-selection lifecycle terminology from generated docs, architecture, release
   guidance, current-state topics, and the changelog. Legitimate effort state transitions and ADR
   lifecycle terminology remain. In every ADR status-transition and implementation commit, run
   `./x render` and commit the regenerated `docs/decisions/INDEX.md`, awf-owned outputs, and Sundial
   adopter outputs with their sources. Historical ADRs and implemented plans remain unchanged as
   append-only records of the retired design.

10. Back each new invariant with a matching proof annotation on deterministic tests. The resident
    output proof covers exactly the retained effort, memory, and worktree roots plus destructive
    generation-21 cleanup of assignments and metrics. The advisory-transition proof scans rendered
    skill and guide output for catalog-derived advisory relationships and absence of mandatory
    predecessor/successor language. The native-Pi-skill proof covers ordinary discoverable paths,
    enabled/disabled cleanup, absence of router and hidden-body outputs, and parity with the other
    skill targets.

## State changes

- update `config/migrations-and-locks:workflow-telemetry-config-migration`
- update `rendering/catalog-and-targets:enabled-set-closed`
- remove `rendering/catalog-and-targets:exploration-skill-closure`
- update `rendering/catalog-and-targets:requires-skills-exact`
- update `tooling/effort-management:effort-record-authority`
- remove `tooling/effort-management:session-effort-assignment`
- remove `tooling/workflow-telemetry:event-protocol-and-ledger`
- remove `tooling/workflow-telemetry:privacy-integrity-and-retention`
- remove `tooling/workflow-telemetry:canonical-projections-and-diagnostics`
- update `tooling/cli:effort-command-contract`
- remove `tooling/cli:metrics-command-contract`
- update `rendering/singletons-and-payloads:memory-gitignore-always-on`
- remove `rendering/singletons-and-payloads:workflow-telemetry-governed-outputs-and-resident-data`
- add `rendering/singletons-and-payloads:resident-output-preservation`
- update `rendering/project-output-plan:output-plan-complete`
- update `rendering/guide-and-doc-templates:guide-entry-point-routing`
- update `rendering/guide-and-doc-templates:working-memory-single-home`
- update `rendering/workflow-skill-templates:mandatory-approval-boundaries`
- update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- add `rendering/workflow-skill-templates:workflow-transitions-advisory`
- remove `rendering/adapter-outputs:pi-workflow-telemetry-runtime`
- update `rendering/pi-workflows:pi-session-handoff-lifecycle`
- update `rendering/pi-workflows:pi-session-handoff-public-contract`
- update `rendering/pi-workflows:pi-session-handoff-workflow`
- remove `rendering/pi-workflows:pi-lifecycle-enforcing-workflow-router`
- add `rendering/pi-workflows:pi-native-workflow-skills`
- remove `rendering/pi-workflows:pi-workflow-telemetry-public-contract`
- update `rendering/pi-runtime:pi-extension-target-render`
- update `rendering/pi-runtime:pi-minimum-runtime`
- update `rendering/pi-runtime:pi-real-runtime-smoke`

## Consequences

Pi loses metrics, dashboard, effort-selection, and workflow-loader commands. Existing users must use
ordinary awf effort commands for optional durable coordination and Pi's native skill discovery for
workflow selection. Upgrading intentionally deletes local telemetry and session-assignment bytes;
there is no migration or export path because the removed data has no durable authority or justified
retention value.

The implementation deletes substantially more code than it adds. Effort and handoff behavior become
independent, Pi extension startup has fewer failure modes, and workflow selection no longer depends
on extension version or administrative state. The complete skill catalog adds a small amount of
catalog metadata, but one source replaces router-specific mappings and scattered entry guidance.

Advisory transitions reduce mechanical workflow conformance. Quality controls within a selected
skill, project invariants, approval boundaries, staged checks, and the gate remain mandatory. The
catalog and skill descriptions make the recommended development sequence visible without
rejecting truthful non-linear work.

The removal crosses persisted local state, CLI grammar, migration, generated outputs, Pi runtime,
workflow templates, current-state claims, and documentation. A staged implementation plan and
complete gate coverage are required.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep best-effort usage writes without reports | Even a minimal writer retains a Pi runtime hook, persisted protocol, privacy contract, failure handling, and tests for data with no demonstrated consumer. |
| Keep metrics reports but stop new collection | The legacy readers, selectors, aggregation, diagnostics, and CLI surface remain a large permanent maintenance burden. |
| Preserve obsolete residents behind tracked ignore sentinels | It leaves permanent product artifacts for data explicitly judged disposable and makes ownership ambiguous. |
| Keep Pi effort selection without telemetry | Session assignment and propagation have no necessary workflow or handoff role; direct effort commands are sufficient when coordination is useful. |
| Move `awf_workflow` into a smaller extension | Pi already provides native discoverable skills, so another extension and hidden-body protocol solve no remaining problem. |
| Keep mandatory successor instructions without runtime validation | Prose-only edges still reject legitimate entry points and recreate the same rigidity as agent errors rather than tool errors. |
| Remove chain relationships from the catalog entirely | Common workflow order remains useful selection guidance when clearly marked advisory. |

## Status history

- 2026-07-28: Proposed
- 2026-07-28: Accepted; content-sha256: 14d573f1968759d08f99a1deb6fdfd94fefeb10b8130b6c3dc1b26a64bb96a4d
- 2026-07-28: Implemented; content-sha256: 14d573f1968759d08f99a1deb6fdfd94fefeb10b8130b6c3dc1b26a64bb96a4d
