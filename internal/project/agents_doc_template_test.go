package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// invariant: rendering/guide-and-doc-templates:guide-entry-point-routing (TestAgentsDocNativeSkillRouter)
// invariant: rendering/guide-and-doc-templates:external-code-design-authority (TestAgentsDocNativeSkillRouter)
func TestAgentsDocNativeSkillRouter(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"testCmd": "go test ./...",
			"gateCmd": "make gate",
		},
		"layout": testLayout(),
		"data":   map[string]any{},
		"skills": map[string]bool{"brainstorming": true, "adr-lifecycle": true, "tdd": true, "bugfix": true},
	}
	out := renderGolden(t, "agents-doc/AGENTS.md.tmpl", data)
	for _, phrase := range []string{
		"## You and this project",
		"## Identity",
		"## Invariants",
		"## Workflow",
		"## Commands",
		"## Document map",
		"Route settled content by authority lifetime",
		"Preserve the approved design boundary",
		"make gate",
		"Treat exposed native-skill descriptions as routing metadata.",
		"Before loading a skill, identify the next concrete action.",
		"a possible later edit, render, documentation update, review, or commit does not justify loading its skill now.",
		"Load multiple bodies only when each independently governs that same next action before another routing decision can occur.",
		"Before any mutation, load the native skill that governs that action.",
		"Change size, including a minimal change, never excuses this routing step.",
	} {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
	for _, banned := range []string{"docs/maintainable-code-design.md", "Enabled skills:", "example-brainstorming", "purpose", "Trigger:", "Usually follows:", "Common follow-ups:", "fallback", "brainstorming → ADR", "warranted by", "A plan may use exact content/diffs", "V2 ADR", "pollute parent context", "Chain skills"} {
		if strings.Contains(out, banned) {
			t.Errorf("guide retains evicted prose or catalog residue %q:\n%s", banned, out)
		}
	}
	if got := strings.Count(out, "Stage the complete transaction"); got != 1 {
		t.Errorf("guide must carry exactly one concise gate rule, got %d:\n%s", got, out)
	}
}

func TestAgentsDocOwnershipScope(t *testing.T) {
	out := renderGolden(t, "agents-doc/AGENTS.md.tmpl", map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"layout": testLayout(),
		"data":   map[string]any{},
	})
	for _, phrase := range []string{
		"responsible for its long-term health as well as the task in front of you",
		"Defects caused by the transaction or blocking its safe completion are repaired in the same commit",
		"unrelated concrete defects are recorded and routed separately without expanding scope",
	} {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected ownership boundary %q in output:\n%s", phrase, out)
		}
	}
	for _, stale := range []string{
		"Bugs you notice in passing are yours",
		"coverage gaps are yours",
		"documentation drift is yours to fix",
	} {
		if strings.Contains(out, stale) {
			t.Errorf("guide retains stale broad ownership phrase %q:\n%s", stale, out)
		}
	}
}

func TestAgentsDocTemplateConfigDriven(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"gateCmd": "",
		},
		"layout": testLayout(),
		"skills": map[string]bool{"brainstorming": true, "adr-lifecycle": true},
		"data": map[string]any{
			"invariants": []map[string]any{
				{"text": "**Custom rule.**", "ref": "ADR-0009"},
			},
		},
		"docs": []map[string]any{
			{"title": "Architecture", "desc": "system shape", "path": "docs/architecture.md"},
		},
	}
	out := renderGolden(t, "agents-doc/AGENTS.md.tmpl", data)
	for _, phrase := range []string{
		"**Custom rule.** (ADR-0009)",
		"[docs/architecture.md](docs/architecture.md)",
	} {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
	if strings.Contains(out, "]()") {
		t.Errorf("empty-string vars must not render empty-target links:\n%s", out)
	}
}

// invariant: rendering/guide-and-doc-templates:guide-entry-point-routing (TestGuideOmitsLocalAndStandardSkillMetadata)
func TestGuideOmitsLocalAndStandardSkillMetadata(t *testing.T) {
	const localDescription = "Route ultraviolet nebula work through its native procedure."
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\n", map[string]string{
		"skills/nebula-router.yaml": "data:\n  description: " + localDescription + "\n",
	})
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}

	banned := []string{
		"Enabled skills:", "Trigger:", "Usually follows:", "Common follow-ups:", "fallback",
		"example-brainstorming", "example-bugfix", "example-nebula-router", localDescription,
		"(chain):", "(task):", "(support):",
	}
	for _, residue := range banned {
		if strings.Contains(string(body), residue) {
			t.Errorf("guide retains skill catalog residue %q:\n%s", residue, body)
		}
	}
}
