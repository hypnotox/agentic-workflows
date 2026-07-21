---
date: 2026-07-21
adrs: [143]
status: Implemented
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

- [ ] **Task 1.1: Define the V2 format and heterogeneous history parser.** In `internal/adr/adr.go`, add `CurrentStateV2` to `Format`, add `IsV2()`, change `ADR.History` from `[]StatusEntry` to `[]HistoryEvent`, and keep V1 behavior unchanged. In `internal/adr/history.go`, replace `StatusEntry` with these contract-bearing declarations:

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
  ```

  Do not add `IsGoverned`, application projection types, or projection methods in Phase 1; their first production consumers land with them in Phase 2 so this phase remains dead-code clean.

  In `internal/adr/history.go`, retain the V1 parser and add an exact V2 parser. Status lines admit Implementing only in V2. Applied lines must match `- YYYY-MM-DD: Applied; state-sequence: <positive integer>; operations: <verb> `<qualified-id>`[, ...]`; parse the operation list using the existing qualified-ID grammar, reject empty/duplicate/undeclared operations, and require declaration-relative order. Treat the most recent status event, not the final line, as lifecycle state. In `internal/adr/status.go` and `format.go`, make transition validation format-aware and implement every ADR-0143 items 2-7 history shape, digest, date, prefix, cardinality, and implicit/explicit exclusion. Thread both cutoffs through record parsing: below V1 is legacy, V1 through before V2 is V1, and at/above V2 is V2. A missing V2 cutoff means all governed records remain V1.

  Extend `internal/adr/format_test.go` and `internal/adr/adr_test.go` with table cases for every legal/illegal V2 status edge; Proposed/Accepted direct terminal shorthand; first/middle/final Applied shapes; partial Abandoned; `None.` and one-operation Implementing refusal; digest freeze/repeat; non-descending dates; exact metadata order; malformed separators/code spans/IDs; declaration-order violations; duplicates; mixed sequencing; prefix deletion or mutation; and V1 compatibility. Run `go test ./internal/adr`; it must pass with no V1 regression.

- [ ] **Task 1.2: Add dormant V2 lock storage without activating the cutoff.** In `internal/manifest/manifest.go`, add `ADRFormatV2From int `json:"adrFormatV2From,omitempty"`` and any private presence bit needed to distinguish absent from malformed zero. Canonical marshal order is `adrFormatV1From`, `adrFormatV2From`, `legacyAdrGaps`, then `initializedWithVersion`. A permanent schema-14 lock may omit V2. A schema-15 permanent lock must have positive `V1 <= V2`; both fields are immutable after V2 activation. Bridge authority must reject V2 metadata. Extend `AuthorityState`, parse, marshal, load/save, and error messages without changing pre-V2 serialized output.

  In `internal/project/project.go:syncReport`, preserve the new field exactly. Update `internal/manifest/manifest_test.go`, `internal/project/project_test.go`, and `cmd/awf/run_test.go` for absent round-trip, canonical V2 round-trip, invalid zero/negative/reversed boundaries, bridge mixing, sync preservation, and no pre-activation lock diff. Do not register schema 15 or edit `.awf/awf.lock` in this phase. Run `go test ./internal/manifest ./internal/project ./cmd/awf`; all pass and `git diff -- .awf/awf.lock` is empty.

- [ ] **Task 1.3: Verify and commit the dormant record model.** Run:

  ```sh
  gofmt -w internal/adr/adr.go internal/adr/format.go internal/adr/history.go internal/adr/operations.go internal/adr/status.go internal/adr/adr_test.go internal/adr/format_test.go internal/manifest/manifest.go internal/manifest/manifest_test.go internal/project/project.go internal/project/project_test.go cmd/awf/run_test.go
  git diff --check
  ./x check
  git add internal/adr/adr.go internal/adr/format.go internal/adr/history.go internal/adr/operations.go internal/adr/status.go internal/adr/adr_test.go internal/adr/format_test.go internal/manifest/manifest.go internal/manifest/manifest_test.go internal/project/project.go internal/project/project_test.go cmd/awf/run_test.go
  ./x check --staged
  ./x gate
  ```

  Every command must exit zero; checks and the gate must be clean. Commit:

  ```commit
  feat(adr-system): model v2 application history
  ```

## Phase 2: Generalize authority and pair validation by applied operation

- [ ] **Task 2.1: Add application projections, then make corpus history and topic provenance operation-specific.** Create `internal/adr/application.go` with these exact declarations and add `func (a ADR) IsGoverned() bool` in `internal/adr/adr.go`:

  ```go
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

  func (a ADR) ApplicationBatches() ([]ApplicationBatch, error)
  func (a ADR) OperationProgress() (OperationProgress, error)
  func (c Corpus) OperationProgress(number string) (OperationProgress, bool, error)
  ```

  V1 and direct V2 Implemented records with operations project one implicit batch from the terminal sequence. Explicit V2 events project one batch per Applied event. `None.` projects empty slices. Proposed and Accepted project every declaration as Remaining. Implementing projects recorded operations as Applied and the declaration-order complement as Remaining. Implemented rejects any remainder. Abandoned projects recorded operations as Applied and the complement as Canceled. Never infer an applied remove from claim absence. All returned slices, including nested batch operation slices, are defensive copies in declaration or event order. The corpus method returns `(zero, false, nil)` for a missing ADR, `(zero, true, err)` for an invalid present ADR projection, and `(progress, true, nil)` on success.

  In `internal/adr/corpus.go`, change `ClaimOperationHistory` to consume `ADR.ApplicationBatches()` and attach each operation's inherited sequence. Include applied operations from Implementing, Implemented, and partially Abandoned V2 ADRs; exclude Remaining and Canceled operations. Keep `OperationRecord.Status` so history can present the owning lifecycle state. Add corpus queries for declared/applied/remaining/canceled progress by ADR number so callers never read `ADR.Sections` or re-derive status semantics. Preserve fresh-slice behavior and legacy removed baselines.

  In `internal/topic/corpus.go`, replace terminal-Implemented provenance checks with exact inverse applied-operation checks through the corpus. An Origin requires an applied add; each Revised-by requires an applied update; a canceled or merely remaining operation never authorizes provenance. Preserve the legacy bootstrap exemption and ADR-number identity rules. In `internal/topic/query.go`, let topic history show an Applied operation immediately with its batch sequence regardless of Implementing, Implemented, or partially Abandoned owner status.

  Extend `internal/adr/corpus_test.go` and `internal/topic/corpus_test.go`/`query_test.go` for interleaved sequences, partial abandonment, pending/canceled exclusion, immediate topic history, legacy baselines, and defensive returned-slice mutation. Run `go test ./internal/adr ./internal/topic`; it must pass.

- [ ] **Task 2.2: Project static current-state truth from batches.** In `internal/currentstate/check.go`, replace `filterV1`, `mutationADRs`, and terminal-sequence assumptions with governed-record application projections. Validate one global contiguous namespace containing V1 implicit, V2 implicit, and V2 explicit batches. Within one claim history, compare operation batch sequences. Applied add/update/remove operations must have their required result and inverse provenance; Remaining operations retain the current pending preconditions; Canceled operations impose neither result nor pending precondition. An applied remove, including one owned by partially Abandoned V2, enters removed-ID history and prevents reuse. Preserve update substance, Origin stability, and Revised-by prefix/order rules.

  Extend `internal/currentstate/check_test.go` with valid and invalid matrices for all lifecycle states, interleaved ADR batches, multiple operations sharing a sequence, sequence gaps/duplicates, pending preconditions, canceled operations, partially Abandoned adds/updates/removes, inverse provenance, removed-ID no-reuse, and V1 fixtures. Run `go test ./internal/currentstate`; it must pass.

- [ ] **Task 2.3: Diff newly appended batches in `CheckPair`.** In `internal/currentstate/transition.go`, validate V2 history deltas before claim reconciliation. Permit only: status plus first batch when entering Implementing; one batch while remaining Implementing; final batch plus terminal status when reaching Implemented; terminal status only when reaching Abandoned; and existing direct implicit transitions. Any deletion or mutation of a prior event fails. Derive `pairOps` from batches present after but absent before. Allow at most one new batch per ADR. Allow several ADR batches in one pair only when every target ID is distinct; require their explicit sequences to be the next consecutive values. Reject same-claim cross-batch operations because no intermediate snapshot exists.

  Feed the resulting flattened operations into the existing add/update/remove matcher. Retain substantive update comparison, exact prior Revised-by prefix, one appended owner, preserved Origin, unmatched-operation diagnostics, and unmatched-mutation diagnostics. Errors must name ADR, operation, expected next sequence/event shape, or claim ID as applicable.

  Expand `internal/currentstate/transition_test.go` for direct V1/V2, first/middle/final/abandonment pairs, multi-operation batch, multiple disjoint ADR batches, same-ADR duplicate batch, cross-batch duplicate target, sequence ordering, every unmatched mutation direction, reverse-event deletion, and status/event order. Run `go test ./internal/currentstate`; it must pass.

- [ ] **Task 2.4: Verify and commit batch authority.** Run:

  ```sh
  gofmt -w internal/adr/adr.go internal/adr/application.go internal/adr/corpus.go internal/adr/corpus_test.go internal/topic/corpus.go internal/topic/corpus_test.go internal/topic/query.go internal/topic/query_test.go internal/currentstate/check.go internal/currentstate/check_test.go internal/currentstate/transition.go internal/currentstate/transition_test.go
  git diff --check
  ./x check
  git add internal/adr/adr.go internal/adr/application.go internal/adr/corpus.go internal/adr/corpus_test.go internal/topic/corpus.go internal/topic/corpus_test.go internal/topic/query.go internal/topic/query_test.go internal/currentstate/check.go internal/currentstate/check_test.go internal/currentstate/transition.go internal/currentstate/transition_test.go
  ./x check --staged
  ./x gate
  ```

  Every command must exit zero; checks and the gate must be clean. Commit:

  ```commit
  feat(invariants): validate applied adr operation batches
  ```

## Phase 3: Thread V2 boundaries through project loading and presentation

- [ ] **Task 3.1: Use both cutoffs in working, staged, and audit snapshots.** In `internal/adr/format.go`, add the exact value type `type FormatBoundaries struct { V1From int; V2From int }`; zero V2 means no V2 region. Change the router to `func ParseRecord(name string, data []byte, boundaries FormatBoundaries) (ADR, error)`. In `internal/currentstate/load.go`, change the loader to `func LoadFromTree(tree *snapshot.Tree, cfg *config.Config, boundaries adr.FormatBoundaries, gaps []int) (Loaded, error)` and its private `adrsFromTree` analogously. In `internal/currentstate/check.go` and `internal/currentstate/transition.go`, remove only the obsolete cutoff parameters, retaining the established input types: `func Check(records []adr.ADR, topics []topic.Topic) []Finding` and `func CheckPair(before, after Universe) []Finding`. Update all production callers in `internal/project/topics.go` and `internal/project/currentstate.go`, plus direct callers in `internal/currentstate/check_test.go` and `internal/currentstate/transition_test.go`; parsed records already carry their format.

  In `internal/project/currentstate.go`, replace each single `Cutoff int` snapshot field with `Boundaries adr.FormatBoundaries`, constructed only from the snapshot's lock, and carry it through `workingState`, Git tree loading, staged HEAD/index loading, and range audit. Extend `validatePermanentLockTransition` to reject V2 cutoff deletion or mutation; leave only the explicit schema-15 upgrade edge for Phase 4 to enable. Audit must continue comparing each included commit to its first parent through the same `CheckPair` call.

  In `internal/audit/audit.go`, replace `IsV1()` filters with `IsGoverned()` and recognize Implementing status/index co-changes without independently parsing lifecycle semantics. Update `internal/currentstate/load_test.go`, `internal/project/staged_test.go`, `internal/project/currentstate_test.go`, `internal/project/audit_inputs_test.go`, and `internal/audit/audit_test.go` for mixed boundaries, staged parity, first-parent range parity, immutable cutoff rejection, and V1-only live-lock compatibility. Run `go test ./internal/currentstate ./internal/project ./internal/audit`; it must pass.

- [ ] **Task 3.2: Present Implementing progress without making ADR prose current authority.** In `internal/adr/index.go`, include Implementing in In flight. In `internal/project/context.go`, replace `PendingChange` with:

  ```go
  type PendingChange struct {
      ADR      string `json:"adr"`
      Title    string `json:"title"`
      Status   string `json:"status"`
      Applied  int    `json:"applied"`
      Declared int    `json:"declared"`
      Op       string `json:"op"`
      Claim    string `json:"claim"`
  }
  ```

  Emit one row per Remaining operation, sorted by ADR number then claim ID. Accepted rows use `Status: "Accepted"`, `Applied: 0`, and the full declared count. Implementing rows use `Status: "Implementing"`, the applied count, and the full declared count. Implemented and Abandoned emit no rows. In `cmd/awf/context.go`, change the heading literally to `## Pending changes (not yet current)`. Render an Accepted row as `  ADR-0002 (Title; Accepted) update alpha/one:rule` and an Implementing row as `  ADR-0003 (Title; Implementing; 1/3 applied) remove alpha/one:old-rule`. JSON uses the struct fields above without omission. Current claims remain independent, and applied operations are never repeated as pending guidance. Use corpus progress queries rather than direct status literals or section reads. Do not implement explicit ADR-path artifact recognition.

  Update `internal/adr/index_test.go`, `internal/project/context_test.go`, and `cmd/awf/context_test.go` with exact human and JSON golden output for Accepted, first/middle Implementing progress, Implemented, and partially Abandoned states. Confirm current claims remain in their ordinary section. Run `go test ./internal/adr ./internal/project ./cmd/awf`; it must pass.

