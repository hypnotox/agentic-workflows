package project

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
)

// invariant: rendering/project-output-plan:kind-dispatch-single-table (TestKindDescriptorsCoverAllKinds)
func TestKindDescriptorsCoverAllKinds(t *testing.T) {
	got := make([]string, len(kindDescriptors))
	for i, d := range kindDescriptors {
		got[i] = d.Plural
	}
	want := []string{"skills", "agents", "docs", "domains"}
	if !slices.Equal(got, want) {
		t.Fatalf("kind set drift: got %v want %v", got, want)
	}
	// Every catalog-backed kind resolves a pool; domains (freeform) must not.
	for _, d := range kindDescriptors {
		hasPool := d.poolNames != nil
		if (d.Plural == "domains") == hasPool {
			t.Errorf("%s poolNames presence wrong (hasPool=%v)", d.Plural, hasPool)
		}
	}
	for _, d := range kindDescriptors {
		wantFreeform := d.Plural == "domains"
		if d.freeformDomain != wantFreeform {
			t.Errorf("%s freeformDomain = %v, want %v", d.Plural, d.freeformDomain, wantFreeform)
		}
	}
	// The exported facet accessors resolve through the table, unknown kinds false.
	for _, c := range []struct {
		kind                string
		freeform, doc, skil bool
	}{
		{"skill", false, false, true},
		{"agent", false, false, false},
		{"doc", false, true, false},
		{"domain", true, false, false},
		{"bogus", false, false, false},
	} {
		if got := IsFreeformDomainKind(c.kind); got != c.freeform {
			t.Errorf("IsFreeformDomainKind(%s) = %v, want %v", c.kind, got, c.freeform)
		}
	}
}

// loadCmdAwfPackage loads the cmd/awf production sources (tests excluded),
// optionally overlaying one file so a negative case can be committed rather
// than hand-mutated. Syntax only: the scan matches string literals, so no type
// information is needed.
func loadCmdAwfPackage(t *testing.T, overlay map[string][]byte) []*packages.Package {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	pkgs, err := packages.Load(&packages.Config{
		Dir:     root,
		Mode:    packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax,
		Overlay: overlay,
	}, "./cmd/awf")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) == 0 {
		t.Fatal("no package loaded for ./cmd/awf")
	}
	for _, pkg := range pkgs {
		if len(pkg.Errors) != 0 {
			t.Fatal(pkg.Errors[0])
		}
	}
	return pkgs
}

// kindLiteralFindings reports every equality or switch-case comparison in the
// loaded sources whose string-literal operand is one of the descriptor table's
// kind names. Such a comparison decides a kind fact outside the table.
func kindLiteralFindings(pkgs []*packages.Package, kindNames map[string]bool) []string {
	isKindLiteral := func(expr ast.Expr) bool {
		lit, ok := expr.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return false
		}
		value, err := strconv.Unquote(lit.Value)
		return err == nil && kindNames[value]
	}
	var findings []string
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.BinaryExpr:
					if node.Op != token.EQL && node.Op != token.NEQ {
						return true
					}
					for _, operand := range []ast.Expr{node.X, node.Y} {
						if isKindLiteral(operand) {
							pos := pkg.Fset.Position(node.Pos())
							findings = append(findings, "equality on a kind literal at "+
								filepath.ToSlash(filepath.Base(pos.Filename))+":"+strconv.Itoa(pos.Line))
						}
					}
				case *ast.CaseClause:
					for _, expr := range node.List {
						if isKindLiteral(expr) {
							pos := pkg.Fset.Position(expr.Pos())
							findings = append(findings, "switch case on a kind literal at "+
								filepath.ToSlash(filepath.Base(pos.Filename))+":"+strconv.Itoa(pos.Line))
						}
					}
				}
				return true
			})
		}
	}
	sort.Strings(findings)
	return findings
}

// TestCmdKindFactsResolveThroughTable proves the widened half of the claim: no
// cmd/awf production source decides a kind fact by comparing against a kind
// name; every such fact reaches the descriptor table through the exported
// accessors, whose bodies live beside the table in this package. The detector
// matches equality and switch-case comparisons only - a kind fact smuggled
// through slices.Contains, a map literal, or a case-folded compare stays
// invisible to it; extend the operand shapes if one ever appears.
// invariant: rendering/project-output-plan:kind-dispatch-single-table (TestCmdKindFactsResolveThroughTable)
func TestCmdKindFactsResolveThroughTable(t *testing.T) {
	kindNames := map[string]bool{}
	for _, d := range kindDescriptors {
		kindNames[d.Singular] = true
		kindNames[d.Plural] = true
	}

	production := loadCmdAwfPackage(t, nil)
	if findings := kindLiteralFindings(production, kindNames); len(findings) != 0 {
		t.Errorf("cmd/awf decides kind facts outside the descriptor table:\n\t%s",
			strings.Join(findings, "\n\t"))
	}

	// Committed negative case: an equality and a switch case on kind literals
	// must both be flagged, so the detector cannot silently stop detecting.
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(root, filepath.FromSlash("cmd/awf/kind_fact_fixture.go"))
	violating := loadCmdAwfPackage(t, map[string][]byte{fixture: []byte(`package main

func fixtureKindEquality(kind string) bool { return kind == "skill" }

func fixtureKindSwitch(kind string) int {
	switch kind {
	case "domains":
		return 1
	}
	return 0
}
`)})
	findings := kindLiteralFindings(violating, kindNames)
	var equalityFlagged, caseFlagged bool
	for _, f := range findings {
		if strings.HasPrefix(f, "equality on a kind literal at kind_fact_fixture.go") {
			equalityFlagged = true
		}
		if strings.HasPrefix(f, "switch case on a kind literal at kind_fact_fixture.go") {
			caseFlagged = true
		}
	}
	if !equalityFlagged {
		t.Errorf("an equality on a kind literal escaped the detector: %#v", findings)
	}
	if !caseFlagged {
		t.Errorf("a switch case on a kind literal escaped the detector: %#v", findings)
	}
}

