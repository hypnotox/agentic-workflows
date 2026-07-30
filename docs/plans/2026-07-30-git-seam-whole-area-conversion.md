---
date: 2026-07-30
adrs: [181, 182]
status: Proposed
---
# Plan: Git seam whole-area conversion

## Goal

Implement ADR-0181 (single-home ownership) and ADR-0182 (git access through one semantic
seam): consolidate every git capability behind `internal/git`'s handle and runner, convert
every production and test consumer, reshape gitfixture into the only fixture constructor,
and apply both ADRs' claims in their declared transactions. Non-goals: no backend
switching, no `internal/git` package split, no repo-wide error-identity policy, and no
conversion of single-home backlog items outside the git area.

## Architecture summary

Seven phases. Phase 1 merges main so the branch sits on the integrated severity and
state-ownership transactions. Phases 2-5 are vertical slices, each landing seam capability
together with its consumers so every phase is reachable, covered, and green: runner and
control-root internals; the handle with object reads and the open/deadline path; the
commit-range walk with the audit tools; the effort/worktree composition with lifecycle and
cleanliness. Phase 6 reshapes the fixture lane. Phase 7 lands the repo-walking proofs,
applies all twelve claim operations in the two ADRs' declared transactions, updates the
obligated docs, and flips both ADRs and this plan.

## File structure

- **Created:** `internal/git/runner.go` (native runner + `CommandError`),
  `internal/git/handle.go` (`Open`, `Repo`, read entrypoints), `internal/git/walk.go`
  (`Commit`, `FileChange`, range-walk entrypoints), `internal/git/residentroot.go`
  (shared resident-root resolution), per-entrypoint contract-suite test files under
  `internal/git/`, `internal/git/seamwalker_test.go` and
  `internal/git/fixturewalker_test.go` (repo-walking proofs), native-lane files in
  `internal/testsupport/gitfixture/`.
- **Modified:** `internal/git/controlroot.go`, `internal/git/git.go`,
  `internal/snapshot/{working,index,commit,range}.go`, `internal/audit/audit.go`,
  `internal/migrate/{remove_workflow_residents,unified_effort_residents}.go`,
  `internal/upgrade/upgrade.go`, `internal/project/{project,currentstate,install}.go`,
  `internal/effort/{service,store}.go`, `internal/worktree/{manager,topology}.go`,
  `cmd/awf/{main,sync,effort,gate,memorygate,prosegate}.go`, `cmd/repoaudit/main.go`,
  `internal/testsupport/gitfixture/gitfixture.go`, the twenty-two converting test files,
  `cmd/awf/testmain_test.go`, the three agent sidecars, `.awf/parts/workflow/chain.md`,
  `.awf/docs/glossary.yaml`, `.awf/topics/metadata/tooling/{git-access,audit-and-snapshots}.yaml`,
  `.awf/topics/parts/tooling/{git-access,audit-and-snapshots}/current-state.md`,
  `.awf/topics/parts/code-design/single-home/current-state.md`,
  `internal/git/parserange_test.go`, `docs/pitfalls.md` sources, `docs/decisions/0181-*.md`,
  `docs/decisions/0182-*.md`, this plan, and every rendered output of the above.
- **Deleted:** `internal/worktree/git.go`, `internal/audit/git.go`,
  `internal/git/git_test.go`'s `TestWorktreeStatusInjectsGlobalExcludes` (function, not file).

## Phase 1: Baseline sync with integrated main

**Execution mode: inline.**

