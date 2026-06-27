package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
)

// gitInit initialises a git repository at root via go-git (no host git binary).
func gitInit(t *testing.T, root string) {
	t.Helper()
	if _, err := git.PlainInit(root, false); err != nil {
		t.Fatalf("git init: %v", err)
	}
}

// seedHooksPath writes a repo-local core.hooksPath directly through go-git, so
// the test fixture does not go through awf's own write path.
func seedHooksPath(t *testing.T, root, val string) {
	t.Helper()
	repo, err := git.PlainOpen(root)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := repo.Config()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Raw.Section("core").SetOption("hooksPath", val)
	if err := repo.SetConfig(cfg); err != nil {
		t.Fatal(err)
	}
}

// readHooksPath reads the repo-local core.hooksPath directly through go-git.
func readHooksPath(t *testing.T, root string) string {
	t.Helper()
	repo, err := git.PlainOpen(root)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := repo.Config()
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Raw.Section("core").Option("hooksPath")
}

func TestRunSetupActivatesHooksIdempotent(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	if err := os.MkdirAll(filepath.Join(root, ".githooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	for i := range 2 { // idempotent: run twice
		if err := runSetup(root, false, io.Discard, io.Discard); err != nil {
			t.Fatalf("runSetup #%d: %v", i, err)
		}
	}
	if got := readHooksPath(t, root); got != ".githooks" {
		t.Errorf("core.hooksPath = %q, want .githooks", got)
	}
}

func TestRunSetupNoGithooksErrors(t *testing.T) {
	root := t.TempDir()
	if err := runSetup(root, false, io.Discard, io.Discard); err == nil {
		t.Error("expected error when .githooks/ is absent")
	}
}

func TestRunSetupNonGitWarnsNoop(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".githooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runSetup(root, false, io.Discard, io.Discard); err != nil {
		t.Errorf("non-git dir should be a no-op, got: %v", err)
	}
}

// gitRepoWithGithooks returns a fresh git repo with a .githooks directory.
func gitRepoWithGithooks(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitInit(t, root)
	if err := os.MkdirAll(filepath.Join(root, ".githooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRunSetupRefusesForeignHooksPath(t *testing.T) {
	root := gitRepoWithGithooks(t)
	seedHooksPath(t, root, ".husky")
	err := runSetup(root, false, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "--force-hooks") {
		t.Fatalf("expected refusal naming --force-hooks, got: %v", err)
	}
	if got := readHooksPath(t, root); got != ".husky" {
		t.Errorf("core.hooksPath = %q, want .husky (unchanged)", got)
	}
}

func TestRunSetupForceHooksOverrides(t *testing.T) {
	root := gitRepoWithGithooks(t)
	seedHooksPath(t, root, ".husky")
	if err := runSetup(root, true, io.Discard, io.Discard); err != nil {
		t.Fatalf("runSetup --force-hooks: %v", err)
	}
	if got := readHooksPath(t, root); got != ".githooks" {
		t.Errorf("core.hooksPath = %q, want .githooks", got)
	}
}
