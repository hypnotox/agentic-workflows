---
date: 2026-07-21
adrs: [143]
status: Proposed
---
# Plan: Incremental ADR State Operation Application

## Goal

Implement ADR-0143 so V2 ADRs can apply frozen State changes in append-only, pair-checked batches across multiple commits while V1 and legacy history remain valid. The explicit non-goals are reversible Applied events, endpoint-only audit proof, and the later explicit ADR-path context presentation.

## Architecture summary

Build the V2 event and operation-projection model behind an absent cutoff first, then generalize static and pairwise authority checking and project presentation while the feature remains dormant. The final coupled activation transaction registers schema generation 15, atomically upgrades the repository lock, switches scaffolding to V2, publishes workflow guidance, applies all ADR-0143 claim operations and proof markers, and freezes the ADR and plan. This final coupling is mandatory because migration registration makes the checked-in schema-14 lock stale, V2 scaffolding requires a live cutoff, and the current V1 handshake requires all nine claim mutations to accompany ADR-0143's direct Implemented transition.

## File structure

- **Created:** `internal/adr/application.go`, `internal/migrate/adrformatv2.go`, `internal/migrate/adrformatv2_test.go`.
- **Modified:** `internal/adr/adr.go`, `internal/adr/application.go`, `internal/adr/corpus.go`, `internal/adr/format.go`, `internal/adr/history.go`, `internal/adr/index.go`, `internal/adr/operations.go`, `internal/adr/status.go`, and their tests; `internal/manifest/manifest.go` and tests; `internal/currentstate/check.go`, `load.go`, `transition.go`, and tests; `internal/topic/corpus.go`, `query.go`, and tests; `internal/project/context.go`, `currentstate.go`, `project.go`, `scaffold.go`, staged/project/context tests; `internal/audit/audit.go` and tests; `internal/migrate/migrate.go` and tests; `cmd/awf/init.go`, `upgrade.go`, run/init/upgrade tests; `internal/catalog/standard.go`; `internal/configspec/spec.go`; ADR, skill, reviewer, agent-guide, architecture, glossary, working-with-awf, README, current-state, and example-adopter authored sources listed in Phase 4; generated files and locks emitted by `./x sync`; `docs/decisions/0143-incremental-adr-state-operation-application.md`; this plan.
- **Deleted:** none.

## Phase 1: Add the dormant V2 record and lock model

- [ ] **Task 1.1: Define the V2 format, heterogeneous history, and application projection.** In `internal/adr/adr.go`, add `CurrentStateV2` to `Format`, add `IsV2()` and `IsGoverned()` predicates, change `ADR.History` from `[]StatusEntry` to `[]HistoryEvent`, and keep V1 behavior unchanged. In new `internal/adr/application.go`, define these contract-bearing declarations:

  ```go
  type HistoryEventKind uint8
  const (
      HistoryStatus HistoryEventKind = iota + 1
      HistoryApplied
  )

  type HistoryEvent struct {
      Kind        HistoryEventKind
      Date        string
      Status      string
      Digest      string
      Sequence    int
      HasSequence bool
      Rationale   string
      Operations  []Operation
  }

  type ApplicationBatch struct {
      Sequence   int
      Operations []Operation
      Implicit   bool
  }

  type AppliedOperation struct {
      Operation Operation
      Sequence  int
  }

  type OperationProgress struct {
      Applied   []AppliedOperation
      Remaining []Operation
      Canceled  []Operation
  }
  ```

  Add `func (a ADR) ApplicationBatches() ([]ApplicationBatch, error)` and `func (a ADR) OperationProgress() (OperationProgress, error)`. Returned slices must be fresh copies. V1 and direct V2 Implemented records with operations project one implicit batch from the terminal sequence. Explicit V2 events project one batch per Applied event. `None.` projects empty slices. Proposed and Accepted project every declaration as Remaining. Implementing projects recorded operations as Applied and the declaration-order complement as Remaining. Implemented rejects any remainder. Abandoned projects recorded operations as Applied and the complement as Canceled. Never infer an applied remove from claim absence.

  In `internal/adr/history.go`, retain the V1 parser and add an exact V2 parser. Status lines admit Implementing only in V2. Applied lines must match `- YYYY-MM-DD: Applied; state-sequence: <positive integer>; operations: <verb> `<qualified-id>`[, ...]`; parse the operation list using the existing qualified-ID grammar, reject empty/duplicate/undeclared operations, and require declaration-relative order. Treat the most recent status event, not the final line, as lifecycle state. In `internal/adr/status.go` and `format.go`, make transition validation format-aware and implement every ADR-0143 items 2-7 history shape, digest, date, prefix, cardinality, and implicit/explicit exclusion. Thread both cutoffs through record parsing: below V1 is legacy, V1 through before V2 is V1, and at/above V2 is V2. A missing V2 cutoff means all governed records remain V1.

  Extend `internal/adr/format_test.go`, `history` tests, `operations` tests, and `adr` tests with table cases for every legal/illegal V2 status edge; Proposed/Accepted direct terminal shorthand; first/middle/final Applied shapes; partial Abandoned; `None.` and one-operation Implementing refusal; digest freeze/repeat; non-descending dates; exact metadata order; malformed separators/code spans/IDs; declaration-order violations; duplicates; mixed sequencing; prefix deletion or mutation; and V1 compatibility. Run `go test ./internal/adr`; it must pass with no V1 regression.

