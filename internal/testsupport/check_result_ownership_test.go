package testsupport_test

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestOrdinaryCheckProducerCensus inventories the complete working composition.
// A producer can enter the aggregate only through an explicit, reviewed call in
// one of these two composition functions.
func TestOrdinaryCheckProducerCensus(t *testing.T) {
	root := testsupport.RepoRoot(t)
	path := filepath.Join(root, "internal/project/check.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	functions := functionDeclarations(file)
	want := map[string]map[string]int{
		"checkReport": {
			"Compose": 1, "Diagnostics": 1, "Evaluate": 1, "SplitWarnings": 1,
			"ValidateCommandWiring": 1, "advisoryResultsWithState": 1,
			"checkWithTrackingState": 1, "fullProfile": 1, "planArtifactResults": 1,
		},
		"checkWithTrackingState": {
			"Additional": 1, "Findings": 1, "LoadOptional": 1, "Locked": 1,
			"New": 1, "ReadFile": 1, "ResolveOutput": 1, "SplitWarnings": 1,
			"Tracking": 1, "adrRelatedResult": 1, "append": 8, "fullProfile": 2,
			"isNested": 2, "len": 1, "lockPath": 1, "pendingADRResult": 1,
			"pitfallResult": 1, "planResult": 1, "referenceResult": 1,
			"residentRoots": 1, "root": 1, "trackingFindings": 2, "trackingInformation": 2,
		},
	}
	for name, expected := range want {
		fn := functions[name]
		if fn == nil {
			t.Fatalf("ordinary working composition %s is absent", name)
		}
		if got := directCallCounts(fn); !reflect.DeepEqual(got, expected) {
			t.Errorf("%s direct calls = %v, want exact producer composition %v", name, got, expected)
		}
	}
}

// invariant: tooling/cli:check-severity-by-protected-property (TestProducerRankPropertyCensus)
func TestProducerRankPropertyCensus(t *testing.T) {
	root := testsupport.RepoRoot(t)
	want := []string{
		`internal/generatedcheck/generatedcheck.go:GuideSizeAdvisory:severity.Warn|"heuristic-quality"`,
		`internal/generatedcheck/generatedcheck.go:errorFinding:severity.Error|PropertyReproducibility`,
		`internal/memorycite/memorycite.go:Result:severity.Error|"effort-memory-citation"`,
		`internal/pitfallcheck/pitfallcheck.go:finding:severity.Error|PropertyCorrectness`,
		`internal/plancheck/plancheck.go:finding:rank|property`,
		`internal/project/check.go:advisoryResultsWithState:severity.Warn|propertyHeuristic`,
		`internal/project/check.go:pendingADRResult:severity.Error|propertyAuthority`,
		`internal/project/currentstate.go:currentStateResult:coverage.Severity|propertyCurrentCoverage`,
		`internal/project/currentstate.go:currentStateResult:severity.Error|propertyCurrentState`,
		`internal/project/currentstate.go:currentStateResult:severity.Error|propertyPlanArtifact`,
		`internal/prosegate/prosegate.go:Result:severity.Warn|"prose-restraint"`,
		`internal/referencecheck/referencecheck.go:finding:severity.Error|property`,
		`internal/vocabularycheck/vocabularycheck.go:errorFinding:severity.Error|PropertyCorrectness`,
		`internal/vocabularycheck/vocabularycheck.go:warning:severity.Warn|PropertyHeuristic`,
	}
	var got []string
	testsupport.WalkRepoSources(t, root, func(relative string, content []byte) {
		file, qualifier, imported := parsedCheckResultConsumer(t, relative, content)
		if imported {
			got = append(got, findingConstructionCensus(t, relative, file, qualifier)...)
		}
	})
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("production Finding construction census changed:\n got %v\nwant %v", got, want)
	}
}

