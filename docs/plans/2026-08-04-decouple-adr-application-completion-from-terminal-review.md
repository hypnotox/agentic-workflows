---
format: plan-v2
date: 2026-08-04
adrs: [decouple-adr-application-completion-from-terminal-review]
status: Proposed
---
# Plan: Decouple ADR Application Completion from Terminal Review

## Goal

Make governed ADR State changes an unordered, exactly-once completion set that may become fully
Applied while the ADR remains Implementing, preserve chronological correction and merge ordering,
and defer only terminal status until review settles. Do not add a status, event kind, schema
migration, or audit-rule change.

## Architecture summary

`internal/adr` remains the semantic owner: history parsing validates declared membership and event
chronology, lifecycle validation owns status/cardinality, and operation projection derives
Applied/Remaining/Canceled progress. `internal/currentstate` continues to order distinct merge
occurrences by ADR identity and intra-ADR history while treating operation positions inside one event
as unordered membership. Phase 1 lands the parser, progress, pair-transition, workflow, and rendered
contract with the first three ADR operations in a deliberately out-of-declaration-order Applied
batch. Phase 2 proves and applies the retained merge-order contract as the final standalone batch,
leaving the ADR all-Applied and Implementing. Settled terminal review later appends only the
Implemented status event and flips this plan to Implemented.

## Phase 1: Unordered application and review-separated completion

**Execution mode: subagent-driven.**

Advances: ["merge-chronology"]
Completes: ["unordered-application", "workflow-guidance"]

### Task 1.1: Pin the broadened governed lifecycle with failing regressions
Latitude: exact
Applying: ["decouple-adr-application-completion-from-terminal-review:application-progress-independent-of-review", "decouple-adr-application-completion-from-terminal-review:declarations-are-an-unordered-completion-set", "decouple-adr-application-completion-from-terminal-review:event-chronology-remains-authoritative", "decouple-adr-application-completion-from-terminal-review:final-application-precedes-terminal-status", "decouple-adr-application-completion-from-terminal-review:correction-window-closes-at-terminal-status", "decouple-adr-application-completion-from-terminal-review:abandonment-still-cancels-work", "decouple-adr-application-completion-from-terminal-review:direct-terminal-shorthand-retained", "decouple-adr-application-completion-from-terminal-review:application-and-terminal-transactions-separated", "decouple-adr-application-completion-from-terminal-review:additive-history-compatibility"]
Paths: ["internal/adr/format_test.go", "internal/adr/corpus_test.go", "internal/adr/pending_test.go", "internal/currentstate/transition_test.go", "internal/currentstate/check_test.go", "internal/project/staged_test.go"]

Establish the subagent-driven phase baseline before editing: `git status --short` prints no paths,
`./x check` exits zero with clean drift and state, and `./x gate` exits zero with full coverage and no
dead code.

Change `TestParseV2LifecycleAndApplications`, `TestParseV2RejectsInvalidHistory`,
`TestHistoryTransitionValid`, `TestCorrectiveReapplication`,
`TestApplicationProjectionContracts`, `TestOperationProgressReapplied`,
`TestCheckPairV2IncrementalBatches`, and `TestIncrementalADRLifecyclePublicPairs` so they establish
all of these exact boundaries:

- a one-operation ADR may enter Implementing with that operation Applied and no Remaining member;
- a multi-operation Applied event may list declared operations in reverse or mixed declaration order,
  and later batches may first-apply an earlier declaration after a later declaration;
- every first application remains declared and exact-once, while an undeclared operation, a duplicate
  inside one event, and a second Applied occurrence remain errors;
- Reapplied still requires an earlier Applied occurrence, refuses remove, counts no second time in
  progress, and remains legal after all first applications while status is Implementing;
- an all-Applied Implementing projection reports no Remaining or Canceled operation;
- the final Applied batch and its matching claim mutation form a valid same-status pair, and a later
  status-only Implemented pair with byte-identical topics is valid;
