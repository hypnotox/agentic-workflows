// cmd/awf/list_add_test.go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunAddAppendsAndRejects(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".claude"), 0o755)
	_ = os.WriteFile(filepath.Join(root, ".claude", "awf.yaml"),
		[]byte("prefix: example\nvars: {}\nskills: {}\nagents: {}\nhooks: []\n"), 0o644)

	if err := runAdd(root, "no-such-skill"); err == nil {
		t.Errorf("expected error adding unknown skill")
	}
	if err := runAdd(root, "tdd"); err != nil {
		t.Fatalf("runAdd tdd: %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(root, ".claude", "awf.yaml"))
	if !strings.Contains(string(b), "tdd:") {
		t.Errorf("tdd not appended:\n%s", b)
	}
	if err := runAdd(root, "tdd"); err == nil {
		t.Errorf("expected error adding already-present skill")
	}
}
