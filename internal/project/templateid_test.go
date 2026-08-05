package project

import (
	"go/ast"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/hypnotox/agentic-workflows/templates"
)

// templateIDScanPatterns are the production sources the template-ID claim
// scopes itself to: every package awf ships under internal/ and cmd/.
var templateIDScanPatterns = []string{"./internal/...", "./cmd/..."}

// sanctionedTemplateIDFiles are the declaration files template identity is
// allowed to be spelled in, as repository-relative slash paths: the catalog's
// own DocEntry TIDs, the kind-descriptor table, and the two declaration
// tables in the project package (the target descriptors and the non-catalog
// singleton ids). Every other production file resolves an id through one of
// them (ADR-0195 item 5). The consolidation left output_plan.go with zero
// literals, so it is deliberately NOT sanctioned: reintroducing an inline id
// at its historical duplication site must fail this scan, and the per-file
// vacuity check below hard-fails any entry that stops contributing.
var sanctionedTemplateIDFiles = map[string]bool{
	"internal/catalog/standard.go":  true,
	"internal/project/kind.go":      true,
	"internal/project/singleton.go": true,
	"internal/project/target.go":    true,
}

// isTemplateIDLiteral reports whether a string literal spells a full template-ID
// path: it ends in the template suffix and carries a path separator. The
// separator is what distinguishes an id ("hooks/pre-commit.sh.tmpl") from a
// bare suffix used for a string operation (".sh.tmpl" in a TrimSuffix), which
// names no template and is deliberately not a finding.
func isTemplateIDLiteral(lit *ast.BasicLit) (string, bool) {
	if lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil { // coverage-ignore: a parsed STRING literal always unquotes
		return "", false
	}
	if !strings.HasSuffix(value, ".tmpl") || !strings.Contains(value, "/") {
		return "", false
	}
	return value, true
}

// loadTemplateIDPackages loads the production sources of every package under
// internal/ and cmd/ (tests excluded), optionally overlaying one file so a
// negative case can be committed rather than hand-mutated. Syntax only: the
// scan matches string literals, so no type information is needed.
func loadTemplateIDPackages(t *testing.T, overlay map[string][]byte) (string, []*packages.Package) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	pkgs, err := packages.Load(&packages.Config{
		Dir:     root,
		Mode:    packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax,
		Overlay: overlay,
	}, templateIDScanPatterns...)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) == 0 {
		t.Fatalf("no packages loaded for %v", templateIDScanPatterns)
	}
	for _, pkg := range pkgs {
		if len(pkg.Errors) != 0 {
			t.Fatal(pkg.Errors[0])
		}
	}
	// Each pattern half must resolve to real packages: a pattern that silently
	// matched nothing would leave half the claimed scope unscanned while the
	// aggregate check above stayed green.
	var hasInternal, hasCmd bool
	for _, pkg := range pkgs {
		if strings.Contains(pkg.PkgPath, "/internal/") {
			hasInternal = true
		}
		if strings.Contains(pkg.PkgPath, "/cmd/") {
			hasCmd = true
		}
	}
	if !hasInternal || !hasCmd {
		t.Fatalf("a scan pattern matched no packages (internal=%v cmd=%v)", hasInternal, hasCmd)
	}
	return root, pkgs
}

// templateIDFindings partitions every full template-ID literal in the loaded
// production sources into the sanctioned declarations and the second homes.
// The per-file declared counts are returned so the scan can prove it saw every
// declaration file rather than silently matching nothing at all.
func templateIDFindings(root string, pkgs []*packages.Package) (findings []string, declared map[string]int) {
	declared = map[string]int{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok {
					return true
				}
				value, isID := isTemplateIDLiteral(lit)
				if !isID {
					return true
				}
				position := pkg.Fset.Position(lit.Pos())
				rel, err := filepath.Rel(root, position.Filename)
				if err != nil { // coverage-ignore: every loaded production file lies under the module root
					rel = position.Filename
				}
				rel = filepath.ToSlash(rel)
				if sanctionedTemplateIDFiles[rel] {
					declared[rel]++
					return true
				}
				findings = append(findings, "template id "+strconv.Quote(value)+" spelled at "+rel+":"+strconv.Itoa(position.Line))
				return true
			})
		}
	}
	sort.Strings(findings)
	return findings, declared
}

