// Package testsupport provides shared test-fixture helpers used across awf's
// test suites: TestMain HOME isolation, project-config scaffolding, ADR
// frontmatter fixtures, file-writing primitives, and the seam-swap idiom. It
// is a leaf package -
// only the Go standard library may be imported here (see the gitfixture
// subpackage for the go-git-dependent helpers) - so it is safe to import from
// any package's tests without risking an import cycle (ADR-0044). deps_test.go
// enforces the zero-internal-deps constraint mechanically.
package testsupport

import (
	"os"
	"path/filepath"
	"strconv"
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

// WriteAwfConfig writes <root>/.awf/config.yaml with the given content - the
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
	title   string
	date    string
	tags    []string
	related []int
	domains []string
	body    string
}

// WithTitle sets the ADR's number+title heading text - the part after
// "# ADR-", e.g. "0001: My Title". Defaults to "0001: T".
func WithTitle(title string) ADROption { return func(o *adrOpts) { o.title = title } }

// WithDate sets the frontmatter date field. Omitted from the frontmatter
// entirely when never called - some fixtures deliberately carry no date.
func WithDate(date string) ADROption { return func(o *adrOpts) { o.date = date } }

// WithTags sets the frontmatter tags array.
func WithTags(tags ...string) ADROption { return func(o *adrOpts) { o.tags = tags } }

// WithRelated sets the frontmatter related array (ADR numbers).
func WithRelated(nums ...int) ADROption { return func(o *adrOpts) { o.related = nums } }

// WithDomains sets the frontmatter domains array.
func WithDomains(domains ...string) ADROption { return func(o *adrOpts) { o.domains = domains } }

// WithBody appends raw markdown (e.g. "## Context\nx\n") after the title
// heading.
func WithBody(body string) ADROption { return func(o *adrOpts) { o.body = body } }

// ADR builds a ---delimited ADR frontmatter fixture as a raw string: a status
// field plus any of date/tags/domains
// supplied via opts, a "# ADR-<title>" heading, and an optional trailing body. It intentionally
// does not import internal/adr and marshal its real frontmatter struct -
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
	if o.related != nil {
		parts := make([]string, len(o.related))
		for i, n := range o.related {
			parts[i] = strconv.Itoa(n)
		}
		b.WriteString("related: [" + strings.Join(parts, ", ") + "]\n")
	}
	if o.domains != nil {
		b.WriteString("domains: [" + strings.Join(o.domains, ", ") + "]\n")
	}
	b.WriteString("---\n# ADR-" + o.title + "\n")
	b.WriteString(o.body)
	return b.String()
}

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

// RepoRoot ascends from the test's working directory to the directory holding
// go.mod, so a repo-wide scan is never anchored to a package's depth.
func RepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil { // coverage-ignore: the test process always has a working directory
		t.Fatal(err)
	}
	for {
		if _, serr := os.Stat(filepath.Join(dir, "go.mod")); serr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir { // coverage-ignore: the test tree always sits under a go.mod
			t.Fatal("go.mod not found above the test working directory")
		}
		dir = parent
	}
}

// WalkRepoFiles calls fn(relPath, contents) for each repository-owned file for
// which include returns true. It is the single definition of awf's repo-walk
// boundary: hidden trees (notably .claude/worktrees/) and nested checkouts (a
// directory carrying its own .git directory or gitdir-pointer file) are pruned.
// The caller owns file selection, so the same boundary can scan Go, Markdown,
// or any other repository surface without duplicating traversal rules.
func WalkRepoFiles(t *testing.T, root string, include func(rel string) bool, fn func(rel string, body []byte)) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, werr error) error {
		if werr != nil { // coverage-ignore: walking a readable checkout does not fault
			return werr
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil { // coverage-ignore: path always sits under root
			return rerr
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			if _, serr := os.Lstat(filepath.Join(path, ".git")); serr == nil {
				return filepath.SkipDir // a nested checkout owns its own files
			} else if !os.IsNotExist(serr) { // coverage-ignore: repository paths remain statable during a test scan
				return serr
			}
			return nil
		}
		rel = filepath.ToSlash(rel)
		if !include(rel) {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil { // coverage-ignore: the walk just listed this regular file
			return rerr
		}
		fn(rel, body)
		return nil
	})
	if err != nil { // coverage-ignore: callbacks do not error and the walk cannot fault
		t.Fatal(err)
	}
}

// WalkRepoSources preserves the production-Go scanner API over WalkRepoFiles.
func WalkRepoSources(t *testing.T, root string, fn func(rel string, body []byte)) {
	t.Helper()
	WalkRepoFiles(t, root, func(rel string) bool {
		return strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go")
	}, fn)
}