- [ ] **Task 3.3: Make scaffolding format-aware but keep V2 inactive.** Change `internal/adr.NewFile` to take an explicit `Format` and replace only the frontmatter format marker in the rendered scaffold; it must not infer format from free-form template prose. Change `internal/project.Project.NewADR` to load the lock, compute the next ADR number, and select V2 only when a positive V2 cutoff exists and the next identity is at or above it; otherwise select V1. Keep the current repository lock without a V2 cutoff, so normal scaffolding remains V1 until Phase 4.

  Update `internal/adr/adr_test.go`, `internal/project/project_test.go`, and `cmd/awf/new_test.go` for explicit V1/V2 selection, no overwrite, number/heading parity, marker stripping, and cutoff boundary cases. Run `go test ./internal/adr ./internal/project ./cmd/awf`; it must pass.

- [ ] **Task 3.4: Verify and commit project integration.** Run:

  ```sh
  gofmt -w internal/adr/adr.go internal/adr/format.go internal/adr/index.go internal/adr/index_test.go internal/adr/adr_test.go internal/currentstate/check.go internal/currentstate/check_test.go internal/currentstate/load.go internal/currentstate/load_test.go internal/currentstate/transition.go internal/currentstate/transition_test.go internal/project/context.go internal/project/context_test.go internal/project/currentstate.go internal/project/currentstate_test.go internal/project/staged_test.go internal/project/audit_inputs_test.go internal/project/project.go internal/project/project_test.go internal/project/topics.go internal/project/topics_test.go internal/audit/audit.go internal/audit/audit_test.go cmd/awf/context.go cmd/awf/context_test.go cmd/awf/new_test.go
  git diff --check
  ./x check
  test -z "$(git diff --name-only -- .awf/awf.lock docs/decisions/template.md)"
  git add internal/adr/adr.go internal/adr/format.go internal/adr/index.go internal/adr/index_test.go internal/adr/adr_test.go internal/currentstate/check.go internal/currentstate/check_test.go internal/currentstate/load.go internal/currentstate/load_test.go internal/currentstate/transition.go internal/currentstate/transition_test.go internal/project/context.go internal/project/context_test.go internal/project/currentstate.go internal/project/currentstate_test.go internal/project/staged_test.go internal/project/audit_inputs_test.go internal/project/project.go internal/project/project_test.go internal/project/topics.go internal/project/topics_test.go internal/audit/audit.go internal/audit/audit_test.go cmd/awf/context.go cmd/awf/context_test.go cmd/awf/new_test.go
  ./x check --staged
  ./x gate
  ```

  Every command must exit zero; the empty-diff assertion confirms activation has not occurred. Commit:

  ```commit
  feat(tooling): expose incremental adr progress
  ```

