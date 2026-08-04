---
format: plan-v2
date: 2026-08-04
adrs: [remove-the-sundial-example-adopter]
status: Proposed
---
# Plan: Remove the Sundial Example Adopter

## Goal

Remove the committed `examples/sundial` adoption and every repository-specific dependency on it while preserving essential render, drift-check, target-output, runner, nested-adopter, and upgrade confidence in temporary fixtures. Non-goals are a replacement committed example, changes to generic nested-adopter support, and edits to retained ADRs, completed plans, changelog entries, or research reports that accurately mention Sundial historically.

## Architecture summary

Execution first accepts the settled ADR, then establishes package-owned temporary coverage before deleting any example-backed checks. The final implementation transaction removes the fixed nested tree, root orchestration, obsolete proofs, and active guidance together with all five declared claim operations: four removals and one update. ADR-0229 permits that transaction to leave every operation Applied while the ADR remains `Implementing`; terminal implementation review later owns the status-only `Implemented` event and plan freeze. Generic nested-adopter fixtures remain generic, and generated documentation changes only through authored `.awf/` inputs followed by `./x render`.

## Phase 1: Accept the removal decision

**Execution mode: inline.**

Completes: ["decision-authorized"]

### Task 1.1: Accept the reviewed ADR
Latitude: exact
Paths: ["docs/decisions/remove-the-sundial-example-adopter.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Post-check: `./awf check` and `./x check` both reach a clean terminal state, and `./awf context docs/decisions/remove-the-sundial-example-adopter.md` reports the ADR as Accepted with all four remove operations pending.

In `docs/decisions/remove-the-sundial-example-adopter.md`, change frontmatter `status: Proposed` to `status: Accepted` and append `- 2026-08-04: Accepted; content-sha256: <computed digest>` to `## Status history`. Establish the first digest without changing Context, Decision, State changes, Consequences, or Alternatives. Obtain the digest mechanically: place a 64-lowercase-hex placeholder, run `./awf check`, and replace it with the computed digest reported by the mismatch; do not precompute or copy a digest from this plan. Run `./x render`, which regenerates the decision index and lock, then run the Post-check from the resulting tree.

### Phase close

Stage only the ADR transition, regenerated `docs/decisions/INDEX.md`, and `.awf/awf.lock`. Confirm `git diff --cached --check`, `./awf check staged`, and `./x gate` pass, then create the phase commit.

```commit
docs(adr): accept removing the Sundial example adopter
```

## Phase 2: Preserve executable adopter contracts in temporary fixtures

**Execution mode: subagent-driven.**

Completes: ["temporary-coverage"]

### Task 2.1: Make full-catalog evaluation exercise both supported targets and drift checks
Latitude: exact
Applying: ["remove-the-sundial-example-adopter:temporary-contract-fixtures"]
Paths: ["internal/evals/fixture_test.go"]
Post-check: `go test ./internal/evals -run 'TestFullCatalogCoverage' -count=1` passes. Inside each target subtest, remove one representative rendered skill file after initialization and require `Project.Check` to report that target-native path as missing, restore it by `SyncReport`, tamper `AGENTS.md`, require hand-edited drift, restore again, and require the final check to be clean.

Before dispatch, require `git status --short` to print no entries, `./x check` to be clean while Sundial still exists, and `go test ./internal/evals ./internal/project ./cmd/awf` to pass from the Phase 1 commit.

Refactor the existing catalog-derived fixture without hand-listing skills, agents, or optional docs. `TestFullCatalogCoverage` must run the same assertions for `claude` and `pi`: initialize a temporary project through `project.InitializeReport`, prove every catalog skill and agent has its target-native rendered file, open the initialized project, and require `Project.Check` to return neither an error nor drift. Derive target output roots from the target under test rather than retaining Claude-only path helpers. Inspect the rendered target tree and fail on any unresolved `<no value>` token. Keep the existing `tooling/evaluations:evals-full-catalog-coverage` proof marker on the test whose catalog derivation and artifact assertions substantively prove the current claim; this task broadens test confidence but does not change that claim's prose or declare a new invariant.

### Task 2.2: Add a representative authored-adoption render and drift lifecycle fixture
Latitude: exact
Applying: ["remove-the-sundial-example-adopter:temporary-contract-fixtures"]
Paths: ["internal/project/adopter_fixture_test.go", "internal/project/example_wiring_test.go"]
Post-check: `go test ./internal/project -run 'TestTemporaryAdopterRenderDriftLifecycle|TestPiExtensionEditorQuietStrip' -count=1` passes; tampering with the selected managed output makes the lifecycle test observe drift, and removing the Pi directive from a rendered temporary output makes the Pi test fail.

Create `internal/project/adopter_fixture_test.go` in package `project` with helper `temporaryAuthoredAdopter(t *testing.T) string`. Use `scaffoldFiles` with this exact config shape: prefix `fixture`; integration branch `main`; targets `claude` and `pi`; skills `tdd` and `exploring`; agent `explorer` (the exact `exploring` requirement closure); no optional docs; vars `testCmd: go test ./...` and `gateCmd: ./x gate`; domain `alpha`; `currentState.maxTopicsPerPath: 8`; and `bootstrap.enabled`, `hooks.enabled`, and `runner.enabled` all true. Supply these exact authored fixture files relative to `.awf/`: `skills/tdd.yaml` with `data.testSurfaces` containing `{name: Unit, kind: isolated, location: internal}`, `skills/parts/tdd/notes.md` containing `Fixture-specific TDD notes.`, `domains/alpha.yaml` with `paths: ["internal/**"]`, `domains/parts/alpha/current-state.md` containing `Fixture domain guidance.`, `topics/metadata/alpha/model.yaml` with literal content `title: Model`, `summary: Fixture model rules.`, and `paths: ["internal/**"]`, and `topics/parts/alpha/model/current-state.md` with exact content `Fixture model guidance.`, a blank line, and `## Claims`.

Call `Open`, then `InitializeReport` with `InitAuthority{InitializedWithVersion: Version}`. Require a clean `Project.Check`, append `tampered\n` to the exact managed output `AGENTS.md`, require the next check's drift list to contain `AGENTS.md` with hand-edited kind, call `SyncReport`, and require the final check to be clean. Use standard-library assertions and no new package-level swappable seam.

In `TestPiExtensionEditorQuietStrip`, replace the second committed rendered root with the temporary Pi fixture. Enumerate governed TypeScript outputs from `piTarget.Outputs`; require each output actually selected by the fixture to carry the provenance banner and immediate `// @ts-nocheck` directive, and keep the reverse walk that rejects undeclared TypeScript files under the temporary `.pi/extensions` root. Continue checking the root container harness's copy-strip-compile ordering separately. Do not weaken `rendering/pi-runtime:pi-extension-target-render` or `rendering/pi-workflows:pi-extension-editor-quiet-strip` backing.

### Task 2.3: Make the historical CLI upgrade fixture finish with a clean adopted project
Latitude: exact
Applying: ["remove-the-sundial-example-adopter:temporary-contract-fixtures"]
Paths: ["cmd/awf/run_test.go"]
Post-check: `go test ./cmd/awf -run 'TestRunUpgradeLegacyAdopterRendersAndChecksClean' -count=1` passes; removing `runUpgrade`'s terminal sync makes the expected `AGENTS.md` assertion or repository drift check fail.

Rename and strengthen `TestRunUpgradeAppliesLegacyMigration` as `TestRunUpgradeLegacyAdopterRendersAndChecksClean`. Preserve its real generation-zero `.claude/awf.yaml` and legacy lock input and its assertion that at least one migration applies. After `runUpgrade`, assert the current `.awf/config.yaml` and `.awf/awf.lock` exist, assert root `AGENTS.md` exists and contains the stable heading `# example Agent Guide`, and call `runCheckRepo` against the upgraded working tree, requiring a clean repository-universe result. Do not call aggregate `runCheck`: this fixture intentionally leaves the Git index at its committed legacy state, so staged-universe checking would test an unrelated unstaged-lock condition. This is the CLI composition seam for migration plus terminal rendering plus normal adopter repository checking; focused migration transformations remain owned by `internal/migrate` and `internal/upgrade` tests.

### Phase close

Stage exactly the Phase 2 test files. Run the three Post-check commands from the clean phase result, then run `git diff --cached --check`, `./awf check staged`, and `./x gate`. Create the phase commit only with Sundial still present, the four removal claims still active and backed, and the single-home claim still carrying its pre-removal scope.

```commit
test(tooling): replace Sundial integration coverage
```

## Phase 3: Remove Sundial and apply every removal operation

**Execution mode: subagent-driven.**

Completes: ["sundial-absent", "active-state-current"]

### Task 3.1: Remove fixed-path root runner and staged-hook orchestration
Latitude: exact
Applying: ["remove-the-sundial-example-adopter:no-committed-example-adopter"]
Paths: ["x", ".githooks/pre-commit", ".githooks/check-nested-staged", "cmd/awf/check_test.go", "internal/project/context_wrapper_test.go"]
Post-check: `test ! -e .githooks/check-nested-staged` succeeds; `rg -n 'examples/sundial|check-nested-staged|example adopter|nested Sundial' x .githooks cmd/awf/check_test.go internal/project/context_wrapper_test.go` returns no output; and `go test ./cmd/awf ./internal/project -run 'TestRepositoryPreCommit|TestContextSpillObservabilityContract' -count=1` passes.

Before dispatch, require `git status --short` to print no entries, `./x check` to be clean, the Phase 2 focused tests to pass, `test -d examples/sundial` to succeed, and the ADR to be Accepted with five Remaining operations and no Applied operation.

In `x`, remove only the source-built Sundial render block from `render)` and the Sundial repo check, zero-note parsing, and separate Go-module test from `check)`. Preserve root render, context-spill validation, every gate lane, cleanup behavior, and command forwarding.

In `.githooks/pre-commit`, remove the staged-helper discovery/refusal, the fixed `check_slice "$tmp/examples/sundial"` call, and helper invocation. Delete `.githooks/check-nested-staged`. Preserve staged-slice repository creation, root build and drift check, explicit `rm -rf -- "$tmp"` plus trap clearing before delegation, and the rendered payload exec.

Update `cmd/awf/check_test.go` accordingly: remove the missing-helper and Sundial nested-transition tests and fixed-path expectations, but retain `TestRepositoryPreCommitRemovesSliceBeforePayload` as a behavioral root-only proof that the slice is gone and temporary Git environment is cleared before the payload runs. Adjust that fixture rather than deleting it. In `internal/project/context_wrapper_test.go`, remove the fake source-build/Sundial-directory setup made unnecessary by the simplified root `x`, while preserving the context-spill advisory behavior under test.

### Task 3.2: Delete the committed tree and remove Sundial-only tests while preserving generic behavior
Kind: batch
Latitude: exact
Applying: ["remove-the-sundial-example-adopter:no-committed-example-adopter", "remove-the-sundial-example-adopter:generic-nested-adopter-capability-remains"]
Paths: ["pathspec:examples/sundial", "internal/project/example_wiring_test.go", "internal/project/repository_wiring_test.go", "internal/project/staged_test.go", "internal/project/notes_test.go", "internal/git/seamwalker_test.go", "internal/migrate/dropauditbase_test.go", "internal/testsupport/deps_test.go"]
Representative: `git rm -r -- examples/sundial` removes the entire committed adoption, while the staged-current-state fixture moves from the fixed `examples/sundial/` prefix to a neutral temporary nested-adopter path and continues calling `CheckStaged` from that nested root.
Edge: `TestPiExtensionLegacySweepPredicate`, the Pi container wiring tests, the temporary-output form of `TestPiExtensionEditorQuietStrip`, and generic nested-adopter staged behavior remain; only tests and assertions whose contract is the committed Sundial tree, its fictional Go module, generated prose, or hand-written runner are deleted.
Post-check: `test ! -e examples/sundial` succeeds; `git ls-files examples/sundial` returns no output; `go test ./internal/project ./internal/git ./internal/migrate ./internal/testsupport -count=1` passes; and `rg -n 'Sundial|sundial|examples/sundial|example-adopter safety case' internal/project internal/git internal/migrate internal/testsupport` returns no output.

Delete every tracked path below `examples/sundial` with the exact `git rm` command above. In `internal/project/example_wiring_test.go`, delete `TestExampleAdopterWiring`, `assertExampleDecisionRouting`, `TestSundialConfirmedEffortBoundary`, and `TestExampleAdoptsRunner`, including the four retiring proof markers. Move the remaining repository-owned Pi container and editor-strip tests to `internal/project/repository_wiring_test.go` with `git mv`; do not duplicate them or change their surviving invariant markers.

Keep generic nested-adopter behavior by renaming only fixture paths and labels in `internal/project/staged_test.go`; the test must still construct a nested adopted tree and prove its HEAD-to-index current-state transition. Remove the `examples/` module exclusion from `internal/git/seamwalker_test.go` because no committed nested module remains, while retaining testdata exclusions. Generalize the synthetic production-import case in `internal/testsupport/deps_test.go`, the empty-tag-vocabulary comment in `internal/project/notes_test.go`, and the migration-coverage comment in `internal/migrate/dropauditbase_test.go` so their actual contracts remain unchanged without claiming Sundial exercises them.

### Task 3.3: Retire four claims, update claim scope, and remove stale active guidance
Kind: batch
Latitude: exact
Applying: ["remove-the-sundial-example-adopter:no-committed-example-adopter", "remove-the-sundial-example-adopter:history-remains-history"]
Paths: ["README.md", ".awf/config.yaml", ".awf/agents/plan-reviewer.yaml", ".awf/parts/agents-doc/awf-setup.md", ".awf/parts/working-with-awf/overview.md", ".awf/docs/glossary.yaml", ".awf/docs/pitfalls.yaml", ".awf/docs/parts/development/command-runner.md", ".awf/docs/parts/roadmap/ideas.md", ".awf/docs/parts/testing/gate.md", ".awf/docs/parts/testing/tiers.md", ".awf/topics/parts/code-design/single-home/current-state.md", ".awf/topics/parts/rendering/companion-scripts/current-state.md", ".awf/topics/parts/tooling/quality-gates/current-state.md", "internal/configspec/spec.go", "docs/decisions/remove-the-sundial-example-adopter.md", "docs/decisions/INDEX.md", "glob:docs/*.md", "glob:docs/topics/**/*.md", "glob:docs/domains/**/*.md", ".claude/agents/plan-reviewer.md", ".pi/agents/plan-reviewer.md", "AGENTS.md", "CLAUDE.md", ".awf/awf.lock"]
Representative: remove the complete `example-adopter-checked`, `example-module-isolated`, and `example-zero-notes` claim blocks from the quality-gates part and `runner-example-adopted` from companion scripts, then remove their proof markers in Task 3.2 in the same phase transaction.
Edge: keep `.awf/config.yaml`'s `example-adopter` tag key because retained ADR-0090 uses it, but change its meaning to explicitly historical wording; preserve the user-requested "Applying every state operation does not mean terminal review has settled" pitfall even though its incident names Sundial; do not edit implemented ADRs, completed plans, changelog entries, or research reports.
Post-check: `./x render && ./x check` are clean; `./awf context docs/decisions/remove-the-sundial-example-adopter.md` reports all five operations Applied with zero Remaining while status remains Implementing; the active-source residue command in the task body returns no unexpected paths; and `git diff -- docs/decisions/0090-in-repo-example-adopter-as-onboarding-artifact-and-rendered-output-quality-oracle.md 'docs/plans/**' 'docs/research/**' 'changelog/**'` returns no output.

Remove `contextIgnore: examples/**` from `.awf/config.yaml`. Retain the historical `example-adopter` tag vocabulary key with wording that says it classifies the former committed adopter, because removing it would invalidate retained ADR tags. Generalize the config-reference descriptor in `internal/configspec/spec.go` so it describes configured exclusions without naming a current example.

Delete the obsolete example-adopter and quality-oracle glossary terms, onboarding links, roadmap statement, runner and testing prose, agent-guide comparison, and the single-home scope exclusion. In `.awf/agents/plan-reviewer.yaml`, replace `the root project or an example adopter` with `one or more adopted project roots`; preserve the entire generated-source-closure lens and its requirement that every configured root pairs authored causes with generated outputs.

Apply these exact `.awf/docs/pitfalls.yaml` dispositions:

- Preserve `Applying every state operation does not mean terminal review has settled` byte-for-byte.
- In `A schema-generation bump needs awf upgrade, not awf render`, delete the second-tree upgrade instructions and state that the repository's one adopted tree must run the source-built upgrade before render/check.
- In `A data: list override replaces the catalog defaults wholesale`, replace the former per-project/Sundial verification sentence with a requirement to read every affected enabled target output.
- In `Topic and decision edits regenerate navigation outside the authored file`, retain the navigation/staging lesson but delete the Sundial/config-reference recurrence and fixed nested staged-check claim.
- Rewrite `A plan editing a catalog template or default under-enumerates the render fan-out` to enumerate all enabled targets, the root lock, generated outputs, and changelog obligation for the one adopted root; remove its `example-adopter` tag and every former second-root incident/instruction.
- In `A token or convention rename must sweep every rendered doc surface`, replace the fixed Sundial pathspec sentence with the generic rule that a leading-path pathspec never reaches a separately rooted nested adoption and such a root, when present in another repository, must be named explicitly.
- In `Registering a migration has fallout the plan never lists`, generalize the historic broken-test inventory so it does not name `example_wiring_test.go`, and replace the two-lock/separate-module ending with the one root lock's required source-built upgrade.
- Delete the complete `A new awf check note producer must be inert for the example adopter` entry; focused note-producer tests retain the generic opt-in behavior.

Edit only these authored sources; never hand-edit their generated docs.

Apply all five ADR operations atomically. In `docs/decisions/remove-the-sundial-example-adopter.md`, change frontmatter to `status: Implementing`, append the dated `Implementing; content-sha256: <accepted digest>` event, then append one `Applied; operations:` event containing the four declared removals and the single-home update. ADR-0229 permits this all-Applied nonterminal state. Remove the four claim blocks and their proof markers and update `code-design/single-home:single-implementation` to remove the obsolete example-adopter scope exclusion in the same staged phase transaction. Preserve its Origin, Backing, and Verify fields and append `ADR-remove-the-sundial-example-adopter` exactly once to `Revised-by`. Do not append `Implemented`; terminal review owns that later status-only event.

Run `./x render` after Tasks 3.1 and 3.2 have removed every fixed-path invocation and the nested tree. Inspect all regenerated outputs and stage them with their authored causes. Define the active-source residue set with:

```bash
rg -l 'Sundial|sundial|examples/sundial|committed example adopter|example adopter' \
  README.md x .awf .githooks internal cmd tools .github \
  --glob '!docs/decisions/**' --glob '!docs/plans/**' --glob '!docs/research/**' --glob '!changelog/**' \
  | grep -vE '^(\.awf/docs/pitfalls\.yaml|\.awf/config\.yaml)$'
```

The command must return no output. Then verify the two excluded authored files rather than trusting the exclusion:

```bash
grep -Fxq '  example-adopter: Historical tag for the former committed example adopter' .awf/config.yaml
if grep -vFx '  example-adopter: Historical tag for the former committed example adopter' .awf/config.yaml | rg -n 'Sundial|sundial|examples/sundial|committed example adopter|example adopter'; then exit 1; fi
if awk '$0 == "    - title: \"Applying every state operation does not mean terminal review has settled\"" {skip=1; next} skip && /^    - title:/ {skip=0} !skip {print}' .awf/docs/pitfalls.yaml | rg -n 'Sundial|sundial|examples/sundial|committed example adopter|example adopter'; then exit 1; fi
```

The exact historical tag line and the one skipped lifecycle-pitfall block are the only allowed authored occurrences; generated `docs/pitfalls.md` may mirror that incident. Generic fixture paths such as `examples/nested` remain legal where they test product behavior and do not name the removed tree.

### Phase close

Run every Phase 3 Post-check from the final ordered tree. Inspect `git status --short`; refuse any path outside the declared authored/test/deletion scope and renderer-produced outputs. Stage the complete transaction explicitly, including every deletion, the ADR Applied batch, current-state removals, authored docs, rendered docs and guides, decision index, and lock. Run `git diff --cached --check`, `./awf check staged`, and `./x gate`, then create the phase commit. The commit leaves the ADR Implementing with all operations Applied and leaves the plan Proposed for terminal review.

```commit
refactor(awf): remove Sundial adopter (applies ADR removal batch)
```

## Definition of done

- `dod: decision-authorized` The reviewed removal ADR is Accepted with its then-current content stamp and four pending removal operations before implementation changes begin.
- `dod: temporary-coverage` Temporary fixtures prove full-catalog Claude and Pi rendering and clean drift checks, a representative authored-adoption render/tamper/repair lifecycle, Pi governed-output directives, and legacy CLI upgrade followed by clean checking without depending on a committed example tree.
- `dod: sundial-absent` No tracked `examples/sundial` path, fixed-path root runner branch, nested staged helper, fictional module test, or Sundial-only generated-prose assertion remains.
- `dod: active-state-current` All five operations are Applied atomically with four claim/proof deletions and the single-home scope update; active config and documentation describe the single adopted root and package-owned fixtures, while retained historical records and required historical tag metadata remain intact.

## Notes

- ADR-0229 is an integrated prerequisite. Its all-Applied Implementing state is intentional: implementation review occurs after Phase 3, then the review workflow appends only the `Implemented` status event and flips this plan to `Implemented` in the deferred terminal transaction.
- The approved but uncommitted idea to add an evaluation-claim update was withdrawn after the user identified the former strict-subset lifecycle model as defective. This plan does not add or update a claim merely to reserve terminal-review work.
- The lifecycle misconception pitfall was committed before planning and remains an explicit exception to the otherwise complete removal of active Sundial naming.