- an explicit Implemented record with any Remaining operation and an all-Applied explicit Abandoned
  record remain invalid;
- legacy adjacent final-Applied-plus-Implemented histories and direct implicit Proposed/Accepted to
  Implemented histories remain valid unchanged.

In `TestIncrementalADRLifecyclePublicPairs`, replace the old coupled final transaction with three
observable steps: an out-of-declaration-order batch, a standalone batch that exhausts Remaining while
status stays Implementing, and a status-only Implemented commit with unchanged topic claims. Keep the
range-audit assertion so every pair is exercised through the public staged and historical paths.
Do not weaken invariant markers or assertions to obtain the initial failure. Run:

```sh
go test ./internal/adr ./internal/currentstate ./internal/project -run 'TestHistoryTransitionValid|TestParseV2LifecycleAndApplications|TestParseV2RejectsInvalidHistory|TestCorrectiveReapplication|TestApplicationProjectionContracts|TestOperationProgressReapplied|TestCheckPairV2IncrementalBatches|TestIncrementalADRLifecyclePublicPairs'
```

Before production edits, it must fail on the newly legal unordered, all-Applied Implementing, or
status-only transition expectations rather than on fixture syntax.

### Task 1.2: Broaden the single lifecycle model without weakening history
Applying: ["decouple-adr-application-completion-from-terminal-review:application-progress-independent-of-review", "decouple-adr-application-completion-from-terminal-review:declarations-are-an-unordered-completion-set", "decouple-adr-application-completion-from-terminal-review:event-chronology-remains-authoritative", "decouple-adr-application-completion-from-terminal-review:final-application-precedes-terminal-status", "decouple-adr-application-completion-from-terminal-review:correction-window-closes-at-terminal-status", "decouple-adr-application-completion-from-terminal-review:abandonment-still-cancels-work", "decouple-adr-application-completion-from-terminal-review:direct-terminal-shorthand-retained", "decouple-adr-application-completion-from-terminal-review:application-and-terminal-transactions-separated", "decouple-adr-application-completion-from-terminal-review:additive-history-compatibility"]
Paths: ["internal/adr/history.go", "internal/adr/format.go", "internal/adr/application.go", "internal/adr/operations.go"]

In `parseAppliedOperations`, retain exact grammar, declaration lookup, and within-event uniqueness but
remove only the strictly increasing declaration-position rejection. Do not sort the stored operation
list: retained event bytes and parsed membership stay in authored order even though that position has
no semantic ordering value.

In `validateV2History`, allow Implementing with any nonempty Applied set including all declarations,
remove the minimum-two-declaration constraint, and permit Amended and eligible Reapplied occurrences
after the last first application. Require explicit Implemented to have every declaration Applied but
not to be immediately adjacent to the last Applied event. Keep complete Abandoned invalid, direct
implicit completion unchanged, dates nondecreasing, prior history immutable, and first Applied
occurrences exact-once.

In `HistoryTransitionValid`, accept both the new status-only Implementing-to-Implemented append and
the previously valid adjacent Applied-plus-status append. Same-status Applied and Reapplied
transactions remain one event per authored pair. In `OperationProgress`, require nonempty Applied for
Implementing but permit an empty complement; continue projecting deterministic Applied and Remaining
results by declaration iteration rather than treating authored event-list position as presentation
order. Correct the duplicate-operation diagnostic in `operations.go` so it offers Reapplied while the
ADR remains Implementing, not only while another operation remains.

Run `gofmt -w internal/adr/history.go internal/adr/format.go internal/adr/application.go internal/adr/operations.go` and rerun Task 1.1's focused command; it must exit zero. Then run
`go test ./internal/adr ./internal/currentstate ./internal/project`; it must exit zero.

