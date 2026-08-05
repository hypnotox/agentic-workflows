package severity_test

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func TestRankString(t *testing.T) {
	for _, tc := range []struct {
		rank severity.Rank
		want string
	}{
		{severity.Error, "error"},
		{severity.Warn, "warn"},
	} {
		if got := tc.rank.String(); got != tc.want {
			t.Fatalf("Rank(%d).String() = %q, want %q", tc.rank, got, tc.want)
		}
	}
}

func TestErrorIsZeroValue(t *testing.T) {
	var zero severity.Rank
	if zero != severity.Error {
		t.Fatalf("zero Rank = %v, want Error", zero)
	}
}

// Every rank-bearing surface renders through the one shared type. The source
// census covers every production finding-group producer, including the command
// aggregation which this external package cannot import without a cycle.
// invariant: tooling/audit-commands:severity-single-spelling (TestOneSpellingAcrossEveryRankSurface)
func TestOneSpellingAcrossEveryRankSurface(t *testing.T) {
	for _, tc := range []struct {
		what string
		got  string
		want string
	}{
		{"severity.Error", severity.Error.String(), "error"},
		{"severity.Warn", severity.Warn.String(), "warn"},
		{"audit.Finding", audit.Finding{Severity: severity.Warn}.Severity.String(), "warn"},
		{"topic.CoverageFinding", topic.CoverageFinding{Severity: severity.Error}.Severity.String(), "error"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s renders %q, want %q", tc.what, tc.got, tc.want)
		}
	}

	expected := map[string]int{
		"cmd/awf/check_presentation.go":          2,
		"internal/audit/presentation.go":         2,
		"internal/memorycite/presentation.go":    2,
		"internal/project/check_presentation.go": 1,
		"internal/prosegate/presentation.go":     3,
	}
	found := map[string]int{}
	testsupport.WalkRepoSources(t, testsupport.RepoRoot(t), func(rel string, body []byte) {
		categories, err := reportCategories(rel, body)
		if err != nil {
			t.Errorf("%s: %v", rel, err)
		}
		if categories > 0 {
			found[rel] = categories
		}
	})
	if len(found) != len(expected) {
		t.Fatalf("finding-group producer files = %#v, want %#v", found, expected)
	}
	for path, want := range expected {
		if got := found[path]; got != want {
			t.Errorf("%s category constructions = %d, want %d", path, got, want)
		}
	}
}

func TestReportCategoriesRejectsDynamicLabel(t *testing.T) {
	_, err := reportCategories("dynamic.go", []byte(`package fixture

var label = "errors"
var _ = presentation.ReportCategory{Label: label}
`))
	if err == nil {
		t.Fatal("dynamic category label was accepted")
	}
}

func reportCategories(path string, source []byte) (int, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, source, 0)
	if err != nil {
		return 0, err
	}
	var problems []error
	seen := map[*ast.CompositeLit]struct{}{}
	check := func(literal *ast.CompositeLit) {
		if _, ok := seen[literal]; ok {
			return
		}
		seen[literal] = struct{}{}
		label, err := reportCategoryLabel(literal)
		if err != nil {
			problems = append(problems, err)
			return
		}
		if label != "errors" && label != "warnings" {
			problems = append(problems, fmt.Errorf("ReportCategory label %q, want errors or warnings", label))
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.CompositeLit)
		if !ok {
			return true
		}
		if isReportCategory(literal.Type) {
			check(literal)
		}
		if isReportCategorySlice(literal.Type) {
			for _, element := range literal.Elts {
				if category, ok := element.(*ast.CompositeLit); ok {
					check(category)
				}
			}
		}
		return true
	})
	return len(seen), errors.Join(problems...)
}

func isReportCategory(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "ReportCategory" {
		return false
	}
	qualifier, ok := selector.X.(*ast.Ident)
	return ok && qualifier.Name == "presentation"
}

func isReportCategorySlice(expr ast.Expr) bool {
	slice, ok := expr.(*ast.ArrayType)
	return ok && isReportCategory(slice.Elt)
}

func reportCategoryLabel(literal *ast.CompositeLit) (string, error) {
	for _, element := range literal.Elts {
		field, ok := element.(*ast.KeyValueExpr)
		if !ok {
			return "", errors.New("ReportCategory has an unresolved positional label")
		}
		key, ok := field.Key.(*ast.Ident)
		if !ok || key.Name != "Label" {
			continue
		}
		value, ok := field.Value.(*ast.BasicLit)
		if !ok || value.Kind != token.STRING {
			return "", errors.New("ReportCategory has a dynamic Label")
		}
		label, err := strconv.Unquote(value.Value)
		if err != nil {
			return "", fmt.Errorf("unquote ReportCategory Label: %w", err)
		}
		return label, nil
	}
	return "", errors.New("ReportCategory is missing Label")
}
