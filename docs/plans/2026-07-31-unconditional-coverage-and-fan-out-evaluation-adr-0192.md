---
date: 2026-07-31
adrs: [0192]
status: Implemented
---
# Plan: Unconditional coverage and fan-out evaluation (ADR-0192)

## Goal

Make the `awf check` current-state report evaluate topic coverage and topic fan-out for every adopted tree, removing the two `Cfg.CurrentState != nil` guards that made evaluation depend on whether the adopter's config declares a `currentState:` block, and correct every claim, comment, and doc that states the removed behaviour. Non-goals: this plan does not retire `currentState.maxClaimsPerTopic` (a separate ADR), does not touch `maxTopicsPerPath`, and does not audit other decisions for the self-contradiction shape ADR-0192 describes.

## Architecture summary

Two phases, matching ADR-0192's V2 operation batching.

Phase 1 is the behaviour change and every correction entangled with it. Removing the guards immediately falsifies the closing sentence of `config/configuration:severity-not-configurable` and breaks the test that proves it, so the code change, the claim update, the test inversion, and the marker removal must land as one transaction. The rationale clause of `config/migrations-and-locks:severity-keys-dropped` and the present-tense comment in `internal/migrate/dropseveritysettings.go` are falsified by the same commit, so they travel with it. Phase 1 applies two of the three declared operations and moves ADR-0192 to `Implementing`.

Phase 2 is the deferred final batch. It adds `rendering/sync-and-drift:coverage-evaluation-unconditional`, restores proof markers onto the tests phase 1 inverted, widens the topic summary that claim requires, and flips ADR-0192 to `Implemented`. It runs after terminal review, because the lifecycle rejects an all-applied non-terminal state (`Implementing` requires at least one operation still pending) while the executing-plans contract requires the flip to follow terminal review. The inverted tests therefore exist in phase 1 without the new claim's markers, which return with the claim.

## File structure

- **Created:** none.
- **Modified:** `internal/project/currentstate.go`, `internal/project/currentstate_test.go`, `internal/project/staged_test.go`, `internal/migrate/dropseveritysettings.go`, `cmd/awf/check_test.go`, `.awf/topics/parts/config/configuration/current-state.md`, `.awf/topics/parts/config/migrations-and-locks/current-state.md`, `.awf/topics/parts/rendering/sync-and-drift/current-state.md`, `.awf/topics/metadata/rendering/sync-and-drift.yaml`, `.awf/docs/glossary.yaml`, `docs/decisions/0192-topic-coverage-and-fan-out-evaluate-independently-of-config-block-presence.md`, plus the rendered outputs `docs/topics/config/configuration.md`, `docs/topics/config/migrations-and-locks.md`, `docs/topics/rendering/sync-and-drift.md`, `docs/glossary.md`, `docs/decisions/INDEX.md`, and `.awf/awf.lock`.
- **Deleted:** none.

## Phase 1: Remove the guards and correct the falsified authority

**Execution mode: inline.** This phase is one independently green coherent implementation transaction. Checkbox tasks are ordered steps, not transaction boundaries.

