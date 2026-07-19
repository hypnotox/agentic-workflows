---
date: 2026-07-19
adrs: [133, 134, 135, 136]
status: Implemented
---
# Plan: Bridge Migration Tooling

## Goal

Complete the releasable bridge half of ADR-0136: inventory every legacy invariant, normalize
retirement bookkeeping and prepared configuration, report readiness without mutation, attest a clean
prepared tree through a recoverable transaction, and refuse ordinary commands after attestation. This
plan does not switch context or invariant authority, implement the new ADR lifecycle or state-impact
checks, create INDEX.md, delete legacy consumers, or cut over awf and Sundial.

## Architecture summary

Create `internal/bridge` above `internal/migrate` and `internal/project`: migration continues to own
ordered schema generations, while the bridge orchestrator consumes the legacy ADR corpus, Plan 1's
topic corpus, a migration-safe project output projection, Git snapshot identity, and manifest
attestation fields. One immutable readiness report feeds `upgrade --check` and attestation. Attestation
precomputes normalization and output operations, journals every prior byte and mode, replaces the lock
last, and leaves either the original tree or a recoverable journal. A central command-state guard
refuses ordinary project commands when an attestation or journal exists.

Plans 1 and 2 are one unreleased tranche. Plan 1 must introduce the release-completeness sentinel as
false during final plan resync; this plan flips it only after every bridge behavior and test is present.
No release may be cut from an intermediate commit. All paths below are rooted at
`/home/hypno/Projects/agentic-workflows`; symbol-and-branch contracts are exhaustive, and tightly
coupled package files may share a task under the review-approved program exception.

## File structure

- **Created:** `internal/bridge/{inventory,history,approvals,normalize,readiness,snapshot,digest,journal,bridge}.go`
  with exact matching `_test.go` files, including `internal/bridge/approvals_test.go` for the strict
  migration approval schema and `internal/bridge/snapshot_test.go` for the cross-schema adapter;
  `cmd/awf/upgrade_test.go`.
- **Modified:** `internal/adr/{adr,status,declarations}_test.go` and production siblings;
  `internal/config/{config,edit,edit_test}.go`; `internal/project/{project,output_plan,output_plan_test,
  sweep,sweep_test,example_wiring_test}.go`; `internal/manifest/{manifest,manifest_test}.go`;
  `internal/git/{git,git_test}.go`; `internal/testsupport/gitfixture/gitfixture.go`;
  `internal/clispec/{clispec,clispec_test}.go`; `cmd/awf/{main,main_test,dispatch,upgrade,gate_test,
  failure_paths_test}.go`; `cmd/releasecheck/{main,main_test}.go`; `.github/workflows/release.yml`;
  `templates/{docs/working-with-awf.md.tmpl,bootstrap/awf-upgrade.sh.tmpl,hooks/pre-commit.sh.tmpl}`;
  `.awf/parts/workflow/{composing-the-gate,local-hooks}.md`; modify Plan 1's
  `.awf/parts/agents-doc/commands.md` override;
  `.awf/docs/parts/{architecture/components,architecture/data-flow,releasing/content}.md`;
  `.awf/domains/parts/{adr-system,config,invariants,rendering,tooling}/current-state.md`; `README.md`;
  `changelog/CHANGELOG.md`; and Plan 1's release-sentinel task during final resync.
- **Generated:** root and Sundial locks plus rendered architecture, workflow, release (including
  `docs/releasing.md`), domain, working-with-awf, bootstrap, hook, and agent-guide outputs selected by
  `./x sync`; release-guidance source changes and `docs/releasing.md` land in the same commit.
- **Deleted:** none. The committed Sundial fixture remains unattested and legacy-authoritative.

## Coupled Phases 1-2: Inventory, normalization, and the first production caller

Phases 1 and 2 share one closing commit. The bridge inventory/history/normalization functions have no
truthful main-reachable caller until `upgrade --check`; a Phase 1 commit would fail the dead-code gate.
Do not add a temporary caller. Tasks 1.1-1.5 remain review checkpoints, then Phase 2 closes and gates
the coupled group.

## Phase 1: Inventory and normalize enumerable legacy obligations

- [ ] **Task 1.1: Expose the exact legacy ADR facts.** In `internal/adr/adr.go`, parse and retain the
  frontmatter date without changing current validation. In `internal/adr/status.go`, add
  `IsLegacyShipped`, true only for Implemented and Superseded. Keep `InvariantDecls` as the declaration
  class source. Extend ADR tests for valid/missing dates, both shipped statuses, inactive statuses,
  and backed/unbacked preservation. Run `go test ./internal/adr`; expected: `ok`.

