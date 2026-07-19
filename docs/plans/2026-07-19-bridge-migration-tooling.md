---
date: 2026-07-19
adrs: [133, 134, 135, 136]
status: Proposed
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

- **Created:** `internal/bridge/{inventory,history,normalize,readiness,digest,journal,bridge}.go` and
  matching tests; `cmd/awf/upgrade_test.go`.
- **Modified:** `internal/adr/{adr,status,declarations}_test.go` and production siblings;
  `internal/config/{config,edit,edit_test}.go`; `internal/project/{output_plan,output_plan_test,
  sweep,sweep_test,example_wiring_test}.go`; `internal/manifest/{manifest,manifest_test}.go`;
  `internal/git/{git,git_test}.go`; `internal/testsupport/gitfixture/gitfixture.go`;
  `internal/clispec/{clispec,clispec_test}.go`; `cmd/awf/{main,main_test,dispatch,upgrade,gate_test,
  failure_paths_test}.go`; `cmd/releasecheck/{main,main_test}.go`; `.github/workflows/release.yml`;
  `templates/{docs/working-with-awf.md.tmpl,bootstrap/awf-upgrade.sh.tmpl,hooks/pre-commit.sh.tmpl}`;
  `.awf/parts/workflow/{composing-the-gate,local-hooks}.md`;
  `.awf/docs/parts/{architecture/components,architecture/data-flow,releasing/content}.md`;
  `.awf/domains/parts/{adr-system,config,invariants,rendering,tooling}/current-state.md`; `README.md`;
  `changelog/CHANGELOG.md`; and Plan 1's release-sentinel task during final resync.
- **Generated:** root and Sundial locks plus rendered architecture, workflow, release, domain,
  working-with-awf, bootstrap, hook, and agent-guide outputs selected by `./x sync`.
- **Deleted:** none. The committed Sundial fixture remains unattested and legacy-authoritative.

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
  carrier Decision item, and active/retired state. Do not call the narrower runtime
  `invariants.DeclaringADRs` and do not infer prose or topics.

- [ ] **Task 1.3: Parse and plan append-only Migration history.** Create
  `internal/bridge/history.go`. Parse one optional `## Migration history` section outside fences. Accept
  only ``- YYYY-MM-DD: retired invariant `ADR-NNNN#slug`; basis: encoded`` and the migration-basis form
  ending `; rationale: <nonempty text>`. Reject unknown bases, duplicates, malformed dates, an encoded
  entry without an effective token, and a migration entry without rationale. Create
  `internal/bridge/normalize.go` raw-byte edits that append missing encoded entries using the carrier
  ADR date, preserve all other bytes, and run before Superseded-to-Implemented frontmatter rewrites.
  Repeated planning must return byte-identical no-op output.

- [ ] **Task 1.4: Add inventory/history fixtures.** Create focused bridge tests for Implemented and
  Superseded declarers/carriers, inactive Proposed/Accepted tokens, lapsed tokens, encoded/migration
  entries, duplicates, unknown bases, missing rationale, fenced examples, byte preservation, and
  idempotence. Keep generation-10 retirement-token tests unchanged. Run
  `go test ./internal/adr ./internal/bridge`; expected: both packages report `ok`.

- [ ] **Task 1.5: Commit the inventory foundation with current docs.** Update the invariants and
  ADR-system authored current-state sources and Unreleased changelog in the same commit, explicitly
  calling this migration-only inventory rather than runtime authority. Run `./x sync`, `./x check`,
  and `./x gate`; stage only Phase 1 paths and generated fan-out; commit:

  ```commit
  feat(invariants): inventory legacy migration obligations
  ```

## Phase 2: Assemble a read-only readiness report

- [ ] **Task 2.1: Add comment-preserving prepared-config conversion.** In
  `internal/config/edit.go`, add `ConvertInvariantsToCurrentState`: copy enabled legacy source globs,
  marker, optional close, and testGlobs; set coverage error, fan-out warn, and maximum 8; remove the
  old key; preserve unrelated YAML comments/order. Refuse `invariants.disabled: true` and conflicts
  with a nonidentical authored currentState block. Parse and validate the proposed bytes through strict
  config. Add round-trip/idempotence/refusal tests.

