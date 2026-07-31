---
date: 2026-08-01
adrs: [199]
status: Proposed
---
# Plan: Fork the verification commands by universe

## Goal

Execute ADR-0199: fork `awf check` into `check repo` and `check staged` universe sub-groups, delete
the `--staged` flag and the `check invariants` command, add a narrow `check staged drift`, gate the
whole check family, and make the two staged scanners correct inside a nested adopter. Non-goals: the
config-tree hygiene sweep and dead-reference halves of staged drift, and the wider check-architecture
cleanup, both of which ADR-0199 defers by name.

## Architecture summary

Six phases, each one green transaction closing in one commit. Phase 1 removes a mechanism, which
shrinks the surface Phase 2 has to make depth-aware. Phase 2 is the fork itself and carries the bulk
of the respelling. Phases 3 to 5 add behaviour that the forked surface makes expressible. Phase 6
removes the dead command and turns on the example adopter's gates, which is last because it depends
on Phase 3's scanner fix.

ADR-0199 is `Proposed`. Phase 1 appends its `Accepted` and `Implementing` status events and Applied
batch 1; Phases 2 to 6 each append one further Applied batch in declaration order. The batch
partition is 2 / 7 / 1 / 1 / 1 / 1 across the thirteen declared operations. The `Implemented` flip
and the plan's own `status: Implemented` freeze are deferred to the post-terminal-review transaction
and belong to no phase here.

## File structure

- **Created:** `cmd/awf/checkrepo.go`, `cmd/awf/checkstaged.go`, `internal/migrate/retargetcheckcommands.go`,
  `internal/migrate/retargetcheckcommands_test.go`, and the topic-claim prose for three added claims
  under `.awf/topics/parts/`.
- **Modified:** `internal/clispec/clispec.go`, `cmd/awf/dispatch.go`, `cmd/awf/main.go`,
  `cmd/awf/check.go`, `cmd/awf/gate.go`, `cmd/awf/prosegate.go`, `cmd/awf/memorygate.go`,
  `internal/project/gatedcommands.go`, `internal/project/currentstate.go`,
  `internal/migrate/migrate.go`, `x`, `README.md`, `.githooks/check-nested-staged`,
  `templates/hooks/pre-commit.sh.tmpl`, `templates/docs/working-with-awf.md.tmpl`,
  `examples/sundial/.awf/config.yaml`, the authored `.awf/` inputs ADR-0199 items 6 and 12 name, and
  the test files each phase names.
- **Deleted:** `cmd/awf/invariants.go`, `cmd/awf/invariants_test.go`.

## Phase 1: Gate the check family and remove the per-child gating mechanism

**Execution mode: inline.** One green transaction. Implements ADR-0199 item 13's gating half.
Deliberately does NOT touch `StateExempt`, which stays on all three children until Phase 2: the claim
that describes it also names the command spellings, and a claim carries exactly one update operation.

- [ ] **Task 1.1: Drop the `Ungated` classification from the three check children.** In
  `internal/clispec/clispec.go`, at the `prose` child (`Gating: Ungated, StateExempt: true`), the
  `memory` child, and the `commit` child, delete `Gating: Ungated,` from each, leaving
  `StateExempt: true,` in place. All three then inherit the `check` group's `Gating: Gated`. Change
  nothing else in the table.
- [ ] **Task 1.2: Remove `ResolvedGating`'s child branch and `UngatedGroupChildren`.** In
  `internal/clispec/clispec.go`, delete `UngatedGroupChildren` entirely and reduce `ResolvedGating` to
  returning the top-level command's `Gating`, or delete it and have the driver read `top.Gating`
  directly if that leaves no caller. Update `cmd/awf/main.go`'s gating call site accordingly.
  `GatedCommandNames` is unchanged. Forbidden: leaving either function present-but-unreachable or
  guarded by a `coverage-ignore`; ADR-0199 item 13 requires removal, and the dead-code gate would
  reject the former.
- [ ] **Task 1.3: Collapse the published projection to one list.** In
  `internal/project/gatedcommands.go`, remove the exclusions half: the second projection, the
  `if len(exclusions) == 0` branch and its `coverage-ignore` comment, and the "except ..." rendering.
  The generated gated-command value becomes the single list `GatedCommandNames()` returns. Verify the
  rendered agent-guide bullet loses its "except `check prose`, `check memory`, and `check commit`"
  clause after `./x render`.
