The Pi runtime floor and the awf/pi-tools ownership boundary.

## Claims

### `invariant: pi-extension-target-render`

The fixed Pi target renders the awf subagent profile adapter and bounded model-routing module with provenance; `effort-workflow` additionally renders the Pi-target-owned `using-effort` skill and `awf-effort` index/client pair through the same output predicate. The effort client alone strictly invokes and decodes activity protocol v2 and owner-scoped memory protocol v1 through bounded transport; its index owns direct serialized association, fixed-path transient context, dynamic memory-tool activation, Pi file-queue participation, heartbeat/shutdown lifecycle, and Remote Pi translation. The profile adapter owns protocol-v2 registration, role policy, model routing, and Git policy, while independently installed pi-tools owns general context usage, handoff, scheduling, child execution, confinement, execution facts, and presentation. No awf context-usage, handoff, runner, telemetry, workflow-router, scheduler, process-supervisor, or progress-renderer output renders, and every retained file follows normal output-plan, drift, cleanup, target-sensitive hash, generated-checkout, adopter-example, editor-quiet, and container-coverage semantics.
Origin: ADR-0148
Revised-by: ADR-0162, ADR-0164, ADR-0167, ADR-0173, ADR-0209, ADR-0218, ADR-0225, ADR-0239, ADR-0251, ADR-0279
Backing: test

### `invariant: pi-implementation-state-boundary`

The implementation profile serializes against itself and enforces caller-selected commit permission against an optional invocation-owned verification checkout, defaulting to the project root. An explicit identity resolves relative to the project root after one leading `@` is removed, canonicalizes filesystem aliases, and must be an exact live checkout root whose Git common directory matches the project root. For a linked checkout, the absolute Git directory's `gitdir` backlink must canonically identify the selected checkout's non-symlink regular `.git` file; copied pointers, selected-entry symlinks, and other invalid identities refuse before dispatch without worktree enumeration or `git worktree list` parsing. `beforeRun` and `afterRun` snapshot the resolved checkout and expose both snapshots in structured profile data while parent and child CWD and rendered role loading remain rooted. A changed selected HEAD under no-commit permission and an unchanged selected HEAD under commit permission are terminal policy failures with accurate checkout repair; unverifiable snapshots report commit verification unavailable.
Origin: ADR-0148
Revised-by: ADR-0260, ADR-0279
Backing: test

### `invariant: pi-session-handoff-workflow`

Pi runtime owns the single executable session-replacement protocol, projected only by the Pi `using-effort` skill. After a persisted formal phase or approval checkpoint, or another safe resumable effort point, it judges retained-context relevance and successor work using current `[session context]` model-window and active-branch-compaction evidence with no fixed threshold. It either continues autonomously or invokes `handoff_session` alone with exactly `Continue with effort <slug>.`; the kickoff identifies only the effort and carries no phase or task limit, association mechanic, resume procedure, or handoff-log instruction. The associated effort is reoriented from repository authority and owned memory before substantive work. Managed-worktree use is pre-integration only; integration, deferred lifecycle closure, worktree removal, and retrospective use the governed primary checkout. Handoff validates only dual-format effort identity; it does not validate mutable metadata, parse state or activity, select an effort, or mutate memory. A replacement session logs its actual boundary before substantive work; continuation, cancellation, or failure that leaves the old session active logs none. Awf does not own the independently installed pi-tools handoff implementation.
Origin: ADR-move-pi-session-handoff-authority-to-pi-runtime
Backing: test

### `invariant: pi-minimum-runtime`

The retained awf effort entrypoint requires the adopter-supplied compatible Pi runtime APIs it directly uses, including active-tool access and the package-exported real-path file-mutation queue; its actionable incompatibility guard and numeric 0.84.2 floor remain scoped to that output, with pi-ai and pi-tui 0.84.2 and the checksummed fork-v0.84.2.2 artifact as its proof graph. The direct `using_effort` companion needs no `changeCwd` capability and optional Remote Pi events remain advisory. The profile adapter has no package-version read or Pi minimum-runtime guard; it instead requires independently installed pi-tools protocol v2, treats final profile registration as compatibility, and reports one actionable no-fallback failure when capability is missing, incompatible, late, or rejected.
Origin: ADR-0148
Revised-by: ADR-0162, ADR-0167, ADR-0209, ADR-0218, ADR-0219, ADR-0225, ADR-0239, ADR-0279, ADR-0283
Backing: test

### `invariant: pi-real-runtime-smoke`

The deterministic pinned-runtime smoke covers generated TypeScript loading, native Pi skill discovery, protocol-v2 profile negotiation through a contract double, live model-routing selection and rendered role preparation, and retained effort tool registration. The strict lane directly composes source-only pi-tools/testing v0.3.0 recordings for generic Pi seams and covers adapter policy and effort behavior at 100 percent; the real SDK smoke remains the proof of generated extension loading, six-profile negotiation, model-routing delivery, native skill discovery, and effort registration on the pinned fork. Awf neither imports nor behavior-tests an adopter pi-tools runtime.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0161, ADR-0162, ADR-0164, ADR-0167, ADR-0173, ADR-0209, ADR-0279, ADR-0283
Backing: test

### `invariant: pi-tools-integration-boundary`

Awf subscribes to protocol-v2 capability and registration-result events during extension factory initialization and emits a correlated request with one stable registration id. A compatible capability atomically receives all six awf profiles with default suppression. Missing, incompatible, late, and rejected registration produce one actionable no-fallback notice. Source-only pi-tools/testing v0.3.0 owns generic test recordings, while an independently installed adopter pi-tools runtime remains protocol-v2 compatible rather than revision-pinned and owns general context usage, handoff, child execution, scheduling, confinement, execution facts, and presentation.
Origin: ADR-0279
Revised-by: ADR-0283
Backing: test