func TestResidualKindDescriptorProjectsDocTemplateAndUnknownSections(t *testing.T) {
	descriptor, ok := descriptorByPlural("docs")
	if !ok || descriptor.templateID(catalog.Standard, "architecture") != catalog.Standard.Docs["architecture"].TID {
		t.Fatalf("docs descriptor = %#v, %t", descriptor, ok)
	}
	domain, ok := descriptorByPlural("domains")
	if !ok || domain.templateID(catalog.Standard, "example") != "domains/domain.md.tmpl" {
		t.Fatalf("domain descriptor = %#v, %t", domain, ok)
	}
	state := testState(&config.Config{})
	if sections := declaredSections(renderInputsForTest(state), "unknown", "artifact"); sections != nil {
		t.Fatalf("unknown sections = %v", sections)
	}
}

func TestKindLookups(t *testing.T) {
	if got := Kinds(); !slices.Equal(got, []string{"skill", "agent", "doc", "domain"}) {
		t.Fatalf("Kinds() = %v", got)
	}
	if pl, ok := PluralKind("skill"); !ok || pl != "skills" {
		t.Errorf("PluralKind(skill) = %q,%v", pl, ok)
	}
	if _, ok := PluralKind("bogus"); ok {
		t.Error("PluralKind(bogus) should be false")
	}
	if _, ok := descriptorByPlural("bogus"); ok {
		t.Error("descriptorByPlural(bogus) should be false")
	}
}

func descriptorMust(t *testing.T, plural string) kindDescriptor {
	t.Helper()
	d, ok := descriptorByPlural(plural)
	if !ok {
		t.Fatalf("descriptor %q missing", plural)
	}
	return d
}

func TestKindAccessors(t *testing.T) {
	cat := &catalog.Catalog{
		Skills:    map[string]catalog.SkillSpec{"tdd": {Sections: []string{"a"}}},
		Agents:    map[string]catalog.AgentSpec{"rev": {Name: "rev", Description: "reviewer", Sections: []string{"b"}}},
		Docs:      map[string]catalog.DocEntry{"arch": {Sections: []string{"c"}, TID: "docs/arch.md.tmpl"}},
		DomainDoc: catalog.TargetSpec{Sections: []string{"d"}},
	}

	// poolNames facet (via CatalogNames) for every catalog-backed kind, plus the
	// no-pool (domain) and unknown branches.
	for _, c := range []struct{ kind, want string }{
		{"skill", "tdd"}, {"agent", "rev"}, {"doc", "arch"},
	} {
		pool, ok := CatalogNames(cat, c.kind)
		if !ok || !slices.Contains(pool, c.want) {
			t.Errorf("CatalogNames(%s) = %v,%v", c.kind, pool, ok)
		}
	}
	if _, ok := CatalogNames(cat, "domain"); ok {
		t.Error("CatalogNames(domain) should be false (no pool)")
	}
	if _, ok := CatalogNames(cat, "bogus"); ok {
		t.Error("CatalogNames(bogus) should be false")
	}

	// sections facet: catalog-backed kinds report presence; domains keep the
	// singleton's sections but report no per-name presence.
	if s, ok := descriptorMust(t, "skills").sections(cat, "tdd"); !ok || !slices.Equal(s, []string{"a"}) {
		t.Errorf("skills sections = %v,%v", s, ok)
	}
	if s, ok := descriptorMust(t, "agents").sections(cat, "rev"); !ok || !slices.Equal(s, []string{"b"}) {
		t.Errorf("agents sections = %v,%v", s, ok)
	}
	if s, ok := descriptorMust(t, "docs").sections(cat, "arch"); !ok || !slices.Equal(s, []string{"c"}) {
		t.Errorf("docs sections = %v,%v", s, ok)
	}
	if s, ok := descriptorMust(t, "domains").sections(cat, "tooling"); ok || !slices.Equal(s, []string{"d"}) {
		t.Errorf("domains sections = %v,%v", s, ok)
	}

	// outPath facet: skills/agents place adapter artifacts; docs/domains are neutral (nil).
	tgt := Target{SkillDir: ".claude/skills", AgentDir: ".claude/agents"}
	if got := descriptorMust(t, "skills").outPath(tgt, "awf", "tdd"); got != ".claude/skills/awf-tdd/SKILL.md" {
		t.Errorf("skills outPath = %q", got)
	}
	if got := descriptorMust(t, "agents").outPath(tgt, "awf", "rev"); got != ".claude/agents/rev.md" {
		t.Errorf("agents outPath = %q", got)
	}
	for _, pl := range []string{"docs", "domains"} {
		if descriptorMust(t, pl).outPath != nil {
			t.Errorf("%s outPath should be nil", pl)
		}
	}
}
