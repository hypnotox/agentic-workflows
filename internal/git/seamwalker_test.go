package git

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// The two walkers in this package and its fixture sibling are the mechanical
// half of the seam: the contract suites prove each entrypoint behaves, and these
// prove nothing bypasses them. They read source rather than types because the
// thing being forbidden is a construction, not a call graph, and a bypass added
// tomorrow must fail without anyone remembering the rule exists.

// gitLibraryPrefixes are the import paths that reach a Git repository in
// process. The whole go-git organisation is covered rather than the two modules
// in use today: go-billy is go-git's filesystem abstraction, gcfg parses
// .git/config, and go-git-fixtures builds repositories, so each reaches the
// backend as directly as go-git itself. Naming the organisation also closes a
// trap in the narrower form, where "github.com/go-git/go-git/" does not prefix
// -match "github.com/go-git/go-git-fixtures/v4".
var gitLibraryPrefixes = []string{
	"github.com/go-git/",
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
	// The local name os/exec was imported under, so an aliased import is caught
	// too. Empty when the file does not import it at all.
	execName := importLocalName(file, "os/exec")
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if !isExecConstruction(node.Fun, execName) {
				return true
			}
			for _, arg := range node.Args {
				if literalString(arg) == "git" {
					findings = append(findings, gitAccessFinding{path, "constructs a git subprocess directly; route it through the seam's runner in internal/git"})
					return true
				}
			}
		case *ast.CompositeLit:
			// exec.Cmd{Path: "git"} builds the same process without a call.
			sel, ok := node.Type.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Cmd" {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || execName == "" || pkg.Name != execName {
				return true
			}
			for _, elt := range node.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if ok && literalString(kv.Value) == "git" {
					findings = append(findings, gitAccessFinding{path, "constructs a git subprocess directly; route it through the seam's runner in internal/git"})
					return true
				}
			}
		}
		return true
	})
	return findings, nil
}

type execConstruction struct {
	Path        string
	Constructor string
	Command     string
	Literal     bool
}

// scanExecConstructions reports every os/exec construction, including one
// whose executable is not a literal. The fixture census uses the broader shape
// because its two allowed process boundaries form a closed set.
func scanExecConstructions(path string, src []byte) ([]execConstruction, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	execName := importLocalName(file, "os/exec")
	parents := astParentMap(file)
	constructions := []execConstruction{}
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if !isExecConstruction(node.Fun, execName) {
				return true
			}
			sel := node.Fun.(*ast.SelectorExpr)
			commandIndex := 0
			if sel.Sel.Name == "CommandContext" {
				commandIndex = 1
			}
			construction := execConstruction{Path: path, Constructor: sel.Sel.Name}
			if len(node.Args) > commandIndex {
				if lit, ok := node.Args[commandIndex].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					construction.Command, err = strconv.Unquote(lit.Value)
					if err != nil { // coverage-ignore: the parser accepted the literal, so it unquotes
						return false
					}
					construction.Literal = true
				}
			}
			constructions = append(constructions, construction)
		case *ast.SelectorExpr:
			pkg, ok := node.X.(*ast.Ident)
			if !ok || execName == "" || pkg.Name != execName || (node.Sel.Name != "Command" && node.Sel.Name != "CommandContext") {
				return true
			}
			if call, ok := parents[node].(*ast.CallExpr); ok && call.Fun == node {
				return true
			}
			constructions = append(constructions, execConstruction{Path: path, Constructor: node.Sel.Name + "Alias"})
		case *ast.CompositeLit:
			sel, ok := node.Type.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Cmd" {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || execName == "" || pkg.Name != execName {
				return true
			}
			construction := execConstruction{Path: path, Constructor: "Cmd"}
			for _, elt := range node.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, keyOK := kv.Key.(*ast.Ident)
				if !keyOK || key.Name != "Path" {
					continue
				}
				if lit, literal := kv.Value.(*ast.BasicLit); literal && lit.Kind == token.STRING {
					construction.Command, err = strconv.Unquote(lit.Value)
					if err != nil { // coverage-ignore: the parser accepted the literal, so it unquotes
						return false
					}
					construction.Literal = true
				}
			}
			constructions = append(constructions, construction)
		}
		return true
	})
	if err != nil { // coverage-ignore: every parsed string literal unquotes
		return nil, err
	}
	return constructions, nil
}

