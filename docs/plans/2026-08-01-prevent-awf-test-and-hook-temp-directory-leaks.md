---
date: 2026-08-01
adrs: []
status: Implemented
---
# Plan: Prevent awf test and hook temp directory leaks

## Goal

Stop the deterministic staged-slice leak in the repository pre-commit hook, bound interrupted
Go `TestMain` HOME residue under one conservatively reaped awf-owned tree, and provide a safe
repo-private recovery command. Non-goals are cleaning arbitrary Go `t.TempDir` residue, claiming
cleanup after `SIGKILL`, changing the shipped awf hook payload, or changing the current Windows
release matrix.

## Architecture summary

The hand-written `.githooks/pre-commit` stub remains the owner of its staged slice. It deletes the
slice after `.githooks/check-nested-staged` returns, keeps the `EXIT` trap armed until that deletion
succeeds, disarms the trap only after success, and then replaces itself with the rendered payload.
A real-hook regression runs with an isolated `TMPDIR` and proves the payload receives an empty temp
root.

`internal/testsupport` gains the single test-temp policy owner. On Linux and macOS it selects
`<os.TempDir()>/awf-test-homes-<effective-uid>`, validates an absolute non-root real directory owned
by the effective user with no group or other permission bits, and allocates only `home-<decimal>`
children through `os.MkdirTemp(root, "home-")`. The manager receives its root, clock, filesystem
operations, and platform validator through constructors or parameters; it introduces no mutable
package-global seam. Cleanup considers only real, safe, canonical child directories. Safe mode
selects homes whose modification time is strictly before `now-24h`; force mode selects every
canonical home. Root-level symlinks and unrelated entries are preserved. Candidate traversal sums
regular-file logical sizes before deletion, but the result counts a home and its bytes only when
removal succeeds. A concurrent `fs.ErrNotExist` means no removal and no error; every other
candidate failure is retained in deterministic path order and returned after the remaining
candidates are attempted.

`RunIsolated` composes the production manager, warns but continues when the startup stale sweep has
partial failures, fails closed when root preparation or home allocation fails, sets `HOME`, runs
the suite, and removes the current home. A current-home removal error changes an otherwise-zero
suite result to 1 while preserving an existing nonzero result. Linux and macOS own the real
ownership implementation; a Windows build-tagged file returns an unsupported-platform error only
to retain compile compatibility.

The repo-private `cmd/testtmpclean` is the outside-package production consumer of the exported
cleanup mode and entrypoint. `./x clean-test-tmp` invokes it. The default is the same strict stale
selection; `--all` prints an explicit concurrent-test warning before cleanup. Result rendering
stays in `internal/testsupport`; the command owns only argument parsing, warning selection, and
exit mapping. A successfully scanned cleanup always prints one summary line, partial cleanup prints
that summary and exits nonzero after printing the joined error, and a root/platform failure exits
nonzero without claiming a removal.

Every path below is repository-root-relative. At the start of Phases 1 through 3, the owner must
enter the managed worktree, run `worktree_root=$(git rev-parse --show-toplevel)`, verify
`git branch --show-current` prints `awf/prevent-awf-test-and-hook-temp-directory-leaks`, and
`cd "$worktree_root"` before running the baseline or tasks. Phase 4 instead runs in the integration
checkout after integration, as its own baseline specifies. These rules resolve one exact execution
root without baking a machine-specific checkout location into the durable plan.

## File structure

Created:

- `internal/testsupport/testtemp.go`: common manager, canonical-name and age policy, cleanup result
  rendering, production entrypoint, and `RunIsolated` orchestration.
- `internal/testsupport/testtemp_unix.go`: Linux/macOS root selection, effective-user ownership, and
  permission validation.
- `internal/testsupport/testtemp_windows.go`: compile-compatible unsupported-platform selection.
- `internal/testsupport/testtemp_test.go`: injected manager, cleanup, accounting, concurrency, error,
  rendering, and `RunIsolated` tests.
- `internal/testsupport/testtemp_unix_test.go`: Linux/macOS real-root and permission safety tests.
- `cmd/testtmpclean/main.go`: repo-private cleanup command, parser, warning, and exit composition.
- `cmd/testtmpclean/main_test.go`: argument, output, warning, partial-failure, and runner-dispatch
  tests.

Modified:

- `.githooks/pre-commit`: remove the staged slice before rendered-payload handoff.
- `cmd/awf/check_test.go`: retain the static hook contract and add the isolated-`TMPDIR` real-hook
  regression.
