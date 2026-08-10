---
format: current-state-v4
slug: atomic-exclusive-artifact-publication
status: Implementing
date: 2026-08-10
---
# ADR-atomic-exclusive-artifact-publication: Atomic Exclusive Artifact Publication


## Context

ADR and plan scaffolding and project backup creation each currently choose an apparently
free destination with `os.Stat` and later publish with truncating `os.WriteFile`. Another
process can claim the name between those operations. Plans can therefore violate their
refuse-overwrite contract, and backups can overwrite the rescue copy they promise to
preserve. Numbered ADR creation is worse: two titles with different slugs produce different
filenames even when both corpus snapshots allocate the same highest-plus-one number, so
exclusive destination creation alone cannot preserve numeric identity uniqueness.

The repository already has cross-platform atomic no-replace publication in
`internal/effort`: Linux uses `renameat2(RENAME_NOREPLACE)`, Darwin uses
`renamex_np(RENAME_EXCL)`, and Windows uses a no-replace move plus the available file flush.
Reimplementing a weaker `O_CREATE|O_EXCL` write in each consumer would close the namespace
race only after exposing the destination to partial content, and would violate the
single-home rule for a mechanism now needed by several packages.

ADR-0202 requires numbered ADRs to allocate highest-plus-one on the integration branch and
forbids number reservations that can leave gaps. Serialization must therefore cover the
identity-corpus read, number selection, scaffold preparation, and no-replace publication,
without reserving a corpus number durably. The standard library has no one portable
cross-process advisory lock API. `github.com/gofrs/flock` is already present in the module
graph and provides the required released-platform behavior; making it a direct production
dependency is narrower than growing separate Unix and Windows lock implementations.

## Decision

1. `decision: single-exclusive-publication-home` A neutral leaf package named
   `internal/filepublication` owns complete-file atomic no-replace publication by path.
   The no-replace creation mechanism currently private to `internal/effort` moves to that
   package, and effort creation, ADR scaffolding, plan scaffolding, and backup creation use
   the one implementation. Replacement and removal policies that are specific to effort
   resident identity remain in `internal/effort`.
2. `decision: publish-complete-or-refuse` A creation is prepared as a complete
   same-directory temporary file and atomically published only when the destination is
   absent. Destination existence remains a matchable refusal and never changes existing
   bytes. A failure before publication leaves no partial destination; temporary cleanup is
   best effort and never replaces the primary error. ADR and plan scaffolds retain their
   requested `0644` mode, and backups retain the source file's permission bits; mode setup
   occurs before publication so no post-publication mode window is introduced. This is
   namespace atomicity and complete-file publication, not a new promise of power-loss
   durability beyond the platform behavior already owned by effort publication.
3. `decision: serialize-adr-allocation` ADR scaffolding takes a cross-process advisory lock
   keyed by the decisions directory's OS-resolved canonical identity before reading the
   identity corpus and holds it through publication. Canonicalization resolves absolute and
   symbolic-link aliases; on Windows it additionally resolves the final volume and path
   spelling so case and volume aliases share one identity. A SHA-256 encoding of that
   canonical identity forms the collision-resistant filename under a stable per-user cache
   location outside the repository. The lock uses `github.com/gofrs/flock` as a direct
   dependency. Its file is persistent and harmless; it is unlocked but not deleted,
   avoiding split-inode locking, and the operating system releases the advisory lock on
   descriptor close or process death. Distinct linked-worktree decision directories
   intentionally have distinct keys. The lock serializes allocation rather than reserving
   a number, so ADR-0202's contiguous highest-plus-one contract is unchanged.
4. `decision: retain-consumer-policy` Consumers retain their domain policy. ADR owns corpus
   identity and slug checks, plan owns its computed-path refusal, and project owns backup
   suffix selection. Backup creation retries the next suffix only after a matchable
   destination-exists refusal; other publication failures stop immediately. The shared
   package owns mechanism and neutral error identity, not naming, allocation, retry, or
   presentation policy.
5. `decision: prove-concurrent-publication` Deterministic tests synchronize competing
   publishers at the namespace boundary and prove that one complete destination wins while
   existing bytes never change. Cross-process ADR proving units invoke two creators against
   one canonical decisions directory, including different slugs competing for the same next
   number and alias paths resolving to the same lock. Plan and backup tests force a
   destination collision at publication; mode tests prove scaffold modes and backup source
   permissions. The added `exclusive-file-publication-single-home` claim is an invariant
   with `Backing: test`, proved by the shared-package and cross-package structural tests
   under `internal/filepublication`.

## State changes

- add `tooling/file-publication:exclusive-file-publication-single-home`
- update `adr-system/adr-lifecycle:adr-new-no-overwrite`
- update `adr-system/adr-lifecycle:adr-new-sequential-numbering`
- update `adr-system/plan-artifacts:plan-new-unnumbered`
- update `rendering/sync-and-drift:sync-backs-up-foreign`
- update `rendering/companion-scripts:runner-prune-backup`

## Consequences

- Concurrent ADR creators that target one decisions directory observe serialized corpus
  state and cannot return duplicate numbers or clobber a pending record. Worktrees remain
  independent until their existing merge-time numbering boundary.
- Plans and backups uphold their no-overwrite promises at the publication boundary rather
  than relying on a check-then-write window.
- A new small package and one direct dependency are accepted to keep the mechanism in one
  cross-platform home. Callers still carry their own naming and retry rules rather than
  acquiring a universal filesystem abstraction.
- Same-directory temporary files may remain after abrupt process death. They are not valid
  ADRs, plans, or backups, and ordinary failed calls remove them best-effort; automated stale
  cleanup is outside this decision.
- The stable advisory lock location is per-user local state rather than repository state.
  Cache-directory unavailability is an explicit scaffolding error, not a fallback to unsafe
  allocation.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Use `os.OpenFile` with `O_CREATE|O_EXCL` directly in each consumer | It duplicates policy and can expose a partially written destination after interruption. |
| Make `internal/filesystem.Handle` own publication | That package owns deliberately composed root-confined access; path-based cross-platform publication is a distinct mechanism and forcing it into the handle would widen that boundary. |
| Serialize numbered ADR creation only with a process-local mutex | It does not protect separate CLI processes, the race that matters. |
| Use an exclusive sentinel file or directory as the ADR lock | A crash leaves a stale reservation that blocks future creation or requires unsafe stale-owner inference. |
| Optimistically publish numbered ADRs and repair duplicate numbers afterward | A transient invalid corpus is externally visible, and two successful callers cannot both retain the number without violating identity uniqueness. |

## Status history

- 2026-08-10: Proposed
- 2026-08-10: Implementing; content-sha256: 748444a3c6dab12a892cdc71e3bb9ed2c1edafd48281491c2fa99f91dd4f0b33
- 2026-08-10: Applied; operations: add `tooling/file-publication:exclusive-file-publication-single-home`
- 2026-08-10: Applied; operations: update `adr-system/plan-artifacts:plan-new-unnumbered`, update `rendering/sync-and-drift:sync-backs-up-foreign`, update `rendering/companion-scripts:runner-prune-backup`
- 2026-08-10: Applied; operations: update `adr-system/adr-lifecycle:adr-new-no-overwrite`, update `adr-system/adr-lifecycle:adr-new-sequential-numbering`
