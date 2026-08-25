package migrate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// invariant: config/migrations-and-locks:workflow-telemetry-config-migration (TestWorkflowTelemetryMigrationIsHistoricalInputForGeneration20)
func TestWorkflowTelemetryMigrationIsHistoricalInputForGeneration20(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, config.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigPath(root), []byte("prefix: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out Changes
	if err := applyWorkflowTelemetry(root, &out); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(config.ConfigPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(first), "workflowTelemetry:") || out.String() != "workflow-telemetry: added workflowTelemetry defaults\n" {
		t.Fatalf("schema-17 materialization = %q, output=%q", first, out.String())
	}
	out.Reset()
	if err := applyWorkflowTelemetry(root, &out); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(config.ConfigPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) || out.Len() != 0 {
		t.Fatalf("historical materialization was not idempotent: %q", out.String())
	}
	if err := applyWorkflowTelemetry(t.TempDir(), &out); err != nil {
		t.Fatalf("absent config: %v", err)
	}

	registryRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(registryRoot, config.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigPath(registryRoot), []byte("prefix: registry\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock := &manifest.Lock{AWFVersion: "0.21.0", SchemaVersion: 16, Files: map[string]manifest.Entry{}}
	if err := lock.Save(config.LockPath(registryRoot)); err != nil {
		t.Fatal(err)
	}
	applied, _, err := upgradeLegacyForTest(testContext(t), registryRoot)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, migration := range registry {
		if migration.To > 16 {
			want = append(want, migration.Name)
		}
	}
	if got := strings.Join(applied, ","); got != strings.Join(want, ",") {
		t.Fatalf("applied = %q, want %q", got, strings.Join(want, ","))
	}
	body, err := os.ReadFile(config.ConfigPath(registryRoot))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "workflowTelemetry") {
		t.Fatalf("generation 20 retained historical block:\n%s", body)
	}
}

func TestWorkflowTelemetryHistoricalMigrationRejectsMalformedYAML(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, config.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigPath(root), []byte("prefix: [bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyWorkflowTelemetry(root, &Changes{}); err == nil {
		t.Fatal("malformed config accepted")
	}
}
