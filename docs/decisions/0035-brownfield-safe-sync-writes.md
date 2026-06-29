---
status: Proposed
date: 2026-06-29
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [tooling, adoption, sync, safety]
related: [0016, 0023]
domains: [tooling]
---
# ADR-0035: Brownfield-Safe Sync Writes

## Context

ADR-0023 made `awf init --force` back up colliding non-managed files before overwriting them, so
a one-shot adoption never silently destroys hand-written content. But that guard lives only in the
`init` path (`cmd/awf/main.go:295` calls `BackupFile`); `awf sync` has no equivalent. `sync` writes
every rendered file through a single unguarded `os.WriteFile` loop (`internal/project/project.go`,
the write at ~line 95), so the moment a sync would produce a path that already holds a foreign
hand-written file, that file is destroyed with no backup and no report.

An adopter hit this directly: after initial adoption, enabling a previously-disabled doc in
`config.yaml` and running `sync` clobbered an existing hand-written file of the same name, because
that path was never recorded in the lock and so was never treated as a collision. `init`'s backup
does not help — the destructive write happens on a later routine `sync`, not at adoption.

The same gap underlies the `ACTIVE.md` ownership trap. `docs/decisions/ACTIVE.md` (and the per-domain
`docs/domains/<domain>.md` indexes) are awf-generated on *every* sync and recorded in the lock with
empty hash fields. A project that already generates its own `ACTIVE.md` via some external mechanism
has awf silently take over that file on the first sync that writes it; the adopter's old generator
then fights awf's. There is no signal that ownership changed hands.

The unifying principle is the one ADR-0023 already chose for `init`, simply not yet enforced
everywhere awf writes: **awf must never silently overwrite a file it did not write.** A file
recorded in the lock is awf's own output and is overwritten freely; a file on disk that is *not* in
the lock is foreign and must be preserved before awf claims its path. This ADR extends that rule to
`sync`. It is a pure extension recorded through `related` (no supersedence): it generalises
ADR-0023's `init --force` backup to a new write path and reuses ADR-0016's collision/lock-ownership
model unchanged — it narrows no decision item of either, so neither is partially superseded (the
"extension vs partial-item supersedence" distinction ADR-0031 draws). Both predecessors keep their
`Implemented` status.

## Decision

1. **Sync backs up foreign files before overwriting.** Before `sync` writes any rendered output —
   including the generated `ACTIVE.md` and the per-domain index docs — it checks, against the lock
   as it exists at the *start* of the sync (the prior manifest), whether the target path already
   exists on disk and is *not* recorded there as awf-written. If both hold, the file is foreign:
   `sync` copies it to a free `<path>.awf-bak[.N]` sibling via the existing `BackupFile` /
   `freeBackupPath` mechanism (never clobbering a prior backup), reports the backup on stdout, then
   proceeds with the write. A path recorded in the prior lock is awf-managed and is overwritten with
   no backup, so routine re-sync of awf's own output stays silent and clean. This mirrors
   ADR-0023's `init --force` rule and shares its implementation; like that rule, a foreign file that
   happens to be byte-identical to awf's output is still backed up (membership in the lock, not
   content equality, is the test).

2. **Surface ADR-index ownership takeover.** When the foreign file `sync` backs up is the generated
   ADR index (`docs/decisions/ACTIVE.md` under a default layout) or a per-domain index under the
   domains directory, the report additionally states that awf now owns that file's generation and
   that any external generator for it should be retired. The match is against the layout-derived
   paths — the ADR-index path and the domains-directory prefix the layout already exposes (the same
   `ActiveMd` / `DomainsDir` sources `awf check` keys on), not hardcoded literals — so the signal
   stays correct when the docs layout is relocated. This is a deterministic, path-based signal (no
   content heuristic): the takeover is reported the one time it happens — the first sync that writes
   the path — after which the file is lock-recorded and no longer foreign.

3. **No new flag; backup is unconditional.** Unlike `init`, which gates backup behind `--force`
   (because `init` otherwise refuses on collision), `sync` has no refusal mode to preserve: it
   always backs up a foreign file and always proceeds. There is no `sync --force` and no way to opt
   out of the backup, because a non-destructive backup has no downside worth a flag.

## Invariants

- `inv: sync-backs-up-foreign` — during `awf sync`, a target path that exists on disk and is not
  recorded as awf-written in the lock at the start of the sync is copied to a free `.awf-bak[.N]`
  sibling before being overwritten, and the backup is reported; a path recorded in that lock is
  overwritten with no backup. Backed by a test that syncs over both a foreign colliding file and a
  lock-recorded one and asserts exactly the foreign file is backed up.

## Consequences

- The reported clobber bug is closed: enabling a new target and syncing over a hand-written file of
  the same name now preserves that file in `.awf-bak` and reports it, instead of destroying it.
- The `ACTIVE.md` / domain-index takeover becomes visible: the adopter is told, once, that awf has
  assumed ownership and should retire any competing generator — turning a silent fight into an
  actionable one-time message. awf cannot itself remove the adopter's external generator; surfacing
  the takeover is the boundary of what it can do deterministically.
- `init` and `sync` now enforce one consistent rule from one mechanism (`BackupFile`), so "awf never
  silently overwrites a foreign file" holds across every write path, not just adoption.
- Cost: a foreign file byte-identical to awf's output is still backed up, producing a `.awf-bak`
  that conveys no new content. Accepted for consistency with ADR-0023 and to keep the rule a simple
  lock-membership test; the volume is negligible because foreign collisions arise only at adoption
  or when newly enabling a target.
- The backup must be keyed on the *prior* lock, read before any write in the sync; an implementation
  that consulted the freshly-written lock would treat every awf file as already-owned and never back
  anything up. The plan must thread the pre-sync manifest into the write step.

Doc-currency obligations the implementing commit(s) must satisfy:

- The `tooling` domain narrative gains the sync-backup-and-ownership behaviour, alongside ADR-0023's
  `init`-backup wording (same `related` lineage).
- The new `inv: sync-backs-up-foreign` slug is backed by a `// invariant: sync-backs-up-foreign`
  comment on the guarded write path in the commit that flips this ADR to `Implemented` (ADR-0008).
- No `docs/decisions/README.md` index row is owed (the README is a how-to guide; `ACTIVE.md` is the
  generated index — ADR-0005), matching ADR-0023.
- The status flip to `Implemented` regenerates `docs/decisions/ACTIVE.md` and the `tooling` domain
  index via `./x sync`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Refuse + abort sync on a foreign collision (require `--force`) | Interrupts routine syncs and is heavier than warranted; a non-destructive backup achieves safety without halting the workflow. |
| Warn but still overwrite without backup | The hand-written content is still lost — only the warning differs from today's silent loss. |
| Back up only when content differs from awf's output | Diverges from ADR-0023's `init` behaviour, splitting one principle into two subtly different rules; the byte-identical case is rare and harmless. |
| Bespoke "externally generated" detection for `ACTIVE.md` | No reliable content signal exists; the general foreign-file rule already preserves it, and a path-based ownership note is deterministic where a heuristic would be fragile. |