- [ ] **Task 1.4: Update the tests that pin the removed shape.** `internal/clispec/clispec_test.go`'s
  `TestUngatedGroupChildren` is deleted with the function it pins. `cmd/awf/checkgroup_test.go`'s
  gating assertion (the proof marker for `tooling/cli:group-child-gating-honored`) is deleted with the
  claim. `internal/project/gatedcommands_test.go` asserts one list rather than two, and keeps its
  proof marker for `tooling/cli:gated-commands-generated`. The `cmd/awf/gate_test.go` and
  `cmd/awf/run_test.go` gated-command tables gain the three newly-gated names. Add a test asserting
  that `awf check prose` on a behind-the-project binary refuses; extend the existing
  `tooling/cli:version-compat-gate` proof rather than adding a second marker for the same claim.
- [ ] **Task 1.5: Apply ADR-0199 batch 1 and its claim mutations.** Append to ADR-0199's Status
  history, in this order, an `Accepted` event, an `Implementing` event carrying the content digest,
  and an `Applied` event reading
  `- 2026-08-01: Applied; operations: update tooling/cli:gated-commands-generated, remove tooling/cli:group-child-gating-honored`.
  In `.awf/topics/parts/tooling/cli/current-state.md`, delete the `group-child-gating-honored` claim
  block entirely, and rewrite `gated-commands-generated`'s prose so it describes one projection: the
  top-level commands whose gating classification is not ungated, with no group-children exclusion
  list. Append `ADR-0199` to `gated-commands-generated`'s `Revised-by`, preserving its existing
  `Origin`. Obtain the digest by writing 64 zeros and reading the computed value back from
  `./x check`, per the pitfalls entry on frozen digests.
- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction and create the one
  phase-closing commit; it requires `awf check --staged` and `./x gate` to pass, enforced by the wired
  pre-commit hook (confirm with `git config core.hooksPath`).

```commit
refactor(tooling): gate the whole check family and drop per-child gating
```

## Phase 2: Fork the command surface into repo and staged universes

**Execution mode: subagent-driven.** Baseline for the phase owner: start from a clean worktree on
branch `awf/fork-verification-commands-by-universe` with Phase 1 committed; `git status --short`
returns empty, `./x check` prints `awf check: clean`, and `./x gate` exits zero. This phase is large
(the respelling reaches every tracked population) and benefits from a dedicated owner.

- [ ] **Task 2.1: Restructure the `check` command table.** In `internal/clispec/clispec.go`, replace
  the flat children with two group children. `repo` holds `drift`, `state`, `prose`, `memory`;
  `staged` holds `state`, `drift`, `commit`. Delete `"--staged"` from every `BoolFlags` list under
  `check`, and from the `check` group itself. `commit` keeps `MaxPos: 1` and `StateExempt: true`;
  `prose` and `memory` lose `StateExempt` (ADR-0199 item 13: they no longer run standalone from the
  payload). `staged` and `repo` each carry `MaxPos: -1` so a leaf name reaches the handler. Rewrite
  each affected `HelpBody` so no usage line spells `--staged` and each names its universe.
- [ ] **Task 2.2: Make `resolve` return the leaf and carry the resolved path.** In
  `cmd/awf/dispatch.go`, make `resolve` descend while the next argument names a child of the current
  node, returning the deepest matched node as `cmd` and the joined child path as `sub` (for example
  `"repo prose"`). `cmdCtx.sub` becomes that joined path. `runCheckGroup` selects on it, so
  `repo state` and `staged state` no longer collide. `checkSubcommands()` enumerates the leaf set of
  the group actually addressed rather than one flattened level, and the "subcommand must come first"
  message keeps working. Forbidden: hand-parsing a third positional in the handler, which would stop
  `awf check staged commit extra-junk` being rejected by `parseArgs`.
- [ ] **Task 2.3: Make the driver's remaining per-node reads depth-correct.** In `cmd/awf/main.go`:
  `cmd.StateExempt` already reads the resolved node and is correct once Task 2.2 lands, so verify
  rather than change it; `globalHelp` recurses to any depth, indenting each level; the
  `awf help <group> <child>` path accepts a third name; and the `gateStaged` selection at the former
  `top.Name == "check" && sub == "" && inv.bools["--staged"]` predicate becomes a test on the resolved
  path being under `staged`. `guardProjectState`'s `staged` parameter derives from the same test.
- [ ] **Task 2.4: Split the handlers by universe.** Create `cmd/awf/checkrepo.go` with `runCheckRepo`
  and `cmd/awf/checkstaged.go` with `runCheckStaged` (moved from `cmd/awf/check.go`). `runCheckRepo`
  runs drift, current-state, prose, and memory, and owns both project-level notes: the version-ahead
  note and `AdvisoryNotes`, emitted once whether invoked directly or through bare `check`
  (ADR-0199 item 4). `runCheckStaged` runs the transition check and, from Phase 5, staged drift; it
  excludes `commit`. Bare `runCheck` invokes both and, outside a git repository, runs the repo
  universe alone and prints that the staged universe was unavailable. `checkLockVsBinary` loses its
  `staged` bool: `runCheckRepo` compares the working lock, `runCheckStaged` the index lock.
