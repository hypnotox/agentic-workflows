// cmd/awf/main_test.go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunInitScaffoldsAndSyncs(t *testing.T) {
	root := t.TempDir()
	// Rename tempdir base via a child dir so prefix is predictable.
	proj := filepath.Join(root, "acme")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runInit(proj); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	cfg, err := os.ReadFile(filepath.Join(proj, ".claude", "awf.yaml"))
	if err != nil {
		t.Fatalf("config not scaffolded: %v", err)
	}
	if !containsLine(string(cfg), "prefix: acme") {
		t.Errorf("scaffold prefix wrong:\n%s", cfg)
	}
	if _, err := os.Stat(filepath.Join(proj, ".claude", "awf.lock")); err != nil {
		t.Errorf("lock not written: %v", err)
	}
}

func containsLine(s, line string) bool {
	for _, l := range splitLines(s) {
		if l == line {
			return true
		}
	}
	return false
}