// invariant: rendering/project-output-plan:check-report-single-plan (TestRepositoryCheckerOwnershipCensus)
// invariant: tooling/cli:check-severity-by-protected-property (TestRepositoryCheckerOwnershipCensus)
// TestRepositoryCheckerOwnershipCensus protects the policy-free aggregation
// boundary: semantic owners cannot depend on their aggregator, and aggregation
// cannot reverse into project or application coordination.
func TestRepositoryCheckerOwnershipCensus(t *testing.T) {
	root := testsupport.RepoRoot(t)
	ownerPackages := map[string]bool{
		"internal/configcheck": true, "internal/generatedcheck": true,
		"internal/memorycite": true, "internal/pitfallcheck": true,
		"internal/plancheck": true, "internal/prosegate": true,
		"internal/referencecheck": true, "internal/vocabularycheck": true,
	}
	aggregatorImports := map[string]bool{}
	testsupport.WalkRepoSources(t, root, func(relative string, content []byte) {
		directory := filepath.ToSlash(filepath.Dir(relative))
		if ownerPackages[directory] || relative == "internal/project/currentstate.go" {
			assertImportsExclude(t, relative, content, "internal/repositorycheck")
		}
		if directory != "internal/repositorycheck" {
			return
		}
		file, err := parser.ParseFile(token.NewFileSet(), relative, content, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range importPaths(t, file) {
			aggregatorImports[imported] = true
		}
		ast.Inspect(file, func(node ast.Node) bool {
			condition, ok := node.(*ast.IfStmt)
			if ok && strings.Contains(expressionSource(condition.Cond), ".Kind") {
				t.Errorf("%s routes a result destination by evidence Kind", relative)
			}
			return true
		})
	})
	assertExactImportSet(t, aggregatorImports, []string{
		"fmt", "slices",
		"github.com/hypnotox/agentic-workflows/internal/checkresult",
		"github.com/hypnotox/agentic-workflows/internal/manifest",
		"github.com/hypnotox/agentic-workflows/internal/presentation",
		"github.com/hypnotox/agentic-workflows/internal/severity",
	})
}

func functionDeclarations(file *ast.File) map[string]*ast.FuncDecl {
	out := map[string]*ast.FuncDecl{}
	for _, declaration := range file.Decls {
		if fn, ok := declaration.(*ast.FuncDecl); ok {
			out[fn.Name.Name] = fn
		}
	}
	return out
}

func directCallCounts(fn *ast.FuncDecl) map[string]int {
	out := map[string]int{}
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch called := call.Fun.(type) {
		case *ast.Ident:
			out[called.Name]++
		case *ast.SelectorExpr:
			out[called.Sel.Name]++
		}
		return true
	})
	return out
}

func parsedCheckResultConsumer(t *testing.T, relative string, content []byte) (*ast.File, string, bool) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), relative, content, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if path != "github.com/hypnotox/agentic-workflows/internal/checkresult" {
			continue
		}
		qualifier := "checkresult"
		if spec.Name != nil {
			qualifier = spec.Name.Name
			if qualifier == "." || qualifier == "_" {
				t.Fatalf("%s imports checkresult with unsupported alias %q", relative, qualifier)
			}
		}
		return file, qualifier, true
	}
	return file, "", false
}

func findingConstructionCensus(t *testing.T, relative string, file *ast.File, qualifier string) []string {
	t.Helper()
	var out []string
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			literal, ok := node.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if isCheckResultFindingType(literal.Type, qualifier) {
				out = append(out, findingConstruction(t, relative, fn.Name.Name, literal))
				return false
			}
			array, ok := literal.Type.(*ast.ArrayType)
			if !ok || !isCheckResultFindingType(array.Elt, qualifier) {
				return true
			}
			for _, element := range literal.Elts {
				finding, ok := element.(*ast.CompositeLit)
				if !ok {
					t.Fatalf("%s:%s has non-literal Finding element", relative, fn.Name.Name)
				}
				out = append(out, findingConstruction(t, relative, fn.Name.Name, finding))
			}
			return false
		})
	}
	return out
}

func isCheckResultFindingType(expression ast.Expr, qualifier string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == qualifier && selector.Sel.Name == "Finding"
}

func findingConstruction(t *testing.T, path, function string, literal *ast.CompositeLit) string {
	t.Helper()
	fields := map[string]string{}
	for _, element := range literal.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := pair.Key.(*ast.Ident)
		if ok && (key.Name == "Rank" || key.Name == "Property") {
			fields[key.Name] = expressionSource(pair.Value)
		}
	}
	if fields["Rank"] == "" || fields["Property"] == "" {
		t.Fatalf("%s:%s Finding omits explicit rank or property", path, function)
	}
	return path + ":" + function + ":" + fields["Rank"] + "|" + fields["Property"]
}

func expressionSource(expression ast.Expr) string {
	var out strings.Builder
	if err := format.Node(&out, token.NewFileSet(), expression); err != nil {
		return "<invalid>"
	}
	return out.String()
}

func importPaths(t *testing.T, file *ast.File) []string {
	t.Helper()
	paths := make([]string, 0, len(file.Imports))
	for _, imported := range file.Imports {
		value, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, value)
	}
	return paths
}

func assertExactImportSet(t *testing.T, gotSet map[string]bool, want []string) {
	t.Helper()
	got := slices.Sorted(maps.Keys(gotSet))
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RepositoryChecker package imports = %v, want exact policy-free allowlist %v", got, want)
	}
}

func assertImportsExclude(t *testing.T, relative string, content []byte, forbidden string) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), relative, content, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range importPaths(t, file) {
		if strings.Contains(value, forbidden) {
			t.Errorf("%s imports forbidden %q", relative, value)
		}
	}
}
