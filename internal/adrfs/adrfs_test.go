package adrfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewCreatesScaffoldAndPreservesExistingFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	relative, err := New(root, "choose-store")
	if err != nil {
		t.Fatal(err)
	}
	wantRelative := filepath.Join("docs", "decisions", "choose-store.md")
	if relative != wantRelative {
		t.Fatalf("New() path = %q, want %q", relative, wantRelative)
	}
	body, err := os.ReadFile(filepath.Join(root, relative))
	if err != nil {
		t.Fatal(err)
	}
	for _, heading := range []string{"# Decision: choose-store", "## Context and question", "## Material alternatives", "## Decision and rationale", "## Consequences", "## Affected topics"} {
		if !strings.Contains(string(body), heading) {
			t.Errorf("scaffold missing %q", heading)
		}
	}

	edited := []byte("author edit\n")
	if err := os.WriteFile(filepath.Join(root, relative), edited, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(root, "choose-store"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("repeat New() error = %v", err)
	}
	preserved, err := os.ReadFile(filepath.Join(root, relative))
	if err != nil || string(preserved) != string(edited) {
		t.Fatalf("existing file = %q, %v", preserved, err)
	}
}

func TestNewRejectsUnsafeSlug(t *testing.T) {
	t.Parallel()

	for _, slug := range []string{"", ".", "..", "-leading", "bad slug", "nested/decision", `nested\decision`} {
		if _, err := New(t.TempDir(), slug); err == nil || !strings.Contains(err.Error(), "invalid decision slug") {
			t.Errorf("New(%q) error = %v", slug, err)
		}
	}
}