## Phase 4: Activate schema 15 and publish the V2 contract

Phases 4.1 through 4.5 are one coupled transaction and share one closing commit. Do not commit an intermediate task: registering schema 15 makes the repository lock stale; the schema and cutoff must be one save; V2 scaffolding cannot precede the cutoff; and V1 ADR-0143 requires all claim operations, proofs, docs, and its terminal transition together.

- [ ] **Task 4.1: Implement the atomic schema-15 cutoff migration and first adoption.** Create `internal/migrate/adrformatv2.go` with `type lockSaver func(*manifest.Lock, string) error`, `func applyADRFormatV2Cutoff(root string, out io.Writer) error`, and private `applyADRFormatV2CutoffWithSave(root string, out io.Writer, save lockSaver) error`. The registry wrapper passes a saver that calls `Lock.Save`; tests inject a failing saver and compare pre/post lock bytes.

  Add `OwnsSchemaStamp bool` to `internal/migrate.Migration` and register generation 15 exactly as `{To: 15, Name: "adr-format-v2-cutoff", Apply: applyADRFormatV2Cutoff, OwnsSchemaStamp: true}`. Change `Upgrade` so it calls `stampLockSchema` only when the highest applied migration does not own the schema stamp. For an existing V1 adopter, the generation-15 Apply path computes highest existing ADR identity plus one through an ADR corpus query, preserves `ADRFormatV1From` and `LegacyADRGaps`, assigns `ADRFormatV2From`, `SchemaVersion = 15`, and current `AWFVersion` in memory, then invokes its saver exactly once. `Upgrade` must not save again. Re-running at generation 15 is a no-op. Any scan, validation, or save failure leaves original lock bytes unchanged. Do not rewrite ADR files.

  For first adoption in `internal/project.InitAuthority`/`InitializeReport` and `cmd/awf/init.go`, empty projects set both cutoffs to 1; brownfield projects set both to highest identity plus one and retain every lower gap in `LegacyADRGaps`. Extend `internal/adr.AdoptionBoundary` or add a corpus-owned query so migrations do not add a third raw-reader exemption. A V2 permanent lock requires `V1 <= V2`; the allowed staged transition is exactly schema 14 without V2 to schema 15 with the computed V2 cutoff and otherwise identical permanent authority. Later V2 cutoff changes fail.

  Bump `internal/project.Version` to `0.20.0`, add schema 15 minimum version `0.20.0`, and update `internal/project/version_test.go`. Add `internal/migrate/adrformatv2_test.go` and update `internal/migrate/migrate_test.go`, `internal/manifest/manifest_test.go`, `internal/project/staged_test.go`, `cmd/awf/initrender_test.go`, `cmd/awf/run_test.go`, and `cmd/awf/upgrade_test.go` for empty/brownfield/existing V1, gaps, idempotence, one-save atomic result, injected failure preserving bytes, illegal cutoff mutation, and schema/version gating.

