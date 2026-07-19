---
date: 2026-07-19
adrs: [133, 134, 135, 136]
status: Proposed
---
# Plan: Current-State Authority Runtime

## Goal

Implement the following current-state release: permanent snapshots, new-format ADR lifecycle and
claim operations, topic-backed context/invariants/coverage, static and transition checks, historical
INDEX.md, attestation-consuming final upgrade, and removal of legacy authority engines. This plan does
not author or attest the real awf/Sundial topic corpora, rewrite their markers/config, flip the four
ADRs, or publish the breaking release; Plan 4 performs that project cutover.

## Architecture summary

Introduce immutable `internal/snapshot` trees for working, index, and commit views, and
`internal/currentstate` for static graph checks and before/after transactions. Refocus `internal/adr`
on legacy identity plus strict current-state-v1 format, State changes, status history, digests, and
operation order. Replace ordinary context and invariant consumers with the Plan 1 topic corpus, drive
staged and audit checks from snapshot pairs, replace ACTIVE/domain ADR indexes with INDEX/topic
navigation, and make plain upgrade verify a Plan 2 attestation before a journaled final lock commit.

Plan 3's final legacy-deletion phase and Plan 4's real adopter cutover are one unreleased execution
tranche: after the legacy engine is deleted, source `./x sync`/`./x check` cannot operate on the still-
bridge awf/Sundial trees. Render/check authored changes immediately before that switch, run `./x gate`
after it, do not release, and execute Plan 4 next. All paths are rooted at
`/home/hypno/Projects/agentic-workflows`; the previously approved symbol-contract and tightly-coupled-
file exceptions apply to this program.

## File structure

- **Created:** `internal/snapshot/{snapshot,working,git,range}.go` and tests;
  `internal/currentstate/{check,transition,legacy_absence}.go` and tests;
  `internal/adr/{format,operations,history,digest}.go` and tests.
- **Modified:** `internal/{git,audit,adr,topic,manifest,project,clispec}` runtime/tests;
  `cmd/awf/{main,upgrade,check,context,invariants,topic,dispatch}` and tests;
  `templates/{adr-template,adr-readme,domains,hooks,skills,agents,agents-doc,docs}`;
  `internal/catalog/standard.go`; `.awf/` authored ADR/workflow/agent/reviewer/domain/architecture
  sources; `README.md`; `changelog/CHANGELOG.md`.
- **Deleted:** `internal/adr/{coverage,citations,domain}.go` and tests; legacy declaration parsing after
  bridge extraction; migration-only `internal/bridge/{inventory,history,normalize,readiness,snapshot}`
  and tests; legacy runtime invariant implementation if replaced; desired `docs/decisions/ACTIVE.md`;
  all supersession/tag-tier/domain-index runtime paths.
- **Generated:** `docs/decisions/INDEX.md`, topic-only domain docs, updated standards/runtime copies,
  and final-upgrade fixture locks. Real awf/Sundial generated cutover remains Plan 4.

## Phase 1: Add immutable snapshot trees

- [ ] **Task 1.1: Create the snapshot value model.** Add `internal/snapshot/snapshot.go` with immutable
  `File {Path, Mode, Bytes}` and sorted `Tree` lookup/list/subtree methods. Copy input bytes, reject
  duplicate/unsafe paths and unsupported modes, and expose no filesystem handle.

- [ ] **Task 1.2: Load working, HEAD, index, and commit trees.** Add `working.go` for tracked plus
  nonignored untracked regular files while excluding generated outputs, contextIgnore, nested adopted
  projects/repositories, symlink traversal, deleted/missing paths; add `git.go` reusing
  `internal/git.OpenRepo` and stage-0 index blobs without working-tree reads. Preserve executable mode.

- [ ] **Task 1.3: Enumerate explicit-range pairs.** Add `range.go`: every included commit yields its
  parent-to-commit tree pair; root commits use empty before; merges use first parent as integration
  baseline while included branch commits remain individually enumerated. Replace audit's merge-empty
  shortcut. Test additions/deletions/modes, ignored/untracked/nested/symlink behavior, linked worktrees,
  unmerged index refusal, merge pairs, and working/index isolation.

- [ ] **Task 1.4: Integrate the first production caller and commit.** Route existing staged path
  selection through snapshot trees without changing authority output, update architecture/git docs,
  run `go test ./internal/snapshot ./internal/git ./internal/audit ./cmd/awf`, `./x sync`, `./x check`,
  and `./x gate`; commit:

  ```commit
  refactor(tooling): add immutable repository snapshots
  ```