// topicTemplatesImportFindings reports every internal/topic production file
// importing the embedded templates package. The claim's third clause says
// internal/topic receives template identity and content from its caller; a
// regained templates import is the re-read path coming back, and no string
// literal would betray it (fs.ReadFile(templates.FS, tid) spells no id).
func topicTemplatesImportFindings(pkgs []*packages.Package) []string {
	var findings []string
	for _, pkg := range pkgs {
		if !strings.HasSuffix(pkg.PkgPath, "/internal/topic") {
			continue
		}
		for _, file := range pkg.Syntax {
			for _, imp := range file.Imports {
				if strings.Contains(imp.Path.Value, "/templates") {
					pos := pkg.Fset.Position(imp.Pos())
					findings = append(findings, filepath.ToSlash(filepath.Base(pos.Filename))+":"+strconv.Itoa(pos.Line))
				}
			}
		}
	}
	sort.Strings(findings)
	return findings
}

// TestTemplateIDsDeriveFromTheDeclarationTables proves the claim: no production
// file under internal/ or cmd/ outside the declaration tables spells a full
// template-ID path, so an id is always resolved through the catalog entry, the
// kind descriptor, or a declaration table row. The retired
// validateDeclarationPlanParity compared one derivation with itself once the
// consolidation landed; this scan replaces it by forbidding the second
// derivation outright.
//
// The detector matches string literals only - an id assembled from fragments
// that never spell a separator and the suffix together, or read out of data,
// stays invisible to it; extend the shapes if one ever appears.
// invariant: rendering/project-output-plan:template-id-single-derivation (TestTemplateIDsDeriveFromTheDeclarationTables)
func TestTemplateIDsDeriveFromTheDeclarationTables(t *testing.T) {
	root, production := loadTemplateIDPackages(t, nil)
	findings, declared := templateIDFindings(root, production)
	if len(findings) != 0 {
		t.Errorf("template identity has a second home outside the declaration tables:\n\t%s",
			strings.Join(findings, "\n\t"))
	}
	// Vacuity guards: the declaration tables carry many ids, so a scan that
	// matched nothing at all is broken rather than clean, and a sanctioned file
	// that stops contributing is a stale allowlist entry the scan reads over.
	total := 0
	for _, count := range declared {
		total += count
	}
	if total < 20 {
		t.Errorf("sanctioned template-id declarations seen = %d, want at least 20 - the scan is matching nothing", total)
	}
	for file := range sanctionedTemplateIDFiles {
		if declared[file] == 0 {
			t.Errorf("sanctioned file %s declares no template id - remove the stale allowlist entry", file)
		}
	}

	// The claim's third clause: internal/topic receives template identity and
	// content from its caller. A regained templates import is the re-read path.
	if imports := topicTemplatesImportFindings(production); len(imports) != 0 {
		t.Errorf("internal/topic imports the embedded templates package again at %s - it must receive identity and content from its caller",
			strings.Join(imports, ", "))
	}

	// Committed negative case: a bare id constant, an id inside a table row, and
	// an id built by concatenation must all be flagged, while a bare suffix used
	// for a string operation must not, so the detector cannot silently stop
	// detecting or start over-reporting. A regained templates import inside
	// internal/topic must be flagged by the import scan for the same reason.
	topicFixture := filepath.Join(root, filepath.FromSlash("internal/topic/template_import_fixture.go"))
	_, importing := loadTemplateIDPackages(t, map[string][]byte{topicFixture: []byte(`package topic

import "github.com/hypnotox/agentic-workflows/templates"

var fixtureEmbeddedRead = templates.FS
`)})
	if imports := topicTemplatesImportFindings(importing); len(imports) == 0 {
		t.Error("a regained internal/topic templates import escaped the import scan")
	}

	fixture := filepath.Join(root, filepath.FromSlash("internal/project/template_id_fixture.go"))
	_, violating := loadTemplateIDPackages(t, map[string][]byte{fixture: []byte(`package project

import "strings"

const fixtureTemplateID = "hooks/pre-commit.sh.tmpl"

var fixtureTemplateTable = []struct{ Kind, TID string }{
	{"docs", "docs/testing.md.tmpl"},
}

func fixtureTemplateJoin(name string) string { return "skills/" + name + "/SKILL.md.tmpl" }

func fixtureSuffixTrim(name string) string { return strings.TrimSuffix(name, ".sh.tmpl") }
`)})
	fixtureFindings, _ := templateIDFindings(root, violating)
	var flagged []string
	for _, f := range fixtureFindings {
		if strings.Contains(f, "template_id_fixture.go") {
			flagged = append(flagged, f)
		}
	}
	if len(flagged) != 3 {
		t.Errorf("template-id spellings flagged = %d, want 3 (constant, table row, concatenated tail): %#v",
			len(flagged), flagged)
	}
	for _, want := range []string{`"hooks/pre-commit.sh.tmpl"`, `"docs/testing.md.tmpl"`, `"/SKILL.md.tmpl"`} {
		if !slicesContainsSubstring(flagged, want) {
			t.Errorf("template id %s escaped the detector: %#v", want, flagged)
		}
	}
	for _, unwanted := range []string{`".sh.tmpl"`, `"docs"`} {
		if slicesContainsSubstring(flagged, unwanted) {
			t.Errorf("a non-id literal %s was flagged: %#v", unwanted, flagged)
		}
	}

	// A conforming consumer that resolves its id through the tables must NOT be
	// flagged: the rule turns on spelling an id, not on using one.
	_, conforming := loadTemplateIDPackages(t, map[string][]byte{fixture: []byte(`package project

func fixtureConformingTID(name string) string { return mustDescriptor("skills").tid(name) }

func fixtureConformingBase() string { return baseTID("agents") }
`)})
	conformingFindings, _ := templateIDFindings(root, conforming)
	for _, f := range conformingFindings {
		if strings.Contains(f, "template_id_fixture.go") {
			t.Errorf("a conforming consumer was flagged: %q", f)
		}
	}
}

