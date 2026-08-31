package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestLoadProjectSessionRejectsMalformedRepository(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("not a gitdir pointer"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProjectSession(testContext(t), root); err == nil || errors.Is(err, awfgit.ErrNotARepository) {
		t.Fatalf("malformed repository error = %v", err)
	}
}

func mustOpenGit(t *testing.T, root string) *awfgit.Repo {
	t.Helper()
	repo, err := awfgit.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestResolveProjectResidentRoot(t *testing.T) {
	ctx := testContext(t)
	repo := gitfixture.InitRepo(t)
	primary := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	linked := filepath.Join(t.TempDir(), "linked")
	gitfixture.NativeWorktreeAdd(t, repo, linked, "linked")
	want := filesystem.NormalizePlatformPath(primary)
	if got := awfgit.ProjectResidentRoot(ctx, linked); got != want {
		t.Fatalf("resident root = %q, want primary %q", got, want)
	}
}

func TestResolveProjectResidentRootFallsBackOutsideGit(t *testing.T) {
	ctx := testContext(t)
	root := t.TempDir()
	if got := awfgit.ProjectResidentRoot(ctx, root); got != root {
		t.Fatalf("resident root = %q, want invoking root", got)
	}
}

func TestResolveProjectResidentRootFallsBackOnUnsafeResident(t *testing.T) {
	ctx := testContext(t)
	root := gitfixture.InitRepo(t).Root()
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(root, ".awf")); err != nil {
		t.Fatal(err)
	}
	if got := awfgit.ProjectResidentRoot(ctx, root); got != root {
		t.Fatalf("resident root = %q, want invoking root", got)
	}
}
