package testsupport_test

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
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
			"residentRoots": 1, "root": 1, "trackingInformation": 2,
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
	files := []string{
		"internal/generatedcheck/generatedcheck.go",
		"internal/referencecheck/referencecheck.go",
		"internal/plancheck/plancheck.go",
		"internal/pitfallcheck/pitfallcheck.go",
		"internal/vocabularycheck/vocabularycheck.go",
		"internal/prosegate/prosegate.go",
		"internal/memorycite/memorycite.go",
		"internal/project/check.go",
		"internal/project/currentstate.go",
	}
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
	for _, path := range files {
		got = append(got, findingConstructionCensus(t, root, path)...)
	}
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
	owners := []string{
		"internal/generatedcheck/generatedcheck.go",
		"internal/referencecheck/referencecheck.go",
		"internal/configcheck/configcheck.go",
		"internal/plancheck/plancheck.go",
		"internal/pitfallcheck/pitfallcheck.go",
		"internal/vocabularycheck/vocabularycheck.go",
		"internal/project/currentstate.go",
		"internal/prosegate/prosegate.go",
		"internal/memorycite/memorycite.go",
	}
	for _, owner := range owners {
		assertImportsExclude(t, filepath.Join(root, owner), "internal/repositorycheck")
	}
	aggregator := filepath.Join(root, "internal/repositorycheck/repositorycheck.go")
	assertExactImports(t, aggregator, []string{
		"fmt", "slices",
		"github.com/hypnotox/agentic-workflows/internal/checkresult",
		"github.com/hypnotox/agentic-workflows/internal/manifest",
		"github.com/hypnotox/agentic-workflows/internal/presentation",
		"github.com/hypnotox/agentic-workflows/internal/severity",
	})
	file, err := parser.ParseFile(token.NewFileSet(), aggregator, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		condition, ok := node.(*ast.IfStmt)
		if ok && strings.Contains(expressionSource(condition.Cond), ".Kind") {
			t.Error("RepositoryChecker routes a result destination by evidence Kind")
		}
		return true
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

func findingConstructionCensus(t *testing.T, root, relative string) []string {
	t.Helper()
	path := filepath.Join(root, relative)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, content, 0)
	if err != nil {
		t.Fatal(err)
	}
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
			if isCheckResultFindingType(literal.Type) {
				out = append(out, findingConstruction(t, relative, fn.Name.Name, literal))
				return false
			}
			array, ok := literal.Type.(*ast.ArrayType)
			if !ok || !isCheckResultFindingType(array.Elt) {
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

func isCheckResultFindingType(expression ast.Expr) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == "checkresult" && selector.Sel.Name == "Finding"
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

func assertExactImports(t *testing.T, path string, want []string) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, imp := range file.Imports {
		value, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, value)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s imports = %v, want exact policy-free allowlist %v", path, got, want)
	}
}

func assertImportsExclude(t *testing.T, path, forbidden string) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range file.Imports {
		value, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(value, forbidden) {
			t.Errorf("%s imports forbidden %q", path, value)
		}
	}
}
