// internal/project/golden_test.go
package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEndToEndGolden(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}

	agent, err := os.ReadFile(filepath.Join(root, ".claude/agents/code-reviewer.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agent), "Specialised reviewer for example") {
		t.Errorf("agent not interpolated:\n%s", agent)
	}

	pre, err := os.ReadFile(filepath.Join(root, ".githooks/pre-commit"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pre), "awf check") || !strings.Contains(string(pre), "make gate") {
		t.Errorf("pre-commit hook wrong:\n%s", pre)
	}
	// Rendered hook must be a POSIX text file (trailing newline).
	if !strings.HasSuffix(string(pre), "\n") {
		t.Errorf("pre-commit must end with a newline:\n%q", pre)
	}
	// Hook must be executable.
	info, _ := os.Stat(filepath.Join(root, ".githooks/pre-commit"))
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("pre-commit not executable: %v", info.Mode())
	}

	push, err := os.ReadFile(filepath.Join(root, ".githooks/pre-push"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(push), "make gate full") {
		t.Errorf("pre-push hook wrong:\n%s", push)
	}

	// A fresh check on the synced tree is clean.
	drift, err := p.Check()
	if err != nil || len(drift) != 0 {
		t.Errorf("expected clean check, got drift=%#v err=%v", drift, err)
	}
}
