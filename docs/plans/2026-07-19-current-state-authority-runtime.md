---
date: 2026-07-19
adrs: [133, 134, 135, 136]
status: Proposed
---
# Plan: Current-State Authority Runtime

## Goal

Implement the current-state release runtime: immutable snapshots, the current-state-v1 ADR lifecycle,
topic-only context and invariants, static/staged/range checks, historical INDEX.md, sealed-attestation
consumption, and removal of legacy authority. Plan 4 owns real awf/Sundial content, attestation, ADR
outcomes, and release; all authority-changing work below closes with that cutover.

## Architecture summary

`internal/snapshot` provides complete immutable working/index/commit trees; consumers apply eligibility
filters. `internal/currentstate` loads one ADR/topic view per tree for static and before/after checks.
`internal/adr` retains legacy identity/history below the lock cutoff and strictly parses format-v1 ADRs
above it. Context, invariants, coverage, staged check, and audit consume topic claims and snapshot
pairs. Plain upgrade verifies the unchanged Plan 2 seal, applies the new output plan through the
journal, promotes cutoff/gaps to permanent lock fields, and removes migration-only code.

Only Phase 1 has a standalone commit. Phases 2-5 and Plan 4 are one coupled, unreleased closing slice:
Plan 4 first commits prepared adopter content under the bridge runtime, obtains clean-HEAD attestations,
then the executor applies the uncommitted runtime patch, runs final upgrades, sync/check/gate, and
commits everything. This preserves the seal's HEAD while allowing dirty non-sealed runtime source; the
sealed path digest must remain unchanged until consumed. Paths are rooted at
`/home/hypno/Projects/agentic-workflows`; approved symbol-contract/coupled-file exceptions apply, but
verification commands and affected sets remain exact.

## File structure

- **Created:** `internal/snapshot/{snapshot,index}.go` and tests in Phase 1; coupled slice adds
  `internal/snapshot/{working,commit,range}.go`, `internal/currentstate/{check,transition,
  legacy_absence}.go`, `internal/adr/{format,operations,history,digest}.go`, and exact matching tests.
- **Modified:** `internal/{git,audit,adr,topic,manifest,project,clispec,catalog}`; `cmd/awf/{main,
  upgrade,check,context,invariants,topic,dispatch}`; ADR/domain/hook/doc/skill/agent templates; authored
  `.awf/` workflow, agent-guide, ADR guide, architecture, glossary, pitfalls, config-reference, and
  domain sources; `README.md`; `changelog/CHANGELOG.md`.
- **Deleted in the coupled slice:** `internal/adr/{coverage,citations,domain}.go` and tests; unused
  declarations; legacy invariant runtime if replaced; migration-only bridge inventory/history/
  normalize/readiness/snapshot code; desired `docs/decisions/ACTIVE.md`; every legacy authority caller.
- **Generated in the coupled slice:** `docs/decisions/INDEX.md`, topic-only domain docs, every changed
  root/runtime/Sundial rendering, and both permanent locks. Plan 4 enumerates the actual fan-out after
  `./x sync` with `git diff --name-only` and asserts it against the output plan.

## Phase 1: Add the independently reachable index snapshot seam

- [ ] **Task 1.1: Add immutable Tree and index loading only.** Create
  `internal/snapshot/snapshot.go` with copied-byte `File {Path, Mode, Bytes}` and sorted Tree lookup/
  list methods used by the index caller; reject duplicate/unsafe paths and unsupported modes. Create
  `index.go` using `internal/git.OpenRepo` and stage-0 index blobs, preserving executable mode and
  rejecting unmerged indexes. Do not add working, commit, range, or unused subtree APIs yet.

- [ ] **Task 1.2: Route an existing production caller.** Replace `cmd/awf/context.go`'s legacy staged
  path-byte assembly with the index Tree while preserving current legacy output. Test staged additions,
  deletions, executable modes, unstaged isolation, unmerged refusal, linked worktrees, byte copying,
  and deterministic order in `internal/snapshot/{snapshot,index}_test.go` and context tests.

- [ ] **Task 1.3: Document and commit.** Update `.awf/docs/parts/architecture/components.md` and
  `data-flow.md` with the neutral snapshot seam, then run:

  ```sh
  go test ./internal/snapshot ./internal/git ./cmd/awf
  ./x sync
  ./x check
  ./x gate
  ```

  Expected: packages report `ok`, both checks are drift-free, coverage is 100%, and deadcode/prose are
  clean. Stage the named sources/tests, rendered architecture fan-out, and lock; commit:

  ```commit
  refactor(tooling): add immutable index snapshots
  ```

## Coupled Phases 2-5 and Plan 4: Implement and activate one authority engine