- [ ] **Task 1.2: Build the closed invariant inventory.** Create `internal/bridge/inventory.go` with
  `InvariantKey`, `LegacyInvariant`, `Inventory`, and `BuildInventory`. Enumerate declarations from
  legacy-shipped ADRs; reject duplicate `ADR-NNNN#slug` anchors; subtract only effective legacy
  retirement tokens carried by legacy-shipped ADRs; retain declarer, slug, backing class, carrier,
  carrier Decision item, and active/retired state. After Migration history parsing, adjudicate the
  inventory exactly once: an encoded entry validates but does not independently retire beyond its
  matching effective token; a valid `basis: migration` entry retires its exact declared key; only
  keys remaining live enter claim mapping. Reject a migration-history key that was never declared and
  reject any topic mapping or approval for a retired key. Do not call the narrower runtime
  `invariants.DeclaringADRs` and do not infer prose or topics. Create `internal/bridge/approvals.go` to
  parse authored `.awf/current-state-migration.yaml` when present with exactly top-level `version: 1` and
  `invariantApprovals`; require each sequence entry to contain exactly nonempty string scalars
  `key: ADR-NNNN#slug` and `destination: domain/topic:slug`. Reject unknown/duplicate fields,
  non-string or empty scalars, malformed identities, and duplicate keys without consulting topics.
  The schema has no `approved`, reviewer, timestamp, or signature field.

- [ ] **Task 1.3: Parse and plan append-only Migration history.** Create
  `internal/bridge/history.go`. Parse one optional `## Migration history` section outside fences. Accept
  only ``- YYYY-MM-DD: retired invariant `ADR-NNNN#slug`; basis: encoded`` and the migration-basis form
  ending `; rationale: <nonempty text>`. Reject unknown bases, duplicates, malformed dates, an encoded
  entry without an effective token, and a migration entry without rationale. Create
  `internal/bridge/normalize.go` raw-byte edits that append missing encoded entries using the carrier
  ADR date, preserve all other bytes, and run before Superseded-to-Implemented frontmatter rewrites.
  Repeated planning must return byte-identical no-op output.

- [ ] **Task 1.4: Add inventory/history/approval fixtures.** Create focused bridge tests for
  Implemented and Superseded declarers/carriers, inactive Proposed/Accepted tokens, lapsed tokens,
  encoded/migration entries, duplicates, unknown bases, missing rationale, fenced examples, byte
  preservation, and idempotence. Test parser acceptance of an absent pre-preparation file and valid empty/populated files, then make readiness require the file and exact `invariantApprovals: []` when no live mapping exists. Cover every strict-schema
  rejection: wrong version, missing/extra/duplicate top-level or entry fields, wrong node kinds,
  non-string and empty scalars, malformed keys/destinations, and duplicate keys. Keep generation-10
  retirement-token tests unchanged. Run `go test ./internal/adr ./internal/bridge`; expected: both
  packages report `ok`.

- [ ] **Task 1.5: Checkpoint the coupled implementation without committing.** Update the invariants
  and ADR-system authored current-state sources and Unreleased changelog, explicitly calling this
  migration-only inventory rather than runtime authority. Run
  `go test ./internal/adr ./internal/bridge`; expected: both packages report `ok`. Do not run the
  whole-program dead-code gate or commit until Phase 2 installs `upgrade --check` as the first real
  caller.

## Phase 2: Assemble a read-only readiness report

- [ ] **Task 2.1: Add comment-preserving prepared-config conversion.** In
  `internal/config/edit.go`, add `ConvertInvariantsToCurrentState`: copy enabled legacy source globs,
  marker, optional close, and testGlobs; set coverage error, fan-out warn, and maximum 8; remove the
  old key; preserve unrelated YAML comments/order. Refuse `invariants.disabled: true` and conflicts
  with a nonidentical authored currentState block. Parse and validate the proposed bytes through strict
  config. Add round-trip/idempotence/refusal tests.

