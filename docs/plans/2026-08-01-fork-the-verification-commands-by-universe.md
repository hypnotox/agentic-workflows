---
date: 2026-08-01
adrs: [210]
status: Proposed
---
# Plan: Fork the verification commands by universe

## Goal

Execute ADR-0210: fork `awf check` into `check repo` and `check staged` universe sub-groups, delete
the `--staged` flag and the `check invariants` command, add a narrow `check staged drift`, gate the
whole check family, and make the two staged scanners consult their knob before reading any index.
Non-goals: the
config-tree hygiene sweep and dead-reference halves of staged drift, and the wider check-architecture
cleanup, both of which ADR-0210 defers by name.

## Architecture summary

Five phases, each one green transaction closing in one commit. Phase 1 removes a mechanism, which
shrinks the surface Phase 2 has to make depth-aware. Phase 2 is the fork, carries the bulk of the
respelling, and includes the scanners' knob-first fix, which cannot follow it: once bare check runs
the repo universe, a scanner that opens a repository before reading its knob fails on every non-git
tree. Phase 3 adds staged drift. Phase 4 removes the dead command, rescopes the example invocation,
and turns on the example's gates, which must be one transaction because one claim describes all
three. Phase 5 discloses a disabled child last, because that disclosure is what the example's
now-enabled gates keep quiet.

ADR-0210 is `Proposed`. Phase 1 appends its `Accepted` and `Implementing` status events and Applied
batch 1; Phases 2 to 5 each append one further Applied batch in declaration order. The batch
partition is 2 / 11 / 1 / 2 / 1 across the seventeen declared operations, and the ADR's State changes
list is sequenced to match. Every batch task also flips the ADR's frontmatter `status:` field, which
the parser cross-checks against the latest Status history entry. The `Implemented` flip lands in
Phase 5's transaction alongside the final Applied batch, because an `Implementing` status with no
remaining operations is rejected. The plan's own `status: Implemented` freeze is separate and is
deferred to the post-terminal-review transaction.

## File structure

- **Created:** `cmd/awf/checkrepo.go`, `cmd/awf/checkstaged.go`, `internal/migrate/retargetcheckcommands.go`,
  `internal/migrate/retargetcheckcommands_test.go`. The three added claims land inside existing topic
  files, not new ones.
- **Modified:** `internal/clispec/clispec.go`, `cmd/awf/dispatch.go`, `cmd/awf/main.go`,
  `cmd/awf/check.go`, `cmd/awf/gate.go`, `cmd/awf/prosegate.go`, `cmd/awf/memorygate.go`,
  `internal/project/gatedcommands.go`, `internal/project/currentstate.go`,
  `internal/project/validate.go`, `internal/catalog/standard.go`, `internal/configspec/spec.go`,
  `internal/migrate/migrate.go`, `.awf/config.yaml`, `x`, `README.md`, `.githooks/check-nested-staged`,
  `templates/hooks/pre-commit.sh.tmpl`, `templates/docs/working-with-awf.md.tmpl`,
  `examples/sundial/.awf/config.yaml`, the authored `.awf/` inputs ADR-0210 items 6 and 12 name, the
  test files each phase names, and the nine topic-claim part files the batches mutate:
  `.awf/topics/parts/tooling/cli/current-state.md`,
  `.awf/topics/parts/tooling/quality-gates/current-state.md`,
  `.awf/topics/parts/tooling/audit-and-snapshots/current-state.md`,
  `.awf/topics/parts/code-design/dependency-composition/current-state.md`,
  `.awf/topics/parts/rendering/catalog-and-targets/current-state.md`,
  `.awf/topics/parts/config/validation/current-state.md`,
  `.awf/topics/parts/rendering/companion-scripts/current-state.md`,
  `.awf/topics/parts/rendering/sync-and-drift/current-state.md`, and
  `.awf/topics/parts/invariants/current-state-authority/current-state.md`.
- **Deleted:** `cmd/awf/invariants.go`, `cmd/awf/invariants_test.go`.

## Phase 1: Gate the check family and remove the per-child gating mechanism

**Execution mode: inline.** One green transaction. Implements ADR-0210 item 13's gating half.
Deliberately does NOT touch `StateExempt`, which stays on all three children until Phase 2: the claim
that describes it also names the command spellings, and a claim carries exactly one update operation.

- [ ] **Task 1.1: Drop the `Ungated` classification from the three check children.** In
  `internal/clispec/clispec.go`, at the `prose` child (`Gating: Ungated, StateExempt: true`), the
  `memory` child, and the `commit` child, delete `Gating: Ungated,` from each, leaving
  `StateExempt: true,` in place. All three then inherit the `check` group's `Gating: Gated`. Change
  nothing else in the table.
