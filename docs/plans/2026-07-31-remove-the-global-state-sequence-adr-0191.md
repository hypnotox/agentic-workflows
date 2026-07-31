---
date: 2026-07-31
adrs: [0191]
status: Proposed
---
# Plan: Remove the global state sequence (ADR-0191)

## Goal

Implement ADR-0191: remove the repository-global contiguous `state-sequence` namespace, order
per-claim provenance by ascending final ADR number, make an applied remove an absorbing tombstone,
union `Revised-by` commutatively across merges, and retrofit the corpus with one ordered schema
migration (generation 27). Non-goals: the merge-time ADR-numbering design (a separate effort), any
edit to historical ADR Decision prose or to dated `docs/plans/*.md` files that mention the retired
concept, and any change to `content-sha256` digest mechanics.

## Architecture summary

Backing symmetry forces the shape: the tests that back the sequence claims cannot change in a
commit that leaves those claims untouched, so the behaviour commit is itself an application
transaction. Phase 1 flips ADR-0191 to Accepted. Phase 2 is the flag day: one transaction carrying
the grammar and validation rewrite, the presentation-field removals, migration 27 plus its run over
this repository, the documentation-source sweep with re-render, the test sweep with relocated proof
markers, the claim mutations for eight of the nine declared operations, and ADR-0191's Implementing
status plus first Applied event. Phase 3 is the deferred final transaction after terminal review:
the ninth operation (`update-requires-substance`), the final Applied event, and the Implemented
flip. Ordering inside phase 2 runs bottom-up: `internal/adr` (parse and project), then
`internal/currentstate` (validate), then presentation (`internal/topic`, `internal/project`,
`cmd/awf`), then `internal/migrate`, then the corpus run, then doc sources and render, then claims,
markers, and ADR events.

## File structure

- **Created:**
  - `internal/migrate/adrnumberprovenance.go` (migration 27)
  - `internal/migrate/adrnumberprovenance_test.go`
- **Modified:**
  - `internal/adr/history.go`, `internal/adr/application.go`, `internal/adr/corpus.go`,
    `internal/adr/format.go` and their `_test.go` files
  - `internal/currentstate/check.go`, `internal/currentstate/transition.go`,
    `internal/currentstate/check_test.go`, `internal/currentstate/transition_test.go`,
    `internal/currentstate/aggregate_test.go`
  - `internal/topic/query.go`, `internal/topic/query_test.go`, `internal/topic/corpus_test.go`
  - `internal/project/project.go`, `internal/project/version_test.go`,
    `internal/project/context_adr_test.go`
  - `internal/project/context_adr.go`, `internal/project/context_paths.go`, and the
    `internal/project` tests that assert sequence output (`context_paths_test.go`,
    `golden_test.go`, `topics_test.go`, `staged_test.go`, `mergeaggregate_test.go`)
  - `cmd/awf/topic.go`, `cmd/awf/context.go`, `cmd/awf/topic_test.go`, `cmd/awf/context_test.go`,
    `cmd/awf/initrender_test.go`
  - `internal/migrate/migrate.go` (registry entry)
  - `templates/skills/adr-lifecycle/SKILL.md.tmpl`, `templates/adr-readme/README.md.tmpl`,
    `templates/adr-template/template.md.tmpl`, `templates/agents/plan-reviewer.md.tmpl`,
    `templates/agents/adr-reviewer.md.tmpl`, `templates/agents/code-reviewer.md.tmpl`,
    `templates/skills/reviewing-plan/SKILL.md.tmpl`,
    `templates/skills/reviewing-impl/SKILL.md.tmpl`,
    `templates/skills/reviewing-plan-resync/SKILL.md.tmpl`, `.awf/docs/glossary.yaml` (rendered
    `docs/glossary.md` and the reviewer agent and skill outputs follow via `./x render`)
  - `.awf/agents/plan-reviewer.yaml`, `.awf/domains/parts/adr-system/current-state.md`,
    `.awf/docs/pitfalls.yaml`
  - `.awf/topics/parts/invariants/current-state-authority/current-state.md`,
    `.awf/topics/parts/adr-system/adr-lifecycle/current-state.md`
  - `.awf/topics/parts/rendering/pi-workflows/current-state.md` and
    `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md` (their `Revised-by`
    lines rewritten by the migration run, not by hand) and their rendered
    `docs/topics/rendering/pi-workflows.md` and
    `docs/topics/rendering/workflow-skill-templates.md`
  - `changelog/CHANGELOG.md`, `.awf/awf.lock`, `examples/sundial/.awf/awf.lock`
  - `docs/decisions/0191-replace-the-global-state-sequence-with-adr-number-provenance-order.md`
    (status events only)
  - Every governed `docs/decisions/*.md` status-history line carrying `state-sequence` (rewritten
    by the migration run, not by hand)
  - All rendered outputs of the sources above (`docs/decisions/README.md`,
    `docs/decisions/template.md`, `docs/decisions/INDEX.md`, `docs/domains/adr-system.md`,
    `docs/pitfalls.md`, `docs/topics/invariants/current-state-authority.md`,
    `docs/topics/adr-system/adr-lifecycle.md`, `.claude/` and `.pi/` agent and skill copies, and
    the `examples/sundial` copies), all via `./x render`