- [ ] **Task 2.2: Derive mappings, apply approval evidence, and plan qualified marker rewrites.**
  Extend `internal/bridge/normalize.go` to scan only legacy configured sources and independently map
  each live inventory key to exactly one Plan 1 topic invariant with matching legacy Origin and
  exactly preserved backing class before consulting approval data. An approval cannot select among or disambiguate
  candidates. There is no reviewed-exception path: a genuine class change requires valid reviewed migration retirement rationale and a distinct current-state claim that is not mapped as the same obligation. Require the approval file at readiness and exactly one entry whose key and destination
  match each derived live mapping; reject missing, duplicate, unknown, retired-key, malformed, and
  destination-mismatch approvals. Retired records forbid entries and compute approval only from valid
  encoded history evidence or a valid migration rationale. Then rewrite proof markers to qualified
  IDs and rewrite `touches-invariant:` to `touches-state:` while preserving a required note. Reject
  missing/duplicate mappings, changed class, note-less touches, and every remaining unqualified marker
  in the scan universe; ignore historical ADR prose and nested projects. Validate proposed bytes with
  Plan 1's marker parser.

- [ ] **Task 2.3: Expose the deterministic prepared and terminal projections.** In
  `internal/project/output_plan.go`, add a read-only bridge projection containing sorted path, bytes,
  desired mode, policy, dependency hashes, and planned deletion/reservation facts. Its prepared view
  matches ordinary Plan 1 rendering. Its migration-safe terminal view is identical except that it
  schedules `docs/decisions/ACTIVE.md` and every generated domain ADR-index output for deletion and
  refuses any replacement at those paths. It does not generate INDEX.md, change domain prose, or
  switch authority. The bridge closed-tree sweep claims optional `.awf/current-state-migration.yaml`
  when present as authored migration input, never as generated output or permanent configuration.
  Both projections retain it unchanged, and planned mutations omit it unless another independently
  planned normalization changes it; digest membership alone is not a mutation. Attestation journals
  and applies the legacy-output deletions, after which command refusal makes the index-less locked
  state non-operational until Plan 3's current-state release generates INDEX.md. Prove both views
  byte/mode/deletion exact, approval-file retention, and no spurious approval-file mutation. Reserve
  the fixed bridge journal path in `sweep.go`; an existing journal is transaction state, not an orphan.

- [ ] **Task 2.4: Build the one readiness report and bridge snapshot adapter.** Create
  `internal/bridge/readiness.go` with stable `Finding {Code, Path, Detail}` and `Report`. Findings sort
  by Code, then slash-relative Path, then Detail; report every independent failure. Use these literal
  codes and canonical paths: `config-conversion` and `coverage-severity` at `.awf/config.yaml`;
  `domain-key` at the offending domain sidecar; `inflight-adr`, `migration-history`, and
  `invariant-inventory` at the ADR path; `claim-mapping` at the claim part or declaring ADR when
  absent; `invariant-approval` at `.awf/current-state-migration.yaml` for every missing, duplicate,
  unknown, retired-key, malformed, or destination-mismatch approval; `marker-mapping` at the source
  site; `topic-coverage` at the uncovered repository path;
  `topic-corpus` at the metadata/part input; `output-plan` at the colliding output; and
  `legacy-output` at each ACTIVE/domain-index path.

  Over proposed in-memory bytes, independently require strict config conversion;
  `currentState.topicCoverage: error` (warn/off fail); canonical domain keys; no Proposed/Accepted ADR;
  planned Superseded normalization; valid migration history; exact live-invariant mapping with strict class preservation and
  retired keys unmapped; a required approval file, including an empty list for zero live mappings; independently derived mappings followed by exact approval matching; retired
  records approved only by valid encoded history or migration rationale and absent from the allowlist;
  qualified markers; repository-wide domain-owned scoped topic coverage with
  empty/global topics excluded; topic parse/references/backing/render completeness; collision-free
  terminal output planning; and terminal deletion of every legacy generated index.

  Create `internal/bridge/snapshot.go` as the migration-only cross-schema adapter. It reads legacy HEAD
  only for ADR identity/status, invariant declarations/effective retirements, legacy proof/touches
  markers, and cutoff baseline; it reads the prepared tree entirely through the new config/topic
  engine. It validates every final old-HEAD/prepared-tree inventory, mapping, marker, and cutoff fact
  before readiness can seal them. It never assembles legacy context or supplies Plan 3's permanent
  staged/range checker.

