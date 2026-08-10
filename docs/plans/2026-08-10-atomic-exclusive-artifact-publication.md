---
format: plan-v2
date: 2026-08-10
adrs: [atomic-exclusive-artifact-publication]
status: Proposed
---
# Plan: Atomic Exclusive Artifact Publication

## Goal

Make ADR, plan, and backup creation publish complete files without replacing a concurrent
winner, and serialize numbered ADR allocation across processes. Migration behavior, Git
boundary handling, and broader direct-filesystem conversion are non-goals.

## Architecture summary

A new standard-library-and-x/sys leaf package owns same-directory complete-file preparation
and the released platforms' atomic no-replace namespace operation. `internal/effort`,
`internal/adr`, `internal/plan`, and `internal/project` depend inward on that mechanism while
retaining their own identity, naming, retry, and presentation policy. ADR additionally owns a
canonical-directory-keyed `gofrs/flock` advisory lock held across corpus read through
publication. Three sequential subagent-driven phases keep the shared mechanism, ordinary
consumers, and cross-process ADR allocation independently green and reviewed.

## Phase 1: Establish the shared publication home

**Execution mode: subagent-driven.**

Advances: ["authority-current"]
Completes: ["shared-publication"]

### Task 1.1: Add failing complete-file and no-replace tests
Applying: ["atomic-exclusive-artifact-publication:publish-complete-or-refuse", "atomic-exclusive-artifact-publication:prove-concurrent-publication"]
Paths: ["glob:internal/filepublication/**", "internal/effort/platform_test.go", "internal/effort/durability_test.go"]
Post-check: Run the new package tests over the new package and the retained effort publication tests over `internal/effort`; before production extraction, record that the new complete-file/no-replace assertions fail because the shared package does not exist, then after Task 1.2 require both packages to pass with no skipped collision or mode case.

Create focused tests for complete same-directory preparation, destination-exists error identity,
unchanged existing bytes, temporary cleanup after ordinary failure, requested permission mode,
and two barrier-released publishers producing exactly one complete winner. Add a structural test
under `internal/filepublication` that detects a released-platform no-replace implementation outside
the package and pins the intended inward production dependency direction. Preserve the existing
effort durability and platform assertions as compatibility evidence rather than rewriting their
oracle.