func astParentMap(root ast.Node) map[ast.Node]ast.Node {
	parents := map[ast.Node]ast.Node{}
	stack := []ast.Node{}
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if len(stack) > 0 {
			parents[node] = stack[len(stack)-1]
		}
		stack = append(stack, node)
		return true
	})
	return parents
}

// isExecConstruction reports whether fun names os/exec's process constructors,
// under whatever local name the file imported the package as.
func isExecConstruction(fun ast.Expr, execName string) bool {
	if execName == "" {
		return false
	}
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return pkg.Name == execName && (sel.Sel.Name == "Command" || sel.Sel.Name == "CommandContext")
}

// importLocalName returns the name a file refers to importPath by: the explicit
// alias when there is one, otherwise the path's last segment. Empty when the
// file does not import it, so a bare identifier match cannot false-positive.
func importLocalName(file *ast.File, importPath string) string {
	for _, imp := range file.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil || p != importPath { // coverage-ignore: the parser accepted every import path in this file
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return path.Base(p)
	}
	return ""
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
//
// Traversal is testsupport.WalkRepoFiles, which is the repository's single
// definition of the repo-walk boundary: it prunes hidden trees and nested
// checkouts. Hand-rolling the walk here was a real defect and not a stylistic
// one - it made both walkers fail in the primary checkout whenever a managed
// worktree was present under .awf/worktrees, which is the normal state during
// an effort and precisely the state integration leaves behind.
func walkGitAccess(t *testing.T, testFiles bool, allowed []string) ([]gitAccessFinding, int) {
	t.Helper()
	root := moduleRoot(t)
	findings, seen := []gitAccessFinding{}, 0
	testsupport.WalkRepoFiles(t, root, func(rel string) bool {
		// A testdata tree holds fixture sources that are never compiled.
		if rel == "testdata" || strings.HasPrefix(rel, "testdata/") || strings.Contains(rel, "/testdata/") {
			return false
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") != testFiles {
			return false
		}
		seen++
		return !allowedGitAccess(rel, allowed)
	}, func(rel string, body []byte) {
		found, err := scanGitAccess(rel, body)
		if err != nil { // coverage-ignore: every non-allowlisted module file compiles, so it parses
			t.Fatal(err)
		}
		findings = append(findings, found...)
	})
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

// TestGitFixtureHasOneNativeGitProcessBoundary closes the fixture carve-out:
// the package may construct Git directly, but exactly one production site may
// do so, forcing every wrapper through the deadlined boundary in native.go.
// invariant: tooling/git-access:fixture-isolation-parity (TestGitFixtureHasOneNativeGitProcessBoundary)
func TestGitFixtureHasOneNativeGitProcessBoundary(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	seen := 0
	constructions := []execConstruction{}
	testsupport.WalkRepoFiles(t, root, func(rel string) bool {
		return strings.HasPrefix(rel, "internal/testsupport/gitfixture/") && strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go")
	}, func(rel string, body []byte) {
		seen++
		found, err := scanExecConstructions(rel, body)
		if err != nil {
			t.Fatal(err)
		}
		constructions = append(constructions, found...)
	})
	if seen == 0 {
		t.Fatal("walked no gitfixture production files, so the census proves nothing")
	}
	want := []execConstruction{
		{Path: "internal/testsupport/gitfixture/native.go", Constructor: "Command", Command: "ssh-keygen", Literal: true},
		{Path: "internal/testsupport/gitfixture/native.go", Constructor: "CommandContext", Command: "git", Literal: true},
	}
	if len(constructions) != len(want) {
		t.Fatalf("gitfixture process constructions = %+v, want exact closed set %+v", constructions, want)
	}
	for i := range want {
		if constructions[i] != want[i] {
			t.Fatalf("gitfixture process construction %d = %+v, want %+v", i, constructions[i], want[i])
		}
	}
}

// invariant: tooling/git-access:fixture-isolation-parity (TestGitDeadlineDeclarationsRemainEqual)
func TestGitDeadlineDeclarationsRemainEqual(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	declarations := []struct {
		path string
		name string
	}{
		{"internal/git/runner.go", "CommandTimeout"},
		{"internal/testsupport/testsupport.go", "gitTestDeadline"},
		{"internal/testsupport/gitfixture/native.go", "nativeGitDeadline"},
	}
	for _, declaration := range declarations {
		expression, err := constExpression(filepath.Join(root, declaration.path), declaration.name)
		if err != nil {
			t.Fatal(err)
		}
		if expression != "2 * time.Minute" {
			t.Errorf("%s %s = %s, want shared two-minute ceiling", declaration.path, declaration.name, expression)
		}
	}
}

func constExpression(filePath, name string) (string, error) {
	body, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, body, 0)
	if err != nil {
		return "", err
	}
	for _, declaration := range file.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			values := spec.(*ast.ValueSpec)
			for index, ident := range values.Names {
				if ident.Name != name || index >= len(values.Values) {
					continue
				}
				var rendered bytes.Buffer
				if err := printer.Fprint(&rendered, fset, values.Values[index]); err != nil {
					return "", err
				}
				return rendered.String(), nil
			}
		}
	}
	return "", fmt.Errorf("constant %s not found in %s", name, filePath)
}

