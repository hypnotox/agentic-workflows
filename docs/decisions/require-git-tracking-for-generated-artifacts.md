---
format: current-state-v4
slug: require-git-tracking-for-generated-artifacts
status: Proposed
date: 2026-08-14
---
# ADR-require-git-tracking-for-generated-artifacts: Require Git Tracking for Generated Artifacts


## Context

awf renders a deterministic set of project artifacts and instructs adopters to commit those outputs with `.awf/awf.lock`. Ordinary repository drift currently compares planned bytes with working-tree bytes, while staged drift compares planned bytes with entries that happen to be present in the Git index. Neither universe proves that every generated artifact is tracked. In particular, an adopter's global ignore rules can hide a newly rendered file from `git status` and ordinary `git add`, leaving awf's filesystem check clean while the eventual commit omits that file. Staged drift also currently treats an absent expected output as neutral.

ADR-0210 deliberately restricted staged drift to `stale` and `hand-edited`. Preventing incomplete commits requires a successor decision because index absence is a third blocking property. The generated set already has one project-owned authority in the output plan, except for the lock, which awf writes separately as transaction authority. Git membership and ignore semantics remain owned by the `internal/git` seam.

## Decision

1. `decision: generated-artifacts-must-be-indexed` Repository and staged drift checks require every awf output-plan write and `.awf/awf.lock` to be present in Git's index. An absent path is blocking `untracked` drift, independent of repository, local, or global ignore rules. This expands ADR-0210's staged drift vocabulary beyond `stale` and `hand-edited`.
2. `decision: ownership-follows-existing-authorities` The project output plan remains the authority for which generated outputs require tracking, with the separately written lock included explicitly, while the Git seam remains the authority for index membership. Nested adopted projects retain their existing resident-output exclusion because those outputs live outside the adopted subtree's index authority.
3. `decision: no-git-degrades-narrowly` Outside a Git repository, only the tracking projection is unavailable. Repository drift continues to evaluate its filesystem properties and reports a dedicated non-failing tracking-unavailable advisory.

## State changes

- add `rendering/sync-and-drift:generated-artifacts-tracked`
- update `rendering/sync-and-drift:staged-drift-rendered-output`
- update `rendering/project-output-plan:check-report-single-plan`
- update `tooling/cli:repo-check-capability-plan`

## Consequences

Generated output can no longer pass awf verification merely because ignored working-tree bytes are current; adopters receive a blocking finding before committing an incomplete render transaction. Staged deletion of a generated artifact also becomes visible even when its expected content is otherwise fresh.

The check must inspect index metadata in addition to rendering and filesystem state. An untracked path takes precedence over a simultaneous missing-file classification so one path receives one actionable root-cause finding. The lock requires an explicit check because it is not an output-plan node, and staged preparation must preserve an actionable result when that lock itself is absent.

Tracked files remain valid when a later ignore rule matches them because ignore rules do not change index membership. Unreadable index metadata prevents a confident tracking result. Outside Git, adopters receive an advisory instead of losing the repository drift checks that do not require Git. For a nested adopter, resident-root self-ignore outputs remain outside tracking enforcement because they live outside the adopted subtree's index authority; this accepted gap preserves the existing resident ownership boundary.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Infer tracking from the working-tree status universe | That universe intentionally mixes tracked files with visible untracked files and excludes ignored untracked files, so it cannot answer index membership directly. |
| Derive the required set solely from `.awf/awf.lock` | The lock describes the previous render transaction, can omit newly planned outputs, and cannot establish tracking of the lock itself. |
| Reuse the complete staged blob snapshot for every check | Reading all blob contents is unnecessary for membership and couples the tracking question to blob readability. |
| Invoke `git ls-files` once per generated path | Repeated subprocess queries are inefficient and bypass the backend-neutral semantic seam. |

## Status history

- 2026-08-14: Proposed
