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

// invariant: config/migrations-and-locks:workflow-telemetry-config-migration
func TestWorkflowTelemetryMigrationIsHistoricalInputForGeneration20(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, config.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigPath(root), []byte("prefix: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
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
	lock := &manifest.Lock{AWFVersion: "0.21.0", SchemaVersion: 16, Files: map[string]manifest.Entry{}, ADRFormatV1From: 1, ADRFormatV2From: 1, LegacyADRGaps: []int{}}
	if err := lock.Save(config.LockPath(registryRoot)); err != nil {
		t.Fatal(err)
	}
	applied, err := Upgrade(registryRoot, &out)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(applied, ","), "workflow-telemetry,enable-runner,rename-retired-commands,drop-workflow-telemetry,remove-workflow-residents,unified-effort-residents,implementer-agent-closure,explorer-grounding-closure"; got != want {
		t.Fatalf("applied = %q, want %q", got, want)
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
	if err := applyWorkflowTelemetry(root, &bytes.Buffer{}); err == nil {
		t.Fatal("malformed config accepted")
	}
}
