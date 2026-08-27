//go:build !windows

package contextop

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/contextq"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestFocusedWorkingStateMatchesCompleteForUnsupportedFilesystemEntry(t *testing.T) {
	root := contextPreparationFixture(t)
	path := filepath.Join(root, "README.md")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}

	state, repo := contextPreparationProject(t, root)
	focused, err := workingState(testsupport.Context(t), state, repo, []string{"internal/foo/x.go"})
	if err != nil {
		t.Fatal(err)
	}
	complete, err := workingCompleteState(testsupport.Context(t), state, repo)
	if err != nil {
		t.Fatal(err)
	}
	options := contextq.ContextOptions{Selection: contextq.SelectionExplicit}
	got := contextq.RenderContextText(contextq.New(focused).ContextForOptions([]string{"README.md"}, options), "live state for this project", nil)
	want := contextq.RenderContextText(contextq.New(complete).ContextForOptions([]string{"README.md"}, options), "live state for this project", nil)
	if got != want {
		t.Fatalf("focused output differs from complete for unsupported filesystem entry\nfocused:\n%s\ncomplete:\n%s", got, want)
	}
}
