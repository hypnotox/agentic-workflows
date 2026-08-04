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
	for _, test := range []struct {
		name string
		err  interface {
			Diagnostic() (presentation.Diagnostic, error)
		}
		want string
	}{
		{
			name: "unchanged refusal",
			err:  &RefusalError{Category: "topology", Condition: "path exists", ChangedTopology: false, NextActions: []string{"inspect the existing path", "perform safe manual cleanup", "retry add"}},
			want: "condition: path exists\nstate: topology\n\ndiagnostic:\n  changed:\n    managed topology: no\n  steps:\n    step 1: inspect the existing path\n    step 2: perform safe manual cleanup\n    step 3: retry add\n",
		},
		{
			name: "changed refusal",
			err:  &RefusalError{Category: "operation", Condition: "removal probe failed", ChangedTopology: true, NextActions: []string{"run `git worktree list --porcelain`", "inspect the managed path and branch", "resolve the reported probe failure", "retry ordinary removal"}, Err: errors.New("probe failed")},
			want: "condition: removal probe failed\nstate: operation\ncause: probe failed\n\ndiagnostic:\n  changed:\n    managed topology: yes\n  steps:\n    step 1: run `git worktree list --porcelain`\n    step 2: inspect the managed path and branch\n    step 3: resolve the reported probe failure\n    step 4: retry ordinary removal\n",
		},
		{
			name: "creation with rollback cause",
			err:  &CreationError{Message: "creation", Condition: "creation failed", ChangedEffort: true, ChangedTopology: true, Cause: errors.New("add failed"), RollbackCause: errors.New("rollback failed"), Steps: []string{"inspect", "retry"}},
			want: "condition: creation failed\nstate: operation\ncause: add failed | rollback failed\n\ndiagnostic:\n  changed:\n    effort resident: yes\n    managed topology: yes\n  steps:\n    step 1: inspect\n    step 2: retry\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostic, diagnosticErr := test.err.Diagnostic()
			if diagnosticErr != nil {
				t.Fatal(diagnosticErr)
			}
			document, documentErr := diagnostic.Document()
			if documentErr != nil {
				t.Fatal(documentErr)
			}
			var out bytes.Buffer
			if renderErr := presentation.Render(&out, document); renderErr != nil {
				t.Fatal(renderErr)
			}
			if out.String() != test.want {
				t.Fatalf("diagnostic = %q, want %q", out.String(), test.want)
			}
		})
	}
	if (&CreationError{}).Unwrap() != nil {
		t.Fatal("nil creation cause unwrap was non-nil")
	}
	addCause := errors.New("add failed")
	rollbackCause := errors.New("rollback failed")
	creation := &CreationError{Cause: addCause, RollbackCause: rollbackCause}
	if !errors.Is(creation, addCause) || !errors.Is(creation, rollbackCause) {
		t.Fatalf("creation error lost mechanism identity: %v", creation)
	}
	if !errors.Is(&CreationError{Cause: addCause}, addCause) {
		t.Fatal("creation error lost its sole add mechanism identity")
	}
}

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
	if _, err := (&RefusalError{}).Diagnostic(); err == nil || !strings.Contains(err.Error(), "modeled recovery actions") {
		t.Fatalf("refusal without modeled actions diagnostic error = %v", err)
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