- [ ] **Task 4.2: Upgrade this repository, upgrade the example adopter, and activate V2 scaffolding.** Run `go build -o /tmp/awf-v2 ./cmd/awf`, `/tmp/awf-v2 upgrade`, and `(cd examples/sundial && /tmp/awf-v2 upgrade)`, in that order; each command must exit zero. The root upgrade must preserve every historical ADR byte and write schema 15 plus `adrFormatV2From` equal to the next ADR identity in one `.awf/awf.lock` result. The example upgrade must write its schema-15 lock before root `./x sync` invokes checks over the example. Verify with `git diff -- docs/decisions/0*.md` that no historical ADR changed except the intentional ADR-0143 transition performed in Task 4.5. Update `templates/adr-template/template.md.tmpl` and `.awf/parts/adr-template/frontmatter.md` so the selected V2 scaffold renders `format: current-state-v2`; preserve coherent V1 fixture scaffolding through the explicit `Format` API rather than a second authored template.

- [ ] **Task 4.3: Publish lifecycle, workflow, and project documentation.** Update these authored sources and the named semantics, without rewriting historical ADRs or plans:

  - `templates/adr-readme/README.md.tmpl`, `.awf/parts/adr-readme/index.md`: two cutoffs, V2 statuses, exact Applied grammar, digest/event rules, partial abandonment, batch sequencing, In flight membership, and forward correction.
  - `templates/skills/proposing-adr/SKILL.md.tmpl`, `templates/skills/adr-lifecycle/SKILL.md.tmpl`, `templates/skills/executing-plans/SKILL.md.tmpl`, `templates/skills/subagent-driven-development/SKILL.md.tmpl`, `templates/skills/reviewing-adr/SKILL.md.tmpl`, `templates/skills/reviewing-impl/SKILL.md.tmpl`, `templates/skills/writing-plans/SKILL.md.tmpl`, `templates/skills/reviewing-plan/SKILL.md.tmpl`, `templates/skills/reviewing-plan-resync/SKILL.md.tmpl`, and `templates/skills/retrospective/SKILL.md.tmpl`: V2 scaffolding and exact first/middle/final batch transactions; planning, review, and retrospective treatment of Applied/Remaining/Canceled operations; no instruction that all operations wait for one final commit.
  - `templates/agents/adr-reviewer.md.tmpl`, `templates/agents/plan-reviewer.md.tmpl`, `templates/agents/code-reviewer.md.tmpl`: review declared/applied/remaining/canceled partitions, pair atomicity, sequence order, and current-claim truth.
  - `templates/agents-doc/AGENTS.md.tmpl`: workflow permits one ADR's operations across individually checked commits and states stable-history forward correction. No file under `.awf/parts/agents-doc/` currently overrides that workflow section, so do not edit the unrelated `awf-setup.md`, `commands.md`, `identity.md`, or `you-and-this-project.md` parts.
  - `internal/catalog/standard.go`: replace `adrStates` with this exact ordered value:

    ```go
    []any{
        map[string]any{"name": "Proposed", "meaning": "ADR is written and under review; content is freely mutable", "mutability": "Freely mutable; body and status may both change"},
        map[string]any{"name": "Accepted", "meaning": "Design is finalised; implementation authorised but not yet started", "mutability": "Status and append-only Status history only; the body is frozen; a schema retrofit may migrate the encoding"},
        map[string]any{"name": "Implementing", "meaning": "Design is frozen; a nonempty strict subset of declared operations is applied", "mutability": "Status and append-only Status history only; Applied events may append while operations remain"},
        map[string]any{"name": "Implemented", "meaning": "All declared claim operations are applied", "mutability": "Terminal; status and append-only Status history only; the body is frozen; a schema retrofit may migrate the encoding"},
        map[string]any{"name": "Abandoned", "meaning": "Execution stopped; applied operations remain historical and unapplied operations are canceled", "mutability": "Terminal; status and append-only Status history only; the final entry carries a rationale; the body is frozen"},
    }
    ```
  - `internal/configspec/spec.go`: change the `skills/adr-lifecycle` `adrStates` description literally to `The decision-record lifecycle states (list of {name, meaning, mutability}) the skill's state table renders; the default is the five-state current-state-v2 lifecycle.` Do not add the lock cutoffs as config keys: `internal/configspec` describes authored configuration, while `.awf/domains/parts/config/current-state.md` owns the permanent lock prose.
  - `.awf/domains/parts/adr-system/current-state.md`, `.awf/domains/parts/config/current-state.md`, `.awf/domains/parts/tooling/current-state.md`, `.awf/docs/parts/architecture/data-flow.md`, `.awf/docs/glossary.yaml`, `templates/docs/working-with-awf.md.tmpl`, and `README.md`: current architecture, dual-cutoff first adoption and migration, context progress, audit pair semantics, and terminology.

  Keep every template missing-key-zero safe. Update `internal/project/golden_test.go`, `internal/project/spine_test.go`, `internal/project/catalog_sweep_test.go`, `internal/project/frontmatter_test.go`, `internal/catalog/catalog_test.go`, `internal/configspec/spec_test.go`, and `cmd/awf/initrender_test.go` with representative V2 data and empty-data cases; empty rendering must contain no unresolved token, empty interpolation, or V1-only claim about new scaffolds.

