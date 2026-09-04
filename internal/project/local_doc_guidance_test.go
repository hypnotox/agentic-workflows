package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLocalDocAuthoringGuidance proves the shipped docs and generated-document
// skill describe the declaration, narrow edit boundary, checks, and recovery.
func TestLocalDocAuthoringGuidance(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	for _, rel := range []string{
		"docs/working-with-awf.md",
		"docs/doc-standard.md",
		".claude/skills/awf-maintenance/SKILL.md",
		".pi/skills/awf-maintenance/SKILL.md",
	} {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"localDocs", "awf:edit-in-place"} {
			if !strings.Contains(string(b), want) {
				t.Errorf("%s lacks %q", rel, want)
			}
		}
	}
	for rel, want := range map[string]string{
		"docs/working-with-awf.md":                "Removal or uninstall refuses to destroy a present authored body",
		".claude/skills/awf-maintenance/SKILL.md": "Removing a declaration or uninstalling refuses when a present local document contains protected authored content",
		".pi/skills/awf-maintenance/SKILL.md":     "Removing a declaration or uninstalling refuses when a present local document contains protected authored content",
	} {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), want) {
			t.Errorf("%s lacks no-clobber retirement guidance %q", rel, want)
		}
	}
	for _, rel := range []string{".claude/skills/awf-maintenance/SKILL.md", ".pi/skills/awf-maintenance/SKILL.md"} {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "except the declared local document body") {
			t.Errorf("%s permits direct edits outside the in-place exception", rel)
		}
	}
}

func TestLocalDocCommandGuidance(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	for _, rel := range []string{"docs/working-with-awf.md"} {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"awf new doc", "<name> <description>", "--title", "Api V2", "docs/<name>.md"} {
			if !strings.Contains(string(b), want) {
				t.Errorf("%s lacks %q", rel, want)
			}
		}
	}
}

// invariant: rendering/doc-outputs:local-doc-output-complete (TestLocalDocDiscoveryGuidance)
func TestLocalDocDiscoveryGuidance(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	for _, rel := range []string{"docs/working-with-awf.md"} {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		got := string(b)
		for _, want := range []string{"AGENTS.md` document map", "Markdown links", "skill references"} {
			if !strings.Contains(got, want) {
				t.Errorf("%s lacks %q", rel, want)
			}
		}
	}
	b, err := os.ReadFile(filepath.Join(root, "docs/working-with-awf.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"does not make them catalog documents", "or widen staged drift"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("guide lacks boundary %q", want)
		}
	}
}
