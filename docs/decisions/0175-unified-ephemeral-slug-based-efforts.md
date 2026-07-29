---
format: current-state-v2
status: Proposed
date: 2026-07-29
---
# ADR-0175: Unified Ephemeral Slug-Based Efforts

## Context

ADR-0164 separated lightweight local efforts from workflow telemetry, but retained more state than
local coordination needs. Each effort is currently a mutable UUID-named JSON record beside an
optional standalone memory file and optional UUID-named worktree. The record carries an active,
completed, or abandoned lifecycle, memory presence, worktree metadata, and integration disposition.
Commands can rename, reopen, repair, complete, abandon, and manually mark integration. This creates
a second state machine beside Git and makes the human identity agents must carry differ from every
resident path and branch they operate.

The optional-memory rule also divides one workflow concept across two roots. An agent may create an
effort without the checkpoint it needs for continuation, or create standalone memory without the
effort that owns it. Current guidance recommends efforts only when durable coordination is useful
and describes checkpoint memory as optional. It therefore cannot make creation, checkpoint updates,
handoffs, terminal review, worktree integration, retrospective, and cleanup one reliable chain.

Managed worktree integration has the same over-modeling problem. Integration is a Git operation on
actual repository topology, not durable effort state. Recording pending, integrated, or manually
integrated disposition can disagree with refs and worktree registration. Conversely, completing an
effort does not remove its worktree, so a terminal local record can coexist with resources that still
need integration or cleanup.

The desired unit is smaller and stricter: no effort for a minimal simple fix, but one mandatory
memory-owning effort for a concrete multi-step outcome. Its meaningful immutable slug should be the
identity agents use, its directory should be the only active-effort fact, and Git should remain the
authority for optional managed-worktree state. This change must preserve repository-wide confinement,
restart safety, publication-safe rendering, append-only decision history, and explicit diagnostics
for every refusal or partial mutation.

## Decision

1. Replace separate effort records and memory files with one repository-wide resident directory per
   effort at `.awf/efforts/<slug>/`. It contains `state.json` and `memory.md`. A successfully
   published directory is the only fact that an effort exists and is active; awf keeps no effort
   ledger, lifecycle state, integration disposition, lock, or retained terminal record.

2. `awf effort new <outcome-title>` derives one meaningful lowercase kebab-case slug from the title.
   The slug is immutable and is the public command identity, resident path segment, worktree path
   segment, and branch suffix. Creation rejects a collision with an existing or incomplete reserved
   slug actionably rather than appending a number or choosing a different identity. Static
   `state.json` retains an awf-allocated UUID as an internal identity together with the slug, title,
   creation time, and schema version; no command renames either identity.

3. Creation reserves the slug directory exclusively, writes the initial owned `memory.md`, and
   publishes `state.json` last. A command enumerating or selecting efforts ignores a directory whose
   static state was never published, but never deletes, overwrites, or guesses malformed, incomplete,
   symlinked, unconfinable, or foreign bytes. It reports an actionable manual-cleanup diagnostic when
   a requested slug is blocked by such bytes. Successful creation always includes memory; remove the
   standalone memory command and every no-memory creation mode.

4. The owned memory file is the only working-memory location. It carries the effort slug and the
   normal brief, decisions, phase, next action, update time, and handoff log needed for restart and
   fresh-session continuation. Checkpoint, resume, and handoff guidance always names
   `.awf/efforts/<slug>/memory.md`; no command or agent creates `.awf/memory/` content independently.
   Tracked ADRs, plans, code, and current-state documentation remain authoritative over this
   ephemeral checkpoint.

5. Finishing is deletion, not a lifecycle transition. `awf effort finish <slug>` validates the
   complete resident and managed-resource preconditions, then removes the whole effort directory.
   There is no complete, abandon, reopen, repair, or historical-listing state machine. An operator
   who intentionally discards work uses the same guarded resource cleanup and finish operation;
   durable project history belongs in Git rather than a local terminal record.

