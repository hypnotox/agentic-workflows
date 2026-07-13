---
status: Implemented
date: 2026-07-08
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [config, tooling, security, testing]
related: [10, 16, 23, 39]
domains: [config, tooling]
---
# ADR-0076: Lock integrity and corrupt-lock failure policy

## Context

`.awf/awf.lock` is the single point of trust for every awf decision that distinguishes
awf-managed files from foreign ones and old schemas from current ones. The 2026-07-08 deep-dive
audit traced one root cause into a chain of verified consequences:

- **Non-atomic writes.** `manifest.Lock.Save` is a truncate-in-place `os.WriteFile`, written
  once at the end of sync. A crash mid-save leaves a truncated lock; a merge conflict in the
  committed lock produces the same "present but unreadable" state. Two migrations rewrite the
  user-authored `.awf/config.yaml` the same way (`configedit.go`, `singletonstandarddocs.go`).
- **Silent fallbacks in every reader.** A present-but-unreadable lock is today indistinguishable
  from a missing one at each call site, and every site guesses differently:
  `migrate.Generation` returns `Current()` (so `awf upgrade` reports "already current" on a
  genuinely old tree and the ADR-0039 schema gate is bypassed) and its legacy branch returns
  `1` on a corrupt pre-relocation lock; `cmd/awf`'s `lockVsBinary` skips the version sub-check
  (ADR-0039 Decision 5 mandates this today — this ADR flips that clause); `project.SyncReport`
  drops the error (`old, _ :=`) and treats every rendered file as foreign — a reproduced
  backup storm (one spurious `.awf-bak` per lock entry) with pruning silently skipped;
  `Project.Audit` proceeds with an empty generated-paths set; `CollisionsAt` — reached from
  **ungated** `awf init` — reports every rendered path as a collision; `Check` and `Uninstall`
  misreport the state as "no lock", and `Check`'s message points the user at `awf sync`,
  i.e. at the backup storm.
- **Adjacent UX gaps in the same failure family.** `awf upgrade` prints "already current" when
  the binary is *behind* the tree's schema and when run outside any project; a missing
  `.awf/config.yaml` surfaces as a raw ENOENT with no `awf init` hint.

