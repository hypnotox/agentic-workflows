package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestUninstallRemovesGeneratedFilesAndLock(t *testing.T) {
	root := scaffoldProject(t)
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatalf("expected AGENTS.md before uninstall: %v", err)
	}
	if err := runUninstall(root, io.Discard); err != nil {
		t.Fatalf("runUninstall: %v", err)
	}
	for _, rel := range []string{"AGENTS.md", "CLAUDE.md", ".claude", "docs", filepath.Join(".awf", "awf.lock")} {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Errorf("expected %s removed, stat err = %v", rel, err)
		}
	}
	// The authored config is left in place.
	if _, err := os.Stat(filepath.Join(root, ".awf", "config.yaml")); err != nil {
		t.Errorf("config.yaml should remain: %v", err)
	}
}

func TestRunUninstallDispatch(t *testing.T) {
	root := scaffoldProject(t)
	swapGetwd(t, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "uninstall"}, &out, &errb); code != 0 {
		t.Fatalf("uninstall dispatch failed: %s", errb.String())
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
		t.Errorf("AGENTS.md should be removed after uninstall dispatch")
	}
}

func TestUninstallNoLockErrors(t *testing.T) {
	root := t.TempDir()
	if err := runUninstall(root, io.Discard); err == nil {
		t.Error("expected error when no lock is present")
	}
}