6. Keep managed worktrees optional and separate at `.awf/worktrees/<slug>/` on branch
   `awf/<slug>`. Worktree creation derives those values from the selected effort and an explicit base
   or caller HEAD, while native Git remains authoritative for registration, branch identity,
   cleanliness, ancestry, merge state, and repository identity. Static effort state stores no
   worktree attachment or integration metadata.

7. Managed integration is a stateless Git utility invoked from the clean target checkout that will
   receive the effort. It verifies the selected effort, expected managed path, worktree registration,
   `awf/<slug>` branch, repository identity, target cleanliness, operation state, and relevant
   ancestry before changing bytes. It fast-forwards when possible and otherwise performs a normal
   merge commit. It never runs tests or review, pushes, resolves conflicts, removes a worktree,
   records an integration disposition, or finishes the effort.

8. A merge conflict is a visible partial mutation, not a claimed rollback. The refusal reports that
   target bytes changed and directs the agent to resolve or abort the merge. A resolved conflict must
   pass the staged check and project gate and receive renewed terminal implementation review before
   resource removal, retrospective, or finish continues.

9. Managed worktree removal is restartable and inspects actual Git topology on every invocation. It
   safely handles the path, registration, and `awf/<slug>` branch as independently present or absent,
   removes only the topology it can prove belongs to that effort, and reports any remaining concrete
   action. `awf effort finish` refuses until the managed path, registration, and branch are all absent,
   preventing checkpoint deletion while integration or cleanup may remain.

10. Every enforced refusal states the failed condition, whether the command changed any bytes or Git
    topology, and one concrete next action. Confinement, symlink, ownership, repository-identity,
    foreign-byte, unresolved-merge, and destructive-topology checks are never forceable. Restartable
    commands inspect current filesystem and Git facts rather than trusting a prior attempted step.

11. Efforts use a user-managed single-writer-per-effort contract. Different slug directories may be
    operated concurrently, but awf adds no repository-wide or per-effort coordination lock. Exclusive
    creation prevents two writers from allocating the same slug; after creation, agents and users are
    responsible for not concurrently mutating one effort's memory or managed Git resources.

12. Advance the config schema and migrate resident ownership atomically with the ordinary upgrade.
    Upgrade discards legacy UUID JSON effort records and standalone `.awf/memory/` content rather than
    inventing slugs or migrating stale lifecycle state. It reports the reset under Breaking changes
    and refuses before destructive cleanup while any legacy UUID worktree path, registration, or
    branch remains, naming the required integration or removal action. The migration is retryable,
    preserves malformed or foreign bytes for manual handling, and leaves the new `.awf/efforts/` and
    `.awf/worktrees/` roots as the only resident roots owned by render, drift, discovery, sweep, and
    uninstall behavior.

13. Move the working-memory citation detector and Pi handoff confinement contract to the unified
    memory path. Durable ADRs, plans, and commit-message bodies may name the efforts directory or an
    angle-bracket placeholder path but may not cite a concrete ephemeral effort memory file. Pi
    handoff remains independent of lifecycle selection and accepts the confined owned memory path;
    it does not parse state, assign an effort, or mutate effort resources.

14. Treat the full agent workflow as first-class generated guidance and tested scope. The agent guide,
    working-memory documentation, and every applicable brainstorming, ADR, planning, implementation,
    review, checkpoint, handoff, retrospective, debugging, and task skill must agree that: minimal
    simple fixes use no effort; once a concrete non-minimal outcome is identified the agent creates
    or resumes exactly one slugged effort with owned memory; checkpoints and handoffs carry the slug
    and exact owned memory path; standalone memory is forbidden; and one effort has one writer.

15. For a worktree-backed effort, generated guidance inserts an explicit worktree-integration phase
    after terminal implementation review and before retrospective. The agent integrates into the
    intended clean target checkout, settles any conflict through the gate and renewed terminal
    review, removes the managed worktree explicitly, runs retrospective, updates any warranted
    durable lessons or changelog material, and only then finishes the effort. Without a managed
    worktree, terminal review proceeds directly to retrospective and finish. No skill treats review,
    retrospective, or finish as implicit effects of integration.

