---
date: 2026-07-31
adrs: [0192]
status: Proposed
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
- **Modified:** `internal/project/currentstate.go`, `internal/project/currentstate_test.go`, `internal/project/staged_test.go`, `internal/migrate/dropseveritysettings.go`, `.awf/topics/parts/config/configuration/current-state.md`, `.awf/topics/parts/config/migrations-and-locks/current-state.md`, `.awf/topics/parts/rendering/sync-and-drift/current-state.md`, `.awf/topics/metadata/rendering/sync-and-drift.yaml`, `.awf/docs/glossary.yaml`, `docs/decisions/0192-topic-coverage-and-fan-out-evaluate-independently-of-config-block-presence.md`, plus the rendered outputs `docs/topics/config/configuration.md`, `docs/topics/config/migrations-and-locks.md`, `docs/topics/rendering/sync-and-drift.md`, `docs/glossary.md`, `docs/decisions/INDEX.md`, and `.awf/awf.lock`.
- **Deleted:** none.

## Phase 1: Remove the guards and correct the falsified authority

**Execution mode: inline.** This phase is one independently green coherent implementation transaction. Checkbox tasks are ordered steps, not transaction boundaries.

- [ ] **Task 1.1: Invert the working-tree no-policy test.** In `internal/project/currentstate_test.go`, `TestCheckCurrentStateNoPolicy` currently asserts the removed behaviour and its comment names it the backing site for the clause this plan strikes. Replace the four comment lines above it (`// TestCheckCurrentStateNoPolicy proves coverage is skipped ...` through `// invariant: config/configuration:severity-not-configurable`) with a comment stating that the test proves coverage and fan-out still evaluate for a tree declaring no `currentState:` block. Do NOT add a proof marker here; the new claim does not exist until phase 2. Invert the body: replace the `if report.Coverage != nil { t.Fatalf("coverage = %#v; want nil without a currentState policy", report.Coverage) }` block with an assertion that `report.Coverage != nil`, failing with `t.Fatal("coverage = nil; want evaluation without a currentState policy")`. The fixture's domain `alpha` selects `internal/**` and the repo contains `internal/bar.go` with no owning topic, so coverage now produces an uncovered finding; replace the `if len(report.Findings()) != 0` assertion with one requiring at least one finding. Run `go test ./internal/project/ -run TestCheckCurrentStateNoPolicy`; it fails until task 1.3 lands, which is expected at this step.

- [ ] **Task 1.2: Add the staged-path no-policy test.** In `internal/project/staged_test.go`, add `TestCheckStagedNoPolicy` next to the existing staged tests. It builds the same fixture shape as the file's other staged tests but writes a config with no `currentState:` block (take `csYAML` from `internal/project/currentstate_test.go` and omit its trailing `currentState:` and `maxTopicsPerPath: 8` lines; both files are package `project`, so define the variant locally rather than exporting one). Call `CheckStaged` and assert `report.Coverage != nil`, failing with `t.Fatal("staged coverage = nil; want evaluation without a currentState policy")`. Add no proof marker. This test pins the staged half of the contract, which task 1.3's second guard removal is the only thing that makes pass; without it, reverting just that guard leaves the suite green.

- [ ] **Task 1.3: Remove both guards and correct the two function doc comments.** In `internal/project/currentstate.go`, in `CheckCurrentState`, replace the guarded assignment at the `if ws.Cfg.CurrentState != nil {` block with an unguarded `report.Coverage = topic.EvaluateCoverage(ws.Loaded.Topics, eligiblePaths(ws.Tree, ws.Lock, ws.Cfg.ContextIgnore), coveragePolicy(ws.Cfg.CurrentState))`. In `CheckStaged`, do the same for the `if afterCfg.CurrentState != nil {` block, keeping its `afterCfg`/`afterTree`/`afterLock` arguments unchanged. No nil handling is added: `coveragePolicy` reaches `EffectiveMaxTopicsPerPath`, which returns 8 on a nil receiver (`internal/config/config.go:143-148`). Then correct both doc comments: in `CheckCurrentState`'s comment replace the sentence `Coverage runs only when the project configures a currentState policy.` with `Coverage and fan-out always evaluate, whether or not the project configures a currentState policy (ADR-0192).`, and in `CheckStaged`'s comment replace `Coverage runs only when the staged config declares a currentState policy.` with `Coverage and fan-out always evaluate, whether or not the staged config declares a currentState policy (ADR-0192).` Run `go test ./internal/project/ -run 'TestCheckCurrentStateNoPolicy|TestCheckStagedNoPolicy'`; both pass. Run `go test ./internal/project/`; the full package passes.

