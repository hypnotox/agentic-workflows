package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// TestRunUpgradeRendersSuccessfulFinalJournalMutation retains the command
// boundary contract: the operation supplies semantic presentation and cmd/awf
// chooses the selected output stream.
func TestUpgradeGroundingCollisionAdaptsMigrationDiagnostic(t *testing.T) {
	adapter := upgradeGroundingCollision{&migrate.GroundingSkillCollisionError{Path: "skills/grounding.yaml"}}
	diagnostic, err := adapter.UpgradeDiagnostic([]string{"earlier migration changed config"})
	if err != nil {
		t.Fatal(err)
	}
	var rendered bytes.Buffer
	document, err := diagnostic.Document()
	if err != nil {
		t.Fatal(err)
	}
	if err := presentation.Render(&rendered, document); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered.String(), "migration: change: earlier migration changed config") {
		t.Fatalf("diagnostic = %q", rendered.String())
	}
	if adapter.Error() == "" || adapter.Unwrap() == nil {
		t.Fatal("adapter did not preserve the migration error")
	}
}

func TestRunUpgradeRendersSuccessfulFinalJournalMutation(t *testing.T) {
	root := scaffoldProject(t)
	var stdout bytes.Buffer
	if err := runUpgrade(testContext(t), root, &stdout); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "status: completed") {
		t.Fatalf("upgrade output = %q, want completed operation presentation", stdout.String())
	}
}
