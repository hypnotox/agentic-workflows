package testsupport_test

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
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
	want := map[string][]string{
		"internal/generatedcheck/generatedcheck.go": {
			`PropertyReproducibility checkresult.Property = "reproducibility"`,
			`Rank: severity.Error, Property: PropertyReproducibility`,
			`Rank: severity.Warn, Property: "heuristic-quality"`,
		},
		"internal/referencecheck/referencecheck.go": {
			`PropertyCorrectness checkresult.Property = "correctness"`,
			`PropertyAuthority checkresult.Property = "authority"`,
			`Rank: severity.Error, Property: property`,
		},
		"internal/plancheck/plancheck.go": {
			`PropertyAuthority checkresult.Property = "authority"`,
			`PropertyDetail checkresult.Property = "plan-detail-quality"`,
			`Rank: rank, Property: property`,
		},
		"internal/pitfallcheck/pitfallcheck.go": {
			`PropertyCorrectness checkresult.Property = "correctness"`,
			`Rank: severity.Error, Property: PropertyCorrectness`,
		},
		"internal/vocabularycheck/vocabularycheck.go": {
			`PropertyCorrectness checkresult.Property = "correctness"`,
			`PropertyHeuristic     checkresult.Property = "heuristic-quality"`,
			`Rank: severity.Error, Property: PropertyCorrectness`,
			`Rank: severity.Warn, Property: PropertyHeuristic`,
		},
		"internal/prosegate/prosegate.go": {
			`Rank: severity.Warn, Property: "prose-restraint"`,
		},
		"internal/memorycite/memorycite.go": {
			`Rank: severity.Error, Property: "effort-memory-citation"`,
		},
		"internal/project/check.go": {
			`Rank: severity.Error, Property: propertyAuthority`,
			`Rank: severity.Warn, Property: propertyHeuristic`,
		},
		"internal/project/currentstate.go": {
			`Rank: severity.Error, Property: propertyCurrentState`,
			`Rank: coverage.Severity, Property: propertyCurrentCoverage`,
			`Rank: severity.Error, Property: propertyPlanArtifact`,
		},
	}
	for path, fragments := range want {
		content, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		for _, fragment := range fragments {
			if got := strings.Count(string(content), fragment); got != 1 {
				t.Errorf("%s classification fragment %q occurs %d times, want exactly 1", path, fragment, got)
			}
		}
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
	for _, forbidden := range []string{"internal/project", "cmd/", "internal/application", "internal/currentstate"} {
		assertImportsExclude(t, aggregator, forbidden)
	}
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

func expressionSource(expression ast.Expr) string {
	var out strings.Builder
	if err := format.Node(&out, token.NewFileSet(), expression); err != nil {
		return "<invalid>"
	}
	return out.String()
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