- [ ] **Task 1.2: Add dormant V2 lock storage without activating the cutoff.** In `internal/manifest/manifest.go`, add `ADRFormatV2From int `json:"adrFormatV2From,omitempty"`` and any private presence bit needed to distinguish absent from malformed zero. Canonical marshal order is `adrFormatV1From`, `adrFormatV2From`, `legacyAdrGaps`, then `initializedWithVersion`. A permanent schema-14 lock may omit V2. A schema-15 permanent lock must have positive `V1 <= V2`; both fields are immutable after V2 activation. Bridge authority must reject V2 metadata. Extend `AuthorityState`, parse, marshal, load/save, and error messages without changing pre-V2 serialized output.

  In `internal/project/project.go:syncReport`, preserve the new field exactly. Update `internal/manifest/manifest_test.go`, `internal/project/project_test.go`, and `cmd/awf/run_test.go` for absent round-trip, canonical V2 round-trip, invalid zero/negative/reversed boundaries, bridge mixing, sync preservation, and no pre-activation lock diff. Do not register schema 15 or edit `.awf/awf.lock` in this phase. Run `go test ./internal/manifest ./internal/project ./cmd/awf`; all pass and `git diff -- .awf/awf.lock` is empty.

- [ ] **Task 1.3: Verify and commit the dormant record model.** Run `gofmt -w` on changed Go files, `git diff --check`, `./x check`, and `./x gate`; all must finish clean. Stage only Phase 1 files and commit:

  ```commit
  feat(adr-system): model v2 application history
  ```

## Phase 2: Generalize authority and pair validation by applied operation

- [ ] **Task 2.1: Make corpus history and topic provenance operation-specific.** In `internal/adr/corpus.go`, change `ClaimOperationHistory` to consume `ADR.ApplicationBatches()` and attach each operation's inherited sequence. Include applied operations from Implementing, Implemented, and partially Abandoned V2 ADRs; exclude Remaining and Canceled operations. Keep `OperationRecord.Status` so history can present the owning lifecycle state. Add corpus queries for declared/applied/remaining/canceled progress by ADR number so callers never read `ADR.Sections` or re-derive status semantics. Preserve fresh-slice behavior and legacy removed baselines.

  In `internal/topic/corpus.go`, replace terminal-Implemented provenance checks with exact inverse applied-operation checks through the corpus. An Origin requires an applied add; each Revised-by requires an applied update; a canceled or merely remaining operation never authorizes provenance. Preserve the legacy bootstrap exemption and ADR-number identity rules. In `internal/topic/query.go`, let topic history show an Applied operation immediately with its batch sequence regardless of Implementing, Implemented, or partially Abandoned owner status.

  Extend `internal/adr/corpus_test.go` and `internal/topic/corpus_test.go`/`query_test.go` for interleaved sequences, partial abandonment, pending/canceled exclusion, immediate topic history, legacy baselines, and defensive returned-slice mutation. Run `go test ./internal/adr ./internal/topic`; it must pass.

