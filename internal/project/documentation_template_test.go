package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocArchitectureTemplate(t *testing.T) {
	out := renderGolden(t, "docs/architecture.md.tmpl", map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"data":   map[string]any{},
	})
	for _, phrase := range []string{
		"# Architecture",
		"`Loader` opens and validates selected project state to construct one immutable `Session`",
		"`internal/outputplan` and `internal/publisher`",
		"the `Publisher` that owns output planning, rendering coordination, backup decisions, and publication",
		"the policy-free `RepositoryChecker` preserves ordered aggregation",
		"each concern owner retains its classification, severity, and policy",
		"`internal/projectmutation` and focused operations: a mechanics-only transaction boundary",
		"focused operations retain validation, mutation and rollback order, recovery policy, outcomes, and presentation",
		"one awf-owned protocol-v2 profile adapter with no Pi-specific effort association or memory tools",
		"`pi-tools` owns general Pi mechanics",
	} {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected architecture ownership phrase %q:\n%s", phrase, out)
		}
	}
	for _, stale := range []string{
		"configuration loading, rendering, output planning, and repository checks",
		"retained effort integration",
		"auxiliary commands may provide repository checks and diagnostics",
	} {
		if strings.Contains(out, stale) {
			t.Errorf("architecture retains stale ownership phrase %q:\n%s", stale, out)
		}
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

// invariant: rendering/doc-outputs:pi-runtime-reference-output (TestDailyDocumentationOwnership)
func TestDailyDocumentationOwnership(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\n")
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"docs/working-with-awf.md", "docs/pi-runtime-reference.md", "AGENTS.md", "docs/config-reference.md"} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatalf("%s missing: %v", path, err)
		}
	}
	daily, err := os.ReadFile(filepath.Join(root, "docs/working-with-awf.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"./awf render", "./awf check", "./awf upgrade", "generated"} {
		if !strings.Contains(string(daily), want) {
			t.Errorf("daily guide missing %q", want)
		}
	}
}
