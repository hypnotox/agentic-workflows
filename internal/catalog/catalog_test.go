package catalog

import (
	"testing"
	"testing/fstest"

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
	arch, ok := cat.Docs["architecture"]
	if !ok {
		t.Fatalf("architecture not in docs map, got: %v", cat.Docs)
	}
	if arch.Title != "Architecture" || len(arch.Sections) == 0 {
		t.Errorf("architecture doc spec = %+v", arch)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(fstest.MapFS{}); err == nil {
		t.Fatal("expected error for missing catalog.yaml, got nil")
	}
}

func TestLoadMalformedYAML(t *testing.T) {
	fsys := fstest.MapFS{
		"catalog.yaml": &fstest.MapFile{Data: []byte("skills: [this is: not valid mapping")},
	}
	if _, err := Load(fsys); err == nil {
		t.Fatal("expected error for malformed catalog.yaml, got nil")
	}
}

func TestAgentsDocSectionsNonEmpty(t *testing.T) {
	cat, err := Load(templates.FS)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	sections := cat.Singletons["agents-doc"].Sections
	if len(sections) == 0 {
		t.Error("expected agents-doc Sections to be non-empty")
	}
	expected := []string{"you-and-this-project", "identity", "invariants", "workflow", "commands", "document-map"}
	for _, s := range expected {
		found := false
		for _, sec := range sections {
			if sec == s {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected section %q in agents-doc Sections, got: %v", s, sections)
		}
	}
}
