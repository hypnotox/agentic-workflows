---
format: current-state-v2
status: Implementing
date: 2026-07-30
---
# ADR-0189: Managed worktrees as the default effort execution location

## Context

An effort today is created without any execution isolation: `awf effort new` publishes the
resident record and memory in the invoking checkout, and a managed worktree exists only when
someone separately runs `awf effort worktree add <slug>`. ADR-0164 deliberately made Add
"separate from effort creation", and the guide treats the worktree as conditional topology
that terminal review integrates and removes when present.

In practice, independently running efforts therefore share the primary checkout by default.
Concurrent sessions collide on the shared Git index, on staged transactions, and on
uncommitted files; the workflow compensates with discipline (fresh `git status` before add,
pathspec commits) instead of structure. The optional worktree step exists precisely to
prevent this, but because it is a second command that agents must remember, the default
path remains the colliding one.

Brainstorming settled the inversion: isolation should be the default an effort gets for
free, and sharing the invoking checkout should be the explicit exception. The design was
grounded against the current code and verified:

- `awf effort new` durably publishes `.awf/efforts/<slug>/state.json` after its always-owned
  `memory.md` (ADR-0164, ADR-0175); creation stores no worktree attachment state.
- `worktree.Manager.Add(slug, base)` (`internal/worktree/manager.go`) already requires the
  effort to exist, validates managed topology (path, registrations, branch), and creates
  `.awf/worktrees/<slug>/` on `awf/<slug>`. Its `Result` is a purely line-oriented protocol
  (`Condition`, `ChangedTopology`, `NextAction`) with no structured path or branch facts.
- `effort.Service.Finish` reports `FinishResult{Renamed, Cleaned}` and prose errors; a
  caller that must distinguish "removal refused because managed topology is present" from
  other failures would have to parse error text.
- The one-writer contract (ADR-0175, ADR-0186) serializes same-effort mutations at the
  workflow level; no binary coordination lock exists.
- An unrelated in-flight modification, `docs/plans/2026-07-30-orienting-support-skill-adr-0187.md`,
  must remain untouched by this work.

The user approved the grounded design, including the safety-constrained failure model:
after a worktree creation failure, the just-created effort may be removed only when managed
topology is proven absent; anything less certain retains the effort and reports recovery
steps rather than risking deletion next to ambiguous state.

## Decision

1. `awf effort new` creates one managed worktree per effort by default. After publishing
   the effort residents it invokes the same standalone `Manager.Add` machinery to create
   `.awf/worktrees/<slug>/` on `awf/<slug>`; the orchestration lives in the existing
   worktree lifecycle manager and duplicates no Git worktree behavior.
2. The `new` reply carries the worktree outcome alongside the unchanged effort object:
   successful default creation reports `"worktree": {"path": <path>, "branch": <branch>}`
   in JSON. `--no-worktree` is the explicit exception and keeps execution in the invoking
   checkout; its output reports explicit absence: text uses `worktree=none`, JSON uses
   `"worktree": null`, and the next action names the invoking checkout. `show` and `list`
   keep their current shapes: the worktree field is a creation outcome report, not stored
   state, and Git remains the topology authority for later inspection. `schemaVersion`
   stays 2; the effort object's shape is unchanged. Effort command output expresses the
   owned memory path resolvably: when the invoking checkout is not the primary root, the
   `memoryPath` value and the text `memory=` fact are primary-root-qualified absolute
   paths; from the primary root the repository-relative form remains. Guides and
   checkpoint partials keep the literal `.awf/efforts/<slug>/memory.md` form and state
   that it is primary-root-relative.
3. Default creation preserves standalone Add base semantics: the new branch starts at the
   invoking checkout's `HEAD`. `awf effort new --base <ref>` exposes the same base
   selection as `worktree add --base` and is invalid combined with `--no-worktree`.
4. Failure of the worktree step is handled safety-constrained transactionally: rollback
   invokes the existing restartable finish path, whose managed-topology guard already
   refuses removal unless topology is proven absent, so the just-created effort is removed
   only in that proven case; otherwise the effort is retained and the output reports the
   actual state and concrete recovery steps. Successful creation reports the worktree path
   and explicitly directs the caller to continue the effort there.
5. The shared worktree result gains structured path and branch facts so standalone Add and
   combined creation consume one result type; the `new` reply's worktree field (item 2) is
   their first consumer, and the line-oriented mutation protocol remains the text surface.
6. Effort finish gains a narrow structured outcome and error classification sufficient for
   rollback reporting; it exists to distinguish the finish path's managed-topology refusal
   (item 4) from other failures, and the orchestration never parses error prose.
7. Parallelism stays one worktree per effort. Same-effort mutations remain serialized by
   the explicit one-writer workflow contract; no binary coordination lock is added.
