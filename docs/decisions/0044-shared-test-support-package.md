---
status: Proposed
date: 2026-07-01
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [testing, tooling]
related: [0006, 0020]
domains: [tooling]
---
# ADR-0044: Shared Test-Support Package

## Context

A 7-agent parallel code review (every finding independently re-verified against source by a
fresh-context agent) found that the ~11,000-line test suite — nearly double the ~6,300 lines of
production code — has no shared test-support package anywhere in the repo, and no `testdata/`
fixture directories. Instead, the same handful of fixture-building idioms have been hand-rolled
independently, repeatedly, across packages:

- **Git-repo fixtures.** `internal/audit/git_test.go` defines `testSig` (a fixed
  `*object.Signature`), `initRepo` (`git.PlainInit` + `t.TempDir`), and `commit(repo, dir, msg
  string, write map[string]string, remove ...string) plumbing.Hash`. `cmd/awf/audit_test.go`
  independently defines `auditSig` (the identical Name/Email/When values under a different name)
  and `auditCommit(repo, root, msg string, write map[string]string) plumbing.Hash` — the same
  `Worktree`/`Add`/`Commit` sequence, but with the `remove` capability silently dropped. This is a
  live copy-drift symptom, not a considered simplification: the second copy is quietly weaker than
  the first.
- **`TestMain` HOME isolation.** `cmd/awf/testmain_test.go` and `internal/audit/testmain_test.go`
  define byte-identical `TestMain` bodies (`os.MkdirTemp`, `os.Setenv("HOME", ...)`, `m.Run()`,
  `os.RemoveAll`, `os.Exit`) isolating go-git's global-gitignore read from the developer's real
  `HOME`, differing only in the temp-dir prefix string.
- **Project-fixture setup.** A "mkdir `.awf`, write `config.yaml`[, sync]" sequence is hand-rolled
  independently at least 7 times: `internal/project/project_test.go`'s `scaffoldFiles`,
  `cmd/awf/run_test.go`'s `scaffoldProject`, `cmd/awf/list_add_test.go`'s `scaffoldedProject`,
  `cmd/awf/audit_test.go`'s `auditProject`, two separate inline copies inside
  `cmd/awf/check_test.go`, `cmd/awf/gate_test.go`'s `gateFixture`, and an inline copy in
  `cmd/awf/invariants_test.go`.
- **ADR frontmatter fixtures.** Hand-rolled `---\nstatus: ...\n---\n# ADR-...` literals and ad hoc
  local helpers (each with a slightly different shape) are duplicated across 13+ test sites in 5
  packages: `internal/adr`, `internal/invariants`, `internal/project`, `internal/audit`,
  `cmd/awf`.
- **File-writing boilerplate.** No shared "write a file, creating its parent directories" helper
  exists. `internal/migrate/migrate_test.go`'s `mustWrite` and `internal/project/render_tree_test.go`'s
  `writeFileAt` are two independent local reimplementations of the same
  `os.MkdirAll(filepath.Dir(path))` + `os.WriteFile` shape; a repo-wide grep counts roughly 120 raw
  `os.WriteFile` and 56 raw `os.MkdirAll` call sites in `_test.go` files, the large majority written
  by hand because there is nothing to import.
- **Seam-swap idiom.** A `swap<Seam>(t *testing.T, fn <T>)` helper — `orig := seam; seam = fn;
  t.Cleanup(func() { seam = orig })` — is retyped identically for 4 different package-private seam
  variables: `cmd/awf/run_test.go`'s `swapGetwd`, `internal/coverage/coverage_test.go`'s
  `swapGetwd` and `swapHasGoMod`, and `cmd/awf/init_test.go`'s `forceNonInteractive`.
  (`internal/adr/adr_test.go`'s `swapNow` looks similar but is a genuinely different shape — the
  external test package `adr_test` has no exported pointer to `internal/adr`'s unexported `now`
  seam, only a `SetNowForTest(fn) (prev func() time.Time)` setter-returns-previous accessor. It
  does not fit a `*T`-pointer-based helper and is out of scope for this decision; it keeps its
  existing accessor.)
- **Coverage/covercheck fixtures.** `cmd/covercheck/main_test.go`'s `modWith` duplicates
  `internal/coverage/coverage_test.go`'s `module()`/`writeProfile()` fixture logic — the identical
  `go.mod`, `f.go`, and `cover.out`-prefix literals hardcoded a second time in a sibling package.