- [ ] **Task 4.4: Apply ADR-0143's nine current-state operations and proofs.** Edit the three authored topic parts with the literal blocks below. Preserve their surrounding claim order; for updated claims, replace the complete existing block. Do not run sync yet because provenance cannot cite ADR-0143 until Task 4.5 completes its terminal transition.

  In `.awf/topics/parts/adr-system/adr-lifecycle/current-state.md` use:

  ```md
  ### `invariant: fresh-adoption-v1-cutoff`

  Empty first adoption seals both ADR format cutoffs at 1; brownfield first adoption seals both at the highest existing identity plus one with every lower gap explicit; upgrading an existing V1 adopter preserves its V1 cutoff and seals the V2 cutoff at the highest identity plus one; every ADR is legacy below V1, V1 from that cutoff to before V2, and V2 at or above the V2 cutoff.
  Origin: ADR-0139
  Revised-by: ADR-0143
  Backing: test

  ### `invariant: adr-status-enum-and-matrix`

  Every governed ADR is routed by the two immutable format cutoffs: V1 retains its four statuses and five legal edges, while V2 recognizes Proposed, Accepted, Implementing, Implemented, and Abandoned and accepts only the format-specific status, history-event, digest, and application-cardinality transitions.
  Origin: ADR-0135
  Revised-by: ADR-0143
  Backing: test

  ### `invariant: applied-history-events-append-only`

  Stable V2 Status history is prefix-append-only: each Applied event records one nonempty, declaration-ordered batch of previously unapplied operations with one positive state sequence, and a checked pair refuses deletion or mutation of any prior event.
  Origin: ADR-0143
  Backing: test
  ```

  In `.awf/topics/parts/config/migrations-and-locks/current-state.md` add:

  ```md
  ### `invariant: adr-v2-cutoff-atomic-immutable`

  Schema-15 upgrade writes the schema generation and computed ADR V2 cutoff in one atomic lock save without rewriting authored ADRs, and every later staged transition preserves both permanent format cutoffs exactly.
  Origin: ADR-0143
  Backing: test
  ```

  In `.awf/topics/parts/invariants/current-state-authority/current-state.md` use:

  ```md
  ### `invariant: abandoned-remove-pair-attributed`

  An applied V2 remove continues to attribute claim absence after its ADR becomes Abandoned, while a remaining operation canceled by abandonment never attributes absence; snapshot-pair validation proves the Applied event and actual removal together.
  Origin: ADR-0139
  Revised-by: ADR-0143
  Backing: test

  ### `invariant: implemented-impact-bidirectional`

  Every applied governed state operation has its required current or removed result, and every active claim Origin or revision has the inverse applied ADR operation; Remaining and Canceled operations provide no authority.
  Origin: ADR-0135
  Revised-by: ADR-0143
  Backing: test

  ### `invariant: removed-claim-id-not-reused`

  Once any applied remove records a qualified claim ID, no later add may reuse it, regardless of whether the removing ADR later ends Implemented or partially Abandoned.
  Origin: ADR-0135
  Revised-by: ADR-0143
  Backing: test

  ### `invariant: state-impact-transition-atomic`

  Every newly appended application batch and exactly its matching claim mutations occur in one HEAD-to-index transaction; staged validation refuses an operation record or mutation split across snapshot pairs.
  Origin: ADR-0135
  Revised-by: ADR-0143
  Backing: test

  ### `invariant: application-batch-sequence-order`

  V1 implicit batches and V2 implicit or explicit batches share one unique contiguous global state-sequence namespace, and every applied claim operation inherits its batch sequence for provenance and history ordering.
  Origin: ADR-0143
  Backing: test
  ```

  Add or retain exact `// invariant: <qualified-id>` proof markers on these files: first-adoption integration in `cmd/awf/initrender_test.go`; V2 lifecycle and append-only history in `internal/adr/format_test.go`; atomic immutable cutoff in `internal/migrate/adrformatv2_test.go`; abandonment, bidirectional authority, no-reuse, and sequence tests in `internal/currentstate/check_test.go`; batch-pair atomicity in `internal/currentstate/transition_test.go`. Remove the four obsolete `Verify:` lines converted to test backing. Leave the authored and generated tree unsynced only until Task 4.5 supplies valid ADR provenance.

