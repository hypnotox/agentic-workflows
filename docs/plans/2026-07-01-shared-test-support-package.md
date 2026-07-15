# Shared Test-Support Package

Implements [ADR-0044](../decisions/0044-shared-test-support-package.md). Design rationale lives
there; this plan is the execution record only.

## Goal

Consolidate the hand-rolled fixture idioms duplicated across awf's test suites (git-repo fixtures,
`TestMain` HOME isolation, project-fixture (`.awf/config.yaml`) setup, ADR-frontmatter fixtures,
file-writing boilerplate, and the seam-swap idiom) into one new leaf package,
`internal/testsupport` (plus its `gitfixture` subpackage), migrate every duplication site named in
ADR-0044's Context to call the shared helpers instead of its own local copy, and back the package's
zero-`internal/*`-dependency constraint with a mechanical test.

## Architecture summary

- `internal/testsupport` is a new leaf package: only the Go standard library may be imported by its
  own (non-`gitfixture`) files. It exports `WriteFile`, `WriteAwfConfig`, `SwapVar[T any]`,
  `WriteGoModule`, `WriteProfile`, `ADR` (+ `ADROption`s `WithTitle`/`WithDate`/`WithTags`/
  `WithDomains`/`WithRetiresInvariants`/`WithBody`), and `RunIsolated`.
- `internal/testsupport/gitfixture` is a subpackage isolating the `go-git` dependency: `Sig`,
  `InitRepo`, `Commit`. Kept separate so a caller that only needs e.g. `testsupport.WriteFile` does
  not pull `go-git` into its test binary.
- `internal/testsupport/deps_test.go` backs `inv: testsupport-zero-internal-deps` by parsing every
  non-test `.go` file under `internal/testsupport/` (including `gitfixture/`) with `go/parser` and
  failing on any `github.com/hypnotox/agentic-workflows/internal/*` import (or, outside
  `gitfixture/`, any `github.com/go-git/*` import).
