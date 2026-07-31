package contextq

import (
	"go/ast"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// corePattern and queryPattern are the two halves the boundary claim is about:
// the sync core whose exported surface must carry no context result vocabulary,
// and this package whose reach into that core must stay inside the seam.
const (
	corePattern  = "./internal/project"
	queryPattern = "./internal/contextq"
	cmdPattern   = "./cmd/awf"
)

// movedVocabulary is the context result vocabulary this package took over
// (ADR-0194 item 8). None of these names may reappear in internal/project's
// exported surface: a re-export would mean the core grew a second home for the
// query's own types, which is exactly what the carve removed. The list spans
// both halves of the move: the names that unexported here, and the
// command-facing names that stayed exported from this package - a core
// re-export of either would be a second home.
var movedVocabulary = []string{
	"ADRArtifactContext", "ADROperationContext", "ADROperationDetail",
	"ArtifactLink", "ArtifactRecord", "ArtifactSnapshot",
	"ContextAuthorityCounts", "ContextClaimImpact", "ContextClassificationCount",
	"ContextDirectory", "ContextEvidence", "ContextExactEntry", "ContextGroup",
	"ContextPathImpact", "ContextPathTopic", "ContextPendingImpact",
	"ContextProvenance", "ContextRelationshipSource", "ContextRelationships",
	"ContextRequestReport", "ContextSelectorImpact", "ContextWarning",
	"DomainRef", "PathClassification", "PendingChange", "RequestStatus",
	"TopicImpact", "UncoveredTopic", "UnownedEntry",
	"ContextResult", "ContextOptions", "ContextFacet", "ContextFacetError",
	"ContextSelection", "UncoveredResult",
}

// seamSurface is every internal/project symbol this package may name: the
// assembled-state value and its two core-side constructors, plus the core
// declaration vocabulary the seam's fields are typed in. ArtifactRole and its
// role constants stay core vocabulary by decision (ADR-0194 item 1); Layout and
// OutputDeclaration type two ContextState fields. The set is exactly what
// production names today, so extending it is a deliberate widening of the seam
// rather than an accident. Test sources are outside the scan and may reach
// further (Project, Open, Version) to build fixtures.
var seamSurface = map[string]bool{
	"ContextState": true, "StagedContextState": true,
	"Layout": true, "OutputDeclaration": true,
	"ArtifactRole": true, "ArtifactConfig": true, "ArtifactLock": true,
	"ArtifactManifest": true, "ArtifactTemplate": true, "ArtifactConventionPart": true,
	"ArtifactAuthoredData": true, "ArtifactTopicMetadata": true, "ArtifactClaimPart": true,
	"ArtifactDecisionRecord": true, "ArtifactManagedOutput": true, "ArtifactProtocolDescriptor": true,
}

// loadBoundaryPackages loads the production sources of one pattern (tests
// excluded), optionally overlaying one file so a negative case can be committed
// rather than hand-mutated. Syntax only: the scan matches declarations and
// selector expressions, so no type information is needed.
func loadBoundaryPackages(t *testing.T, pattern string, overlay map[string][]byte) []*packages.Package {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	pkgs, err := packages.Load(&packages.Config{
		Dir:     root,
		Mode:    packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax,
		Overlay: overlay,
	}, pattern)
	if err != nil {
		t.Fatal(err)
	}
	// A pattern that silently matched nothing would leave its half of the claimed
	// scope unscanned while the findings list stayed reassuringly empty.
	if len(pkgs) == 0 {
		t.Fatalf("no packages loaded for %s", pattern)
	}
	for _, pkg := range pkgs {
		if len(pkg.Errors) != 0 {
			t.Fatal(pkg.Errors[0])
		}
		if len(pkg.Syntax) == 0 {
			t.Fatalf("%s loaded no syntax", pkg.PkgPath)
		}
	}
	return pkgs
}

// exportedNames collects every exported top-level declaration name in pkgs:
// types, functions, methods excluded (a method is reached through its receiver
// type), constants, and variables.
func exportedNames(pkgs []*packages.Package) map[string]string {
	out := map[string]string{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			site := func(node ast.Node) string {
				p := pkg.Fset.Position(node.Pos())
				return filepath.ToSlash(filepath.Base(p.Filename)) + ":" + strconv.Itoa(p.Line)
			}
			for _, decl := range file.Decls {
				switch node := decl.(type) {
				case *ast.FuncDecl:
					if node.Recv == nil && node.Name.IsExported() {
						out[node.Name.Name] = site(node)
					}
				case *ast.GenDecl:
					for _, spec := range node.Specs {
						switch s := spec.(type) {
						case *ast.TypeSpec:
							if s.Name.IsExported() {
								out[s.Name.Name] = site(s)
							}
						case *ast.ValueSpec:
							for _, name := range s.Names {
								if name.IsExported() {
									out[name.Name] = site(name)
								}
							}
						}
					}
				}
			}
		}
	}
	return out
}

