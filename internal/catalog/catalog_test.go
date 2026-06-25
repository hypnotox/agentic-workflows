package catalog

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/templates"
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
	arch, ok := cat.Docs["architecture"]
	if !ok {
		t.Fatalf("architecture not in docs map, got: %v", cat.Docs)
	}
	if arch.Title != "Architecture" || len(arch.Sections) == 0 {
		t.Errorf("architecture doc spec = %+v", arch)
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
	expected := []string{"you-and-this-project", "identity", "invariants", "workflow", "commands", "document-map"}
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