### Task 1.3: Apply lifecycle authority and replace choreography guidance
Kind: batch
Latitude: exact
Applying: ["decouple-adr-application-completion-from-terminal-review:application-progress-independent-of-review", "decouple-adr-application-completion-from-terminal-review:declarations-are-an-unordered-completion-set", "decouple-adr-application-completion-from-terminal-review:event-chronology-remains-authoritative", "decouple-adr-application-completion-from-terminal-review:final-application-precedes-terminal-status", "decouple-adr-application-completion-from-terminal-review:correction-window-closes-at-terminal-status", "decouple-adr-application-completion-from-terminal-review:abandonment-still-cancels-work", "decouple-adr-application-completion-from-terminal-review:direct-terminal-shorthand-retained", "decouple-adr-application-completion-from-terminal-review:application-and-terminal-transactions-separated", "decouple-adr-application-completion-from-terminal-review:additive-history-compatibility", "decouple-adr-application-completion-from-terminal-review:publication-safe-template-rendering"]
Paths: [".awf/topics/parts/adr-system/adr-lifecycle/current-state.md", ".awf/domains/parts/adr-system/current-state.md", ".awf/parts/working-with-awf/commands.md", ".awf/agents/plan-reviewer.yaml", ".awf/docs/pitfalls.yaml", "internal/catalog/standard.go", "internal/catalog/catalog_test.go", "templates/adr-readme/README.md.tmpl", "templates/adr-template/template.md.tmpl", "templates/skills/adr-lifecycle/SKILL.md.tmpl", "templates/skills/executing-plans/SKILL.md.tmpl", "templates/skills/subagent-driven-development/SKILL.md.tmpl", "templates/skills/reviewing-impl/SKILL.md.tmpl", "templates/skills/reviewing-adr/SKILL.md.tmpl", "templates/skills/proposing-adr/SKILL.md.tmpl", "templates/skills/writing-plans/SKILL.md.tmpl", "templates/skills/reviewing-plan-resync/SKILL.md.tmpl", "templates/agents/adr-reviewer.md.tmpl", "templates/agents/plan-reviewer.md.tmpl", "templates/agents/code-reviewer.md.tmpl", "internal/project/golden_test.go", "internal/project/target_test.go", "internal/project/example_wiring_test.go", "internal/project/guide_scopes_test.go", "internal/project/spine_test.go", "docs/decisions/decouple-adr-application-completion-from-terminal-review.md", "glob:.pi/skills/**", "glob:.pi/agents/**", "glob:.claude/skills/**", "glob:.claude/agents/**", "glob:docs/decisions/**", "glob:docs/domains/**", "glob:docs/topics/**", "docs/working-with-awf.md", "docs/pitfalls.md", "AGENTS.md", "glob:examples/sundial/**", ".awf/awf.lock"]
Representative: "Replace every rule that says Implementing requires both Applied and Remaining, batches must follow declaration order, Reapplied closes after the final first application, or terminal review owns the final Applied batch with the new set-membership, correction-until-terminal, implementation-owned-batch, and status-only-review contracts."
Edge: "Preserve the first Implementing status plus first Applied atomic pair, direct implicit completion, complete-Abandoned refusal, append-only event chronology, ascending ADR identity and intra-ADR history ordering, deterministic declaration-order presentation, pending/numbered terminology, and coherent generic output when template data is unset."
Post-check: "After `./x render`, `rg -n 'nonempty strict subset|at least one applied and one remaining|while another operation remains|corrections after the final Applied|final Applied batch.*Implemented|final batch and the Implemented flip|declaration-ordered operation-list' .awf templates internal/catalog .pi .claude docs AGENTS.md examples/sundial` returns no stale normative statement; retained historical ADRs and implemented plans may match only when a second path-scoped search explicitly limits results to `docs/decisions/[0-9]*` or completed `docs/plans/`. `./x check` exits zero, and publication-safety tests emit no unresolved-value or no-value token."

Update these three claim blocks in `.awf/topics/parts/adr-system/adr-lifecycle/current-state.md`,
preserving Origin and prior Revised-by entries, appending
`ADR-decouple-adr-application-completion-from-terminal-review`, retaining `Backing: test`, and making
the following substantive contracts explicit:

1. `adr-status-enum-and-matrix`: Implementing requires a nonempty Applied set and permits no
   Remaining member; Implemented requires complete application; status-only terminal and legacy
   adjacent terminal shapes are legal; direct implicit and abandonment rules stay unchanged.
2. `corrective-reapplication`: add/update corrections remain legal throughout Implementing,
   including after all first applications, require earlier Applied, count no progress, and use
   unordered membership while chronological event order remains.
3. `applied-history-events-append-only`: each Applied event is a nonempty duplicate-free subset of
   declared not-yet-applied operations in any order; each declaration is first-applied exactly once;
   event history remains an exact append-only prefix.

Keep the existing proof markers on the strengthened tests from Task 1.1. Update the ADR-system domain
narrative and every authored guidance surface in `Paths` so execution owns every explicit batch,
terminal review owns only the status flip, and reviewers distinguish unordered membership from
ordered history. Correct all three stale lifecycle pitfalls: terminal adjacency, the all-Applied
fixture workaround, and the lack of a nonterminal all-Applied state. Strengthen catalog and project
render tests to assert the new Implementing meaning, status-only terminal flow, unordered application,
retained Reapplied prerequisite, cross-target parity, Sundial parity, and empty-data publication
safety. Do not edit generated files directly.

After all ADR body edits are complete, move the pending ADR from Proposed to Implementing in this
same transaction. Change frontmatter `status:` to `Implementing`; append an Implementing status event
using the canonical digest reported by `./awf check` after temporarily placing exactly 64 lowercase
hex placeholder characters; then append this exact first Applied membership in the deliberately
out-of-declaration-order sequence shown:

```text
update `adr-system/adr-lifecycle:applied-history-events-append-only`, update `adr-system/adr-lifecycle:adr-status-enum-and-matrix`, update `adr-system/adr-lifecycle:corrective-reapplication`
```

The first history event and Applied event use the execution date. The fourth State changes operation
remains unapplied for Phase 2. Run `./x render`; inspect authored diffs and generated root and Sundial
outputs; run `gofmt -w internal/catalog/standard.go internal/catalog/catalog_test.go internal/project/golden_test.go internal/project/target_test.go internal/project/example_wiring_test.go internal/project/guide_scopes_test.go internal/project/spine_test.go`; run `go test ./internal/catalog ./internal/project`; then run `./x check` and `git diff --check`. Every command must exit zero, the index must list the ADR as Implementing, and context over the ADR path must report exactly the three named operations Applied and only `merge-transition-ordered-aggregate` Remaining.

### Phase close

Stage the complete Phase 1 transaction explicitly, including production, tests, three claim updates,
proofs, ADR history, authored guidance, every actual rendered diff, index, and lock. Run
`./awf check staged`; it must be clean and prove the three matching claim mutations in the same pair.
Run `./x gate`; it must exit zero with full coverage and no dead code. Create one commit:

```commit
feat(adr-system): decouple application choreography
```

## Phase 2: Preserve aggregate chronology and exhaust application

**Execution mode: subagent-driven.**

Completes: ["merge-chronology"]

### Task 2.1: Prove unordered membership inside ordered merge occurrences
Latitude: exact
Applying: ["decouple-adr-application-completion-from-terminal-review:event-chronology-remains-authoritative", "decouple-adr-application-completion-from-terminal-review:declarations-are-an-unordered-completion-set", "decouple-adr-application-completion-from-terminal-review:additive-history-compatibility"]
Paths: ["internal/currentstate/aggregate_test.go", "internal/currentstate/transition_test.go", "internal/currentstate/numbering_test.go"]

Establish the subagent-driven phase baseline before editing: `git status --short` prints no paths;
`./x check` and `./x gate` both exit zero; the ADR is Implementing with the three Phase 1 operations
Applied and only `invariants/current-state-authority:merge-transition-ordered-aggregate` Remaining.