16. Update all command help, configuration reference, workflow and development documentation,
    architecture, glossary, pitfalls, changelog, rendered root declarations, generated guides and
    skills, Pi extension contracts, and tests in the same implementation. Deterministic tests cover
    slug derivation and collision, publication ordering and incomplete directories, legacy reset,
    resident-root ownership, refusal diagnostics and partial mutations, actual-Git restartability,
    citation and handoff paths, and complete generated-agent lifecycle coverage.

## State changes

- add `config/migrations-and-locks:unified-effort-resident-migration`
- update `tooling/effort-management:effort-record-authority`
- update `tooling/effort-management:managed-worktree-lifecycle`
- update `tooling/cli:effort-command-contract`
- update `tooling/quality-gates:memory-citation-gate`
- update `rendering/singletons-and-payloads:memory-gitignore-always-on`
- update `rendering/singletons-and-payloads:resident-output-preservation`
- update `rendering/project-output-plan:output-plan-complete`
- update `rendering/guide-and-doc-templates:working-memory-single-home`
- update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- add `rendering/workflow-skill-templates:unified-effort-workflow-coverage`
- update `rendering/pi-workflows:pi-session-handoff-lifecycle`
- update `rendering/pi-workflows:pi-session-handoff-public-contract`
- update `rendering/pi-workflows:pi-session-handoff-workflow`

## Consequences

Agents gain one meaningful identity and one checkpoint path for the whole outcome. Active effort
state becomes inspectable from directory presence, while worktree truth comes from Git rather than a
mutable local disposition. Removing terminal records and standalone memory eliminates lifecycle
repair and reconciliation paths, and guarded finish makes forgotten resources visible before the
only checkpoint is deleted.

The stricter workflow deliberately adds ceremony to concrete multi-step changes: they always create
memory, and worktree-backed work always integrates and removes resources between terminal review and
retrospective. Minimal simple fixes avoid the ceremony entirely. Meaningful deterministic slugs can
rarely collide; rejection is less convenient than suffixing but keeps identity stable and forces the
user to distinguish genuinely separate outcomes.

Upgrade intentionally deletes legacy effort and memory residents after proving no legacy managed
worktree remains. Those records are ephemeral coordination state rather than durable authority, so a
lossy reset is more truthful than fabricating slugs, history, or integration facts. Users must settle
or remove legacy worktrees before upgrading.

The implementation crosses resident storage, native Git mutation, CLI grammar, migration, output
planning, gates, Pi handoff, every workflow stage, generated documentation, and current-state claims.
A phased implementation plan, safety-focused tests, staged checks, the full gate, independent review,
and conditional post-review worktree integration are required.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep UUIDs as the public identity and add a display slug | Agents would still carry two identities, and paths and branches would remain opaque. |
| Auto-suffix colliding slugs | Silent suffixes weaken the promise that the outcome-derived slug is immutable and meaningful; an explicit distinction is safer. |
| Keep memory optional or retain `.awf/memory/` | Either choice preserves split ownership and permits a multi-step effort without its required checkpoint. |
| Keep completed or abandoned records | A retained lifecycle recreates a local history whose truth can diverge from Git and requires reopen, repair, and pruning semantics. |
| Store worktree and integration state in `state.json` | Git already owns those facts; duplicating them creates reconciliation and partial-failure states. |
| Integrate automatically during finish | Integration can conflict and requires gates and review; combining it with destructive checkpoint cleanup obscures changed bytes and unsafe recovery. |
| Automatically remove the worktree after integration | Explicit restartable removal exposes dirty or foreign topology and prevents integration from becoming an unexpectedly destructive command. |
| Migrate legacy records by deriving slugs from titles | Collisions, malformed records, stale lifecycle fields, and optional memory make the result guesswork rather than a truthful migration. |
| Add awf coordination locks | A lock cannot coordinate external Git and editor mutations reliably; the simpler contract is one user-managed writer per effort and independent concurrency across efforts. |

## Status history

- 2026-07-29: Proposed