### Task 1.2: Extract no-replace creation and migrate effort creation
Applying: ["atomic-exclusive-artifact-publication:single-exclusive-publication-home", "atomic-exclusive-artifact-publication:publish-complete-or-refuse"]
Paths: ["glob:internal/filepublication/**", "internal/effort/publication_linux.go", "internal/effort/publication_darwin.go", "internal/effort/publication_windows.go", "internal/effort/publication_other.go", "internal/effort/store.go", "internal/effort/activity.go", "internal/effort/platform_test.go", "internal/effort/platform_windows_test.go", "internal/effort/durability_test.go", ".awf/topics/parts/tooling/file-publication/current-state.md", ".awf/docs/parts/architecture/components.md", "docs/architecture.md", "docs/topics/tooling/file-publication.md", "docs/decisions/atomic-exclusive-artifact-publication.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Post-check: Confirm every released-platform no-replace creation implementation is declared only under `internal/filepublication`, effort's expected-absent publication callers use that package, effort replacement/removal remains under `internal/effort`, the added claim renders with `Origin: ADR-atomic-exclusive-artifact-publication` and a real proof marker, and `./x check` reports no drift. Read the rendered claim and architecture component together and confirm they state one publication home, retain effort-owned replacement/removal policy, and make no contradictory power-loss durability promise.

Add a one-sentence package ownership comment and the smallest exported concrete publication API
needed by its first production consumer. Move only the expected-absent creation half of effort's
platform files; retain effort-specific expected-identity replacement, rollback, and removal. Make
effort's expected-absent path call the shared package without changing its durable resident
ordering or error semantics. Apply `tooling/file-publication:exclusive-file-publication-single-home`
as the first pair-atomic ADR batch by transitioning the ADR to Implementing and adding the
matching test-backed claim source. Update the architecture source and render generated docs.

### Phase close

The shared package and effort migration land together so no unused composition surface or dead
production code exists between commits.

```commit
refactor(code-design): centralize exclusive file publication
```

## Phase 2: Convert plan and backup consumers

**Execution mode: subagent-driven.**

Advances: ["authority-current"]
Completes: ["consumer-no-clobber"]

### Task 2.1: Add failing consumer collision and mode tests
Applying: ["atomic-exclusive-artifact-publication:retain-consumer-policy", "atomic-exclusive-artifact-publication:prove-concurrent-publication"]
Paths: ["internal/plan/plan_test.go", "internal/plan/publication_test.go", "internal/project/install_test.go", "internal/project/runner_test.go"]
Post-check: Run the named plan-publication and backup-publication tests before Task 2.2 and record the current overwrite, suffix-retry, or error-propagation failure; after Task 2.2 rerun the same focused tests and require every collision, mode, and non-collision error case to pass.

Add deterministic tests that force a competing destination at the publication boundary rather
than relying on scheduler timing. Plan scaffolding must return its existing overwrite refusal and
preserve the winner's bytes. Concurrent backup creation must keep both complete rescue copies,
retry only an existence collision to the next suffix, preserve source permission bits, and return
any non-collision publication error without retrying. Keep the seam private and consumer-shaped;
do not add a mutable package global or provider-owned test interface.

### Task 2.2: Publish plans and backups through the shared mechanism
Applying: ["atomic-exclusive-artifact-publication:retain-consumer-policy", "atomic-exclusive-artifact-publication:publish-complete-or-refuse"]
Paths: ["internal/plan/plan.go", "internal/plan/plan_test.go", "internal/project/install.go", "internal/project/install_test.go", "internal/project/runner_test.go", ".awf/topics/parts/adr-system/plan-artifacts/current-state.md", ".awf/topics/parts/rendering/sync-and-drift/current-state.md", ".awf/topics/parts/rendering/companion-scripts/current-state.md", "docs/topics/adr-system/plan-artifacts.md", "docs/topics/rendering/sync-and-drift.md", "docs/topics/rendering/companion-scripts.md", "docs/decisions/atomic-exclusive-artifact-publication.md", ".awf/awf.lock"]
Post-check: Confirm `internal/plan` and `internal/project` contain no Stat-then-WriteFile publication sequence for the converted paths, backup suffix retry matches only destination existence, all three updated claims render with the ADR in `Revised-by`, and `./x check` reports no drift. Read the rendered plan and backup claims and confirm their prose preserves overwrite-refusal wording, suffix-only collision retry, source-mode preservation, and non-collision error propagation.

Replace plan's check-then-write with complete exclusive publication while retaining its computed
path and refusal prose. Replace backup free-name selection plus truncating copy with source read
and mode inspection followed by exclusive publication; retry suffixes only on the shared
matchable existence error. Apply one ADR batch containing updates to
`adr-system/plan-artifacts:plan-new-unnumbered`,
`rendering/sync-and-drift:sync-backs-up-foreign`, and
`rendering/companion-scripts:runner-prune-backup`, with their matching claim changes and rendered
outputs.

### Phase close

The ordinary consumers, regression tests, and their complete current-state batch close together.

```commit
fix(code-design): publish plans and backups exclusively
```

## Phase 3: Serialize ADR allocation and finish authority

**Execution mode: subagent-driven.**

Completes: ["adr-serialized", "authority-current"]

### Task 3.1: Add failing cross-process ADR allocation tests
Applying: ["atomic-exclusive-artifact-publication:serialize-adr-allocation", "atomic-exclusive-artifact-publication:prove-concurrent-publication"]
Paths: ["internal/adr/adr_test.go", "glob:internal/adr/*_test.go"]
Post-check: Run the named ADR concurrency proving units with subprocesses sharing one decisions directory; before the lock implementation record a duplicate-number or overwrite-contract failure, then require one valid corpus containing unique sequential numbers, unchanged winner bytes, and alias spellings that resolve to one lock identity.

Add deterministic subprocess tests for two numbered creators with different slugs competing for
one next number, two pending creators competing for one slug path, process release after a held
lock, and supported path aliases mapping to the same lock. Use explicit synchronization rather
than sleeps. Pin that each successful return leaves a parseable corpus and that failures never
clobber an existing record.

### Task 3.2: Lock the ADR corpus transaction and publish exclusively
Applying: ["atomic-exclusive-artifact-publication:serialize-adr-allocation", "atomic-exclusive-artifact-publication:publish-complete-or-refuse"]
Paths: ["internal/adr/adr.go", "glob:internal/adr/scaffold_lock*.go", "glob:internal/adr/*_test.go", "go.mod", "go.sum"]
Post-check: Confirm the advisory lock begins before `loadIdentityCorpus` and remains held through shared publication, the lock cache filename is SHA-256 of the OS-resolved canonical decisions-directory identity, lock files are unlocked but never deleted, `github.com/gofrs/flock` is a direct dependency, ADR scaffolding, plan scaffolding, standard backup creation, and effort expected-absent publication use `internal/filepublication`, and every released target cross-compiles through `./x gate`.

Introduce the smallest ADR-owned lock helper, including released-platform canonicalization needed
to collapse absolute, symlink, Windows case, and volume aliases. Store persistent lock files under
a stable per-user cache directory with restrictive directory and file permissions. Hold the lock
across identity load, slug and number selection, template preparation, and exclusive publication;
return canonicalization, cache, lock, preparation, and publication failures without falling back
to the old unsafe path. Keep number and slug policy in `internal/adr` and use the shared publisher
only for the completed target bytes.

### Task 3.3: Apply ADR claims, document behavior, and record the fix
Applying: ["atomic-exclusive-artifact-publication:serialize-adr-allocation", "atomic-exclusive-artifact-publication:prove-concurrent-publication"]
Paths: [".awf/topics/parts/adr-system/adr-lifecycle/current-state.md", "docs/topics/adr-system/adr-lifecycle.md", "docs/decisions/atomic-exclusive-artifact-publication.md", "changelog/CHANGELOG.md", ".awf/awf.lock"]
Post-check: Confirm the final Applied event names exactly the two remaining ADR operations, both claims render with the ADR in `Revised-by` and proof evidence covering cross-process competition, the Unreleased changelog describes the user-visible no-clobber correction, and `./x check` reports no drift. Read both rendered ADR claim bodies and the changelog together and reject any statement that changes ADR-0202's branch-dependent numbering, introduces number reservation, permits overwrite, or misstates advisory-lock lifetime.

Apply the final pair-atomic ADR batch updating
`adr-system/adr-lifecycle:adr-new-no-overwrite` and
`adr-system/adr-lifecycle:adr-new-sequential-numbering`. Strengthen their prose to name
complete-file no-replace publication and one canonical-directory transaction across concurrent
processes without changing ADR-0202's branch-dependent numbering semantics. Add the focused
Unreleased changelog entry and render all generated authority outputs. Leave the ADR and plan
nonterminal for deferred closure after implementation assurance.

### Phase close

The cross-process behavior, dependency promotion, remaining authority operations, and changelog
land as the final implementation transaction.

```commit
fix(adr-system): serialize ADR scaffold publication
```

## Definition of done

- `dod: shared-publication` One cross-platform package owns complete-file atomic no-replace creation, and effort creation uses it without changing replacement or removal policy.
- `dod: consumer-no-clobber` Plan scaffolding and every standard backup path preserve a concurrent winner, retain required modes, and expose non-collision failures.
- `dod: adr-serialized` Concurrent creators against one canonical decisions directory cannot overwrite a record or return duplicate numbered identities.
- `dod: authority-current` Every declared ADR operation is Applied with matching authored and rendered claims, architecture and changelog are current, and render/check/gate verification is green.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here.
Delegated owners may report rather than edit; the parent supplies the report to phase review and
reconciles it with findings in one focused post-review settlement commit before checkpointing or
later execution. Record deviations, spike answers, follow-ups, and findings surfaced during
implementation.

- Initial plan review: added omitted effort callers, Windows tests, INDEX generation, a structural
  single-home proof, deterministic Phase 2 failure evidence, an internal-package plan test seam,
  and focused semantic rendering checks. The internal plan test file is preferred over exporting a
  test-only capability or converting the established external suite. Baseline declarations were
  not added because execution workflow owns generic clean/green setup; plan closure remains
  deferred until post-assurance finalization; and parent-owned post-review settlement commits remain
  authorized by the subagent-driven workflow.
- Phase 1 review settlement: no implementation deviation was reported. Five mechanical findings
  were settled by widening the invariant proof across the production tree and behavioral units,
  restoring a package-private Windows creation oracle, exercising expected-identity effort fault
  stages, synchronizing the concurrency test after complete preparation at the namespace boundary,
  and using wrapped-error matching for cleanup. No approved boundary or outcome changed.
- Renewed Phase 1 review settlement: three follow-up findings were settled by attaching proof to
  the existing expected-identity removal and publication-order fault matrices, replacing raw-token
  structural detection with parsed call and import inspection plus positive and negative detector
  cases, and reducing the one-operation Windows test seam to direct function injection. The
  structural detector correction was reasoned from dependency-composition and test-design
  authority; it changed verification shape without changing production behavior or scope.
- Final Phase 1 assurance settlement: the remaining structural-proof finding was settled with
  import-identity-aware call inspection, recursive constant flag resolution, complete inward-only
  package checks, normalized nil conditions, and explicit mutations for zero, aliased, composed,
  unrelated-selector, and alternate-internal-import cases. The stricter proof remains confined to
  the approved single-home invariant and does not change runtime behavior.
- Phase 1 proof closure: three mechanical detector escapes were closed by checking raw import paths
  before alias filtering, honoring inherited constant expressions while conservatively allowing
  only known Unix replacement flags, and binding the Windows effort exception to the single move
  function in `nativeWindowsPublicationAPI`. Blank, dot, inherited-constant, and duplicate-wrapper
  mutations now pin those boundaries without changing production behavior.
- Phase 2 review settlement: no implementation deviation was reported. Review corrections bind
  backup bytes and mode to one opened source, add contextual wrapped failures, exercise sync and
  runner-prune backup refusals, restore displaced marker comments, prove the plan publication mode,
  and use each platform's observed source permissions in backup assertions. The open-handle change
  closes a source replacement race under the approved complete-backup outcome without changing
  naming, retry, or presentation policy.
- Renewed Phase 2 review settlement: wrapped path-error identity is now asserted through both sync
  error surfaces, cleanup close failures join rather than replace the primary source error, and the
  remaining shallow absence predicate uses `errors.Is`. The suggested early changelog entry was not
  applied because Phase 3 explicitly owns the one complete user-visible no-clobber correction after
  ADR serialization lands; Phase 2's authored and rendered current-state claims already travel with
  its nonterminal partial implementation.
- Phase 3 review settlement: the phase-owner response was truncated before its explicit deviation
  field. Review inferred that the initial subprocess proof widened a race interval rather than
  synchronizing at the publication boundary, process-exit and Windows-alias evidence was incomplete,
  and reachable fault paths were over-excluded. The settlement removes the timing amplifier and
  composes deterministic lock-span and publication-collision units with isolated-cache cross-process
  creator and process-death contention proofs. It also proves persistent SHA-256 lock identity and
  permissions, Windows final-path and case canonicalization, released-target corpus-source census,
  reachable canonicalization and cache failures, wrapped cleanup errors, and current claim markers.
  This changes proof shape and test confinement only; the approved runtime boundary and behavior are
  unchanged.
- Renewed Phase 3 review settlement: the canonical-process proof now requires a waiter's failed
  nonblocking acquisition before terminating the holder, then proves acquisition and persistent
  identity after descriptor release. The one no-overwrite marker composes that alias-keyed
  cross-process transaction with the forced no-replace winner assertion. Reachable canonicalization
  and missing-cache failures are directly covered without exclusions, and the Windows test binary
  executed its final-path, case, extended-path, contention, and publication assertions under Wine in
  addition to released-target compilation. A final mechanical correction exercises the deleted-CWD
  canonicalization failure directly and binds the no-overwrite marker to the production
  `acquireScaffoldLock` and `filepublication.Publish` composition with an AST wiring proof. The
  deleted-CWD unit is confined to the non-Windows implementation and asserts both contextual prose
  and wrapped not-exist identity. These corrections strengthen verification only.
- Terminal assurance settlement: the approved Windows volume-alias requirement already determines a
  GUID-volume final-path identity, so the implementation now requests that OS representation and a
  Windows unit creates a distinct drive alias for the same volume. The sequential-numbering marker
  now deterministically proves acquisition before corpus loading and lock retention through
  publication, while retaining cross-process behavior separately. The single-home detector now
  rejects hard-link publication functions that also prepare complete files without rejecting an
  unrelated hard-link operation. Shared publication directly proves a missing-parent error, and the
  changelog now covers plan and backup no-clobber behavior. These authority-preserving corrections
  change proof completeness and documentation only, except for the required Windows volume identity
  correction.