## Phase 2: Implement the current-state ADR and static graph model

- [ ] **Task 2.1: Parse the cutoff-aware ADR format.** Refactor `internal/adr`: below the permanent
  lock cutoff parse legacy identity/title/status/date only where history/provenance needs it; at and
  above cutoff require frontmatter exactly `format: current-state-v1`, status, date, and ordered
  Context, Decision, State changes, Consequences, Alternatives Considered, Status history sections.
  Accept Proposed, Accepted, Implemented, Abandoned with only the five legal edges; enforce sequential
  Decision items. Recorded lower gaps can never be backfilled.

- [ ] **Task 2.2: Parse State changes, history, digest, and sequences.** Add exact `None.` versus unique
  add/update/remove qualified-ID operations; canonical five-section SHA-256; append-only dated history;
  Accepted/terminal digest repetition; Abandoned rationale; terminal state sequence only for
  Implemented mutation ADRs; unique contiguous sequences. Build corpus indexes by operation, claim,
  removed identity, sequence, affected topic/domain, and Accepted pending topic. A removed ID is never
  reused; rename/move/split/merge are expressed only through operation combinations.

- [ ] **Task 2.3: Implement static bidirectional checking.** Create
  `internal/currentstate/check.go` over one snapshot-loaded ADR/topic/marker view. Validate cutoff/gaps,
  status/history/digests/sequences, add-update-remove order, Implemented operations against active or
  absent claims, inverse Origin/Revised-by operations, pre-cutoff Origin exemption, Accepted destination
  metadata, unapplied Abandoned operations, claim references/backing, and permanent coverage/fanout
  severities. Static checks guarantee structure, never semantic fidelity.

- [ ] **Task 2.4: Update scaffolding and lifecycle surfaces.** Make `awf new adr` emit new format and
  Proposed history at/above cutoff. Rewrite ADR template/readme, proposing/review/lifecycle/execution/
  retrospective skills and reviewer agents for Accepted/Abandoned, State changes, digest suggestions,
  atomic claim mutation, and no supersession anchors/tags/relations/invariant declarations. Update
  authored workflow/AGENTS/domain/architecture sources in the same commit.

- [ ] **Task 2.5: Test, render, gate, and commit.** Cover every format/status/operation/history/digest/
  sequence/provenance branch plus publication-safe scaffolds. Run `go test ./internal/adr
  ./internal/currentstate ./cmd/awf`, `./x sync`, `./x check`, `./x gate`; commit:

  ```commit
  feat(adr-system): add checked current-state impacts
  ```

## Phase 3: Switch normal context, invariants, and coverage

- [ ] **Task 3.1: Replace context assembly.** Rewrite `internal/project/context.go` and CLI output so
  each eligible file loads global plus owning-domain path topics, applies state markers only within an
  already matching topic, unions claims per file, and prints topic summaries/current claims. Remove
  Governing/Related ADRs, tags, supersession, plans, pitfalls, and background expansion. Add a separate
  Pending accepted changes section selected only by Accepted operations targeting matched topics;
  current claims remain authoritative.

- [ ] **Task 3.2: Make snapshot semantics exact.** Explicit file/directory and uncovered queries use
  working snapshots; `--staged` and `--uncovered --staged` use the index snapshot exclusively.
  Directories expand eligible descendants. Missing/deleted/generated/ignored/nested/contextIgnore
  paths stay excluded. Remove the old staged-uncovered rejection.

- [ ] **Task 3.3: Extend permanent coverage/fanout.** Extend `internal/topic/coverage.go` to report
  unowned paths and, independently per owning domain, missing scoped-topic coverage; empty/global
  topics never satisfy it. Apply error/warn/off coverage and fanout modes, positive maximum, and
  path-scoped topic count. `awf check`, normal uncovered output, and JSON share one result.

- [ ] **Task 3.4: Replace standalone invariant authority.** Make `awf invariants` and Project checks
  consume only typed topic invariant claims and qualified proof markers. Test-backed claims require a
  proof in source plus testGlobs; unbacked claims require Verify and forbid proofs. Remove ADR
  declaration/retirement imports from the runtime invariant package.

- [ ] **Task 3.5: Prove one authority engine and commit.** Test context selection/per-file union,
  out-of-scope markers, Accepted conflicts, history exclusion, all snapshot universes, uncovered/fanout
  severity, and topic-only invariant reporting. Update CLI help, working-with-awf, AGENTS, domain/
  architecture docs, README, changelog; sync/check/gate; commit:

  ```commit
  feat(tooling): switch runtime to current-state claims
  ```