- [ ] **Task 2.5: Add `awf upgrade --check` with deterministic JSON.** Add mutually exclusive upgrade
  flags to the one clispec table and pass parsed flags through dispatch. Plain upgrade retains ordered
  migration plus sync; `--check` is read-only. Add independent `--json` presentation with sorted schema
  `{ready,findings:[{code,path,detail}],invariantAdjudications:[{key,disposition,destination,origin,
  backing,approved}],plannedMutations:[{path,beforePresent,beforeMode,beforeSHA256,afterPresent,
  afterMode,afterSHA256}]}`. Every inventory key appears once; `approved` is computed, never parsed
  from authored YAML. Retired records forbid destination and approval entries and become approved only
  through valid encoded history or valid migration rationale; live records require qualified
  destination, declaring Origin, `test|unbacked` backing, and exactly matching approval evidence.
  Planned mutations exactly equal terminal journal operations including legacy deletions and exclude
  the unchanged approval file; absent images use present false, mode 0, SHA-256 of empty bytes. Test
  human/JSON parity, sorting, no mutation, adjudication completeness, computed approval, and exact
  journal-plan equality.

- [ ] **Task 2.6: Test every readiness predicate and close the coupled commit.** Use one valid fixture,
  then fail each
  predicate independently and assert its stable code/path. Cover tracked and nonignored untracked
  eligible files, generated/ignored/deleted/nested/contextIgnore exclusions, multi-domain gaps, empty
  topics, globals not satisfying scoped coverage, warn/off severity refusal, migration-retired keys
  omitted from mapping, mapped retired-key refusal, every legacy-HEAD/prepared-tree mismatch, and
  every terminal legacy-output deletion. Cover independent mapping before approval; every approval
  parser and semantic refusal with stable `invariant-approval` path/code; retired approval computation;
  bridge sweep claiming; unchanged-file retention; and exact human/JSON `approved` parity. Run
  `go test ./internal/config ./internal/bridge ./internal/project ./internal/clispec ./cmd/awf`;
  expected: all packages report `ok`. Update config, rendering, invariants, tooling, architecture,
  AGENTS commands, README, working-with-awf, and changelog authored surfaces with the strict migration
  schema, repository-review attribution boundary, JSON schema, read-only guarantee, approval use,
  bridge-only lifecycle, schema-generation-14 rationale, and hash/mode semantics in the same behavior
  commit; sync/check/gate; commit:

  ```commit
  feat(tooling): report current-state upgrade readiness
  ```

## Phase 3: Attest through a recoverable project transaction

- [ ] **Task 3.1: Add snapshot identity and lock attestation.** In `internal/git/git.go`, add
  `HeadAndClean`, returning HEAD and refusing staged, unstaged, untracked, conflicted, or unborn states
  while retaining linked-worktree/submodule support. In `internal/manifest/manifest.go`, add optional
  `BridgeAttestation {Version, PreparedHead, TreeDigest, ADRFormatV1From, LegacyADRGaps}` with stable
  JSON and old-lock omission. Create `internal/bridge/digest.go` over sorted slash-relative path/mode/content records from the post-normalization proposed result for exactly config, domains, ADRs, topics, configured marker sources, and the required
  `.awf/current-state-migration.yaml`. Its digest record includes the approval file's exact path, mode,
  and content; no other path enters the universe. Compute cutoff as highest ADR plus one and gaps as sorted absent lower identities. Test
  every dirty state, every digest input and approval-file content/mode/add/remove invalidation, JSON
  round trip, and old lock.

- [ ] **Task 3.2: Implement the versioned journal contract.** Create
  `internal/bridge/journal.go` at `.awf/current-state-upgrade.journal`. JSON version is integer `1`;
  phases are exactly `prepared`, `applying`, `rolling-back`, and `lock-committed`; fields are
  `version`, `phase`, `finalLockSHA256`, and ordered `operations`. Each operation contains a slash-
  relative `path`, `prior` and `replacement`; each image contains `present`, octal `mode`, and base64
  `content`, with absent encoded as `present:false`, mode 0, empty content. Paths are unique and sorted,
  the lock operation is last, and the complete journal is atomically durable before mutation.

  Unknown versions, phases, duplicate/unsafe paths, invalid modes/base64, missing lock-last, and
  malformed JSON refuse without mutation. Recovery uses this table: a precommit phase with lock hash
  unequal to `finalLockSHA256` restores every operation in reverse order after verifying each current
  image equals either prior or replacement; a precommit phase whose lock already has the final hash
  treats the lock as committed and cleans up; `lock-committed` plus the final hash cleans up only;
  `lock-committed` plus another hash refuses rather than rolling authority back. Any third-party image
  or failed rollback preserves the journal and reports the exact path. Chmod is part of image restore.
  Before lock commit, any failure enters `rolling-back`; after lock commit, cleanup failure leaves the
  attested lock plus journal. Every valid journal phase, including `lock-committed`, blocks all project commands except `awf upgrade --recover`; postcommit recovery only cleans residue and never rolls authority back. Repeated recovery is byte/mode idempotent.

