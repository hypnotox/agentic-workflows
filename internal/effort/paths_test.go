package effort

import (
	"os"
	"path/filepath"
	"testing"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

func TestEffortPathsUseSlugDirectoryAndOwnedMemory(t *testing.T) {
	root := initEffortRepo(t)
	roots, err := awfgit.ResolveControlRoots(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	paths, err := resolvePaths(roots)
	if err != nil {
		t.Fatal(err)
	}
	wantEffort := filepath.Join(root, ".awf", "efforts", "meaningful-slug")
	if paths.effort("meaningful-slug") != wantEffort {
		t.Fatalf("effort path = %s, want %s", paths.effort("meaningful-slug"), wantEffort)
	}
	if paths.stateFile("meaningful-slug") != filepath.Join(wantEffort, "state.json") {
		t.Fatalf("unexpected state path: %s", paths.stateFile("meaningful-slug"))
	}
	if paths.memoryFile("meaningful-slug") != filepath.Join(wantEffort, "memory.md") {
		t.Fatalf("unexpected memory path: %s", paths.memoryFile("meaningful-slug"))
	}
	if paths.activityFile("meaningful-slug") != filepath.Join(wantEffort, "activity.json") {
		t.Fatalf("unexpected activity path: %s", paths.activityFile("meaningful-slug"))
	}
	if got := memoryPublicPath("meaningful-slug"); got != ".awf/efforts/meaningful-slug/memory.md" {
		t.Fatalf("public memory path = %q", got)
	}
	if got := paths.publicMemoryPath("meaningful-slug"); got != ".awf/efforts/meaningful-slug/memory.md" {
		t.Fatalf("primary-root public memory path = %q, want the repository-relative form", got)
	}
	linked := paths
	linked.roots.InvokingRoot = filepath.Join(root, "linked")
	if got := linked.publicMemoryPath("meaningful-slug"); got != filepath.Join(root, ".awf", "efforts", "meaningful-slug", "memory.md") {
		t.Fatalf("linked-checkout public memory path = %q, want the absolute primary-root form", got)
	}
	if err := paths.validate(paths.worktrees); err != nil {
		t.Fatalf("worktrees root rejected: %v", err)
	}
	if err := paths.validate(paths.effortArchive); err != nil {
		t.Fatalf("effort archive root rejected: %v", err)
	}
	if err := paths.validate(filepath.Join(root, "foreign")); err == nil {
		t.Fatal("unknown resident root accepted")
	}
	blocked := filepath.Join(root, "blocked")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := paths.ensure(filepath.Join(blocked, "child")); err == nil {
		t.Fatal("resident root below file accepted")
	}
	if err := os.Chmod(paths.efforts, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := paths.ensure(paths.efforts); err == nil {
		t.Fatal("unsafe resident permissions accepted")
	}
	if err := os.Chmod(paths.efforts, 0o700); err != nil {
		t.Fatal(err)
	}
	badPaths := paths
	badPaths.roots.PrimaryRoot = "relative"
	if err := badPaths.validate(badPaths.efforts); err == nil {
		t.Fatal("invalid resident authority accepted")
	}

	badRoots := roots
	badRoots.PrimaryRoot = "relative"
	if _, err := resolvePaths(badRoots); err == nil {
		t.Fatal("invalid efforts root accepted")
	}
	effortArchive := filepath.Join(root, ".awf", "effort-archive")
	if err := os.Symlink(t.TempDir(), effortArchive); err != nil {
		t.Fatal(err)
	}
	if _, err := resolvePaths(roots); err == nil {
		t.Fatal("symlinked effort archive root accepted")
	}
	if err := os.Remove(effortArchive); err != nil {
		t.Fatal(err)
	}
	worktrees := filepath.Join(root, ".awf", "worktrees")
	if err := os.Symlink(t.TempDir(), worktrees); err != nil {
		t.Fatal(err)
	}
	if _, err := resolvePaths(roots); err == nil {
		t.Fatal("symlinked worktrees root accepted")
	}
}