- `internal/testsupport/testsupport.go`: narrow the package ownership comment and move
  `RunIsolated` into the cohesive test-temp implementation.
- `cmd/awf/testmain_test.go`, `internal/audit/testmain_test.go`, `internal/git/git_test.go`,
  `internal/project/testmain_test.go`, `internal/snapshot/snapshot_test.go`: call parameterless
  `RunIsolated`.
- `x`: add `clean-test-tmp`, pass its arguments to `cmd/testtmpclean`, and extend usage.
- `.awf/docs/parts/development/command-runner.md`: document the cleanup command and modes.
- `.awf/docs/parts/testing/layout.md`: document managed `TestMain` homes and the hook regression.
- `.awf/docs/parts/architecture/components.md`: state test-temp manager and private-command
  ownership.
- `.awf/docs/parts/roadmap/deferred.md`: narrow interrupted temp residue to unmanaged
  `t.TempDir` homes and record future removal of unsupported Windows release targets.
- `docs/development.md`, `docs/testing.md`, `docs/architecture.md`, `docs/roadmap.md`: rendered
  outputs of the changed convention parts.
- `.awf/awf.lock`: rendered hashes for the changed generated documents.
- `docs/plans/2026-08-01-prevent-awf-test-and-hook-temp-directory-leaks.md`: record terminal
  implementation findings and freeze the plan after implementation review settles.

Deleted: none.

## Phase 1: Stop the deterministic pre-commit staged-slice leak

**Execution mode: subagent-driven.** Start only from a clean managed worktree where
`git status --short` prints nothing, `./x check` exits zero with no drift finding, and
`go test ./cmd/awf -run '^TestRepositoryPreCommit'` exits zero.

- [ ] **Task 1.1: Add the failing real-hook cleanup regression.** In `cmd/awf/check_test.go`, add
  `TestRepositoryPreCommitRemovesSliceBeforePayload`. Reuse the existing real-hook fixture shape:
  initialize a `gitfixture` repository, stage an executable copy of
  `.githooks/check-nested-staged`, create the required `examples/sundial` directory in the staged
  slice, and provide a fake `go` executable that returns success for the slice-wide build and
  writes the existing wrapper-style executable for `go build -o awf-slice ./cmd/awf`. The wrapper
  must return zero for both `check` and `check --staged`; it must not run this repository's real
  gate.

  Stage a fixture `.awf/hooks/pre-commit.sh` that fails unless its inherited `TMPDIR` contains no
  entry, then writes a marker outside `TMPDIR`. Point only the child hook process at a fresh empty
  `TMPDIR`, prepend the fake tools directory to its `PATH`, execute the real repository
  `.githooks/pre-commit` with the fixture repository as `cmd.Dir`, and assert all of these
  observables:

  1. the hook exits zero;
  2. the payload marker exists, proving handoff occurred;
  3. `os.ReadDir(tmpRoot)` returns an empty slice after exit, proving no staged slice remains.

  Keep tools and the payload marker outside the isolated `TMPDIR`, so the emptiness assertion
  measures only hook-created paths. Run
  `go test ./cmd/awf -run '^TestRepositoryPreCommitRemovesSliceBeforePayload$'`; it must fail on the
  current `exec` path because the payload observes the `mktemp -d` directory.

- [ ] **Task 1.2: Remove the slice before replacing the hook shell.** In
  `.githooks/pre-commit`, immediately after
  `bash "$staged_helper" "$repo_root" "$awf_slice"`, insert exactly this lifecycle ordering:

  ```bash
  rm -rf -- "$tmp"
  trap - EXIT
  exec bash .awf/hooks/pre-commit.sh "$@"
  ```

  Replace the old direct `exec` rather than adding a second payload call. Do not disarm the trap
  before `rm -rf` succeeds: under `set -e`, a failed explicit removal must still leave the trap
  available for a retry during shell exit. Do not move cleanup earlier than the nested staged
  helper, its last consumer. Do not edit generated `.awf/hooks/pre-commit.sh`.

  Extend `TestRepositoryPreCommitHasOnlyPermanentPath` so its required static sequence includes
  `rm -rf -- "$tmp"`, `trap - EXIT`, and the existing `exec`, and rejects an `exec` that precedes
  cleanup. Then run
  `go test ./cmd/awf -run '^(TestRepositoryPreCommitHasOnlyPermanentPath|TestRepositoryPreCommitRejectsSliceMissingNestedHelper|TestRepositoryPreCommitInvokesNestedStagedHelperForInvalidTransition|TestRepositoryPreCommitRemovesSliceBeforePayload|TestHookCommandHelper)$'`;
  it must exit zero.

