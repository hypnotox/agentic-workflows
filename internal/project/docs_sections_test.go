package project

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// TestDocsSectionParity asserts that for every catalog doc the declared section
// set equals the template's marker-block set, and that each doc renders from
// template defaults with no leaked <no value> token.
// invariant: rendering/guide-and-doc-templates:docs-section-parity (TestDocsSectionParity)
func TestDocsSectionParity(t *testing.T) {
	cat := catalog.Standard
	for name, spec := range cat.Docs {
		if spec.AgentsDoc || spec.Path != "" {
			// Root and structural docs have their own output shape and are covered
			// by their singleton parity tests. Mandatory is sidecar location only.
			continue
		}
		tid := spec.TID
		if tid == "" {
			tid = fmt.Sprintf("docs/%s.md.tmpl", name)
		}
		src, err := fs.ReadFile(templates.FS, tid)
		if err != nil {
			t.Fatalf("read %s: %v", tid, err)
		}
		var markers []string
		for _, s := range render.ParseSections(string(src)) {
			if s.IsSection {
				markers = append(markers, s.Name)
			}
		}
		want := append([]string(nil), spec.Sections...)
		got := append([]string(nil), markers...)
		sort.Strings(want)
		sort.Strings(got)
		if strings.Join(want, ",") != strings.Join(got, ",") {
			t.Errorf("%s: section mismatch: catalog %v vs template markers %v", name, want, got)
		}
		asm, parts := render.Assemble(render.ParseSections(string(src)), nil, render.HTMLComment)
		out, err := render.Execute(asm,
			map[string]any{"prefix": "awf", "vars": map[string]any{},
				"layout": map[string]any{"adrReadme": "docs/decisions/README.md"}, "data": map[string]any{}}, parts, "test")
		if err != nil {
			t.Fatalf("render %s: %v", tid, err)
		}
		if strings.Contains(out, "<no value>") {
			t.Errorf("%s: <no value> leaked into rendered doc", name)
		}
	}
}

// TestDocsSectionParityMembershipUsesOutputShape prevents Mandatory from
// becoming a hidden membership oracle: name-derived docs participate regardless
// of sidecar location, while root and structural docs do not.
func TestDocsSectionParityMembershipUsesOutputShape(t *testing.T) {
	entries := map[string]catalog.DocEntry{
		"root":       {AgentsDoc: true, Mandatory: false},
		"structural": {Path: "structural.md", Mandatory: false},
		"named-root": {Mandatory: true},
		"named-docs": {Mandatory: false},
	}
	got := map[string]bool{}
	for name, entry := range entries {
		if !entry.AgentsDoc && entry.Path == "" {
			got[name] = true
		}
	}
	want := map[string]bool{"named-root": true, "named-docs": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("name-derived parity membership = %v, want %v", got, want)
	}
}

