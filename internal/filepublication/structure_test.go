package filepublication

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// invariant: tooling/file-publication:exclusive-file-publication-single-home (TestExclusivePublicationHasOneReleasedPlatformHome)
func TestExclusivePublicationHasOneReleasedPlatformHome(t *testing.T) {
	root := repositoryRoot(t)
	for _, name := range []string{"internal/effort/publication_linux.go", "internal/effort/publication_darwin.go", "internal/effort/publication_windows.go"} {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "if expected == nil") {
			t.Fatalf("released-platform no-replace implementation remains outside internal/filepublication: %s", name)
		}
	}
	effort, err := os.ReadFile(filepath.Join(root, "internal/effort/store.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(effort), `"github.com/hypnotox/agentic-workflows/internal/filepublication"`) {
		t.Fatal("internal/effort does not depend inward on internal/filepublication")
	}
	for _, name := range []string{"publication.go", "publication_linux.go", "publication_darwin.go", "publication_windows.go"} {
		raw, err := os.ReadFile(filepath.Join(root, "internal/filepublication", name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "internal/effort") {
			t.Fatalf("internal/filepublication depends outward on effort: %s", name)
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repository root")
		}
		dir = parent
	}
}