- [ ] **Task 1.3: Document the repaired hook lifecycle in the same transaction.** Append one
  sentence to `.awf/docs/parts/testing/layout.md` naming the isolated-`TMPDIR` real-hook regression
  and stating that the hand-written stub removes its staged slice before rendered-payload handoff.
  Run `./x render`, review only the corresponding `docs/testing.md` and `.awf/awf.lock` changes,
  and run `./x check`; it must exit zero with no drift or note finding. Do not edit generated
  `docs/testing.md` directly.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage only `.githooks/pre-commit`,
  `cmd/awf/check_test.go`, `.awf/docs/parts/testing/layout.md`, `docs/testing.md`, and
  `.awf/awf.lock`, then create the one phase-closing commit. It requires
  `awf check --staged` and `./x gate` to pass, enforced by a wired pre-commit hook or run manually
  first in a clone without one (checkable with `git config core.hooksPath`):

```commit
fix(tooling): remove the staged hook slice before payload handoff
```

## Phase 2: Manage `TestMain` homes under one stale-reaped root

**Execution mode: subagent-driven.** Start only after Phase 1 is committed, with
`git status --short` empty, `./x check` clean, `go test ./internal/testsupport` green, and
`go test ./cmd/awf ./internal/audit ./internal/git ./internal/project ./internal/snapshot` green.

- [ ] **Task 2.1: Write manager boundary, safety, and accounting tests first.** Create
  `internal/testsupport/testtemp_test.go` as `package testsupport`, so it can construct the manager
  with local injected dependencies without exporting a test seam. Use real directories below
  `t.TempDir()` for the common path and local copies of an `osTestTempFS()` function bundle with
  one operation replaced to induce each fault. Do not add a package-level swappable variable.

  Pin these independent behaviors with focused tests:

  - root preparation rejects empty, relative, unclean, filesystem-root, symlink, non-directory,
    foreign-owner/platform-validator, and group/world-accessible roots; it accepts an existing safe
    root and the create race where `mkdir` reports `fs.ErrExist` and the subsequent `lstat` sees the
    safe directory;
  - allocation creates only a direct child matching `^home-[0-9]+$` and never accepts a path
    outside the validated root;
  - safe selection preserves equality at exactly `now-24h`, preserves newer homes, and selects a
    home one nanosecond older; force selection includes all canonical safe homes regardless of age;
  - unrelated files/directories, names such as `home-`, `home-abc`, nested lookalikes, and
    root-level symlinks named both canonically and noncanonically remain untouched;
  - a canonical entry that is not a safe owned directory produces a per-path failure and is not
    removed;
  - logical-byte accounting sums regular-file `Info.Size()` values before removal, excludes
    directories and symlink targets, and increments neither homes nor bytes when traversal or
    removal fails;
  - candidate inspection and deletion attempts occur in sorted path order, multiple failures are
    joined in that same order, cleanup continues after one failure, and the aggregate result does
    not retain removed paths solely for tests;
  - `fs.ErrNotExist` from candidate `lstat`, traversal, or removal models concurrent disappearance
    and contributes neither removal nor failure;
  - invalid roots and root-read failures abort without touching any child;
  - result rendering is exactly
    `test temp cleanup: removed <homes> home(s), <bytes> logical byte(s)\n`, including zero values.

  Create `internal/testsupport/testtemp_unix_test.go` with `//go:build linux || darwin`. Exercise
  the production root selector against an injected temp base, assert its basename uses the current
  effective UID and its permissions exclude group/other access, and prove a chmod-created unsafe
  root is refused. The actual foreign-owner branch may carry one reasoned `coverage-ignore`
  because an unprivileged test cannot create that fixture; every portable branch remains covered.

  Run `go test ./internal/testsupport -run 'TestTestTemp|TestCleanup'`. It must fail to compile
  before the implementation files exist.

