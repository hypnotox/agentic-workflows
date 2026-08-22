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
			"validateCommandWiring": 1,
		},
		"checkWithTrackingState": {
			"adrRelatedLinkResult": 1, "checkGeneratedTracking": 1,
			"checkLockedFiles": 1, "deadReferenceResult": 1,
			"deadSkillReferenceResult": 1, "driftProjection": 2,
			"fullProfile": 2, "glossaryResult": 1, "len": 1,
			"lockPath": 1, "pendingADRResult": 1, "pitfallResult": 1,
			"planResult": 1, "planWriteFiles": 1, "sweepConfigTreeResult": 1,
			"tagVocabularyResult": 1, "unusedDataResult": 1,
			"unusedVarResult": 1,
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
		"checkReport":              {"error(propertyAuthority)", "informationItem"},
		"checkWithTrackingState":   {"error(finding.Property)"},
		"sweepConfigTreeResult":    {"errorDrift(propertyReproducibility)"},
		"unusedVarResult":          {"informationDrift"},
		"unusedDataResult":         {"informationDrift"},
		"deadReferenceResult":      {"errorDrift(propertyCorrectness)"},
		"deadSkillReferenceResult": {"errorDrift(propertyCorrectness)"},
		"planResult":               {"errorDrift(propertyAuthority)"},
		"pitfallResult":            {"errorDrift(propertyCorrectness)"},
		"glossaryResult":           {"errorDrift(propertyCorrectness)"},
		"tagVocabularyResult":      {"errorDrift(propertyCorrectness)"},
		"adrRelatedLinkResult":     {"errorDrift(propertyAuthority)"},
		"pendingADRResult":         {"errorDrift(propertyAuthority)"},
		"planArtifactResults":      {"errorDrift(propertyAuthority)", "warning(propertyPlanDetail)"},
		"advisoryResultsWithState": {"informationItem", "warning(propertyHeuristic)", "warning(propertyHeuristic)"},
		"checkGeneratedTracking":   {"error(propertyReproducibility)"},
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

	locked := functions["checkLockedFiles"]
	if locked == nil {
		t.Fatal("locked-output semantic producer is absent")
	}
	lockedCount := 0
	ast.Inspect(locked.Body, func(node ast.Node) bool {
		literal, ok := node.(*ast.CompositeLit)
		if !ok || expressionText(fset, literal.Type) != "lockedFinding" {
			return true
		}
		lockedCount++
		property := ""
		for _, element := range literal.Elts {
			field, ok := element.(*ast.KeyValueExpr)
			if ok && expressionText(fset, field.Key) == "Property" {
				property = expressionText(fset, field.Value)
			}
		}
		if property != "propertyReproducibility" {
			t.Errorf("locked finding %d property = %q, want explicit propertyReproducibility", lockedCount, property)
		}
		return true
	})
	if lockedCount != 11 {
		t.Errorf("locked finding producer count = %d, want 11 explicit sites", lockedCount)
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
