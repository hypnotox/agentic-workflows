package effort

import (
	"os"
	"path/filepath"
	"testing"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

func TestEffortPathsClosedResidentRoots(t *testing.T) {
	primary := filepath.Join(t.TempDir(), "primary")
	p, err := resolvePaths(awfgit.ControlRoots{PrimaryRoot: primary})
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{p.efforts, p.memory, p.worktrees, p.assign} {
		if err := p.ensure(root); err != nil {
			t.Fatal(err)
		}
		if info, err := os.Stat(root); err != nil || info.Mode().Perm() != 0o700 {
			t.Fatalf("resident %s: %v, %v", root, info, err)
		}
	}
	if err := p.ensure(filepath.Join(primary, "other")); err == nil {
		t.Fatal("unknown resident root accepted")
	}
	if p.record(idA) != filepath.Join(p.efforts, idA+".json") || p.memoryFile(idA) != filepath.Join(p.memory, idA+".md") || p.managedWorktree(idA) != filepath.Join(p.worktrees, idA) || p.assignments() != filepath.Join(p.assign, "sessions.json") {
		t.Fatal("stable ID-derived paths changed")
	}
}

func TestEffortPathsRejectUnsafePrimary(t *testing.T) {
	if _, err := resolvePaths(awfgit.ControlRoots{PrimaryRoot: "relative"}); err == nil {
		t.Fatal("relative primary accepted")
	}
	primary := filepath.Join(t.TempDir(), "primary")
	if err := os.MkdirAll(primary, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(primary, ".awf")); err != nil {
		t.Skip(err)
	}
	if _, err := resolvePaths(awfgit.ControlRoots{PrimaryRoot: primary}); err == nil {
		t.Fatal("symlinked resident ancestor accepted")
	}
}
