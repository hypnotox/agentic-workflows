---
format: plan-v2
date: 2026-08-11
adrs: [root-confined-sync-filesystem-mutations]
status: Implemented
---
# Plan: Root-confined sync filesystem mutations

## Goal

Make every ordinary render and first-adoption sync filesystem mutation operate through the selected
tracked or resident root-confined handle, with safe final-symlink behavior and preserved sync policy.
Uninstall, upgrade migrations, snapshot capture, and unrelated direct filesystem effects remain out
of scope.

## Architecture summary

`internal/project` owns a private cohesive sync-filesystem contract and selects concrete
`internal/filesystem.Handle` values for the tracked checkout and any distinct primary resident root
before mutation. Existing resident-output policy chooses the handle while paths remain
slash-relative. The handle gains only confined chmod and one-open bytes-plus-mode observation;
project routes lock I/O, output creation and complete replacement, backups, pruning, and ancestor
cleanup through that contract. Managed final symlinks are replaced without following them; foreign
symlinks are backed up only when their target is readable inside the selected root and otherwise
refuse. Replacement applies bytes and final mode before namespace commit, so change reporting occurs
only after success.

## Phase 1: Accept the reviewed decision

**Execution mode: inline.**

Advances: ["root-confined-sync"]

### Task 1.1: Accept the review-settled ADR
Latitude: exact
Paths: ["docs/decisions/root-confined-sync-filesystem-mutations.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]

Use `awf-adr-lifecycle` to transition the unchanged Proposed record to Accepted after its settled ADR
review. Regenerate the index and lock without applying any State changes operation. Confirm the
record's accepted content digest covers the reviewed decision and that every operation remains
unapplied.

### Phase close

Accept the reviewed root-confined sync decision as the authority for production work.

```commit
docs(adr): accept root-confined sync mutations
```

## Phase 2: Establish the confined sync dependency and publication path

**Execution mode: subagent-driven.**

Advances: ["root-confined-sync"]

### Task 2.1: Pin confined handle and sync publication behavior
Applying: ["root-confined-sync-filesystem-mutations:consumer-owned-sync-contract", "root-confined-sync-filesystem-mutations:final-symlink-policy", "root-confined-sync-filesystem-mutations:complete-output-replacement", "root-confined-sync-filesystem-mutations:preserve-sync-policy"]
Paths: ["internal/filesystem/handle_test.go", "internal/project/project_test.go", "internal/project/install_test.go", "internal/project/runner_test.go"]

Starting dependency: Phase 1 has accepted ADR-root-confined-sync-filesystem-mutations without
applying its claim operations.

Add failing focused tests before production edits. Prove confined chmod and one-open bytes-plus-mode
observation preserve mode and wrapped error identity; complete replacement replaces a final symlink
rather than its target and commits neither bytes nor mode on failure. At the project boundary prove a
managed final symlink is replaced without target access, a foreign in-root symlink preserves backup
bytes and source mode, and a foreign escaping or broken symlink refuses before backup or replacement.
Preserve backup suffix retry, concurrent complete publication, runner-prune rescue behavior, and the
existing corrupt-lock refusal.

### Task 2.2: Compose the two-root sync filesystem contract
Latitude: exact
Applying: ["root-confined-sync-filesystem-mutations:complete-sync-mutation-boundary", "root-confined-sync-filesystem-mutations:consumer-owned-sync-contract", "root-confined-sync-filesystem-mutations:final-symlink-policy", "root-confined-sync-filesystem-mutations:complete-output-replacement", "root-confined-sync-filesystem-mutations:preserve-sync-policy", "root-confined-sync-filesystem-mutations:bounded-conversion"]
Paths: ["internal/filesystem/handle.go", "internal/project/project.go", "internal/project/install.go"]

Add `Handle.Chmod(path string, mode fs.FileMode) error` and
`Handle.ReadWithMode(path string) (contents []byte, mode fs.FileMode, err error)` as the only new
provider capabilities exercised by Task 2.1. `ReadWithMode` opens once beneath the selected root and
returns neutral bytes plus permission mode from that same file handle. Replace `syncOperation` with
a private `syncFilesystem` contract containing exactly `MkdirAll`, `Chmod`, `Publish`, `Replace`,
`Remove`, `Read`, `ReadWithMode`, and `LinkInfo` in their concrete-handle signatures, and compose all
required tracked and resident handles before mutation. Give the operation-owned routing value one selector that returns
the correct handle and unchanged lock-relative path for each output. Read the lock through the
tracked handle and save its marshaled bytes through confined complete replacement. Route output
entry observation, parent creation, resident marker directory chmod, content comparison, and
complete output replacement through the selected handle. Refactor the backup helper to observe bytes
and mode from one confined open and publish through the same selected handle while retaining project
naming, collision retry, reporting, and error categories. Deliberately ignore handle close errors only
after the operation's own results are settled. Do not convert pruning in this phase and do not add a
provider-owned interface, project policy to `internal/filesystem`, or a second root implementation.

### Task 2.3: Reconcile replacement failure reporting
Applying: ["root-confined-sync-filesystem-mutations:complete-output-replacement", "root-confined-sync-filesystem-mutations:preserve-sync-policy"]
Paths: ["internal/project/project_test.go", "internal/project/coverage_test.go"]

Update the existing injected write/chmod failure coverage to the cohesive replacement contract.
Assert that a failed replacement produces no change record and preserves the old output, while a
successful content or mode correction produces exactly one correctly attributed change after commit.
Retain later-output failure and error-identity coverage without rebuilding a function-field dependency
bag.

### Phase close

Land one green structural transaction in which every new handle capability has its concrete sync
consumer and the publication, backup, and lock paths are confined; ADR claim operations remain
unapplied until pruning and durable claims complete the boundary.

```commit
refactor(code-design): compose root-confined sync publication
```

## Phase 3: Complete mutation confinement and apply current authority

**Execution mode: subagent-driven.**

Completes: ["root-confined-sync"]

### Task 3.1: Prove the complete tracked and resident mutation boundary
Latitude: exact
Applying: ["root-confined-sync-filesystem-mutations:complete-sync-mutation-boundary", "root-confined-sync-filesystem-mutations:preserve-sync-policy"]
Paths: ["internal/project/project_test.go", "internal/project/runner_test.go"]

Starting dependency: Phase 2 routes sync publication, backup, and lock I/O through the selected
handles while the accepted ADR operations remain unapplied.

Add the invariant proof `TestSyncMutationsStayWithinSelectedRoots` with the
`rendering/sync-and-drift:sync-mutations-root-confined` marker. Give the new foreign-symlink proof the
`rendering/sync-and-drift:sync-backs-up-foreign` marker and the confined lock proof the
`config/migrations-and-locks:lock-atomic-save` marker so each revised invariant proves its expanded
semantics. Exercise a tracked output and a resident marker when their roots differ, verifying ordinary
backup bytes and modes, replacement, lock-relative reporting, and final output placement at the
correct anchor. Exercise escaping parent symlinks for output replacement, backup publication, prune
removal, empty-ancestor cleanup, and `.awf/awf.lock` load/save; each case must prove the outside bytes
and modes remain unchanged, the operation refuses when required, and an old lock is not advanced
after an incomplete mutation. Cover managed final symlink pruning as removal of the entry itself.

### Task 3.2: Route prune removal and ancestor cleanup through selected handles
Applying: ["root-confined-sync-filesystem-mutations:complete-sync-mutation-boundary", "root-confined-sync-filesystem-mutations:preserve-sync-policy"]
Paths: ["internal/project/project.go", "internal/project/install.go"]

Replace absolute-path generated-file removal and empty-directory cleanup inside sync with
root-relative operations on the same tracked-or-resident handle selector used for publication.
Preserve actual-removal reporting, absent-file no-op behavior, resident preservation, runner backup
before removal, and deepest-first cleanup. Keep `internal/resident` uninstall behavior unchanged and
do not generalize this sync-owned policy into the filesystem provider.

### Task 3.3: Apply the root-confined sync claims
Latitude: exact
Applying: ["root-confined-sync-filesystem-mutations:complete-sync-mutation-boundary", "root-confined-sync-filesystem-mutations:final-symlink-policy", "root-confined-sync-filesystem-mutations:complete-output-replacement", "root-confined-sync-filesystem-mutations:preserve-sync-policy"]
Paths: [".awf/topics/parts/rendering/sync-and-drift/current-state.md", ".awf/topics/parts/config/migrations-and-locks/current-state.md", "docs/topics/rendering/sync-and-drift.md", "docs/topics/config/migrations-and-locks.md", "docs/decisions/root-confined-sync-filesystem-mutations.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]

Author and apply one pair-atomic claim batch: add
`rendering/sync-and-drift:sync-mutations-root-confined` with `Backing: test` and the Task 3.1 proof;
update `rendering/sync-and-drift:sync-backs-up-foreign` with selected-root routing, one-open confined
source observation, and safe final-symlink refusal; update
`config/migrations-and-locks:lock-atomic-save` so ordinary sync lock load and complete replacement are
root-confined while migration rewrites retain their existing atomic helper. Transition the ADR to
Implementing and append one Applied event naming exactly those three operations. Render generated
docs and inspect the complete changed claim blocks for the intended meaning, contradictory legacy
fragments, and correct pending provenance.

### Phase close

Land the complete confined mutation boundary and its pair-atomic current-state claim batch, leaving
the ADR Implementing for deferred terminal closure after implementation assurance.

```commit
feat(rendering): confine sync mutations (applies ADR batch)
```

## Definition of done

- `dod: root-confined-sync` Ordinary and first-adoption sync open the required tracked and resident
  root handles before mutation; every output, backup, prune, ancestor-cleanup, and lock mutation uses
  the selected handle and a slash-relative path; escaping and broken symlinks cannot change outside
  bytes or modes; established backup, reporting, pruning, and lock-authority semantics remain proven;
  all three ADR State changes operations are Applied with current generated documentation and the
  full project gate passes.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated
owners may report rather than edit; the parent supplies the report to phase review and reconciles it
with findings in one focused post-review settlement commit before checkpointing or later execution.
Record deviations, spike answers, follow-ups, and findings surfaced during implementation.

- Plan review settled the provider shape as `Handle.Chmod` plus one-open `Handle.ReadWithMode`
  returning neutral bytes and permission mode, and fixed the private sync contract to the exact
  concrete method set. This keeps capability in the provider, sync policy in the consumer, and no
  representation beyond what the approved backup behavior requires.
- Phase 2 review confirmed the replacement-failure tests already lived in `project_test.go`; no
  `coverage_test.go` mutation was needed. Retaining the tests in their existing semantic home
  preserves the approved failure oracle without moving coverage only to satisfy a planned path.
- Phase 2 also updated `internal/project/drift_test.go`, which was omitted from the planned paths,
  because it referenced the removed direct-backup helper and needed to follow the confined backup
  seam.
- Phase 3 also updated `internal/resident/resident_test.go`, which was omitted from the planned
  paths, to keep the unchanged uninstall-owned absent-removal helper covered after sync stopped
  calling it.