- [ ] **Task 3.3: Add attestation, recovery, and the command-state guard atomically.** Implement
  `upgrade --attest-current-state` as readiness plus clean HEAD plus journaled normalization/config/
  marker/status/terminal-output writes, with attestation lock last. The approval file remains
  byte-for-byte and mode-for-mode unchanged through attestation unless another planned edit already
  targets it, and is absent from journal operations merely due to digest membership. Implement
  `upgrade --recover` before config or project opening. In `cmd/awf/main.go`, install refusal in the
  same change so no committed journal/attestation state is reachable without protection.

  Pin this bridge-release matrix: with a valid journal, only `upgrade --recover` may touch the project;
  `--check`, plain upgrade, attestation, and every ordinary project command refuse. With an attested
  lock and no journal, only `upgrade --check` may inspect it; plain upgrade, re-attestation, recovery,
  and ordinary commands refuse with an install-the-current-state-release diagnostic. Plan 3 changes
  plain upgrade to consume the attestation. A malformed journal refuses all project modes including
  recovery with deterministic Git-restoration guidance; a corrupt lock without a journal keeps the
  existing hard refusal; a valid journal permits recovery even when the current lock is corrupt,
  applying the journal state table. Help, version, and changelog bypass project transaction state.
  Static config/context/topic fallback occurs only outside an adopted tree and therefore also bypasses
  it. `--check` remains read-only and does not require cleanliness before attestation. Print
  deterministic operation lines and never claim to have run project tests or gates.

- [ ] **Task 3.4: Failure-inject every transaction edge and matrix cell.** Cover preparation,
  replacement, deletion, chmod, lock replacement, rollback, and cleanup failures; compare full path
  bytes/modes after recovery; exercise every journal/attestation/corruption/mode cell above; assert
  refusal occurs before project/corpus loading; and assert cleanup recovery preserves the committed
  lock. Run `go test ./internal/git ./internal/manifest ./internal/bridge ./cmd/awf`;
  expected: all report `ok`.

- [ ] **Task 3.5: Document, sync, gate, and commit.** Update architecture, config, rendering, tooling,
  working-with-awf, bootstrap-upgrade, workflow gate/hooks, README, release guidance, and changelog sources with the
  exact check/test/gate/clean-HEAD/attest/recover sequence, the deliberate index-pruned command-refusing window, a requirement to obtain and verify the matching current-state binary before attestation, and Git-restoration-plus-bridge-reinstallation escape guidance. Extend Plan 1's
  commands part with bridge preparation, refusal, JSON, and recovery commands so root and Sundial
  AGENTS.md render in the same behavior commit. Run `./x sync`,
  `./x check`, and `./x gate`; commit:

  ```commit
  feat(tooling): attest current-state upgrade readiness
  ```

## Phase 4: Close the bridge release boundary

- [ ] **Task 4.1: Flip the release sentinel only after the bridge is complete.** Final plan resync must
  first add `project.BridgeTrancheComplete = false` and releasecheck refusal to Plan 1's schema/version
  commit, so every Plan-1 implementation commit is mechanically unreleasable. In this phase, after
  Phases 1-3 are committed and green, change only the sentinel to true. Extend releasecheck tests and
  workflow-order pins so GoReleaser cannot run while false and still runs gate/check/releasecheck in
  order when true.

- [ ] **Task 4.2: Keep Sundial as the unattested bridge oracle.** Extend
  `internal/project/example_wiring_test.go` to assert schema 14, no attestation/journal/topic cutover,
  legacy invariants config and ordinary check/invariants behavior, rendered bridge docs/help, and the
  exact command-state matrix from Task 3.3. Assert schema generation remains 14 throughout the
  unreleased Plans 1-2 tranche and that the unattested fixture has no migration approval file. Do not
  author topics, approvals, or rewrite markers there.

- [ ] **Task 4.3: Publish bridge-complete release guidance and commit behavior.** Update release
  guidance, domain/architecture docs, README, and changelog with the sentinel and no-intermediate-
  release rule. Run `./x sync`, `./x check`, `./x gate`, and `git diff --check`; expected: clean drift,
  100% coverage, no dead code, clean prose, and no diff-check output. Stage the sentinel,
  releasecheck/workflow tests, Sundial oracle, authored docs, and generated fan-out; commit:

  ```commit
  feat(awf): close current-state bridge release
  ```

