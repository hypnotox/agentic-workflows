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
	if len(cat.Agents) == 0 || cat.Agents[0] != "code-reviewer" {
		t.Errorf("agents = %v", cat.Agents)
	}
	if len(cat.Hooks) != 2 {
		t.Errorf("hooks = %v", cat.Hooks)
	}
}