- [ ] **Task 1.4: Strike the falsified sentence from `severity-not-configurable`.** In `.awf/topics/parts/config/configuration/current-state.md`, in the `### \`invariant: severity-not-configurable\`` claim, delete the final sentence ` Whether the checks run at all is a separate concern: a tree that declares no currentState block requests neither.` so the prose ends at `...is rejected by strict parsing rather than honoured.` Append `, ADR-0192` to the existing `Revised-by: ADR-0183, ADR-0184` line, preserving the exact prior prefix. Leave `Origin: ADR-0183` and `Backing: test` untouched. The claim remains backed by the surviving proof marker at `internal/config/config_test.go:478`, which proves the severity contract proper; verify with `grep -rn "invariant: config/configuration:severity-not-configurable" --include=*_test.go .`, which must return exactly that one site after task 1.1.

- [ ] **Task 1.5: Correct the `severity-keys-dropped` rationale.** In `.awf/topics/parts/config/migrations-and-locks/current-state.md`, in the `### \`invariant: severity-keys-dropped\`` claim, delete the trailing clause `, because an absent block would stop coverage and fan-out evaluating` so the prose ends at `...instead of letting the emptied currentState block be dropped.` What generation 25 does is unchanged; only the justification is corrected. Append `, ADR-0192` to the existing `Revised-by: ADR-0184, ADR-0185` line.

- [ ] **Task 1.6: Correct the migration's doc comment.** In `internal/migrate/dropseveritysettings.go`, the comment block above `applyDropSeveritySettings` asserts in the present tense that an absent block suppresses the checks and that such a tree is deliberately opted out. Rewrite that paragraph in the past tense: state that when the two keys were the block's only children `RemoveMappingKey` drops the emptied `currentState` key with them, that at generation 25 an absent block suppressed coverage and fan-out because `internal/project/currentstate.go` gated both on `CurrentState != nil`, and that ADR-0192 removed that gate, so the seed this migration still writes is now inert but harmless and is retained because historical migrations are never edited. The migration's executable behaviour is NOT changed: `dropSeverityKeys`, the `HasMapping` probe, and the seeding call all stay exactly as they are. Run `go test ./internal/migrate/`; it passes unchanged.

- [ ] **Task 1.7: Correct the glossary entry.** In `.awf/docs/glossary.yaml`, in the `"topic coverage"` entry, replace `A tree that declares a currentState block evaluates both checks, coverage at error and fan-out at warn; neither rank is configurable and no value suppresses either (ADR-0183).` with `Every adopted tree evaluates both checks, coverage at error and fan-out at warn; neither rank is configurable, no value suppresses either, and no config declaration switches them on or off (ADR-0183, ADR-0192).` Leave the rest of the entry unchanged.

- [ ] **Task 1.8: Record the first Applied batch.** In `docs/decisions/0192-topic-coverage-and-fan-out-evaluate-independently-of-config-block-presence.md`, append to `## Status history` an `Implementing` status event carrying the content stamp, then one `Applied` event in declaration order for the two operations this phase applies: `update config/configuration:severity-not-configurable` and `update config/migrations-and-locks:severity-keys-dropped`. The `add` operation stays pending for phase 2, which is what keeps `Implementing` legal. The digest is `adr.ContentDigest`, the sha256 over exactly the five canonical sections (Context, Decision, State changes, Consequences, Alternatives Considered), each serialized as `## <name>` plus the body with trailing whitespace stripped plus one newline; it is NOT a `sha256sum` of the file. Mechanical procedure: write any 64-lowercase-hex placeholder and any state-sequence placeholder, run `./awf check`, and copy the computed digest and next state sequence from its mismatch message. Never invent a literal sequence value.

- [ ] **Task 1.9: Regenerate.** Run `./x render`. It regenerates `docs/topics/config/configuration.md`, `docs/topics/config/migrations-and-locks.md`, `docs/glossary.md`, `docs/decisions/INDEX.md`, and updates `.awf/awf.lock` with the config-hash change. Run `./x check`; it is clean apart from the pre-existing `maxClaimsPerTopic` note for `rendering/workflow-skill-templates`, which this plan does not address.

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

- [ ] **Task 2.4: Apply the final batch and flip.** In `docs/decisions/0192-...md`, append the `Applied` event for the remaining operation, `add rendering/sync-and-drift:coverage-evaluation-unconditional`, then the terminal `Implemented` status event carrying its content stamp and state sequence, in that order. Use the same mechanical digest and sequence procedure as task 1.8: placeholder, `./awf check`, copy from the mismatch message. Flip this plan's `status:` to `Implemented` and record any deviation surfaced during execution in its Notes section. Run `./x render`, then `./x check`.

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
