package testsupport_test

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestOrdinaryCheckProducerCensus makes the Phase 1 conversion mutation
// sensitive. It inventories the complete working composition and every
// semantic constructor call, so a new producer, bypass, or property change
// must declare its classification here rather than inherit consumer policy.
func TestOrdinaryCheckProducerCensus(t *testing.T) {
	root := testsupport.RepoRoot(t)
	fset := token.NewFileSet()
	path := filepath.Join(root, "internal/project/check.go")
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	functions := functionDeclarations(file)

	wantCompositionCalls := map[string]map[string]int{
		"checkReport": {
			"advisoryResultsWithState": 1, "checkWithTrackingState": 1,
			"fullProfile": 1, "knownDynamicPlanDiagnosticCategory": 1,
			"planArtifactResults": 1, "reportFromBatch": 1,
		},
		"checkWithTrackingState": {
			"adrRelatedResult": 1, "append": 1, "fullProfile": 2,
			"glossaryResult": 1, "len": 1, "lockPath": 1,
			"pendingADRResult": 1, "pitfallResult": 1, "planResult": 1,
			"projectCatalog": 1, "referenceResult": 1, "tagVocabularyResult": 1,
		},
	}
	for name, want := range wantCompositionCalls {
		fn := functions[name]
		if fn == nil {
			t.Fatalf("ordinary working composition %s is absent", name)
		}
		if got := directCallCounts(fn); !reflect.DeepEqual(got, want) {
			t.Errorf("%s direct calls = %v, want exact producer composition %v", name, got, want)
		}
	}

	wantSemanticCalls := map[string][]string{
		"checkReport": {"error(propertyAuthority)", "informationItem"},
		"planResult":  {"errorDrift(propertyAuthority)"}, "pitfallResult": {"errorDrift(propertyCorrectness)"},
		"glossaryResult": {"errorDrift(propertyCorrectness)"}, "tagVocabularyResult": {"errorDrift(propertyCorrectness)"},
		"pendingADRResult":         {"errorDrift(propertyAuthority)"},
		"planArtifactResults":      {"errorDrift(propertyAuthority)", "warning(propertyPlanDetail)"},
		"advisoryResultsWithState": {"informationItem", "warning(propertyHeuristic)"},
	}
	gotSemanticCalls := map[string][]string{}
	for name, fn := range functions {
		calls := semanticCalls(fset, fn)
		if len(calls) == 0 {
			continue
		}
		sort.Strings(calls)
		gotSemanticCalls[name] = calls
	}
	for _, calls := range wantSemanticCalls {
		sort.Strings(calls)
	}
	if !reflect.DeepEqual(gotSemanticCalls, wantSemanticCalls) {
		t.Errorf("semantic producer calls = %#v, want exact census %#v", gotSemanticCalls, wantSemanticCalls)
	}

	for _, forbidden := range []string{"checkLockedFiles", "checkDeadRefs", "checkDeadSkillRefs", "checkADRRelatedLinks", "sweepConfigTree", "unusedVarDrift", "unusedDataDrift", "validateCommandWiring"} {
		if functions[forbidden] != nil {
			t.Errorf("project retains duplicate %s policy", forbidden)
		}
	}
	for _, owner := range []string{"internal/generatedcheck/generatedcheck.go", "internal/referencecheck/referencecheck.go", "internal/configcheck/configcheck.go"} {
		ownerFile, err := parser.ParseFile(fset, filepath.Join(root, owner), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range ownerFile.Imports {
			if imp.Path.Value == `"github.com/hypnotox/agentic-workflows/internal/project"` {
				t.Errorf("owner %s imports project", owner)
			}
		}
	}
	referenceFile, err := parser.ParseFile(fset, filepath.Join(root, "internal/referencecheck/referencecheck.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range referenceFile.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "Check" {
			continue
		}
		if len(fn.Type.Params.List) != 5 {
			t.Errorf("ReferenceChecker Check parameter count = %d, want semantic plan/prefix/sets/existence only", len(fn.Type.Params.List))
		}
	}
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
		if ident, ok := call.Fun.(*ast.Ident); ok {
			out[ident.Name]++
		}
		return true
	})
	return out
}

func semanticCalls(fset *token.FileSet, fn *ast.FuncDecl) []string {
	var out []string
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch selector.Sel.Name {
		case "error", "errorDrift", "warning":
			property := "<missing>"
			if len(call.Args) > 0 {
				property = expressionText(fset, call.Args[0])
			}
			out = append(out, selector.Sel.Name+"("+property+")")
		case "informationItem", "informationDrift":
			out = append(out, selector.Sel.Name)
		}
		return true
	})
	return out
}

func expressionText(fset *token.FileSet, expression ast.Expr) string {
	var out bytes.Buffer
	if err := format.Node(&out, fset, expression); err != nil {
		return "<invalid>"
	}
	return out.String()
}
