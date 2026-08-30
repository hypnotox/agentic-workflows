package publisher

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

func TestADRReadmeDecisionRouting(t *testing.T) {
	out := renderGolden(t, "adr-readme/README.md.tmpl", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "skills": map[string]bool{}, "layout": testLayout(),
	})
	for _, want := range []string{
		"remains meaningful after implementation",
		"post-implementation",
		"counterfactual",
		"user explicitly accepted that the mechanism itself is load-bearing and the record explains why it is load-bearing",
		"Implementation plan",
		"rollout inventories",
		"proof transactions",
		"Historical ADRs remain unchanged",
		"explicitly accepted",
		"narrowest durable commitment",
		"outside the ADR until accepted",
		"Implementation plan or direct execution",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ADR README missing decision-routing contract %q:\n%s", want, out)
		}
	}
	for _, residue := range []string{"<no value>", "{{", "}}"} {
		if strings.Contains(out, residue) {
			t.Errorf("ADR README contains publication residue %q:\n%s", residue, out)
		}
	}
}

func TestEndToEndGolden(t *testing.T) {
	assertV3ADRTemplatePublicationSafe(t)
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}

	agent, err := os.ReadFile(filepath.Join(root, ".claude/agents/code-reviewer.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agent), "Independent fresh-context reviewer for example") {
		t.Errorf("agent not interpolated:\n%s", agent)
	}

	proposingADR, err := os.ReadFile("../../.pi/skills/awf-proposing-adr/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(proposingADR), "preserve exactly the frontmatter emitted by `./awf new adr`") {
		t.Errorf("project proposing skill lost scaffold authority:\n%s", proposingADR)
	}
	adrReviewer, err := os.ReadFile("../../.pi/agents/adr-reviewer.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"post-implementation", "counterfactual", "reasoned finding"} {
		if !strings.Contains(string(adrReviewer), want) {
			t.Errorf("project ADR reviewer missing semantic routing %q:\n%s", want, adrReviewer)
		}
	}
	if strings.Contains(string(adrReviewer), "## Doc-currency checklist") {
		t.Errorf("project ADR reviewer retains implementation-inventory checklist:\n%s", adrReviewer)
	}

	// The review-discipline spine is spliced in from templates/partials via awf:include
	// (ADR-0052); its content must appear in the fully rendered agent.
	for _, want := range []string{"## Classification rules", "## Dedup rule", "Impl review complete"} {
		if !strings.Contains(string(agent), want) {
			t.Errorf("spine partial not spliced: missing %q in:\n%s", want, agent)
		}
	}

	adrReadme, err := os.ReadFile(filepath.Join(root, "docs/decisions/README.md"))
	if err != nil {
		t.Fatalf("adr-readme not rendered: %v", err)
	}
	if !strings.Contains(string(adrReadme), "remains meaningful after implementation") {
		t.Errorf("adr-readme lost decision routing:\n%s", adrReadme)
	}

	agentsGuide, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("agent guide not rendered: %v", err)
	}
	if !strings.Contains(string(agentsGuide), "Route settled content by authority lifetime") {
		t.Errorf("agent guide lost decision routing:\n%s", agentsGuide)
	}

	// A fresh check on the synced tree is clean.
	drift, err := checkProject(p, testContext(t))
	if err != nil || len(drift) != 0 {
		t.Errorf("expected clean check, got drift=%#v err=%v", drift, err)
	}
}

func TestSemanticRenderingReviewEmptyDataAndLiteralPlaceholder(t *testing.T) {
	data := map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "skills": map[string]bool{}, "layout": testLayout(),
	}
	for _, tc := range []struct {
		name        string
		path        string
		instruction string
		fallback    string
	}{
		{"code reviewer", "agents/code-reviewer.md.tmpl", semanticCodeReviewInstruction, "Independent reviewer for implementation diffs, separate from the implementer."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := renderGolden(t, tc.path, data)
			if !strings.Contains(out, tc.instruction) {
				t.Errorf("empty-data render missing exact instruction %q:\n%s", tc.instruction, out)
			}
			if !strings.Contains(out, tc.fallback) {
				t.Errorf("empty-data render missing coherent generic fallback %q:\n%s", tc.fallback, out)
			}
			for _, residue := range []string{"<no value>", "{{ ."} {
				if strings.Contains(out, residue) {
					t.Errorf("empty-data render contains unresolved token %q:\n%s", residue, out)
				}
			}
		})
	}
}

func TestTemplateHashCoversExpandedSource(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderAll(p)
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
	// from its raw bytes; TemplateHash must be over the authored source-aware
	// projection (ADR-0052), not over generated assembly bytes.
	expanded, err := render.ExpandIncludesSource(string(raw), tid, templates.FS)
	if err != nil {
		t.Fatal(err)
	}
	if got != manifest.Hash([]byte(expanded.AuthoredText())) {
		t.Errorf("TemplateHash = %q, want authored expanded projection", got)
	}
	// invariant: rendering/render-engine:include-in-templatehash (TestTemplateHashCoversExpandedSource)
	if got == manifest.Hash(raw) {
		t.Error("TemplateHash equals raw-source hash; expected expanded-source hash")
	}
}