// coreVocabularyFindings reports every moved result-vocabulary name that the
// core's exported surface declares again.
func coreVocabularyFindings(pkgs []*packages.Package) []string {
	exported := exportedNames(pkgs)
	var findings []string
	for _, name := range movedVocabulary {
		if site, ok := exported[name]; ok {
			findings = append(findings, "internal/project re-exports context vocabulary "+name+" at "+site)
		}
	}
	sort.Strings(findings)
	return findings
}

// seamBreachFindings reports every internal/project symbol this package's
// production sources name that is not part of the seam surface.
func seamBreachFindings(pkgs []*packages.Package) []string {
	var findings []string
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			alias := ""
			for _, spec := range file.Imports {
				path, err := strconv.Unquote(spec.Path.Value)
				if err != nil || path != "github.com/hypnotox/agentic-workflows/internal/project" {
					continue
				}
				alias = "project"
				if spec.Name != nil {
					alias = spec.Name.Name
				}
			}
			if alias == "" {
				continue
			}
			ast.Inspect(file, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				ident, ok := sel.X.(*ast.Ident)
				if !ok || ident.Name != alias || seamSurface[sel.Sel.Name] {
					return true
				}
				p := pkg.Fset.Position(sel.Pos())
				findings = append(findings, "contextq reaches past the seam for project."+sel.Sel.Name+
					" at "+filepath.ToSlash(filepath.Base(p.Filename))+":"+strconv.Itoa(p.Line))
				return true
			})
		}
	}
	sort.Strings(findings)
	return findings
}

// cmdRenderFindings reports every cmd/awf production function that takes a
// contextq result value and builds text from it itself - the rendering clause
// of the claim coming undone. A conforming command hands the value to this
// package's Render entries or to a JSON encoder; a function that receives a
// result type and calls a fmt print/format helper or strings.Builder is
// rendering it in the wrong home.
func cmdRenderFindings(pkgs []*packages.Package) []string {
	resultTypes := map[string]bool{"ContextResult": true, "UncoveredResult": true}
	takesResult := func(fn *ast.FuncDecl) bool {
		fields := []*ast.Field{}
		if fn.Recv != nil {
			fields = append(fields, fn.Recv.List...)
		}
		if fn.Type.Params != nil {
			fields = append(fields, fn.Type.Params.List...)
		}
		for _, field := range fields {
			expr := field.Type
			if star, ok := expr.(*ast.StarExpr); ok {
				expr = star.X
			}
			if sel, ok := expr.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "contextq" && resultTypes[sel.Sel.Name] {
					return true
				}
			}
		}
		return false
	}
	buildsText := func(fn *ast.FuncDecl) bool {
		found := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if ident, ok := sel.X.(*ast.Ident); ok {
				if ident.Name == "fmt" && (strings.HasPrefix(sel.Sel.Name, "Fprint") || strings.HasPrefix(sel.Sel.Name, "Sprint")) {
					found = true
				}
				if ident.Name == "strings" && sel.Sel.Name == "Builder" {
					found = true
				}
			}
			return true
		})
		return found
	}
	var findings []string
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || !takesResult(fn) || !buildsText(fn) {
					continue
				}
				p := pkg.Fset.Position(fn.Pos())
				findings = append(findings, "cmd/awf renders a contextq result in "+fn.Name.Name+
					" at "+filepath.ToSlash(filepath.Base(p.Filename))+":"+strconv.Itoa(p.Line))
			}
		}
	}
	sort.Strings(findings)
	return findings
}

