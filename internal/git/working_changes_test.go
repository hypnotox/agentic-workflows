package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// invariant: tooling/quality-gates:affected-package-feedback (TestWorktreeChangedPaths)
// TestWorktreeChangedPaths pins the complete HEAD-to-working-tree evidence
// used by consumers that must not silently omit local edits.
func TestWorktreeChangedPaths(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{
		"deleted.go":  "package fixture\n",
		"renamed.go":  "package fixture\n",
		"changed.go":  "package fixture\n",
		"unstaged.go": "package fixture\n",
	})
	if err := os.Remove(filepath.Join(root, "deleted.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "renamed.go"), filepath.Join(root, "renamed_new.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "changed.go"), []byte("package fixture\n// changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "untracked.go"), []byte("package fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.CommandContext(t.Context(), "git", "-C", root, "add", "-A").CombinedOutput(); err != nil {
		t.Fatalf("stage: %v: %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(root, "unstaged.go"), []byte("package fixture\n// unstaged\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := statusRepo(t, root).WorktreeChangedPaths(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"changed.go", "deleted.go", "renamed.go", "renamed_new.go", "unstaged.go", "untracked.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("paths = %#v, want %#v", got, want)
	}
}

func TestParseWorktreeChangedPathsRejectsMalformedTransport(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte("? untracked.go"),
		[]byte("! ignored.go\x00"),
		[]byte("2 bad\x00"),
		[]byte("2 . N... 100644 100644 100644 a b R100 new.go\x00"),
	} {
		if _, err := parseWorktreeChangedPaths(raw); err == nil {
			t.Fatalf("parseWorktreeChangedPaths(%q) unexpectedly succeeded", raw)
		}
	}
}
