package main

import (
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
		if err := runSetup(root); err != nil {
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
	if err := runSetup(root); err == nil {
		t.Error("expected error when .githooks/ is absent")
	}
}

func TestRunSetupNonGitWarnsNoop(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".githooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runSetup(root); err != nil {
		t.Errorf("non-git dir should be a no-op, got: %v", err)
	}
}
