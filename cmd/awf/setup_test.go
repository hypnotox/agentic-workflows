package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSetupActivatesHooksIdempotent(t *testing.T) {
	root := t.TempDir()
	if err := exec.Command("git", "-C", root, "init").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".githooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	for i := range 2 { // idempotent: run twice
		if err := runSetup(root, false, io.Discard, io.Discard); err != nil {
			t.Fatalf("runSetup #%d: %v", i, err)
		}
	}
	out, err := exec.Command("git", "-C", root, "config", "core.hooksPath").Output()
	if err != nil {
		t.Fatalf("read core.hooksPath: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != ".githooks" {
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
	if err := exec.Command("git", "-C", root, "init").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".githooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRunSetupRefusesForeignHooksPath(t *testing.T) {
	root := gitRepoWithGithooks(t)
	if err := exec.Command("git", "-C", root, "config", "core.hooksPath", ".husky").Run(); err != nil {
		t.Fatalf("seed hooksPath: %v", err)
	}
	err := runSetup(root, false, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "--force-hooks") {
		t.Fatalf("expected refusal naming --force-hooks, got: %v", err)
	}
	out, _ := exec.Command("git", "-C", root, "config", "core.hooksPath").Output()
	if got := strings.TrimSpace(string(out)); got != ".husky" {
		t.Errorf("core.hooksPath = %q, want .husky (unchanged)", got)
	}
}

func TestRunSetupForceHooksOverrides(t *testing.T) {
	root := gitRepoWithGithooks(t)
	if err := exec.Command("git", "-C", root, "config", "core.hooksPath", ".husky").Run(); err != nil {
		t.Fatalf("seed hooksPath: %v", err)
	}
	if err := runSetup(root, true, io.Discard, io.Discard); err != nil {
		t.Fatalf("runSetup --force-hooks: %v", err)
	}
	out, _ := exec.Command("git", "-C", root, "config", "core.hooksPath").Output()
	if got := strings.TrimSpace(string(out)); got != ".githooks" {
		t.Errorf("core.hooksPath = %q, want .githooks", got)
	}
}
