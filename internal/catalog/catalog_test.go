package catalog

import (
	"testing"

	"agentic-workflows/templates"
)

func TestLoadFromEmbed(t *testing.T) {
	cat, err := Load(templates.FS)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	spec, ok := cat.Skills["tdd"]
	if !ok {
		t.Fatal("tdd not in catalog")
	}
	if len(spec.Sections) != 2 || spec.Sections[0] != "surfaces" {
		t.Errorf("tdd sections = %v", spec.Sections)
	}
	if _, ok := cat.Agents["code-reviewer"]; !ok {
		t.Errorf("code-reviewer not in agents map, got: %v", cat.Agents)
	}
	if len(cat.Hooks) != 2 {
		t.Errorf("hooks = %v", cat.Hooks)
	}
}

func TestAgentsDocSectionsNonEmpty(t *testing.T) {
	cat, err := Load(templates.FS)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cat.AgentsDoc.Sections) == 0 {
		t.Error("expected AgentsDoc.Sections to be non-empty")
	}
	expected := []string{"overview", "build-test", "workflow-chain", "layout", "conventions"}
	for _, s := range expected {
		found := false
		for _, sec := range cat.AgentsDoc.Sections {
			if sec == s {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected section %q in AgentsDoc.Sections, got: %v", s, cat.AgentsDoc.Sections)
		}
	}
}