- [ ] **Task 2.2: Project static current-state truth from batches.** In `internal/currentstate/check.go`, replace `filterV1`, `mutationADRs`, and terminal-sequence assumptions with governed-record application projections. Validate one global contiguous namespace containing V1 implicit, V2 implicit, and V2 explicit batches. Within one claim history, compare operation batch sequences. Applied add/update/remove operations must have their required result and inverse provenance; Remaining operations retain the current pending preconditions; Canceled operations impose neither result nor pending precondition. An applied remove, including one owned by partially Abandoned V2, enters removed-ID history and prevents reuse. Preserve update substance, Origin stability, and Revised-by prefix/order rules.

  Extend `internal/currentstate/check_test.go` with valid and invalid matrices for all lifecycle states, interleaved ADR batches, multiple operations sharing a sequence, sequence gaps/duplicates, pending preconditions, canceled operations, partially Abandoned adds/updates/removes, inverse provenance, removed-ID no-reuse, and V1 fixtures. Run `go test ./internal/currentstate`; it must pass.

- [ ] **Task 2.3: Diff newly appended batches in `CheckPair`.** In `internal/currentstate/transition.go`, validate V2 history deltas before claim reconciliation. Permit only: status plus first batch when entering Implementing; one batch while remaining Implementing; final batch plus terminal status when reaching Implemented; terminal status only when reaching Abandoned; and existing direct implicit transitions. Any deletion or mutation of a prior event fails. Derive `pairOps` from batches present after but absent before. Allow at most one new batch per ADR. Allow several ADR batches in one pair only when every target ID is distinct; require their explicit sequences to be the next consecutive values. Reject same-claim cross-batch operations because no intermediate snapshot exists.

  Feed the resulting flattened operations into the existing add/update/remove matcher. Retain substantive update comparison, exact prior Revised-by prefix, one appended owner, preserved Origin, unmatched-operation diagnostics, and unmatched-mutation diagnostics. Errors must name ADR, operation, expected next sequence/event shape, or claim ID as applicable.

  Expand `internal/currentstate/transition_test.go` for direct V1/V2, first/middle/final/abandonment pairs, multi-operation batch, multiple disjoint ADR batches, same-ADR duplicate batch, cross-batch duplicate target, sequence ordering, every unmatched mutation direction, reverse-event deletion, and status/event order. Run `go test ./internal/currentstate`; it must pass.

- [ ] **Task 2.4: Verify and commit batch authority.** Run `gofmt -w` on changed files, `git diff --check`, `./x check`, and `./x gate`; all must finish clean. Stage only Phase 2 files and commit:

  ```commit
  feat(invariants): validate applied adr operation batches
  ```

## Phase 3: Thread V2 boundaries through project loading and presentation

- [ ] **Task 3.1: Use both cutoffs in working, staged, and audit snapshots.** In `internal/currentstate/load.go`, replace the single cutoff argument with a boundary value containing V1 and optional V2 cutoffs, and route mixed legacy/V1/V2 records deterministically. In `internal/project/currentstate.go`, carry both boundaries through `workingState`, Git tree loading, staged HEAD/index loading, and range audit. Extend `validatePermanentLockTransition` to reject V2 cutoff deletion or mutation; leave only the explicit schema-15 upgrade edge for Phase 4 to enable. Audit must continue comparing each included commit to its first parent through the same `CheckPair` call.

  In `internal/audit/audit.go`, replace `IsV1()` filters with `IsGoverned()` and recognize Implementing status/index co-changes without independently parsing lifecycle semantics. Update `internal/currentstate/load_test.go`, `internal/project/staged_test.go`, `internal/project/currentstate_test.go`, `internal/project/audit_inputs_test.go`, and `internal/audit/audit_test.go` for mixed boundaries, staged parity, first-parent range parity, immutable cutoff rejection, and V1-only live-lock compatibility. Run `go test ./internal/currentstate ./internal/project ./internal/audit`; it must pass.

