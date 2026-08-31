package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

func TestExactRegistrationRefusalAndManagedPathBranches(t *testing.T) {
	cause := errors.New("cause")
	refused := refusalCause("test", "condition", false, cause, "next")
	if !errors.Is(refused, cause) || !strings.Contains(refused.Error(), "changed topology: no") {
		t.Fatalf("refusal = %v", refused)
	}
	var nilRefusal *RefusalError
	if nilRefusal.Unwrap() != nil {
		t.Fatal("nil refusal unwrap was non-nil")
	}

	registered := func(registrations ...awfgit.WorktreeRegistration) Runner {
		return &checkoutStub{worktreeList: func(context.Context) ([]awfgit.WorktreeRegistration, error) { return registrations, nil }}
	}
	if err := exactRegistration(testContext(t), registered(awfgit.WorktreeRegistration{Path: "/other", HEAD: "abc", Branch: "refs/heads/awf/wanted"}), "/wanted", "refs/heads/awf/wanted"); err == nil || !strings.Contains(err.Error(), "elsewhere") {
		t.Fatalf("foreign branch error = %v", err)
	}
	if err := exactRegistration(testContext(t), registered(awfgit.WorktreeRegistration{Path: "/other", HEAD: "abc", Branch: "refs/heads/main"}), "/wanted", "refs/heads/awf/wanted"); err == nil || !strings.Contains(err.Error(), "uniquely") {
		t.Fatalf("missing registration error = %v", err)
	}
	if err := exactRegistration(testContext(t), registered(awfgit.WorktreeRegistration{Path: "/wanted", HEAD: "abc", Detached: true}), "/wanted", "refs/heads/awf/wanted"); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("detached registration error = %v", err)
	}
	faulted := &checkoutStub{worktreeList: func(context.Context) ([]awfgit.WorktreeRegistration, error) {
		return nil, errors.New("registration probe")
	}}
	if err := exactRegistration(testContext(t), faulted, "/wanted", "refs/heads/awf/wanted"); err == nil {
		t.Fatal("registration probe error was hidden")
	}

	root := filesystem.NormalizePlatformPath(t.TempDir())
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