- [ ] **Task 2.5: Respell every authored invocation.** Batch task. Exhaustive affected-site set: every
  tracked file outside `docs/decisions/`, `docs/plans/`, `changelog/`, and `docs/research/` (which are
  append-only and keep the historical spelling) and outside the rendered outputs listed in
  `.awf/awf.lock` (which `./x render` rewrites). Representative, in `.awf/parts/workflow/local-hooks.md`:
  `` `awf check --staged` `` becomes `` `awf check staged` ``. Edge, in `.githooks/check-nested-staged`:
  `"$awf_bin" check --staged` becomes `"$awf_bin" check staged`, a shell invocation rather than prose,
  so the backtick-free form is required. Post-check:
  `git grep -F -- 'check --staged' -- . ':!docs/decisions' ':!docs/plans' ':!changelog' ':!docs/research'`
  returns no output.
- [ ] **Task 2.6: Rewrite the pre-commit payload template.** In `templates/hooks/pre-commit.sh.tmpl`,
  delete the `{{ . }} --staged` line and the `check prose` and `check memory` lines, leaving the
  configured `checkCmd` line and the gate line. Mirror the change in `x` if it hardcodes any of them.
  ADR-0199 item 1: bare check now covers both universes and both scans, so keeping the standalone
  lines would run each twice per commit.
- [ ] **Task 2.7: Update the tests the fork invalidates.** `cmd/awf/checkgroup_test.go`'s
  `TestCheckChildrenRejectStaged` is deleted: with `--staged` gone from the table there is no flag to
  reject. Add tests covering `awf check repo prose` resolving to the leaf, `repo state` and
  `staged state` dispatching differently, `globalHelp` listing grandchildren, and bare `awf check`
  degrading outside a git repository using the `bare(t)` seam in `cmd/awf/run_test.go`. Extend
  `cmd/awf/help_test.go`'s `TestCliCommandSpecSingleSource` for the deeper tree; it carries the proof
  marker for `tooling/cli:help-lists-group-children`.
- [ ] **Task 2.8: Apply ADR-0199 batch 2 and its claim mutations.** Append one `Applied` event listing,
  in declaration order: update `tooling/cli:group-child-project-guard-exemption` (prose narrows to
  `awf check staged commit` alone), update `tooling/cli:help-lists-group-children` (children at any
  depth), update `tooling/cli:invariants-in-check` (its second conjunct narrows to the current-state
  evaluation's own contribution rather than the whole command's exit status), add
  `tooling/cli:check-universe-groups` (the fork's contract: membership by subject, what each bare form
  runs, and the non-aggregating child), update `tooling/quality-gates:memory-citation-gate`,
  update `tooling/audit-and-snapshots:commit-gate-shared-rule`, and update
  `code-design/dependency-composition:dependency-composition-commit-classification` (the last three
  respellings). Each updated claim gains `ADR-0199` appended to its `Revised-by` list at its ascending
  position and preserves its `Origin`. The added claim carries `Origin: ADR-0199`, `Backing: test`, and
  a proof marker placed on the Task 2.7 test that asserts the bare aggregate's membership.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
feat(tooling): fork the check commands into repo and staged universes
```

## Phase 3: Make the staged scanners project-root correct and knob-first

**Execution mode: inline.** Implements ADR-0199 item 9. Phase 6 depends on this.

- [ ] **Task 3.1: Read the enablement knob before opening any repository.** In `cmd/awf/prosegate.go`
  and `cmd/awf/memorygate.go`, move the config load and the knob test ahead of the `stagedTree` call.
  Load the config from the project root's `.awf/config.yaml` on disk rather than from the index, so a
  disabled gate returns without requiring git at all. Behaviour required: knob off and no git
  repository returns success without scanning; knob on and no git repository still refuses with the
  existing unable-to-read-staged-files error. This ordering is what makes ADR-0199 item 4's
  degradation reach the repo universe's own index readers.
- [ ] **Task 3.2: Resolve the scan corpus and prefixes against the project root.** In both scanners,
  after obtaining the index tree, filter it to blobs under the project root's prefix relative to the
  containing repository, and rebase each blob path to project-relative before scanning. `runProseGate`
  then scans only the adopter's own tracked files, and `runMemoryGate`'s `docsDir`-derived
  `decisions/` and `plans/` prefixes match. Acceptance: from `examples/sundial`,
  `../../awf check repo prose` exits zero rather than failing with a stat error on a parent-only path,
  and a project-relative `proseGate.exemptions` entry matches.
- [ ] **Task 3.3: Cover both behaviours.** Add tests for the four combinations of knob on/off against
  git present/absent, and a nested-tree test asserting the corpus is the subtree, not the parent. The
  knob-on-without-git case carries the proof marker for
  `tooling/quality-gates:prose-gate-refuses-without-git`.
- [ ] **Task 3.4: Apply ADR-0199 batch 3.** Append one `Applied` event for
  `update tooling/quality-gates:prose-gate-refuses-without-git`, and rewrite that claim's prose to say
  the command refuses outside a git repository **when the gate is enabled**, and reports itself
  disabled without touching git when it is not. Append `ADR-0199` to its `Revised-by`.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
fix(tooling): resolve the staged scanners against the project root
```