- [ ] **Task 3.2: Present Implementing progress without making ADR prose current authority.** In `internal/adr/index.go`, include Implementing in In flight. In `internal/project/context.go`, keep Accepted output unchanged; for Implementing emit only Remaining operations and a concise `applied/declared` progress label, while current claims independently show applied truth. Implemented and Abandoned emit no pending notice. Use corpus progress queries rather than direct status literals or section reads. Do not implement explicit ADR-path artifact recognition; only preserve/export the Applied/Remaining/Canceled and sequence seam required by that follow-up.

  Update `internal/adr/index_test.go` and `internal/project/context_test.go` with exact golden output for Accepted, first/middle Implementing progress, Implemented, and partially Abandoned states. Confirm current claims remain in their ordinary section and applied operations are not duplicated as pending guidance. Run `go test ./internal/adr ./internal/project`; it must pass.

- [ ] **Task 3.3: Make scaffolding format-aware but keep V2 inactive.** Change `internal/adr.NewFile` to take an explicit `Format` and replace only the frontmatter format marker in the rendered scaffold; it must not infer format from free-form template prose. Change `internal/project.Project.NewADR` to load the lock, compute the next ADR number, and select V2 only when a positive V2 cutoff exists and the next identity is at or above it; otherwise select V1. Keep the current repository lock without a V2 cutoff, so normal scaffolding remains V1 until Phase 4.

  Update `internal/adr/adr_test.go`, `internal/project/project_test.go`, and `cmd/awf/new_test.go` for explicit V1/V2 selection, no overwrite, number/heading parity, marker stripping, and cutoff boundary cases. Run `go test ./internal/adr ./internal/project ./cmd/awf`; it must pass.

- [ ] **Task 3.4: Verify and commit project integration.** Run `gofmt -w` on changed files, `git diff --check`, `./x check`, and `./x gate`; all must finish clean. Confirm `git diff -- .awf/awf.lock docs/decisions/template.md` is empty so activation has not occurred. Stage only Phase 3 files and commit:

  ```commit
  feat(tooling): expose incremental adr progress
  ```

## Phase 4: Activate schema 15 and publish the V2 contract

Phases 4.1 through 4.5 are one coupled transaction and share one closing commit. Do not commit an intermediate task: registering schema 15 makes the repository lock stale; the schema and cutoff must be one save; V2 scaffolding cannot precede the cutoff; and V1 ADR-0143 requires all claim operations, proofs, docs, and its terminal transition together.

- [ ] **Task 4.1: Implement the atomic schema-15 cutoff migration and first adoption.** Create `internal/migrate/adrformatv2.go` with a generation-15 migration registered in `internal/migrate/migrate.go` as `adr-format-v2-cutoff`. For an existing permanent V1 adopter, compute highest existing ADR identity plus one through an ADR corpus query, preserve `ADRFormatV1From` and `LegacyADRGaps`, set `ADRFormatV2From`, `SchemaVersion = 15`, and the current `AWFVersion`, then call `Lock.Save` exactly once. Re-running at generation 15 is a no-op. Any scan/validation/save failure leaves the original lock bytes unchanged. Do not rewrite ADR files.

  For first adoption in `internal/project.InitAuthority`/`InitializeReport` and `cmd/awf/init.go`, empty projects set both cutoffs to 1; brownfield projects set both to highest identity plus one and retain every lower gap in `LegacyADRGaps`. Extend `internal/adr.AdoptionBoundary` or add a corpus-owned query so migrations do not add a third raw-reader exemption. A V2 permanent lock requires `V1 <= V2`; the allowed staged transition is exactly schema 14 without V2 to schema 15 with the computed V2 cutoff and otherwise identical permanent authority. Later V2 cutoff changes fail.

  Bump `internal/project.Version` to `0.20.0`, add schema 15 minimum version `0.20.0`, and update `internal/project/version_test.go`. Add `internal/migrate/adrformatv2_test.go` plus migration, manifest, staged, init, upgrade, and run tests for empty/brownfield/existing V1, gaps, idempotence, one-save atomic result, injected failure preserving bytes, illegal cutoff mutation, and schema/version gating.

