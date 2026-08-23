package upgrade

import (
	"bytes"
	"errors"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestUpgradeFailureDiagnosticCarriesChangedAxisRecovery(t *testing.T) {
	failure := errors.New("terminal sync failed")
	diagnostic, err := (upgradeFailure{changes: []string{"first: changed config", "second: wrote lock"}, cause: failure}).Diagnostic()
	if err != nil {
		t.Fatal(err)
	}
	document, err := diagnostic.Document()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := presentation.Render(&out, document); err != nil {
		t.Fatal(err)
	}
	want := "condition: upgrade has not reached terminal sync\nstate: operation\ncause: terminal sync failed\n\ndiagnostic:\n  changed:\n    migration: change: first: changed config\n    migration: change: second: wrote lock\n  steps:\n    step 1: run awf upgrade --recover if an upgrade journal exists\n    step 2: inspect the listed changed axes\n    step 3: restore the project from version control if recovery cannot complete\n"
	if out.String() != want {
		t.Fatalf("diagnostic = %q, want %q", out.String(), want)
	}
}

func TestUpgradeFailureDiagnosticUsesRetryBeforeAnyChange(t *testing.T) {
	diagnostic, err := (upgradeFailure{cause: errors.New("first migration pre-write sync failed")}).Diagnostic()
	if err != nil {
		t.Fatal(err)
	}
	if diagnostic.State != "operation" || len(diagnostic.Changed) != 0 || len(diagnostic.Steps) != 1 {
		t.Fatalf("diagnostic = %#v, want retry-only operation diagnostic", diagnostic)
	}
}

func TestOperationPresentationPropagatesInvalidSemanticValues(t *testing.T) {
	cause := errors.New("failure")
	if got := newUpgradeFailure(nil, nil, presentation.Mutation{}, cause).Error(); got != cause.Error() {
		t.Fatalf("error = %q", got)
	}
	if got := newJournalFailure("condition", Outcome{}, cause).Error(); got != cause.Error() {
		t.Fatalf("error = %q", got)
	}
	if _, err := upgradeMutation(presentation.Mutation{}, []string{"\n"}, nil); err == nil {
		t.Fatal("invalid migration identity accepted")
	}
	if _, err := upgradeMutation(presentation.Mutation{}, nil, []string{"\n"}); err == nil {
		t.Fatal("invalid migration change accepted")
	}
}

func TestUpgradeFailureDiagnosticRejectsInvalidOwnerFacts(t *testing.T) {
	for _, failure := range []upgradeFailure{
		{changes: []string{"\n"}, cause: errors.New("migration failure")},
		{sync: presentation.Mutation{Changes: []presentation.MutationChange{{Label: "invalid_label", Values: []presentation.Value{mustProse(t, "changed output")}}}}, cause: errors.New("sync failure")},
	} {
		if _, err := failure.Diagnostic(); err == nil {
			t.Fatalf("failure %#v produced a diagnostic despite invalid owner facts", failure)
		}
	}
}

func TestJournalFailureUsesTerminalChangedAxes(t *testing.T) {
	failure := journalFailure{condition: "recovery has not reached terminal state", outcome: Outcome{Evidence: []Evidence{{Action: "applied", Path: ".awf/config.yaml"}}, Changed: []Evidence{}}, cause: errors.New("recovery failed")}
	diagnostic, err := failure.Diagnostic()
	if err != nil {
		t.Fatal(err)
	}
	if diagnostic.State != "operation" || len(diagnostic.Changed) != 0 || len(diagnostic.Steps) != 1 {
		t.Fatalf("diagnostic = %#v, want terminal retry-only diagnostic", diagnostic)
	}
}