Two prior texts pin parts of the current behavior: ADR-0039 Decision 5 ("absent **or
unparseable** lock → version sub-check skipped, not an error") and ADR-0016 Decision 6
(`Generation` "keyed on directory presence, not on a readable lock"). The
uninstall-then-upgrade ambiguity — a **missing** lock beside a present tree reads as current —
was examined 2026-07-07 and deliberately kept; nothing here reopens it.

The user constraint for the implementation, verbatim: "I want to do this TDD though, so test
should exist and fail first before the fix comes in."

## Decision

1. **Atomic writes for trust-bearing files.** A shared helper (`manifest.WriteFileAtomic`:
   temp file created in the destination directory under a distinctive name, chmod `0o644`
   before `os.Rename`, best-effort temp removal on error) is used by `Lock.Save` and by every
   migration that rewrites an *existing* config file (`configedit.go`,
   `singletonstandarddocs.go`). Fresh-file writes during the 0→1 tree port stay plain — the
   legacy source survives until the port completes, so a torn fresh write loses nothing.
   Durability bar: rename-only, no fsync — the recovery path below makes a power-loss torn
   write survivable, and process-crash atomicity is what the chain requires. Windows
   assumption stated explicitly: Go's `os.Rename` replaces an existing destination; CI does
   not exercise it (ubuntu-only).

2. **A present-but-unreadable lock is a hard error in every reader.** New choke point
   `manifest.LoadOptional(path) (*Lock, bool, error)`: missing file → `(nil, false, nil)` —
   a found flag rather than a bare nil pair, since the repo's `nilnil` linter forbids
   returning a nil pointer beside a nil error; present but unreadable or unparseable → an
   error carrying the one-place recovery hint (restore `.awf/awf.lock` from version
   control, or delete it deliberately to re-adopt). `manifest.Load` keeps its
   signature for hard-require callers, which split missing vs corrupt via
   `errors.Is(err, os.ErrNotExist)` (the `read lock: %w` wrap preserves it; the parse branch
   never carries ENOENT). The full reader inventory converts:
   `migrate.Generation` (both the relocated and the legacy-lock branches — `Generation` gains
   an error return, rippling through `GateState`, `Upgrade`, `gate()`, and `runUpgrade`),
   `cmd/awf` `lockVsBinary`, `project.SyncReport` (refuses **before any file write**, so a
   corrupt lock can never produce a backup or skip pruning), `Project.Audit`, `CollisionsAt`,
   `Project.Check`, and `project.Uninstall` (which also wraps the underlying error it drops
   today). A missing lock keeps each caller's current semantics. The ninth production
   `manifest.Load` site, `stampLockSchema`, deliberately stays on bare `manifest.Load`: once
   `Generation` converts, a corrupt lock fails `Upgrade` upfront and can no longer reach the
   stamp — which is exactly the corrected justification its coverage-ignore receives in
   Decision 6.

3. **Partial supersedence.** This ADR supersedes ADR-0039 Decision item 5's *unparseable*
   clause only. The full surviving skip set: an absent lock, an absent or empty `awfVersion`
   field, and an `awfVersion` that fails semver normalization all still skip the version
   sub-check; only a present-but-unparseable lock flips to the Decision-2 hard error.
   ADR-0039's textual-contract invariant ("the version sub-check never errors") is
   unmodified — the new failure fires upstream at the lock load, before the sub-check runs.
   ADR-0039 stays Implemented. It likewise narrows ADR-0016 Decision item 6: `Generation` remains keyed on
   directory presence for *which era* a tree belongs to, but a present-and-unreadable lock in
   the detected era is an error rather than a sentinel generation. The
   `docs/pitfalls.md` section documenting `Generation`'s sentinel semantics updates in the
   same change.

4. **Truthful failure messages, one recovery hint.** `Check`: missing lock keeps
   "no lock (run `awf sync`)"; corrupt lock reports the Decision-2 hint instead of pointing
   at `awf sync`. `Uninstall`: missing → "nothing to uninstall"; corrupt → refuses (it cannot
   know what to delete) with the hint. `awf upgrade`: binary behind the tree's schema →
   the version-gate's "update your pinned awf" guidance instead of "already current"; run
   outside any project → the no-project message. Outside-project detection comes from a new
   small `migrate` presence export — `Generation` cannot express it (it returns `Current()`
   for "nothing present", and that stays).

5. **No-project hint at the config-load boundary.** Any project-requiring command that finds
   no `.awf/config.yaml` reports "not an awf project (run `awf init`)" alongside the
   underlying error; `awf init` itself is exempt. This rides here deliberately: it is the
   same principle as Decision 4 — a truthful failure with one named recovery action at the
   config/lock trust boundary — applied to the last silent state the same audit surfaced.

6. **Failure-path e2e tests, in-process.** The failure matrix lands as scenario tests in
   `cmd/awf` driving the package's `run(args, stdout, stderr)` seam against real scaffolded
   temp trees: corrupt-lock variants (truncated JSON, garbage bytes, merge-conflict markers)
   × commands (`sync`, `check`, `upgrade`, `uninstall`, `init`), asserting exit code, message
   content, zero `.awf-bak` files created, and the corrupt lock left untouched. Because the
   version gate runs before `project.Open`, the CLI-level assertions for gated commands
   target the gate's error; the `SyncReport`/`Check`/`Audit` conversions are defense-in-depth
   for the exported API and are asserted at package level. No built-binary tier — in-process
   tests count toward the coverage gate and need no new infrastructure. Per the TDD
   constraint, each behavioral conversion lands with its failing test written first; the plan
   pre-sorts error branches into fault-injectable (red test required) versus genuinely
   untriggerable OS faults (`// coverage-ignore` with an ADR-0012-compliant reason) so no
   test is written against an untriggerable fault. The factually wrong coverage-ignore at
   `migrate.go`'s post-`Upgrade` lock load ("the lock just loaded cleanly" — false for a
   corrupt legacy lock carried by the relocation) is corrected as part of the conversion.