- [ ] **Task 4.5: Complete the V1 bootstrap transaction, sync once, and commit activation.** Change ADR-0143 frontmatter to `status: Implemented`, change this plan to `status: Implemented`, and append a grammar-valid probe entry with 64 zero hex characters and sequence 1:

  ```md
  - 2026-07-21: Implemented; content-sha256: 0000000000000000000000000000000000000000000000000000000000000000; state-sequence: 1
  ```

  Run `./x check`; it must exit nonzero and report the exact expected content digest for ADR-0143. Replace only the zero digest with that reported digest. Run `./x check` again; it must exit nonzero and report the exact next global state sequence for ADR-0143. Replace only sequence 1 with that reported value. If either run fails first on a different diagnostic, fix that structural error without changing the frozen ADR body, rerun the same probe step, and require the named digest/sequence diagnostic before proceeding.

  Before sync and the final gate, add `TestIncrementalADRLifecyclePublicPairs` to `internal/project/staged_test.go`. Build one `internal/testsupport/gitfixture` repository and assert through the public staged/range project seams that a V1 direct transaction passes unchanged; a V2 ADR enters Implementing with a strict subset, appends an interleaved middle batch, reaches Implemented with the remainder, and appears In flight until terminal; a partially Abandoned ADR retains applied provenance and cancels remaining operations; deleting an Applied event with the inverse mutation fails; and a correct endpoint never rescues a bad intermediate pair.

  Now run `./x sync` exactly once. It must regenerate INDEX.md, lock hashes, domains/topics/docs, target copies, AGENTS.md, and example outputs with valid Implemented provenance. Run these exact formatting and focused verification commands; each must exit zero:

  ```sh
  gofmt -w internal/adr/adr.go internal/adr/format.go internal/adr/history.go internal/adr/index.go internal/adr/corpus.go internal/adr/adr_test.go internal/adr/format_test.go internal/adr/index_test.go internal/adr/corpus_test.go internal/manifest/manifest.go internal/manifest/manifest_test.go internal/migrate/migrate.go internal/migrate/adrformatv2.go internal/migrate/migrate_test.go internal/migrate/adrformatv2_test.go internal/currentstate/check.go internal/currentstate/transition.go internal/currentstate/check_test.go internal/currentstate/transition_test.go internal/topic/corpus.go internal/topic/query.go internal/topic/corpus_test.go internal/topic/query_test.go internal/project/project.go internal/project/currentstate.go internal/project/context.go internal/project/staged_test.go internal/project/project_test.go internal/project/context_test.go internal/project/golden_test.go internal/project/spine_test.go internal/project/catalog_sweep_test.go internal/project/frontmatter_test.go internal/project/version_test.go internal/audit/audit.go internal/audit/audit_test.go internal/catalog/standard.go internal/catalog/catalog_test.go internal/configspec/spec.go internal/configspec/spec_test.go cmd/awf/init.go cmd/awf/upgrade.go cmd/awf/initrender_test.go cmd/awf/run_test.go cmd/awf/upgrade_test.go
  go test ./internal/adr ./internal/currentstate ./internal/topic ./internal/manifest ./internal/migrate ./internal/project ./internal/audit ./cmd/awf
  go test ./internal/project -run '^TestIncrementalADRLifecyclePublicPairs$'
  git diff --check
  ./x check
  ```

  Inspect `git status --short` and abort if any path is unrelated to Tasks 4.1-4.5. Stage the two new migration files explicitly, then stage the reviewed tracked change set with the command-derived exact set; this is not `git add -A` and cannot include untracked residue:

  ```sh
  git add internal/migrate/adrformatv2.go internal/migrate/adrformatv2_test.go
  git diff --name-only -z | xargs -0 git add --
  git diff --cached --check
  ./x check --staged
  ./x gate
  ```

  The staged check and gate must be clean, with 100% statement coverage, no dead code, no drift, no prose-gate findings, and no staged transition finding. Confirm `git diff --cached -- docs/decisions/0*.md` contains only ADR-0143 among historical decision files. Commit the coupled activation:

  ```commit
  feat(adr-system): activate incremental operation batches
  ```

## Verification

- Run `go test ./internal/adr ./internal/currentstate ./internal/topic ./internal/manifest ./internal/migrate ./internal/project ./internal/audit ./cmd/awf`; all packages pass.
- Run `./x check`; it reports clean drift and current-state validation.
- Run `./x invariants`; every test-backed claim resolves to at least one proof and no unbacked claim has a proof.
- Run `./x gate full`; all full-tier checks finish clean.
- Rerun `go test ./internal/project -run '^TestIncrementalADRLifecyclePublicPairs$'`; it exits zero and the already-committed public-pair integration test covers every intermediate and reverse pair.
- Run `git status --short`; it is empty after the final commit.

## Notes

- Explicit ADR-path recognition and its Applied/Remaining/Canceled presentation remain owned by the paused context-topic agent UX effort. This plan exposes the corpus seam and normal pending-progress output only.
- Historical V1 ADRs, completed plans, and V1-era changelog prose remain untouched.
- Before stable publication, branch-local amend, squash, and rebase remain valid. After publication, correction uses a successor ADR rather than deleting Applied history.