## Phase 4: Disclose a disabled child

**Execution mode: inline.** Implements ADR-0199 item 7.

- [ ] **Task 4.1: Emit a disabled note per skipped opt-in child.** In `runCheckRepo`, when `prose` or
  `memory` is skipped because its knob is off, print one `note:` line naming the child and the knob
  that disables it, for example `note: prose: disabled (proseGate.enabled)`. The note is non-failing
  and never changes the exit code, consistent with `tooling/cli:completeness-advisory-nonfailing`.
  Directly invoking a disabled child prints the same line and exits zero.
- [ ] **Task 4.2: Cover the disclosure, and resolve the sundial ordering coupling.** Add a test
  asserting both notes appear with both knobs off and neither appears with both on, carrying the proof
  marker for the claim added in Task 4.3. `x`'s example step greps sundial's output for `^note: ` and
  fails on a hit, and sundial's knobs are not enabled until Phase 6, so this phase would otherwise
  leave `./x check` red. Resolve it by running Phase 6 Task 6.4 (enabling both knobs in
  `examples/sundial/.awf/config.yaml`, with its findings fixed) as part of this phase instead, and
  record in Notes that Task 6.4 moved. Do not weaken or scope the `^note: ` grep.
- [ ] **Task 4.3: Apply ADR-0199 batch 4.** Append one `Applied` event for
  `add tooling/cli:check-disabled-child-disclosure`, and author the claim with `Origin: ADR-0199`,
  `Backing: test`, and its proof marker on the Task 4.2 test.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
feat(tooling): disclose a disabled opt-in check
```

## Phase 5: Add staged drift

**Execution mode: inline.** Implements ADR-0199 item 5, in its positively-bounded scope.

- [ ] **Task 5.1: Render from the staged config and compare against the staged output tree.** Add the
  staged drift evaluation and wire it into `runCheckStaged`. Reuse the existing snapshot-backed
  readers rather than building new machinery: `config.TreeReader`, `project.ProjectTreeReader` with
  `snapshotTreeReader`, and `StagedContextState`'s assembly of staged config, corpora, and lock.
  Emit exactly the stale and hand-edited comparison of re-rendered bytes against the staged output
  tree. Forbidden, per ADR-0199 item 5: the config-tree hygiene sweep, the dead-reference probe,
  stale-backup flagging, invalid-frontmatter drift, orphaned-path drift, and provenance-banner or
  managed-output-attribution checks. Watch the known trap: `topicHash` reads absolute paths while the
  tree loader stores repo-relative ones, which produces spurious `stale` on every topic doc if
  unhandled.
- [ ] **Task 5.2: Cover the hole it closes.** Add a test staging a `.awf/` config change without its
  re-rendered output and asserting `awf check staged` reports drift, plus a test asserting a fully
  staged render is clean. The first carries the proof marker for the claim added in Task 5.3.
- [ ] **Task 5.3: Apply ADR-0199 batch 5.** Append one `Applied` event for
  `add rendering/sync-and-drift:staged-drift-rendered-output`, and author the claim with
  `Origin: ADR-0199`, `Backing: test`, its proof marker, and prose naming both what it emits and that
  every other drift kind is out of scope.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
feat(rendering): compare staged config against the staged output tree
```

## Phase 6: Remove check invariants and migrate adopter vars

**Execution mode: inline.** Implements ADR-0199 items 6, 11, and 12. Item 8's sundial enablement moves
to Phase 4 Task 4.2 for the ordering reason recorded there.