8. No runtime-specific relocation is added (Pi included): agents own moving their
   operations to the reported worktree. Command success and failure protocols carry the
   worktree state, location, and actionable next steps so that relocation is always
   instructed, never assumed. One pre-existing contract is repaired in the same change:
   Pi's rendered handoff extension resolves its repository root from its own rendered
   location, so memory validation fails outright from any managed worktree (the rendered
   extension's root is the worktree, where the effort memory does not exist). The handoff
   root resolution is changed to the primary control root so `validateMemoryPath` accepts
   the effort memory from any managed worktree. This is root resolution, not relocation:
   the session still moves itself.
9. Workflow, guide, checkpoint, and architecture authority follow the new default: the
   workflow doc's working-memory home, the rendered guides, the shared checkpoint
   partials, and the architecture overview direct effort execution to the effort's managed
   worktree unless the effort was created with `--no-worktree`. Two homes state the
   current separation most directly and move in lockstep with this change: the standard
   template `templates/docs/working-with-awf.md.tmpl` and this project's local override
   `.awf/parts/working-with-awf/commands.md` (both currently describe creation and
   `worktree add` as separate operations), plus the glossary's "Managed effort worktree"
   entry (currently "Optional"), which is reworded to default-with-`--no-worktree`-exception.
10. The added claim `tooling/effort-management:default-worktree-creation` is an invariant
    with `Backing: test`; its proof lands on the combined-creation test in the worktree or
    effort package when the operation is Applied. Operation motivation: items 1 and 4
    drive the added claim and the `managed-worktree-lifecycle` update; items 4 and 6 drive
    the `effort-record-authority` update; items 2, 3, and 5 drive the
    `effort-command-contract` update; item 9 drives the guide and checkpoint rendering
    updates; item 8 drives the `pi-session-handoff-public-contract` update.

## State changes

- add `tooling/effort-management:default-worktree-creation`
- update `tooling/effort-management:effort-record-authority`
- update `tooling/effort-management:managed-worktree-lifecycle`
- update `tooling/cli:effort-command-contract`
- update `rendering/guide-and-doc-templates:working-memory-single-home`
- update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- update `rendering/pi-workflows:pi-session-handoff-public-contract`

## Consequences

- Independently running efforts stop sharing the primary checkout by default; index and
  staging collisions between concurrent sessions become the exception rather than the
  norm. The shared-checkout discipline still applies inside one worktree, but no longer
  across efforts.
- Every default effort now costs a worktree checkout on disk and a branch, and terminal
  review's conditional integration and removal becomes the common path instead of the
  rare one. Efforts whose work is genuinely tied to the invoking checkout must remember
  `--no-worktree`.
- Creation can now partially fail. The safety-constrained rollback keeps failure handling
  honest: a proven-clean failure leaves nothing behind, and an ambiguous one prefers a
  retained effort plus recovery instructions over any deletion near unproven topology.
- The structured worktree facts and the finish outcome classification are new small
  surfaces that must stay narrow; they exist for orchestration and reporting, not as a
  general status API. Stored worktree attachment state remains ruled out.
- Agents receive relocation as instructions in command output rather than as harness
  behavior; a runtime that ignores the reported next action will keep working in the
  invoking checkout, which the output makes visible but cannot prevent.
- Base selection inherits the invoking checkout: an agent that starts effort B while
  sitting in effort A's managed worktree branches `awf/<B>` off A's unmerged tip, and the
  worktree default turns that from a rare edge case into a reachable everyday one.
  `--base <ref>` is the escape; the reported branch and path make the result inspectable.
- This changes the default behavior of a shipped command for every adopter: an existing
  `awf effort new` invocation now performs a Git mutation and inherits `Manager.Add`'s
  refusal surface (for example an in-progress rebase or merge in the invoking checkout).
  The change lands with an `[Unreleased]` changelog entry naming the new default and the
  `--no-worktree` escape.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep worktrees opt-in (status quo) | The collision-prone shared checkout stays the default; the isolation step keeps being forgotten. |
| Guidance-layer default (skills always run `worktree add` after `effort new`) | Rendered prose is advisory and unenforced; the diagnosed failure mode is exactly agents skipping instructed steps, and the binary's own success protocol still could not report worktree state. |
| Unconditional rollback on worktree failure | Deleting the effort next to ambiguous or partially created topology risks destroying user state; safety-constrained removal only on proven-absent topology. |
| A binary coordination lock for same-effort writers | The one-writer workflow contract already serializes same-effort mutations; a lock adds stored state and stuck-lock failure modes for no new guarantee. |
| Runtime-specific relocation (e.g. Pi session move) | Couples the binary to harness internals; reporting the path and next action keeps relocation portable across runtimes. |
| Separate combined-creation implementation | Duplicating Git worktree behavior outside `Manager.Add` forks the validated topology checks; orchestrating the existing machinery keeps one authority. |

## Status history

- 2026-07-30: Proposed
- 2026-07-31: Implementing; content-sha256: b4496ecf0ecf853073f1efea851da34b004df3b34471f2e2f60669d050422f52
- 2026-07-31: Applied; state-sequence: 102; operations: add `tooling/effort-management:default-worktree-creation`, update `tooling/effort-management:effort-record-authority`, update `tooling/effort-management:managed-worktree-lifecycle`, update `tooling/cli:effort-command-contract`
- 2026-07-31: Applied; state-sequence: 103; operations: update `rendering/guide-and-doc-templates:working-memory-single-home`, update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
