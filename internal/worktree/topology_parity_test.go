package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These closed porcelain bytes are the parity fixture for worktree parsing.
func TestWorktreePorcelainParityFixtures(t *testing.T) {
	cases := []struct {
		name, raw string
		accepted  bool
	}{
		{"branch", "worktree /x\x00HEAD abc\x00branch refs/heads/x\x00\x00", true},
		{"detached", "worktree /x\x00HEAD abc\x00detached\x00\x00", true},
		{"bare", "worktree /bare\x00bare\x00\x00", true},
		{"prunable", "worktree /x\x00HEAD abc\x00branch refs/heads/x\x00prunable gone\x00\x00", true},
		{"missing final delimiter", "worktree /x\x00HEAD abc\x00branch refs/heads/x\x00", false},
		{"missing HEAD", "worktree /x\x00branch refs/heads/x\x00\x00", false},
		{"valueless HEAD", "worktree /x\x00HEAD \x00branch refs/heads/x\x00\x00", false},
		{"valueless branch", "worktree /x\x00HEAD abc\x00branch \x00\x00", false},
		{"valueless prunable", "worktree /x\x00HEAD abc\x00branch refs/heads/x\x00prunable \x00\x00", false},
		{"missing state", "worktree /x\x00HEAD abc\x00\x00", false},
		{"branch detached", "worktree /x\x00HEAD abc\x00branch refs/heads/x\x00detached\x00\x00", false},
		{"detached value", "worktree /x\x00HEAD abc\x00detached nope\x00\x00", false},
		{"detached separator", "worktree /x\x00HEAD abc\x00detached \x00\x00", false},
		{"locked", "worktree /x\x00HEAD abc\x00branch refs/heads/x\x00locked reason\x00\x00", false},
		{"unknown", "worktree /x\x00HEAD abc\x00branch refs/heads/x\x00future x\x00\x00", false},
		{"duplicate HEAD", "worktree /x\x00HEAD abc\x00HEAD def\x00branch refs/heads/x\x00\x00", false},
		{"duplicate branch", "worktree /x\x00HEAD abc\x00branch refs/heads/x\x00branch refs/heads/y\x00\x00", false},
		{"duplicate prunable", "worktree /x\x00HEAD abc\x00branch refs/heads/x\x00prunable x\x00prunable y\x00\x00", false},
		{"duplicate bare", "worktree /x\x00bare\x00bare\x00\x00", false},
		{"bare fields", "worktree /x\x00bare nope\x00\x00", false},
		{"bare after HEAD", "worktree /x\x00HEAD abc\x00bare\x00\x00", false},
		{"bare separator", "bare \x00\x00", false},
		{"empty record", "worktree /x\x00HEAD abc\x00branch refs/heads/x\x00\x00\x00\x00", false},
		{"duplicate worktree", "worktree /x\x00worktree /y\x00HEAD abc\x00branch refs/heads/x\x00\x00", false},
		{"HEAD before worktree", "HEAD abc\x00worktree /x\x00branch refs/heads/x\x00\x00", false},
		{"branch before worktree", "branch refs/heads/x\x00worktree /x\x00HEAD abc\x00\x00", false},
		{"detached before worktree", "detached\x00worktree /x\x00HEAD abc\x00\x00", false},
		{"bare before worktree", "bare\x00\x00", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := registrations(testContext(t), func(context.Context, string, ...string) ([]byte, error) { return []byte(tc.raw), nil }, ".")
			if tc.accepted && err != nil {
				t.Fatal(err)
			}
			if !tc.accepted && err == nil {
				t.Fatal("accepted malformed porcelain")
			}
		})
	}
	_, err := registrations(testContext(t), func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("runner") }, ".")
	if err == nil {
		t.Fatal("runner error was hidden")
	}
}

func TestExactRegistrationRefusalAndManagedPathBranches(t *testing.T) {
	cause := errors.New("cause")
	refused := refusalCause("test", "condition", false, "next", cause)
	if !errors.Is(refused, cause) || !strings.Contains(refused.Error(), "changed topology: no") {
		t.Fatalf("refusal = %v", refused)
	}
	var nilRefusal *RefusalError
	if nilRefusal.Unwrap() != nil {
		t.Fatal("nil refusal unwrap was non-nil")
	}

	runner := func(raw string) Runner {
		return func(context.Context, string, ...string) ([]byte, error) { return []byte(raw), nil }
	}
	if err := exactRegistration(testContext(t), runner("worktree /other\x00HEAD abc\x00branch refs/heads/awf/wanted\x00\x00"), ".", "/wanted", "refs/heads/awf/wanted"); err == nil || !strings.Contains(err.Error(), "elsewhere") {
		t.Fatalf("foreign branch error = %v", err)
	}
	if err := exactRegistration(testContext(t), runner("worktree /other\x00HEAD abc\x00branch refs/heads/main\x00\x00"), ".", "/wanted", "refs/heads/awf/wanted"); err == nil || !strings.Contains(err.Error(), "uniquely") {
		t.Fatalf("missing registration error = %v", err)
	}
	if err := exactRegistration(testContext(t), runner("worktree /wanted\x00HEAD abc\x00detached\x00\x00"), ".", "/wanted", "refs/heads/awf/wanted"); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("detached registration error = %v", err)
	}

	root := t.TempDir()
	oldOwner := managedOwner
	defer func() { managedOwner = oldOwner }()
	managedOwner = func(string, os.FileInfo) error { return errors.New("foreign owner") }
	if err := safeManagedPath(root); err == nil || !strings.Contains(err.Error(), "foreign owner") {
		t.Fatalf("owner error = %v", err)
	}
	managedOwner = oldOwner
	link := filepath.Join(root, "link")
	if err := os.Symlink(t.TempDir(), link); err != nil {
		t.Fatal(err)
	}
	if err := safeManagedPath(link); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink error = %v", err)
	}
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := safeManagedPath(file); err == nil || !strings.Contains(err.Error(), "file-type") {
		t.Fatalf("file error = %v", err)
	}
	if err := safeManagedPath(filepath.Join(root, "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing error = %v", err)
	}
	if err := safeManagedPath(string(filepath.Separator)); err == nil || !strings.Contains(err.Error(), "no components") {
		t.Fatalf("root-only error = %v", err)
	}
}
