package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunAddAppendsAndRejects(t *testing.T) {
	root := t.TempDir()
	awf := filepath.Join(root, ".claude", "awf")
	_ = os.MkdirAll(awf, 0o755)
	_ = os.WriteFile(filepath.Join(awf, "config.yaml"),
		[]byte("prefix: example\nvars:\n  testCmd: go test ./...\n  gateCmd: make gate\nskills: []\nagents: []\nhooks: []\n"), 0o644)

	if err := runAdd(root, "no-such-skill", io.Discard); err == nil {
		t.Errorf("expected error adding unknown skill")
	}
	if err := runAdd(root, "tdd", io.Discard); err != nil {
		t.Fatalf("runAdd tdd: %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(awf, "config.yaml"))
	if !strings.Contains(string(b), "- tdd") {
		t.Errorf("tdd not appended:\n%s", b)
	}
	if err := runAdd(root, "tdd", io.Discard); err == nil {
		t.Errorf("expected error adding already-present skill")
	}
}
