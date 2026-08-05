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

func TestReportCategoriesResolvesPresentationAndLocalAliases(t *testing.T) {
	categories, err := reportCategories("aliases.go", []byte(`package fixture

import output "github.com/hypnotox/agentic-workflows/internal/presentation"

type category = output.ReportCategory
type categoryAlias = category
type categories = []categoryAlias

var _ = categories{{Label: "errors"}}
`))
	if err != nil || categories != 1 {
		t.Fatalf("aliases: categories=%d err=%v, want 1 and no error", categories, err)
	}
}

func TestReportCategoriesRejectsUnresolvedCategoryForms(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{"presentation-import-alias", `package fixture
import output "github.com/hypnotox/agentic-workflows/internal/presentation"
var _ = output.ReportCategory{Label: "invalid"}`},
		{"local-type-alias", `package fixture
import output "github.com/hypnotox/agentic-workflows/internal/presentation"
type category = output.ReportCategory
var _ = category{Label: "invalid"}`},
		{"missing-label", `package fixture
import "github.com/hypnotox/agentic-workflows/internal/presentation"
var _ = presentation.ReportCategory{}`},
		{"invalid-label", `package fixture
import "github.com/hypnotox/agentic-workflows/internal/presentation"
var _ = presentation.ReportCategory{Label: "invalid"}`},
		{"dynamic-label", `package fixture
import "github.com/hypnotox/agentic-workflows/internal/presentation"
var label = "errors"
var _ = presentation.ReportCategory{Label: label}`},
		{"positional-construction", `package fixture
import "github.com/hypnotox/agentic-workflows/internal/presentation"
var _ = presentation.ReportCategory{"errors"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			categories, err := reportCategories(tc.name+".go", []byte(tc.source))
			if err == nil || categories != 1 {
				t.Fatalf("categories=%d err=%v, want 1 and a rejection", categories, err)
			}
		})
	}
}

type reportCategoryType int

const (
	notReportCategory reportCategoryType = iota
	reportCategory
	reportCategorySlice
)

func reportCategories(path string, source []byte) (int, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, source, 0)
	if err != nil {
		return 0, err
	}
	types := reportCategoryTypes(file)
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
		switch reportCategoryTypeOf(literal.Type, types) {
		case notReportCategory:
		case reportCategory:
			check(literal)
		case reportCategorySlice:
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

func reportCategoryTypes(file *ast.File) map[string]reportCategoryType {
	imports := map[string]bool{}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path != "github.com/hypnotox/agentic-workflows/internal/presentation" {
			continue
		}
		name := "presentation"
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = true
	}
	types := map[string]reportCategoryType{}
	for name := range imports {
		if name != "." {
			types[name] = notReportCategory
		}
	}
	if imports["."] {
		types["ReportCategory"] = reportCategory
	}
	for changed := true; changed; {
		changed = false
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE {
				continue
			}
			for _, spec := range general.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || !typeSpec.Assign.IsValid() || types[typeSpec.Name.Name] != notReportCategory {
					continue
				}
				if kind := reportCategoryTypeOf(typeSpec.Type, types); kind != notReportCategory {
					types[typeSpec.Name.Name] = kind
					changed = true
				}
			}
		}
	}
	return types
}

func reportCategoryTypeOf(expr ast.Expr, types map[string]reportCategoryType) reportCategoryType {
	if selector, ok := expr.(*ast.SelectorExpr); ok && selector.Sel.Name == "ReportCategory" {
		if qualifier, ok := selector.X.(*ast.Ident); ok && types[qualifier.Name] == notReportCategory {
			_, imported := types[qualifier.Name]
			if imported {
				return reportCategory
			}
		}
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return types[ident.Name]
	}
	if slice, ok := expr.(*ast.ArrayType); ok && reportCategoryTypeOf(slice.Elt, types) == reportCategory {
		return reportCategorySlice
	}
	return notReportCategory
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