This repo's own history establishes the precedent for a dedicated package when a duplicated concern
needs one tested, canonical home: `internal/frontmatter` (ADR-0006) consolidated `---`-delimited
frontmatter parsing that had drifted across two ad hoc implementations, and `internal/refs`
(introduced alongside ADR-0020) consolidated markdown-link extraction. Both cases were framed
around a *binding constraint* — "all `---`-frontmatter parsing in non-test code routes through this
package" — not merely the existence of a new file. The same framing applies here: the binding
constraint worth recording is that `internal/testsupport` must stay a leaf package with **zero**
dependency on any other `internal/*` awf package, so it remains safely importable from *any*
package's tests (including `internal/config`, whose own tests a naive test-helper package could
otherwise be tempted to depend on, or be depended on by, creating an import cycle). Verified: the
only non-stdlib dependency needed by any of the consolidation targets above is
`github.com/go-git/go-git/v5` (already a direct `go.mod` dependency, used today by
`internal/audit/git_test.go` and `cmd/awf/audit_test.go` directly), and no `internal/*` package's
production code imports go-git in a way that could create a cycle back into a test-only consumer of
`internal/testsupport`.

## Decision

1. **Add `internal/testsupport`, a leaf package with zero `internal/*` imports.** Only the Go
   standard library may be imported directly by `internal/testsupport`'s own files. Every exported
   helper is `t.Helper()`-marked where it takes a `*testing.T`.

