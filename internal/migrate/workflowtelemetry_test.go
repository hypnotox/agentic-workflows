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
	if !strings.HasPrefix(string(got), "# leading\nprefix: x\nworkflowTelemetry:\n") || !strings.Contains(string(got), "maxCompletedEffortCount: 7 # keep") {
		t.Fatalf("migration did not preserve unrelated order/comment:\n%s", got)
	}
	cfg, err := config.Parse(filepath.Join(root, ".awf"), got)
	if err != nil {
		t.Fatal(err)
	}
	want := config.DefaultWorkflowTelemetryConfig()
	want.Retention.MaxCompletedEffortCount = 7
	if cfg.WorkflowTelemetry != want {
		t.Fatalf("migrated defaults = %#v, want %#v", cfg.WorkflowTelemetry, want)
	}
	if Current() != 17 {
		t.Fatalf("current schema = %d, want 17", Current())
	}
	first := append([]byte(nil), got...)
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
	second, err := os.ReadFile(config.ConfigPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("idempotent migration changed bytes")
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

	registryRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(registryRoot, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	authored := "prefix: registry\nworkflowTelemetry:\n  retention:\n    maxCompletedEffortAgeDays: 0\n    maxCompletedEffortCount: 7\n  widget:\n    enabled: false\n    showCost: false\n  diagnostics:\n    heuristicsEnabled: false\n    minimumBaselineSamples: 4\n    baselinePercentile: 80\n    thresholds:\n      phaseReentryCount: 4\n      phaseDurationSeconds: 5\n      phaseTokens: 6\n      compactionCount: 7\n      handoffCount: 8\n      toolFailureCount: 9\n      gateFailureCount: 10\n      cacheReadPercentBelow: 11\n      subagentQueueWaitSeconds: 12\n      implementationReworkCount: 13\n"
	if err := os.WriteFile(config.ConfigPath(registryRoot), []byte(authored), 0o644); err != nil {
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
	if len(applied) != 1 || applied[0] != "workflow-telemetry" {
		t.Fatalf("registry applied = %v", applied)
	}
	upgraded, err := manifest.Load(config.LockPath(registryRoot))
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.SchemaVersion != 17 {
		t.Fatalf("registry schema = %d", upgraded.SchemaVersion)
	}
	body, err := os.ReadFile(config.ConfigPath(registryRoot))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != authored {
		t.Fatal("registry migration changed fully authored non-default telemetry")
	}
}