- [ ] **Task 1.1: Merge main into the branch.** In the worktree, run `git merge main`.
  Expected conflicts and their resolutions: `docs/decisions/INDEX.md` (regenerate via
  `./x render`, never hand-merge); `.awf/awf.lock` (take main's, then re-render);
  `.awf/topics/metadata/tooling/audit-and-snapshots.yaml` and its current-state part
  (take main's severity-chain edits; this plan's Phase 7 narrows paths afterward);
  `internal/project/project.go` and `internal/project/topics.go` (take main's
  state-ownership shape; this plan's later phases re-apply seam changes on top);
  ADR numbering (if main consumed 0181/0182, renumber this branch's two ADRs and their
  file names to the next free numbers, update their cross-references, and re-render;
  the same renumber applies to this plan's `adrs:` frontmatter).
- [ ] **Task 1.2: Re-render and verify.** Run `./x render`; stage everything the merge
  and render touched; `./awf check --staged` clean; `./x gate` ends `0 issues` with
  `check prose: clean` and `check memory: clean`.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
chore(tooling): merge integrated main into the git-seam branch
```

## Phase 2: Native runner, control-root conversion, ladder extraction

**Execution mode: subagent-driven.** Baseline: `git status --porcelain` empty;
`./x gate` ends `0 issues`.

- [ ] **Task 2.1: `internal/git/runner.go` with `CommandError`.** New unexported runner
  type plus exported error: `type CommandError struct { Args []string; ExitCode int;
  Stderr string; Err error }` with `Error()` (formats args, exit code, stderr) and
  `Unwrap()`. Runner behaviour: constructor takes the validated repository root; its run
  method takes `ctx context.Context` and argv, returns stdout bytes. It hard-errors
  (before spawning) when `ctx.Deadline()` is absent, with a message naming the missing
  deadline; pins the repo via `-C <root>`; builds env from
  `isolatedGitEnvironment(os.Environ())` unconditionally; captures stderr; translates a
  non-zero exit into `*CommandError`. A probe helper wraps run for
  exit-code-1-as-answer: exit 0 -> `(true, nil)`, exit 1 -> `(false, nil)`, otherwise
  `(false, *CommandError)`. Unexport `IsolatedGitEnvironment` (rename to
  `isolatedGitEnvironment`; grounding confirmed no external consumer; its end-to-end
  tests keep working via `ResolveControlRoots`).
- [ ] **Task 2.2: Convert `internal/git`'s own subprocess sites.** `runGitBytes` in
  `controlroot.go` and the inline exec in `WorktreeChangeCounts` (`git.go`) route through
  the runner. `WorktreeChangeCounts` keeps porcelain v2 semantics and gains isolation,
  stderr-carrying errors, and the deadline requirement; thread `ctx` parameters up
  through `ResolveControlRoots`, `ListWorktreeRegistrations`, and `WorktreeChangeCounts`
  signatures and their existing callers (compile-driven; callers temporarily pass a
  deadlined context built at their current boundaries with the Task 3.4 constant, which
  this task introduces early in `cmd/awf` if not yet present).
- [ ] **Task 2.3: Extract the identity ladder.** In `controlroot.go`, extract the repeated
  check-act-recheck sequences into named unexported operations (shape: one operation that
  performs lstat-validate, act, and re-validate around a single path, and one that
  resolves a stable identity through `EvalSymlinks` with re-validation). Replace each
  unrolled site with calls. Collapse the byte-identical `coverage-ignore` comments into
  the few the named operations genuinely need, each keeping the existing reason string.
  Post-check: `grep -c 'requires an OS race or fault' internal/git/controlroot.go`
  returns a small handful (the extraction's own escapes, not the current per-site
  count), and `./x gate` still reports 100% coverage.
- [ ] **Task 2.4: Runner and topology contract suites.** New test files in
  `internal/git`: runner suite proving (a) a polluted environment (`GIT_DIR`,
  `GIT_WORK_TREE`, `GIT_INDEX_FILE`, `GIT_CONFIG_GLOBAL` set via `t.Setenv` to hostile
  values) does not affect an isolated invocation; (b) a deadline-less context is refused
  without spawning; (c) a failing invocation's error `errors.As`-matches `*CommandError`
  and carries stderr; (d) the probe helper's three outcomes. Topology suite pins
  `ResolveControlRoots`/`ListWorktreeRegistrations` semantics on fixture repos with
  registered worktrees (use the existing native fixtures until Phase 6 converts them).
  These suites are serial (`t.Setenv`).
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
refactor(code-design): one isolated deadlined native git runner
```

## Phase 3: Handle, object reads, and the open path

**Execution mode: subagent-driven.** Baseline: `git status --porcelain` empty;
`./x gate` ends `0 issues`.

- [ ] **Task 3.1: `internal/git/handle.go`.** `func Open(root string) (*Repo, error)`
  absorbs the tolerant `worktreeConfig` open (current `OpenRepo` body) and validates the
  root once; `type Repo` holds the root, the opened go-git repository (unexported), and
  the runner. Methods (all `ctx context.Context` first): `IndexBlobs`, `CommitBlobs(rev)`,
  `RangeBlobs(base, head)`, `WorkingPaths`, `HeadExists`, `HeadHash`, `ChangeCounts` -
  bodies move from the existing package functions; go-git-backed methods keep their
  current semantics including the global-excludes injection inside `WorkingPaths`. A
  not-a-repository sentinel `ErrNotARepository` is returned from `Open` where go-git
  reports `ErrRepositoryNotExists`; no go-git or go-billy type or sentinel appears in any
  exported signature. `OpenContainingRepo`'s discovery loop moves behind
  `OpenContaining(start string) (*Repo, string, error)`. Unexport `GlobalExcludePatterns`
  and the old package-level read functions as their callers convert in this phase;
  delete any that end the phase with no caller (the dead-code gate enforces this).
- [ ] **Task 3.2: Snapshot threading (batch).** `internal/snapshot`'s `WorkingTree`,
  `IndexTree`, `CommitTree`, `RangePair` take `(ctx context.Context, repo *git.Repo, ...)`
  instead of `repoRoot string`. Representative: `snapshot.IndexTree(root)` in
  `cmd/awf/memorygate.go` (`runMemoryGate`) becomes `snapshot.IndexTree(ctx, repo)`
  where `repo, err := git.Open(root)` is composed at the handler boundary with the
  Task 3.4 deadline. Edge: `internal/project/currentstate.go`'s uses inside
  `workingCurrentState` and the staged-check path receive the handle through their
  callers (post-state-ownership shapes; thread, do not re-open per call). Exhaustive
  affected-site set: the callers of those four snapshot functions in `cmd/awf/gate.go`,
  `cmd/awf/main.go`, `cmd/awf/memorygate.go`, `cmd/awf/prosegate.go`,
  `internal/project/context.go`, `internal/project/currentstate.go`,
  `internal/project/topics.go`. Post-check: `grep -rn "repoRoot string" internal/snapshot`
  returns no output; `go build ./...` clean.
- [ ] **Task 3.3: Migrate, upgrade, and project reads.**
  `internal/migrate/remove_workflow_residents.go` and `unified_effort_residents.go`
  replace `git.OpenRepo` + `errors.Is(err, gogit.ErrRepositoryNotExists)` with
  `git.Open` + `errors.Is(err, git.ErrNotARepository)` and drop their go-git imports;
  their branch enumeration (`legacyBranches`) consumes a `Repo` method (`Branches(ctx)`
  returning names) added here with this concrete consumer. `internal/upgrade/upgrade.go`'s
  `HeadHash` and `internal/project/currentstate.go`'s `HeadExists` become handle calls.
  Post-check:
  `grep -rln "go-git" internal/migrate internal/upgrade internal/project --include=*.go`
  over non-test files returns no output.
- [ ] **Task 3.4: The open/deadline path and resident-root single home.** In `cmd/awf`,
  add `const gitCommandTimeout = 2 * time.Minute` (the hang-prevention ceiling from
  ADR-0182 item 4) beside the dispatch context plumbing; every handler that reaches git
  derives `ctx, cancel := context.WithTimeout(...)` at its boundary. `project.Open` and
  `Loader.Open` gain a leading `ctx context.Context` parameter threaded to their git
  calls. Convert all nine `context.Background()` feeds: `cmd/awf/effort.go` (three),
  `cmd/awf/sync.go`, `internal/migrate/remove_workflow_residents.go`,
  `internal/migrate/unified_effort_residents.go` (two), `internal/project/install.go`,
  `internal/project/project.go`. Move the duplicated resident-root resolution into
  `internal/git/residentroot.go` as `ProjectResidentRoot(ctx, invocationPath)` (body:
  `ResolveControlRoots` -> `ResidentRoot(ResidentEfforts)` -> parent-of-parent of the
  primary, with the two existing error-tolerant fallbacks); `cmd/awf/sync.go`'s
  `resolveProjectResidentRoot` and `project.Open`'s inline copy both call it. Post-check:
  `grep -rn "context.Background()" cmd internal --include=*.go | grep -v _test` returns
  no output.
- [ ] **Task 3.5: Object-read contract suite.** Pin per entrypoint on fixture repos:
  staged/commit/range blob enumeration including rename and deletion edges;
  `WorkingPaths` honouring the repository gitignore chain AND a global excludes file
  (regression case: the global-gitignore scope incident); a tracked-looking path below
  an ignored managed-worktree root staying excluded (the worktree-root incident); `Open`
  succeeding on a repo whose config carries `extensions.worktreeConfig = true` (the
  `PlainOpen` incident); `Open` on a non-repo returning `ErrNotARepository` via
  `errors.Is`.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
refactor(code-design): git handle, object reads, deadlined open
```

## Phase 4: Commit-range walk and the audit tools

**Execution mode: subagent-driven.** Baseline: `git status --porcelain` empty;
`./x gate` ends `0 issues`.

- [ ] **Task 4.1: `internal/git/walk.go`.** Move `Commit` and `FileChange` (fields as
  currently declared in `internal/audit/git.go`) into `internal/git`; add `Repo` methods
  `RangeCommits(ctx, base, head) ([]Commit, error)` (revision resolution, merge-base,
  preorder iteration, per-commit `FileChange` stats via tree diff - bodies move from
  `internal/audit/git.go`'s `Collect`, `toCommit`, `toFileChange`, `scopedPath`) and
  `FileText(ctx, rev, path) (string, bool, error)` (body from `fileText`).
- [ ] **Task 4.2: Audit converts, `internal/audit/git.go` deletes.** `audit.Run`'s
  collection path takes the walk results through its existing narrow inputs; the package
  drops its go-git import entirely, and callers of the moved types in
  `internal/project` and `cmd/awf` update to `git.Commit`/`git.FileChange` in the same
  task. Post-check: `grep -rln "go-git" internal/audit --include=*.go` returns no
  output.
- [ ] **Task 4.3: repoaudit converts.** `cmd/repoaudit/main.go` replaces `realGit` and
  `gitError` with a `git.Open` handle composed in `main` and runner-backed calls through
  its existing `gitFunc` seam signature (the seam type stays consumer-owned; production
  binds handle methods). The `coverage-ignore` on `realGit` disappears with the
  function.
- [ ] **Task 4.4: Walk contract suite.** Pin: commit enumeration order and bounds for a
  linear range and a merged range; per-file add/delete stats for create, modify, delete,
  and rename; `FileText` at both range ends; an unresolvable revision yielding an error
  that is not a `CommandError` (library-side identity).
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
refactor(code-design): commit-range walk behind the seam
```

## Phase 5: Effort/worktree composition, lifecycle, and the one cleanliness oracle

**Execution mode: subagent-driven.** Baseline: `git status --porcelain` empty;
`./x gate` ends `0 issues`.

- [ ] **Task 5.1: Seam gains the lifecycle and effort operations.** `Repo` methods with
  bodies moved from `internal/worktree/git.go` and `internal/effort/service.go`:
  `WorktreeAdd(ctx, path, branch, base)`, `WorktreeRemove(ctx, path)`,
  `WorktreeList(ctx)`, `Ancestor(ctx, base, head) (bool, error)`,
  `BranchExists(ctx, name) (bool, error)`, branch creation/deletion as currently shaped
  in the worktree runner's callers, and `ValidateRefName(ctx, name) (bool, error)` (body
  from `internal/effort/store.go`'s `check-ref-format` exec). All native calls go
  through the Phase 2 runner; probes use the probe helper.
- [ ] **Task 5.2: Delete `internal/worktree/git.go`; convert the Manager.** The `Runner`
  contract stays consumer-owned in `internal/worktree` (narrow, as currently consumed);
  production wiring in `cmd/awf` binds `Repo` methods to it. `worktree.Open` takes its
  dependencies explicitly (no nil defaults, panic-on-nil per the `project.NewLoader`
  model), stops constructing the inner `effort.Open`, and receives the composed
  `*effort.Service`. Delete the stored `Manager.ctx`; `Add`, `Integrate`, `Remove` (and
  the private helpers they call) take `ctx`. The cleanliness refusal consumes
  `Repo.ChangeCounts` (porcelain v2; the v1 duplicate dies with the file).
- [ ] **Task 5.3: Effort converts.** Delete `nativeGit`, `nativeBranchExists`, the
  `Options.Git`/`Options.Fault` fields, the `Service.git` field, and the inline exec in
  `store.go`. `effort.Open` takes explicit dependencies (clock, UUID, worktree listing,
  branch probe, ref validation, removal) with no silent defaults; delete stored
  `Service.ctx`; `New` and `Finish` (the operations that reach git) take `ctx`; `List`
  and `Show` do not. Fault injection in store tests moves to the injected dependencies.
- [ ] **Task 5.4: Composition root and test conversion (batch).** `cmd/awf` composes
  handle, seam-backed runner bindings, effort service, and worktree manager in the
  dispatch wiring; the `openWorktreeManager` package global is deleted and its test
  substitutions become constructor-argument fakes. Representative: a
  `worktree/manager_test.go` test that today writes `m.run = fake` constructs
  `worktree.Open` with a fake runner argument instead. Edge:
  `internal/effort/service_test.go`'s fault-path tests inject failing dependencies
  through the constructor rather than post-construction `.fault`/`.removeTree` field
  writes. Exhaustive set: every post-construction field write on `Manager`/`Service`
  values in `internal/worktree/manager_test.go`,
  `internal/worktree/topology_parity_test.go`, `internal/effort/service_test.go`,
  `internal/effort/store_test.go`, `internal/effort/safety_test.go`, and every
  `openWorktreeManager` substitution in `cmd/awf/effort_test.go` and
  `cmd/awf/effort_worktree_test.go`. Post-check:
  `grep -rn "\.run = \|\.roots = \|\.fault = \|\.removeTree = \|\.worktrees = \|\.branchExists = " internal/worktree internal/effort --include=*_test.go`
  returns no output, and `grep -rn "openWorktreeManager" cmd/awf` returns no output.
- [ ] **Task 5.5: Lifecycle and effort contract suites.** Pin: add/list/remove of a
  registered worktree round-trip; `Ancestor` truth table (ancestor, non-ancestor,
  unrelated histories); `BranchExists` both outcomes; `ValidateRefName` accepting a
  slug-shaped name and rejecting `..`, space, and trailing-slash forms; `ChangeCounts`
  on clean, staged-only, unstaged-only, and untracked-only trees (the oracle edges both
  consumers rely on).
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
refactor(code-design): effort and worktree composition through the seam
```

## Phase 6: Fixture lanes

**Execution mode: subagent-driven.** Baseline: `git status --porcelain` empty;
`./x gate` ends `0 issues`.

- [ ] **Task 6.1: gitfixture two-lane reshape.** Exported API becomes neutral: hashes as
  `string`, repositories as a fixture-owned opaque `Fixture` value carrying the root; no
  `*git.Repository` or `plumbing.Hash` in any exported signature. The go-git lane gains:
  unmerged index entries, explicit filemodes including gitlink, allow-empty commits,
  add-all staging, branch-ref creation. A native lane (direct `os/exec` git inside
  gitfixture, environment-isolated standalone since
  `tooling/quality-gates:testsupport-zero-internal-deps` forbids importing
  `internal/git`) provides registered managed worktree add/remove and any state the
  converting tests exercise that go-git cannot express. Existing exported helpers keep
  their names where their shapes survive; changed signatures compile-drive their
  callers.
- [ ] **Task 6.2: Convert the twenty-two files (batch with helpers).** Exhaustive set -
  go-git importers: `cmd/awf/audit_test.go`, `cmd/awf/memorygate_test.go`,
  `cmd/awf/prosegate_test.go`, `cmd/awf/run_test.go`, `internal/audit/git_test.go`
  (whatever of it survives Phase 4), `internal/git/git_test.go` (allowlisted; convert
  only where gitfixture already serves it), `internal/migrate/remove_workflow_residents_test.go`,
  `internal/migrate/unified_effort_residents_test.go`,
  `internal/project/audit_inputs_test.go`, `internal/project/staged_test.go`,
  `internal/snapshot/commit_test.go`, `internal/snapshot/index_test.go`,
  `internal/snapshot/range_test.go`, `internal/snapshot/working_test.go`; exec-based
  builders: `internal/project/context_test.go`, `internal/project/topics_test.go`,
  `internal/worktree/manager_test.go`, `internal/effort/store_test.go`,
  `cmd/awf/topic_test.go`, `cmd/awf/context_test.go`, `cmd/awf/audit_test.go`,
  `internal/migrate/remove_workflow_residents_test.go`. Representative:
  `internal/snapshot/index_test.go`'s unmerged-entry setup becomes a gitfixture
  unmerged-entry call. Edge: `internal/worktree/manager_test.go`'s registered-worktree
  setup becomes a native-lane call. Helper partition permitted: one helper per package
  directory, path-disjoint, commit-disabled, shared files parent-owned. Post-check: the
  Phase 7 fixture walker's grep form (go-git imports or `exec.Command("git"` in
  `*_test.go` outside `internal/testsupport/gitfixture` and `internal/git`) returns no
  output.
- [ ] **Task 6.3: Assertion corrections and parallelism.** Rewrite
  `cmd/awf/testmain_test.go`'s assertion so it states the seam contract (git reached
  only through `internal/git`, no ambient host git config) instead of "purely through
  go-git". Delete `TestWorktreeStatusInjectsGlobalExcludes` from
  `internal/git/git_test.go` (superseded by the Phase 3 suite). Add `t.Parallel()` to
  converted package tests where no `t.Setenv`/`t.Chdir`/shared-state constraint
  remains; leave the isolation and missing-binary suites serial.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
refactor(code-design): gitfixture as the single two-lane fixture home
```

## Phase 7: Walkers, claims, docs, and the lifecycle flip

**Execution mode: inline.**

- [ ] **Task 7.1: Repo-walking proofs and the entrypoint table.**
  `internal/git/seamwalker_test.go`: walks non-test `.go` files module-wide and fails on
  any go-git/go-billy import or `exec.Command("git"`/`exec.CommandContext(..., "git"`
  construction outside `internal/git/**` and `internal/testsupport/gitfixture/**`.
  `internal/git/fixturewalker_test.go`: walks `*_test.go` files and fails on the same
  two forms outside `internal/testsupport/gitfixture/**` and `internal/git/**`. An
  entrypoint-table test enumerates every exported `Repo` method plus the free
  entrypoints and fails when one lacks a registered contract-suite function (a map from
  entrypoint name to suite function, complete by construction).
- [ ] **Task 7.2: Apply ADR-0181's operations and anchors.** Author the two claims in
  `.awf/topics/parts/code-design/single-home/current-state.md` per ADR-0181 items 2-5
  (`single-implementation` encoding items 2 and 3, including the reasoned-divergence and
  new-consumer clauses; `no-coverage-fork` encoding item 4; both `Origin: ADR-0181`,
  `Backing: unbacked`, `Verify:` lines derived from the items). Add the focus item
  naming `code-design/single-home` to the three agent sidecars, comparing against and
  backfilling the current catalog defaults each list replaces; extend
  `.awf/parts/workflow/chain.md` to name the topic beside its siblings; add a
  `single home` entry to `.awf/docs/glossary.yaml`.
- [ ] **Task 7.3: Apply ADR-0182's ten operations in one batch.** In
  `.awf/topics/parts/tooling/git-access/current-state.md`: author the six new claims per
  ADR-0182 item 11 (four `Backing: test` with proof markers placed on the seam walker,
  the fixture walker, the entrypoint-table test, and the runner suite; two
  `Backing: unbacked` with `Verify:` lines), and re-add the two range-parser claims
  (`Origin: ADR-0182`, prose preserving ADR-0127 by reference, existing test backing).
  Remove the two range-parser claims from
  `.awf/topics/parts/tooling/audit-and-snapshots/current-state.md` and narrow that
  topic's metadata paths to `internal/audit/**` and `internal/snapshot/**`. Rewrite the
  two proof markers in `internal/git/parserange_test.go` to the new qualified ids.
- [ ] **Task 7.4: Obligated docs.** Rewrite the three pitfalls entries (repo-open route,
  global-excludes injection, gitfile resolution) to name the seam entrypoints; update
  `docs/architecture.md`'s `internal/worktree` bullet and go-git dependency note; add
  the contract-suite category and the serial-by-construction exception to
  `docs/testing.md`. All via their `.awf` parts/sources followed by `./x render`.
- [ ] **Task 7.5: Flip and freeze.** Apply the direct Proposed-to-Implemented transition
  to both ADRs per the `awf-adr-lifecycle` skill (append the status events with their
  frozen content digests, and the batch state sequence covering the ten 0182
  operations); regenerate `INDEX.md`; set this plan's `status: Implemented` and record
  implementation-surfaced findings in its Notes. Terminal state: `./awf check` clean,
  `./awf check --staged` clean, `./x gate` ends `0 issues`.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
docs(code-design): apply single-home and git-access authority
```

## Verification

- `./awf check` and `./x gate` clean at every phase close; at Phase 7 close both
  walkers and the entrypoint table are green, `./awf topic tooling/git-access` lists
  the eight claims with their backing, and `./awf topic tooling/audit-and-snapshots` no
  longer lists the range-parser claims.
- `grep -rn "context.Background()" cmd internal --include=*.go | grep -v _test` returns
  no output.
- `GOOS=windows go build ./cmd/awf` and `GOOS=darwin GOARCH=arm64 go build ./cmd/awf`
  stay clean.
- Advisory (optional, post-implementation): the deterministic gremlins recipe over
  `internal/git` per `docs/testing.md`, trusted only on a zero-timeout report.

## Notes

- Control-root resolution (`ResolveControlRoots`) and `ParseRange` remain free
  functions: both operate before or without an opened repository (root discovery and
  pure parsing); every root-bound entrypoint is a `Repo` method. This is the plan's
  reading of ADR-0182 item 2's "entrypoints as methods" and is flagged for the plan
  review to confirm.
- Integration after this plan completes: `awf effort integrate
  single-home-and-git-seam-decisions` from a clean main checkout, then renewed review
  per `awf-reviewing-impl`, then worktree removal and retrospective per the effort
  lifecycle.
- The severity chain and this branch both touched `tooling/audit-and-snapshots`;
  Phase 1's merge is where that reconciles.