- [ ] **Task 1.1: Invert the working-tree no-policy test.** In `internal/project/currentstate_test.go`, `TestCheckCurrentStateNoPolicy` currently asserts the removed behaviour and its comment names it the backing site for the clause this plan strikes. Replace the four comment lines above it (`// TestCheckCurrentStateNoPolicy proves coverage is skipped ...` through `// invariant: config/configuration:severity-not-configurable`) with a comment stating that the test proves coverage and fan-out still evaluate for a tree declaring no `currentState:` block. Do NOT add a proof marker here; the new claim does not exist until phase 2. Invert the body: replace the `if report.Coverage != nil { t.Fatalf("coverage = %#v; want nil without a currentState policy", report.Coverage) }` block with an assertion that `report.Coverage != nil`, failing with `t.Fatal("coverage = nil; want evaluation without a currentState policy")`.

  The test must exercise BOTH halves of the claim it will back in phase 2, not just coverage; under-proving it would repeat the wrong-side-backing defect ADR-0192 exists to correct. Extend the fixture so it yields one finding of each kind, and assert the exact findings rather than a count:
  - Coverage: the domain `alpha` already selects `internal/**` and `internal/bar.go` has no owning topic, so it yields `topic.CoverageFinding{Path: "internal/bar.go", Domain: "alpha", Kind: topic.Uncovered, Severity: severity.Error}`.
  - Fan-out: add a file `internal/foo/x.go` and nine topics scoped to `internal/foo/**`, as metadata files `.awf/topics/metadata/alpha/fan1.yaml` through `fan9.yaml`, each with body `title: Fan N\nsummary: Fan-out fixture topic N.\npaths:\n  - internal/foo/**\n`. Every topic metadata file REQUIRES a matching current-state part or the corpus fails to load with `topic alpha/fanN has metadata but no current-state part`, and the test then dies at its `CheckCurrentState` error branch instead of asserting findings. So add nine `.awf/topics/parts/alpha/fanN/current-state.md` files too. Make exactly one of them claim-bearing (reuse the `csRuleTopic` helper) and leave the other eight claimless with body `"Intro.\n\n## Claims\n"`, the shape used at `internal/project/domains_test.go:136`.

  The one claim-bearing topic matters: claimless topics count toward fan-out but never satisfy scoped coverage, so nine claimless topics would make `internal/foo/x.go` both over budget AND uncovered, yielding a third finding. One claim-bearing topic covers the path while all nine still count for fan-out, so the fixture yields exactly one finding of each kind with `Topics: 9` intact. The already-pinned claimless-topic branch at `internal/topic/coverage_test.go:40-47` needs no duplicate here.

  Expected findings, in sorted order:

  ```go
  []topic.CoverageFinding{
      {Path: "internal/bar.go", Domain: "alpha", Kind: topic.Uncovered, Severity: severity.Error},
      {Path: "internal/foo/x.go", Kind: topic.Fanout, Severity: severity.Warn, Topics: 9},
  }
  ```

  Note the fan-out finding carries no `Domain`: fan-out is counted across owners, not per owner. Replace the `if len(report.Findings()) != 0` assertion with a `reflect.DeepEqual` against that slice, matching the precedent at `internal/topic/coverage_test.go:44-52`. This adds a `reflect` import to `internal/project/currentstate_test.go`. Run `go test ./internal/project/ -run TestCheckCurrentStateNoPolicy`; it fails until task 1.3 lands, which is expected at this step.

- [ ] **Task 1.2: Add the staged-path no-policy test.** In `internal/project/staged_test.go`, add `TestCheckStagedNoPolicy` next to the existing staged tests. It builds the same fixture shape as the file's other staged tests but writes a config with no `currentState:` block (take `csYAML` from `internal/project/currentstate_test.go` and omit its trailing `currentState:` and `maxTopicsPerPath: 8` lines; both files are package `project`, so define the variant locally rather than exporting one). Call `CheckStaged` and assert `report.Coverage != nil`, failing with `t.Fatal("staged coverage = nil; want evaluation without a currentState policy")`.

  Give this test the same two-kind shape as task 1.1, so the proof marker task 2.3 places on it backs both halves of the claim on the staged path rather than only coverage: stage `internal/bar.go` for the owned-but-unscoped coverage finding, and stage `internal/foo/x.go` plus the same nine `internal/foo/**` topics (metadata AND part for each, one claim-bearing) for the fan-out finding. An unowned path yields no finding at all, so the coverage half needs a path its domain owns but no claim-bearing topic scopes. Pin both by `reflect.DeepEqual`:

  ```go
  []topic.CoverageFinding{
      {Path: "internal/bar.go", Domain: "alpha", Kind: topic.Uncovered, Severity: severity.Error},
      {Path: "internal/foo/x.go", Kind: topic.Fanout, Severity: severity.Warn, Topics: 9},
  }
  ```

  This adds `reflect` and `internal/topic` imports to `internal/project/staged_test.go`, which imports neither today. Add no proof marker. This test pins the staged half of the contract, which task 1.3's second guard removal is the only thing that makes pass; without it, reverting just that guard leaves the suite green.

- [ ] **Task 1.3: Remove both guards and correct the two function doc comments.** In `internal/project/currentstate.go`, in `CheckCurrentState`, replace the guarded assignment at the `if ws.Cfg.CurrentState != nil {` block with an unguarded `report.Coverage = topic.EvaluateCoverage(ws.Loaded.Topics, eligiblePaths(ws.Tree, ws.Lock, ws.Cfg.ContextIgnore), coveragePolicy(ws.Cfg.CurrentState))`. In `CheckStaged`, do the same for the `if afterCfg.CurrentState != nil {` block, keeping its `afterCfg`/`afterTree`/`afterLock` arguments unchanged. No nil handling is added: `coveragePolicy` reaches `EffectiveMaxTopicsPerPath`, which returns 8 on a nil receiver (`internal/config/config.go:143-148`). Then correct both doc comments: in `CheckCurrentState`'s comment replace the sentence `Coverage runs only when the project configures a currentState policy.` with `Coverage and fan-out always evaluate, whether or not the project configures a currentState policy (ADR-0192).`, and in `CheckStaged`'s comment replace `Coverage runs only when the staged config declares a currentState policy.` with `Coverage and fan-out always evaluate, whether or not the staged config declares a currentState policy (ADR-0192).` Run `go test ./internal/project/ -run 'TestCheckCurrentStateNoPolicy|TestCheckStagedNoPolicy'`; both pass. Run `go test ./internal/project/`; the full package passes.