- [ ] **Task 2.2: Implement the common manager with explicit dependencies.** Create
  `internal/testsupport/testtemp.go`. Use a private `testTempFS` function bundle containing only
  the operations the policy consumes (`mkdir`, `mkdirTemp`, `lstat`, `readDir`, `walkDir`, and
  `removeAll`), an `osTestTempFS()` constructor, and a `testTempManager` constructed with an
  explicit root string, `func() time.Time`, filesystem bundle, and platform validation function.
  Reject nil required dependencies at construction; do not silently default them in the injected
  constructor. The production constructor supplies `time.Now`, the OS bundle, and the
  build-tagged root/validator.

  Define the common policy exactly as follows:

  - `testHomePrefix = "home-"`, canonical names are that prefix followed by one or more ASCII
    decimal digits and nothing else;
  - `staleTestHomeAge = 24 * time.Hour`, with safe selection implemented as
    `modTime.Before(now.Add(-staleTestHomeAge))`, never `<=`;
  - `ensureRoot` requires a cleaned absolute path whose volume root is not the path itself, creates
    the directory with `0o700`, resolves an `fs.ErrExist` race by re-inspection, uses `Lstat` rather
    than following a symlink, requires a real directory, and invokes the platform validator;
  - candidate enumeration sorts `ReadDir` entries by name before processing and accepts only a
    canonical `Lstat`-verified real directory that passes the same platform safety check;
  - size traversal never follows symlinks and adds only nonnegative regular-file logical sizes;
  - deletion occurs only after selection, safety, and complete size traversal; a successful
    `RemoveAll` contributes one home and its precomputed bytes; wrapped `fs.ErrNotExist` at any
    candidate step is ignored as concurrent disappearance; all other errors retain path and cause
    identity and do not stop later candidates.

  Keep cleanup result data and its deterministic human rendering in this file. In this phase the
  cleanup mode and production command facade may remain private because `RunIsolated` is the first
  consumer; Phase 3 exports only the surface consumed by `cmd/testtmpclean`.

- [ ] **Task 2.3: Add Unix ownership and compile-only Windows selection.** Create
  `internal/testsupport/testtemp_unix.go` with `//go:build linux || darwin`. Production selection
  must use `filepath.Join(os.TempDir(), fmt.Sprintf("awf-test-homes-%d", os.Geteuid()))`.
  Validation must reject a non-directory or symlink, an owner UID different from `os.Geteuid()`,
  and any `info.Mode().Perm()&0o077 != 0`; compare owner data through `*syscall.Stat_t` and return a
  descriptive error if the ownership representation is unavailable rather than assuming safety.

  Create `internal/testsupport/testtemp_windows.go` with `//go:build windows`. It must define the
  same production-selection and validation hooks but return a stable unsupported-platform error;
  it must not approximate Unix ownership with Windows ACL behavior. This file exists only so
  `GOOS=windows go build ./...` remains green until the release matrix is changed by a later
  effort.

- [ ] **Task 2.4: Move and harden `RunIsolated`, then update every caller.** Remove the old
  prefix-taking implementation and example from `internal/testsupport/testsupport.go`. In
  `testtemp.go`, expose `RunIsolated(m *testing.M) int` and delegate to an unexported
  `runIsolated(run func() int, setHome func(string) error, manager *testTempManager,
  stderr io.Writer) int` so tests inject the suite result and failures without mutable globals.
  The production function constructs the production manager and supplies `m.Run`, `os.Setenv`, and
  `os.Stderr`.

  Order the helper as: prepare/validate root; run safe stale cleanup; write one
  `testsupport: stale test-home cleanup: <error>\n` warning if that cleanup is partial; allocate a
  fresh home; set `HOME`; run the suite; remove the current home. Root preparation, allocation, or
  `HOME` assignment errors fail closed using the existing `RunIsolated` panic contract. Stale
  cleanup failure does not block allocation or the suite. If current-home removal fails, write
  `testsupport: remove current test home <path>: <error>\n` and return 1 only when the suite
  returned 0; preserve every nonzero suite code.

  Add tests for call ordering, best-effort stale warning, each fail-closed pre-run error, successful
  current-home removal, zero-to-one cleanup mapping, and preservation of a nonzero suite result.
  Update the complete caller set to `testsupport.RunIsolated(m)`:

  - `cmd/awf/testmain_test.go`
  - `internal/audit/testmain_test.go`
  - `internal/git/git_test.go`
  - `internal/project/testmain_test.go`
  - `internal/snapshot/snapshot_test.go`

  Update the `internal/testsupport/testsupport.go` package comment so managed `TestMain` HOME
  lifecycle is named as one owned concern while the package remains a standard-library-only leaf.
  Run `rg 'RunIsolated\([^)]*,' --glob '*.go'`; it must return no output. Then run
  `go test ./internal/testsupport ./cmd/awf ./internal/audit ./internal/git ./internal/project ./internal/snapshot`;
  it must exit zero.