- [ ] **Task 1.2: Remove `ResolvedGating`'s child branch and `UngatedGroupChildren`.** In
  `internal/clispec/clispec.go`, delete `UngatedGroupChildren` and `ResolvedGating` entirely, and have
  `cmd/awf/main.go`'s gating call site read `top.Gating` directly. After Task 1.1 that call site is
  `ResolvedGating`'s only production caller; its remaining callers are in tests this phase deletes or
  rewrites, so a reduced-to-passthrough `ResolvedGating` would be indirection with no caller to serve.
  `cmd/awf/gate_test.go` loses its `ResolvedGating` assertion with the function.
  `GatedCommandNames` is unchanged. Forbidden: leaving either function present-but-unreachable or
  guarded by a `coverage-ignore`; ADR-0210 item 13 requires removal, and the dead-code gate would
  reject the former.
- [ ] **Task 1.3: Collapse the published projection to one list.** In
  `internal/project/gatedcommands.go`, remove the exclusions half: the second projection, the
  `if len(exclusions) == 0` branch and its `coverage-ignore` comment, and the "except ..." rendering.
  The generated gated-command value becomes the single list `GatedCommandNames()` returns. Verify the
  rendered agent-guide bullet loses its "except `check prose`, `check memory`, and `check commit`"
  clause after `./x render`.
- [ ] **Task 1.4: Update the tests that pin the removed shape.** `internal/clispec/clispec_test.go`'s
  `TestUngatedGroupChildren` is deleted with the function it pins. `tooling/cli:group-child-gating-honored`
  has TWO proof markers and both are deleted with the claim: `internal/clispec/clispec_test.go`'s
  `TestResolvedGating` (marker at :50) and `cmd/awf/checkgroup_test.go`'s
  `TestCheckUngatedChildrenRunOnSchemaAheadProject` (marker at :172). Leaving either orphaned fails
  `awf check`. `internal/project/gatedcommands_test.go` asserts one list rather than two, and keeps its
  proof marker for `tooling/cli:gated-commands-generated`. The `cmd/awf/gate_test.go` and
  `cmd/awf/run_test.go` gated-command tables gain the three newly-gated names. Add a test asserting
  that `awf check prose` on a behind-the-project binary refuses; extend the existing
  `tooling/cli:version-compat-gate` proof rather than adding a second marker for the same claim.
- [ ] **Task 1.5: Apply ADR-0210 batch 1 and its claim mutations.** Append to ADR-0210's Status
  history, in this order, an `Accepted` event, an `Implementing` event, and an `Applied` event.
  EVERY non-Proposed status entry carries a `content-sha256`, `Accepted` included, and `Implementing`
  repeats the stamp `Accepted` established rather than recomputing it. Flip the frontmatter
  `status: Proposed` to `status: Implementing`; the parser rejects a history whose latest status
  disagrees with the frontmatter. Each operation in an `Applied` line is wrapped in backticks or the
  history fails to parse, so the line reads exactly:

  ```
  - 2026-08-01: Applied; operations: update `tooling/cli:gated-commands-generated`, remove `tooling/cli:group-child-gating-honored`
  ```

  One line, each id inside an inline code span, matching the shipped form in
  `docs/decisions/0195-*.md`. Every later batch task's `Applied` line takes the same form.
  In `.awf/topics/parts/tooling/cli/current-state.md`, delete the `group-child-gating-honored` claim
  block entirely, and rewrite `gated-commands-generated`'s prose so it describes one projection: the
  top-level commands whose gating classification is not ungated, with no group-children exclusion
  list. Append `ADR-0210` to `gated-commands-generated`'s `Revised-by`, preserving its existing
  `Origin`. Obtain the digest by writing 64 zeros and reading the computed value back from
  `./x check`, per the pitfalls entry on frozen digests. Regenerate `docs/decisions/INDEX.md` with
  `./x render` and stage it with the flip; the index records the ADR's status and is never
  hand-edited.
- [ ] **Phase-close: render, stage, check, gate, and commit.** Run `./x render` and stage every
  regenerated output with the change that caused it: each batch task mutates
  `.awf/topics/parts/**/current-state.md`, which renders into `docs/`, and each ADR status flip
  changes `docs/decisions/INDEX.md`. Then stage the complete transaction and create the one
  phase-closing commit; it requires the staged check and `./x gate` to pass, enforced by the wired
  pre-commit hook (confirm with `git config core.hooksPath`). Phase 1 still spells the staged check
  `awf check --staged`; from Phase 2 onward it is `awf check staged`.

