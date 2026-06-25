package project

import (
	"fmt"
	"io/fs"
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
// invariant: docs-section-parity
func TestDocsSectionParity(t *testing.T) {
	cat, err := catalog.Load(templates.FS)
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	for name, spec := range cat.Docs {
		tid := fmt.Sprintf("docs/%s.md.tmpl", name)
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
		out, err := render.Render(string(src), nil,
			func(string) (string, error) { return "", nil },
			map[string]any{"prefix": "awf", "vars": map[string]any{}, "data": map[string]any{}})
		if err != nil {
			t.Fatalf("render %s: %v", tid, err)
		}
		if strings.Contains(out, "<no value>") {
			t.Errorf("%s: <no value> leaked into rendered doc", name)
		}
	}
}

// TestSectionOrphanDetection asserts that a convention part whose section is not
// in the target's catalog-declared set is reported as drift, while a part at a
// genuinely declared section is not. The valid section is read from the live
// catalog so the test stays correct as the taxonomy evolves.
// invariant: section-orphan-flagged
func TestSectionOrphanDetection(t *testing.T) {
	cat, err := catalog.Load(templates.FS)
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	valid := cat.Docs["architecture"].Sections[0]
	const orphan = "definitely-not-a-section"
	cfg := "prefix: example\n" + sprintfVars("") +
		"skills: []\nagents: []\nhooks: []\ndocs:\n  - architecture\n"
	root := scaffoldFiles(t, cfg, map[string]string{
		"docs/parts/architecture/" + valid + ".md":  "## Valid\n\noverride body\n",
		"docs/parts/architecture/" + orphan + ".md": "## Bogus\n\nstray\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	var sawOrphan, sawValid bool
	for _, d := range drift {
		if d.Kind != "orphaned" {
			continue
		}
		switch d.Path {
		case ".claude/awf/docs/parts/architecture/" + orphan + ".md":
			sawOrphan = true
		case ".claude/awf/docs/parts/architecture/" + valid + ".md":
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