- [ ] **Task 2.5: Prove the safety boundaries are mutation-sensitive.** Temporarily make each of
  these representative mutations one at a time: change the strict `Before` comparison to include
  equality, accept a nondecimal canonical suffix, skip the root symlink check, count bytes before a
  failed removal, and return 0 after current-home removal fails. For each mutation,
  `go test ./internal/testsupport` must fail at a named assertion. Revert each mutation and confirm
  `go test ./internal/testsupport` exits zero. Then run `./x mutants ./internal/testsupport`; it
  must complete without a timeout, and every non-equivalent survivor affecting the new manager
  must be killed by an added assertion.

- [ ] **Task 2.6: Document managed homes and the remaining interruption boundary.** Edit the
  owning sources and render in this same transaction:

  - append to `.awf/docs/parts/testing/layout.md` that the five `TestMain` suites allocate
    `home-<decimal>` below one per-effective-user root, automatically sweep only homes strictly
    older than 24 hours, and map failure to remove the current home;
  - append an `internal/testsupport` component bullet to
    `.awf/docs/parts/architecture/components.md`, stating that it owns isolated-home allocation,
    conservative cleanup, logical-byte accounting, and human result rendering;
  - replace `The test suite leaks temp homes on interrupted runs` in
    `.awf/docs/parts/roadmap/deferred.md` with a narrowed item for unmanaged Go `t.TempDir`
    directories: managed `TestMain` homes are bounded and recoverable, but arbitrary `t.TempDir`
    cleanup still cannot survive abrupt process death and is not selected by the manager;
  - add a separate roadmap item to remove Windows from `.goreleaser.yaml` and the cross-compile
    gate in a future release-policy change, noting that this phase retains compile compatibility
    but implements test-temp ownership only on Linux and macOS.

  Run `./x render`; review the resulting `docs/testing.md`, `docs/architecture.md`,
  `docs/roadmap.md`, and `.awf/awf.lock` diffs and run `./x check`, which must exit zero. Do not
  hand-edit generated outputs.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the new manager files,
  `internal/testsupport/testsupport.go`, the five exact caller files listed in Task 2.4, the three
  `.awf/docs/parts/` files from Task 2.6, `docs/testing.md`, `docs/architecture.md`,
  `docs/roadmap.md`, and `.awf/awf.lock`. Create the one phase-closing commit after
  `awf check --staged` and `./x gate` pass, enforced by a wired
  pre-commit hook or run manually first in a clone without one:

```commit
feat(tooling): manage isolated test homes under a stale-reaped root
```

## Phase 3: Add explicit cleanup tooling and current documentation

**Execution mode: subagent-driven.** Start only after Phase 2 is committed, with
`git status --short` empty, `./x check` clean, `go test ./internal/testsupport` green, and
`GOOS=windows GOARCH=amd64 go build ./...` exiting zero.

- [ ] **Task 3.1: Write the cleanup command contract tests first.** Create
  `cmd/testtmpclean/main_test.go` in `package main`. Structure production as
  `run(args []string, stdout, stderr io.Writer, clean cleanerFunc) int`, where the injected
  function is a parameter and never a package-global seam. Add independent tests that assert:

  - no arguments selects stale cleanup, emits no warning, forwards the manager-owned summary to
    stdout, leaves stderr empty, and exits 0;
  - exactly `--all` writes
    `testtmpclean: warning: --all can remove homes used by concurrent test processes\n` to stderr
    before invoking cleanup, selects force cleanup, forwards the summary to stdout, and exits 0;
    have the injected cleaner assert that the warning is already present at call time so ordering
    is deterministic without comparing separate streams;
  - any unknown argument or more than one argument prints exactly
    `usage: testtmpclean [--all]\n` to stderr, leaves stdout empty, does not invoke cleanup, and
    exits 2;
  - a partial cleanup preserves the stdout summary, prints `testtmpclean: <error>\n` to stderr, and
    exits 1;
  - a root or unsupported-platform failure prints the prefixed error to stderr, leaves stdout
    empty, and exits 1.

  Also add repository contract tests that read root `x` and the new command source. Require the
  `clean-test-tmp)` dispatch, `go run ./cmd/testtmpclean "$@"`, and the command in the usage line;
  reject the summary literal `test temp cleanup:` anywhere under `cmd/testtmpclean`, while the
  exact-rendering test in `internal/testsupport` pins that literal at its owning package. Run
  `go test ./cmd/testtmpclean`; it must fail before the command exists.