// TestLiveTemplateIDsResolve derives the complete live identity population from
// its existing owners and verifies every live entry resolves in the embedded FS.
// coOwnedRunnerTID is recognition-only and must not enter this population.
func TestLiveTemplateIDsResolve(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	ids := p.liveTemplateIDs()
	for _, descriptor := range kindDescriptors {
		if descriptor.baseTID != "" && !ids[descriptor.baseTID] {
			t.Errorf("kind-derived base template %q is not live", descriptor.baseTID)
		}
		if descriptor.freeformDomain && !ids[descriptor.tid("")] {
			t.Errorf("kind-derived domain template %q is not live", descriptor.tid(""))
		}
	}
	if ids[coOwnedRunnerTID] {
		t.Error("recognition-only runner is live")
	}
	for tid := range ids {
		if _, err := fs.ReadFile(templates.FS, tid); err != nil {
			t.Errorf("live template %q does not resolve: %v", tid, err)
		}
	}

	originalBase := kindDescriptors[0].baseTID
	kindDescriptors[0].baseTID = "missing/kind-base.tmpl"
	baseIDs := p.liveTemplateIDs()
	kindDescriptors[0].baseTID = originalBase
	if !baseIDs["missing/kind-base.tmpl"] {
		t.Error("a missing kind-derived base identity escaped the live population")
	}
	if _, err := fs.ReadFile(templates.FS, "missing/kind-base.tmpl"); err == nil {
		t.Error("missing kind-derived base fixture unexpectedly resolves")
	}

	domainIndex := -1
	for i := range kindDescriptors {
		if kindDescriptors[i].freeformDomain {
			domainIndex = i
			break
		}
	}
	if domainIndex < 0 {
		t.Fatal("no freeform domain descriptor")
	}
	originalDomainTID := kindDescriptors[domainIndex].tid
	kindDescriptors[domainIndex].tid = func(string) string { return "missing/domain.tmpl" }
	domainIDs := p.liveTemplateIDs()
	kindDescriptors[domainIndex].tid = originalDomainTID
	if !domainIDs["missing/domain.tmpl"] {
		t.Error("a missing kind-derived domain identity escaped the live population")
	}
	if _, err := fs.ReadFile(templates.FS, "missing/domain.tmpl"); err == nil {
		t.Error("missing kind-derived domain fixture unexpectedly resolves")
	}

	missing := p.Cat.Docs["architecture"]
	missing.TID = "missing/live-template.tmpl"
	p.Cat.Docs["missing-live-fixture"] = missing
	if _, err := p.OutputPlan(testContext(t)); err == nil || !strings.Contains(err.Error(), "missing/live-template.tmpl") {
		t.Fatalf("missing live template error = %v", err)
	}
}

// slicesContainsSubstring reports whether any finding contains the substring.
func slicesContainsSubstring(findings []string, substring string) bool {
	for _, f := range findings {
		if strings.Contains(f, substring) {
			return true
		}
	}
	return false
}
