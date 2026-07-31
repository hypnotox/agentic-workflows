package git

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The two walkers in this package and its fixture sibling are the mechanical
// half of the seam: the contract suites prove each entrypoint behaves, and these
// prove nothing bypasses them. They read source rather than types because the
// thing being forbidden is a construction, not a call graph, and a bypass added
// tomorrow must fail without anyone remembering the rule exists.

// gitLibraryPrefixes are the import paths that reach a Git repository in
// process. go-billy accompanies go-git as its filesystem abstraction, so a file
// importing it alone is still reaching the backend directly.
var gitLibraryPrefixes = []string{
	"github.com/go-git/go-git/",
	"github.com/go-git/go-billy/",
}

// gitAccessFinding is one construction that reaches Git outside the seam.
type gitAccessFinding struct {
	Path   string
	Detail string
}

func (f gitAccessFinding) String() string { return f.Path + ": " + f.Detail }

// scanGitAccess reports every direct Git access in one parsed file: an import of
// a Git library, or an os/exec construction naming the git binary. Both forms
// are detected structurally, so neither an unusual argument shape nor a
// reformatting hides one. A subprocess whose command name is a variable is
// undetectable here and is not claimed to be caught; the seam's own review
// covers that, and no such site exists.
func scanGitAccess(path string, src []byte) ([]gitAccessFinding, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	findings := []gitAccessFinding{}
	for _, imp := range file.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil { // coverage-ignore: the parser accepted the file, so every import path is a well-formed quoted string
			return nil, fmt.Errorf("%s: unquote import %s: %w", path, imp.Path.Value, err)
		}
		for _, prefix := range gitLibraryPrefixes {
			if strings.HasPrefix(p, prefix) {
				findings = append(findings, gitAccessFinding{path, fmt.Sprintf("imports the Git library %q directly; consume the seam in internal/git instead", p)})
			}
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if !isExecConstruction(call.Fun) {
			return true
		}
		for _, arg := range call.Args {
			if literalString(arg) == "git" {
				findings = append(findings, gitAccessFinding{path, "constructs a git subprocess directly; route it through the seam's runner in internal/git"})
				return true
			}
		}
		return true
	})
	return findings, nil
}

// isExecConstruction reports whether fun names os/exec's process constructors.
func isExecConstruction(fun ast.Expr) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return pkg.Name == "exec" && (sel.Sel.Name == "Command" || sel.Sel.Name == "CommandContext")
}

// literalString returns the value of an untyped string literal, or the empty
// string for any other expression.
func literalString(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil { // coverage-ignore: the parser accepted this literal, so it unquotes
		return ""
	}
	return value
}

// walkGitAccess scans every module file the selector accepts and returns the
// findings outside allowed. Paths are module-relative and slash-separated so an
// allowlist entry reads the same on every platform. The count of files seen is
// returned so a caller can refuse a walk that silently matched nothing.
func walkGitAccess(t *testing.T, testFiles bool, allowed []string) ([]gitAccessFinding, int) {
	t.Helper()
	root := moduleRoot(t)
	findings, seen := []gitAccessFinding{}, 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil { // coverage-ignore: the walk covers a checked-out module the test binary is already running from
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil { // coverage-ignore: every walked path is rooted at root
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			// examples/sundial is a separate module with its own dependencies,
			// and testdata holds fixture sources that are never compiled.
			if rel == ".git" || rel == "examples" || d.Name() == "testdata" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") != testFiles {
			return nil
		}
		seen++
		if allowedGitAccess(rel, allowed) {
			return nil
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil { // coverage-ignore: the walk just enumerated this file
			return readErr
		}
		found, scanErr := scanGitAccess(rel, src)
		if scanErr != nil { // coverage-ignore: every non-allowlisted module file compiles, so it parses
			return scanErr
		}
		findings = append(findings, found...)
		return nil
	})
	if err != nil { // coverage-ignore: every callback error above is itself unreachable
		t.Fatal(err)
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].String() < findings[j].String() })
	return findings, seen
}

// allowedGitAccess reports whether rel is inside an allowlisted area. An entry
// ending in "/" is a directory prefix; any other entry is an exact file.
func allowedGitAccess(rel string, allowed []string) bool {
	for _, entry := range allowed {
		if strings.HasSuffix(entry, "/") && strings.HasPrefix(rel, entry) {
			return true
		}
		if rel == entry {
			return true
		}
	}
	return false
}

// seamAllowlist is where production code may reach Git directly: the seam
// itself, and the fixture package, which cannot consume the seam because
// tooling/quality-gates:testsupport-zero-internal-deps forbids it importing any
// internal package.
var seamAllowlist = []string{
	"internal/git/",
	"internal/testsupport/gitfixture/",
}

// TestNoProductionGitAccessOutsideTheSeam fails when any non-test file in the
// module imports a Git library or builds a git subprocess outside the seam.
// This is the enforcement half of one-implementation-per-entrypoint: without it
// the seam is a convention, and the seven bypassing call sites this effort
// removed could return one at a time without anyone noticing.
func TestNoProductionGitAccessOutsideTheSeam(t *testing.T) {
	t.Parallel()
	findings, seen := walkGitAccess(t, false, seamAllowlist)
	if seen == 0 {
		t.Fatal("walked no production files, so the walk proves nothing")
	}
	for _, f := range findings {
		t.Errorf("%s", f)
	}
}

// TestSeamWalkerDetectsBothBypassForms proves the walker itself, because a
// scanner that silently matches nothing passes exactly like a clean tree. Each
// case is a source shape the walker must reject, including the
// exec.CommandContext(t.Context(), "git", ...) spelling that a naive regexp
// misses because a bracket expression cannot cross the inner parenthesis.
func TestSeamWalkerDetectsBothBypassForms(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "go-git import",
			src:  "package p\n\nimport gogit \"github.com/go-git/go-git/v5\"\n\nvar _ = gogit.PlainInit\n",
			want: "imports the Git library",
		},
		{
			name: "go-billy import",
			src:  "package p\n\nimport \"github.com/go-git/go-billy/v5\"\n\nvar _ billy.Filesystem\n",
			want: "imports the Git library",
		},
		{
			name: "exec.Command",
			src:  "package p\n\nimport \"os/exec\"\n\nvar _ = exec.Command(\"git\", \"status\")\n",
			want: "constructs a git subprocess",
		},
		{
			name: "exec.CommandContext with a call in the context argument",
			src:  "package p\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\nfunc f(t *testing.T) { _ = exec.CommandContext(t.Context(), \"git\", \"status\") }\n",
			want: "constructs a git subprocess",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			findings, err := scanGitAccess("probe.go", []byte(test.src))
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != 1 || !strings.Contains(findings[0].Detail, test.want) {
				t.Fatalf("findings = %v, want one containing %q", findings, test.want)
			}
		})
	}
	clean := "package p\n\nimport \"os/exec\"\n\nvar _ = exec.Command(\"hg\", \"status\")\n"
	findings, err := scanGitAccess("probe.go", []byte(clean))
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("findings for a non-git subprocess = %v, want none", findings)
	}
	if _, err := scanGitAccess("probe.go", []byte("package")); err == nil {
		t.Fatal("scanGitAccess accepted an unparseable file")
	}
}