## Invariants

- `invariant: lock-atomic-save` — `.awf/awf.lock` and migration rewrites of existing config files
  are written via the temp-file-plus-rename helper; no direct `os.WriteFile` of the lock or
  of an existing `.awf/config.yaml` remains in `internal/manifest` or `internal/migrate`
  (fresh-file writes in the 0→1 tree port exempt per Decision 1).
- `invariant: corrupt-lock-refuses` — a present-but-unreadable `.awf/awf.lock` causes a hard error
  in every lock reader (gate, upgrade, sync, check, audit, uninstall, init collisions);
  in particular `SyncReport` refuses before writing any file, so a corrupt lock can never
  create a backup, skip a prune, or be overwritten.
- A missing lock (ENOENT) preserves each caller's pre-existing semantics (textual contract;
  the ADR-0039 absent-skip and the documented missing-lock-with-tree ambiguity both stand).

## Consequences

- **The worst repair scenario disappears.** Today a truncated lock silently degrades three
  independent subsystems (schema gate, backup/prune, audit) and the tool's own advice makes
  it worse. After this ADR the failure is a single loud, explained state with a two-option
  recovery, and the atomic save makes reaching that state require a power loss or a merge
  conflict rather than any interrupted process.
- **A behavior flip for a documented edge.** Scripts that relied on ADR-0039's
  unparseable-lock-skip (none known) now fail loudly; that is the point. The change is
  adopter-visible and lands in the changelog.
- **Signature ripple.** `migrate.Generation` gains an error return, touching `GateState`,
  `Upgrade`, `gate()`, and `runUpgrade`. Contained; no exported API outside the module
  changes meaning otherwise.
- **`awf init` gets stricter in a corrupt-lock tree** (refuses with the hint instead of
  listing every file as a collision) — strictly better diagnostics for the same refusal.
- **The e2e suite becomes the template for future failure-path coverage** — subsequent
  failure-family efforts (e.g. read-only trees) extend the same matrix rather than inventing
  a harness.
- **Doc currency at the flip.** The commit that flips this ADR to Implemented regenerates
  `docs/decisions/ACTIVE.md` via `./x sync`, adds the two invariant bullets (with ADR-0076
  citations) to the agent guide's invariants source under `.awf/` and re-renders AGENTS.md,
  refreshes the `docs/pitfalls.md` `Generation`-sentinel section (Decision 3), and records
  the adopter-visible behavior flip in the changelog.
- **Not addressed, deliberately:** auto-recovery from git (awf must not read VCS state a
  command didn't ask about); fsync durability; the missing-lock-with-tree ambiguity
  (standing decision); rendered-artifact write atomicity (re-sync heals, drift check
  detects).

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Warn and rebuild (sync proceeds as if no lock) | Keeps the backup storm, and a rebuilt lock stamps the current schema onto a possibly old tree — the gate bypass survives in a new form. |
| Auto-recover the lock from `git show HEAD:.awf/awf.lock` | awf silently reading VCS state a command didn't request; a stale committed lock would mask real drift. Rejected in brainstorm. |
| Tri-state `manifest.Load` signature (`lock, found, err`) | Forces churn on every healthy caller for a distinction only some need; `LoadOptional` beside `Load` puts the policy in one place with minimal ripple. |
| Per-caller `errors.Is` classification, no helper | Copy-pastes the recovery hint and lets the next caller silently regress to swallowing — the exact one-sided pattern this repo's review focus already flags. |
| Atomic writes for all rendered artifacts | Dozens of call sites churn for files re-sync already heals and the drift check already detects; no observable behavior change. |
| Built-binary e2e tier (`./x e2e`) | True process boundary, but a new gate surface whose coverage doesn't count toward the 100% gate; in-process `run()` tests exercise everything but `main()`'s exit plumbing, which is already invariant-pinned (`single-os-exit`). |