- [ ] **Task 3.2: Export the narrow manager facade and implement the private command.** In
  `internal/testsupport/testtemp.go`, export documented `CleanupMode`, `CleanupStale`,
  `CleanupAll`, and `CleanTestTemps(mode CleanupMode, output io.Writer) error`. This function
  constructs the production manager, runs the selected cleanup, writes the manager-owned summary
  after every successful root scan (including partial removal), and returns the deterministic
  joined candidate errors. Reject an unknown mode before touching the root. Keep result fields,
  formatting, filesystem dependencies, and constructors private. Implement the facade over an
  unexported `cleanTestTemps(mode, output, managerFactory)` helper whose factory is a required
  parameter; the exported function supplies the production factory, while tests inject a local
  manager without a mutable global.

  Extend `internal/testsupport/testtemp_test.go` with direct facade tests. Prove an unknown mode
  returns before invoking the factory or writing output; stale and all modes reach the matching
  manager selection; successful and zero-removal scans write the exact summary; a partial removal
  writes the summary and returns the joined error; and root preparation failure returns the error
  without writing a summary. The Unix production-root test remains responsible for proving the
  exported facade's production factory selects the per-effective-user root.

  Create `cmd/testtmpclean/main.go` with a one-sentence package comment stating that the command
  owns repo-private test-temp cleanup invoked by `./x clean-test-tmp` and is not part of the shipped
  `awf` CLI. `main` calls the tested `run` boundary and maps its code to `os.Exit`; keep only that
  process boundary under reasoned coverage ignores. The command parses arguments and selects an
  exported cleanup mode, prints the force warning to stderr before invoking cleanup, passes stdout
  to `testsupport.CleanTestTemps`, prints prefixed returned errors to stderr, and maps usage to 2
  and cleanup failures to 1. It must not reproduce canonical-name, stale-age, byte-count, or result
  formatting policy.

- [ ] **Task 3.3: Wire `./x clean-test-tmp`.** In `x`, add a `clean-test-tmp)` case that executes
  `go run ./cmd/testtmpclean "$@"`. Let the Go command own the optional-argument grammar so direct
  and runner invocation cannot drift. Add `clean-test-tmp [--all]` to the single usage line. Do not
  add this recovery command to `gate`, `test`, or hook execution, and do not broaden it to arbitrary
  `/tmp` entries. Run `go test ./cmd/testtmpclean ./internal/testsupport`,
  `./x clean-test-tmp unexpected`, and `./x clean-test-tmp --all unexpected`; the tests must pass
  and both manual invalid invocations must print the usage, report the command's underlying
  `exit status 2`, and exit nonzero without deleting a home. The direct command tests, rather than
  the `go run` wrapper, pin the command's usage mapping to 2 because the Go tool itself exits 1
  when the invoked program exits nonzero.

- [ ] **Task 3.4: Document the explicit cleanup command and render.** Edit only the owning
  sources for behavior introduced in this phase:

  - add a `./x clean-test-tmp [--all]` row to
    `.awf/docs/parts/development/command-runner.md`, stating the strict 24-hour default, the
    concurrency warning and all-canonical-home behavior of `--all`, the Linux/macOS support
    boundary, and that partial cleanup exits nonzero;
  - add `cmd/testtmpclean` to the repo-owned supporting commands in
    `.awf/docs/parts/architecture/components.md`, stating that it owns only parsing, warning, and
    exit mapping while `internal/testsupport` owns cleanup and rendering.

  Run `./x render`; review only the corresponding `docs/development.md`, `docs/architecture.md`,
  and `.awf/awf.lock` changes. Do not hand-edit generated files. `./x check` must exit zero with no
  drift or note finding.