Do not commit any Phase 2-5 task independently. Plan 4's preparation commit and attestations happen
before these runtime edits are applied. The shared final commit subject is defined by Plan 4.

## Phase 2: Complete snapshots and the ADR/static model

- [ ] **Task 2.1: Complete faithful snapshot universes.** Add working and commit Trees containing the
  complete selected filesystem/Git universe, not context-filtered views. Working includes tracked and
  nonignored untracked regular files without following symlinks; commit/index never read working bytes.
  Add range pairs: root uses empty parent; merges use first-parent only for transition pairs while
  existing audit `Commit.Changes` keeps its legacy merge-empty semantics for unrelated rules. Context/
  coverage later apply generated, contextIgnore, nested-adopter, symlink, deleted, and eligibility
  filters. Test every universe and both merge semantics.

- [ ] **Task 2.2: Parse cutoff-aware ADRs.** Below cutoff parse legacy identity/title/status/date and
  provenance history only. At/above cutoff require exact format/status/date frontmatter; ordered six
  sections; Proposed/Accepted/Implemented/Abandoned; five legal edges; sequential Decision items;
  exact None or unique add/update/remove operations; canonical five-section digest; ordered Status
  history; Abandoned rationale; mutation-only positive state sequence. Enforce cutoff gaps, contiguous
  unique sequences, one add, ordered updates, at most one remove, and no ID reuse. Build pending and
  removed-identity indexes.

- [ ] **Task 2.3: Build the complete static checker.** `internal/currentstate/check.go` validates
  lifecycle/digest/sequence/operation/provenance/reference/backing facts. Proposed and Accepted adds
  must be absent and their updates/removes present; Abandoned operations remain wholly unapplied;
  Implemented operations and active Origin/Revised-by are bidirectional; only pre-cutoff Origin gets
  the bootstrap exemption. Coverage/fanout is deferred to Phase 3's shared evaluator.

- [ ] **Task 2.4: Update topic history and authoring.** `awf topic <removed-id>` fails normally but
  resolves direct add/update/remove ADR history with `--history` in human and JSON; active selectors
  retain Plan 1 behavior. Make `awf new adr`, templates, ADR README, lifecycle/proposing/review/
  planning/execution/retrospective skills, and reviewer agents use new format, State changes, digest/
  sequence guidance, Accepted/Abandoned, and no tags/relations/supersession/invariant declarations.

- [ ] **Task 2.5: Test the model.** Add exhaustive parser/static/topic-history fixtures, including
  partial Proposed/Accepted/Abandoned mutations and removed-with/without-history selectors. Do not
  create `currentstate.Check` until the coupled slice provides its Phase 3 production caller; implement
  Tasks 2.3 and 3.4 together if deadcode ordering requires it.

## Phase 3: Replace normal authority and plain check

- [ ] **Task 3.1: Implement one coverage result.** Extend topic coverage with findings keyed by
  `(path, owner-domain, kind)`. Error findings make check/command exit nonzero; warn findings render in
  human/JSON but exit zero; off emits nothing. Coverage is independent per owner; empty/global topics
  do not satisfy scoped coverage. Fanout counts path-scoped topics once per path across owners, emits
  one path finding, and excludes globals. Human/JSON/uncovered/check consume the same sorted result.

- [ ] **Task 3.2: Replace context.** Working explicit/directory/uncovered queries and index staged/
  staged-uncovered queries apply eligibility filters after Tree load. Select global and owner-bounded
  topics per file; state markers narrow only their already-applicable topic; union per-file selections.
  Output summaries/current claims plus a separate Accepted pending section targeted only by matched
  topics. Delete ADR/tag/relation/plan/pitfall/background/supersession expansion.

- [ ] **Task 3.3: Replace invariant authority.** Standalone invariants and project checks consume only
  typed topic claims and qualified proof markers; test backing requires source plus testGlobs, unbacked
  requires Verify and forbids proof. Remove ADR imports from the authority path.

- [ ] **Task 3.4: Wire plain `awf check`.** Load exactly one working Tree, build ADR/topic/marker
  corpora from it, run the complete static checker and coverage/fanout evaluator, and route errors versus
  warnings exactly as Task 3.1. Test no mixed working/index reads and output parity.

## Phase 4: Add atomic staged and range checks

- [ ] **Task 4.1: Implement `CheckPair`.** HEAD/index or parent/commit Trees enforce matching ADR
  transitions and claim add/update/remove mutations. Updates preserve Origin, retain prior Revised-by
  as exact prefix, append once, and change a canonical nonformat/nonprovenance field. Reject unmatched
  operations/mutations and run after-state static checks.

- [ ] **Task 4.2: Wire staged check and audit.** Add `check --staged`; the hook invokes it. Audit checks
  every explicit-range transition pair, including first-parent merge integration, while existing audit
  rules retain their prior merge behavior. Update reviewing-impl prose. Test every split/mismatch,
  whitespace/provenance-only update, merge, root, and no-working-read case.

