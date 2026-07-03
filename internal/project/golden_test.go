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

	// A fresh check on the synced tree is clean.
	drift, err := p.Check()
	if err != nil || len(drift) != 0 {
		t.Errorf("expected clean check, got drift=%#v err=%v", drift, err)
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
	if got == manifest.Hash(raw) {
		t.Error("TemplateHash equals raw-source hash; expected expanded-source hash")
	}
}