- [ ] **Task 6.1: Delete the command and its only production callers.** Delete `cmd/awf/invariants.go`
  and `cmd/awf/invariants_test.go`, remove the `invariants` child from `internal/clispec/clispec.go`,
  and delete `Project.CurrentStateInvariants` and `InvariantReport` from
  `internal/project/currentstate.go`. Remove `x`'s `(cd examples/sundial && "$bindir/awf" check invariants)`
  line. Remove the false descriptions at `README.md:185` and `README.md:278` and the mermaid node at
  `README.md:159` rather than correcting them. Delete the `check invariants` line from
  `templates/docs/working-with-awf.md.tmpl` and the sentence describing the standalone report from
  `.awf/domains/parts/invariants/current-state.md`. Also update
  `examples/sundial/.awf/docs/parts/testing/layout.md` and `internal/project/example_wiring_test.go`,
  which pins the removed `x` line verbatim. Verify with
  `git grep -F 'check invariants' -- . ':!docs/decisions' ':!docs/plans' ':!changelog'` returning no
  output, and `./x gate`'s dead-code step passing.
- [ ] **Task 6.2: Add the chained migration.** Create `internal/migrate/retargetcheckcommands.go` at
  the next schema generation, registered in `internal/migrate/migrate.go`. It retargets `check prose`
  to `check repo prose`, `check memory` to `check repo memory`, and `check commit` to
  `check staged commit`, and clears a var whose value invokes `check invariants`. It matches a
  three-token invocation; `retiredCommandRe` is anchored to a two-token form and must not be copied.
  Per ADR-0199 item 11 it does NOT match the bare `invariants` spelling, which the 18-to-19 migration
  already rewrote and which otherwise belongs to a non-awf runner's vocabulary. Leave
  `internal/migrate/renameretiredcommands.go` untouched. Add
  `internal/migrate/retargetcheckcommands_test.go` covering each retarget, the clear, idempotent
  replay, and a value naming another runner being left alone.
- [ ] **Task 6.3: Update the authored inputs whose descriptions the fork changes.** Per ADR-0199 item
  12: replace the superseded follow-on entry at `.awf/docs/parts/roadmap/deferred.md`, landing the
  carried-forward check-architecture cleanup in `.awf/docs/parts/roadmap/ideas.md`; resolve the
  deferred entry noting `awf check drift` and `awf check state` as uninvoked; and take the semantic
  update in `.awf/parts/workflow/composing-the-gate.md` and `.awf/docs/parts/testing/gate.md`, both of
  which describe the two scans as separate non-gate steps the payload runs on its own.
- [ ] **Task 6.4: Apply ADR-0199 batch 6, the final batch.** Append one `Applied` event for
  `update tooling/quality-gates:example-adopter-checked`, and rewrite that claim's prose to drop
  `awf check invariants` from what `./x check` runs inside the example and to record that the example
  runs with both opt-in gates enabled. Append `ADR-0199` to its `Revised-by`. Do NOT flip ADR-0199 to
  `Implemented` here: that flip and the plan's own freeze belong to the deferred
  post-terminal-review transaction.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
feat(tooling): remove check invariants and migrate adopter vars
```

## Verification

- `git grep -F -- 'check --staged' -- . ':!docs/decisions' ':!docs/plans' ':!changelog' ':!docs/research'`
  returns no output, and the same for `git grep -F 'check invariants'` with the same exclusions.
- `awf check repo`, `awf check staged`, and bare `awf check` each exit zero on a clean tree; bare
  `awf check` in a non-git directory exits zero having reported the staged universe unavailable.
- `awf check staged` reports drift when a `.awf/` change is staged without its rendered output.
- From `examples/sundial`, both `../../awf check repo prose` and `../../awf check repo memory` exit
  zero, and `./x check` exits zero with no `note:` line from the example.
- `./x gate` passes, including the 100% coverage floor with no new `coverage-ignore` beyond any this
  plan names, and the dead-code step with `runInvariants`, `CurrentStateInvariants`, `InvariantReport`,
  and `UngatedGroupChildren` all gone.
- ADR-0199 carries `Accepted`, `Implementing`, and six `Applied` events whose operations, concatenated
  in order, equal its declared thirteen.

## Notes

- Phase 4 Task 4.2 absorbs the sundial enablement that ADR-0199 item 8 describes, because `x`'s
  zero-notes rule for the example would otherwise go red between Phase 4 and Phase 6. Phase 6 keeps
  the claim update for the example adopter, since that operation also carries the `check invariants`
  removal.
- ADR-0199 item 5 defers the sweep and dead-reference halves of staged drift. The carried-forward
  check-architecture cleanup lands as a roadmap entry in Task 6.3 and is not implemented here.
- The three added claims' prose is authored in the phase that applies its operation, not in advance;
  ADR-0199 declares the operations, and the claim text lands with the mutation.