```commit
refactor(tooling): gate the whole check family and drop per-child gating
```

## Phase 2: Fork the command surface into repo and staged universes

**Execution mode: subagent-driven.** Baseline for the phase owner: start from a clean worktree on
branch `awf/fork-verification-commands-by-universe` with Phase 1 committed; `git status --short`
returns empty, `./x check` prints `awf check: clean`, and `./x gate` exits zero. This phase is large
(the respelling reaches every tracked population) and benefits from a dedicated owner.

- [ ] **Task 2.1: Restructure the `check` command table.** In `internal/clispec/clispec.go`, replace
  the flat children with two group children plus one carried-over leaf. `repo` holds `drift`,
  `state`, `prose`, `memory`; `staged` holds `state`, `drift`, `commit`. `invariants` STAYS a direct
  child of `check`, unmoved and unrespelled, until Phase 4 deletes it. Rehoming it here would respell
  the `x` invocation that `tooling/quality-gates:example-adopter-checked` names verbatim, and that
  claim takes its single update in Phase 4 alongside two other changes to the same sentence. Delete `"--staged"` from every `BoolFlags` list under
  `check`, and from the `check` group itself. `commit` keeps `MaxPos: 1` and `StateExempt: true`;
  `prose` and `memory` lose `StateExempt` (ADR-0210 item 13: they no longer run standalone from the
  payload). `staged` and `repo` each carry `MaxPos: -1` so a leaf name reaches the handler. Rewrite
  each affected `HelpBody` so no usage line spells `--staged` and each names its universe.
- [ ] **Task 2.2: Make `resolve` return the leaf and carry the resolved path.** In
  `cmd/awf/dispatch.go`, make `resolve` descend while the next argument names a child of the current
  node, returning the deepest matched node as `cmd` and the joined child path as `sub` (for example
  `"repo prose"`). `cmdCtx.sub` becomes that joined path. `runCheckGroup` selects on it, so
  `repo state` and `staged state` no longer collide. `checkSubcommands()` enumerates the leaf set of
  the group actually addressed rather than one flattened level. State the enumeration rule exactly,
  because `check` is now a mixed group (`repo` and `staged` are groups, `invariants` is a leaf until
  Phase 4): for a group addressed directly, enumerate its immediate child names; for a universe group
  addressed directly, enumerate its own leaf names. A naive flattening would print
  `drift, state, prose, memory, state, drift, commit, invariants`, duplicating two names with no
  universe qualifier, which is worse than today's message. The "subcommand must come first" message
  keeps working. Forbidden: hand-parsing a third positional in the handler, which would stop
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
  (ADR-0210 item 4). `runCheckStaged` runs the transition check and, from Phase 3, staged drift; it
  excludes `commit`. Bare `runCheck` invokes both and, outside a git repository, runs the repo
  universe alone and prints that the staged universe was unavailable. `checkLockVsBinary` loses its
  `staged` bool: `runCheckRepo` compares the working lock, `runCheckStaged` the index lock.
- [ ] **Task 2.5: Respell every authored invocation.** Batch task. Exhaustive affected-site set: every
  tracked file outside `docs/decisions/`, `docs/plans/`, `changelog/`, and `docs/research/` (which are
  append-only and keep the historical spelling) and outside the rendered outputs listed in
  `.awf/awf.lock` (which `./x render` rewrites). `.awf/parts/workflow/local-hooks.md` is
  EXCLUDED from this batch and handled by Task 2.9, because it enumerates payload steps Task 2.6
  deletes rather than respells. Representative, in `.awf/domains/parts/adr-system/current-state.md`:
  `` `awf check --staged` `` becomes `` `awf check staged` ``. Three edges, each a different
  transformation shape: in `.githooks/check-nested-staged`, `"$awf_bin" check --staged` becomes
  `"$awf_bin" check staged`, a shell invocation with no backticks; in `internal/worktree/manager.go`
  (around :380-382), the user-facing refusal and next-action string literals take the new spelling;
  and in `internal/project/phase_transaction_ownership_test.go` (around :81-110), a golden pin
  including an exact `strings.Count(phase, "awf check --staged") != 1` assertion takes it. Also in the
  set and not prose: `internal/project/spine_test.go` (around :307 and :895-910) and
  `internal/evals/chain_test.go` (around :224 and :356). Post-check:
  `git grep -F -- 'check --staged' -- . ':!docs/decisions' ':!docs/plans' ':!changelog' ':!docs/research'`
  returns no output.
