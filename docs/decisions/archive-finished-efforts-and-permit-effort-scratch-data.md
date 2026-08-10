---
format: current-state-v4
slug: archive-finished-efforts-and-permit-effort-scratch-data
status: Proposed
date: 2026-08-10
---
# ADR-archive-finished-efforts-and-permit-effort-scratch-data: Archive Finished Efforts and Permit Effort Scratch Data


## Context

Effort memory is deliberately ephemeral coordination state. ADR-0175 made
`awf effort finish` a restartable deletion so completed residents could not become a
second project history or accumulate privacy and cleanup obligations. That keeps Git and
current-state documentation authoritative, but it also discards the only local record of
how an effort used memory. A machine-local collection of finished memories would support
later manual inspection across several efforts without making those records tracked,
queryable, or authoritative.

Efforts also have no confined home for disposable files. Testing and investigation commonly
use system temporary directories even when their fixtures, logs, or experiment outputs
belong to one effort. The resident currently accepts only `state.json`, `memory.md`, and
optional `activity.json`; any other direct leaf makes the resident foreign and prevents
ordinary operations. Permitting arbitrary direct leaves would weaken that safety boundary,
but one explicit opaque scratch directory can provide a local drop area while keeping the
owned protocol closed.

The existing finish sequence first proves managed worktree topology absent, validates the
resident, and atomically renames it to a UUID-bearing finishing reservation before deleting
that reservation. The reservation makes pre-deletion interruption retryable by slug and
prevents immediate slug reuse. Replacing deletion with a second same-control-root rename can
retain the validated bytes while preserving the existing active-to-finishing transition.
The archive destination still needs no-replace collision safety, explicit cross-parent
durability reporting, and an ignore marker before any private memory can move there.

The archive is intentionally not durable project history. It is ignored, local,
unmanaged, manually disposable data that may be included in filesystem backups or exposed
by local access despite being absent from ordinary Git status. The user accepted that
trade-off and explicitly ruled out archive commands, programmatic analysis, and retention
policy.

## Decision

1. `decision: archived-finish` Finishing an effort retires its active resident by moving the
   complete validated directory to the repository-wide, self-ignored machine-local path
   `.awf/effort-archive/<uuid>-<slug>/` instead of deleting it. The archive is ephemeral and
   non-authoritative: awf exposes no archive inventory, selection, restoration, retention,
   pruning, analysis, or other management surface, and users may inspect or delete its
   descendants manually.
2. `decision: stable-archive-identity` The archive name uses the effort's exact immutable
   lowercase UUID, one hyphen, and its exact slug. This preserves recognizable identity,
   allows a finished slug to be reused, and makes destination collision a refusal rather
   than replacement. UUID uniqueness does not relax no-clobber safety.
3. `decision: restartable-archive-transition` Finish retains its guarded active-to-finishing
   reservation transition. Moving that reservation to the archive is the completion and
   slug-release point. Before that point, retry by slug remains valid; after it, the result
   names the exact archive destination, and durability uncertainty directs inspection of
   the reported source and destination rather than blind retry. The move follows the
   repository's platform-specific atomic namespace and parent-sync durability model and
   never falls back to copy-and-delete across filesystems.
4. `decision: archive-ignore-precondition` A new registered config generation introduces
   the archive root so a project on an older generation must run `awf upgrade` before effort
   commands can proceed; ordinary render repairs the marker only after the project is at the
   current generation. Finish may archive private effort bytes only after proving that the
   confined archive root is a real safely owned directory and its governed marker is a safe
   regular file whose bytes match the planned self-ignore output. A missing, symlinked,
   foreign, or stale root or marker refuses before the active resident changes. This closes
   the window in which a newer binary could move private memory into an unignored archive
   in an older project.
5. `decision: opaque-effort-scratch` A valid active effort may contain one optional
   `scratch/` direct child. The child itself must be a real, safely owned directory, but awf
   treats every descendant as opaque and does not traverse, validate, list, interpret, or
   manage it. Creation does not scaffold scratch. Finish preserves it through the directory
   move, while the owned state, memory, and activity protocols remain closed and strictly
   validated.
6. `decision: creation-rollback-is-not-finish` Safety-constrained rollback after default
   managed-worktree creation fails continues to remove the just-created resident when and
   only when topology is proven absent. It uses a narrow internal rollback boundary rather
   than public finish, because a creation that never succeeded is not a finished effort and
   should not enter the archive.
