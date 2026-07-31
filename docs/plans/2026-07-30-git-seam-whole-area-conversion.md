---
date: 2026-07-30
adrs: [181, 191]
status: Proposed
---
# Plan: Git seam whole-area conversion

## Goal

Implement ADR-0181 (single-home ownership) and ADR-0191 (git access through one semantic
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
applies all thirteen claim operations in the two ADRs' declared transactions, updates the
obligated docs, and flips both ADRs and this plan.

## File structure

- **Created:** `internal/git/runner.go` (native runner + `CommandError`),
  `internal/git/handle.go` (`Open`, `Repo`, read entrypoints), `internal/git/walk.go`
  (`Commit`, `FileChange`, range-walk entrypoints), `internal/git/residentroot.go`
  (shared resident-root resolution), per-entrypoint contract-suite test files under
  `internal/git/`, `internal/git/seamwalker_test.go`,
  `internal/git/fixturewalker_test.go` (repo-walking proofs),
  `internal/git/entrypoints_test.go` (the entrypoint table), native-lane files in
  `internal/testsupport/gitfixture/`.
- **Modified:** `internal/git/controlroot.go`, `internal/git/git.go`,
  `internal/snapshot/{working,index,commit,range}.go`, `internal/audit/audit.go`,
  `internal/migrate/{remove_workflow_residents,unified_effort_residents}.go`,
  `internal/upgrade/upgrade.go`, `internal/project/{project,currentstate,install}.go`,
  `internal/effort/{service,store}.go`, `internal/worktree/{manager,topology}.go`,
  `cmd/awf/{main,sync,effort,gate,memorygate,prosegate}.go`, `cmd/repoaudit/main.go`,
  `cmd/repoaudit/main_test.go`,
  `internal/testsupport/gitfixture/gitfixture.go`, the converting test files of Task 6.2,
  `cmd/awf/testmain_test.go`, the three agent sidecars, `.awf/parts/workflow/chain.md`,
  `.awf/docs/glossary.yaml`, `.awf/docs/pitfalls.yaml`,
  `.awf/docs/parts/architecture/{components,dependencies}.md`,
  `.awf/docs/parts/testing/{tiers,layout}.md`,
  `.awf/topics/metadata/tooling/audit-and-snapshots.yaml` (git-access.yaml already
  carries its two selectors from the proposal commit; no change owed),
  `.awf/topics/parts/tooling/{git-access,audit-and-snapshots}/current-state.md`,
  `.awf/topics/parts/code-design/single-home/current-state.md`,
  `internal/git/parserange_test.go`, `docs/pitfalls.md` sources, `changelog/CHANGELOG.md`,
  `docs/decisions/0181-*.md`,
  `docs/decisions/0191-*.md`, this plan, and every rendered output of the above.
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
  state-ownership shape; this plan's later phases re-apply seam changes on top).
- [ ] **Task 1.1b: Renumber the git-seam ADR (unconditional).** Main has consumed the
  0182+ identities (its own 0182-0185 exist), and ADR-0181 is already on main and keeps
  its number. Rename this branch's git-seam ADR file and heading to the next free
  identity awf reports after the merge (do not hardcode a number), then update every
  reference to the old number: the `Origin:` targets in Task 7.3, this plan's `adrs:`
  frontmatter, and every `ADR-0191` literal in this plan's Goal, Architecture summary,
  File structure, and Tasks. Confirm ADR-0181's item 7 forward reference still carries
  no literal number (it names "its own ADR next in this effort", so no edit is owed).
  Re-render and confirm `./awf check` is clean.
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
  method takes `ctx context.Context` and argv, returns stdout bytes. It pins the repo
  via `-C <root>`; builds env from `IsolatedGitEnvironment(os.Environ())`
  unconditionally (the symbol stays exported until Phase 5 deletes its one remaining
  consumer in `internal/worktree/git.go`); captures stderr; translates a
  non-zero exit into `*CommandError`. The deadline hard-error is NOT added in this
  phase: it activates in Task 3.4 together with the nine feed conversions it gates
  (two of those feeds swallow errors into a silent resident-root fallback, so
  enforcement and feed conversion must land in one transaction). The
  exit-code-1-as-answer probe helper does NOT land here: the dead-code gate rejects it
  with no consumer, and its first consumers (branch existence, ancestor) are Phase 5
  scope - it lands in Task 5.1 with its three-outcome test. The existing injection
  seam `nativeGitRunner`
  (`controlroot.go`) is superseded: `resolveControlRoots`, `runGitPathWith`, and
  `runGitTextWith` rebind to the new runner type, and no second runner abstraction
  survives the phase.
- [ ] **Task 2.2: Convert `internal/git`'s own subprocess sites.** `runGitBytes` in
  `controlroot.go` and the inline exec in `WorktreeChangeCounts` (`git.go`) route through
  the runner. `WorktreeChangeCounts` keeps porcelain v2 semantics and gains isolation,
  stderr-carrying errors, the deadline requirement, and a `ctx context.Context`
  parameter (`ResolveControlRoots` and `ListWorktreeRegistrations` already take ctx;
  only `WorktreeChangeCounts` needs the new parameter, threaded to its existing
  callers compile-driven). Add `const gitCommandTimeout = 2 * time.Minute` in `cmd/awf`
  in this task (the hang-prevention ceiling from the git-seam ADR item 4); callers pass
  a deadlined context built at their current boundaries with it.
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
  values) does not affect an isolated invocation; (b) a failing invocation's error
  `errors.As`-matches `*CommandError` and carries stderr. (The deadline-refusal test
  moves to Task 3.4 with the enforcement it proves; the probe helper's three-outcome
  test moved to Task 5.1 with the helper.) Topology suite pins
  `ResolveControlRoots`/`ListWorktreeRegistrations` semantics on fixture repos with
  registered worktrees (use the existing native fixtures until Phase 6 converts them).
  These suites are serial (`t.Setenv`).
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
refactor(code-design): one isolated native git runner
```

## Phase 3: Handle, object reads, and the open path

**Execution mode: subagent-driven.** Baseline: `git status --porcelain` empty;
`./x gate` ends `0 issues`.

- [ ] **Task 3.0: Real-git ignore semantics for isolated native status (user decision,
  Phase 2 review).** The isolated environment strips the user/system git config, which
  narrowed the cleanliness oracle's ignore universe (a Phase 2 regression the review
  caught empirically). Restore real-git semantics: before building the isolated
  environment, resolve the effective `core.excludesFile` with an ambient-environment
  `git config --get core.excludesFile` read (user and system scopes), and when set,
  pass `-c core.excludesfile=<resolved>` on native status invocations, keeping every
  other isolation property (credential, prompt, repo-selection). Pin the behaviour in
  the runner/status tests now and again in the Task 3.5 suite with the
  global-gitignore pitfall as the named regression case; the pitfalls prepend sentence
  ("Git itself owns repository, global, and system ignore semantics") thereby stays
  true and needs no rewrite.
- [ ] **Task 3.1: `internal/git/handle.go`.** `func Open(root string) (*Repo, error)`
  absorbs the tolerant `worktreeConfig` open (current `OpenRepo` body) and validates the
  root once; `type Repo` holds the root, the opened go-git repository (unexported), and
  the runner. Methods (all `ctx context.Context` first): `IndexBlobs`, `CommitBlobs(rev)`,
  `RangeBlobs(base, head)`, `WorkingPaths`, `HeadExists`, `HeadHash`, `ChangeCounts`,
  and `ChangedPaths(staged bool, rangeSpec string)` (the existing exported function
  becomes a method per the ADR's amended item 3; its consumer in `cmd/awf/context.go`
  converts in Task 3.4) - bodies move from the existing package functions; go-git-backed methods keep their
  current semantics including the global-excludes injection inside `WorkingPaths`. A
  not-a-repository sentinel `ErrNotARepository` is returned from `Open` where go-git
  reports `ErrRepositoryNotExists`; no go-git or go-billy type or sentinel appears in any
  exported signature. `OpenContainingRepo`'s discovery loop moves behind
  `OpenContaining(start string) (*Repo, string, error)`. Unexport `GlobalExcludePatterns`
  and the old package-level read functions as their callers convert in this phase;
  delete any that end the phase with no caller (the dead-code gate enforces this). In
  the same task, delete `TestWorktreeStatusInjectsGlobalExcludes` from
  `internal/git/git_test.go`: its case-sensitive substring check goes vacuous with the
  unexport and would red the gate; the Task 3.5 working-tree-paths suite supersedes it
  (per the git-seam ADR item 9).
- [ ] **Task 3.2: Snapshot threading (batch).** `internal/snapshot`'s `WorkingTree`,
  `IndexTree`, `CommitTree`, `RangePair` take `(ctx context.Context, repo *git.Repo, ...)`
  instead of `repoRoot string`. Representative: `snapshot.IndexTree(root)` in
  `cmd/awf/memorygate.go` (`runMemoryGate`) becomes `snapshot.IndexTree(ctx, repo)`
  where `repo, err := git.Open(root)` is composed at the handler boundary with the
  Task 3.4 deadline. Per the ADR's amended item 6, the handle is a construction-time
  dependency: `Loader` receives the composed `*git.Repo` as a required constructor
  dependency (panic-on-nil, `NewLoader` model) and the `Project` it opens carries it as
  a field written once at construction; methods read the field and take only `ctx`.
  Edge: `internal/project/currentstate.go`'s uses inside `workingCurrentState` and the
  staged-check path read the `Project` field (post-state-ownership shapes; no re-open
  per call, no per-method repo parameter). Exhaustive
  affected-site set: the callers of those four snapshot functions in `cmd/awf/gate.go`,
  `cmd/awf/main.go`, `cmd/awf/memorygate.go`, `cmd/awf/prosegate.go`,
  `internal/project/context.go`, `internal/project/currentstate.go`,
  `internal/project/topics.go`. Post-check: `grep -rn "repoRoot" internal/snapshot
  --include=*.go` returns no output (signatures and doc comments alike); `go build ./...`
  clean.
- [ ] **Task 3.3: Migrate, upgrade, and project reads.**
  `internal/migrate/remove_workflow_residents.go` and `unified_effort_residents.go`
  replace `git.OpenRepo` + `errors.Is(err, gogit.ErrRepositoryNotExists)` with
  `git.Open` + `errors.Is(err, git.ErrNotARepository)` and drop their go-git imports;
  their branch enumeration (`legacyBranches`) consumes a `Repo` method (`Branches(ctx)`
  returning names) added here with this concrete consumer. `internal/upgrade/upgrade.go`'s
  `HeadHash` and `internal/project/currentstate.go`'s `HeadExists` become handle calls.
  Post-check:
  `grep -rln "go-git" internal/migrate internal/upgrade internal/project --include=*.go | grep -v _test`
  returns no output.
- [ ] **Task 3.4: The open/deadline path and resident-root single home.** Activate the
  runner's deadline hard-error here (before spawning, when `ctx.Deadline()` is absent,
  with a message naming the missing deadline), together with its refusal test in the
  runner suite - enforcement and the feed conversions below are one transaction. The
  same transaction converts every TEST feed that reaches the runner: add a shared
  deadlined-context test helper and adopt it at the `t.Context()` and
  `context.Background()` test call sites that reach git across `internal/git`,
  `internal/audit`, `internal/project`, `internal/migrate`, and `cmd/awf` (enumerate
  by grep at execution time; the enforcement turns every missed one red, so the gate
  is the completeness check). Every
  `cmd/awf` handler that reaches git derives
  `ctx, cancel := context.WithTimeout(..., gitCommandTimeout)`
  at its boundary (the constant exists from Task 2.2). `project.Open` and `Loader.Open`
  gain a leading `ctx context.Context` parameter, and the loader's injected
  `project.ResolveResidentRoot` contract type changes from `func(string) string` to
  `func(context.Context, string) string` so the context reaches the resolution. Convert
  all nine `context.Background()` feeds: `cmd/awf/effort.go` (three), `cmd/awf/sync.go`,
  `internal/migrate/remove_workflow_residents.go`,
  `internal/migrate/unified_effort_residents.go` (two), `internal/project/install.go`,
  `internal/project/project.go`. Move the duplicated resident-root resolution into
  `internal/git/residentroot.go` as
  `func ProjectResidentRoot(ctx context.Context, invocationPath string) string` (body:
  `ResolveControlRoots` -> `ResidentRoot(ResidentEfforts)` -> parent-of-parent of the
  primary; on any error at either step it returns `invocationPath`, matching both
  current copies' fallback-to-root behaviour exactly); `cmd/awf/sync.go`'s
  `resolveProjectResidentRoot` and `project.Open`'s closure both delegate to it. The
  `ChangedPaths` consumer in `cmd/awf/context.go` converts to the handle method here.
  Post-check: `grep -rn "context.Background()" cmd internal --include=*.go | grep -v _test`
  returns no output.
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
  `internal/audit/git.go`'s `Collect`, `toCommit`, `toFileChange`, `scopedPath`),
  `FileText(ctx, rev, path) (string, bool, error)` (body from `fileText`), and - per
  the ADR's amended item 3, first consumers repoaudit in Task 4.3 -
  `MergeBase(ctx, a, b) (string, error)`,
  `RangeChangedPaths(ctx, base, head) ([]string, error)` (diff --name-only semantics),
  and `RangeDiffText(ctx, base, head) (string, error)` (unified diff text with the
  prefix options repoaudit's parser expects, runner-backed).
- [ ] **Task 4.2: Audit converts, `internal/audit/git.go` deletes.** `audit.Run`'s
  collection path takes the walk results through its existing narrow inputs; the package
  drops its go-git import entirely, and callers of the moved types in
  `internal/project` and `cmd/awf` update to `git.Commit`/`git.FileChange` in the same
  task. The two live symbols the file also holds have named destinations:
  `ruleUncommittedChanges` and its `touches-state:` comment relocate intact into
  `internal/audit/audit.go`, consuming `Repo.ChangeCounts`; `splitMessage` travels with
  `toCommit` into `internal/git/walk.go`. The three `invariant:` proof markers in
  `internal/audit/git_test.go` (`audit-empty-range-clean` and the two
  `audit-uncommitted-changes` markers) and the tests carrying them are preserved
  verbatim, relocated into `internal/audit/audit_test.go`. The remaining
  `internal/audit/git_test.go` tests split by subject: tests exercising the moved walk
  machinery (`TestCollectNormalRange`, `TestCollectMergeCommitCarriesNoChanges`,
  `TestCollectUnrelatedHistories`, `TestSplitMessage`, `TestToCommitRootAndFileText`,
  `TestCollectWorktreeConfigExtension`, and kin) are superseded by the Task 4.4 walk
  contract suite, which re-pins their behaviours in `internal/git` - carry over any
  edge one of them covers that the suite draft lacks; tests exercising audit-level
  policy over walk results (adopter filtering and rerooting, for example
  `TestCollectNestedAdopterFiltersAndReroots`) adapt to drive audit's collection path
  through the seam and relocate into `internal/audit/audit_test.go`. Post-check:
  `grep -rln "go-git" internal/audit --include=*.go | grep -v _test` returns no output
  (test files convert in Phase 6; the enduring oracle is the Phase 7 import-based seam
  walker, since `internal/audit/testmain_test.go` keeps the literal `go-git` in prose).
- [ ] **Task 4.3: repoaudit converts.** `cmd/repoaudit/main.go` replaces `realGit` and
  `gitError` with a `git.Open` handle composed in `main`; its `gitFunc` raw-argv
  contract is replaced by a narrow consumer-owned contract over the semantic
  entrypoints its five call sites need (merge-base, range changed-paths, range diff
  text, file text) per the resolution recorded in the git-seam ADR's amended item 3.
  `cmd/repoaudit/main_test.go` converts with it (its `gitError` test dies with the
  function; fake implementations satisfy the new contract). The `coverage-ignore` on
  `realGit` disappears with the function.
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

- [ ] **Task 5.1: Seam gains the lifecycle and effort operations.** The
  exit-code-1-as-answer probe helper lands here with its first consumers (exit 0 ->
  `(true, nil)`, exit 1 -> `(false, nil)`, otherwise `(false, *CommandError)`) plus its
  three-outcome test in the runner suite. `Repo` methods with
  bodies moved from `internal/worktree/git.go` and `internal/effort/service.go`:
  `WorktreeAdd(ctx, path, branch, base)`, `WorktreeRemove(ctx, path)`,
  `WorktreeList(ctx)`, `Ancestor(ctx, base, head) (bool, error)`,
  `BranchExists(ctx, name) (bool, error)`, branch creation/deletion as currently shaped
  in the worktree runner's callers, and `ValidateRefName(ctx, name) (bool, error)` (body
  from `internal/effort/store.go`'s `check-ref-format` exec). All native calls go
  through the Phase 2 runner; probes use the probe helper.
- [ ] **Task 5.2: Delete `internal/worktree/git.go`; convert the Manager.** With the
  file's deletion, unexport `IsolatedGitEnvironment` (rename to
  `isolatedGitEnvironment`) - this was its one remaining consumer, and the end-to-end
  isolation tests keep working via `ResolveControlRoots`. The `Runner`
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
- [ ] **Task 6.2: Convert the fixture lane (batch with helpers).** Exhaustive set -
  go-git importers: `cmd/awf/audit_test.go`, `cmd/awf/memorygate_test.go`,
  `cmd/awf/prosegate_test.go`, `cmd/awf/run_test.go`,
  `internal/audit/audit_test.go` (the Phase 4 relocation destination),
  `internal/migrate/remove_workflow_residents_test.go`,
  `internal/migrate/unified_effort_residents_test.go`,
  `internal/project/audit_inputs_test.go`, `internal/project/staged_test.go`,
  `internal/snapshot/commit_test.go`, `internal/snapshot/index_test.go`,
  `internal/snapshot/range_test.go`, `internal/snapshot/working_test.go`; git-subprocess
  builders: `internal/project/context_test.go`, `internal/project/topics_test.go`,
  `internal/worktree/manager_test.go`, `internal/effort/store_test.go`,
  `cmd/awf/topic_test.go`, `cmd/awf/context_test.go`, `cmd/awf/effort_test.go`,
  `cmd/awf/run_test.go` (in both sublists: go-git import and a worktree-add exec),
  `internal/migrate/remove_workflow_residents_test.go` (both sublists).
  `internal/git/git_test.go` is allowlisted; nothing is owed there and it is
  deliberately outside this set. Representative: `internal/snapshot/index_test.go`'s
  unmerged-entry setup becomes a gitfixture unmerged-entry call. Edge:
  `internal/worktree/manager_test.go`'s registered-worktree setup becomes a native-lane
  call. Helper partition permitted: one helper per package directory, path-disjoint,
  commit-disabled, shared files parent-owned. Post-check (self-contained, runnable at
  this phase's close): both of
  `grep -rln '"github.com/go-git/go-git' --include=*_test.go . | grep -v -e '^./internal/testsupport/gitfixture/' -e '^./internal/git/' -e '^./internal/testsupport/deps_test.go'`
  (the deps_test.go exclusion is a string-literal fixture for the dependency checker,
  not a real import) and
  `grep -rlnE 'exec\.Command(Context)?\(.*"git"' --include=*_test.go . | grep -v -e '^./internal/testsupport/gitfixture/' -e '^./internal/git/'`
  return no output. The second pattern uses `.*`, not `[^)]*`: a bracket expression
  cannot cross the `)` in `exec.CommandContext(t.Context(), "git", ...)`, so the
  narrower form silently matched nothing and reported success over two unconverted
  files. Run both under real GNU grep; a `grep` aliased to ugrep strips the leading
  `./` and quietly defeats the `^./` exclusions, making the check look permanently
  failing instead.
- [ ] **Task 6.3: Doc-comment correction and parallelism.** Rewrite
  `cmd/awf/testmain_test.go`'s `TestMain` doc comment so it states the seam contract
  (git reached only through `internal/git`, no ambient host git config) instead of
  "purely through go-git". Add `t.Parallel()` to converted package tests where no
  `t.Setenv`/`t.Chdir`/shared-state constraint remains; leave the isolation and
  missing-binary suites serial.
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
  two forms outside `internal/testsupport/gitfixture/**` and `internal/git/**`. Detect
  both forms structurally, from the parsed import list and the `os/exec` call
  expressions, not by matching source text: a textual scan cannot tell an import from a
  string constant, and it is what let `exec.CommandContext(t.Context(), "git", ...)`
  escape the Task 6.2 post-check. A structural walker consequently owes
  `internal/testsupport/deps_test.go` NO carve-out even though the grep post-check
  excludes it, because that file carries the go-git path only as dependency-checker
  fixture data and never imports it. Prove that rather than assume it: each allowlist
  entry gets a test asserting it still shields a real finding, so an entry that stops
  being load-bearing is removed instead of silently widening the hole.
  `internal/git/entrypoints_test.go`: enumerates every exported `Repo` method plus the
  free entrypoints (the Notes' exhaustive list) and fails when one lacks a registered
  contract-suite function (a map from entrypoint name to suite function, complete by
  construction). In `internal/testsupport/gitfixture`, replace the single-clause
  `TestNativeLaneIsolation` with a table over the native lane's whole isolation policy:
  every inherited `GIT_*` variable plus the credential and prompt helpers is stripped from
  a deliberately hostile environment, and each pinned value is present with the expected
  setting. Add the SAME table over `internal/git`'s own `isolatedGitEnvironment`: the two
  tables together are the proof carrier for `fixture-isolation-parity` (ADR-0191 item 11),
  because the seam half is otherwise unbacked - every pin can be deleted from it with the
  whole repository green, since Git's defaults are benign under a temporary HOME. Assert
  each side's whole resulting environment by equality, not by a `GIT_` prefix sweep, so an
  ADDED pin fails as loudly as a dropped one (two of the six pins carry no `GIT_` prefix).
  Both must fail if any single pin or the strip is removed; verify that by mutation, one
  pin at a time, before placing the markers. Do NOT bound the lane's invocations with a
  deadline: the asymmetry with the seam's runner is deliberate and is instead recorded at
  `runGit` and in the pitfalls doc.
- [ ] **Task 7.2: Apply ADR-0181's operations and anchors.** Author the two claims in
  `.awf/topics/parts/code-design/single-home/current-state.md` per ADR-0181 items 2-5
  (`single-implementation` encoding items 2 and 3, including the reasoned-divergence and
  new-consumer clauses; `no-coverage-fork` encoding item 4; both `Origin: ADR-0181`,
  `Backing: unbacked`, `Verify:` lines derived from the items). Add the focus item
  naming `code-design/single-home` to the three agent sidecars, comparing against and
  backfilling the current catalog defaults each list replaces; extend
  `.awf/parts/workflow/chain.md` to name the topic beside its siblings; add a
  `single home` entry to `.awf/docs/glossary.yaml`.
- [ ] **Task 7.3: Apply ADR-0191's eleven operations in one batch.** In
  `.awf/topics/parts/tooling/git-access/current-state.md`: author the seven new claims per
  ADR-0191 item 12 (five `Backing: test` with proof markers placed on the seam walker,
  the fixture walker, the entrypoint-table test, the runner suite, and BOTH isolation
  tables - the seam's and the fixture lane's - which together prove
  `fixture-isolation-parity`; a marker on only one of them would leave the half the
  terminal review was raised about unmarked; two `Backing: unbacked` with `Verify:` lines), and re-add the two range-parser claims
  (`Origin: ADR-0191`, prose preserving ADR-0127 by reference, existing test backing).
  Remove the two range-parser claims from
  `.awf/topics/parts/tooling/audit-and-snapshots/current-state.md` and narrow that
  topic's metadata paths to `internal/audit/**` and `internal/snapshot/**`. Rewrite the
  two proof markers in `internal/git/parserange_test.go` to the new qualified ids.
- [ ] **Task 7.4: Obligated docs.** Exact sources, each followed by `./x render`:
  `.awf/docs/pitfalls.yaml` (the three entries: repo-open route, global-excludes
  injection, gitfile resolution - rewritten to name the seam entrypoints);
  `.awf/docs/parts/architecture/components.md` (the `internal/worktree` bullet);
  `.awf/docs/parts/architecture/dependencies.md` (the go-git note);
  `.awf/docs/parts/testing/tiers.md` and `.awf/docs/parts/testing/layout.md` (the
  contract-suite category and the serial-by-construction exception). Also add the
  `changelog/CHANGELOG.md` `[Unreleased]` entry covering the adopter-visible changes:
  the isolated cleanliness-oracle semantics, the git deadline ceiling and its refusal
  failure mode, and the `CommandError` error-shape change.
- [ ] **Task 7.5: Flip and freeze.** Apply the direct Proposed-to-Implemented transition
  to both ADRs per the `awf-adr-lifecycle` skill: each ADR appends one batch (ADR-0181's
  two adds; the git-seam ADR's eleven operations), the two batches taking the next two
  consecutive global state-sequence values awf reports at execution time (never a
  literal number - the counter is repo-global and concurrent efforts advance it), with
  frozen content digests on both status events. ADR-0181's claims from Task 7.2 land in
  the same commit as its flip. Regenerate `INDEX.md`; set this plan's
  `status: Implemented` and record implementation-surfaced findings in its Notes.
  Terminal state: `./awf check` clean, `./awf check --staged` clean, `./x gate` ends
  `0 issues`.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
refactor(code-design): apply single-home and git-access authority
```

## Verification

- `./awf check` and `./x gate` clean at every phase close; at Phase 7 close both
  walkers and the entrypoint table are green, `./awf topic tooling/git-access` lists
  the nine claims with their backing, and `./awf topic tooling/audit-and-snapshots` no
  longer lists the range-parser claims.
- `grep -rn "context.Background()" cmd internal --include=*.go | grep -v _test` returns
  no output.
- `GOOS=windows go build ./cmd/awf` and `GOOS=darwin GOARCH=arm64 go build ./cmd/awf`
  stay clean.
- Advisory (optional, post-implementation): the deterministic gremlins recipe over
  `internal/git` per `docs/testing.md`, trusted only on a zero-timeout report.

## Notes

- Free-entrypoint principle: an operation stays a free function only when it precedes
  or does without an opened repository (root discovery, pure parsing); everything
  root-bound on an open repo is a `Repo` method. The exhaustive free-entrypoint list
  for the entrypoint table: `ResolveControlRoots`, `ListWorktreeRegistrations`,
  `ProjectResidentRoot`, `ParseRange`. (`ListWorktreeRegistrations` qualifies because
  it inspects registration topology across roots before any single repo is the
  subject.) `ChangedPaths` is a `Repo` method per the ADR's amended item 3.
- Integration after this plan completes: `awf effort integrate
  single-home-and-git-seam-decisions` from a clean main checkout, then renewed review
  per `awf-reviewing-impl`, then worktree removal and retrospective per the effort
  lifecycle.
- The severity chain and this branch both touched `tooling/audit-and-snapshots`;
  Phase 1's merge is where that reconciles.
- Implementation-surfaced (Phase 2): the probe helper deferred to Task 5.1 (dead-code
  gate, no consumer before Phase 5); `resolveControlRoots` folded into
  `ResolveControlRoots` (the unexported wrapper lost its injection purpose once the
  runner is constructed from the validated root inside the function), with its fake-
  runner test converted to the PATH-script pattern its neighbours use (now skips on
  Windows); three pre-existing coverage escapes found claiming unreachability on
  covered blocks were removed; ladder escapes 39 to 21, all 21 empirically confirmed
  uncovered.
- Phase 2 review settlement: mid-run context expiry now preserves its context cause
  through `errors.Join` inside `CommandError`; two more "requires an OS race" escapes
  proved deterministically reachable and covered (symlinked registered worktree;
  bare record from a linked checkout); ctx-first reorder on the two runner helpers;
  comment corrections. User decision: the isolated oracle keeps real-git ignore
  semantics (Task 3.0).
- Phases 3 and 4 recorded no Notes entries at the time; their deviations and review
  settlements live in the bodies of 138b2406, 2803c07b, 41576883, 9d08d8fb, 55c11bcd,
  4f926208, and 21dc619d.
- Re-baseline before Phase 5: main was merged into the branch (a0937adf) because main's
  ADR-0186 and ADR-0189 work had changed roughly 620 lines across exactly the files
  Phase 5 rewrites. The seam ADR renumbered 0186 to 0191 inside that merge commit, since
  main consumed 0186 through 0190 and the corpus refuses two files declaring one number.
  Task 5.4's "exhaustive set" file lists predate that merge; the two grep post-checks
  were treated as the completeness oracle instead.
- Implementation-surfaced (Phase 5), six deviations: `effort.Open` and `worktree.Open`
  take already-resolved `ControlRoots` rather than `(ctx, invokingRoot)`, which is forced
  by deleting the stored ctx (a constructor taking no context cannot resolve roots) and
  is what let every post-construction `roots` write become a constructor argument;
  worktree's `Runner` is a 14-method contract plus a separate `OpenCheckout` opener,
  because `Remove` reasons about the invoking and managed checkouts together while a
  handle is pinned to one root; the lifecycle methods landed in a new
  `internal/git/lifecycle.go` rather than in `handle.go`; seven `Repo` methods not named
  in Task 5.1 converted (`ResolveCommit`, `CurrentBranch`, `GitPath`, `MergeFastForward`,
  `MergeNoCommit`, `WorktreePrune`, `BranchDelete`), because every native invocation left
  in the manager had to become a seam method for `internal/worktree/git.go` to die;
  `Dependencies` carries one optional member, the durability fault hook (see the ADR-0191
  item 7 amendment); and `parseWorktreePorcelain` now rejects an empty record, which the
  seam parser previously accepted and the deleted worktree parser rejected, removing a
  real divergence and making a `len(records) == 0` check unreachable.
- Behaviour change (Phase 5), deliberate and user-approved: the checkout cleanliness
  refusal no longer carries an untracked-resident allowance. The deleted implementation
  matched `?? .awf/efforts/...` and `?? .awf/worktrees/...` with a regexp and read such a
  tree clean; `ChangeCounts` returns counts and cannot express that, so owned resident
  state stays invisible only because awf renders the `.gitignore` covering it. This is
  NOT behaviour preservation: with those `.gitignore` files on disk but absent from the
  index, integration and removal now refuse as dirty where they previously proceeded.
  Reachable windows are after `awf init` or `awf render` before those files are first
  committed, an adopter upgrading from an awf predating resident-gitignore rendering, and
  any `git rm --cached`. Kept because the direction is fail-safe and the old regexp was
  over-permissive, tolerating any untracked file under those two roots including a
  developer's own uncommitted work. Task 7.4's changelog entry names this explicitly.
- Phase 5 review settlement: `ValidateRefName` had silently dropped the `--branch` flag
  the body it replaced used, which is not a spelling difference (a one-level name like
  `main` is a valid branch but not a valid bare refname). It now validates the qualified
  `refs/heads/` form, which answers the same branch question AND keeps the exit-0/1
  contract the probe helper requires, where `--branch` reports an invalid name with exit
  128 and would surface a malformed name as a fault rather than a negative. Also: the
  invoking-checkout cleanliness refusal was pinned at the manager layer in both
  directions (it survived deletion with the suite green before), the `ChangeCounts`
  contract suite gained the ignored-resident case the whole design rests on, three
  comments describing properties the code lacks were corrected, and the stale tracked
  `docs/topics/tooling/git-access.md.awf-bak` left by the Phase 1 merge was deleted.
- Phase 6 review settlement: the phase closed green over two files Task 6.2 named but
  never converted, `cmd/awf/effort_test.go` and `cmd/awf/effort_worktree_test.go`, with
  15 raw-subprocess call sites between them. The mechanism was the post-check itself:
  its `[^)]*` bracket expression cannot cross the `)` in
  `exec.CommandContext(t.Context(), "git", ...)`, which is the exact spelling both files
  used, so the check matched nothing and reported success. The pattern is corrected to
  `.*` in Task 6.2 and the correction is carried into Task 7.1's walker spec, which
  quoted the same shape. Both files now build state through the native lane, and their
  `commandGit` and `effortWorktreeGit` helpers are deleted; the second helper appended
  to `os.Environ()` without stripping inherited `GIT_*`, so it was also the last fixture
  in the tree with weaker isolation than the lane it should have used. Task 7.1 also
  gains the `internal/testsupport/deps_test.go` allowlist entry the Task 6.2 post-check
  already carried. Smaller items: `NativeConfig` unexported (no consumer outside the
  package, and the dead-code gate cannot see a test-only package);
  `NativeRevisionExists` now takes `t` and reads only exit 1 as "absent", failing on any
  other nonzero exit, because a fault previously satisfied a must-be-absent assertion
  for the wrong reason and this settlement made it load-bearing in the positive
  direction too; and a duplicated `initWorktreeRepo` doc comment collapsed to one.
- User decisions (2026-07-31, after the Phase 6 review): the fixture lane's duplicate of
  the seam's environment isolation gains a backed claim rather than resting on a reader
  noticing it, so ADR-0191 is amended with a `fixture-isolation-parity` operation (its
  eleventh) and item 11, and Task 7.1 gains the proving table. The lanes' OTHER divergence,
  the fixture running without a deadline where the seam structurally refuses one, is
  accepted as-is and deliberately NOT guarded; it is recorded instead at `runGit` and as a
  docs/pitfalls.md entry, because its cost is diagnostic (a blocked fixture hangs to the
  test binary's timeout rather than failing fast) and the note is what a future
  investigation needs to start in the right place.
- Phase 7 SPLIT, and the plan's Task 7.5 is superseded on ordering. The plan places the
  claim batches and both Implemented flips inside the phase-closing commit, but the
  rendered execution contract now holds that the final operations batch and the flip land
  only after terminal review settles; that contract postdates this plan. The split is not
  discretionary: a terminal status freezes an ADR body, a claim cannot be withdrawn
  without a `remove` operation, and `Implementing` requires a pending operation, so an
  all-applied non-terminal state is not expressible and a premature flip cannot be walked
  back (the same corner ADR-0187 was forced into). Because ADR-0191's operations are
  declared indivisible, the claims cannot land early either, and a proof marker citing an
  unapplied claim fails `awf check`. Phase 7 therefore lands as 7.1 and 7.4 first
  (walkers, entrypoint table, isolation table, obligated docs, changelog) with the proof
  markers withheld, and Tasks 7.2, 7.3 and 7.5 land together in the final transaction
  after terminal review, markers returning with their claims.
- Implementation-surfaced (Phase 7): the fixture walker owes
  `internal/testsupport/deps_test.go` NO allowlist entry after all. That carve-out was
  recommended for a text-matching walker, which cannot tell an import from a string
  constant; these walkers parse the import list, so the file's go-git path is invisible to
  them. `TestFixtureAllowlistEntriesAreAllLoadBearing` proved it by removing each entry and
  requiring a finding, and the entry was deleted rather than kept as harmless.
- Implementation-surfaced (Phase 7): the entrypoint table found `Branches` reached only by
  the cancellation and error suites, with nothing asserting the set it returns. A body
  returning an empty map, full ref names, or remote branches alongside local ones would
  have stayed green. `TestBranchesReportsEveryLocalBranchShortName` now pins it. This is
  the table earning its place on its first run.
- Correction to this plan's Notes: the free-entrypoint list of four is incomplete. The seam
  exports seven free functions - `Open`, `OpenContaining`, `ResolveControlRoots`,
  `ListWorktreeRegistrations`, `MergeInProgress`, `ParseRange`, `ProjectResidentRoot` - and
  all seven satisfy the free-entrypoint principle, since each precedes an opened repository
  or does without one. The entrypoint table enumerates from source rather than from a list,
  so it is unaffected by the omission; `ProjectResidentRoot` is the one entrypoint whose
  suite lives outside `internal/git`, in `cmd/awf`, beside its only consumer.
- Terminal review settlement (2026-07-31). The review returned "do not freeze yet" with three
  blockers, all now closed, plus concerns settled in the same pass.
  B1, the parity claim asserted a two-sided guarantee that was one-sided: the fixture lane's
  isolation was exhaustively pinned while the SEAM's six pins could all be deleted with the
  whole repository green, because Git's defaults are benign under a temporary HOME and no
  behavioural test can see the difference. `internal/git/isolation_test.go` adds the mirror
  table, mutation-verified pin by pin, and ADR-0191 item 11 now states what two tables prove
  rather than asserting automatic detection.
  B2, the two walker claims read as "nothing reaches Git outside the seam" while the walkers
  decide a narrower question. ADR item 10 now scopes both claims to Go source constructions in
  this module and names the three things deliberately outside that scope: a Go test may exec a
  shell script that runs git, the Pi TypeScript extensions run their own git subprocesses
  (including a working-tree cleanliness read and a second gitfile resolution, named as
  follow-up work rather than covered), and a test may forge `.git` internals with ordinary file
  writes. The library prefix also broadened to the whole `github.com/go-git/` organisation,
  which the narrower form failed to prefix-match against `go-git-fixtures`, and the seam
  allowlist gained the load-bearing test its fixture twin already had.
  B3, five `coverage-ignore` comments stated reachability facts that are false (the review
  found four; the verify pass found a fifth, ten lines from one of them and in the same
  class - resolving a merge base walks the graph, so an ordinary range inside a shallow
  clone's fetched window still runs off its boundary). Four are now covered by tests rather
  than reworded: a shallow clone's boundary commit resolves a recorded
  parent that was never fetched (NumParents counts hashes; resolving one is an object lookup),
  a malformed `packed-refs` line fails the eager branch enumeration, and the context check
  inside `blobsOfTree` makes both of its callers' error branches reachable on a healthy
  repository. The fourth, the same parent lookup inside the range walk, keeps an escape with a
  corrected justification: the walk fails while enumerating ancestry, before any commit reaches
  `toCommit`.
  Concerns: `Root` was registered against a suite that called a same-named method on the
  fixture type and never touched the seam's, so the entrypoint table passed while proving
  nothing; `TestWorkingPaths` now exercises it, and the table additionally requires a
  registered suite to name its entrypoint. Six entrypoint behaviours survived mutation
  repo-wide, including two safety properties their own doc comments assert - `BranchDelete`
  refusing unmerged work and `MergeFastForward` refusing anything that would create a commit -
  both now pinned and mutation-verified. `TestRangeNativeReadOperations` was a smoke test
  wearing a contract suite's registration: its two-commit linear history could not distinguish
  merge-base from rev-parse, a diff against base from a diff of base..head, or -U0 from -U3. It
  now builds a fork with an intervening commit, a dirty working tree, and repository-local diff
  configuration, which falsifies four of the five flags; `diff.mnemonicprefix` is subsumed by
  the other two under every reachable configuration and is documented as defence rather than
  claimed as proven.
  Also: the two-minute deadline ceiling had three independent copies and now has one home in
  the seam that both binaries reference; `CommandError` names a timeout or cancellation instead
  of rendering a killed process as an unexplained "exit status -1"; the runner's "only
  subprocess boundary" comment was false and now names its one sibling; and the changelog's
  claim that managed-worktree operations previously inherited the ambient Git environment was
  wrong (they were already isolated) and omitted a change in the permissive direction (the
  cleanliness answer now honours the global gitignore), both corrected.
- Follow-up, NOT in this effort's scope: the Pi TypeScript extensions run their own git
  subprocesses and reimplement the gitfile resolution `internal/git` owns. Consolidating them
  needs its own decision, because the seam is a Go package and the extensions are a different
  runtime.
- Verify pass on the terminal-review settlement (2026-07-31) returned "not safe to freeze"
  and found four more things, all closed here.
  The two repo-walkers hand-rolled their own traversal and pruned only top-level `.git`,
  `examples`, and `testdata`, so they flagged every file inside a managed worktree under
  `.awf/worktrees/` and inside `.claude/worktrees/`. Both walkers therefore FAILED in the
  primary checkout whenever an effort worktree existed, which is the normal state during an
  effort and exactly the state integration leaves behind: the two claims they back were
  backed by tests that could not be run where they matter. The repository already had one
  definition of that boundary, `testsupport.WalkRepoFiles`, whose own comment calls itself
  the single definition of it. Routing through it is both the fix and the point: a
  hand-rolled duplicate of a documented single home, inside the effort about single homes.
  A fifth false `coverage-ignore` was found ten lines from one the review had already
  flagged and in the same class: resolving a merge base walks the graph, so an ordinary
  range wholly inside a shallow clone's fetched window still runs off its boundary. Covered.
  The entrypoint table's reference check was vacuous: printing the whole `FuncDecl` includes
  the function's own NAME, so seventeen of the thirty-five registrations were satisfied by
  the test identifier alone. It now prints the body only, which surfaced one registration
  reaching its entrypoint solely through a helper. The same-named-method residual remains
  and is disclosed at the check, which is why the registry comment demands a suite that
  asserts what the entrypoint ANSWERS rather than one that merely mentions it.
  ADR item 11's "adding one cannot pass without a deliberate edit naming the other" was
  false: both tables swept only `GIT_`-prefixed keys, while two of the six pins carry no
  such prefix, so a seventh pin from that same credential-helper family passed green. Both
  tables now assert the whole resulting environment by equality, which makes an added pin
  fail as loudly as a dropped one, and the ADR sentence says why equality is what is needed.
  Also corrected: ADR item 12 still described the parity backing as one table; Tasks 7.1 and
  7.3 still directed a single marker onto the fixture table, which would have left the seam
  half - the half the review was raised about - unmarked; and the changelog's global-gitignore
  bullet was half wrong, since `awf audit`'s cleanliness read already honoured the global
  ignore before this effort (it ran with no environment override at all) while the worktree
  refusal did not. Smaller: the walker now detects an aliased `os/exec` import and an
  `exec.Cmd{Path: "git"}` literal, so item 10's scope sentence is true as written, and
  testsupport's copy of the deadline ceiling now names the seam's const the way the seam's
  already named it.