- [ ] **Task 1.4: Strike the falsified sentence from `severity-not-configurable`.** In `.awf/topics/parts/config/configuration/current-state.md`, in the `### \`invariant: severity-not-configurable\`` claim, delete the final sentence ` Whether the checks run at all is a separate concern: a tree that declares no currentState block requests neither.` so the prose ends at `...is rejected by strict parsing rather than honoured.` The claim's provenance lines read `Origin: ADR-0183` then `Revised-by: ADR-0184`; append `, ADR-0192` to the second, yielding exactly `Revised-by: ADR-0184, ADR-0192`. Do NOT write `ADR-0183` into `Revised-by`: the transition validator requires the prior list as an exact prefix extended by exactly one entry, and an unmatched `ADR-0183` entry is separately rejected as naming an ADR with no applied update operation. Leave `Origin: ADR-0183` and `Backing: test` untouched. The claim remains structurally backed by the surviving proof marker at `internal/config/config_test.go:478`, which sits on `TestCurrentStateStrictValidation` and proves the strict-parsing clause; the fixed-rank clause has no marker once task 1.1 removes the other one, but that gap is pre-existing rather than created here, because the removed marker proved the struck sentence and not the ranks. No new backing is owed by this plan. Verify with `grep -rn "invariant: config/configuration:severity-not-configurable" --include=*_test.go .`, which must return exactly that one site after task 1.1.

- [ ] **Task 1.5: Correct the `severity-keys-dropped` rationale.** In `.awf/topics/parts/config/migrations-and-locks/current-state.md`, in the `### \`invariant: severity-keys-dropped\`` claim, delete the trailing clause `, because an absent block would stop coverage and fan-out evaluating` so the prose ends at `...instead of letting the emptied currentState block be dropped.` What generation 25 does is unchanged; only the justification is corrected. Append `, ADR-0192` to the existing `Revised-by: ADR-0184, ADR-0185` line.

- [ ] **Task 1.6: Correct the migration's doc comment.** In `internal/migrate/dropseveritysettings.go`, the comment block above `applyDropSeveritySettings` asserts in the present tense that an absent block suppresses the checks and that such a tree is deliberately opted out. Rewrite that paragraph in the past tense: state that when the two keys were the block's only children `RemoveMappingKey` drops the emptied `currentState` key with them, that at generation 25 an absent block suppressed coverage and fan-out because `internal/project/currentstate.go` gated both on `CurrentState != nil`, and that ADR-0192 removed that gate, so the seed this migration still writes is now inert but harmless and is retained because historical migrations are never edited. The migration's executable behaviour is NOT changed: `dropSeverityKeys`, the `HasMapping` probe, the seeding call, and the announcement string printed at the seeding site all stay exactly as they are. The announcement's present-tense wording about keeping coverage and fan-out evaluating is stale after this commit, and it stays stale on purpose: it is command output and therefore behaviour, and ADR-0192 item 7 freezes migration behaviour. Note in the rewritten comment that the migration test pins only the announcement prefix by substring and does NOT pin the stale trailing clause, so nothing would go red if the clause were edited. That is precisely why retaining it has to be a deliberate recorded choice rather than something the suite backstops. Say so, so an implementation reviewer reads the wording as retained on purpose rather than as a missed correction. Run `go test ./internal/migrate/`; it passes unchanged.

- [ ] **Task 1.7: Correct the glossary entry.** In `.awf/docs/glossary.yaml`, in the `"topic coverage"` entry, replace `A tree that declares a currentState block evaluates both checks, coverage at error and fan-out at warn; neither rank is configurable and no value suppresses either (ADR-0183).` with `Every adopted tree evaluates both checks, coverage at error and fan-out at warn; neither rank is configurable, no value suppresses either, and no config declaration switches them on or off (ADR-0183, ADR-0192).` Leave the rest of the entry unchanged.

