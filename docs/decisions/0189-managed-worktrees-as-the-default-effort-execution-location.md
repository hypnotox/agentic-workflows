---
format: current-state-v2
status: Proposed
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
2. `--no-worktree` is the explicit exception and keeps execution in the invoking checkout.
   Its output reports explicit absence: text uses `worktree=none`, JSON uses
   `"worktree": null`, and the next action names the invoking checkout.
3. Default creation preserves standalone Add base semantics: the new branch starts at the
   invoking checkout's `HEAD`. `awf effort new --base <ref>` exposes the same base
   selection as `worktree add --base` and is invalid combined with `--no-worktree`.
4. Failure of the worktree step is handled safety-constrained transactionally: the
   just-created effort is removed only when managed topology is proven absent; otherwise
   the effort is retained and the output reports the actual state and concrete recovery
   steps. Successful creation reports the worktree path and explicitly directs the caller
   to continue the effort there.
5. The shared worktree result gains structured path and branch facts so standalone Add and
   combined creation consume one result type; the line-oriented mutation protocol remains
   the text surface.
6. Effort finish gains a narrow structured outcome and error classification sufficient for
   rollback reporting; the orchestration never parses error prose.
7. Parallelism stays one worktree per effort. Same-effort mutations remain serialized by
   the explicit one-writer workflow contract; no binary coordination lock is added.
8. No runtime-specific relocation is added (Pi included): agents own moving their
   operations to the reported worktree. Command success and failure protocols carry the
   worktree state, location, and actionable next steps so that relocation is always
   instructed, never assumed.
9. Workflow, guide, checkpoint, and architecture authority follow the new default: the
   workflow doc's working-memory home, the rendered guides, the shared checkpoint
   partials, and the architecture overview direct effort execution to the effort's managed
   worktree unless the effort was created with `--no-worktree`.

## State changes

- add `tooling/effort-management:default-worktree-creation`
- update `tooling/effort-management:effort-record-authority`
- update `tooling/effort-management:managed-worktree-lifecycle`
- update `tooling/cli:effort-command-contract`
- update `rendering/guide-and-doc-templates:working-memory-single-home`
- update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`

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

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep worktrees opt-in (status quo) | The collision-prone shared checkout stays the default; the isolation step keeps being forgotten. |
| Unconditional rollback on worktree failure | Deleting the effort next to ambiguous or partially created topology risks destroying user state; safety-constrained removal only on proven-absent topology. |
| A binary coordination lock for same-effort writers | The one-writer workflow contract already serializes same-effort mutations; a lock adds stored state and stuck-lock failure modes for no new guarantee. |
| Runtime-specific relocation (e.g. Pi session move) | Couples the binary to harness internals; reporting the path and next action keeps relocation portable across runtimes. |
| Separate combined-creation implementation | Duplicating Git worktree behavior outside `Manager.Add` forks the validated topology checks; orchestrating the existing machinery keeps one authority. |

## Status history

- 2026-07-30: Proposed