## Phase 4: Enforce staged and range transactions

- [ ] **Task 4.1: Implement pair checking.** Create `internal/currentstate/transition.go` with
  `CheckPair(before, after snapshot.Tree)`. Detect transitions into Implemented; require each add absent/
  present with matching Origin, each update present in both with Origin preserved, prior Revised-by as
  exact prefix and transitioning ADR appended once, plus a canonical nonformat/nonprovenance change;
  require remove present/absent. Every claim mutation belongs to exactly one transition and every
  operation has one mutation. Run after-state static checks.

- [ ] **Task 4.2: Add `awf check --staged`.** Add the flag to clispec/dispatch, load HEAD and index
  entirely from snapshots, and never mix working config/topics/markers. Replace rendered pre-commit
  payload's plain check with `awf check --staged`. Cover direct Proposed-to-Implemented and every split
  half/mismatch/whitespace/provenance-only update.

- [ ] **Task 4.3: Check every explicit-range pair.** Integrate transition findings into audit and
  implementation review for every enumerated parent/commit pair, first-parent merges included. Remove
  ACTIVE/domain-index cochange rules. Update reviewing-impl skill and audit docs to state that endpoint
  comparison is reporting only and never atomicity proof.

- [ ] **Task 4.4: Test, sync, gate, and commit.** Run `go test ./internal/currentstate ./internal/audit
  ./internal/project ./cmd/awf`, `./x sync`, `./x check`, `./x gate`; commit:

  ```commit
  feat(tooling): check atomic current-state transitions
  ```

## Coupled Phases 5 and Plan 4: Final runtime and real adopter cutover

This phase may be implemented and reviewed in Plan 3, but its closing production commit is coupled to
Plan 4's awf/Sundial cutover. There is no gate-clean, source-checkable repository state after deleting
the legacy engine but before migrating both adopters. Prepare docs/generated outputs and run
`./x sync && ./x check` before the switch; share the final closing commit with Plan 4 and run `./x gate`
after the complete cutover.

- [ ] **Task 5.1: Consume the sealed attestation.** Refactor Plan 2's journal into the permanent final-
  upgrade seam. Plain upgrade on an attested bridge verifies PreparedHead and relevant-tree digest,
  validates the complete new tree, journals all replacements/deletions, replaces the lock last, clears
  attestation, and promotes cutoff/gaps to permanent lock fields. Unattested/changed seals refuse.
  Journal recovery remains available. Delete migration-only inventory/readiness/snapshot adapter code
  from the current-state binary.

- [ ] **Task 5.2: Generate historical INDEX and topic-only domains.** Rename layout ActiveMd to
  DecisionIndex at `decisions/INDEX.md`; render Proposed/Accepted under In flight and Implemented/
  Abandoned under compact History. Remove supersession/currentness prose. Delete Decisions data from
  domain templates and render compact topic navigation only. Replace output-plan ACTIVE with INDEX;
  final upgrade owns legacy deletion already projected by Plan 2.

- [ ] **Task 5.3: Delete legacy authority.** Delete supersession coverage/citation/domain-index code,
  tag-tier context, Implemented-ADR invariant authority, legacy status derivation, ACTIVE generators,
  and migration inventory. Create `legacy_absence_test.go` denying their identifiers, desired paths,
  and imports; add import-boundary tests proving context/invariants do not import ADR authority.
  Deadcode remains the second removal proof.

- [ ] **Task 5.4: Prepare documentation and hand off without a standalone commit.** Update all authored
  architecture/domain/workflow/ADR/agent/reviewer docs, README, changelog, and templates; run
  `./x sync`, `./x check`, then apply runtime deletion. Run focused tests and `./x gate`. Do not commit
  or flip this plan yet: execute Plan 4's migration tasks, then close both plans in its final coupled
  commit.

## Verification

- Normal context and invariant reporting consume only topic claims; history requires explicit query.
- Static, staged, and range checks share snapshot-loaded corpora and enforce the full operation graph.
- INDEX is historical/navigation-only; domain docs contain topics, not ADR indexes.
- Final upgrade accepts only the unchanged sealed attestation and remains recoverable until lock commit.
- Source denylist, import boundaries, and deadcode prove the legacy authority engine is absent.
- No real awf/Sundial current-state content or release occurs before Plan 4.

## Notes

- Plan 4 owns the semantic curation and the shared closing commit for coupled Phase 5.
