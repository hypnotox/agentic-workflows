---
format: current-state-v4
slug: root-confined-sync-filesystem-mutations
status: Proposed
date: 2026-08-11
---
# ADR-root-confined-sync-filesystem-mutations: Root-confined sync filesystem mutations

## Context

Project sync currently derives slash-relative rendered paths but turns them into lexical absolute
paths before mutation. It then creates parents, observes and writes outputs, corrects modes, creates
foreign-file and retired-runner backups, prunes obsolete outputs and their empty ancestors, and
loads and saves the lock through direct `os` or absolute-path helpers. A symlink at an output path or
in one of its parents can therefore redirect a write, chmod, backup, removal, or authority-file
operation outside the selected checkout. Exclusive backup publication prevents a competing writer
from clobbering a backup destination, but it does not confine either the source read or the
publication parent.

The output model has two legitimate anchors. Tracked outputs belong to the invoking checkout, while
resident-root marker outputs belong to the primary checkout when it differs. Root confinement must
preserve that routing rather than forcing every output beneath `Project.Root` or treating a primary
resident write as an escape.

ADR-0216 established `internal/filesystem` as the single production home for deliberately composed
root-confined access, selected at an outer boundary, and explicitly left project sync as a bounded
future conversion candidate. Its concrete handle already provides the kernel-backed `os.Root`
boundary and most mutation capabilities sync needs. The remaining conversion must keep project
policy local: output-root selection, foreign attribution, backup naming and retry, change reporting,
prune meaning, and lock authority do not belong in the filesystem mechanism.

A final symlink needs explicit policy. A managed symlink can be replaced as the entry at the output
path without following its target. A foreign symlink remains subject to the existing backup-before-
overwrite contract, but an escaping or broken target cannot be copied as file bytes without either
leaving the root or inventing a different backup representation. Safe refusal is the only behavior
that preserves both confinement and the established backup meaning.

## Decision

1. `decision: complete-sync-mutation-boundary` Treat one sync operation as a complete root-confined
   filesystem mutation boundary. Before its first mutation, select root-confined handles for the
   tracked checkout and for a distinct primary resident root, route every lock-relative output
   through the existing tracked-versus-resident policy, and perform lock load and save, output
   observation and replacement, parent creation and mode correction, backup publication, retired
   output removal, and empty-ancestor cleanup through the selected handle using slash-relative
   paths.

2. `decision: consumer-owned-sync-contract` Keep the root-confined mechanism in
   `internal/filesystem` and the sync policy in `internal/project`. Project declares one private,
   cohesive structural contract for the multi-operation dependency needed by sync; the concrete
   production handle supplies it and gains only capabilities used by this consumer. Open every
   required production handle before mutation begins, and do not turn close failure after reported
   durable operations into a contradictory sync failure.

3. `decision: final-symlink-policy` Observe the final output entry without following it before
   choosing sync behavior. Replace a managed final symlink without reading its target. For a foreign
   final symlink, retain backup-before-overwrite only when one confined open can read its target
   bytes and mode; refuse without backup or replacement when the target escapes the selected root,
   is broken, or otherwise cannot be read safely.

4. `decision: complete-output-replacement` Replace each rendered output as one complete
   same-directory file whose final mode is established before the confined namespace replacement.
   A failed replacement commits neither new output bytes nor a mode-only change, and sync reports an
   output change only after the replacement succeeds.

5. `decision: preserve-sync-policy` Preserve project-owned foreign attribution, backup suffix and
   collision retry, complete exclusive backup publication, source permission bits, error identity,
   backup and change reporting, corrupt-lock pre-mutation refusal, actual-removal reporting, and
   deepest-first empty-ancestor pruning across both output anchors. Back the complete root-confined
   sync mutation invariant with tests.

6. `decision: bounded-conversion` Leave uninstall, upgrade migrations, snapshot capture, and
   unrelated historical direct filesystem effects outside this conversion. They remain governed by
   their existing authority and qualify for later root-confined conversion only through their own
   concrete consumer boundary.

## State changes

- add `rendering/sync-and-drift:sync-mutations-root-confined`
- update `rendering/sync-and-drift:sync-backs-up-foreign`
- update `config/migrations-and-locks:lock-atomic-save`

## Consequences

A symlinked output, output parent, backup parent, prune parent, or lock parent can no longer redirect
sync mutation outside its selected tracked or resident root. The lock remains the final authority
write, but its read and replacement now share the same confinement boundary as the outputs whose
state it records. Opening both anchors before mutation prevents a late resident-root open failure
from following successful tracked mutations.

Output replacement becomes complete and mode-coherent instead of an in-place write followed by a
separate chmod. This changes partial-failure reporting deliberately: a mode failure before rename no
longer leaves new bytes that sync must report as changed. Same-directory replacement does not claim
a stronger power-loss durability guarantee than the underlying handle provides.

Foreign in-root symlinks retain byte-copy backup behavior. Foreign escaping and broken symlinks now
cause a safe refusal, so an adopter must replace or repair the symlink before rendering. Managed
symlinks need no rescue copy and are replaced directly as governed output entries.

The project package gains a private multi-operation test seam because confinement, root selection,
replacement, backup, pruning, and lock behavior form one cohesive sync dependency. The provider does
not gain a universal interface or project policy. The conversion adds capability to the existing
single production handle rather than introducing another filesystem implementation.

Deliberately ignoring post-operation root-handle close failures loses that diagnostic signal. This is
accepted because each durable operation reports its own outcome, while returning a close error after
successful mutation would report the completed sync as failed.

Pruning and lock I/O make the change broader than publication alone, but excluding them would leave
the same redirection class inside one sync transaction and make any claim of a confined sync
boundary misleading. Other filesystem consumers remain unchanged.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Confine rendered-file writes and backups only | Leaves prune deletion, ancestor cleanup, and lock authority I/O redirectable inside the same sync operation. |
| Open the resident-root handle only when its first output is reached | A late open failure could follow already-successful tracked-root mutations. |
| Canonicalize or clean lexical absolute paths before direct OS calls | Path validation cannot prevent a symlink substitution or traversal race after validation. |
| Back up an escaping symlink as link text or overwrite it without backup | Changes the established foreign-file backup representation or silently violates backup-before-overwrite. |
| Put output routing and backup policy into `internal/filesystem` | Makes the shared mechanism depend on project and rendering semantics owned by its consumer. |
| Convert every historical direct filesystem caller in the same change | Recreates the broad conversion ADR-0216 rejected and lacks one cohesive policy consumer. |

## Status history

- 2026-08-11: Proposed
