package worktree

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestTopologyDiagnostics(t *testing.T) {
	for _, err := range []interface {
		Diagnostic() (presentation.Diagnostic, error)
	}{
		&RefusalError{Category: "topology", Condition: "path exists", ChangedTopology: true, NextAction: "inspect", Err: errors.New("probe failed")},
		&CreationError{Message: "creation", Condition: "creation failed", ChangedEffort: true, ChangedTopology: true, Cause: errors.New("add failed"), Steps: []string{"inspect", "retry"}},
	} {
		diagnostic, diagnosticErr := err.Diagnostic()
		if diagnosticErr != nil {
			t.Fatal(diagnosticErr)
		}
		document, documentErr := diagnostic.Document()
		if documentErr != nil {
			t.Fatal(documentErr)
		}
		var out bytes.Buffer
		if renderErr := presentation.Render(&out, document); renderErr != nil || out.Len() == 0 {
			t.Fatalf("render=%v output=%q", renderErr, out.String())
		}
	}
	if (&CreationError{}).Unwrap() != nil {
		t.Fatal("nil creation cause unwrap was non-nil")
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

	registered := func(regs ...awfgit.WorktreeRegistration) Runner {
		return &checkoutStub{worktreeList: func(context.Context) ([]awfgit.WorktreeRegistration, error) { return regs, nil }}
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