- [ ] **Task 1.8: Correct the check fixture comment.** In `cmd/awf/check_test.go`, the `coverageYAML` helper comment asserts the behaviour this phase removes: "The currentState block must stay non-empty: coverage is only evaluated when the config declares one, and a bare \"currentState:\" key is a hard parse error." Rewrite the middle clause: the fixture keeps its non-empty block because the warn fixtures need the fan-out budget, and a bare `currentState:` key remains a hard parse error, but the coverage-gating rationale is retired by ADR-0192. The fixture body and every test using it are unchanged, and the suite stays green either way, which is exactly why nothing mechanical catches this and it must be done deliberately.

- [ ] **Task 1.9: Record the first Applied batch.** In `docs/decisions/0192-topic-coverage-and-fan-out-evaluate-independently-of-config-block-presence.md`, set the frontmatter `status:` on line 3 to `Implementing` (the parser refuses when the latest Status history status does not match frontmatter), then append these two lines to `## Status history`, in this order, replacing only the bracketed placeholders:

  ```
  - YYYY-MM-DD: Implementing; content-sha256: <64 lowercase hex characters>
  - YYYY-MM-DD: Applied; operations: update `config/configuration:severity-not-configurable`, update `config/migrations-and-locks:severity-keys-dropped`
  ```

  Operations appear in the ADR's declaration order and each id is in an inline code span. The `add` operation stays pending for phase 2, which is what keeps `Implementing` legal. The digest is `adr.ContentDigest`, the sha256 over exactly the five canonical sections (Context, Decision, State changes, Consequences, Alternatives Considered), each serialized as `## <name>` plus the body with trailing whitespace stripped plus one newline; it is NOT a `sha256sum` of the file. Mechanical procedure: write any 64-lowercase-hex placeholder, run `./awf check`, and copy the computed digest from its mismatch message. No event carries a `state-sequence:` segment: ADR-0191 removed that namespace and `awf check` reports any survivor as a blocking finding.