- [ ] **Task 2.2: Plan qualified marker rewrites.** Extend `internal/bridge/normalize.go` to scan only
  legacy configured sources, map each live inventory key to exactly one Plan 1 topic invariant with
  matching legacy Origin and backing class, rewrite proof markers to qualified IDs, and rewrite
  `touches-invariant:` to `touches-state:` while preserving a required note. Reject missing/duplicate
  mappings, changed class, note-less touches, and every remaining unqualified marker in the scan
  universe; ignore historical ADR prose and nested projects. Validate proposed bytes with Plan 1's
  marker parser.

- [ ] **Task 2.3: Expose a deterministic prepared-output projection.** In
  `internal/project/output_plan.go`, add a read-only bridge projection containing sorted path, bytes,
  desired mode, policy, dependency hashes, and planned deletion/reservation facts. Prove it matches
  normal output-plan bytes and modes. Reserve the fixed bridge journal path in `sweep.go`; an existing
  journal is transaction state, not an orphan. Do not generate INDEX.md or remove ACTIVE/domain ADR
  indexes in this plan.

- [ ] **Task 2.4: Build the one readiness report.** Create `internal/bridge/readiness.go` with stable
  `Finding {Code, Path, Detail}` and `Report`. Over proposed in-memory bytes, independently check:
  strict config conversion; canonical domain keys; no Proposed/Accepted ADR; planned Superseded
  normalization; valid migration history; exact live-invariant mapping/class; qualified markers;
  repository-wide domain-owned scoped topic coverage with empty/global topics excluded; topic parse,
  references, backing, and render completeness; and collision-free output planning. Model legacy
  ACTIVE/domain-index removal as an explicit following-release output-plan prerequisite, not as a file
  mutation or a readiness failure the bridge can never satisfy before Plan 3 supplies that plan.

- [ ] **Task 2.5: Add `awf upgrade --check`.** Add mutually exclusive upgrade flags to the one
  clispec table and pass parsed flags through dispatch. Refactor `cmd/awf/upgrade.go` so plain upgrade
  retains ordered migration plus sync, while `--check` assembles and prints the readiness report
  without writes, chmods, index changes, or lock changes. Invalid combinations fail before filesystem
  access. Add human-output ordering and full-tree digest no-mutation tests in `cmd/awf/upgrade_test.go`.

- [ ] **Task 2.6: Test every readiness predicate and commit.** Use one valid fixture, then fail each
  predicate independently and assert its stable code/path. Cover tracked and nonignored untracked
  eligible files, generated/ignored/deleted/nested/contextIgnore exclusions, multi-domain gaps, empty
  topics, and globals not satisfying scoped coverage. Run `go test ./internal/config ./internal/bridge
  ./internal/project ./internal/clispec ./cmd/awf`; expected: all packages report `ok`. Update config,
  rendering, invariants, tooling, architecture, README, working-with-awf, and changelog authored
  surfaces in the same behavior commit; sync/check/gate; commit:

  ```commit
  feat(tooling): report current-state upgrade readiness
  ```

## Phase 3: Attest through a recoverable project transaction

- [ ] **Task 3.1: Add snapshot identity and lock attestation.** In `internal/git/git.go`, add
  `HeadAndClean`, returning HEAD and refusing staged, unstaged, untracked, conflicted, or unborn states
  while retaining linked-worktree/submodule support. In `internal/manifest/manifest.go`, add optional
  `BridgeAttestation {Version, PreparedHead, TreeDigest, ADRFormatV1From, LegacyADRGaps}` with stable
  JSON and old-lock omission. Create `internal/bridge/digest.go` over sorted path/mode/content records
  for config, domains, ADRs, topics, and configured marker sources. Compute cutoff as highest ADR plus
  one and gaps as sorted absent lower identities. Test every dirty state, digest input, JSON round trip,
  and old lock.

- [ ] **Task 3.2: Implement the versioned journal.** Create `internal/bridge/journal.go` with a fixed
  `.awf/current-state-upgrade.journal` JSON file. Precompute all operations and replacements before
  destination mutation; snapshot each path's prior absence or bytes and mode; record transaction
  phase and final lock hash; use same-directory atomic writes; replace the lock last. Before lock
  commit, any failure rolls back; rollback failure preserves the journal. After lock commit, cleanup
  failure preserves a successful attestation plus stale journal. Recovery is idempotent and chooses
  rollback or cleanup solely from the recorded commit phase and actual lock hash.