Extend the invariant-backed merge aggregate coverage so one Applied batch contains operations in an
order different from State changes while distinct batches still reconcile in ascending ADR identity
and intra-ADR history order. Assert membership order neither reorders occurrences nor changes claim
chain semantics. Keep tests that reject mutated prior events, Reapplied-before-Applied, illegal
cross-ADR claim chains, and numbering-induced chronology changes. Do not sort event operation lists
or weaken retained-history byte equality. Run:

```sh
gofmt -w internal/currentstate/aggregate_test.go internal/currentstate/transition_test.go internal/currentstate/numbering_test.go
go test ./internal/currentstate -run 'Test.*Aggregate|Test.*Numbering|TestCheckPairV2IncrementalBatches'
```

Both commands must exit zero against the Phase 1 implementation.

### Task 2.2: Apply aggregate-order authority as a standalone final batch
Latitude: exact
Applying: ["decouple-adr-application-completion-from-terminal-review:event-chronology-remains-authoritative", "decouple-adr-application-completion-from-terminal-review:application-and-terminal-transactions-separated"]
Paths: [".awf/topics/parts/invariants/current-state-authority/current-state.md", ".awf/domains/parts/invariants/current-state.md", "docs/decisions/decouple-adr-application-completion-from-terminal-review.md", "docs/plans/2026-08-04-decouple-adr-application-completion-from-terminal-review.md", "docs/topics/invariants/current-state-authority.md", "docs/domains/invariants.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]

Update `invariants/current-state-authority:merge-transition-ordered-aggregate`, preserving Origin and
all prior Revised-by entries, appending
`ADR-decouple-adr-application-completion-from-terminal-review`, and retaining its existing proof
marker. State that aggregate reconciliation preserves distinct occurrences in ascending ADR identity
and intra-ADR history order while operation positions inside one Applied or Reapplied event are
unordered membership and create no chronology. Update the invariants domain narrative only where it
currently implies list-position ordering.

Append one Applied event to the ADR naming exactly:

```text
update `invariants/current-state-authority:merge-transition-ordered-aggregate`
```

Do not append Implemented and do not change ADR or plan frontmatter status. This transaction must
leave all four declarations Applied, Remaining empty, and the ADR still Implementing. Run
`./x render`, `go test ./internal/currentstate`, `./x check`, and `git diff --check`; each exits zero.
`./awf context docs/decisions/decouple-adr-application-completion-from-terminal-review.md` must show
all operations Applied with none Remaining while the ADR remains in flight. The generated index must
still list it under Implementing, proving terminal review is independent from application completion.

### Phase close

Stage the complete Phase 2 transaction explicitly: strengthened merge tests, claim/proof mutation,
domain narrative if changed, final Applied event, and actual rendered index/topic/domain/lock diffs.
Run `./awf check staged`; it must be clean and prove exactly the final claim mutation without a status
flip. Run `./x gate`; it must exit zero with full coverage and no dead code. Create one commit:

```commit
test(invariants): preserve merge batch chronology
```

## Definition of done

- `dod: unordered-application` Governed V2, V3, and V4 records accept first applications in any declaration order, require exact-once complete membership at Implemented, and retain every undeclared, duplicate, abandonment, direct-transition, and chronological-history guard.
- `dod: workflow-guidance` Authored and rendered root, multi-target, and Sundial guidance assigns explicit batches to implementation, terminal status to settled review, Reapplied correction until terminal, and publication-safe output under empty data.
- `dod: merge-chronology` Current-state aggregate reconciliation treats operation positions inside one event as unordered while preserving ascending ADR identity, intra-ADR history order, atomic claim mutation, and exact retained history; the linked ADR ends implementation all-Applied and Implementing.

## Notes

- The existing terminal `audit-domain-doc-staleness` behavior is intentionally unchanged. It remains
a branch-level advisory evaluated when the ADR reaches Implemented.
- After both phases receive settled report-only implementation review and any managed-worktree
integration review, the terminal-review flow appends only the ADR's Implemented status event with the
same canonical digest, flips this plan to Implemented, renders, checks, gates, and commits that
lifecycle transaction. It appends no Applied or Reapplied event and mutates no current-state claim.
