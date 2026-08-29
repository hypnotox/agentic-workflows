package resident

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// consumerPatterns are the packages the resident single-home claim scopes
// itself to - the sync core and every command binary,
// a live consumer carved out of the core after the claim was scoped (the proof
// is deliberately stricter than the claim sentence's "internal/project or
// cmd"). internal/git is deliberately absent - its ResidentName constants are
// the git seam's own spelling, decided untouched by ADR-0195 item 7 and
// recorded there as a tolerated parallel.
var consumerPatterns = []string{"./internal/project/...", "./cmd/..."}

// loadConsumerPackages loads the production sources of every package that
// consumes resident policy (tests excluded), optionally overlaying one file so
// a negative case can be committed rather than hand-mutated. Syntax only: the
// scan matches declarations and string literals, so no type information is
// needed.
func loadConsumerPackages(t *testing.T, overlay map[string][]byte) []*packages.Package {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	pkgs, err := packages.Load(&packages.Config{
		Dir:     root,
		Mode:    packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax,
		Overlay: overlay,
	}, consumerPatterns...)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) == 0 {
		t.Fatalf("no packages loaded for %v", consumerPatterns)
	}
	for _, pkg := range pkgs {
		if len(pkg.Errors) != 0 {
			t.Fatal(pkg.Errors[0])
		}
	}
	// Each pattern half must resolve to real packages: a pattern that silently
	// matches nothing would leave half the claimed scope unscanned while the
	// aggregate check above stays green.
	var hasProject, hasCmd bool
	for _, pkg := range pkgs {
		if strings.Contains(pkg.PkgPath, "/internal/project") {
			hasProject = true
		}
		if strings.Contains(pkg.PkgPath, "/cmd/") {
			hasCmd = true
		}
	}
	if !hasProject || !hasCmd {
		t.Fatalf("a consumer pattern matched no packages (project=%v cmd=%v)", hasProject, hasCmd)
	}
	return pkgs
}

// residentNameLiteral reports whether a string literal spells a resident root
// name, in any of the spellings a consumer would reach for: the bare name, a
// leading- or trailing-slash fragment concatenated onto the config dir, or the
// whole config-relative root path. A template ID such as
// "efforts/gitignore.tmpl" is deliberately not a match: template identity has
// its own single-derivation claim, and this one is about the root set.
func residentNameLiteral(names map[string]bool, lit *ast.BasicLit) bool {
	if lit.Kind != token.STRING {
		return false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil { // coverage-ignore: a parsed STRING literal always unquotes
		return false
	}
	value = strings.Trim(value, "/")
	value = strings.TrimPrefix(value, config.DirName+"/")
	return names[value]
}

// declaresResidentTable reports whether a composite literal re-declares the
// resident root table: a slice or array literal that enumerates at least one
// root name, either directly (the table is a name list) or inside a row
// literal (a consumer that pairs the names with its own per-root columns).
// The set is closed here, so a second enumeration of it anywhere else is a
// second home for it whatever shape the consumer gives its rows.
func declaresResidentTable(names map[string]bool, lit *ast.CompositeLit) bool {
	switch lit.Type.(type) {
	case *ast.ArrayType:
	default:
		return false
	}
	for _, element := range lit.Elts {
		switch node := element.(type) {
		case *ast.BasicLit:
			if residentNameLiteral(names, node) {
				return true
			}
		case *ast.CompositeLit:
			for _, column := range node.Elts {
				if basic, ok := column.(*ast.BasicLit); ok && residentNameLiteral(names, basic) {
					return true
				}
			}
		}
	}
	return false
}

// residentSingleHomeFindings reports every place a consumer package spells a
// resident root name or re-declares the resident table. Either is a second
// home for policy this package owns.
func residentSingleHomeFindings(pkgs []*packages.Package, names map[string]bool) []string {
	var findings []string
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			// A struct tag is a serialization name, not resident policy; collect
			// the tags so the literal walk below can skip them.
			tags := map[*ast.BasicLit]bool{}
			ast.Inspect(file, func(n ast.Node) bool {
				if field, ok := n.(*ast.Field); ok && field.Tag != nil {
					tags[field.Tag] = true
				}
				return true
			})
			at := func(pos token.Pos) string {
				p := pkg.Fset.Position(pos)
				return filepath.ToSlash(filepath.Base(p.Filename)) + ":" + strconv.Itoa(p.Line)
			}
			ast.Inspect(file, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.ImportSpec:
					return false
				case *ast.BasicLit:
					if !tags[node] && residentNameLiteral(names, node) {
						findings = append(findings, "resident root name spelled at "+at(node.Pos()))
					}
				case *ast.CompositeLit:
					if declaresResidentTable(names, node) {
						findings = append(findings, "resident table redeclared at "+at(node.Pos()))
					}
				}
				return true
			})
		}
	}
	sort.Strings(findings)
	return findings
}