- Because Go's coverage instrumentation only ever covers non-`_test.go` files (verified empirically:
  a `go test -coverprofile` run never emits a `_test.go` line into the profile), `testsupport.go` and
  `gitfixture.go` are themselves subject to the repo's 100%-coverage gate the moment they exist,
  independent of when their call sites migrate. Phase 1 therefore ships each with its own direct
  `_test.go` file exercising every branch that is not marked `// coverage-ignore:` (defensive
  branches (a failed `os.MkdirTemp`, a failed `git.PlainInit` into a fresh temp dir, etc.) are
  ignored, mirroring `internal/adr/adr.go`'s existing convention), so `./x gate` stays green from the
  package's very first commit, before any call site is touched. `RunIsolated` is the one exception:
  it wraps `(*testing.M).Run`, which cannot be constructed standalone outside the real `go test`
  entry point, so its introduction is folded into the same commit that migrates the first two real
  `TestMain` call sites, the only commit in this plan where "add code" and "add its first caller"
  land together.
- Every other consolidation target keeps its own package-local convenience wrapper where one already
  existed and added value beyond a raw call (`scaffoldFiles`, `scaffoldProject`, `scaffoldedProject`,
  `auditProject`, `gateFixture`, `internal/coverage`'s `module`/`writeProfile`,
  `cmd/covercheck`'s `modWith`, `internal/invariants`'s `writeADR`/`writeRetiringADR`,
  `internal/project`'s `writeADR`); only their *bodies* change, from hand-rolled `os.MkdirAll`/
  `os.WriteFile` to a call into `internal/testsupport`. The idioms ADR-0044 identifies as pure
  boilerplate with no added value (`mustWrite`/`mustMkdir` in `internal/migrate`, `writeFileAt` in
  `internal/project`, `swapGetwd`/`swapHasGoMod` in `cmd/awf`/`internal/coverage`, `testSig`/
  `initRepo`/`commit`/`auditSig`/`auditCommit` in `internal/audit`/`cmd/awf`) are deleted outright,
  with every call site switched to call `internal/testsupport`/`gitfixture` directly.
- `ADR()` builds a raw `---`-delimited string template (not by importing `internal/adr` and
  marshaling its frontmatter struct; seeADR-0044's Consequences) with a fixed field order
  (`status`, `date`, `tags`, `domains`, `retires_invariants`) matching every migrated fixture's
  existing byte layout, so most migrations are exact-byte-identical; the few that are not (a
  `tags: []` literal collapsing to an omitted field, or a bare `body` payload gaining a `# ADR-0001:
  T` heading it never had) are semantically inert; the consuming test only ever parses frontmatter
  or asserts on unrelated content, never the exact bytes changed.

## Tech stack

Go 1.26 (module `github.com/hypnotox/agentic-workflows`). New package `internal/testsupport` (leaf,
stdlib-only) and subpackage `internal/testsupport/gitfixture` (adds `github.com/go-git/go-git/v5`,
already a direct `go.mod` dependency; no `go.mod`/`go.sum` change). Every other change is to
existing `_test.go` files; no production (non-test) behavior changes anywhere outside
`internal/testsupport` itself.

## File structure

- Created: `internal/testsupport/testsupport.go`, `internal/testsupport/testsupport_test.go`,
  `internal/testsupport/deps_test.go`, `internal/testsupport/gitfixture/gitfixture.go`,
  `internal/testsupport/gitfixture/gitfixture_test.go`
- Modified: `cmd/awf/testmain_test.go`, `internal/audit/testmain_test.go`
- Modified: `internal/coverage/coverage_test.go`
- Modified: `cmd/awf/run_test.go`, `cmd/awf/audit_test.go`, `cmd/awf/list_add_test.go`,
  `cmd/awf/init_test.go`, `cmd/awf/uninstall_test.go`, `cmd/awf/commitgate_test.go`,
  `cmd/awf/new_test.go`, `cmd/awf/check_test.go`, `cmd/awf/gate_test.go`,
  `cmd/awf/invariants_test.go`
- Modified: `internal/project/project_test.go`, `internal/project/coverage_test.go`,
  `internal/project/domains_test.go`, `internal/project/render_tree_test.go`,
  `internal/project/drift_test.go`
- Modified: `internal/adr/adr_test.go`, `internal/invariants/invariants_test.go`,
  `internal/audit/audit_test.go`, `internal/audit/git_test.go`
- Modified: `internal/migrate/migrate_test.go`, `internal/migrate/singletonstandarddocs_test.go`
- Modified: `cmd/covercheck/main_test.go`
- Modified: `.awf/domains/parts/tooling/current-state.md`
- Created: `.awf/docs/parts/testing/layout.md`
- Modified: `docs/decisions/0044-shared-test-support-package.md` (frontmatter `status` only)
- Modified (rendered, via `./x sync`): `docs/domains/tooling.md`, `docs/testing.md`,
  `docs/decisions/ACTIVE.md`, `.awf/awf.lock`

## Phase 1: Build `internal/testsupport` + `gitfixture`, migrate `TestMain` sites

- [ ] **Task 1.1: Create `internal/testsupport/testsupport.go` and its direct unit tests.** Create
  `internal/testsupport/testsupport.go`:

  ```go
  // Package testsupport provides shared test-fixture helpers used across awf's
  // test suites: project-config scaffolding, ADR frontmatter fixtures,
  // file-writing primitives, and the seam-swap idiom. It is a leaf package:
  // only the Go standard library may be imported here (see the gitfixture
  // subpackage for the go-git-dependent helpers), so it is safe to import from
  // any package's tests without risking an import cycle (ADR-0044). deps_test.go
  // enforces the zero-internal-deps constraint mechanically.
  package testsupport

  import (
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  )

  // WriteFile creates path's parent directories and writes content to it,
  // failing the test on either error. The primitive every other file-writing
  // helper in this package is built from.
  func WriteFile(t *testing.T, path, content string) {
  	t.Helper()
  	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { // coverage-ignore: MkdirAll under a fresh t.TempDir() fails only on a permission fault a test cannot trigger
  		t.Fatal(err)
  	}
  	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { // coverage-ignore: WriteFile into a dir just created above fails only on a permission fault a test cannot trigger
  		t.Fatal(err)
  	}
  }

  // WriteAwfConfig writes <root>/.awf/config.yaml with the given content, the
  // project-fixture seed step every scaffold-style helper across awf's test
  // suites starts from.
  func WriteAwfConfig(t *testing.T, root, yamlContent string) {
  	t.Helper()
  	WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), yamlContent)
  }

  // SwapVar overrides *seam with val for the duration of the test, restoring
  // the original value via t.Cleanup. Covers the swapGetwd/swapHasGoMod/
  // forceNonInteractive-shaped idiom: a *T package-private seam variable
  // reassigned for one test, in the same package as the test. Does not cover
  // internal/adr's swapNow, an external-test-package accessor of a different
  // shape (ADR-0044 Context).
  func SwapVar[T any](t *testing.T, seam *T, val T) {
  	t.Helper()
  	orig := *seam
  	*seam = val
  	t.Cleanup(func() { *seam = orig })
  }

  // WriteGoModule writes a minimal go.mod (pinned to this repo's Go toolchain
  // version) and an f.go containing srcBody under dir.
  func WriteGoModule(t *testing.T, dir, modPath, srcBody string) {
  	t.Helper()
  	WriteFile(t, filepath.Join(dir, "go.mod"), "module "+modPath+"\n\ngo 1.26\n")
  	WriteFile(t, filepath.Join(dir, "f.go"), srcBody)
  }

  // WriteProfile writes dir/cover.out with a "mode: set\n" prefix followed by
  // body, returning the file's path.
  func WriteProfile(t *testing.T, dir, body string) string {
  	t.Helper()
  	p := filepath.Join(dir, "cover.out")
  	WriteFile(t, p, "mode: set\n"+body)
  	return p
  }

  // ADROption configures an ADR fixture built by ADR.
  type ADROption func(*adrOpts)

  type adrOpts struct {
  	title             string
  	date              string
  	tags              []string
  	domains           []string
  	retiresInvariants []string
  	body              string
  }

  // WithTitle sets the ADR's number+title heading text, the part after
  // "# ADR-", e.g. "0001: My Title". Defaults to "0001: T".
  func WithTitle(title string) ADROption { return func(o *adrOpts) { o.title = title } }

  // WithDate sets the frontmatter date field. Omitted from the frontmatter
  // entirely when never called; some fixtures deliberately carry no date.
  func WithDate(date string) ADROption { return func(o *adrOpts) { o.date = date } }

  // WithTags sets the frontmatter tags array.
  func WithTags(tags ...string) ADROption { return func(o *adrOpts) { o.tags = tags } }

  // WithDomains sets the frontmatter domains array.
  func WithDomains(domains ...string) ADROption { return func(o *adrOpts) { o.domains = domains } }

  // WithRetiresInvariants sets the frontmatter retires_invariants array.
  func WithRetiresInvariants(slugs ...string) ADROption {
  	return func(o *adrOpts) { o.retiresInvariants = slugs }
  }

  // WithBody appends raw markdown (e.g. "## Context\nx\n") after the title
  // heading.
  func WithBody(body string) ADROption { return func(o *adrOpts) { o.body = body } }

  // ADR builds a ---delimited ADR frontmatter fixture as a raw string: a status
  // field plus any of date/tags/domains/retires_invariants supplied via opts, a
  // "# ADR-<title>" heading, and an optional trailing body. It intentionally
  // does not import internal/adr and marshal its real frontmatter struct;
  // doing so would break this package's zero-internal-deps invariant (see
  // ADR-0044's Consequences).
  func ADR(status string, opts ...ADROption) string {
  	o := adrOpts{title: "0001: T"}
  	for _, opt := range opts {
  		opt(&o)
  	}
  	var b strings.Builder
  	b.WriteString("---\nstatus: " + status + "\n")
  	if o.date != "" {
  		b.WriteString("date: " + o.date + "\n")
  	}
  	if o.tags != nil {
  		b.WriteString("tags: [" + strings.Join(o.tags, ", ") + "]\n")
  	}
  	if o.domains != nil {
  		b.WriteString("domains: [" + strings.Join(o.domains, ", ") + "]\n")
  	}
  	if o.retiresInvariants != nil {
  		b.WriteString("retires_invariants: [" + strings.Join(o.retiresInvariants, ", ") + "]\n")
  	}
  	b.WriteString("---\n# ADR-" + o.title + "\n")
  	b.WriteString(o.body)
  	return b.String()
  }
  ```

  Create `internal/testsupport/testsupport_test.go`:

  ```go
  package testsupport_test

  import (
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )

  func TestWriteFileCreatesParentDirs(t *testing.T) {
  	dir := t.TempDir()
  	path := filepath.Join(dir, "a", "b", "c.txt")
  	testsupport.WriteFile(t, path, "hello\n")
  	got, err := os.ReadFile(path)
  	if err != nil {
  		t.Fatal(err)
  	}
  	if string(got) != "hello\n" {
  		t.Errorf("WriteFile content = %q, want %q", got, "hello\n")
  	}
  }

  func TestWriteAwfConfig(t *testing.T) {
  	root := t.TempDir()
  	testsupport.WriteAwfConfig(t, root, "prefix: example\n")
  	got, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
  	if err != nil {
  		t.Fatal(err)
  	}
  	if string(got) != "prefix: example\n" {
  		t.Errorf("WriteAwfConfig content = %q", got)
  	}
  }

  func TestSwapVar(t *testing.T) {
  	seam := 1
  	t.Run("swap", func(t *testing.T) {
  		testsupport.SwapVar(t, &seam, 2)
  		if seam != 2 {
  			t.Fatalf("seam = %d, want 2", seam)
  		}
  	})
  	if seam != 1 {
  		t.Errorf("seam not restored after subtest, got %d, want 1", seam)
  	}
  }

  func TestWriteGoModule(t *testing.T) {
  	dir := t.TempDir()
  	testsupport.WriteGoModule(t, dir, "example.com/m", "package m\nfunc F() {}\n")
  	mod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
  	if err != nil {
  		t.Fatal(err)
  	}
  	if !strings.Contains(string(mod), "module example.com/m") || !strings.Contains(string(mod), "go 1.26") {
  		t.Errorf("go.mod = %q", mod)
  	}
  	src, err := os.ReadFile(filepath.Join(dir, "f.go"))
  	if err != nil {
  		t.Fatal(err)
  	}
  	if string(src) != "package m\nfunc F() {}\n" {
  		t.Errorf("f.go = %q", src)
  	}
  }

  func TestWriteProfile(t *testing.T) {
  	dir := t.TempDir()
  	path := testsupport.WriteProfile(t, dir, "example.com/m/f.go:2.1,2.5 1 1\n")
  	if path != filepath.Join(dir, "cover.out") {
  		t.Errorf("WriteProfile path = %q", path)
  	}
  	got, err := os.ReadFile(path)
  	if err != nil {
  		t.Fatal(err)
  	}
  	if !strings.HasPrefix(string(got), "mode: set\n") {
  		t.Errorf("profile missing mode header: %q", got)
  	}
  }

  func TestADRMinimal(t *testing.T) {
  	got := testsupport.ADR("Proposed")
  	want := "---\nstatus: Proposed\n---\n# ADR-0001: T\n"
  	if got != want {
  		t.Errorf("ADR(minimal) = %q, want %q", got, want)
  	}
  }

  func TestADREveryOption(t *testing.T) {
  	got := testsupport.ADR("Implemented",
  		testsupport.WithTitle("0002: Full"),
  		testsupport.WithDate("2026-06-25"),
  		testsupport.WithTags("x", "y"),
  		testsupport.WithDomains("tooling"),
  		testsupport.WithRetiresInvariants("old-slug"),
  		testsupport.WithBody("## Context\nbody\n"),
  	)
  	want := "---\nstatus: Implemented\ndate: 2026-06-25\ntags: [x, y]\ndomains: [tooling]\nretires_invariants: [old-slug]\n---\n# ADR-0002: Full\n## Context\nbody\n"
  	if got != want {
  		t.Errorf("ADR(full) =\n%q\nwant\n%q", got, want)
  	}
  }
  ```

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Stage `internal/testsupport/testsupport.go internal/testsupport/testsupport_test.go`. Commit:
    `test(awf): add internal/testsupport with core fixture helpers`

- [ ] **Task 1.2: Create `internal/testsupport/gitfixture` and its direct unit tests.** Create
  `internal/testsupport/gitfixture/gitfixture.go`:

  ```go
  // Package gitfixture provides go-git-backed test fixtures (a fixed commit
  // signature, a fresh repo, and a write+commit helper) for awf's test suites
  // that need a real git repository. It is kept separate from
  // internal/testsupport so a caller that only needs e.g.
  // testsupport.WriteFile does not have to pull go-git into its test binary
  // (ADR-0044).
  package gitfixture

  import (
  	"os"
  	"path/filepath"
  	"testing"
  	"time"

  	"github.com/go-git/go-git/v5"
  	"github.com/go-git/go-git/v5/plumbing"
  	"github.com/go-git/go-git/v5/plumbing/object"
  )

  // Sig is the fixed commit signature used by InitRepo/Commit fixtures across
  // awf's test suites.
  var Sig = &object.Signature{Name: "T", Email: "t@example.com", When: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}

  // InitRepo creates a fresh git repository in a new t.TempDir(), returning the
  // repository and its root path.
  func InitRepo(t *testing.T) (*git.Repository, string) {
  	t.Helper()
  	dir := t.TempDir()
  	repo, err := git.PlainInit(dir, false)
  	if err != nil { // coverage-ignore: PlainInit into a fresh empty t.TempDir() fails only on a permission fault a test cannot trigger
  		t.Fatalf("init: %v", err)
  	}
  	return repo, dir
  }

  // Commit writes/removes the given paths in repo's worktree (rooted at dir),
  // stages them, and commits with Sig, returning the commit hash.
  func Commit(t *testing.T, repo *git.Repository, dir, msg string, write map[string]string, remove ...string) plumbing.Hash {
  	t.Helper()
  	wt, err := repo.Worktree()
  	if err != nil { // coverage-ignore: Worktree() on a just-initialized non-bare repo cannot fail
  		t.Fatalf("worktree: %v", err)
  	}
  	for name, content := range write {
  		if werr := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); werr != nil { // coverage-ignore: writing into the repo's own worktree dir fails only on a permission fault a test cannot trigger
  			t.Fatalf("write %s: %v", name, werr)
  		}
  		if _, aerr := wt.Add(name); aerr != nil { // coverage-ignore: Add of a path just written above cannot fail
  			t.Fatalf("add %s: %v", name, aerr)
  		}
  	}
  	for _, name := range remove {
  		if _, rerr := wt.Remove(name); rerr != nil { // coverage-ignore: Remove of a path the caller asserts is tracked cannot fail in a test fixture
  			t.Fatalf("remove %s: %v", name, rerr)
  		}
  	}
  	h, err := wt.Commit(msg, &git.CommitOptions{Author: Sig, Committer: Sig})
  	if err != nil { // coverage-ignore: Commit with a valid signature and staged tree cannot fail
  		t.Fatalf("commit: %v", err)
  	}
  	return h
  }
  ```

  Create `internal/testsupport/gitfixture/gitfixture_test.go`:

  ```go
  package gitfixture_test

  import (
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
  )

  func TestInitRepoAndCommit(t *testing.T) {
  	repo, dir := gitfixture.InitRepo(t)
  	base := gitfixture.Commit(t, repo, dir, "feat(awf): base", map[string]string{"a.txt": "1\n"})
  	if base.IsZero() {
  		t.Fatal("expected a non-zero base commit hash")
  	}
  	head := gitfixture.Commit(t, repo, dir, "feat(awf): head", map[string]string{"b.txt": "2\n"}, "a.txt")
  	if head == base {
  		t.Fatal("expected head to differ from base")
  	}
  	c, err := repo.CommitObject(head)
  	if err != nil {
  		t.Fatal(err)
  	}
  	if c.Author.Name != gitfixture.Sig.Name || c.Author.Email != gitfixture.Sig.Email {
  		t.Errorf("commit author = %+v, want Sig %+v", c.Author, gitfixture.Sig)
  	}
  }
  ```

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Stage `internal/testsupport/gitfixture/gitfixture.go internal/testsupport/gitfixture/gitfixture_test.go`.
    Commit: `test(awf): add internal/testsupport/gitfixture`

- [ ] **Task 1.3: Add the zero-internal-deps invariant test.** Create
  `internal/testsupport/deps_test.go`:

  ```go
  package testsupport_test

  import (
  	"go/parser"
  	"go/token"
  	"path/filepath"
  	"strconv"
  	"strings"
  	"testing"
  )

  // TestZeroInternalDeps enforces mechanically that internal/testsupport (and
  // its gitfixture subpackage) stays a leaf package: no non-test .go file may
  // import any github.com/hypnotox/agentic-workflows/internal/* package, so
  // this package stays safely importable from any package's tests without
  // risking an import cycle. gitfixture/ is the sole exception permitted to
  // import go-git.
  // invariant: testsupport-zero-internal-deps
  func TestZeroInternalDeps(t *testing.T) {
  	files, err := filepath.Glob("*.go")
  	if err != nil {
  		t.Fatal(err)
  	}
  	sub, err := filepath.Glob(filepath.Join("gitfixture", "*.go"))
  	if err != nil {
  		t.Fatal(err)
  	}
  	files = append(files, sub...)
  	for _, f := range files {
  		if strings.HasSuffix(f, "_test.go") {
  			continue
  		}
  		fset := token.NewFileSet()
  		astFile, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
  		if err != nil {
  			t.Fatalf("parse %s: %v", f, err)
  		}
  		allowGoGit := strings.HasPrefix(f, "gitfixture"+string(filepath.Separator))
  		for _, imp := range astFile.Imports {
  			path, err := strconv.Unquote(imp.Path.Value)
  			if err != nil {
  				t.Fatalf("%s: unquote import %s: %v", f, imp.Path.Value, err)
  			}
  			if strings.HasPrefix(path, "github.com/hypnotox/agentic-workflows/internal/") {
  				t.Errorf("%s imports internal package %q: internal/testsupport must stay a leaf package (ADR-0044)", f, path)
  			}
  			if !allowGoGit && strings.HasPrefix(path, "github.com/go-git/") {
  				t.Errorf("%s imports go-git package %q: only gitfixture/ may depend on go-git", f, path)
  			}
  		}
  	}
  }
  ```

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.` (`deps_test.go` is itself a `_test.go`
    file, so it is exempt from the coverage profile, per this plan's Architecture summary.)
  - Run `./x invariants`. Expect `awf invariants: clean` (ADR-0044 stays `Proposed` until Phase 6, so
    its `inv:` tag is not yet enforced; this just confirms the new backing comment does not itself
    break the scan).
  - Stage `internal/testsupport/deps_test.go`. Commit:
    `test(awf): back the testsupport-zero-internal-deps invariant`

- [ ] **Task 1.4: Add `RunIsolated` and migrate both `TestMain` sites.** In
  `internal/testsupport/testsupport.go`, change the package doc comment from:

  ```go
  // Package testsupport provides shared test-fixture helpers used across awf's
  // test suites: project-config scaffolding, ADR frontmatter fixtures,
  // file-writing primitives, and the seam-swap idiom. It is a leaf package:
  ```

  to:

  ```go
  // Package testsupport provides shared test-fixture helpers used across awf's
  // test suites: TestMain HOME isolation, project-config scaffolding, ADR
  // frontmatter fixtures, file-writing primitives, and the seam-swap idiom. It
  // is a leaf package:
  ```

  Then append at the end of the file:

  ```go

  // RunIsolated gives m a throwaway HOME (os.MkdirTemp(prefix)) for the
  // duration of its run, so go-git's global-gitignore read finds nothing
  // belonging to the developer's real machine, then removes the temp HOME and
  // returns the run's exit code for the caller to pass to os.Exit:
  //
  //	func TestMain(m *testing.M) { os.Exit(testsupport.RunIsolated(m, "awf-test-home")) }
  func RunIsolated(m *testing.M, prefix string) int {
  	home, err := os.MkdirTemp("", prefix)
  	if err != nil { // coverage-ignore: MkdirTemp fails only on an unwritable system temp dir, which a test cannot construct portably
  		panic(err)
  	}
  	if err := os.Setenv("HOME", home); err != nil { // coverage-ignore: Setenv fails only on a malformed name, which the fixed literal "HOME" cannot produce
  		panic(err)
  	}
  	code := m.Run()
  	_ = os.RemoveAll(home)
  	return code
  }
  ```

  In `cmd/awf/testmain_test.go`, change:

  ```go
  package main

  import (
  	"os"
  	"testing"
  )

  // TestMain isolates this package's tests from the host by giving them a throwaway
  // HOME, so go-git's global-gitignore read (the uncommitted-changes audit rule)
  // finds nothing. awf drives git purely through go-git (no host git binary, and
  // no host git config), so the tests build their state in temp repos and never
  // read or write the developer's machine.
  func TestMain(m *testing.M) {
  	home, err := os.MkdirTemp("", "awf-test-home")
  	if err != nil {
  		panic(err)
  	}
  	if err := os.Setenv("HOME", home); err != nil {
  		panic(err)
  	}
  	code := m.Run()
  	_ = os.RemoveAll(home)
  	os.Exit(code)
  }
  ```

  to:

  ```go
  package main

  import (
  	"os"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )

  // TestMain isolates this package's tests from the host by giving them a throwaway
  // HOME, so go-git's global-gitignore read (the uncommitted-changes audit rule)
  // finds nothing. awf drives git purely through go-git (no host git binary, and
  // no host git config), so the tests build their state in temp repos and never
  // read or write the developer's machine.
  func TestMain(m *testing.M) {
  	os.Exit(testsupport.RunIsolated(m, "awf-test-home"))
  }
  ```

  In `internal/audit/testmain_test.go`, change:

  ```go
  package audit

  import (
  	"os"
  	"testing"
  )

  // TestMain isolates this package's tests from the host by giving them a throwaway
  // HOME, so go-git's global-gitignore read finds nothing. The uncommitted-changes
  // rule reads live global ignore patterns by design, so the tests must not inherit
  // the developer's.
  func TestMain(m *testing.M) {
  	home, err := os.MkdirTemp("", "awf-audit-test-home")
  	if err != nil {
  		panic(err)
  	}
  	if err := os.Setenv("HOME", home); err != nil {
  		panic(err)
  	}
  	code := m.Run()
  	_ = os.RemoveAll(home)
  	os.Exit(code)
  }
  ```

  to:

  ```go
  package audit

  import (
  	"os"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )

  // TestMain isolates this package's tests from the host by giving them a throwaway
  // HOME, so go-git's global-gitignore read finds nothing. The uncommitted-changes
  // rule reads live global ignore patterns by design, so the tests must not inherit
  // the developer's.
  func TestMain(m *testing.M) {
  	os.Exit(testsupport.RunIsolated(m, "awf-audit-test-home"))
  }
  ```

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.` (`RunIsolated` is now exercised for
    real by both packages' `TestMain`.)
  - Run `./x check`. Expect `awf check: clean`.
  - Stage `internal/testsupport/testsupport.go cmd/awf/testmain_test.go
    internal/audit/testmain_test.go`. Commit:
    `test(awf): route TestMain HOME isolation via RunIsolated`

## Phase 2: Seam-swap sites

- [ ] **Task 2.1: `internal/coverage`: route `swapGetwd`/`swapHasGoMod` through `SwapVar`.** In
  `internal/coverage/coverage_test.go`, change the import block from:

  ```go
  import (
  	"errors"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  )
  ```

  to:

  ```go
  import (
  	"errors"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  Replace every occurrence of `swapGetwd(t, ` with `testsupport.SwapVar(t, &getwd, ` (4 occurrences:
  the calls inside `TestCheckProfileResolvesModule`, `TestCheckProfileGetwdError`,
  `TestCheckProfileNoModule`, `TestCheckProfileNoModuleLine`), and the 1 occurrence of
  `swapHasGoMod(t, ` with `testsupport.SwapVar(t, &hasGoMod, ` (inside `TestCheckProfileNoModule`).
  Then delete the now-unused local definitions at the end of the file:

  ```go

  // swapGetwd overrides the package getwd seam for the duration of a test.
  func swapGetwd(t *testing.T, fn func() (string, error)) {
  	t.Helper()
  	orig := getwd
  	getwd = fn
  	t.Cleanup(func() { getwd = orig })
  }

  func swapHasGoMod(t *testing.T, fn func(string) bool) {
  	t.Helper()
  	orig := hasGoMod
  	hasGoMod = fn
  	t.Cleanup(func() { hasGoMod = orig })
  }
  ```

  (delete this whole trailing block; nothing replaces it).

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Stage `internal/coverage/coverage_test.go`. Commit:
    `test(awf): route internal/coverage seam-swaps via SwapVar`

- [ ] **Task 2.2: `cmd/awf`: route every `swapGetwd` call site through `SwapVar`.** `swapGetwd` is
  defined once, in `cmd/awf/run_test.go`, and called from 7 files in package `main`:
  `run_test.go`, `audit_test.go`, `list_add_test.go`, `init_test.go`, `uninstall_test.go`,
  `commitgate_test.go`, `new_test.go`: 28 call sites total, every one matching the exact literal
  prefix `swapGetwd(t, ` (verified: `grep -n "swapGetwd(" cmd/awf/*.go | grep -v "func swapGetwd"`
  returns 28 lines, all of the shape `swapGetwd(t, func() ...`). In each of the 7 files, replace
  every occurrence of `swapGetwd(t, ` with `testsupport.SwapVar(t, &getwd, ` (this is a safe blind
  substitution: the prefix is byte-identical at every site and `getwd`'s type,
  `func() (string, error)`, matches every call's closure).

  Then, in `cmd/awf/run_test.go`, delete the now-orphaned definition:

  ```go

  // swapGetwd overrides the package getwd seam for the duration of a test.
  func swapGetwd(t *testing.T, fn func() (string, error)) {
  	t.Helper()
  	orig := getwd
  	getwd = fn
  	t.Cleanup(func() { getwd = orig })
  }
  ```

  (delete this whole block; it sits between `minimalYAML`'s closing backtick and
  `TestRunNoArgs`).

  Finally add the `testsupport` import to each of the 7 files (only `run_test.go`'s block is shown
  in full below; the other 6 follow the same "insert one import line, alphabetically, in the existing
  local-import group or as a new group after the stdlib group" rule):

  `cmd/awf/run_test.go`, change:

  ```go
  import (
  	"bytes"
  	"errors"
  	"io"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  )
  ```

  to:

  ```go
  import (
  	"bytes"
  	"errors"
  	"io"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  `cmd/awf/audit_test.go`, change:

  ```go
  import (
  	"bytes"
  	"io"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  	"time"

  	"github.com/go-git/go-git/v5"
  	"github.com/go-git/go-git/v5/plumbing"
  	"github.com/go-git/go-git/v5/plumbing/object"
  )
  ```

  to:

  ```go
  import (
  	"bytes"
  	"io"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  	"time"

  	"github.com/go-git/go-git/v5"
  	"github.com/go-git/go-git/v5/plumbing"
  	"github.com/go-git/go-git/v5/plumbing/object"
  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  `cmd/awf/list_add_test.go`, change:

  ```go
  import (
  	"bytes"
  	"io"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/project"
  )
  ```

  to:

  ```go
  import (
  	"bytes"
  	"io"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/project"
  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  `cmd/awf/init_test.go`, change:

  ```go
  import (
  	"bytes"
  	"encoding/json"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  )
  ```

  to:

  ```go
  import (
  	"bytes"
  	"encoding/json"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  `cmd/awf/uninstall_test.go`, change:

  ```go
  import (
  	"bytes"
  	"io"
  	"os"
  	"path/filepath"
  	"testing"
  )
  ```

  to:

  ```go
  import (
  	"bytes"
  	"io"
  	"os"
  	"path/filepath"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  `cmd/awf/commitgate_test.go`, change:

  ```go
  import (
  	"bytes"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  )
  ```

  to:

  ```go
  import (
  	"bytes"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  `cmd/awf/new_test.go`, change:

  ```go
  import (
  	"bytes"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  )
  ```

  to:

  ```go
  import (
  	"bytes"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Run `./x check`. Expect `awf check: clean`.
  - Stage `cmd/awf/run_test.go cmd/awf/audit_test.go cmd/awf/list_add_test.go cmd/awf/init_test.go
    cmd/awf/uninstall_test.go cmd/awf/commitgate_test.go cmd/awf/new_test.go`. Commit:
    `test(awf): route cmd/awf's swapGetwd sites via testsupport.SwapVar`

- [ ] **Task 2.3: `cmd/awf/init_test.go`: route `forceNonInteractive` through `SwapVar`.**
  `forceNonInteractive` is called with zero arguments at 5 sites (`isInteractive` is hardcoded to
  `false`), so it stays as a named convenience wrapper; only its body changes. Change:

  ```go
  // forceNonInteractive pins the isInteractive seam to false for the test, so the
  // silent resolution path runs deterministically regardless of the real stdin.
  func forceNonInteractive(t *testing.T) {
  	t.Helper()
  	orig := isInteractive
  	isInteractive = func() bool { return false }
  	t.Cleanup(func() { isInteractive = orig })
  }
  ```

  to:

  ```go
  // forceNonInteractive pins the isInteractive seam to false for the test, so the
  // silent resolution path runs deterministically regardless of the real stdin.
  func forceNonInteractive(t *testing.T) {
  	t.Helper()
  	testsupport.SwapVar(t, &isInteractive, func() bool { return false })
  }
  ```

  (the `testsupport` import already exists from Task 2.2's `swapGetwd` migration in this same file.)

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Stage `cmd/awf/init_test.go`. Commit:
    `test(awf): route cmd/awf's forceNonInteractive via SwapVar`

## Phase 3: Project-fixture setup sites

- [ ] **Task 3.1: `internal/project/project_test.go`: `scaffoldFiles`.** Change the import block
  from:

  ```go
  import (
  	"os"
  	"path/filepath"
  	"reflect"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/config"
  	"github.com/hypnotox/agentic-workflows/internal/manifest"
  )
  ```

  to:

  ```go
  import (
  	"os"
  	"path/filepath"
  	"reflect"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/config"
  	"github.com/hypnotox/agentic-workflows/internal/manifest"
  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  Change:

  ```go
  // scaffoldFiles writes config.yaml plus optional sidecar/part files keyed by path
  // relative to .awf/ (e.g. "skills/tdd.yaml", "skills/parts/x/y.md").
  func scaffoldFiles(t *testing.T, configYAML string, files map[string]string) string {
  	t.Helper()
  	root := t.TempDir()
  	awf := filepath.Join(root, ".awf")
  	if err := os.MkdirAll(awf, 0o755); err != nil {
  		t.Fatal(err)
  	}
  	if err := os.WriteFile(filepath.Join(awf, "config.yaml"), []byte(configYAML), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	for rel, body := range files {
  		p := filepath.Join(awf, rel)
  		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
  			t.Fatal(err)
  		}
  		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
  			t.Fatal(err)
  		}
  	}
  	return root
  }
  ```

  to:

  ```go
  // scaffoldFiles writes config.yaml plus optional sidecar/part files keyed by path
  // relative to .awf/ (e.g. "skills/tdd.yaml", "skills/parts/x/y.md").
  func scaffoldFiles(t *testing.T, configYAML string, files map[string]string) string {
  	t.Helper()
  	root := t.TempDir()
  	testsupport.WriteAwfConfig(t, root, configYAML)
  	for rel, body := range files {
  		testsupport.WriteFile(t, filepath.Join(root, ".awf", rel), body)
  	}
  	return root
  }
  ```

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Stage `internal/project/project_test.go`. Commit:
    `test(awf): route internal/project's scaffoldFiles via WriteAwfConfig`

- [ ] **Task 3.2: `cmd/awf/run_test.go`: `scaffoldProject`.** Change:

  ```go
  // scaffoldProject writes a minimal tree config under root and syncs it, leaving a
  // drift-clean project. It returns root.
  func scaffoldProject(t *testing.T) string {
  	t.Helper()
  	root := t.TempDir()
  	awf := filepath.Join(root, ".awf")
  	if err := os.MkdirAll(awf, 0o755); err != nil {
  		t.Fatal(err)
  	}
  	if err := os.WriteFile(filepath.Join(awf, "config.yaml"), []byte(minimalYAML), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	if err := runSync(root, io.Discard); err != nil {
  		t.Fatalf("scaffold sync: %v", err)
  	}
  	return root
  }
  ```

  to:

  ```go
  // scaffoldProject writes a minimal tree config under root and syncs it, leaving a
  // drift-clean project. It returns root.
  func scaffoldProject(t *testing.T) string {
  	t.Helper()
  	root := t.TempDir()
  	testsupport.WriteAwfConfig(t, root, minimalYAML)
  	if err := runSync(root, io.Discard); err != nil {
  		t.Fatalf("scaffold sync: %v", err)
  	}
  	return root
  }
  ```

  (the `testsupport` import already exists from Task 2.2.)

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Stage `cmd/awf/run_test.go`. Commit:
    `test(awf): route cmd/awf's scaffoldProject via WriteAwfConfig`

- [ ] **Task 3.3: `cmd/awf/list_add_test.go`: `scaffoldedProject`.** Change:

  ```go
  // scaffoldedProject writes a curated-default scaffold (10 core skills, 3 agents,
  // 0 docs (no doc is core after ADR-0043), no domains) and syncs it.
  func scaffoldedProject(t *testing.T) string {
  	t.Helper()
  	root := t.TempDir()
  	awf := filepath.Join(root, ".awf")
  	if err := os.MkdirAll(awf, 0o755); err != nil {
  		t.Fatal(err)
  	}
  	b, err := project.ScaffoldConfig("example", nil, nil, nil)
  	if err != nil {
  		t.Fatal(err)
  	}
  	if err := os.WriteFile(filepath.Join(awf, "config.yaml"), b, 0o644); err != nil {
  		t.Fatal(err)
  	}
  	if err := runSync(root, io.Discard); err != nil {
  		t.Fatalf("sync: %v", err)
  	}
  	return root
  }
  ```

  to:

  ```go
  // scaffoldedProject writes a curated-default scaffold (10 core skills, 3 agents,
  // 0 docs (no doc is core after ADR-0043), no domains) and syncs it.
  func scaffoldedProject(t *testing.T) string {
  	t.Helper()
  	root := t.TempDir()
  	b, err := project.ScaffoldConfig("example", nil, nil, nil)
  	if err != nil {
  		t.Fatal(err)
  	}
  	testsupport.WriteAwfConfig(t, root, string(b))
  	if err := runSync(root, io.Discard); err != nil {
  		t.Fatalf("sync: %v", err)
  	}
  	return root
  }
  ```

  (the `testsupport` import already exists from Task 2.2.)

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Stage `cmd/awf/list_add_test.go`. Commit:
    `test(awf): route cmd/awf's scaffoldedProject via WriteAwfConfig`

- [ ] **Task 3.4: `cmd/awf/audit_test.go`: `auditProject`.** Change:

  ```go
  // auditProject creates a temp project (minimal .awf config) with a git repo and
  // a base commit, returning the root and the base commit hash.
  func auditProject(t *testing.T) (string, plumbing.Hash) {
  	t.Helper()
  	root := t.TempDir()
  	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
  		t.Fatal(err)
  	}
  	cfg := "prefix: example\nskills: []\nagents: []\n"
  	if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(cfg), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	// Sync writes the lock so Project.Audit's generated-path set is populated.
  	if err := runSync(root, io.Discard); err != nil {
  		t.Fatal(err)
  	}
  	repo, err := git.PlainInit(root, false)
  ```

  to:

  ```go
  // auditProject creates a temp project (minimal .awf config) with a git repo and
  // a base commit, returning the root and the base commit hash.
  func auditProject(t *testing.T) (string, plumbing.Hash) {
  	t.Helper()
  	root := t.TempDir()
  	testsupport.WriteAwfConfig(t, root, "prefix: example\nskills: []\nagents: []\n")
  	// Sync writes the lock so Project.Audit's generated-path set is populated.
  	if err := runSync(root, io.Discard); err != nil {
  		t.Fatal(err)
  	}
  	repo, err := git.PlainInit(root, false)
  ```

  (leave the rest of `auditProject` (the `go.mod`/`main.go` writes and the whole-tree commit)
  unchanged; the `testsupport` import already exists from Task 2.2.)

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Stage `cmd/awf/audit_test.go`. Commit:
    `test(awf): route cmd/awf's auditProject via WriteAwfConfig`

- [ ] **Task 3.5: `cmd/awf/check_test.go`: 3 inline setup sequences.** Change the import block
  from:

  ```go
  import (
  	"bytes"
  	"io"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/manifest"
  	"github.com/hypnotox/agentic-workflows/internal/project"
  )
  ```

  to:

  ```go
  import (
  	"bytes"
  	"io"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/manifest"
  	"github.com/hypnotox/agentic-workflows/internal/project"
  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  In `TestRunCheckCleanThenDirty`, change:

  ```go
  	root := t.TempDir()
  	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
  		t.Fatal(err)
  	}
  	if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(checkYAML), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	if err := runSync(root, io.Discard); err != nil {
  		t.Fatal(err)
  	}
  	if err := runCheck(root, io.Discard); err != nil {
  		t.Errorf("expected clean check, got %v", err)
  	}
  ```

  to:

  ```go
  	root := t.TempDir()
  	testsupport.WriteAwfConfig(t, root, checkYAML)
  	if err := runSync(root, io.Discard); err != nil {
  		t.Fatal(err)
  	}
  	if err := runCheck(root, io.Discard); err != nil {
  		t.Errorf("expected clean check, got %v", err)
  	}
  ```

  In `TestRunCheckAheadNotice`'s inner `setup` closure, change:

  ```go
  	setup := func(t *testing.T) string {
  		root := t.TempDir()
  		if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
  			t.Fatal(err)
  		}
  		if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(checkYAML), 0o644); err != nil {
  			t.Fatal(err)
  		}
  		if err := runSync(root, io.Discard); err != nil {
  			t.Fatal(err)
  		}
  		return root
  	}
  ```

  to:

  ```go
  	setup := func(t *testing.T) string {
  		root := t.TempDir()
  		testsupport.WriteAwfConfig(t, root, checkYAML)
  		if err := runSync(root, io.Discard); err != nil {
  			t.Fatal(err)
  		}
  		return root
  	}
  ```

  In `TestRunCheckSurfacesInvariantError`, change:

  ```go
  	root := t.TempDir()
  	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
  		t.Fatal(err)
  	}
  	if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(checkYAML), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	if err := runSync(root, io.Discard); err != nil {
  		t.Fatal(err)
  	}
  	dec := filepath.Join(root, "docs", "decisions")
  ```

  to:

  ```go
  	root := t.TempDir()
  	testsupport.WriteAwfConfig(t, root, checkYAML)
  	if err := runSync(root, io.Discard); err != nil {
  		t.Fatal(err)
  	}
  	dec := filepath.Join(root, "docs", "decisions")
  ```

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Stage `cmd/awf/check_test.go`. Commit:
    `test(awf): route check_test.go's project fixtures via WriteAwfConfig`

- [ ] **Task 3.6: `cmd/awf/gate_test.go`: `gateFixture`.** Change the import block from:

  ```go
  import (
  	"bytes"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/manifest"
  	"github.com/hypnotox/agentic-workflows/internal/migrate"
  	"github.com/hypnotox/agentic-workflows/internal/project"
  )
  ```

  to:

  ```go
  import (
  	"bytes"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/manifest"
  	"github.com/hypnotox/agentic-workflows/internal/migrate"
  	"github.com/hypnotox/agentic-workflows/internal/project"
  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  (`os` is dropped: `gateFixture` is the file's only `os.*` caller, verified via
  `grep -n "os\." cmd/awf/gate_test.go`.) Change:

  ```go
  // gateFixture writes a .awf/ tree with a minimal config.yaml and a hand-written
  // awf.lock carrying the given awfVersion and schemaVersion, returning the root.
  // A negative schema means "write no lock at all".
  func gateFixture(t *testing.T, awfVersion string, schema int) string {
  	t.Helper()
  	root := t.TempDir()
  	awf := filepath.Join(root, ".awf")
  	if err := os.MkdirAll(awf, 0o755); err != nil {
  		t.Fatal(err)
  	}
  	if err := os.WriteFile(filepath.Join(awf, "config.yaml"), []byte("prefix: ex\n"), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	if schema >= 0 {
  		l := &manifest.Lock{AWFVersion: awfVersion, SchemaVersion: schema, Files: map[string]manifest.Entry{}}
  		if err := l.Save(filepath.Join(awf, "awf.lock")); err != nil {
  			t.Fatal(err)
  		}
  	}
  	return root
  }
  ```

  to:

  ```go
  // gateFixture writes a .awf/ tree with a minimal config.yaml and a hand-written
  // awf.lock carrying the given awfVersion and schemaVersion, returning the root.
  // A negative schema means "write no lock at all".
  func gateFixture(t *testing.T, awfVersion string, schema int) string {
  	t.Helper()
  	root := t.TempDir()
  	testsupport.WriteAwfConfig(t, root, "prefix: ex\n")
  	if schema >= 0 {
  		l := &manifest.Lock{AWFVersion: awfVersion, SchemaVersion: schema, Files: map[string]manifest.Entry{}}
  		if err := l.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
  			t.Fatal(err)
  		}
  	}
  	return root
  }
  ```

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Stage `cmd/awf/gate_test.go`. Commit:
    `test(awf): route cmd/awf's gateFixture via WriteAwfConfig`

- [ ] **Task 3.7: `cmd/awf/invariants_test.go`: inline setup.** Change the import block from:

  ```go
  import (
  	"io"
  	"os"
  	"path/filepath"
  	"testing"
  )
  ```

  to:

  ```go
  import (
  	"io"
  	"os"
  	"path/filepath"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  Change:

  ```go
  	root := t.TempDir()
  	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
  		t.Fatal(err)
  	}
  	yaml := "prefix: example\ninvariants:\n  sources:\n    - globs: [\"*.go\"]\n      marker: \"//\"\nskills: []\nagents: []\n"
  	if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(yaml), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	if err := runSync(root, io.Discard); err != nil {
  		t.Fatalf("runSync: %v", err)
  	}
  ```

  to:

  ```go
  	root := t.TempDir()
  	yaml := "prefix: example\ninvariants:\n  sources:\n    - globs: [\"*.go\"]\n      marker: \"//\"\nskills: []\nagents: []\n"
  	testsupport.WriteAwfConfig(t, root, yaml)
  	if err := runSync(root, io.Discard); err != nil {
  		t.Fatalf("runSync: %v", err)
  	}
  ```

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Run `./x check`. Expect `awf check: clean`.
  - Stage `cmd/awf/invariants_test.go`. Commit:
    `test(awf): route invariants_test.go's fixture via WriteAwfConfig`

## Phase 4: ADR-frontmatter fixture sites

Every migration below keeps the migrated fixture either byte-identical to the original (the common
case: `testsupport.ADR`'s field order is `status`, `date`, `tags`, `domains`,
`retires_invariants`, matching every site's existing literal order) or semantically identical where
byte-identity is impossible: an explicit `tags: []` collapses to an omitted `tags` field (Go's
zero-argument variadic call produces `nil`, indistinguishable from never calling `WithTags`), and a
fixture with no heading at all gains a `# ADR-0001: T` default heading. Neither case is observed by
its consuming test (verified per site below).

- [ ] **Task 4.1: `internal/adr/adr_test.go`.** Change the import block from:

  ```go
  import (
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  	"time"

  	"github.com/hypnotox/agentic-workflows/internal/adr"
  )
  ```

  to:

  ```go
  import (
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  	"time"

  	"github.com/hypnotox/agentic-workflows/internal/adr"
  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  In `TestRenderActiveMDGroupsByStatus`, change:

  ```go
  	files := map[string]string{
  		"0001-first-accepted.md": `---
  status: Accepted
  date: 2026-06-24
  tags: [tooling]
  ---
  # ADR-0001: First Accepted
  ## Context
  Something.
  `,
  		"0002-a-proposal.md": `---
  status: Proposed
  date: 2026-06-24
  tags: []
  ---
  # ADR-0002: A Proposal
  ## Context
  Another thing.
  `,
  		"0003-already-implemented.md": `---
  status: Implemented
  date: 2026-06-24
  tags: []
  ---
  # ADR-0003: Already Implemented
  ## Context
  Done.
  `,
  	}
  ```

  to:

  ```go
  	files := map[string]string{
  		"0001-first-accepted.md": testsupport.ADR("Accepted",
  			testsupport.WithDate("2026-06-24"), testsupport.WithTags("tooling"),
  			testsupport.WithTitle("0001: First Accepted"), testsupport.WithBody("## Context\nSomething.\n")),
  		"0002-a-proposal.md": testsupport.ADR("Proposed",
  			testsupport.WithDate("2026-06-24"),
  			testsupport.WithTitle("0002: A Proposal"), testsupport.WithBody("## Context\nAnother thing.\n")),
  		"0003-already-implemented.md": testsupport.ADR("Implemented",
  			testsupport.WithDate("2026-06-24"),
  			testsupport.WithTitle("0003: Already Implemented"), testsupport.WithBody("## Context\nDone.\n")),
  	}
  ```

  (the test asserts only on status-section ordering and per-entry title/filename substrings (never
  on `tags`), so omitting the now-redundant `tags: []` is inert.)

  In `TestParseDirExtractsStatusAndTitle`, change:

  ```go
  	content := "---\nstatus: Accepted\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-0007: Example Title\n## Context\nx\n"
  ```

  to:

  ```go
  	content := testsupport.ADR("Accepted", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
  		testsupport.WithTitle("0007: Example Title"), testsupport.WithBody("## Context\nx\n"))
  ```

  In `TestRenderActiveMDSortsWithinStatusAndOrdersExtra`, change:

  ```go
  		files := map[string]string{
  			"0002-second-accepted.md": "---\nstatus: Accepted\n---\n# ADR-0002: Second Accepted\n",
  			"0001-first-accepted.md":  "---\nstatus: Accepted\n---\n# ADR-0001: First Accepted\n",
  			"0003-draft-status.md":    "---\nstatus: Draft\n---\n# ADR-0003: Draft Status\n",
  		}
  ```

  to:

  ```go
  		files := map[string]string{
  			"0002-second-accepted.md": testsupport.ADR("Accepted", testsupport.WithTitle("0002: Second Accepted")),
  			"0001-first-accepted.md":  testsupport.ADR("Accepted", testsupport.WithTitle("0001: First Accepted")),
  			"0003-draft-status.md":    testsupport.ADR("Draft", testsupport.WithTitle("0003: Draft Status")),
  		}
  ```

  In `TestParseDirExtractsSections`, change:

  ```go
  	content := "---\nstatus: Implemented\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-0009: S\n## Context\nctx body\n## Invariants\n- `inv: example-slug`: a thing.\n## Consequences\ncons\n"
  ```

  to:

  ```go
  	content := testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
  		testsupport.WithTitle("0009: S"), testsupport.WithBody("## Context\nctx body\n## Invariants\n- `inv: example-slug`: a thing.\n## Consequences\ncons\n"))
  ```

  Leave `adrTemplateFixture` (the `docs/decisions/template.md` scaffolding fixture used by
  `NewFile`'s tests) and every malformed-frontmatter literal (`TestParseDirParseError`,
  `TestRenderActiveMDParseError`) untouched; the former is a template fixture, not an ADR instance,
  and the latter are deliberately broken, so `testsupport.ADR` (which only ever emits valid
  frontmatter) cannot express them.

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Stage `internal/adr/adr_test.go`. Commit:
    `test(awf): route internal/adr ADR fixtures through testsupport.ADR`

- [ ] **Task 4.2: `internal/invariants/invariants_test.go`.** Change the import block from:

  ```go
  import (
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/config"
  	"github.com/hypnotox/agentic-workflows/internal/invariants"
  )
  ```

  to:

  ```go
  import (
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/config"
  	"github.com/hypnotox/agentic-workflows/internal/invariants"
  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  Change:

  ```go
  func writeADR(t *testing.T, dir, name, status, invBody string) {
  	t.Helper()
  	content := "---\nstatus: " + status + "\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-X: T\n## Invariants\n" + invBody + "\n## Consequences\nc\n"
  	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
  		t.Fatal(err)
  	}
  }
  ```

  to:

  ```go
  func writeADR(t *testing.T, dir, name, status, invBody string) {
  	t.Helper()
  	content := testsupport.ADR(status, testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
  		testsupport.WithTitle("X: T"), testsupport.WithBody("## Invariants\n"+invBody+"\n## Consequences\nc\n"))
  	testsupport.WriteFile(t, filepath.Join(dir, name), content)
  }
  ```

  Change:

  ```go
  func writeRetiringADR(t *testing.T, dir, name, status, retires, invBody string) {
  	t.Helper()
  	content := "---\nstatus: " + status + "\ndate: 2026-06-25\ntags: [x]\nretires_invariants: [" + retires + "]\n---\n# ADR-X: T\n## Invariants\n" + invBody + "\n## Consequences\nc\n"
  	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
  		t.Fatal(err)
  	}
  }
  ```

  to:

  ```go
  func writeRetiringADR(t *testing.T, dir, name, status, retires, invBody string) {
  	t.Helper()
  	content := testsupport.ADR(status, testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
  		testsupport.WithRetiresInvariants(retires), testsupport.WithTitle("X: T"),
  		testsupport.WithBody("## Invariants\n"+invBody+"\n## Consequences\nc\n"))
  	testsupport.WriteFile(t, filepath.Join(dir, name), content)
  }
  ```

  Leave `goSrc` (writes a `.go` backing-comment fixture, not an ADR) untouched: out of this task's
  scope.

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Stage `internal/invariants/invariants_test.go`. Commit:
    `test(awf): route internal/invariants ADR fixtures via testsupport.ADR`

- [ ] **Task 4.3: `internal/project/domains_test.go`.** Change the import block from:

  ```go
  import (
  	"fmt"
  	"io/fs"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/catalog"
  	"github.com/hypnotox/agentic-workflows/internal/manifest"
  	"github.com/hypnotox/agentic-workflows/internal/render"
  	"github.com/hypnotox/agentic-workflows/templates"
  )
  ```

  to:

  ```go
  import (
  	"fmt"
  	"io/fs"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/catalog"
  	"github.com/hypnotox/agentic-workflows/internal/manifest"
  	"github.com/hypnotox/agentic-workflows/internal/render"
  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  	"github.com/hypnotox/agentic-workflows/templates"
  )
  ```

  Replace every occurrence of the literal `"---\nstatus: Implemented\ndomains: [rendering]\n---\n#
  ADR-0001: Engine\n"` (4 occurrences: in `TestDomainDocRendersIndexAndNarrative`,
  `TestDomainDocStaleOnAdrRetag`, `TestDomainDocMissingWhenDeleted`,
  `TestDomainDocOrphanedWhenDomainRemoved`) with
  `testsupport.ADR("Implemented", testsupport.WithDomains("rendering"), testsupport.WithTitle("0001: Engine"))`
  (exact byte match: `status` then `domains`, no `date`/`tags` in between).

  Replace the single occurrence of `"---\nstatus: Accepted\ndomains: [rendering]\n---\n# ADR-0002:
  Layout\n"` (in `TestDomainDocRendersIndexAndNarrative`) with
  `testsupport.ADR("Accepted", testsupport.WithDomains("rendering"), testsupport.WithTitle("0002: Layout"))`.

  Replace the single occurrence of `"---\nstatus: Accepted\ndomains: [config]\n---\n# ADR-0003:
  Config\n"` (in `TestDomainDocRendersIndexAndNarrative`) with
  `testsupport.ADR("Accepted", testsupport.WithDomains("config"), testsupport.WithTitle("0003: Config"))`.

  Replace the single occurrence of `"---\nstatus: Accepted\ndomains: [rendering]\n---\n# ADR-0002:
  New\n"` (in `TestDomainDocStaleOnAdrRetag`) with
  `testsupport.ADR("Accepted", testsupport.WithDomains("rendering"), testsupport.WithTitle("0002: New"))`.

  Leave `TestGenerateDomainDocsPropagatesIndexError`'s `"---\nstatus: [unterminated\n---\n# ADR-0001:
  Broken\n"` untouched (deliberately malformed).

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Stage `internal/project/domains_test.go`. Commit:
    `test(awf): route project domain-doc fixtures via testsupport.ADR`

- [ ] **Task 4.4: `internal/project/project_test.go` and `internal/project/coverage_test.go`.** In
  `internal/project/project_test.go` (import already carries `testsupport` from Task 3.1), in
  `TestSyncGeneratesActiveMDAndCheckDetectsStaleness`, change:

  ```go
  	adrBody := "---\nstatus: Accepted\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-0001: First\n## Context\nx\n"
  	if err := os.WriteFile(filepath.Join(adrDir, "0001-first.md"), []byte(adrBody), 0o644); err != nil {
  		t.Fatal(err)
  	}
  ```

  to:

  ```go
  	adrBody := testsupport.ADR("Accepted", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
  		testsupport.WithTitle("0001: First"), testsupport.WithBody("## Context\nx\n"))
  	testsupport.WriteFile(t, filepath.Join(adrDir, "0001-first.md"), adrBody)
  ```

  and further down in the same test, change:

  ```go
  	adr2 := strings.Replace(adrBody, "status: Accepted", "status: Implemented", 1)
  	if err := os.WriteFile(filepath.Join(adrDir, "0001-first.md"), []byte(adr2), 0o644); err != nil {
  		t.Fatal(err)
  	}
  ```

  to:

  ```go
  	adr2 := strings.Replace(adrBody, "status: Accepted", "status: Implemented", 1)
  	testsupport.WriteFile(t, filepath.Join(adrDir, "0001-first.md"), adr2)
  ```

  In `internal/project/coverage_test.go`, change the import block from:

  ```go
  import (
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  	"testing/fstest"

  	"github.com/hypnotox/agentic-workflows/internal/config"
  )
  ```

  to:

  ```go
  import (
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  	"testing/fstest"

  	"github.com/hypnotox/agentic-workflows/internal/config"
  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  In `TestCheckInvariantsReportsUnbacked`, change:

  ```go
  	adrBody := "---\nstatus: Implemented\ndate: 2026-06-25\ntags: [x]\n---\n" +
  		"# ADR-0001: First\n## Invariants\n- `inv: my-slug`\n## Context\nx\n"
  	writeFileAt(t, root, filepath.Join("docs", "decisions", "0001-first.md"), adrBody)
  ```

  to:

  ```go
  	adrBody := testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
  		testsupport.WithTitle("0001: First"), testsupport.WithBody("## Invariants\n- `inv: my-slug`\n## Context\nx\n"))
  	writeFileAt(t, root, filepath.Join("docs", "decisions", "0001-first.md"), adrBody)
  ```

  In `TestCheckReportsMissingActiveMD`, change:

  ```go
  	adrBody := "---\nstatus: Accepted\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-0001: First\n## Context\nx\n"
  	writeFileAt(t, root, filepath.Join("docs", "decisions", "0001-first.md"), adrBody)
  ```

  to:

  ```go
  	adrBody := testsupport.ADR("Accepted", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
  		testsupport.WithTitle("0001: First"), testsupport.WithBody("## Context\nx\n"))
  	writeFileAt(t, root, filepath.Join("docs", "decisions", "0001-first.md"), adrBody)
  ```

  (`writeFileAt` itself is migrated in Task 5.1; leaving it in place here is fine, it still exists
  until then.) Leave `TestSyncFailsOnMalformedADR` and `TestCheckFailsOnMalformedADRIndex`
  untouched (deliberately malformed).

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Stage `internal/project/project_test.go internal/project/coverage_test.go`. Commit:
    `test(awf): route internal/project ADR fixtures through testsupport.ADR`

- [ ] **Task 4.5: `internal/audit/audit_test.go`.** Change the import block from:

  ```go
  import (
  	"testing"
  )
  ```

  to:

  ```go
  import (
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  Change:

  ```go
  const proposedADR = "---\nstatus: Proposed\n---\n# ADR\n"
  const acceptedADR = "---\nstatus: Accepted\n---\n# ADR\n"
  ```

  to:

  ```go
  var proposedADR = testsupport.ADR("Proposed")
  var acceptedADR = testsupport.ADR("Accepted")
  ```

  (both consts become vars, since `testsupport.ADR` is a function call; every use of
  `proposedADR`/`acceptedADR` in this file is as an opaque `FileChange.OldText`/`NewText` payload or
  through `statusOf`, which only ever reads the `status:` frontmatter field (verified via
  `grep -n "proposedADR\|acceptedADR" internal/audit/audit_test.go`), so the changed heading text,
  `# ADR-0001: T` instead of `# ADR`, is never observed.)

  Change:

  ```go
  func adrChange(action Action, status string, domains string) FileChange {
  	txt := "---\nstatus: " + status + "\ndomains: [" + domains + "]\n---\nbody\n"
  	return FileChange{Path: "docs/decisions/0099-x.md", Action: action, NewText: txt}
  }
  ```

  to:

  ```go
  func adrChange(action Action, status string, domains string) FileChange {
  	txt := testsupport.ADR(status, testsupport.WithDomains(strings.Split(domains, ", ")...), testsupport.WithBody("body\n"))
  	return FileChange{Path: "docs/decisions/0099-x.md", Action: action, NewText: txt}
  }
  ```

  (`domains` arrives pre-joined, e.g. `"tooling, rendering"` or `"tooling, tooling"`;
  `strings.Split(domains, ", ")` followed by `WithDomains`'s own `", "`-join round-trips to the
  identical `domains: [...]` value. The fixture gains a `# ADR-0001: T` heading the original never
  had; every `adrChange` caller only exercises frontmatter-driven rules
  (`ruleADRDomainCochange`/`ruleDomainDocStaleness`/`ruleUndocumentedDomain`), never the body, so
  this is inert.)

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Stage `internal/audit/audit_test.go`. Commit:
    `test(awf): route internal/audit ADR fixtures through testsupport.ADR`

- [ ] **Task 4.6: `cmd/awf/run_test.go`, `cmd/awf/check_test.go`, `cmd/awf/invariants_test.go`.**
  (All three already import `testsupport` from earlier phases.) In `cmd/awf/run_test.go`'s
  `TestRunInvariantsReportsFindings`, change:

  ```go
  	adr := "---\nstatus: Implemented\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-0001: X\n## Invariants\n- `inv: unbacked-here`: x.\n## Consequences\nc\n"
  	if err := os.WriteFile(filepath.Join(adrDir, "0001-x.md"), []byte(adr), 0o644); err != nil {
  		t.Fatal(err)
  	}
  ```

  to:

  ```go
  	adr := testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
  		testsupport.WithTitle("0001: X"), testsupport.WithBody("## Invariants\n- `inv: unbacked-here`: x.\n## Consequences\nc\n"))
  	testsupport.WriteFile(t, filepath.Join(adrDir, "0001-x.md"), adr)
  ```

  In `cmd/awf/check_test.go`'s `TestRunCheckSurfacesInvariantError`, change:

  ```go
  	adr := func(n, title string) string {
  		return "---\nstatus: Implemented\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-" + n + ": " + title +
  			"\n## Invariants\n- `inv: dup-slug`\n## Context\nx\n"
  	}
  ```

  to:

  ```go
  	adr := func(n, title string) string {
  		return testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
  			testsupport.WithTitle(n+": "+title), testsupport.WithBody("## Invariants\n- `inv: dup-slug`\n## Context\nx\n"))
  	}
  ```

  In `cmd/awf/invariants_test.go`'s `TestRunCheckFailsOnUnbackedInvariant`, change:

  ```go
  	adr := "---\nstatus: Implemented\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-0001: X\n## Invariants\n- `inv: cmd-needs-backing`: x.\n## Consequences\nc\n"
  	if err := os.WriteFile(filepath.Join(adrDir, "0001-x.md"), []byte(adr), 0o644); err != nil {
  		t.Fatal(err)
  	}
  ```

  to:

  ```go
  	adr := testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
  		testsupport.WithTitle("0001: X"), testsupport.WithBody("## Invariants\n- `inv: cmd-needs-backing`: x.\n## Consequences\nc\n"))
  	testsupport.WriteFile(t, filepath.Join(adrDir, "0001-x.md"), adr)
  ```

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Run `./x check`. Expect `awf check: clean`.
  - Stage `cmd/awf/run_test.go cmd/awf/check_test.go cmd/awf/invariants_test.go`. Commit:
    `test(awf): route cmd/awf ADR fixtures through testsupport.ADR`

## Phase 5: File-write + git-fixture + coverage/covercheck fixtures

- [ ] **Task 5.1: `internal/project`: eliminate `writeFileAt`.** `writeFileAt` is defined in
  `internal/project/render_tree_test.go` and called from 4 files: `render_tree_test.go` (4 sites),
  `drift_test.go` (3 sites), `coverage_test.go` (7 sites), `domains_test.go` (1 site, inside its own
  `writeADR` wrapper). Every call site follows `writeFileAt(t, root, REL, BODY)`; the mechanical
  replacement is `testsupport.WriteFile(t, filepath.Join(root, REL), BODY)`.

  In `internal/project/render_tree_test.go`, delete the definition:

  ```go
  func writeFileAt(t *testing.T, root, rel, body string) {
  	t.Helper()
  	abs := filepath.Join(root, rel)
  	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
  		t.Fatal(err)
  	}
  	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
  		t.Fatal(err)
  	}
  }
  ```

  and change its import block from:

  ```go
  import (
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  )
  ```

  to:

  ```go
  import (
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  (`os`/`path/filepath` stay: both are used elsewhere in this file, e.g. `os.ReadFile` +
  `filepath.Join` in `syncAndReadDebugging`/`syncAndReadAgents`.) Replace its 4 call sites:

  - `writeFileAt(t, root, out, "---\nname: \"\"\ndescription: \"\"\n---\nbody\n")` →
    `testsupport.WriteFile(t, filepath.Join(root, out), "---\nname: \"\"\ndescription: \"\"\n---\nbody\n")`
  - `writeFileAt(t, root, out, "---\nname: my-local\ndescription: a local skill\n---\nbody\n")` →
    `testsupport.WriteFile(t, filepath.Join(root, out), "---\nname: my-local\ndescription: a local skill\n---\nbody\n")`
  - `writeFileAt(t, root, ".claude/skills/example-my-local/SKILL.md", valid)` →
    `testsupport.WriteFile(t, filepath.Join(root, ".claude/skills/example-my-local/SKILL.md"), valid)`
  - `writeFileAt(t, root, ".cursor/skills/example-my-local/SKILL.md", valid)` →
    `testsupport.WriteFile(t, filepath.Join(root, ".cursor/skills/example-my-local/SKILL.md"), valid)`

  In `internal/project/drift_test.go`, change the import block from:

  ```go
  import (
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/manifest"
  	"github.com/hypnotox/agentic-workflows/internal/migrate"
  )
  ```

  to:

  ```go
  import (
  	"path/filepath"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/manifest"
  	"github.com/hypnotox/agentic-workflows/internal/migrate"
  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  and replace its 3 call sites:

  - `writeFileAt(t, root, ".awf/skills/tdd.yaml", "data:\n  testSurfaces:\n    - {name: Changed, location: x, kind: y}\n")` →
    `testsupport.WriteFile(t, filepath.Join(root, ".awf/skills/tdd.yaml"), "data:\n  testSurfaces:\n    - {name: Changed, location: x, kind: y}\n")`
  - `writeFileAt(t, root, ".awf/skills/parts/tdd/notes.md", "NEW NOTES BODY\n")` →
    `testsupport.WriteFile(t, filepath.Join(root, ".awf/skills/parts/tdd/notes.md"), "NEW NOTES BODY\n")`
  - `writeFileAt(t, root, ".awf/config.yaml", cfg("now-set"))` →
    `testsupport.WriteFile(t, filepath.Join(root, ".awf/config.yaml"), cfg("now-set"))`

  In `internal/project/coverage_test.go` (import already carries `testsupport` from Task 4.4),
  change:

  ```go
  func corruptSidecar(t *testing.T, root, rel string) {
  	t.Helper()
  	writeFileAt(t, root, filepath.Join(".awf", rel), "bogusUnknownField: true\n")
  }
  ```

  to:

  ```go
  func corruptSidecar(t *testing.T, root, rel string) {
  	t.Helper()
  	testsupport.WriteFile(t, filepath.Join(root, ".awf", rel), "bogusUnknownField: true\n")
  }
  ```

  and replace its 6 remaining call sites:

  - (×2, in `TestSyncFailsOnMalformedADR` and `TestCheckFailsOnMalformedADRIndex`)
    `writeFileAt(t, root, filepath.Join("docs", "decisions", "0001-bad.md"),
    "---\nstatus: [unterminated\n---\n# ADR-0001: Bad\n")` →
    `testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", "0001-bad.md"),
    "---\nstatus: [unterminated\n---\n# ADR-0001: Bad\n")`
  - `writeFileAt(t, root, filepath.Join(".claude", "skills"), "i am a file, not a dir\n")` →
    `testsupport.WriteFile(t, filepath.Join(root, ".claude", "skills"), "i am a file, not a dir\n")`
  - (×2, in `TestCheckInvariantsReportsUnbacked`-adjacent `TestSyncFailsOnMalformedADR` region and
    `TestCheckReportsMissingActiveMD`) `writeFileAt(t, root, filepath.Join("docs", "decisions",
    "0001-first.md"), adrBody)` → `testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions",
    "0001-first.md"), adrBody)`
  - `writeFileAt(t, root, filepath.Join(".awf", "skills", "parts", "debugging", "sub", "x.md"),
    "nested\n")` → `testsupport.WriteFile(t, filepath.Join(root, ".awf", "skills", "parts",
    "debugging", "sub", "x.md"), "nested\n")`

  In `internal/project/domains_test.go` (import already carries `testsupport` from Task 4.3),
  change:

  ```go
  func writeADR(t *testing.T, root, name, body string) {
  	t.Helper()
  	writeFileAt(t, root, filepath.Join("docs", "decisions", name), body)
  }
  ```

  to:

  ```go
  func writeADR(t *testing.T, root, name, body string) {
  	t.Helper()
  	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", name), body)
  }
  ```

  Verify and commit:
  - Run `go build ./...`. Expect no errors (confirms no stray `writeFileAt` reference survives).
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Stage `internal/project/render_tree_test.go internal/project/drift_test.go
    internal/project/coverage_test.go internal/project/domains_test.go`. Commit:
    `test(awf): route internal/project file fixtures via WriteFile`

- [ ] **Task 5.2: `internal/migrate`: eliminate `mustWrite`/`mustMkdir`.** `mustWrite(t, path,
  body)` matches `testsupport.WriteFile(t, path, content)`'s signature exactly (no argument
  restructuring needed), so every occurrence of `mustWrite(t, ` becomes `testsupport.WriteFile(t, `
  via blind substitution, across both `internal/migrate/migrate_test.go` (16 sites) and
  `internal/migrate/singletonstandarddocs_test.go` (5 sites).

  `mustMkdir(t, dir)` has no `testsupport` analogue (it creates a bare directory, not a file); its 5
  call sites in `migrate_test.go` are inlined to their original one-line body instead:

  - `mustMkdir(t, filepath.Join(root, ".claude", "awf"))` (inside `writeLegacyRoot`) →
    ```go
    if err := os.MkdirAll(filepath.Join(root, ".claude", "awf"), 0o755); err != nil {
    	t.Fatal(err)
    }
    ```
  - `mustMkdir(t, filepath.Join(root, ".claude", "awf", "config.yaml"))` (in
    `TestApplyTreeLayoutConfigWriteError`: deliberately squats a directory where a file belongs) →
    ```go
    if err := os.MkdirAll(filepath.Join(root, ".claude", "awf", "config.yaml"), 0o755); err != nil {
    	t.Fatal(err)
    }
    ```
  - `mustMkdir(t, filepath.Join(root, ".claude", "awf", "skills", "parts", "beta", "sec.md"))` (in
    `TestApplyTreeLayoutCopyPartWriteError`: same squatting trick) →
    ```go
    if err := os.MkdirAll(filepath.Join(root, ".claude", "awf", "skills", "parts", "beta", "sec.md"), 0o755); err != nil {
    	t.Fatal(err)
    }
    ```
  - `mustMkdir(t, lock)` (in `TestApplyTreeLayoutLockRemoveError`) →
    ```go
    if err := os.MkdirAll(lock, 0o755); err != nil {
    	t.Fatal(err)
    }
    ```
  - `mustMkdir(t, filepath.Join(root, ".claude", "awf", "parts", "agents-doc", "you-and-this-project.md"))`
    (in `TestPortAgentsDocProseWriteError`: same squatting trick, on the agents-doc prose part) →
    ```go
    if err := os.MkdirAll(filepath.Join(root, ".claude", "awf", "parts", "agents-doc", "you-and-this-project.md"), 0o755); err != nil {
    	t.Fatal(err)
    }
    ```

  Then delete the now-orphaned definitions from `migrate_test.go`:

  ```go
  func mustMkdir(t *testing.T, dir string) {
  	t.Helper()
  	if err := os.MkdirAll(dir, 0o755); err != nil {
  		t.Fatal(err)
  	}
  }

  func mustWrite(t *testing.T, path, body string) {
  	t.Helper()
  	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
  		t.Fatal(err)
  	}
  	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
  		t.Fatal(err)
  	}
  }
  ```

  Change `migrate_test.go`'s import block from:

  ```go
  import (
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/manifest"
  	"gopkg.in/yaml.v3"
  )
  ```

  to:

  ```go
  import (
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/manifest"
  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  	"gopkg.in/yaml.v3"
  )
  ```

  (`os` stays: it backs the 4 inlined `MkdirAll` calls above plus extensive direct
  `os.Stat`/`os.Remove`/`os.ReadFile` use elsewhere in the file.) Change
  `singletonstandarddocs_test.go`'s import block from:

  ```go
  import (
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  )
  ```

  to:

  ```go
  import (
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Stage `internal/migrate/migrate_test.go internal/migrate/singletonstandarddocs_test.go`. Commit:
    `test(awf): route internal/migrate file fixtures via WriteFile`

- [ ] **Task 5.3: Route git fixtures through `gitfixture`.** In `internal/audit/git_test.go`,
  delete:

  ```go
  var testSig = &object.Signature{Name: "T", Email: "t@example.com", When: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}

  func initRepo(t *testing.T) (*git.Repository, string) {
  	t.Helper()
  	dir := t.TempDir()
  	repo, err := git.PlainInit(dir, false)
  	if err != nil {
  		t.Fatalf("init: %v", err)
  	}
  	return repo, dir
  }

  // commit writes/removes files in the worktree and commits, returning the hash.
  func commit(t *testing.T, repo *git.Repository, dir, msg string, write map[string]string, remove ...string) plumbing.Hash {
  	t.Helper()
  	wt, err := repo.Worktree()
  	if err != nil {
  		t.Fatalf("worktree: %v", err)
  	}
  	for name, content := range write {
  		if werr := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); werr != nil {
  			t.Fatalf("write %s: %v", name, werr)
  		}
  		if _, aerr := wt.Add(name); aerr != nil {
  			t.Fatalf("add %s: %v", name, aerr)
  		}
  	}
  	for _, name := range remove {
  		if _, rerr := wt.Remove(name); rerr != nil {
  			t.Fatalf("remove %s: %v", name, rerr)
  		}
  	}
  	h, err := wt.Commit(msg, &git.CommitOptions{Author: testSig, Committer: testSig})
  	if err != nil {
  		t.Fatalf("commit: %v", err)
  	}
  	return h
  }
  ```

  Replace every occurrence of `initRepo(t)` with `gitfixture.InitRepo(t)` (12 occurrences), every
  occurrence of `commit(t, repo, dir, ` with `gitfixture.Commit(t, repo, dir, ` (15 occurrences), and
  both occurrences of `testSig` (inside `orphan`'s `object.Commit{Author: *testSig, Committer:
  *testSig, ...}`) with `gitfixture.Sig`. Change the import block from:

  ```go
  import (
  	"os"
  	"path/filepath"
  	"testing"
  	"time"

  	"github.com/go-git/go-git/v5"
  	"github.com/go-git/go-git/v5/plumbing"
  	"github.com/go-git/go-git/v5/plumbing/object"
  )
  ```

  to:

  ```go
  import (
  	"os"
  	"path/filepath"
  	"testing"

  	"github.com/go-git/go-git/v5"
  	"github.com/go-git/go-git/v5/plumbing"
  	"github.com/go-git/go-git/v5/plumbing/object"
  	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
  )
  ```

  (`time` is dropped: `testSig`'s deletion was its only user, verified via `grep -n "time\."
  internal/audit/git_test.go`; `object` stays, used directly by `orphan`.)

  In `cmd/awf/audit_test.go` (its own `swapGetwd`/`WriteAwfConfig` migrations from Tasks 2.2/3.4
  already landed), delete:

  ```go
  var auditSig = &object.Signature{Name: "T", Email: "t@example.com", When: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
  ```

  and:

  ```go
  func auditCommit(t *testing.T, repo *git.Repository, root, msg string, write map[string]string) plumbing.Hash {
  	t.Helper()
  	wt, err := repo.Worktree()
  	if err != nil {
  		t.Fatal(err)
  	}
  	for name, content := range write {
  		if werr := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); werr != nil {
  			t.Fatal(werr)
  		}
  		if _, aerr := wt.Add(name); aerr != nil {
  			t.Fatal(aerr)
  		}
  	}
  	h, err := wt.Commit(msg, &git.CommitOptions{Author: auditSig, Committer: auditSig})
  	if err != nil {
  		t.Fatal(err)
  	}
  	return h
  }
  ```

  In `auditProject`, change:

  ```go
  	base, err := wt.Commit("feat(awf): base", &git.CommitOptions{Author: auditSig, Committer: auditSig})
  ```

  to:

  ```go
  	base, err := wt.Commit("feat(awf): base", &git.CommitOptions{Author: gitfixture.Sig, Committer: gitfixture.Sig})
  ```

  Replace every occurrence of `auditCommit(t, repo, root, ` with `gitfixture.Commit(t, repo, root, `
  (5 occurrences). Change the import block from:

  ```go
  import (
  	"bytes"
  	"io"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  	"time"

  	"github.com/go-git/go-git/v5"
  	"github.com/go-git/go-git/v5/plumbing"
  	"github.com/go-git/go-git/v5/plumbing/object"
  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  to:

  ```go
  import (
  	"bytes"
  	"io"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/go-git/go-git/v5"
  	"github.com/go-git/go-git/v5/plumbing"
  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
  )
  ```

  (`time` and `.../plumbing/object` are dropped: `auditSig` was their only user, verified via
  `grep -n "object\.\|time\." cmd/awf/audit_test.go`; `os`/`path/filepath` stay, used by
  `auditProject`'s `go.mod`/`main.go` writes.)

  This migration is functional, not merely mechanical: `gitfixture.Commit`'s variadic `remove
  ...string` parameter is new capability at every migrated `cmd/awf/audit_test.go` call site;
  `auditCommit` never had a `remove` parameter (the copy-drift ADR-0044 calls out). No existing test
  passes a `remove` argument through the migrated call sites, so this commit changes no test
  behavior; a future `cmd/awf/audit_test.go` test that needs to exercise a file-removal commit can
  now do so without adding another local helper.

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Stage `internal/audit/git_test.go cmd/awf/audit_test.go`. Commit:
    `test(awf): route git fixtures through internal/testsupport/gitfixture`

- [ ] **Task 5.4: Route coverage/covercheck fixtures through `WriteGoModule`/`WriteProfile`.** In
  `internal/coverage/coverage_test.go` (its `testsupport` import already landed in Task 2.1), change:

  ```go
  // writeProfile writes a coverprofile and returns its path.
  func writeProfile(t *testing.T, dir, body string) string {
  	t.Helper()
  	p := filepath.Join(dir, "cover.out")
  	if err := os.WriteFile(p, []byte("mode: set\n"+body), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	return p
  }

  // module builds a temp module root: go.mod + one source file, and returns root + modpath.
  func module(t *testing.T, src string) (root, modPath string) {
  	t.Helper()
  	root = t.TempDir()
  	modPath = "example.com/m"
  	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+modPath+"\n\ngo 1.26\n"), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	if err := os.WriteFile(filepath.Join(root, "f.go"), []byte(src), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	return root, modPath
  }
  ```

  to:

  ```go
  // writeProfile writes a coverprofile and returns its path.
  func writeProfile(t *testing.T, dir, body string) string {
  	t.Helper()
  	return testsupport.WriteProfile(t, dir, body)
  }

  // module builds a temp module root: go.mod + one source file, and returns root + modpath.
  func module(t *testing.T, src string) (root, modPath string) {
  	t.Helper()
  	root = t.TempDir()
  	modPath = "example.com/m"
  	testsupport.WriteGoModule(t, root, modPath, src)
  	return root, modPath
  }
  ```

  (`module`/`writeProfile` keep their existing call-site signatures (used ~15 times each elsewhere
  in this file); only their bodies now route through `testsupport`, mirroring how `scaffoldFiles`
  etc. were treated in Phase 3, not the full elimination `writeFileAt`/`mustWrite` got in Tasks
  5.1/5.2: `module`'s fixed-`modPath` convention and tuple return are genuine value the shared
  primitive alone doesn't provide.)

  In `cmd/covercheck/main_test.go`, change:

  ```go
  // modWith builds a temp module (go.mod + f.go) and a coverprofile, then chdir's
  // into it so coverage.CheckProfile resolves this module. Returns the profile path.
  func modWith(t *testing.T, profileBody string) string {
  	t.Helper()
  	root := t.TempDir()
  	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	if err := os.WriteFile(filepath.Join(root, "f.go"), []byte("package m\nfunc F() {}\n"), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	prof := filepath.Join(root, "cover.out")
  	if err := os.WriteFile(prof, []byte("mode: set\n"+profileBody), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	t.Chdir(root)
  	return prof
  }
  ```

  to:

  ```go
  // modWith builds a temp module (go.mod + f.go) and a coverprofile, then chdir's
  // into it so coverage.CheckProfile resolves this module. Returns the profile path.
  func modWith(t *testing.T, profileBody string) string {
  	t.Helper()
  	root := t.TempDir()
  	testsupport.WriteGoModule(t, root, "example.com/m", "package m\nfunc F() {}\n")
  	prof := testsupport.WriteProfile(t, root, profileBody)
  	t.Chdir(root)
  	return prof
  }
  ```

  Change its import block from:

  ```go
  import (
  	"bytes"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  )
  ```

  to:

  ```go
  import (
  	"bytes"
  	"strings"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/testsupport"
  )
  ```

  (`os`/`path/filepath` are dropped: `modWith` was their only user in this file, verified via
  `grep -n "os\.\|filepath\." cmd/covercheck/main_test.go`.)

  Verify and commit:
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Stage `internal/coverage/coverage_test.go cmd/covercheck/main_test.go`. Commit:
    `test(awf): route coverage/covercheck fixtures via testsupport helpers`

## Phase 6: Doc currency and ADR flip

- [ ] **Task 6.1: Update the `tooling` domain narrative.** In
  `.awf/domains/parts/tooling/current-state.md`, append a new paragraph after the final existing
  paragraph (the one starting `` `awf new adr "<title>"` (ADR-0042) scaffolds a new ADR file: ``):

  ```
  `internal/testsupport` (ADR-0044) is awf's shared test-fixture package: `TestMain` HOME isolation, project-config scaffolding, ADR frontmatter fixtures, file-writing primitives, and the seam-swap idiom, plus `internal/testsupport/gitfixture` for git-repo fixtures, consolidating idioms that had drifted into independent, sometimes-inconsistent copies across `cmd/awf`, `internal/project`, `internal/audit`, `internal/coverage`, `internal/migrate`, and `internal/invariants`. It is a leaf package: no file in `internal/testsupport` (including `gitfixture`) may import another `internal/*` awf package, mechanically enforced by a dedicated test that walks the package's own import graph, so it stays safely importable from any package's tests.
  ```

- [ ] **Task 6.2: Fill in `docs/testing.md`'s "Test layout" section.** That section has been the
  unfilled project-default placeholder text since `awf init` scaffolded this repo (no
  `.awf/docs/parts/testing/` override exists yet); this plan is the first to establish a real,
  citable test-fixture convention, so it belongs there. Create
  `.awf/docs/parts/testing/layout.md`:

  ```
  Package unit tests are Go `_test.go` files in `internal/<pkg>`, in that package's own test
  package (`package <pkg>` or the black-box `package <pkg>_test` where a test needs no access to
  unexported identifiers). Template golden tests (render assertions against the embedded catalog)
  live in `internal/project/spine_test.go`. CLI integration tests drive the `awf` binary's
  command functions directly (not a subprocess) against a temp directory built with `t.TempDir()`,
  in `cmd/awf/*_test.go`.

  Shared test-fixture building (project-config scaffolding, ADR frontmatter fixtures,
  file-writing primitives, the seam-swap idiom, and git-repo fixtures) goes through
  `internal/testsupport` (and its `gitfixture` subpackage), a leaf package with no dependency on
  any other `internal/*` awf package (ADR-0044). New test code needing one of these idioms calls
  into `internal/testsupport` rather than hand-rolling a local copy.
  ```

- [ ] **Task 6.3: Flip ADR-0044 to Implemented.** In
  `docs/decisions/0044-shared-test-support-package.md`, change the frontmatter `status: Proposed` to
  `status: Implemented`.

- [ ] **Task 6.4: Sync, verify, commit.**
  - Run `./x sync`. Expect `awf sync: done` (re-renders `docs/domains/tooling.md`, `docs/testing.md`,
    and `docs/decisions/ACTIVE.md`).
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Run `./x invariants`. Expect `awf invariants: clean` (ADR-0044's `testsupport-zero-internal-deps`
    slug is backed by `internal/testsupport/deps_test.go`'s comment tag, added in Task 1.3).
  - Run `./x check`. Expect `awf check: clean`.
  - Stage `.awf/domains/parts/tooling/current-state.md .awf/docs/parts/testing/layout.md
    docs/decisions/0044-shared-test-support-package.md docs/domains/tooling.md docs/testing.md
    docs/decisions/ACTIVE.md .awf/awf.lock`. Commit:
    `docs(awf): document internal/testsupport and implement ADR-0044`

## Verification (whole change)

- `./x gate` green; `./x check` clean; `./x invariants` clean at the end of every phase, not just the
  last.
- `go doc ./internal/testsupport` lists `RunIsolated`, `WriteFile`, `WriteAwfConfig`, `SwapVar`,
  `WriteGoModule`, `WriteProfile`, `ADR`, and the six `ADROption`s; `go doc
  ./internal/testsupport/gitfixture` lists `Sig`, `InitRepo`, `Commit`.
- `grep -rn "internal/hypnotox\|internal/agentic" internal/testsupport/*.go
  internal/testsupport/gitfixture/*.go` (excluding `_test.go`) finds nothing: no non-test file under
  `internal/testsupport/` imports another `internal/*` awf package.
- `grep -rln "os.MkdirTemp(\"\", \"awf-.*-test-home\")\|swapGetwd\|swapHasGoMod\|forceNonInteractive\b.*orig :=\|mustWrite\|mustMkdir\|writeFileAt\|testSig\|auditSig\|auditCommit\b" cmd/awf internal/audit internal/project internal/coverage internal/migrate` (excluding this plan's own
  reference text) turns up nothing: every named duplication site from ADR-0044's Context is gone.
- ADR-0044 is `Implemented`; its `testsupport-zero-internal-deps` invariant is backed.

## Execution

Phases are ordered and mostly sequential: Phase 1 must land before any later phase (every subsequent
phase's tasks call into `internal/testsupport`/`gitfixture`). Within Phase 1, Tasks 1.1-1.3 are
independent of each other but Task 1.4 depends on 1.1 (it edits the same file). Phases 2-5 are
independent of each other in principle, but execute them in the written order: 2 and 3 touch the
same `cmd/awf` files repeatedly (import blocks accumulate across tasks) and are simplest to reason
about sequentially; Phase 6 must be last (it documents and flips the ADR after every migration is
real). Execute inline with `awf-executing-plans` (one task at a time, `./x gate` per commit); the
tasks are individually small but the plan is long and touches ~30 files, which benefits from a single
continuous session's context more than subagent dispatch would.