- [ ] **Task 3.5: Verify command error and mutation behavior.** Run
  `go test ./cmd/testtmpclean ./internal/testsupport`; it must exit zero. Temporarily invert the
  `--all` selection, suppress the warning, map a cleanup error to zero, and move result formatting
  into the command one mutation at a time; each behavioral mutation must make a named command or
  manager test fail, and introducing the summary literal under `cmd/testtmpclean` must fail the
  structural ownership assertion.
  Revert every mutation. Run `./x mutants ./cmd/testtmpclean`; it must complete without a timeout,
  and every non-equivalent survivor in the new parser, warning, or exit mapping must be killed by
  an added assertion.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage `internal/testsupport/testtemp.go`,
  `internal/testsupport/testtemp_test.go`, `cmd/testtmpclean/`, `x`,
  `.awf/docs/parts/development/command-runner.md`,
  `.awf/docs/parts/architecture/components.md`, `docs/development.md`, `docs/architecture.md`, and
  `.awf/awf.lock`. Review `git diff --cached --name-only` and confirm it is exactly that
  transaction. Create the one phase-closing commit after `awf check --staged` and `./x gate` pass,
  enforced by a wired pre-commit hook or run manually first in a clone without one:

```commit
feat(tooling): add explicit test temp cleanup tooling
```

## Phase 4: Freeze the implementation record after reviewed integration

**Execution mode: inline.** The main thread owns this transaction in the integration checkout, not
the managed worktree. Defer it until Phases 1 through 3 are committed, the whole-effort
Verification section is green, terminal `awf-reviewing-impl` has settled, and the reviewed work has
been integrated into `main`. If integration required a divergent merge, defer again until the
required renewed implementation review settles on that integrated result. In the integration
checkout, verify `git branch --show-current` prints `main`, `git status --short` prints nothing,
the integrated history contains the three phase-closing commits, and `./x check` is clean before
editing the plan. The plan deliberately remains `status: Proposed` through implementation, review,
and integration so findings or decision corrections can still be recorded directly without
reopening a frozen lifecycle.

- [ ] **Task 4.1: Record actual implementation findings and freeze the plan.** In this plan's
  `## Notes`, append one `Implementation findings:` bullet. If execution matched the plan, use the
  exact text `- Implementation findings: implementation matched the plan; no deviations.` If it
  differed, name every created, modified, or omitted path and every behavioral deviation in that
  bullet, without changing the frozen design rationale. Then change frontmatter `status: Proposed`
  to `status: Implemented`. Make no other repository change in this transaction.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage only
  `docs/plans/2026-08-01-prevent-awf-test-and-hook-temp-directory-leaks.md`, verify
  `git diff --cached --name-only` prints exactly that path, and create the one phase-closing commit
  after `awf check --staged` and `./x gate` pass, enforced by a wired pre-commit hook or run
  manually first in a clone without one:

```commit
docs(plans): freeze temp hygiene implementation record
```

## Verification

After all phases, run these acceptance checks from the managed worktree:

1. `go test ./cmd/awf -run '^TestRepositoryPreCommit'` exits zero, including the real-hook
   isolated-`TMPDIR` regression.
2. `go test ./internal/testsupport ./cmd/testtmpclean` exits zero.
3. `rg 'RunIsolated\([^)]*,' --glob '*.go'` returns no output, proving no caller retains an
   arbitrary prefix.
4. `rg '"awf-(project-|audit-|git-|snapshot-)?test-home"' --glob '*.go'` returns no output, proving
   the old unmanaged home grammars are gone.
5. `GOOS=windows GOARCH=amd64 go build ./...` exits zero, while Linux/macOS tests exercise the real
   ownership policy.
6. `./x render` exits zero and `git diff --exit-code` reports no newly rendered drift.
7. `./x check` exits zero with no drift, invariant, or example-adopter finding.
8. `./x gate` exits zero, including 100% statement coverage, vet, release-target cross-compiles,
   lint, dead-code, and pin checks.

## Notes

- The cleanup command deliberately ignores arbitrary Go `t.TempDir` directories and every
  noncanonical entry below the manager root.
- Automatic and explicit cleanup reduce and recover interrupted-run residue; neither promises to
  execute after abrupt process death or `SIGKILL`.
- Windows remains in the compile matrix only. Removing it from release policy is a separate roadmap
  decision, not a shortcut in this implementation.
- No ADR or preparatory refactor is required because all behavior is repo-private and the ownership
  boundary is already settled.
- Implementation findings: no planned path was created, modified, or omitted differently; terminal
  review hardened `internal/testsupport/testtemp.go` to fail closed when its pre-run stale-cleanup
  warning cannot be written, documented the no-fallback diagnostic behavior in
  `cmd/testtmpclean/main.go`, and made unsafe-root fixture setup failures explicit in
  `internal/testsupport/testtemp_test.go`.