// TestSectionOrphanDetection asserts that a convention part whose section is not
// in the target's catalog-declared set is reported as drift, while a part at a
// genuinely declared section is not. The valid section is read from the live
// catalog so the test stays correct as the taxonomy evolves.
// invariant: rendering/inplace-and-placeholders:section-orphan-flagged (TestSectionOrphanDetection)
func TestSectionOrphanDetection(t *testing.T) {
	cat := catalog.Standard
	valid := cat.Docs["architecture"].Sections[0]
	const orphan = "definitely-not-a-section"
	cfg := "prefix: example\nintegrationBranch: main\n" + sprintfVars("") +
		"skills: []\nagents: []\ndocs:\n  - architecture\n"
	root := scaffoldFiles(t, cfg, map[string]string{
		"docs/parts/architecture/" + valid + ".md":  "## Valid\n\noverride body\n",
		"docs/parts/architecture/" + orphan + ".md": "## Bogus\n\nstray\n",
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	var sawOrphan, sawValid bool
	for _, d := range drift {
		if d.Kind != "orphaned" {
			continue
		}
		switch d.Path {
		case ".awf/docs/parts/architecture/" + orphan + ".md":
			sawOrphan = true
		case ".awf/docs/parts/architecture/" + valid + ".md":
			sawValid = true
		}
	}
	if !sawOrphan {
		t.Errorf("expected orphan drift for undeclared section part %q, got %#v", orphan, drift)
	}
	if sawValid {
		t.Errorf("declared section part %q must not be flagged as orphan, got %#v", valid, drift)
	}
}

// TestAgentsDocSectionParity asserts the agents-doc template's marker-block set
// matches its catalog-declared sections, order-exact. The AgentsDoc entry is
// excluded from both TestDocsSectionParity (Mandatory skip) and
// TestAdrSingletonSectionParity (plainSingletons excludes it), so without this
// test a guide section could half-land with a broken override path (ADR-0069).
// invariant: rendering/guide-and-doc-templates:agents-doc-section-parity (TestAgentsDocSectionParity)
func TestAgentsDocSectionParity(t *testing.T) {
	cat := catalog.Standard
	entry := cat.Docs["agents-doc"]
	src, err := fs.ReadFile(templates.FS, entry.TID)
	if err != nil {
		t.Fatalf("read %s: %v", entry.TID, err)
	}
	var markers []string
	for _, s := range render.ParseSections(string(src)) {
		if s.IsSection {
			markers = append(markers, s.Name)
		}
	}
	if strings.Join(markers, ",") != strings.Join(entry.Sections, ",") {
		t.Errorf("%s markers %v != catalog sections %v", entry.TID, markers, entry.Sections)
	}
}

// TestWorkflowDocChainOrder asserts the workflow doc's default render carries
// the canonical chain string: the ADR step precedes the plan step and the
// resync step is surfaced explicitly.
// invariant: rendering/guide-and-doc-templates:maintainable-code-design-guide (TestWorkflowDocChainOrder)
func TestWorkflowDocChainOrder(t *testing.T) {
	out := renderGolden(t, "docs/workflow.md.tmpl", map[string]any{
		"vars":   map[string]any{},
		"layout": testLayout(),
		"data":   map[string]any{},
	})
	// invariant: rendering/workflow-skill-templates:workflow-chain-adr-before-plan (TestWorkflowDocChainOrder)
	if strings.Index(out, "**ADR**") > strings.Index(out, "**Planning**") {
		t.Errorf("workflow guidance must present ADR before plan:\n%s", out)
	}
	// invariant: rendering/workflow-skill-templates:workflow-chain-surfaces-resync (TestWorkflowDocChainOrder)
	if !strings.Contains(out, "plan↔ADR **resync**") {
		t.Errorf("workflow guidance must surface the resync step:\n%s", out)
	}
	planSelection := []string{"sequencing, coordination, or resumability materially helps", "records and operationalizes approved choices", "rather than inventing speculative structure, checks, or work"}
	for _, want := range planSelection {
		if !strings.Contains(out, want) {
			t.Errorf("workflow plan selection missing %q:\n%s", want, out)
		}
	}
	override, err := os.ReadFile(filepath.Join(repoRootDir(t), ".awf", "parts", "workflow", "chain.md"))
	if err != nil {
		t.Fatalf("read project workflow override: %v", err)
	}
	for _, want := range planSelection {
		if !strings.Contains(string(override), want) {
			t.Errorf("project workflow override missing plan selection %q:\n%s", want, override)
		}
	}
}

// invariant: rendering/guide-and-doc-templates:maintainable-code-design-guide (TestMaintainableCodeDesignGuide)
func TestMaintainableCodeDesignGuide(t *testing.T) {
	entry, ok := catalog.Standard.Docs["maintainable-code-design"]
	if !ok {
		t.Fatal("maintainable-code-design catalog entry missing")
	}
	wantSections := []string{"decision-posture", "contextual-heuristics", "semantic-modeling", "readability", "boundaries-and-dependencies", "pattern-toolbox", "preparatory-refactoring", "failure-modes"}
	if !entry.Mandatory || !entry.DocumentMap || entry.Title != "Maintainable Code Design" || entry.Desc != "decision framework for cohesive models, explicit boundaries, dependencies, refactoring, and testable design" || entry.Path != "maintainable-code-design.md" || entry.TemplateKey != "maintainableCodeDesign" || entry.TID != "docs/maintainable-code-design.md.tmpl" || strings.Join(entry.Sections, ",") != strings.Join(wantSections, ",") {
		t.Errorf("catalog entry = %#v, want mandatory document-map guide with sections %v", entry, wantSections)
	}

	src, err := fs.ReadFile(templates.FS, entry.TID)
	if err != nil {
		t.Fatalf("read %s: %v", entry.TID, err)
	}
	var markers []string
	for _, section := range render.ParseSections(string(src)) {
		if section.IsSection {
			markers = append(markers, section.Name)
		}
	}
	if strings.Join(markers, ",") != strings.Join(wantSections, ",") {
		t.Errorf("markers = %v, want %v", markers, wantSections)
	}

	out := renderGolden(t, entry.TID, map[string]any{"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}})
	assertNoLeaks(t, out)
	for _, want := range append([]string{"# Maintainable Code Design", "SOLID", "DRY", "YAGNI", "Strategy", "Adapter", "without mechanically adding wrapper types", "Make the simplest sufficient solution the default", "Added abstraction, indirection, validation, test machinery, tooling, cleanup, or process must be justified by", "requested behavior", "reproduced defect", "existing documented contract", "clearly applicable project invariant", "Generic robustness, hypothetical future use, and the mere possibility of doing more are insufficient"}, append([]string{"## Decision posture", "## SOLID, DRY, and YAGNI", "## Semantic modeling", "## Readability", "## Boundaries and dependency direction", "## Illustrative pattern toolbox", "## Preparatory refactoring", "## Failure modes"}, []string{"perform it first", "include it in the current effort", "defer it in a durable project-owned record", "decline it with the trade-off stated"}...)...) {
		if !strings.Contains(out, want) {
			t.Errorf("guide missing %q:\n%s", want, out)
		}
	}
	for _, forbidden := range []string{"./x", "github.com/hypnotox/agentic-workflows", "internal/", ".go", "Go package", "<no value>", "{{", "}}", "awf:section", "awf:end"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("guide leaked %q:\n%s", forbidden, out)
		}
	}

	root := scaffold(t, "prefix: example\nintegrationBranch: main\nskills: []\nagents: []\ndocs: []\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs/maintainable-code-design.md")); err != nil {
		t.Errorf("maintainable-code guide not written: %v", err)
	}
	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	line := "- **Maintainable Code Design:** [docs/maintainable-code-design.md](docs/maintainable-code-design.md), decision framework for cohesive models, explicit boundaries, dependencies, refactoring, and testable design"
	if !strings.Contains(string(agents), line) {
		t.Errorf("AGENTS.md missing mandatory guide document-map entry %q:\n%s", line, agents)
	}
}

// invariant: rendering/catalog-and-targets:adr-singleton-section-parity (TestAdrSingletonSectionParity)
func TestAdrSingletonSectionParity(t *testing.T) {
	cat := catalog.Standard
	lay := testLayout()
	for _, sg := range plainSingletons {
		src, err := fs.ReadFile(templates.FS, sg.tid)
		if err != nil {
			t.Fatalf("read %s: %v", sg.tid, err)
		}
		var markers []string
		for _, s := range render.ParseSections(string(src)) {
			if s.IsSection {
				markers = append(markers, s.Name)
			}
		}
		wantSections := sg.sections(cat)
		if strings.Join(markers, ",") != strings.Join(wantSections, ",") {
			t.Errorf("%s markers %v != catalog sections %v", sg.tid, markers, wantSections)
		}
		asm, parts := render.Assemble(render.ParseSections(string(src)), nil, render.HTMLComment)
		out, err := render.Execute(asm, map[string]any{
			"prefix": "awf", "vars": map[string]any{}, "layout": lay, "data": map[string]any{}}, parts, "test")
		if err != nil {
			t.Fatalf("render %s: %v", sg.tid, err)
		}
		if strings.Contains(out, "<no value>") {
			t.Errorf("%s: <no value> leaked", sg.tid)
		}
	}
}
