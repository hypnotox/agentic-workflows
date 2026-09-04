package project

import (
	"errors"
	"fmt"
	"io/fs"
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
		for _, s := range parseSections(string(src)) {
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
		asm, parts := assemble(parseSections(string(src)))
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

// invariant: rendering/inplace-and-placeholders:section-orphan-flagged (TestSectionOrphanDetection)
func TestSectionOrphanDetection(t *testing.T) {
	valid := catalog.Standard.Docs["architecture"].Sections[0]
	const orphan = "definitely-not-a-section"
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\n"+sprintfVars(""), map[string]string{
		"docs/parts/architecture/" + valid + ".md":  "## Valid\n\noverride body\n",
		"docs/parts/architecture/" + orphan + ".md": "## Bogus\n\nstray\n",
	})
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	drift, err := checkProject(p, testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	var sawOrphan, sawValid bool
	for _, finding := range drift {
		if finding.Kind != "orphaned" {
			continue
		}
		switch finding.Path {
		case ".awf/docs/parts/architecture/" + orphan + ".md":
			sawOrphan = true
		case ".awf/docs/parts/architecture/" + valid + ".md":
			sawValid = true
		}
	}
	if !sawOrphan || sawValid {
		t.Fatalf("section drift = %#v", drift)
	}
}

// invariant: rendering/guide-and-doc-templates:agents-doc-section-parity (TestAgentsDocSectionParity)
func TestAgentsDocSectionParity(t *testing.T) {
	cat := catalog.Standard
	entry := cat.Docs["agents-doc"]
	src, err := fs.ReadFile(templates.FS, entry.TID)
	if err != nil {
		t.Fatalf("read %s: %v", entry.TID, err)
	}
	var markers []string
	for _, s := range parseSections(string(src)) {
		if s.IsSection {
			markers = append(markers, s.Name)
		}
	}
	if strings.Join(markers, ",") != strings.Join(entry.Sections, ",") {
		t.Errorf("%s markers %v != catalog sections %v", entry.TID, markers, entry.Sections)
	}
}

func TestMaintainableCodeDesignRetiredToExternalSkillAuthority(t *testing.T) {
	if entry, ok := catalog.Standard.Docs["maintainable-code-design"]; ok {
		t.Fatalf("retired maintainable-code-design catalog entry remains: %#v", entry)
	}
	if _, err := fs.Stat(templates.FS, "docs/maintainable-code-design.md.tmpl"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("retired maintainable-code-design template still owned by AWF: %v", err)
	}

	out := renderGolden(t, "agents-doc/AGENTS.md.tmpl", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{},
	})
	for _, want := range []string{
		"Globally installed `agentic-*` skills govern general context, brainstorming, debugging, code design, planning, implementation, and review.",
		"use `agentic-code-design` only for structural questions raised by agreed behavior",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("AGENTS.md missing external code-design authority %q:\n%s", want, out)
		}
	}
	for _, retired := range []string{"Maintainable Code Design", "docs/maintainable-code-design.md"} {
		if strings.Contains(out, retired) {
			t.Errorf("AGENTS.md document map retains retired AWF ownership %q:\n%s", retired, out)
		}
	}
}
