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
		".claude/skills/awf-using-awf/SKILL.md",
		".pi/skills/awf-using-awf/SKILL.md",
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
	for _, rel := range []string{"docs/working-with-awf.md", ".claude/skills/awf-using-awf/SKILL.md", ".pi/skills/awf-using-awf/SKILL.md"} {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), ".awf-bak") {
			t.Errorf("%s lacks recovery guidance", rel)
		}
	}
	for _, rel := range []string{".claude/skills/awf-using-awf/SKILL.md", ".pi/skills/awf-using-awf/SKILL.md"} {
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
