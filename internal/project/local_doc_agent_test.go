package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// invariant: rendering/doc-outputs:local-doc-output-complete (TestLocalDocAgentGuideProjection)
// invariant: rendering/guide-and-doc-templates:document-map-lists-mandatory-docs (TestLocalDocAgentGuideProjection)
func TestLocalDocAgentGuideProjection(t *testing.T) {
	cfg := "prefix: example\nintegrationBranch: main\nvars: {gateCmd: make gate}\nlocalDocs:\n  - name: runbooks/zulu\n    title: Zulu runbook\n    description: Last local document.\n  - name: runbooks/alpha\n    title: Alpha runbook\n    description: First local document.\n"
	root := scaffold(t, cfg)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	guide, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(guide)
	alpha := "- **Alpha runbook:** [docs/runbooks/alpha.md](docs/runbooks/alpha.md), First local document."
	zulu := "- **Zulu runbook:** [docs/runbooks/zulu.md](docs/runbooks/zulu.md), Last local document."
	if strings.Count(got, alpha) != 1 || strings.Count(got, zulu) != 1 || strings.Index(got, alpha) > strings.Index(got, zulu) {
		t.Fatalf("local document map = %q", got)
	}
	for _, path := range []string{"docs/runbooks/alpha.md", "docs/runbooks/zulu.md"} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatalf("document-map link target %s: %v", path, err)
		}
	}
	if _, ok := p.layout().Docs["runbooks/alpha"]; ok {
		t.Fatal("local document entered Layout.Docs")
	}

	beforeGuide := configHashOf(t, root, "AGENTS.md")
	beforeAlpha := configHashOf(t, root, "docs/runbooks/alpha.md")
	beforeZulu := configHashOf(t, root, "docs/runbooks/zulu.md")
	alphaPath := filepath.Join(root, "docs/runbooks/alpha.md")
	body, err := os.ReadFile(alphaPath)
	if err != nil {
		t.Fatal(err)
	}
	body = []byte(strings.Replace(string(body), "<!-- awf:edit-in-place body -->\n\n", "<!-- awf:edit-in-place body -->\n\npreserved bytes\n", 1))
	if err := os.WriteFile(alphaPath, body, 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteAwfConfig(t, root, strings.Replace(cfg, "Alpha runbook", "Changed alpha", 1))
	if after := configHashOf(t, root, "AGENTS.md"); after == beforeGuide {
		t.Fatal("local metadata did not change agent guide hash")
	}
	if after := configHashOf(t, root, "docs/runbooks/alpha.md"); after == beforeAlpha {
		t.Fatal("local metadata did not change matching output hash")
	}
	if after := configHashOf(t, root, "docs/runbooks/zulu.md"); after != beforeZulu {
		t.Fatal("local metadata changed unrelated output hash")
	}
	p, err = Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	preserved, err := os.ReadFile(alphaPath)
	if err != nil || !strings.Contains(string(preserved), "preserved bytes") {
		t.Fatalf("body preservation = %q, %v", preserved, err)
	}

	reordered := strings.Replace(cfg, "  - name: runbooks/zulu\n    title: Zulu runbook\n    description: Last local document.\n  - name: runbooks/alpha\n    title: Alpha runbook\n    description: First local document.\n", "  - name: runbooks/alpha\n    title: Alpha runbook\n    description: First local document.\n  - name: runbooks/zulu\n    title: Zulu runbook\n    description: Last local document.\n", 1)
	root2 := scaffold(t, reordered)
	if configHashOf(t, root2, "AGENTS.md") == "" {
		t.Fatal("missing reordered guide hash")
	}
	root3 := scaffold(t, cfg)
	if configHashOf(t, root2, "AGENTS.md") != configHashOf(t, root3, "AGENTS.md") {
		t.Fatal("YAML localDocs order changed guide hash")
	}
	root4 := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {gateCmd: make gate}\n")
	root5 := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {gateCmd: make gate}\nlocalDocs: []\n")
	if configHashOf(t, root4, "AGENTS.md") != configHashOf(t, root5, "AGENTS.md") {
		t.Fatal("empty localDocs changed guide hash")
	}
}

// invariant: rendering/doc-outputs:local-doc-output-complete (TestLocalDocReferenceChecksBody)
func TestLocalDocReferenceChecksBody(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {gateCmd: make gate}\nlocalDocs:\n  - name: runbooks/checks\n    title: Checks\n    description: Check references.\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "docs/runbooks/checks.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	b = []byte(strings.Replace(string(b), "<!-- awf:edit-in-place body -->\n\n", "<!-- awf:edit-in-place body -->\n\n[missing](absent.md) example-tdd\n", 1))
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := p.CheckReport(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	var link bool
	for _, d := range report.Drift {
		link = link || d.Path == "docs/runbooks/checks.md" && d.Kind == "dead-reference"
	}
	if !link {
		t.Fatalf("local body reference drift = %#v", report.Drift)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	var local []RenderedFile
	for _, file := range files {
		if file.Path == "docs/runbooks/checks.md" {
			local = append(local, file)
		}
	}
	if got := p.checkDeadSkillRefs(local, map[string]bool{}); len(got) != 1 || got[0].Path != "docs/runbooks/checks.md" || got[0].Kind != "dead-skill-reference" {
		t.Fatalf("local body skill-reference drift = %#v", got)
	}
}