- **Deleted:** none.

## Phase 1: Accept ADR-0191

**Execution mode: inline.**

- [ ] **Task 1.1: Flip ADR-0191 to Accepted.** In
  `docs/decisions/0191-replace-the-global-state-sequence-with-adr-number-provenance-order.md`,
  change frontmatter `status: Proposed` to `status: Accepted` and append to `## Status history`:
  `- <today>: Accepted; content-sha256: <stamp>`. The stamp establishes the first digest: write 64
  zeros, run `./awf check state`, copy the computed digest from the finding, and fix the line
  (this probe remains the documented method for digests). Run `./x render` so
  `docs/decisions/INDEX.md` reflects the flip. Expected terminal state: `./awf check` reports
  clean.
- [ ] **Phase-close: stage, check, gate, and commit.** Stage the ADR file,
  `docs/decisions/INDEX.md`, and `.awf/awf.lock`; run `awf check --staged` then `./x gate`;
  commit:

```commit
docs(adr): accept 0191 ADR-number provenance order
```

## Phase 2: Flag day: behaviour, migration, corpus, docs, and first application batch

**Execution mode: inline.** One transaction. Nothing in this phase commits separately; every task
lands in the single phase-closing commit. Tasks are ordered so the tree compiles at each step, but
only the final staged state must be green.

- [ ] **Task 2.1: Drop the sequence from the event grammar (`internal/adr/history.go`).**
  - `appliedHeadRe` (line 40) becomes
    `^- (\d{4}-\d{2}-\d{2}): Applied; (state-sequence: [1-9][0-9]*; )?operations: (.+)$`: the
    retired segment is tolerated and discarded (ADR-0191 item 1), so a pre-migration corpus still
    loads; the `parseHistory` Applied branch (lines 69-80) loses its `strconv.Atoi`, sets
    `LegacySequence: m[2] != ""`, and reads operation text from capture group 3 as today.
  - Replace `Sequence int` and `HasSequence bool` on `HistoryEvent` (lines 22-31) with one flag,
    `LegacySequence bool`, set true whenever a discarded segment was present (Applied head or
    tail); it carries no value and exists only so the check layer can report the retired segment.
  - In `parseHistoryTail` (lines 147-181), the `state-sequence: ` case (164-173) keeps its shape
    validation but stores nothing except `LegacySequence = true`; the `seenSeq` duplicate
    bookkeeping stays as-is so a duplicated segment still errors.
  - The remaining `Sequence`/`HasSequence` reads in `internal/adr` are dispositioned in task 2.4;
    this task only removes the fields and the parser writes to them.
- [ ] **Task 2.2: Sequence-free batches (`internal/adr/application.go`).** Delete `Sequence` from
  `ApplicationBatch` (lines 10-14) and from `AppliedOperation` (lines 17-20). In
  `ApplicationBatches()` (30-68): explicit batches keep their `HistoryApplied` event order; the
  implicit terminal branch (58-66) no longer requires `HasSequence`, and the error
  `"ADR-%s Implemented status has no state-sequence"` (62) is deleted; an implicit batch is now
  constructed from the terminal event's operations alone. In `OperationProgress()` (72-128): the
  guard at line 87 drops its `batch.Sequence < 1` conjunct and keeps the empty-batch conjunct;
  `AppliedOperation{Operation: op}` (98) carries no sequence. Batch identity for downstream
  consumers is the batch's index in the ADR's history.
