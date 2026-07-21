package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/templates"
)

func TestEndToEndGolden(t *testing.T) {
	assertV2ADRTemplatePublicationSafe(t)
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}

	agent, err := os.ReadFile(filepath.Join(root, ".claude/agents/code-reviewer.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agent), "Independent fresh-context reviewer for example") {
		t.Errorf("agent not interpolated:\n%s", agent)
	}

	// The review-discipline spine is spliced in from templates/partials via awf:include
	// (ADR-0052); its content must appear in the fully rendered agent.
	for _, want := range []string{"## Classification rules", "## Dedup rule", "Impl review complete"} {
		if !strings.Contains(string(agent), want) {
			t.Errorf("spine partial not spliced: missing %q in:\n%s", want, agent)
		}
	}

	plansReadme, err := os.ReadFile(filepath.Join(root, "docs/plans/README.md"))
	if err != nil {
		t.Fatalf("plans-readme not rendered: %v", err)
	}
	if !strings.Contains(string(plansReadme), "Implementation Plans") {
		t.Errorf("plans-readme not interpolated:\n%s", plansReadme)
	}

	// The plans-template singleton renders the ADR-0097 taxonomy, narrowed to the
	// three-field header by ADR-0108: frontmatter spine + canonical headings,
	// section-assembly markers stripped, no unresolved template value.
	// invariant: adr-system/plan-artifacts:plans-template-taxonomy
	plansTemplate, err := os.ReadFile(filepath.Join(root, "docs/plans/template.md"))
	if err != nil {
		t.Fatalf("plans-template not rendered: %v", err)
	}
	for _, want := range []string{
		"date:", "adrs:", "status:",
		"# Plan:", "## Goal", "## Architecture summary",
		"## File structure", "## Phase", "## Verification", "## Notes",
	} {
		if !strings.Contains(string(plansTemplate), want) {
			t.Errorf("plans-template missing taxonomy element %q:\n%s", want, plansTemplate)
		}
	}
	for _, bad := range []string{"awf:section", "awf:end", "{{", "}}"} {
		if strings.Contains(string(plansTemplate), bad) {
			t.Errorf("plans-template leaked marker/token %q:\n%s", bad, plansTemplate)
		}
	}
	// ADR-0108: the gate reference interpolates the configured gateCmd (the
	// fixture sets `make gate`), never a hard-coded "the gate" literal.
	if !strings.Contains(string(plansTemplate), "make gate") {
		t.Errorf("plans-template did not interpolate gateCmd:\n%s", plansTemplate)
	}
	if strings.Contains(string(plansTemplate), "the gate") {
		t.Errorf("plans-template leaked hard-coded gate literal:\n%s", plansTemplate)
	}

	// A fresh check on the synced tree is clean.
	drift, err := p.Check()
	if err != nil || len(drift) != 0 {
		t.Errorf("expected clean check, got drift=%#v err=%v", drift, err)
	}
}

func assertV2ADRTemplatePublicationSafe(t *testing.T) {
	t.Helper()
	out := renderGolden(t, "adr-template/template.md.tmpl", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "skills": map[string]bool{}, "layout": testLayout(),
	})
	implementing := strings.Index(out, "Implementing; content-sha256")
	applied := strings.Index(out, "Applied; state-sequence")
	history := strings.Index(out, "## Status history\n")
	if !strings.Contains(out, "format: current-state-v2") || implementing < 0 || applied < implementing || history < applied {
		t.Fatalf("V2 lifecycle example is not publication-safe:\n%s", out)
	}
	tail := out[history:]
	if strings.Count(tail, "- YYYY-MM-DD:") != 1 || !strings.Contains(tail, "- YYYY-MM-DD: Proposed") {
		t.Fatalf("fresh Proposed Status history contains non-Proposed events:\n%s", tail)
	}
	for _, residue := range []string{"<no value>", "{{", "format: current-state-v1"} {
		if strings.Contains(out, residue) {
			t.Fatalf("empty-data V2 template contains %q:\n%s", residue, out)
		}
	}
}

func TestTemplateHashCoversExpandedSource(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	const tid = "agents/code-reviewer.md.tmpl"
	var got string
	for _, f := range files {
		if f.TemplateID == tid {
			got = f.TemplateHash
		}
	}
	if got == "" {
		t.Fatal("code-reviewer not rendered")
	}
	raw, err := fs.ReadFile(templates.FS, tid)
	if err != nil {
		t.Fatal(err)
	}
	// code-reviewer.md.tmpl carries awf:include directives, so its expanded source differs
	// from its raw bytes; TemplateHash must be over the expanded source (ADR-0052). A
	// regression to manifest.Hash(src) would make these equal.
	// invariant: rendering/render-engine:include-in-templatehash
	if got == manifest.Hash(raw) {
		t.Error("TemplateHash equals raw-source hash; expected expanded-source hash")
	}
}