2. **`internal/testsupport/gitfixture`, a subpackage isolating the `go-git` dependency.** Kept
   separate from the parent package so a caller that only needs e.g. `testsupport.WriteFile` does
   not have to pull `go-git` into its test binary. API:
   - `Sig *object.Signature` — the fixed commit signature (`Name: "T"`, `Email: "t@example.com"`, a
     fixed UTC timestamp) both existing implementations already use identically.
   - `InitRepo(t *testing.T) (*git.Repository, string)` — `git.PlainInit` into a fresh `t.TempDir()`,
     returning the repository and its root path.
   - `Commit(t *testing.T, repo *git.Repository, dir, msg string, write map[string]string, remove
     ...string) plumbing.Hash` — writes/removes the given paths in the worktree, stages them, and
     commits with `Sig`. Superset of both prior implementations (keeps `git_test.go`'s `remove`
     capability that `audit_test.go`'s copy had silently dropped).

3. **`internal/testsupport` exports the following, one per consolidation target identified in
   Context:**
   - `RunIsolated(m *testing.M, prefix string) int` — the `TestMain` HOME-isolation body
     (`os.MkdirTemp(prefix)`, `os.Setenv("HOME", ...)`, `m.Run()`, `os.RemoveAll`, returns the exit
     code for the caller to pass to `os.Exit`). Each package's `TestMain` shrinks to
     `func TestMain(m *testing.M) { os.Exit(testsupport.RunIsolated(m, "<prefix>")) }`.
   - `WriteFile(t *testing.T, path, content string)` — `os.MkdirAll(filepath.Dir(path), 0o755)`
     then `os.WriteFile(path, []byte(content), 0o644)`, `t.Fatal`-ing on either error. The primitive
     every other helper below is built from.
   - `WriteAwfConfig(t *testing.T, root, yaml string)` — creates `<root>/.awf/config.yaml` with
     the given content via `WriteFile`.
   - `ADR(status string, opts ...ADROption) string` — builds a `---`-delimited ADR frontmatter
     fixture as a raw string template (not by constructing and marshaling `internal/adr`'s actual
     frontmatter struct — see Consequences for why). `ADROption` covers the fields the existing ad
     hoc fixtures vary: at minimum `WithDomains(...string)`, `WithTags(...string)`,
     `WithTitle(string)`; extend with further options as call sites migrate and need them.
   - `SwapVar[T any](t *testing.T, seam *T, val T)` — `orig := *seam; *seam = val; t.Cleanup(func()
     { *seam = orig })`. Covers `swapGetwd`, `swapHasGoMod`, and `forceNonInteractive`-shaped call
     sites (a `*T` package-private seam var in the same package as the test); does not cover
     `internal/adr`'s `swapNow` (see Context).
   - `WriteGoModule(t *testing.T, dir, modPath, srcBody string)` — writes a minimal `go.mod` (pinned
     to this repo's Go toolchain version) and an `f.go` containing `srcBody` under `dir`.
   - `WriteProfile(t *testing.T, dir, body string) string` — writes `dir/cover.out` with a
     `"mode: set\n"` prefix followed by `body`, returning the file's path.

4. **Migrate existing call sites.** Every duplication site named in Context is updated to call
   the new helpers instead of its local copy, across `cmd/awf`, `internal/project`,
   `internal/audit`, `internal/coverage`, `internal/migrate`, `internal/invariants`,
   `cmd/covercheck`. The exact task breakdown and sequencing is the plan's to decide; what this
   item commits to is the end state — no local copy of any Context-named idiom survives — plus the
   forward-looking contract that new test code reaches for `internal/testsupport` rather than
   hand-rolling a 6th copy of any of the above.

## Invariants

- `inv: testsupport-zero-internal-deps` — no non-test `.go` file under `internal/testsupport/`
  (including `internal/testsupport/gitfixture/`) imports any `github.com/hypnotox/agentic-workflows/internal/*`
  package; only the Go standard library and (in `gitfixture` only) `github.com/go-git/go-git/v5`
  and its subpackages are permitted. Backed by a test that inspects the package's own import
  graph (e.g. via `go/parser`/`golang.org/x/tools/go/packages` or an equivalent static check) and
  fails if a disallowed import appears — enforced mechanically, not left to code review, so a
  future helper that happens to need e.g. a `config.Config` cannot be added to this package by
  accident.
- Every helper that reports a test failure calls `t.Helper()` first, so failures attribute to the
  calling test's line, not a line inside `internal/testsupport`. (Textual contract — enforced by
  code review, not an `inv:`-tagged mechanical check, consistent with ADR-0020's and ADR-0039's
  untagged-bullet convention.)

## Consequences

Easier:
- New tests across any package reach for one canonical fixture-building surface instead of
  reinventing it; the git-fixture copy-drift (a silently weaker second implementation) cannot recur
  because there is only one implementation.
- A future change to any of the consolidated idioms (e.g. tightening `HOME` isolation, changing the
  project-fixture seed sequence) is a one-place edit instead of a hunt across 5-7 call sites.

Harder / accepted trade-offs:
- `ADR()` builds its fixture as a raw string template rather than importing `internal/adr` and
  marshaling its actual frontmatter struct, because importing `internal/adr` would break the
  zero-`internal/*`-dependency invariant this ADR exists to establish. This means a future change
  to `internal/adr`'s frontmatter schema will not be caught by the compiler in
  `internal/testsupport.ADR()` — the failure mode is a test that fails loudly when its fixture no
  longer parses as the code under test expects, not a silent divergence. This is judged an
  acceptable trade-off: every existing ad hoc fixture already has this exact property (they are all
  raw string literals today), so this is not a regression, and the loud-failure mode is self-
  correcting.
- `internal/testsupport` itself gets only light direct unit coverage; its correctness is
  substantively proven by the migrated call sites' existing test suites continuing to pass.
- Consolidation concentrates blast radius: today a bug in one hand-rolled copy (e.g. the dropped
  `remove` capability in `auditCommit`) breaks only its own package's tests. Once every package
  imports `internal/testsupport`, a bug or breaking change in one shared helper can fail tests
  across every consumer simultaneously, and every consumer's test suite is coupled to this
  package's release cadence.

Explicitly ruled out:
- `internal/testsupport` will never depend on `internal/config`, `internal/project`, or any other
  awf-internal package, even for a helper that would clearly benefit from it (e.g. a hypothetical
  future helper that builds a `config.Config` directly rather than a raw YAML string). Such a
  helper belongs in the consuming package's own test files, not here.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Add a `testdata/` fixture-file convention instead of Go helper functions | Most of the duplicated logic here is *behavior* (spin up a temp git repo, isolate `HOME`, seed a project tree), not static fixture *data* a file could hold; a `testdata/` convention doesn't address the git-fixture, seam-swap, or `TestMain` duplication at all. |
| Let each package keep its own local test helpers, only extracting within a single package (e.g. `check_test.go`'s 3 inline copies) | Leaves the cross-package duplication (git fixtures, project-fixture setup, ADR frontmatter, the seam-swap idiom) unaddressed — the review's findings show the highest-value duplication is precisely the cross-package kind a single-package extraction can't reach. |
| Let `internal/testsupport.ADR()` import `internal/adr` for type-safe frontmatter construction | Would create a package that depends on production code paths from a test-only package, breaking the "safely importable by anyone, including `internal/config`" property that motivates this ADR, and risks a future import cycle if `internal/adr`'s own tests ever need `internal/testsupport`. Rejected in favour of the raw-string-template approach (see Consequences). |