- [ ] **Task 3.3: Add attestation and recovery modes.** Implement
  `upgrade --attest-current-state` as readiness plus clean HEAD plus journaled normalization/config/
  marker/status/output writes, with attestation lock last. Implement `upgrade --recover` before config
  or project opening. `--check` remains read-only and does not require cleanliness. Plain upgrade
  remains available only before attestation. Print deterministic operation lines and never claim to
  have run project tests or gates.

- [ ] **Task 3.4: Failure-inject every transaction edge.** Cover preparation, replacement, deletion,
  chmod, lock replacement, rollback, and cleanup failures; compare full path bytes/modes after
  recovery; assert every command refuses during a journal; assert cleanup recovery preserves the
  committed lock. Run `go test ./internal/git ./internal/manifest ./internal/bridge ./cmd/awf`;
  expected: all report `ok`.

- [ ] **Task 3.5: Document, sync, gate, and commit.** Update architecture, config, rendering, tooling,
  working-with-awf, bootstrap-upgrade, workflow gate/hooks, README, and changelog sources with the
  exact check/test/gate/clean-HEAD/attest/recover sequence and Git rollback guidance. Run `./x sync`,
  `./x check`, and `./x gate`; commit:

  ```commit
  feat(tooling): attest current-state upgrade readiness
  ```

## Phase 4: Close the bridge release boundary

- [ ] **Task 4.1: Centralize journal and attestation refusal.** In `cmd/awf/main.go`, after syntax
  parsing and before handler dispatch, refuse every project command while a journal exists; permit
  only recovery/inspection upgrade modes. When an attested bridge lock exists, refuse every ordinary
  project command, including sync, check, context, invariants, topic, and new; permit migration-safe
  upgrade modes. Preserve help/version/changelog and static fallbacks outside an adopted tree. Derive
  normal command membership from clispec rather than a second list.

- [ ] **Task 4.2: Pin the command matrix.** Extend main/gate/failure-path/Plan-1-topic tests for every
  command class, corrupt journal/lock, fallback order, and refusal before project/corpus loading.
  Prove an unattested schema-14 project still runs the legacy runtime, while an attested lock cannot.

- [ ] **Task 4.3: Mechanically prevent an incomplete bridge release.** During final plan resync, add a
  Plan 1 task that introduces `project.BridgeTrancheComplete = false` and a releasecheck assertion.
  Here, after all bridge tests/docs land, flip it to true. Extend releasecheck tests and workflow-order
  pins so GoReleaser cannot run unless the sentinel is true, the gate/check precede it, and Unreleased
  changelog rules still hold.

- [ ] **Task 4.4: Keep Sundial as the unattested bridge oracle.** Extend
  `internal/project/example_wiring_test.go` to assert schema 14, no attestation/journal/topic cutover,
  legacy invariants config and ordinary check/invariants behavior, and rendered bridge docs/help.
  Do not author topics or rewrite markers there.

- [ ] **Task 4.5: Final documentation, verification, and plan freeze.** Update release guidance,
  domain/architecture docs, README, and changelog in the same commit. Run `./x sync`, `./x check`,
  `./x gate`, and `git diff --check`; expected: clean drift, 100% coverage, no dead code, clean prose,
  and no diff-check output. Record findings, set only this plan to Implemented, leave all linked ADRs
  Proposed, and commit:

  ```commit
  docs(awf): complete current-state bridge tooling
  ```

## Verification

- `upgrade --check` is byte-for-byte read-only and reports every readiness predicate independently.
- Attestation requires readiness and a clean HEAD, records a stable digest/cutoff/gaps, and commits the
  lock last; every injected failure restores or remains recoverable.
- Inventory adjudicates every live legacy invariant exactly once without generating claim prose.
- An unattested schema-14 project retains legacy authority; an attested project and any journal state
  refuse ordinary commands.
- The release pipeline cannot publish the Plan 1-only tranche or an incomplete Plan 2.
- No Plan 3 runtime or Plan 4 adopter-cutover behavior appears in the diff.

## Notes

- Plan 3 supplies the permanent current-state output plan, repository-wide coverage runtime,
  new-format ADR lifecycle, State changes, staged/range checks, INDEX.md, and legacy deletion.
- Plan 4 authors and attests the real awf/Sundial corpora and cuts the breaking release.
