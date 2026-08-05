package upgrade

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestOutcomePresentation(t *testing.T) {
	outcome := Outcome{Evidence: []Evidence{{Action: "applied", Path: ".awf/config.yaml"}}, Changed: []Evidence{{Action: "committed", Path: LockRel()}}}
	for _, tc := range []struct {
		name string
		mapf func() (presentation.Mutation, error)
		want string
	}{
		{"completed", outcome.CompletedMutation, "status: upgrade completed\n\nmutation:\n  changes:\n    journal:\n      applied: .awf/config.yaml\n"},
		{"recovered", outcome.RecoveredMutation, "status: upgrade recovered\n\nmutation:\n  changes:\n    journal:\n      applied: .awf/config.yaml\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutation, err := tc.mapf()
			if err != nil {
				t.Fatal(err)
			}
			document, err := mutation.Document()
			if err != nil {
				t.Fatal(err)
			}
			var rendered bytes.Buffer
			if err := presentation.Render(&rendered, document); err != nil {
				t.Fatal(err)
			}
			if got := rendered.String(); got != tc.want {
				t.Fatalf("rendered = %q, want %q", got, tc.want)
			}
		})
	}

	diagnostic, err := outcome.FailureDiagnostic("upgrade has not reached terminal state", errors.New("journal write failed"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := diagnostic.Document()
	if err != nil {
		t.Fatal(err)
	}
	var rendered bytes.Buffer
	if err := presentation.Render(&rendered, document); err != nil {
		t.Fatal(err)
	}
	want := "condition: upgrade has not reached terminal state\nstate: operation\ncause: journal write failed\n\ndiagnostic:\n  changed:\n    journal: committed: .awf/awf.lock\n  steps:\n    step 1: run awf upgrade --recover if an upgrade journal exists\n    step 2: inspect the listed changed axes\n    step 3: restore the project from version control if recovery cannot complete\n"
	if got := rendered.String(); got != want {
		t.Fatalf("diagnostic = %q, want %q", got, want)
	}
}

func TestOutcomePresentationRejectsLineBreakEvidence(t *testing.T) {
	for _, lineBreak := range []string{"\r", "\n"} {
		t.Run(fmt.Sprintf("%q", lineBreak), func(t *testing.T) {
			outcome := Outcome{Evidence: []Evidence{{Action: "applied", Path: "bad" + lineBreak + "path"}}}
			if _, err := outcome.RecoveredMutation(); err == nil {
				t.Fatal("RecoveredMutation accepted evidence with a line break")
			}
			if _, err := outcome.FailureDiagnostic("recovery failed", errors.New("cause")); err == nil {
				t.Fatal("FailureDiagnostic accepted evidence with a line break")
			}
		})
	}
}

func TestOutcomePresentationUsesTerminalAndNoChangeRemedies(t *testing.T) {
	outcome := Outcome{Evidence: []Evidence{{Action: "applied", Path: ".awf/config.yaml"}}, Changed: []Evidence{}}
	mutation, err := outcome.CompletedMutation()
	if err != nil {
		t.Fatal(err)
	}
	if len(mutation.Changes) != 1 {
		t.Fatalf("mutation = %#v, want journal evidence", mutation)
	}
	diagnostic, err := outcome.FailureDiagnostic("recovery has not reached terminal state", errors.New("cleanup failed"))
	if err != nil {
		t.Fatal(err)
	}
	if diagnostic.State != "operation" || len(diagnostic.Changed) != 0 || len(diagnostic.Steps) != 1 {
		t.Fatalf("diagnostic = %#v, want operation state and only a retry remedy", diagnostic)
	}

	legacy := Outcome{Evidence: []Evidence{{Action: "retained", Path: JournalPath("project")}}}
	diagnostic, err = legacy.FailureDiagnostic("upgrade has not reached terminal state", errors.New("legacy failure"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostic.Changed) != 1 {
		t.Fatalf("legacy diagnostic = %#v, want evidence fallback", diagnostic)
	}

	empty, err := (Outcome{}).CompletedMutation()
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Changes) != 0 {
		t.Fatalf("empty mutation = %#v, want no changes", empty)
	}
}