- [ ] **Task 2.3: Order claim history by ADR number (`internal/adr/corpus.go`).** Delete
  `StateSequence` from `OperationRecord` (28-33) and rewrite its doc comment: ADR number orders
  implemented mutations. In `ClaimOperationHistory` (143-181), replace the sort at line 165 with
  ascending numeric ADR number,
  `sort.SliceStable(records, func(i, j int) bool { return numOf(records[i].record.Number) < numOf(records[j].record.Number) })`,
  where `numOf` is `strconv.Atoi` ignoring the error (numbers are validated four-digit at parse);
  records from one ADR keep event order via the stable sort. Update the `Raw` doc comment
  (183-193): three enumerated consumers, naming the new migration.
- [ ] **Task 2.4: Field-read closure in `internal/adr` (`format.go`, `application.go`).** Every
  remaining `Sequence`/`HasSequence` read is dispositioned here; each deletion removes the full
  enclosing if-statement, never just its return line, so braces stay balanced and no condition
  references a deleted field:
  - Delete the two mixed-mode guards as vacuous: `internal/adr/application.go:46-52` (the
    `mixes explicit Applied events with implicit terminal sequencing` error) and its V2 twin
    inside `validateV2History`, `format.go:403-405` only, keeping the outer
    `if event.Status == statusImplemented && explicit {` at 402 and the still-needed check at
    406-409.
  - Delete the whole if-statements around the six presence validators: in `validateV2History`
    the implicit-terminal requirements whose returns sit at lines 420 and 423; in
    `validateV2StatusEntry` the one at 457; in `validateHistoryEntry` (V1) the three at 482, 485,
    and 489. Presence is no longer status-dependent.
  - Drop the `x.Sequence`/`x.HasSequence` comparisons from `historiesEqual` (`format.go` around
    line 182); drop only the `HasSequence` conjunct, keeping the rationale and digest halves,
    from the first-entry Proposed-scaffold checks (`format.go:314` and `:342`) and from the
    accepted/implementing status-entry checks (`format.go:448` and `:474`).
  - Closure check, reachable here: `grep -n "HasSequence\|\.Sequence" internal/adr/*.go`
    (excluding `_test.go`) returns no production hit (`LegacySequence` does not match either
    pattern).