## Phase 5: Consume attestation, replace indexes, and delete legacy code

- [ ] **Task 5.1: Verify only sealed facts.** Accept Plan 2 attestation version 1; require current HEAD
  equals PreparedHead; recompute the exact sorted path/mode/content digest over config, domains, ADRs,
  topic metadata/parts, and configured marker sources; promote sealed cutoff/gaps; trust legacy
  inventory adjudication only through that unchanged seal. Recompute every permanent new-tree static,
  coverage, output-plan, and transition predicate. The final binary imports no bridge inventory or
  cross-schema adapter.

- [ ] **Task 5.2: Run journaled final upgrade.** Reuse the version-1 image/phase/recovery contract;
  validate the complete operation list, replace lock last, clear attestation, and store permanent
  cutoff/gaps. Tests in `cmd/awf/upgrade_test.go`, `internal/manifest/manifest_test.go`, and journal
  tests cover version/HEAD/digest/path/mode mismatches, each journal phase, rollback/cleanup failures,
  lock-last failure, seal invalidation, cutoff/gap promotion, and current-state command enablement.

- [ ] **Task 5.3: Generate INDEX and topic-only domains.** INDEX renders Proposed/Accepted In flight
  and compact Implemented/Abandoned History. ACTIVE and domain ADR indexes are absent. Output-plan,
  layout, catalog naming, ADR README, domain template, drift/prune tests assert exact paths and status
  sections.

- [ ] **Task 5.4: Delete and deterministically deny legacy authority.** Scan production `.go` outside
  `_test.go`, embedded runtime templates, and desired-output declarations; exclude historical ADRs,
  plans, changelog, and test fixtures. Deny identifiers `SupersessionRef`, `AnnotatedAnchors`, `Chains`,
  `Retirers`, `StateCovered`, `PartiallySuperseded`, `DeclaringADRs`, `RenderActiveMD`,
  `RenderDomainIndex`, and legacy context fields `Governing`, `Related`, `Background`; deny production
  imports of migration inventory/readiness/snapshot packages; deny desired path
  `docs/decisions/ACTIVE.md`. Representative tests place a token in production; edge tests prove a
  historical fixture remains legal. Post-check:

  ```sh
  go test ./internal/currentstate -run '^TestLegacyAuthorityAbsent$' -count=1
  go tool deadcode -json ./... | go run ./cmd/deadcodecheck
  ```

  Expected: package `ok` and `deadcodecheck: no production dead code`.

- [ ] **Task 5.5: Update the exact authored fan-out.** Modify ADR template/readme and index part;
  brainstorming, proposing/reviewing/lifecycle/planning/execution/implementation-review/retrospective
  skill templates and project parts; all three reviewer agents; AGENTS invariants/workflow/commands/
  document-map parts; workflow and working-with-awf templates; architecture components/data-flow;
  adr-system/invariants/rendering/tooling domain parts; config-reference descriptions; glossary terms;
  pitfalls entries; README and Unreleased changelog. Render every enabled root target and Sundial copy.

## Shared Plan 4 closing sequence

Plan 4 must name the preparation commit and execute this exact order:

1. Under the bridge binary, commit prepared awf/Sundial config/topics/markers/ADR normalization while
   legacy authority still runs; run `./x sync`, `./x check`, configured tests, and `./x gate`.
2. From that clean HEAD, attest both adopters. Do not commit attestation; HEAD must remain PreparedHead.
3. Apply the complete uncommitted Plan 3 runtime patch without changing sealed paths; build it and run
   plain final upgrade for root and Sundial, consuming both seals.
4. Run the final runtime's `./x sync`, `./x check`, and `./x gate`. Assert both permanent locks carry
   cutoff/gaps and no attestation; INDEX has both sections; ACTIVE/domain ADR indexes are absent; output
   plan equals generated fan-out; legacy-absence tests are clean.
5. Update Notes and set both Plan 3 and Plan 4 to Implemented; transition ADR-0133-0136 to their Plan 4
   outcomes; sync/check/gate again; stage exact output-plan paths; commit with Plan 4's declared subject.

## Verification

- Normal context/invariants/plain check consume only current-state claims.
- Static, staged, and range checks share faithful snapshot-loaded corpora.
- Removed history is explicit; Accepted changes never override current claims.
- Final upgrade accepts only the unchanged seal and remains journal-recoverable.
- INDEX/domain outputs contain no currentness inference or ADR index.
- Denylist, import boundaries, and deadcode prove the legacy engine is absent.

## Notes

- This plan deliberately has no independent closing commit after Phase 1; Plan 4 owns the shared close.