- [ ] **Task 4.2: Upgrade this repository and activate V2 scaffolding.** Run `go run ./cmd/awf upgrade`. It must complete successfully, preserve every historical ADR byte, and write schema 15 plus `adrFormatV2From` equal to the next ADR identity in one `.awf/awf.lock` result. Verify with `git diff -- docs/decisions/0*.md` that no historical ADR changed except the intentional ADR-0143 transition performed in Task 4.5. Update `templates/adr-template/template.md.tmpl` and `.awf/parts/adr-template/frontmatter.md` so the selected V2 scaffold renders `format: current-state-v2`; preserve coherent V1 fixture scaffolding through the explicit `Format` API rather than a second authored template.

- [ ] **Task 4.3: Publish lifecycle, workflow, and project documentation.** Update these authored sources and the named semantics, without rewriting historical ADRs or plans:

  - `templates/adr-readme/README.md.tmpl`, `.awf/parts/adr-readme/index.md`: two cutoffs, V2 statuses, exact Applied grammar, digest/event rules, partial abandonment, batch sequencing, In flight membership, and forward correction.
  - `templates/skills/proposing-adr/SKILL.md.tmpl`, `adr-lifecycle/SKILL.md.tmpl`, `executing-plans/SKILL.md.tmpl`, `subagent-driven-development/SKILL.md.tmpl`, `reviewing-adr/SKILL.md.tmpl`, `reviewing-impl/SKILL.md.tmpl`: V2 scaffolding and exact first/middle/final batch transactions; no instruction that all operations wait for one final commit.
  - `templates/agents/adr-reviewer.md.tmpl`, `plan-reviewer.md.tmpl`, `code-reviewer.md.tmpl`: review declared/applied/remaining/canceled partitions, pair atomicity, sequence order, and current-claim truth.
  - `templates/agents-doc/AGENTS.md.tmpl` and applicable `.awf/parts/agents-doc/` sources: workflow permits one ADR's operations across individually checked commits and states stable-history forward correction.
  - `internal/catalog/standard.go`: five lifecycle state rows and V2 meanings. `internal/configspec/spec.go`: describe the V2 default and two-cutoff lock metadata.
  - `.awf/domains/parts/adr-system/current-state.md`, `.awf/domains/parts/tooling/current-state.md`, `.awf/docs/parts/architecture/data-flow.md`, `.awf/docs/glossary.yaml`, `templates/docs/working-with-awf.md.tmpl`, and `README.md`: current architecture, context progress, audit pair semantics, and terminology.

  Keep every template missing-key-zero safe. Update template golden/spine/catalog/configspec tests with representative V2 data and empty-data cases; empty rendering must contain no unresolved token, empty interpolation, or V1-only claim about new scaffolds.