- [ ] **Task 2.5: Static checks: ADR-number order and the absorbing remove
  (`internal/currentstate/check.go`).**
  - Delete `checkSequences` (106-135) and its call at line 48. In its place add
    `checkLegacySegments`, called from `Check()` at the same spot: for every history event with
    `LegacySequence` true, emit the blocking finding
    `"ADR-%s carries a retired state-sequence segment; run awf upgrade"`. Post-migration this
    repository emits none; a pre-migration adopter corpus is loud at check time instead of
    failing to parse.
  - Replace `seq int` in `operationAt` (26-30) with `adrNum int` (owner's number) and
    `batchIdx int` (the batch's index within its ADR's history); every sort previously on `seq`
    (in `checkOperationHistory` line 150 and `retiredTopicOperations` line 254) becomes
    `(adrNum, batchIdx)` ascending.
  - In `checkOperationHistory` (137-177): keep the findings for more than one remove, more than
    one add, and history not beginning with an add; replace the
    `"claim %s has an operation after its remove"` finding (173) so only an add after a remove is
    a finding (reuse of a removed id); an update after a remove is legal dominated history and
    produces no finding.
  - `retiredTopicOperations` (238-277): the completeness test becomes "exactly one add first and
    exactly one remove, with only dominated updates permitted after the remove" instead of
    requiring the remove to be the last element, so a fully retired topic whose claim carries a
    dominated tail still classifies retired; task 2.12's tombstone tests cover this shape.
  - In `checkBackward` (336-388): replace the sequence seed (359-364) and loop (365-376) with
    ADR-number comparison. Seed `last` with the Origin ADR's number when the Origin operation
    exists; for each `Revised-by` entry require its number strictly greater than `last`, finding
    message `"claim %s Revised-by entries are not in ascending ADR-number order at ADR-%s"`;
    membership findings keep their existing text.
  - Forward direction (`checkForward`, `checkAppliedOp`, `removedSet`): the `wasRemoved`
    short-circuit already tolerates an applied update whose claim a sibling ADR removed; verify by
    test (task 2.12) that both integration orders pass the static check, and extend the
    short-circuit only if a test proves a gap.
- [ ] **Task 2.6: Transition checks: no expected-next, dominated chains, union extension
  (`internal/currentstate/transition.go`).**
  - `appendedBatch` (178-182): replace `sequence int` with `adrNum int` and `batchIdx int`. In
    `pairOps` (184-278): delete the `maxBefore` scan (190-204) and the expected-next validation
    loop with its finding (233-238, the closing brace of the loop opened at 233 included); the
    cross-ADR batch sort (232) becomes `(adrNum, batchIdx)` ascending; chain building (241-250)
    follows that order. The `AuthoredCommit` one-new-batch cap and duplicate-target rejection are
    unchanged.
  - `foldChain` (297-330): after a remove, further updates are legal and classified dominated
    (they join no `updaters` list and do not alter net effect); an add after a remove stays
    illegal. When the fold sees the remove it captures that step's ADR and returns it for the
    net-remove and net-noop results, never `chain[len(chain)-1]`, so absence stays attributed to
    the remove. Net effect: a chain containing a remove resolves net remove, or net no-op when it
    also begins with the chain's add, regardless of dominated updates after the remove. The fold
    stays pure over operations: a pure-update chain still folds to `adr.OpUpdate` with its
    `updaters`; domination is not decided here.
  - `checkMutations` (132-171) owns the dominated classification, because it holds both
    universes: when an update-verdict chain's claim is absent on both sides and a prior applied
    remove for that id exists in `after.ADRs` (a small helper derives the applied-remove set from
    the after corpus), the chain is dominated history and no mutation is expected; when the claim
    is absent with no such remove, the existing unmatched-mutation and update-target findings
    keep firing; finding when a dominated chain shows a mutation:
    `"claim %s has only dominated updates in this transition, so it must stay absent"`. The
    net-noop path (156-162) is unchanged.
  - `revisedByExtension` (374-396): replace exact-prefix-append with the union rule: the after
    list must equal the ascending duplicate-free union of the before list and the chain's
    updating ADRs; insertion below an existing higher number is legal; dominated updaters are
    excluded (their claim has no list). Keep a finding for any before entry that disappears and
    for any after entry that is neither carried over nor an updater.
- [ ] **Task 2.7: Presentation surfaces.** Delete `StateSequence` from `ADRHistory`
  (`internal/topic/query.go:39-44`) and its copy in `operationADR` (183). Delete
  `stateSequenceSuffix` (`cmd/awf/topic.go:128-133`) and its three call sites (91, 94, 97). In
  `cmd/awf/context.go`: delete only the `if op.StateSequence != 0 { ... }` guard (251-253),
  keeping the `fmt.Fprintln(out, "]")` terminator at 254; edit the removal-history Fprintf (260)
  to `"%s  Removal history: removed by ADR-%s\n"`, dropping the `at state-sequence %d` suffix and
  its argument. Delete `StateSequence` from `ADROperationContext`
  (`internal/project/context_adr.go:17-22`), the `state.sequence` plumbing in
  `projectADRArtifact` (54-81), and the `add(strconv.Itoa(op.StateSequence))` component in
  `contextGroupKey` (`internal/project/context_paths.go:420`); if that leaves `strconv` unused in
  `context_paths.go`, drop the import.
- [ ] **Task 2.8: Migration 27 (`internal/migrate/adrnumberprovenance.go`).** Follow the
  `supersessionkeys.go` shape: pure function `applyADRNumberProvenance(root string, out io.Writer) error`,
  registered in `migrate.go` as the new last entry
  `{To: 27, Name: "adr-number-provenance", Apply: applyADRNumberProvenance}`. Two passes:
  1. Over every corpus ADR via `corpus.Raw`: rewrite each line matching
     `^- \d{4}-\d{2}-\d{2}: Applied; state-sequence: [1-9][0-9]*; operations: ` by deleting the
     `state-sequence: <n>; ` segment, and delete the `; state-sequence: <n>` segment from any
     status-event tail. Only `## Status history` lines change; the digest-covered content is
     untouched, so existing `content-sha256` stamps remain valid (assert in the test).
  2. Over every `.awf/topics/parts/*/*/current-state.md` (plain glob read and write; these files
     are not the ADR corpus): rewrite each `Revised-by: ` line to its ascending duplicate-free
     ADR-number order. This deliberately reorders the two known inversions
     (`rendering/pi-workflows`, ADR-0166 after ADR-0167, and
     `rendering/workflow-skill-templates`, the same pair).
  Announce each rewritten file on `out`. The migration is idempotent (second run rewrites
  nothing) and fails loudly on a malformed Applied line it cannot rewrite.
  `adrnumberprovenance_test.go` mirrors `supersessionkeys_test.go`: fixture ADRs and topic parts,
  byte-exact expected output, digest stability, idempotency, and the malformed-input error.
  Registration closure: add `27: "0.30.0"` to `minVersionBySchema` in
  `internal/project/project.go` (0.29.0 is unreleased, so no `Version` bump); in
  `internal/project/version_test.go`, assert `minVersionBySchema[27] == Version` and move the
  unmapped-schema probe from generation 27 to 27. In `internal/adr/corpus_test.go`,
  `TestCorpusRawAccessEnumerated` gains `"internal/migrate/adrnumberprovenance.go": true` in its
  `want` map and its failure text becomes `the three single-call migration seams`.
- [ ] **Task 2.9: Run the migration on both in-repo trees.** From the worktree root run
  `./awf upgrade`; expected output includes `awf upgrade: applied adr-number-provenance` and the
  lock stamps `SchemaVersion` 27. Then run the same upgrade inside the example adopter,
  `(cd examples/sundial && go run ../.. upgrade)`, carrying its tree to generation 27 through a
  real upgrade rather than a hand edit (precedent: the generation-25 bump touched both locks);
  stage `examples/sundial/.awf/awf.lock` and, if the upgrade or the later render changes
  `examples/sundial/.awf/bootstrap.sh` or other sundial files, stage those too as `git status`
  reports them. Post-check, reachable at this position:
  `git grep -n "state-sequence" -- 'docs/decisions/[0-9]*.md' 'examples/sundial/docs/decisions/[0-9]*.md'`
  returns no line starting with a status-history event (no hit whose matched line begins
  `- <date>:`); the surviving hits are ADR Decision and Context prose (0135, 0143, 0182, 0183,
  0189 bodies). The rendered decisions README and template (root and sundial) and INDEX.md's
  permanent hit inside ADR-0191's own filename are out of this check's pathspec and are covered
  by task 2.10's render and the Verification section.
- [ ] **Task 2.10: Documentation-source sweep.** Edit the sources, never rendered outputs:
  - `templates/skills/adr-lifecycle/SKILL.md.tmpl:53`: grammar becomes
    `- YYYY-MM-DD: Applied; operations: <operation-list>`; delete
    `and use the next sequence \`awf check\` reports`; keep the stamp-repetition and Amended
    sentences.
  - `templates/adr-readme/README.md.tmpl:69-81`: line 75 grammar drops
    `state-sequence: <positive integer>; `; delete the sentence
    `V1 implicit and V2 implicit or explicit batches share one contiguous global sequence.` and
    the words `and terminal sequence` (78-79); add one sentence: `Per-claim provenance is ordered
    by ascending final ADR number.`
  - `templates/adr-template/template.md.tmpl:52-59`: delete `direct Implemented events also carry
    the batch state sequence` (53-54); example line 59 becomes
    `- YYYY-MM-DD: Applied; operations: update \`<domain>/<topic>:<slug>\``.
  - `.awf/agents/plan-reviewer.yaml` `v2-batch-partition-legality` description: keep the
    partition-legality half verbatim through `...exhausted all operations two phases before the
    flip`, ending that sentence with a period there, and follow it with exactly
    `Check partition legality per planned commit, and that no plan pre-computes a content-sha256
    stamp`; everything between (the hardcoded-literals clause, the repo-global sequence sentence,
    and the counter parenthetical) is deleted, including the joining comma.
  - `.awf/domains/parts/adr-system/current-state.md`: line 5's grammar drops
    `state-sequence: <positive integer>; `. Line 7 changes in the same edit: the staged-check
    update rule becomes `preserves the claim's \`Origin\`, unions its \`Revised-by\` with the
    updating ADR at its ascending position, and changes a canonical field`, and the static-check
    enumeration drops `sequence, `; the rest of line 7 stays verbatim.
  - `.awf/docs/pitfalls.yaml`, four entries: in `A V2 ADR's Implementing flip cannot commit
    alone`, drop the state-sequence halves of the two sentences (182, 190), keeping digest
    language. Delete the entries `Concurrent ADR application branches may require replay before
    integration` (209-239) and `An ADR's frozen digest and next state-sequence cannot be read
    directly` (1509-1526) outright; fold the digest-probe survival into a retitled entry
    `An ADR's frozen digest cannot be read directly` carrying only the digest half of the old
    body. In `A plan must not pre-compute check-named lifecycle values` (1330-1340), narrow the
    premise and example to `content-sha256` only. The line-number references above are indicative
    at authoring time; locate each entry by its title.
  - `changelog/CHANGELOG.md` `## [Unreleased]` `### Breaking changes`, new first bullet, exact
    text: `- Remove the repository-global \`state-sequence\` from ADR status history. Applied
    events use \`- <date>: Applied; operations: ...\`; per-claim provenance is ordered by
    ascending final ADR number; an applied remove is an absorbing tombstone. \`awf topic\` drops
    its \`[state-sequence: N]\` suffix, \`awf context\` its per-operation sequence annotations,
    and the \`stateSequence\` field leaves the \`awf topic --json\` contract. Schema generation 27
    strips the segments from every governed ADR and canonicalizes every \`Revised-by\` list to
    ascending ADR number; run \`awf upgrade\`. (ADR-0191)`
  - Review-catalog and glossary sources that say the concept without the hyphenated term:
    `templates/agents/plan-reviewer.md.tmpl:18` drops its `sequences are consecutive` clause and
    restates batch ordering as ascending ADR number and intra-ADR history position;
    `templates/agents/adr-reviewer.md.tmpl:22` drops `and global sequence order`, restating the
    same way; `templates/agents/code-reviewer.md.tmpl:20` rewords `consecutive sequences` in its
    application-pair-correctness lens to the same ADR-number and intra-ADR position phrasing;
    `templates/adr-readme/README.md.tmpl:105-106` rewrites `Multiple ADR batches may share a
    pair only for distinct claim IDs and consecutive sequences` to `Multiple ADR batches may
    share a pair only for distinct claim IDs`; `templates/skills/reviewing-plan/SKILL.md.tmpl:75`
    drops `and global sequence order` from its V2 notes bullet; `templates/skills/reviewing-impl/SKILL.md.tmpl:87` and
    `templates/skills/reviewing-plan-resync/SKILL.md.tmpl:64` reword their softer `sequence
    order`/`sequence ordering` phrases to `ADR-number and intra-ADR position order` so no
    reviewer note is left reading through the old model. In `.awf/docs/glossary.yaml`, the
    `application batch` entry replaces `a contiguous global sequence` and batch-sequence
    inheritance with batch identity by position in the ADR's history.
  - Run `./x render`; every rendered output listed in File structure follows.
- [ ] **Task 2.11: Test sweep (batch task).** Affected set: every `_test.go` hit of
  `git grep -ln "state-sequence\|StateSequence\|stateSequence\|HasSequence\|Sequence:" -- '*_test.go'`,
  which also catches `adr.HistoryEvent{... Sequence: N, HasSequence: true ...}` struct literals
  (at authoring time: `internal/adr/format_test.go`, `internal/adr/corpus_test.go`,
  `internal/currentstate/check_test.go`, `internal/currentstate/transition_test.go`,
  `internal/project/staged_test.go`, `internal/project/mergeaggregate_test.go`,
  `internal/project/context_adr_test.go`, `internal/project/context_paths_test.go`,
  `internal/project/golden_test.go`, `internal/project/topics_test.go`,
  `internal/topic/query_test.go`, `internal/topic/corpus_test.go`, `cmd/awf/topic_test.go`,
  `cmd/awf/context_test.go`, `cmd/awf/initrender_test.go`; the set is the command's output at
  execution time; `internal/currentstate/aggregate_test.go` is touched by task 2.12, not this
  batch). Representative transformation:
  a fixture line `- 2026-07-20: Applied; state-sequence: 1; operations: add \`d/t:c\`` becomes
  `- 2026-07-20: Applied; operations: add \`d/t:c\``, and assertions on
  `[state-sequence: N]` suffixes, `stateSequence` JSON, or `StateSequence` fields are deleted with
  their expectation text. Edge: `internal/adr/format_test.go` keeps exactly one legacy fixture
  asserting that a stale `- <date>: Applied; state-sequence: 1; operations: ...` line parses with
  `LegacySequence` true and no other effect (the deliberate retained literal); tests of deleted
  validations (duplicate sequence across ADRs, contiguity, expected-next,
  sequence-presence-by-status) are deleted, not rewritten, while the
  duplicated-segment-within-one-tail error keeps its test.
  `TestCheckV2BatchSequences` is deleted with `checkSequences`;
  `TestMergeAggregateKeepsSequenceContiguity` is replaced by
  `TestMergeAggregateOrdersBatchesByADRNumber` proving cross-ADR batch order is ADR-number order.
  Post-check: `git grep -n "state-sequence\|StateSequence\|stateSequence" -- 'internal' 'cmd'`
  returns only the retained legacy-parse fixture in `internal/adr/format_test.go`, the
  `LegacySequence` flag with its parser, check, and test sites, the legacy-segment finding text
  and its fixtures in `internal/currentstate`, and the migration's rewrite logic and fixtures in
  `internal/migrate/adrnumberprovenance*.go`.
- [ ] **Task 2.12: New behaviour tests with proof markers.** All in `internal/currentstate`:
  - `TestCheckBackwardOrdersRevisedByByADRNumber` (in `check_test.go`): ascending passes;
    a descending pair fails with the new message; an entry equal to or below Origin fails; and a
    fixture event carrying a legacy segment produces the
    `carries a retired state-sequence segment` finding. Carries the marker
    `invariant: invariants/current-state-authority:provenance-ordered-by-adr-number`.
  - `TestAppliedRemoveAbsorbsConcurrentUpdate` (in `transition_test.go`): two fixtures prove
    convergence: update applied then remove applied, and remove applied then a dominated update
    arriving in a merge aggregate; both end with the claim absent, the dominated batch retained in
    history, and `awf check` clean over the after universe. Carries the marker
    `invariant: invariants/current-state-authority:applied-remove-absorbing-tombstone`.
  - `TestMergeAggregateUnionsRevisedBy` (in `aggregate_test.go`): a merge whose updater inserts
    below an existing higher number passes; a dropped before-entry fails.
  - Existing markers relocate with their tests where task 2.11 renames them; the marker for
    `application-batch-sequence-order` is deleted with `TestCheckV2BatchSequences`.
- [ ] **Task 2.13: Claim mutations and ADR-0191 events.** In
  `.awf/topics/parts/invariants/current-state-authority/current-state.md`:
  - Delete the whole `### \`invariant: application-batch-sequence-order\`` block.
  - Add, with `Origin: ADR-0191` and `Backing: test`:

    `### \`invariant: provenance-ordered-by-adr-number\``

    `A claim's provenance order is ascending final ADR number: the canonical chain is its Origin
    ADR followed by its Revised-by ADRs sorted ascending and duplicate-free, every Revised-by
    entry is greater than the Origin's number, claim history output sorts revision records the
    same way, and no status-history event carries a state sequence.`

    `### \`invariant: applied-remove-absorbing-tombstone\``

    `An applied remove is an absorbing tombstone: the qualified id is currently absent from the
    moment the remove applies, a concurrently developed update that integrates after the remove
    is retained as dominated history with no current effect and an empty required mutation set,
    and update-then-remove and remove-then-dominated-update integration orders converge to the
    same attributed absence.`
  - `merge-transition-ordered-aggregate`: replace `several application batches are legal when the
    global state-sequence stays contiguous, a claim's operations across the pair must form a
    legal ordered chain of at most one leading add, any number of updates, and at most one
    trailing remove` with `several application batches are legal in ascending ADR-number and
    intra-ADR history order, a claim's operations across the pair must form a legal ordered chain
    of at most one leading add, any number of updates, at most one remove, and after the remove
    any number of dominated updates`; append `ADR-0191` to its `Revised-by`.
  - `implemented-impact-bidirectional`: replace `has its required current or removed result` with
    `has its required current, removed, or dominated-history result` and `Remaining and Canceled
    operations provide no authority` with `Remaining, Canceled, and dominated operations provide
    no authority`; `Revised-by` gains `ADR-0191`.
  - `state-impact-transition-atomic`: after `exactly its matching claim mutations occur in one
    HEAD-to-index transaction` insert `, where a dominated operation's required mutation set is
    empty`; `Revised-by` gains `ADR-0191`.
  In `.awf/topics/parts/adr-system/adr-lifecycle/current-state.md`:
  - `applied-history-events-append-only`: delete `with one positive state sequence`; `Revised-by`
    gains `ADR-0191`.
  - `corpus-raw-access-enumerated`: `The two ordered schema migrations` becomes `The three
    ordered schema migrations`; `Revised-by` gains `ADR-0191`.
  In the ADR file, append two events (new grammar, no sequence):
  `- <today>: Implementing; content-sha256: <latest stamp>` and
  `- <today>: Applied; operations: remove
  \`invariants/current-state-authority:application-batch-sequence-order\`, add
  \`invariants/current-state-authority:provenance-ordered-by-adr-number\`, add
  \`invariants/current-state-authority:applied-remove-absorbing-tombstone\`, update
  \`invariants/current-state-authority:merge-transition-ordered-aggregate\`, update
  \`invariants/current-state-authority:implemented-impact-bidirectional\`, update
  \`invariants/current-state-authority:state-impact-transition-atomic\`, update
  \`adr-system/adr-lifecycle:applied-history-events-append-only\`, update
  \`adr-system/adr-lifecycle:corpus-raw-access-enumerated\``.
  `update \`invariants/current-state-authority:update-requires-substance\`` stays remaining, so
  the Implementing state is legal. Run `./x render` again for the topic and INDEX outputs.
- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction (explicit
  paths; the shared-checkout index may carry foreign entries); run `awf check --staged` then
  `./x gate`; both must pass with the new binary semantics. Commit:

```commit
feat(adr-system): remove the global state sequence (ADR-0191)
```

## Phase 3: Final application and Implemented flip (after terminal review)

**Execution mode: inline.** This phase executes in the terminal-review flow (`awf-reviewing-impl`
routes it; `awf-adr-lifecycle` supplies the mechanics) once review of phase 2 settles. It is
recorded here so the transaction content is fixed.

- [ ] **Task 3.1: Apply `update-requires-substance` and flip Implemented.** In
  `.awf/topics/parts/invariants/current-state-authority/current-state.md`, in the
  `update-requires-substance` claim, replace `appends its ADR once` with `adds its ADR once at
  its canonical ascending position`; `Revised-by` gains `ADR-0191`. In the ADR file, append
  `- <date>: Applied; operations: update
  \`invariants/current-state-authority:update-requires-substance\`` and
  `- <date>: Implemented; content-sha256: <latest stamp>`, and set frontmatter
  `status: Implemented`. Run `./x render` (topic outputs and INDEX). This plan's own
  `status:` flips to Implemented in the same transaction.
- [ ] **Phase-close: stage, check, gate, and commit.** Stage the transaction; run
  `awf check --staged` then `./x gate`; commit:

```commit
docs(adr): implement 0191 ADR-number provenance order
```

## Verification

- `./awf check` and `./x gate` are clean at every phase close.
- `git grep -n "state-sequence\|StateSequence\|stateSequence" -- 'internal' 'cmd' 'templates' '.awf/agents' '.awf/domains' '.awf/docs' 'changelog'` returns only the retained negative parse
  fixture in `internal/adr/format_test.go`, the migration implementation and fixtures in
  `internal/migrate/adrnumberprovenance*.go`, and the changelog bullet describing the removal.
- No `## Status history` line anywhere under `docs/decisions/` or
  `examples/sundial/docs/decisions/` matches `state-sequence` (grep returns no such line).
- `./awf topic invariants/current-state-authority` shows the two new claims with
  `Origin: ADR-0191` and no `[state-sequence:` suffix anywhere in its output.
- The two previously inverted `Revised-by` lines
  (`.awf/topics/parts/rendering/pi-workflows/current-state.md` and
  `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`) list `ADR-0166` before
  `ADR-0167`.
- A second `./awf upgrade` run reports nothing to apply (idempotency).

## Notes

- Historical ADR bodies (0135, 0143, 0182) and dated plans keep the words `state-sequence` as
  frozen history; only status-history event lines and living surfaces change.
- The digest probe workflow survives for `content-sha256`; only its sequence half is retired.
- The merge-time numbering effort consumes this change (no sequence shifting at integration); its
  design updates are out of scope here.
- Implementation deviation, found necessary during phase 2 and pinned by
  `TestRevisedByCanonicalReorderIsNotAMutation`: `checkUnmatchedMutation` compares `Revised-by`
  membership as a set rather than an ordered list, because canonical order is derived (ascending
  ADR number) and the migration's reorder of the two legacy inversions must not read as an
  unmatched mutation in the phase-2 staged transaction. `historiesEqual` likewise ignores the
  `LegacySequence` flag so the migration's stripped event lines compare as an exact history
  prefix.
- Terminal-review additions: the migration's residual scan cuts rationale prose first (an
  adopter rationale mentioning the retired term must not abort `awf upgrade`), the
  legacy-segment finding reports once per ADR, and glossary entries for `absorbing tombstone`
  and `dominated operation` landed with the review fixes.