// TestContextQueryBoundary proves the claim: context assembly, classification,
// projection, and result rendering live here; internal/project's exported
// surface carries no context result vocabulary; and this package reaches core
// state only through the assembled context-state value and its two core-side
// constructors.
//
// The detector is syntactic. It matches exported top-level declarations on the
// core side, qualified selector expressions on the query side, and
// result-typed text-building functions on the cmd side, so a vocabulary type
// smuggled through an exported alias in a third package, a core symbol reached
// by dot-import, or a cmd rendering that hides the result behind a local type
// stays invisible to it; extend the shapes if one ever appears.
// invariant: tooling/context-and-topic:context-query-boundary
func TestContextQueryBoundary(t *testing.T) {
	core := loadBoundaryPackages(t, corePattern, nil)
	if findings := coreVocabularyFindings(core); len(findings) != 0 {
		t.Errorf("the core kept context result vocabulary:\n\t%s", strings.Join(findings, "\n\t"))
	}
	query := loadBoundaryPackages(t, queryPattern, nil)
	if findings := seamBreachFindings(query); len(findings) != 0 {
		t.Errorf("contextq reaches into the core outside the ContextState seam:\n\t%s",
			strings.Join(findings, "\n\t"))
	}
	command := loadBoundaryPackages(t, cmdPattern, nil)
	if findings := cmdRenderFindings(command); len(findings) != 0 {
		t.Errorf("cmd/awf renders contextq results itself:\n\t%s", strings.Join(findings, "\n\t"))
	}

	// Committed negative cases: each half must flag its own violation, so the
	// detector cannot silently stop detecting.
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	coreFixture := filepath.Join(root, filepath.FromSlash("internal/project/context_boundary_fixture.go"))
	violatingCore := loadBoundaryPackages(t, corePattern, map[string][]byte{coreFixture: []byte(`package project

type ContextPathImpact struct{ Classification string }

type DomainRef struct{ Name string }
`)})
	coreFindings := coreVocabularyFindings(violatingCore)
	if len(coreFindings) != 2 {
		t.Errorf("re-exported context vocabulary flagged = %#v, want the two fixture types", coreFindings)
	}

	queryFixture := filepath.Join(root, filepath.FromSlash("internal/contextq/boundary_fixture.go"))
	violatingQuery := loadBoundaryPackages(t, queryPattern, map[string][]byte{queryFixture: []byte(`package contextq

import "github.com/hypnotox/agentic-workflows/internal/project"

func fixtureReachesPastSeam() *project.Loader { return nil }

func fixtureStaysInsideSeam(state project.ContextState) *Query { return New(state) }
`)})
	queryFindings := seamBreachFindings(violatingQuery)
	var breaches int
	for _, f := range queryFindings {
		if strings.Contains(f, "boundary_fixture.go") {
			breaches++
		}
	}
	// Exactly the Loader reach: the ContextState parameter in the same fixture is
	// inside the seam, so the rule turns on which symbol is named, not on naming
	// the package at all.
	if breaches != 1 {
		t.Errorf("seam breaches flagged = %d, want 1 (project.Loader only): %#v", breaches, queryFindings)
	}

	// The cmd half: a function that takes a result and prints it must be
	// flagged; one that hands the result to a JSON encoder must not, so the
	// rule turns on building text, not on touching the type.
	cmdFixture := filepath.Join(root, filepath.FromSlash("cmd/awf/context_render_fixture.go"))
	violatingCmd := loadBoundaryPackages(t, cmdPattern, map[string][]byte{cmdFixture: []byte(`package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/contextq"
)

func fixtureRendersResult(w io.Writer, result contextq.ContextResult) {
	fmt.Fprintln(w, "rendered", result)
}

func fixtureEncodesResult(w io.Writer, result contextq.ContextResult) error {
	return json.NewEncoder(w).Encode(result)
}
`)})
	cmdFindings := cmdRenderFindings(violatingCmd)
	var rendered, encoded bool
	for _, f := range cmdFindings {
		if strings.Contains(f, "fixtureRendersResult") {
			rendered = true
		}
		if strings.Contains(f, "fixtureEncodesResult") {
			encoded = true
		}
	}
	if !rendered {
		t.Errorf("a cmd-side result rendering escaped the detector: %#v", cmdFindings)
	}
	if encoded {
		t.Errorf("a conforming JSON encode was flagged as rendering: %#v", cmdFindings)
	}
}