- [ ] **Task 4.4: Freeze only the plan.** Record implementation findings, set only this plan to
  Implemented, and leave all linked ADRs Proposed. Run `./x sync`, `./x check`, and `./x gate`; stage
  only this plan and generated plan/index/lock outputs; commit:

  ```commit
  docs(plans): implement bridge migration tooling
  ```

## Phase 5: Publish the bridge release

- [ ] **Task 5.1: Create the dated v0.18.0 release commit.** After Plan 2 is Implemented and the
  sentinel is true, set `release_date=$(date -u +%F)`, promote bridge notes to
  `[0.18.0] - $release_date`, restore empty Unreleased, verify
  Version/locks 0.18.0, and run sync/check/gate/audit/releasecheck plus pinned GoReleaser check and
  snapshot. Commit `docs(awf): release 0.18.0` on main.

- [ ] **Task 5.2: Publish annotated bridge tag.** From clean main fetch tags, reject an existing local/
  remote v0.18.0, push main, verify origin/main equals HEAD, create annotated `v0.18.0` at that release
  commit, verify its peeled commit, push the tag, and require the release workflow to pass. Sundial
  remains unattested; Plan 4 performs both real attestations.

## Verification

- `upgrade --check` is byte-for-byte read-only and reports every readiness predicate independently.
- Attestation requires readiness and a clean HEAD, records a stable digest/cutoff/gaps, and commits the
  lock last; every injected failure restores or remains recoverable.
- Inventory independently maps every live legacy invariant exactly once, then requires one matching
  authored approval without generating claim prose; retired entries derive approval only from valid
  history evidence or rationale.
- The approval file is optional only before preparation, required at readiness even with `invariantApprovals: []`, participates in exact path/mode/content digest invalidation,
  remains unchanged through attestation, is journal-deleted at final upgrade, and never enters attestation mutations merely as a digest member.
- An unattested schema-14 project retains legacy authority; an attested project and any journal state
  follow the exact Task 3.3 command matrix and refuse ordinary commands.
- The release pipeline cannot publish the Plan 1-only tranche or an incomplete Plan 2.
- No Plan 3 runtime or Plan 4 adopter-cutover behavior appears in the diff.

## Notes

- Plan 3 supplies the permanent current-state output plan, repository-wide coverage runtime,
  new-format ADR lifecycle, State changes, permanent staged/range checks, INDEX.md, and legacy
  consumer deletion. Plan 2 owns only the migration-safe terminal deletion projection and cross-schema
  attestation adapter.
- Plan 4 authors and attests the real awf/Sundial corpora and cuts the breaking release.

## Implementation findings

- The coupled Phases 1-2 and Phase 3 each landed as a single commit, as the dead-code coupling
  anticipated: their bridge functions have no main-reachable caller until `upgrade --check` and
  `upgrade --attest-current-state` respectively.
- Phases 1-2 review surfaced a readiness deadlock. A legacy invariant retired by an effective
  retirement token, whose encoded `## Migration history` ledger line was not yet present, was rejected
  at readiness even though normalization plans to append that exact line and read-only `upgrade --check`
  cannot pre-write it (attestation needs readiness, but readiness demanded the line only attestation
  writes). Fixed by treating the effective token as valid encoded-history evidence at the approval and
  adjudication sites, per ADR-0136 Decisions 4-5. The plan's fixtures pre-wrote ledgers and masked it;
  a regression test now exercises the token-without-ledger path.
- Phase 3 review removed one false `coverage-ignore` on the journal rollback read: that branch is
  reached deterministically by a directory sitting at the lock path during a halted rollback, not by a
  concurrent-removal race, and an existing failure-injection test already covers it.
- Phase 4.1's sentinel flip required inverting three test guards (two in `cmd/releasecheck/main_test.go`,
  one in `internal/project/version_test.go`). The existing release-workflow-order pins already enforce
  that GoReleaser cannot run while releasecheck refuses, so no new workflow steps were added.
- Bridge command prose was not pushed into `examples/sundial` `AGENTS.md` `## Commands` (that section
  renders each adopter's own commands sidecar). Sundial instead surfaces the bridge guidance through its
  rendered `docs/working-with-awf.md`, `.awf/upgrade.sh`, and the shared `awf upgrade --help`, following
  the Phase-2 precedent; Task 4.2's oracle test asserts those rendered surfaces.
