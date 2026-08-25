package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestDocArchitectureTemplate(t *testing.T) {
	out := renderGolden(t, "docs/architecture.md.tmpl", map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"data":   map[string]any{},
	})
	if !strings.Contains(out, "# Architecture") {
		t.Errorf("expected '# Architecture' heading:\n%s", out)
	}
}

// TestGlossaryTemplate is the glossary doc's golden (nonArtifactGoldens-listed:
// docs sit outside the ADR-0080 skills/agents completeness walk). The terms
// value arrives pre-transformed - renderGolden bypasses the project-layer
// transform, whose behavior glossary_test.go owns.
func TestGlossaryTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"data":   map[string]any{"terms": "| alpha | first |\n| beta | second |\n"},
		"skills": map[string]bool{},
		"layout": testLayout(),
	}
	out := renderGolden(t, "docs/glossary.md.tmpl", data)
	for _, want := range []string{"# Glossary", "| Term | Meaning |", "| alpha | first |", "| beta | second |"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "No terms recorded yet") {
		t.Errorf("placeholder must not render alongside a populated table:\n%s", out)
	}
}

// invariant: rendering/doc-outputs:pi-runtime-reference-output (TestDailyAdvancedDocumentationOwnership)
func TestDailyAdvancedDocumentationOwnership(t *testing.T) {
	for _, profile := range []string{"core", "full"} {
		t.Run(profile, func(t *testing.T) {
			root := scaffold(t, "prefix: example\nprofile: "+profile+"\nintegrationBranch: main\n")
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			if err := syncProject(p); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"docs/working-with-awf.md", "docs/pi-runtime-reference.md", "AGENTS.md"} {
				if _, err := os.Stat(filepath.Join(root, path)); err != nil {
					t.Fatalf("%s missing: %v", path, err)
				}
			}
			daily, err := os.ReadFile(filepath.Join(root, "docs/working-with-awf.md"))
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"awf init", "./awf render", "./awf check", "./awf upgrade", "generated"} {
				if !strings.Contains(string(daily), want) {
					t.Errorf("daily guide missing %q", want)
				}
			}
			for _, absent := range []string{"AWF_CONTEXT_SPILL_V1", "bytes=<decimal>", "ExtensionAPI.queueCommand"} {
				if strings.Contains(string(daily), absent) {
					t.Errorf("daily guide duplicates advanced detail %q", absent)
				}
			}
			piPath := filepath.Join(root, "docs/pi-runtime-reference.md")
			pi, err := os.ReadFile(piPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"applies only to adopters using the Pi target", "active-tool and real-path file-mutation queue APIs", "verificationCheckout", "awf-subagents.local.json", "handoff_session"} {
				if !strings.Contains(string(pi), want) {
					t.Errorf("Pi reference missing protocol sentinel %q", want)
				}
			}
			config, err := os.ReadFile(filepath.Join(root, "docs/config-reference.md"))
			if err != nil {
				t.Fatal(err)
			}
			debugging, err := os.ReadFile(filepath.Join(root, "docs/debugging.md"))
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"stray `{{` in", "available keys"} {
				if !strings.Contains(string(config), want) {
					t.Errorf("config reference missing placeholder sentinel %q", want)
				}
			}
			if profile == "full" {
				for _, want := range []string{"anchored dialect", "Unset-var notes and recovery"} {
					if !strings.Contains(string(config), want) {
						t.Errorf("config reference missing advanced sentinel %q", want)
					}
				}
			}
			for _, want := range []string{"exceeds 8,192 bytes", "near-match"} {
				if !strings.Contains(string(debugging), want) {
					t.Errorf("debugging missing recovery sentinel %q", want)
				}
			}
			if profile == "full" {
				for _, want := range []string{"awf changelog --since"} {
					if !strings.Contains(string(debugging), want) {
						t.Errorf("debugging missing advanced upgrade sentinel %q", want)
					}
				}
			}
			agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(agents), "docs/pi-runtime-reference.md") {
				t.Error("AGENTS map does not reach Pi reference")
			}
			lock, err := os.ReadFile(filepath.Join(root, ".awf/awf.lock"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(lock), `"docs/pi-runtime-reference.md"`) {
				t.Error("lock does not own Pi reference")
			}
			plan, err := testPlan(p)
			if err != nil {
				t.Fatal(err)
			}
			var policy OutputPolicy
			for _, node := range plan.Nodes() {
				if output, ok := node.Output(); ok && node.Path() == "docs/pi-runtime-reference.md" {
					policy = output.Policy()
				}
			}
			if !policy.ScanReferences || !policy.ScanSkillReferences {
				t.Errorf("Pi reference policy does not scan links and skill references: %#v", policy)
			}
			testsupport.WriteFile(t, piPath, strings.Replace(string(pi), "working-with-awf.md", "missing-daily-guide.md", 1))
			drift, err := checkProject(p, testContext(t))
			if err != nil {
				t.Fatal(err)
			}
			handEdited := false
			for _, d := range drift {
				handEdited = handEdited || d.Path == "docs/pi-runtime-reference.md" && d.Kind == "hand-edited"
			}
			if !handEdited {
				t.Errorf("Pi reference lacks drift coverage: %#v", drift)
			}
		})
	}
}
