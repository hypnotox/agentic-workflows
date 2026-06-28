package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInitScaffoldsAndSyncs(t *testing.T) {
	root := t.TempDir()
	// Rename tempdir base via a child dir so prefix is predictable.
	proj := filepath.Join(root, "acme")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runInit(proj, false, false, nil, "", io.Discard); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	cfg, err := os.ReadFile(filepath.Join(proj, ".awf", "config.yaml"))
	if err != nil {
		t.Fatalf("config not scaffolded: %v", err)
	}
	if !containsLine(string(cfg), "prefix: acme") {
		t.Errorf("scaffold prefix wrong:\n%s", cfg)
	}
	if _, err := os.Stat(filepath.Join(proj, ".awf", "awf.lock")); err != nil {
		t.Errorf("lock not written: %v", err)
	}
}

func containsLine(s, line string) bool {
	for _, l := range strings.Split(s, "\n") {
		if l == line {
			return true
		}
	}
	return false
}