7. `decision: third-resident-root` Rendering, drift, sweep, uninstall, and confinement
   recognize the archive as a third repository-wide resident root beside active efforts and
   managed worktrees. Only its self-ignoring marker is governed output; archived descendants
   remain local bytes preserved without recursive interpretation. The marker template stays
   coherent under missing-key-zero rendering and cannot emit unresolved or no-value tokens
   when variables are empty.

## State changes

- add `config/migrations-and-locks:archive-root-upgrade-boundary`
- update `tooling/effort-management:effort-record-authority`
- update `tooling/effort-management:default-worktree-creation`
- update `tooling/cli:effort-command-contract`
- update `rendering/singletons-and-payloads:memory-gitignore-always-on`
- update `rendering/singletons-and-payloads:resident-output-preservation`
- update `rendering/project-output-plan:output-plan-complete`

## Consequences

Finished memories and effort-local scratch data remain available for manual comparison after
active work ends, without introducing a terminal effort lifecycle or a tracked source of
truth. Reusing a slug remains possible because archive identity also includes the immutable
UUID. A dedicated scratch boundary lets tests and investigations keep local data with their
effort instead of scattering it through system temporary directories.

The archive consumes disk indefinitely until a user deletes it, and ignored memory may still
be captured by backups, force-added to Git, or read by another local process. awf neither
measures nor mitigates that retention beyond making the root self-ignored and documenting
its disposable, machine-local status. Users who do not want retained content must remove it
manually.

Opaque scratch descendants deliberately receive no type, ownership, symlink, size, or content
validation. Safety comes from validating the scratch boundary itself and from finish moving
the containing directory without traversing or copying descendants. Operations that require
cross-filesystem copying refuse rather than reinterpret arbitrary content.

Finish gains a second durable namespace transition and more precise partial-result reporting.
A destination collision preserves both sides for manual inspection. Once the archive rename
completes, the effort is inactive even if a later parent sync reports uncertain durability;
the exact path in the result is the recovery evidence.

The third resident root widens rendering and repository-walking assumptions. Keeping it in the
single closed resident-root registry allows output planning, drift exemptions, preservation,
and repository-walking exclusions to derive from the same authority. Registering a config
generation makes adopters run upgrade even though their authored configuration may need no
semantic edit; that boundary is the cost of ensuring the ignored root exists before any newer
finish command can retain private bytes. At the current generation, a missing or stale marker
remains an actionable render repair rather than an implicit mutation by finish.

Default worktree creation can no longer reuse public finish for rollback. The narrow internal
delete path is justified only for the just-published resident in the same failed creation
transaction after topology absence is proven; it is not a general deletion command.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Continue deleting finished efforts | It prevents later manual comparison of actual memory usage and discards useful local research material. |
| Add archive list, restore, prune, or analysis commands | Management would turn disposable local bytes into a supported lifecycle and retention model that the user explicitly does not want. |
| Commit finished memories or archive metadata | Effort memory can contain transient or sensitive context and must not become tracked project authority. |
| Store archives outside the repository control root | A global location loses repository identity and the existing confinement, rendering, and preservation model. |
| Permit arbitrary direct effort leaves | It makes owned protocol validation ambiguous; one optional opaque directory keeps a closed root boundary. |
| Recursively validate scratch contents | Arbitrary testing data would inherit complex file-type, symlink, ownership, and compatibility policy with no consumer that needs it. |
| Delete activity before archiving | It mutates the requested complete effort folder and removes potentially useful local context; the entire valid resident moves unchanged. |
| Let failed creation rollback archive through finish | It would collect residents for efforts whose default creation did not complete and conflate transaction rollback with successful finalization. |

Append-only Status history starts with Proposed. Later status events carry the latest content stamp,
and an Amended event records each post-Accepted amendment with its new digest. Incremental
implementation first appends an Implementing status event and its first Applied event. One authored
transaction may append several Applied or Reapplied batches only across distinct claim IDs; a
repeated same-claim occurrence requires a separately observable authored transaction. Status history
may append several events when the prior history remains an exact prefix and the appended events
replay as a legal ordered lifecycle. Each event is unordered membership over declared operations,
while history remains ordered. Amend an unapplied operation directly. An already-applied add or
update may be corrected with a Reapplied event and its material claim correction throughout
Implementing; otherwise use a follow-up ADR or remove plus add. The final explicit batch remains
Implementing, and settled review later appends only Implemented. For example:

- YYYY-MM-DD: Implementing; content-sha256: `<64 lowercase hex characters>`
- YYYY-MM-DD: Applied; operations: update `<domain>/<topic>:<slug>`
- YYYY-MM-DD: Reapplied; operations: update `<domain>/<topic>:<slug>`

## Status history

- 2026-08-10: Proposed