// TestResidentPolicyHasOneHome proves the claim: the resident-root table, the
// resident-path predicate, and anchored output-path resolution have exactly one
// production home here. No file under internal/project or cmd spells a root
// name or re-declares the table, so every consumer reaches the set through this
// package's exported accessors. internal/git is out of scope by decision.
//
// The detector matches string literals and slice/array composite literals only -
// a root name assembled at runtime from fragments, read out of configuration, or
// buried inside a longer below-root path literal (".awf/efforts/sub") stays
// invisible to it; extend the shapes if one ever appears. The claim's middle
// clause (core consumes the set through the Roots value constructed once at
// project open) is carried by internal/project's state-ownership scanner,
// which pins every Project field, roots included, to construction; this test
// proves the spelling and shape halves.
// invariant: rendering/project-output-plan:resident-policy-single-home (TestResidentPolicyHasOneHome)
func TestResidentPolicyHasOneHome(t *testing.T) {
	names := map[string]bool{}
	for _, name := range RootNames() {
		names[name] = true
	}

	production := loadConsumerPackages(t, nil)
	if findings := residentSingleHomeFindings(production, names); len(findings) != 0 {
		t.Errorf("resident policy has a second home outside internal/resident:\n\t%s",
			strings.Join(findings, "\n\t"))
	}

	// Committed negative case: a root-name comparison, a config-dir-relative
	// root path, and both re-declared table shapes (a bare name list and a
	// row-per-root table) must all be flagged, so the detector cannot silently
	// stop detecting.
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(root, filepath.FromSlash("internal/project/resident_policy_fixture.go"))
	violating := loadConsumerPackages(t, map[string][]byte{fixture: []byte(`package project

func fixtureResidentComparison(kind string) bool { return kind == "efforts" }

func fixtureResidentJoin() string { return ".awf" + "/worktrees" }

var fixtureResidentNames = []string{"efforts", "worktrees"}

var fixtureResidentTable = []struct{ Name, TemplateID string }{
	{"efforts", "efforts/gitignore.tmpl"},
}
`)})
	findings := residentSingleHomeFindings(violating, names)
	var nameFlagged, tableFlagged int
	for _, f := range findings {
		if !strings.Contains(f, "resident_policy_fixture.go") {
			continue
		}
		switch {
		case strings.HasPrefix(f, "resident table redeclared"):
			tableFlagged++
		case strings.HasPrefix(f, "resident root name spelled"):
			nameFlagged++
		}
	}
	// Four bare-name literals (the comparison, both name-list entries, and the
	// table row) plus the "/worktrees" fragment; the template ID must not add a
	// sixth.
	if nameFlagged != 5 {
		t.Errorf("resident root-name spellings flagged = %d, want 5 (comparison, two list entries, table row, joined fragment): %#v",
			nameFlagged, findings)
	}
	// The name list and the row table are each one re-declaration; the row
	// literal nested inside the table must not be counted a third time.
	if tableFlagged != 2 {
		t.Errorf("resident table re-declarations flagged = %d, want 2 (name list, row table): %#v", tableFlagged, findings)
	}

	// A conforming consumer that reaches the set through this package must NOT
	// be flagged: the rule turns on spelling the names, not on using them.
	conforming := loadConsumerPackages(t, map[string][]byte{fixture: []byte(`package project

import "github.com/hypnotox/agentic-workflows/internal/resident"

func fixtureConformingLookup(kind string) bool { return resident.IsResidentKind(kind) }

func fixtureConformingTemplateID() string { return "efforts/gitignore.tmpl" }
`)})
	for _, f := range residentSingleHomeFindings(conforming, names) {
		if strings.Contains(f, "resident_policy_fixture.go") {
			t.Errorf("a conforming consumer was flagged: %q", f)
		}
	}
}