func TestExecConstructionCensusRejectsVariableExecutable(t *testing.T) {
	t.Parallel()
	found, err := scanExecConstructions("variable.go", []byte("package p\nimport \"os/exec\"\nfunc f() { name := \"git\"; _ = exec.Command(name) }\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := execConstruction{Path: "variable.go", Constructor: "Command"}
	if len(found) != 1 || found[0] != want {
		t.Fatalf("variable executable constructions = %+v, want nonliteral finding %+v", found, want)
	}
}

func TestExecConstructionCensusRejectsConstructorAlias(t *testing.T) {
	t.Parallel()
	found, err := scanExecConstructions("alias.go", []byte("package p\nimport \"os/exec\"\nfunc f() { ctor := exec.Command; _ = ctor(\"git\") }\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := execConstruction{Path: "alias.go", Constructor: "CommandAlias"}
	if len(found) != 1 || found[0] != want {
		t.Fatalf("aliased executable constructions = %+v, want escape finding %+v", found, want)
	}
}

// TestNoProductionGitAccessOutsideTheSeam fails when any non-test file in the
// module imports a Git library or builds a git subprocess outside the seam.
// This is the enforcement half of one-implementation-per-entrypoint: without it
// the seam is a convention, and the seven bypassing call sites this effort
// removed could return one at a time without anyone noticing.
// invariant: tooling/git-access:all-access-via-seam (TestNoProductionGitAccessOutsideTheSeam)
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

// TestSeamAllowlistEntriesAreAllLoadBearing mirrors its fixture-side twin: an
// allowlist entry that stops shielding a real finding silently widens the hole
// it was carved for, so each must still be doing work.
func TestSeamAllowlistEntriesAreAllLoadBearing(t *testing.T) {
	t.Parallel()
	for _, entry := range seamAllowlist {
		t.Run(entry, func(t *testing.T) {
			t.Parallel()
			narrowed := []string{}
			for _, keep := range seamAllowlist {
				if keep != entry {
					narrowed = append(narrowed, keep)
				}
			}
			findings, _ := walkGitAccess(t, false, narrowed)
			if len(findings) == 0 {
				t.Errorf("allowlist entry %q shields nothing; remove it rather than leave the carve-out open", entry)
			}
		})
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
	for _, extra := range []struct{ name, src, want string }{
		{
			name: "aliased os/exec import",
			src:  "package p\n\nimport osexec \"os/exec\"\n\nvar _ = osexec.Command(\"git\", \"status\")\n",
			want: "constructs a git subprocess",
		},
		{
			name: "exec.Cmd composite literal",
			src:  "package p\n\nimport \"os/exec\"\n\nvar _ = exec.Cmd{Path: \"git\"}\n",
			want: "constructs a git subprocess",
		},
	} {
		t.Run(extra.name, func(t *testing.T) {
			t.Parallel()
			findings, err := scanGitAccess("probe.go", []byte(extra.src))
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != 1 || !strings.Contains(findings[0].Detail, extra.want) {
				t.Fatalf("findings = %v, want one containing %q", findings, extra.want)
			}
		})
	}
	// A file that never imports os/exec cannot be flagged by a bare identifier
	// that happens to be spelled exec.
	shadowed := "package p\n\ntype t struct{}\n\nfunc (t) Command(string, ...string) any { return nil }\n\nvar exec t\n\nvar _ = exec.Command(\"git\", \"status\")\n"
	if findings, err := scanGitAccess("probe.go", []byte(shadowed)); err != nil || len(findings) != 0 {
		t.Fatalf("findings for a shadowed exec identifier = %v, %v; want none", findings, err)
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
