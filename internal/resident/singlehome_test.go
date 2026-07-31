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
// itself to: the sync core and every command binary. internal/git is
// deliberately absent - its ResidentName constants are the git seam's own
// spelling, decided untouched by ADR-0194 item 7 and recorded there as a
// tolerated parallel.
var consumerPatterns = []string{"./internal/project", "./cmd/..."}

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

// residentTableFields is the resident table row's field set. A consumer
// declaring a struct with exactly these fields is re-declaring the table
// shape, which the single-home claim forbids however it spells the names.
var residentTableFields = []string{"Name", "TemplateID"}

// declaresResidentTableShape reports whether a struct type re-declares the
// resident table row: exactly a Name and a TemplateID string field.
func declaresResidentTableShape(st *ast.StructType) bool {
	var fields []string
	for _, field := range st.Fields.List {
		ident, ok := field.Type.(*ast.Ident)
		if !ok || ident.Name != "string" {
			return false
		}
		for _, name := range field.Names {
			fields = append(fields, name.Name)
		}
	}
	sort.Strings(fields)
	return strings.Join(fields, ",") == strings.Join(residentTableFields, ",")
}

// residentSingleHomeFindings reports every place a consumer package spells a
// resident root name or re-declares the resident table shape. Either is a
// second home for policy this package owns.
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
				case *ast.StructType:
					if declaresResidentTableShape(node) {
						findings = append(findings, "resident table shape redeclared at "+at(node.Pos()))
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
// The detector matches string literals and struct-type declarations only - a
// root name assembled at runtime from fragments, or read out of configuration,
// stays invisible to it; extend the shapes if one ever appears.
// invariant: rendering/project-output-plan:resident-policy-single-home
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
	// root path, and a re-declared table must all be flagged, so the detector
	// cannot silently stop detecting.
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(root, filepath.FromSlash("internal/project/resident_policy_fixture.go"))
	violating := loadConsumerPackages(t, map[string][]byte{fixture: []byte(`package project

func fixtureResidentComparison(kind string) bool { return kind == "efforts" }

func fixtureResidentJoin() string { return ".awf" + "/worktrees" }

var fixtureResidentTable = []struct{ Name, TemplateID string }{
	{"efforts", "efforts/gitignore.tmpl"},
}
`)})
	findings := residentSingleHomeFindings(violating, names)
	var nameFlagged, shapeFlagged int
	for _, f := range findings {
		if !strings.Contains(f, "resident_policy_fixture.go") {
			continue
		}
		switch {
		case strings.HasPrefix(f, "resident table shape redeclared"):
			shapeFlagged++
		case strings.HasPrefix(f, "resident root name spelled"):
			nameFlagged++
		}
	}
	// Two bare-name literals (the comparison and the table row) plus the
	// "/worktrees" fragment; the template ID must not add a fourth.
	if nameFlagged != 3 {
		t.Errorf("resident root-name spellings flagged = %d, want 3 (comparison, table row, joined fragment): %#v",
			nameFlagged, findings)
	}
	if shapeFlagged != 1 {
		t.Errorf("resident table shape flagged = %d, want 1: %#v", shapeFlagged, findings)
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