- [ ] **Task 1.10: Regenerate.** Run `./x render`. It regenerates `docs/topics/config/configuration.md`, `docs/topics/config/migrations-and-locks.md`, `docs/glossary.md`, `docs/decisions/INDEX.md`, and updates `.awf/awf.lock` with the config-hash change. Run `./x check`; it is clean apart from the pre-existing `maxClaimsPerTopic` note for `rendering/workflow-skill-templates`, which this plan does not address.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction explicitly by path (no `git add -A`; the checkout is shared with concurrent sessions, so run `git status --short` first and stage only this phase's paths); run `awf check --staged` then `./x gate`; create the one phase-closing commit:

```commit
fix(rendering): evaluate coverage unconditionally (0192 batch)
```

## Phase 2: Add the backing claim and flip

**Execution mode: inline.** This phase is one independently green coherent implementation transaction. It runs only after the terminal review of phase 1 settles, per the executing-plans contract that the final batch and the status flip follow the last terminal review.

- [ ] **Task 2.1: Widen the topic summary.** In `.awf/topics/metadata/rendering/sync-and-drift.yaml`, extend the `summary:` so the topic covers current-state coverage reporting alongside drift detection. The current value is `"How sync and check detect and report drift: hash inputs, attribution, backups, residue, pruning, and cleanup."`; replace it with `"How sync and check detect and report: drift hash inputs, attribution, backups, residue, pruning, cleanup, and current-state coverage and fan-out evaluation."` Leave `title:` and `paths:` unchanged; `paths: internal/project/**` is what makes the claim surface at the defect site and must not be narrowed.

- [ ] **Task 2.2: Add the claim.** In `.awf/topics/parts/rendering/sync-and-drift/current-state.md`, append a new claim section after the existing final claim (`### \`invariant: uninstall-removes-lock-entries\``):

```markdown
### `invariant: coverage-evaluation-unconditional`

The awf check current-state report evaluates topic coverage and topic fan-out for every adopted tree, in the working-tree path and the staged path alike, independent of whether the config declares a currentState block; a tree declaring no block evaluates against the same defaults as a tree that declares one and sets nothing in it.
Origin: ADR-0192
Backing: test
```

- [ ] **Task 2.3: Restore the proof markers.** Add `// invariant: rendering/sync-and-drift:coverage-evaluation-unconditional` immediately above `TestCheckCurrentStateNoPolicy` in `internal/project/currentstate_test.go` and above `TestCheckStagedNoPolicy` in `internal/project/staged_test.go`. Two markers are required, not one: the claim asserts both paths, and a marker on only the working-tree test would leave the staged half unproven, which is the two-layer gap the code-reviewer's `invariant-proof-exercises-its-claim` lens exists to catch. Confirm both tests genuinely exercise the claim by reverting each guard removal from task 1.3 independently and checking that the corresponding test goes red, then restore.

  Also add a second `// invariant: config/configuration:severity-not-configurable` marker above `TestCheckCurrentStateNoPolicy`, alongside the new claim's marker. Phase 1's terminal review found that after task 1.4's strike, that claim's fixed-rank clause ("a requested coverage finding always reports at error and a requested fan-out finding always at warn") carries no proof marker: the removed marker proved only the struck sentence, so the gap is pre-existing rather than created here. The inverted test's `reflect.DeepEqual` already pins `severity.Error` and `severity.Warn` exactly, so closing the gap costs one line and stops a `Backing: test` claim from carrying an unproven universal.

- [ ] **Task 2.4: Apply the final batch and flip.** In `docs/decisions/0192-...md`, set the frontmatter `status:` on line 3 to `Implemented`, then append these two lines to `## Status history`, in this order:

  ```
  - YYYY-MM-DD: Applied; operations: add `rendering/sync-and-drift:coverage-evaluation-unconditional`
  - YYYY-MM-DD: Implemented; content-sha256: <64 lowercase hex characters>
  ```

  No event carries a `state-sequence:` segment; ADR-0191 removed the repository-global sequence namespace and `awf check` reports any survivor as a blocking finding. Use the same mechanical digest procedure as task 1.9: placeholder, `./awf check`, copy from the mismatch message. Flip this plan's `status:` to `Implemented` and record any deviation surfaced during execution in its Notes section. Run `./x render`, then `./x check`.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage explicitly by path after a fresh `git status --short`; run `awf check --staged` then `./x gate`; create the one phase-closing commit:

```commit
feat(invariants): back unconditional evaluation (0192 batch)
```

## Verification

- `git grep -n "CurrentState != nil" -- internal/project/currentstate.go` returns no output.
- `git grep -n "Coverage runs only when" -- internal/project` returns no output.
- `./awf topic rendering/sync-and-drift:coverage-evaluation-unconditional --coverage` resolves the claim and lists both proof sites.
- `./awf context internal/project/currentstate.go` surfaces `rendering/sync-and-drift` among the applicable topics, which is the property that motivated the claim's home.
- `./x check` is clean apart from the pre-existing `maxClaimsPerTopic` note.
- `./x gate` passes, including the 100% statement-coverage floor with no new `coverage-ignore` marker introduced by this plan.

## Notes

- ADR-0192 item 5 requires the existing `TestCheckCurrentStateNoPolicy` to be inverted rather than supplemented. Task 1.1 does that, and deliberately leaves it markerless until phase 2; a marker cannot reference a claim that does not yet exist.
- The `maxClaimsPerTopic` advisory note fires throughout both phases. It is out of scope here and is retired by the follow-on ADR that this work unblocks.
- Whichever of this effort and `remove-global-state-sequence` lands first takes the next schema generation, but neither phase of this plan changes the config schema, so no generation is consumed and no collision arises from this plan.
- **Renumbered from 0190 to 0192 by rewriting this branch's history.** Two concurrent efforts landed on main between phase 1 and integration: one took 0190 (`0190-compress-governed-dispatch-guidance-with-reference-and-shared-partials`) and one took 0191 (`0191-replace-the-global-state-sequence-with-adr-number-provenance-order`), so this effort's 0190 collided and 0191 was gone before it could be claimed. Renumbering on top of the existing commits is not expressible: `awf check --staged` refuses with "ADR-0190 violates the history-prefix rule", and both claim updates then read as provenance-only mutations with no operation behind them, because renumbering an ADR whose operations are already applied rewrites committed provenance. The branch was rewritten instead, with `git filter-branch` over all nine commits renaming the ADR and plan files, updating every reference, and correcting the affected commit subjects.
- **Rebased rather than merged, and that was load-bearing.** Merging main in left the before-side corpus holding 0192 with 0190 and 0191 absent, which fails the ADR-contiguity check outright; the work has to sit on top of main's ADRs, not beside them. The rebase also picked up schema generation 27 via `awf upgrade`.
- **The phase-1 Applied event lost its `state-sequence` segment.** ADR-0191 removed the repository-global sequence namespace outright, so the event that originally read `state-sequence: 107` now carries none, and tasks 1.9 and 2.4 were corrected to the current shape. That sequence would have collided too: 107 was consumed by the other 0190.
