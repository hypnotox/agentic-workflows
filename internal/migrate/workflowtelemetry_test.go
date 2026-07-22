package migrate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

func TestWorkflowTelemetryMigration(t *testing.T) {
	// invariant: config/migrations-and-locks:workflow-telemetry-config-migration
	root := t.TempDir()
	awf := filepath.Join(root, ".awf")
	if err := os.MkdirAll(awf, 0o755); err != nil {
		t.Fatal(err)
	}
	original := "# leading\nprefix: x\nworkflowTelemetry:\n  retention:\n    maxCompletedEffortCount: 7 # keep\n"
	if err := os.WriteFile(config.ConfigPath(root), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := applyWorkflowTelemetry(root, &out); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(config.ConfigPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "maxCompletedEffortCount: 7 # keep") || !strings.Contains(string(got), "phaseDurationSeconds: 14400") {
		t.Fatalf("migrated config:\n%s", got)
	}
	if out.String() != "workflow-telemetry: added workflowTelemetry defaults\n" {
		t.Fatalf("output = %q", out.String())
	}
	out.Reset()
	if err := applyWorkflowTelemetry(root, &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("idempotent output = %q", out.String())
	}
	if err := applyWorkflowTelemetry(t.TempDir(), &out); err != nil {
		t.Fatalf("absent config: %v", err)
	}
	badRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(badRoot, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigPath(badRoot), []byte("prefix: [bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyWorkflowTelemetry(badRoot, &out); err == nil {
		t.Fatal("malformed config accepted")
	}
	if err := os.WriteFile(config.ConfigPath(badRoot), []byte("workflowTelemetry: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyWorkflowTelemetry(badRoot, &out); err == nil {
		t.Fatal("non-mapping workflowTelemetry accepted")
	}
}