- [ ] **Task 4.4: Apply ADR-0143's nine current-state operations and proofs.** Edit only authored topic parts, then sync. Apply these exact claim contracts, preserving each update's Origin and prior Revised-by prefix and appending `ADR-0143`:

  - `.awf/topics/parts/adr-system/adr-lifecycle/current-state.md`:
    - `fresh-adoption-v1-cutoff`: state the empty/brownfield dual-cutoff first-adoption rules, existing-V1 upgrade partition, explicit lower legacy gaps, and V2 requirement at/above its cutoff; set `Backing: test`.
    - `adr-status-enum-and-matrix`: state format-routed V1 compatibility and the five-status V2 matrix with exact history/cardinality validation; set `Backing: test`.
    - add `applied-history-events-append-only` with `Origin: ADR-0143`, stating that stable V2 history is prefix-append-only, each Applied event is one declaration-ordered nonempty batch with one sequence, and reverse deletion is refused; `Backing: test`.
  - `.awf/topics/parts/config/migrations-and-locks/current-state.md`: add `adr-v2-cutoff-atomic-immutable` with `Origin: ADR-0143`, stating that schema-15 upgrade atomically writes schema and computed V2 cutoff without authored rewrites and no later transition can change/remove either cutoff; `Backing: test`.
  - `.awf/topics/parts/invariants/current-state-authority/current-state.md`:
    - `abandoned-remove-pair-attributed`: distinguish an applied V2 remove, which continues to attribute absence after partial abandonment, from a canceled remove, which never does; `Backing: test`.
    - `implemented-impact-bidirectional`: broaden to every applied governed operation and explicitly deny authority to Remaining/Canceled operations; `Backing: test`.
    - `removed-claim-id-not-reused`: trigger on any applied remove regardless of owner terminal status; `Backing: test`.
    - `state-impact-transition-atomic`: require each newly appended batch and exactly its mutations in one HEAD-to-index pair; `Backing: test`.
    - add `application-batch-sequence-order` with `Origin: ADR-0143`, requiring one unique contiguous global sequence across implicit and explicit batches and per-operation history ordering by inherited batch sequence; `Backing: test`.

  Add or retain exact `// invariant: <qualified-id>` proof markers on the expanded tests: first-adoption integration in `cmd/awf/initrender_test.go`; V2 lifecycle and append-only history in `internal/adr/format_test.go`; atomic immutable cutoff in `internal/migrate/adrformatv2_test.go`; abandonment, bidirectional authority, no-reuse, and sequence tests in `internal/currentstate/check_test.go`; batch-pair atomicity in `internal/currentstate/transition_test.go`. Remove obsolete `Verify:` lines when converting claims to test backing. Run `./x sync`; generated domains/topics/docs/runtime copies/example outputs and `.awf/awf.lock` must reach a clean `./x check` fixpoint.

- [ ] **Task 4.5: Complete the V1 bootstrap transaction and commit activation.** Change ADR-0143 frontmatter to `status: Implemented`. Run `./x check` once to obtain the exact expected content digest and next global state sequence. Append one V1 terminal history entry in the exact form `- 2026-07-21: Implemented; content-sha256: <reported-digest>; state-sequence: <reported-sequence>`. Change this plan to `status: Implemented`. Run `./x sync` again so INDEX.md, lock hashes, generated docs, target copies, and examples include the final states.

  Stage the complete activation transaction explicitly, never with `git add -A`. Run `git diff --cached --check`, `./x check --staged`, and `./x gate`; each must finish clean, with 100% statement coverage, no dead code, no drift, no prose-gate findings, and no staged transition finding. Confirm `git diff --cached -- docs/decisions/0*.md` contains only ADR-0143 among historical decision files. Commit the coupled activation:

  ```commit
  feat(adr-system): activate incremental operation batches
  ```

## Verification

- Run `go test ./internal/adr ./internal/currentstate ./internal/topic ./internal/manifest ./internal/migrate ./internal/project ./internal/audit ./cmd/awf`; all packages pass.
- Run `./x check`; it reports clean drift and current-state validation.
- Run `./x invariants`; every test-backed claim resolves to at least one proof and no unbacked claim has a proof.
- Run `./x gate full`; all full-tier checks finish clean.
- In a temporary Git fixture, prove these end states through the public project/check seams: a V1 direct transaction passes unchanged; a V2 ADR enters Implementing with a strict subset, appends an interleaved middle batch, reaches Implemented with the remainder, and appears In flight until terminal; a partially Abandoned ADR retains applied provenance and cancels remaining operations; deleting an Applied event with the inverse mutation fails; endpoint-only correctness never rescues a bad intermediate pair.
- Run `git status --short`; it is empty after the final commit.

## Notes

- Explicit ADR-path recognition and its Applied/Remaining/Canceled presentation remain owned by the paused context-topic agent UX effort. This plan exposes the corpus seam and normal pending-progress output only.
- Historical V1 ADRs, completed plans, and V1-era changelog prose remain untouched.
- Before stable publication, branch-local amend, squash, and rebase remain valid. After publication, correction uses a successor ADR rather than deleting Applied history.
