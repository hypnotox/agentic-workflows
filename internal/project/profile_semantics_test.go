package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestCoreRenderedWorkflowExcludesFullAuthority expands every selected artifact
// for both targets. It deliberately scans operational references instead of
// ordinary prose such as an adopter discussing its own historical documents.
// invariant: rendering/catalog-and-targets:profile-dependency-closure (TestCoreRenderedWorkflowExcludesFullAuthority)
func TestCoreRenderedWorkflowExcludesFullAuthority(t *testing.T) {
	for _, tc := range []struct {
		name    string
		profile catalog.Profile
	}{
		{name: "core", profile: catalog.ProfileCore},
		{name: "full", profile: catalog.ProfileFull},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffold(t, "prefix: example\nprofile: "+string(tc.profile)+"\nintegrationBranch: main\n")
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			files, err := p.RenderAll()
			if err != nil {
				t.Fatal(err)
			}
			if len(files) == 0 {
				t.Fatal("selected profile rendered no artifacts")
			}

			for _, file := range files {
				if tc.profile == catalog.ProfileCore {
					for _, forbidden := range []string{
						"./awf context", "./awf adr", "./awf plan", "./awf audit",
						"current-state", "current-state authority", "decision index", "State changes",
						"refactor ADR", "ADR Context", "ADR scope", "plan Notes", "pending ADR", "ADR review", "plan review", "`Implemented` flip",
						"-proposing-adr", "-writing-plans", "-reviewing-plan",
						"-executing-plans", "-subagent-driven-development", "-adr-lifecycle",
						"adr-reviewer", "plan-reviewer",
					} {
						if strings.Contains(file.Content, forbidden) {
							t.Errorf("Core output %s retains Full-only operational reference %q", file.Path, forbidden)
						}
					}
				}
			}

			if tc.profile == catalog.ProfileFull {
				foundContext, foundADR := false, false
				for _, file := range files {
					foundContext = foundContext || strings.Contains(file.Content, "./awf context")
					foundADR = foundADR || strings.Contains(file.Content, "-proposing-adr")
				}
				if !foundContext || !foundADR {
					t.Fatalf("Full output lost governance semantics: context=%v ADR=%v", foundContext, foundADR)
				}
			}
		})
	}
}

// TestProfileTransitionPreservesHistoryAndRestoresGovernance exercises the
// working-tree Full -> Core -> Full path, including each native target.
// invariant: rendering/project-output-plan:profile-projected-render (TestProfileTransitionPreservesHistoryAndRestoresGovernance)
func TestProfileTransitionPreservesHistoryAndRestoresGovernance(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\n", map[string]string{
		"parts/adr-template/body.md": "Full-only ADR template override.\n",
	})
	historical := map[string]string{
		"docs/decisions/0001-history.md":   "---\nstatus: Implemented\n---\n# ADR-0001: Authored history\n",
		"docs/plans/2026-01-01-history.md": "---\nformat: plan-v2\ndate: 2026-01-01\nstatus: Implemented\n---\n# Plan: Authored history\n",
	}
	for path, content := range historical {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	full, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := full.Sync(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		".claude/skills/example-proposing-adr/SKILL.md",
		".pi/skills/example-proposing-adr/SKILL.md",
		"docs/decisions/INDEX.md",
	} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatalf("Full output %s missing: %v", path, err)
		}
	}
	// Retained history need not remain parseable while Core is selected.
	historical["docs/decisions/0001-history.md"] = "---\nformat: [broken\n"
	historical["docs/plans/2026-01-01-history.md"] = "---\nformat: [broken\n"
	for path, content := range historical {
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Core must receive no Full-only source. The deleted part is representative
	// of the profile-owned .awf sources that must not become dormant layers.
	testsupport.WriteAwfConfig(t, root, "prefix: example\nprofile: core\nintegrationBranch: main\nvars:\n  gateCmd: test-gate\n")
	if err := os.Remove(filepath.Join(root, ".awf", "parts", "adr-template", "body.md")); err != nil {
		t.Fatal(err)
	}
	core, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := core.Sync(); err != nil {
		t.Fatal(err)
	}
	for path, want := range historical {
		got, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || string(got) != want {
			t.Fatalf("Core transition altered authored history %s: got %q err=%v", path, got, err)
		}
	}
	for _, path := range []string{
		".claude/skills/example-proposing-adr/SKILL.md",
		".pi/skills/example-proposing-adr/SKILL.md",
		"docs/decisions/INDEX.md",
	} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatalf("Core transition retained managed Full output %s: %v", path, err)
		}
	}
	for _, dir := range []string{
		".claude/skills/example-proposing-adr",
		".pi/skills/example-proposing-adr",
	} {
		if _, err := os.Stat(filepath.Join(root, dir)); !os.IsNotExist(err) {
			t.Fatalf("Core transition retained empty Full-only ancestor %s: %v", dir, err)
		}
	}
	if _, err := core.CheckReport(testContext(t)); err != nil {
		t.Fatalf("Core consulted malformed retained history: %v", err)
	}

	testsupport.WriteAwfConfig(t, root, "prefix: example\nprofile: full\nintegrationBranch: main\nvars:\n  gateCmd: test-gate\n")
	full, err = Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := full.Sync(); err == nil {
		t.Fatal("Full accepted malformed retained history")
	}
	for path := range historical {
		if err := os.Remove(filepath.Join(root, path)); err != nil {
			t.Fatal(err)
		}
	}
	if err := full.Sync(); err != nil {
		t.Fatalf("Full did not restore governance after history correction: %v", err)
	}
	for _, path := range []string{
		".claude/skills/example-proposing-adr/SKILL.md",
		".pi/skills/example-proposing-adr/SKILL.md",
		"docs/decisions/INDEX.md",
	} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatalf("Full restoration omitted %s: %v", path, err)
		}
	}
}