- [ ] **Task 2.6: Rewrite the pre-commit payload template.** In `templates/hooks/pre-commit.sh.tmpl`,
  delete the `{{ . }} --staged` line and the `check prose` and `check memory` lines, leaving the
  configured `checkCmd` line and the gate line. Mirror the change in `x` if it hardcodes any of them.
  `internal/project/hooks_test.go` pins the payload's four lines verbatim (around :69, and the
  runner-var variant around :112) and is the proof site for
  `rendering/companion-scripts:hook-payloads-fallback-safe`; update it to pin the new two-line set,
  remove `proseGateCmd` and `memoryGateCmd` from the configured-command fixture, and rewrite the
  claim prose to name only command vars that still have payload consumers. This is a deletion rather
  than a respelling, so Task 2.5's batch does not cover it. ADR-0210 item 1: bare check now covers
  both universes and both scans, so keeping the standalone lines would run each twice per commit.
- [ ] **Task 2.6b: Retire the two orphaned scan-command vars.** Delete the `proseGateCmd` and
  `memoryGateCmd` descriptors from `internal/catalog/standard.go` and their availability entries from
  `internal/configspec/spec.go`; do not rewrite either description to claim a replacement consumer.
  Remove both keys from this repository's `.awf/config.yaml` rather than respelling their now-inert
  values. In `internal/project/descriptor_parity_test.go`, remove both keys from `functionalVarKeys`
  and rewrite the comments that call out their introductions, while keeping the exact-set assertion and
  its proof marker. In `internal/project/validate.go`, narrow the runner-disabled hook-command loop
  from four vars to `checkCmd` and `commitGateCmd`, because those are the only awf-verb vars the
  remaining hook payloads consume; update its comments. In `internal/project/validate_test.go`,
  delete the two obsolete refusal cases, remove both keys from the runner-less valid fixture, and
  keep the fixed-order assertions for `checkCmd` then `commitGateCmd`. Render the catalog and config
  reference after these authored removals. Acceptance: outside frozen history and migration tests,
  neither retired key appears as a live descriptor, availability entry, hook requirement, or payload
  fixture.
- [ ] **Task 2.7: Update the tests the fork invalidates.** `cmd/awf/checkgroup_test.go`'s
  `TestCheckChildrenRejectStaged` is deleted: with `--staged` gone from the table there is no flag to
  reject. Add tests covering `awf check repo prose` resolving to the leaf, `repo state` and
  `staged state` dispatching differently, `globalHelp` listing grandchildren, and bare `awf check`
  degrading outside a git repository using the `bare(t)` seam in `cmd/awf/run_test.go`. The
  grandchild-recursion test is `cmd/awf/help_test.go`'s `TestHelpListsGroupChildren` (around :81),
  which walks `clispec.Commands` one level deep and carries the proof marker for
  `tooling/cli:help-lists-group-children`; extend it to any depth.
  `cmd/awf/checkgroup_test.go`'s `TestHelpListsCheckChildren` (around :379) pins the flat child list
  and is updated with it. `TestCliCommandSpecSingleSource` backs a different claim
  (`tooling/cli:cli-command-spec-single-source`) and is not the site for this one. The gated-command
  probe tables also hold argv slices that the fork invalidates: `cmd/awf/gate_test.go`'s
  `gatedProbes` map (around :221-232, entries like `{"awf","check","prose"}`), its parallel table in
  `cmd/awf/run_test.go`, and the refusal test Task 1.4 adds all take the new argv paths
  `check repo prose`, `check repo memory`, and `check staged commit`.
- [ ] **Task 2.7b: Respell the three subcommands the fork renames.** Batch task, separate from Task
  2.5 because the transformation and the affected population differ. Every authored live site naming
  `check prose`, `check memory`, or `check commit` takes `check repo prose`, `check repo memory`, or
  `check staged commit`; the two retired keys and their values are deleted by Task 2.6b instead.
  Representative, a config var: `.awf/config.yaml`'s
  `commitGateCmd: ./awf check commit` becomes `./awf check staged commit`; this one is load-bearing
  for the phase itself, because that var renders `.awf/hooks/commit-msg.sh` and this repository wires
  `core.hooksPath`, so leaving it stale makes the phase's own closing commit fail its own commit-msg
  hook. Three edges: a CI step (`.github/workflows/ci.yml`, around :30-31), the surviving
  `commitGateCmd` catalog descriptor (`internal/catalog/standard.go`, around :248), and
  command-related config-spec prose outside the two availability entries Task 2.6b deletes. The
  retired scan-command descriptors are removed, not respelled. Also in the set:
  `templates/docs/workflow.md.tmpl` (around :81), `README.md` (around :282-284 and :308),
  `.awf/parts/workflow/commit-discipline.md` (around :5), and `.awf/docs/pitfalls.yaml` (around :447
  and :1402). Rewrite `templates/docs/working-with-awf.md.tmpl`'s whole flat check-command list rather
  than only those three entries: document bare `check`, the `check repo` aggregate and its `drift`,
  `state`, `prose`, and `memory` children, and the `check staged` aggregate with `state` plus the
  directly-invoked `commit`; Phase 3 adds `staged drift` when it becomes live. Rewrite README.md's
  command-table overview to the same hierarchy instead of leaving its flat `drift`, `state`, and
  `invariants` list as the organizing model. Decide explicitly and record in
  the commit body whether the `check prose:` and `check memory:` output prefixes in
  `cmd/awf/prosegate.go` and `cmd/awf/memorygate.go` change; they are user-facing strings, not
  invocations. Post-check:
  `git grep -nE 'check (prose|memory|commit)\b' -- . ':!docs/decisions' ':!docs/plans' ':!changelog' ':!docs/research' ':!internal/migrate/renameretiredcommands.go' ':!internal/migrate/renameretiredcommands_test.go'`
  returns no output; the migrate exclusions are required because ADR-0210 item 11 freezes those
  two-token spellings.
