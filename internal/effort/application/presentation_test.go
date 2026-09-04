package application

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/effort"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

func renderDocument(t *testing.T, document presentation.Document) string {
	t.Helper()
	var output bytes.Buffer
	if err := presentation.Render(&output, document); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func renderDiagnostic(t *testing.T, err error) string {
	t.Helper()
	presented, ok := presentError(err).(interface {
		Diagnostic() (presentation.Diagnostic, error)
	})
	if !ok {
		t.Fatalf("error has no application diagnostic: %T", err)
	}
	diagnostic, mapErr := presented.Diagnostic()
	if mapErr != nil {
		t.Fatal(mapErr)
	}
	document, documentErr := diagnostic.Document()
	if documentErr != nil {
		t.Fatal(documentErr)
	}
	return renderDocument(t, document)
}

func TestEffortAndWorktreeResultsMapAtTheApplicationBoundary(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repository  root")
	managed := filepath.Join(root, ".awf", "worktrees", "worktree  root")
	record := effort.Record{Slug: "worktree-root", Title: "Worktree root", MemoryPath: ".awf/efforts/worktree-root/memory.md"}
	result := worktree.Result{
		Condition: "managed worktree added", ChangedTopology: true,
		Path: managed, Branch: "awf/worktree-root", NextAction: "continue the effort in " + managed,
	}
	document, err := newDocument(record, result)
	if err != nil {
		t.Fatal(err)
	}
	want := "status: managed worktree added\n\nmutation:\n  identity:\n    effort: worktree-root\n    title: Worktree root\n    memory: .awf/efforts/worktree-root/memory.md\n    worktree: " + managed + "\n    branch: awf/worktree-root\n  changes:\n    completed:\n      managed topology\n  next actions:\n    step 1: continue the effort in " + managed + "\n"
	if got := renderDocument(t, document); got != want {
		t.Fatalf("application result = %q, want %q", got, want)
	}

	for _, invalid := range []worktree.Result{
		{Condition: "managed worktree added", Path: "bad\npath", NextAction: "continue"},
		{Condition: "managed worktree added", Path: managed, NextAction: "continue\nnow"},
	} {
		if _, err := worktreeMutation(invalid); err == nil || !strings.Contains(err.Error(), "line break") {
			t.Fatalf("invalid worktree result %#v error = %v", invalid, err)
		}
	}
	if _, err := newDocument(effort.Record{}, worktree.Result{}); err == nil {
		t.Fatal("empty new result accepted")
	}
	if _, err := finishDocument(effort.FinishResult{ArchivePath: "bad\npath"}, "demo"); err == nil {
		t.Fatal("multiline archive path accepted")
	}
	if _, err := finishDocument(effort.FinishResult{ArchivePath: ".awf/effort-archive/id-demo"}, " \t\n"); err == nil {
		t.Fatal("blank finish slug accepted")
	}
}

func TestApplicationMapsTopologyDiagnosticsAndPreservesMechanismIdentity(t *testing.T) {
	mechanism := errors.New("probe failed")
	cases := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "unchanged refusal",
			err:  &worktree.RefusalError{Category: "topology", Condition: "path exists", NextActions: []string{"inspect the existing path", "perform safe manual cleanup", "retry add"}},
			want: "condition: path exists\nstate: topology\n\ndiagnostic:\n  changed:\n    managed topology: no\n  steps:\n    step 1: inspect the existing path\n    step 2: perform safe manual cleanup\n    step 3: retry add\n",
		},
		{
			name: "changed refusal",
			err:  &worktree.RefusalError{Category: "operation", Condition: "removal probe failed", ChangedTopology: true, NextActions: []string{"retry"}, Err: mechanism},
			want: "condition: removal probe failed\nstate: operation\ncause: probe failed\n\ndiagnostic:\n  changed:\n    managed topology: yes\n  steps:\n    step 1: retry\n",
		},
		{
			name: "creation",
			err: &CreationError{Message: "creation", Condition: "creation failed", ChangedEffort: true, ChangedTopology: true,
				Cause: errors.New("add failed"), Steps: []string{"inspect", "retry"}},
			want: "condition: creation failed\nstate: operation\ncause: add failed\n\ndiagnostic:\n  changed:\n    effort resident: yes\n    managed topology: yes\n  steps:\n    step 1: inspect\n    step 2: retry\n",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			presented := presentError(test.err)
			if !errors.Is(presented, test.err) {
				t.Fatalf("presented error lost original identity: %v", presented)
			}
			if got := renderDiagnostic(t, test.err); got != test.want {
				t.Fatalf("diagnostic = %q, want %q", got, test.want)
			}
		})
	}

	if got := mechanismCauseText(&CreationError{Cause: &worktree.RefusalError{Err: mechanism}}); got != "probe failed" {
		t.Fatalf("mechanism causes = %q", got)
	}
	if (&CreationError{}).Unwrap() != nil {
		t.Fatal("empty creation error has causes")
	}
	one := &CreationError{Cause: mechanism}
	if !errors.Is(one, mechanism) {
		t.Fatal("creation error lost sole cause")
	}
}

func TestApplicationDiagnosticValidationRejectsInvalidSemanticValues(t *testing.T) {
	for _, err := range []error{
		&worktree.RefusalError{NextActions: []string{"retry\nnow"}},
		&CreationError{Steps: []string{"retry\rnow"}},
	} {
		presented := presentError(err).(interface {
			Diagnostic() (presentation.Diagnostic, error)
		})
		if _, mapErr := presented.Diagnostic(); mapErr == nil || !strings.Contains(mapErr.Error(), "line break") {
			t.Fatalf("invalid diagnostic error = %v", mapErr)
		}
	}
	presented := presentError(&worktree.RefusalError{}).(interface {
		Diagnostic() (presentation.Diagnostic, error)
	})
	if _, err := presented.Diagnostic(); err == nil || !strings.Contains(err.Error(), "modeled recovery actions") {
		t.Fatalf("missing recovery actions error = %v", err)
	}
}
