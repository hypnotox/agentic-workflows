package artifactfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
	"github.com/hypnotox/agentic-workflows/internal/knowledge"
	"github.com/hypnotox/agentic-workflows/internal/projector"
)

func TestAllStartersConformToKnowledgeContract(t *testing.T) {
	root := t.TempDir()
	for _, test := range []struct {
		kind   string
		create func(string, string) (string, error)
	}{
		{"Change Intent", NewIntent},
		{"Specification", NewSpec},
		{"Implementation Plan", NewPlan},
		{"Architecture Decision", NewADR},
		{"Project Topic", func(root, name string) (string, error) { return NewTopic(root, name, []string{"src/**"}) }},
	} {
		relative, err := test.create(root, "ship-it")
		if err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			t.Fatal(err)
		}
		var metadata map[string]any
		if _, found, err := frontmatter.Parse(content, &metadata); err != nil || !found {
			t.Fatalf("%s: frontmatter found=%v, %v", relative, found, err)
		}
		if metadata["type"] != test.kind || metadata["title"] != "ship-it" {
			t.Fatalf("%s: metadata = %v", relative, metadata)
		}
	}
	if findings, err := knowledge.Check(root); err != nil || len(findings) != 0 {
		t.Fatalf("starter bundle: %v, %v", findings, err)
	}
	if _, err := projector.Render(root); err != nil {
		t.Fatal(err)
	}
	if findings, err := projector.Check(root); err != nil || len(findings) != 0 {
		t.Fatalf("adopter check: %v, %v", findings, err)
	}
}

func TestConceptStartersRejectReservedBasenamesBeforeWriting(t *testing.T) {
	for _, name := range []string{"index", "log", "INDEX", "Log"} {
		for _, create := range []func(string, string) (string, error){NewPlan, NewADR,
			func(root, name string) (string, error) { return NewTopic(root, "nested/"+name, []string{"src/**"}) },
		} {
			root := t.TempDir()
			if _, err := create(root, name); err == nil || !strings.Contains(err.Error(), "reserved") {
				t.Fatalf("%q accepted or wrong error: %v", name, err)
			}
			if _, err := os.Stat(filepath.Join(root, "docs")); !os.IsNotExist(err) {
				t.Fatalf("reserved name caused writes: %v", err)
			}
		}
	}
	// Only basenames are reserved, not directories.
	root := t.TempDir()
	for _, create := range []func(string, string) (string, error){NewIntent, NewSpec,
		func(root, name string) (string, error) { return NewTopic(root, name+"/nested", []string{"src/**"}) },
	} {
		if _, err := create(root, "index"); err != nil {
			t.Fatalf("directory name incorrectly reserved: %v", err)
		}
	}
}