- [ ] **Task 2.8: Rewrite the authored inputs whose payload description the fork retires.** Three
  authored parts describe the payload shape Task 2.6 deletes, and go false in this phase rather than a
  later one: `.awf/parts/workflow/local-hooks.md` (which enumerates the payload's steps),
  `.awf/parts/workflow/composing-the-gate.md` (around :9-14), and `.awf/docs/parts/testing/gate.md`
  (around :11-12), the last two describing the scans as separate non-gate steps the payload runs on
  its own. Rewrite all three to the payload's new shape: the configured check command plus the gate.
  Semantically rewrite README.md's hook paragraph in the same transaction: pre-commit runs only the
  configured bare aggregate check and project gate, while commit-msg owns the direct staged commit
  check. Do not merely respell the deleted staged and standalone scan lines.
- [ ] **Task 2.9: Make both staged scanners knob-first.** ADR-0210 item 9. In `cmd/awf/prosegate.go`
  and `cmd/awf/memorygate.go`, move the config load and the knob test ahead of the `stagedTree` call,
  loading the config from the project root's `.awf/config.yaml` on disk so a disabled gate returns
  without requiring git at all. Behaviour required: knob off and no git repository returns success
  without scanning; knob on and no git repository still refuses with the existing
  unable-to-read-staged-files error. This lands in THIS phase, not a later one: Task 2.4 makes bare
  check run the repo universe, and `cmd/awf/initrender_test.go` calls `runCheck` on a freshly
  scaffolded non-git tree, so a scanner that opens a repository first fails the phase's own gate.
  Nothing else about these scanners changes; `internal/git/handle.go` already reroots the index tree
  to the project subtree, so no corpus or prefix filtering is added. Add tests for the four
  combinations of knob on/off against git present/absent; the knob-on-without-git case carries the
  proof marker for `tooling/quality-gates:prose-gate-refuses-without-git`.
- [ ] **Task 2.10: Apply ADR-0210 batch 2 and its claim mutations.** Append one `Applied` event listing
  these eleven operations in declaration order: update
  `tooling/cli:group-child-project-guard-exemption` (prose narrows to `awf check staged commit` alone),
  update `tooling/cli:help-lists-group-children` (children at any depth), update
  `tooling/cli:invariants-in-check` (its second conjunct narrows to the current-state evaluation's
  own contribution rather than the whole command's exit status), add
  `tooling/cli:check-universe-groups` (the fork's contract: membership by subject, what each bare form
  runs, and the non-aggregating child), update
  `rendering/catalog-and-targets:var-descriptor-set-pinned` (remove `proseGateCmd` and
  `memoryGateCmd` from the exact functional set), update `config/validation:hooks-commands-resolvable`
  (runner-disabled hooks require only `checkCmd` and `commitGateCmd`), update
  `rendering/companion-scripts:hook-payloads-fallback-safe` (the pre-commit payload is the configured
  check plus gate and no longer consumes either retired var), update
  `tooling/quality-gates:memory-citation-gate`, update
  `tooling/audit-and-snapshots:commit-gate-shared-rule`, update
  `code-design/dependency-composition:dependency-composition-commit-classification` (these last three
  respell the live commands), and update `tooling/quality-gates:prose-gate-refuses-without-git`, whose
  prose narrows to refusing outside a git repository WHEN THE GATE IS ENABLED and reporting itself
  disabled without touching git when it is not. Each updated claim gains `ADR-0210` appended to its
  `Revised-by` list at its ascending position and preserves its `Origin`. The added claim carries
  `Origin: ADR-0210`, `Backing: test`, and a proof marker placed on the Task 2.7 test that asserts the
  bare aggregate's membership. Flip the ADR frontmatter only if it is not already `Implementing`.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
feat(tooling): fork the check commands into repo and staged universes
```

## Phase 3: Add staged drift

**Execution mode: inline.** Implements ADR-0210 item 5, in its positively-bounded scope.

- [ ] **Task 3.1: Render from the staged config and compare against the staged output tree.** Add
  `(*Project) CheckStagedDrift(ctx) ([]Drift, error)` to `internal/project`, reusing the existing
  comparison and returning only the stale and hand-edited kinds; `runCheckStaged` invokes it and
  formats the result. Forbidden: filtering drift kinds in `cmd/awf`. The drift model is owned by
  `internal/project`, so drift-kind policy stays there and the command binary keeps only
  presentation. Reuse the existing snapshot-backed readers rather than building new machinery:
  `config.TreeReader`, `project.ProjectTreeReader` with `snapshotTreeReader`, and
  `StagedContextState`'s assembly of staged config, corpora, and lock. Also forbidden, per ADR-0210
  item 5: the config-tree hygiene sweep, the dead-reference probe,
  stale-backup flagging, invalid-frontmatter drift, orphaned-path drift, and provenance-banner or
  managed-output-attribution checks. Watch the known trap: `topicHash` reads absolute paths while the
  tree loader stores repo-relative ones, which produces spurious `stale` on every topic doc if
  unhandled.
- [ ] **Task 3.2: Cover and document the hole it closes.** Add a test staging a `.awf/` config change
  without its re-rendered output and asserting `awf check staged` reports drift, plus a test asserting
  a fully staged render is clean. The first carries the proof marker for the claim added in Task 3.3.
  In the same transaction, add `check staged drift` to the universe hierarchy documented in
  `templates/docs/working-with-awf.md.tmpl` and README.md, naming its rendered-output-only scope so
  neither document implies parity with `check repo drift`.
- [ ] **Task 3.3: Apply ADR-0210 batch 3.** Append one `Applied` event for
  `add rendering/sync-and-drift:staged-drift-rendered-output`, and author the claim with
  `Origin: ADR-0210`, `Backing: test`, its proof marker, and prose naming both what it emits and that
  every other drift kind is out of scope.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
feat(rendering): compare staged config against the staged output tree
```

## Phase 4: Remove check invariants, rescope the example, and migrate adopter vars

**Execution mode: inline.** Implements ADR-0210 items 6, 8, 11, and 12. These land together because
one claim, `tooling/quality-gates:example-adopter-checked`, describes all three of what `./x check`
runs inside the example, which command it names, and whether the example's gates are on. A claim takes
exactly one update, so the three changes cannot be split across phases without leaving it false in
between. No ordering dependency on the scanners: they are already correct in a nested tree, and
Phase 2's knob-first change binds only when a gate is off, which after this phase neither is.

- [ ] **Task 4.1: Delete the command and its only production callers.** Delete `cmd/awf/invariants.go`
  and `cmd/awf/invariants_test.go`, remove the `invariants` child from `internal/clispec/clispec.go`,
  and delete `Project.CurrentStateInvariants` and `InvariantReport` from
  `internal/project/currentstate.go`. Remove `x`'s `(cd examples/sundial && "$bindir/awf" check invariants)`
  line. Remove the false descriptions at `README.md:185` and `README.md:278` rather than
  correcting them, which is what ADR-0210 item 6 authorises. The mermaid node at `README.md:159`
  (`CHECK[["awf check /<br/>check invariants"]]`) is CORRECTED to `CHECK[["awf check"]]`, not deleted:
  its `awf check` half is true and stays true, and deleting the node would leave the diagram with no
  edge from invariant claims to any checker. Delete the `check invariants` line from
  `templates/docs/working-with-awf.md.tmpl` and the sentence describing the standalone report from
  `.awf/domains/parts/invariants/current-state.md`. Also update
  `examples/sundial/.awf/docs/parts/testing/layout.md` and `internal/project/example_wiring_test.go`,
  which pins the removed `x` line verbatim. Two further test files reference the deleted symbols and
  must be updated or the phase does not compile: `cmd/awf/run_test.go` (the `runInvariants` sites
  around :280, :799, :827, :836, and :1069-1070, including the dependency-composition wiring-table row
  `{file: "invariants.go", owner: "runInvariants", ...}`), `internal/project/currentstate_test.go`
  (delete the removed report API's tests, then replace the
  `invariants/current-state-authority:invariants-zero-slugs-clean` proof with a focused current-state
  check asserting that a project with no invariant claims loads and reports no findings or error),
  and `cmd/awf/gate_test.go` (the `"check invariants"` gated-command table entry around :226). Verify with
  `git grep -F 'check invariants' -- . ':!docs/decisions' ':!docs/plans' ':!changelog' ':!internal/migrate/renameretiredcommands.go' ':!internal/migrate/renameretiredcommands_test.go'`
  returning no output; the two migrate files are excluded because ADR-0210 item 11 freezes the
  18-to-19 migration, which contractually keeps the retired spelling. Also confirm `./x gate`'s
  dead-code step passes.
- [ ] **Task 4.2: Rescope the example invocation and enable its gates.** In `x` (around :90), the
  `examples/sundial` invocation becomes `awf check repo`; ADR-0210 item 8 requires it because sundial
  is a nested tree whose staged universe would otherwise be evaluated against the containing
  repository's index. Update the verbatim pin in `internal/project/example_wiring_test.go`
  (around :105) to match. Then add to `examples/sundial/.awf/config.yaml`, which declares neither
  block today:

  ```yaml
  proseGate:
    enabled: true
  memoryCite:
    enabled: true
  ```

  Run `./x check` and fix every finding inside the example tree rather than by adding exemptions,
  unless a finding is a genuine depiction, in which case add the narrowest exemption and say so in the
  commit body. Acceptance: `./x check` exits zero with no `note:` line in the example's output, which
  is what `tooling/quality-gates:example-zero-notes` requires and what Phase 5 depends on.
- [ ] **Task 4.3: Add the chained migration.** Create `internal/migrate/retargetcheckcommands.go` at
  the next schema generation, registered in `internal/migrate/migrate.go`. Before inspecting any var
  value, delete the `proseGateCmd` and `memoryGateCmd` keys themselves whatever values they hold.
  For every other var, retarget `check prose` to `check repo prose`, `check memory` to
  `check repo memory`, and `check commit` to `check staged commit`, preserving trailing arguments,
  and clear a var whose value invokes `check invariants`. It matches a three-token invocation;
  `retiredCommandRe` is anchored to a two-token form and must not be copied. Per ADR-0210 item 11 it
  does NOT match the bare `invariants` spelling, which the 18-to-19 migration already rewrote and
  which otherwise belongs to a non-awf runner's vocabulary. Leave
  `internal/migrate/renameretiredcommands.go` untouched. Add
  `internal/migrate/retargetcheckcommands_test.go` covering unconditional deletion of each retired
  key (including arbitrary and already-respelled values), each retarget in a different surviving var,
  trailing-argument preservation, the invariants clear, idempotent replay, and a value naming another
  runner being left alone.
- [ ] **Task 4.4: Update the authored inputs whose descriptions the fork changes.** Per ADR-0210 item
  12: replace the superseded follow-on entry at `.awf/docs/parts/roadmap/deferred.md`, landing the
  carried-forward check-architecture cleanup in `.awf/docs/parts/roadmap/ideas.md`; resolve the
  deferred entry noting `awf check drift` and `awf check state` as uninvoked. The two gate-composition
  parts are NOT here: Task 2.8 rewrites them in the phase that retires what they describe.
- [ ] **Task 4.5: Apply ADR-0210 batch 4.** Append one `Applied` event listing, in declaration order,
  `update invariants/current-state-authority:invariants-zero-slugs-clean` and
  `update tooling/quality-gates:example-adopter-checked`. Rewrite `invariants-zero-slugs-clean` to
  state that a project declaring no invariant claims loads successfully and `awf check repo state`
  reports no backing findings or error; preserve its Origin, append `ADR-0210` to `Revised-by`, and
  place its proof marker on Task 4.1's focused current-state check. Rewrite `example-adopter-checked`
  to drop `awf check invariants` from what `./x check` runs inside the example, record that `./x check`
  invokes the repo universe there, and record that the example runs with both opt-in gates enabled.
  Append `ADR-0210` to its `Revised-by`. Those three example changes are why that claim's single
  update lives here.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
feat(tooling): remove check invariants and rescope the example adopter
```

## Phase 5: Disclose a disabled child

**Execution mode: inline.** Implements ADR-0210 item 7, and closes the ADR. Last because Task 5.1
makes a disabled child emit a `note:` line and `x` fails the example on any such line, so the
example's gates (Phase 4 Task 4.2) must already be on.

- [ ] **Task 5.1: Emit a disabled note per skipped opt-in child.** In `runCheckRepo`, when `prose` or
  `memory` is skipped because its knob is off, print one `note:` line naming the child and the knob
  that disables it, for example `note: prose: disabled (proseGate.enabled)`. The note is non-failing
  and never changes the exit code, consistent with `tooling/cli:completeness-advisory-nonfailing`.
  Directly invoking a disabled child prints the same line and exits zero.
- [ ] **Task 5.2: Cover the disclosure.** Add a test asserting both notes appear with both knobs off
  and neither appears with both on, carrying the proof marker for the claim added in Task 5.3.
  Acceptance for the phase: `./x check` still exits zero with no `note:` line in the example's
  output, which `tooling/quality-gates:example-zero-notes` requires and which Phase 4 Task 4.2
  established by turning the example's gates on.
- [ ] **Task 5.3: Apply ADR-0210 batch 5, the final batch, and flip the ADR to Implemented.** Append
  one `Applied` event for `add tooling/cli:check-disabled-child-disclosure`, and author the claim with
  `Origin: ADR-0210`, `Backing: test`, and its proof marker on the Task 5.2 test.

  Append the `Implemented` status event carrying the frozen digest IN THIS SAME TRANSACTION. The flip
  cannot be deferred: `internal/adr/application.go` refuses an `Implementing` status whose remaining
  operation set is empty, so a final Applied batch that leaves the ADR `Implementing` is rejected
  outright, and `docs/pitfalls.md` records this terminal edge. The PLAN's own `status: Implemented`
  freeze is separate, is not governed by the ADR parser, and stays in the deferred
  post-terminal-review transaction. Flip the ADR frontmatter `status:` to `Implemented` in the same
  transaction, and have the `Implemented` entry repeat the content stamp rather than recompute it.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
feat(tooling): disclose a disabled opt-in check
```

## Verification

- `git grep -F -- 'check --staged' -- . ':!docs/decisions' ':!docs/plans' ':!changelog' ':!docs/research'`
  returns no output. The same for `git grep -F 'check invariants'` with those exclusions plus
  `':!internal/migrate/renameretiredcommands.go' ':!internal/migrate/renameretiredcommands_test.go'`,
  which ADR-0210 item 11 freezes with the retired spelling intact. Outside frozen history and the new
  migration's explicit fixtures, `proseGateCmd` and `memoryGateCmd` are absent from the catalog,
  config-spec availability map, hook validation, and hook payload tests.
- `awf check repo`, `awf check staged`, and bare `awf check` each exit zero on a clean tree; bare
  `awf check` in a non-git directory exits zero having reported the staged universe unavailable.
- `awf check staged` reports drift when a `.awf/` change is staged without its rendered output.
- From `examples/sundial`, both `../../awf check repo prose` and `../../awf check repo memory` exit
  zero, and `./x check` exits zero with no `note:` line from the example.
- `./x gate` passes, including the 100% coverage floor with no new `coverage-ignore` beyond any this
  plan names, and the dead-code step with `runInvariants`, `CurrentStateInvariants`, `InvariantReport`,
  and `UngatedGroupChildren` all gone.
- ADR-0210 carries `Accepted`, `Implementing`, five `Applied` events, and `Implemented`, whose
  operations, concatenated in order, equal its declared seventeen.

## Notes

- Phase 2 retires `proseGateCmd` and `memoryGateCmd` with their only payload consumers and applies
  the three resulting claim updates in the same eleven-operation batch. Phase 4 owns the chained
  schema migration because that is where all invalidated adopter var values, including the removed
  invariants command, are transformed together.
- Phase 4 bundles the `check invariants` removal, the example invocation's rescope to the repo
  universe, and the example's gate enablement because `tooling/quality-gates:example-adopter-checked`
  describes all three in one sentence and takes exactly one update operation. Splitting them would
  leave that claim false between phases. Phase 2 therefore leaves `invariants` a direct child of
  `check` rather than rehoming it, so nothing about that sentence changes until Phase 4.
- Phase 5 is the disclosure rather than an earlier phase because a disabled child emits a `note:`
  line and `x` fails the example on any such line; the example's gates go on in Phase 4.
- ADR-0210 item 9 was narrowed during plan review: its corpus-and-prefixes half described work
  `internal/git/handle.go` already does, so only the knob-first half remains and it rides Phase 2.
- ADR-0210 item 5 defers the sweep and dead-reference halves of staged drift. The carried-forward
  check-architecture cleanup lands as a roadmap entry in Task 4.4 and is not implemented here.
- The three added claims' prose is authored in the phase that applies its operation, not in advance;
  ADR-0210 declares the operations, and the claim text lands with the mutation.
