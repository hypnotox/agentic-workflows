package migrate

import (
	"bytes"
	"strings"
	"testing"
)

// invariant: config/migrations-and-locks:workflow-telemetry-config-migration
func TestDropWorkflowTelemetryRegistered(t *testing.T) {
	if Current() != 28 {
		t.Fatal(Current())
	}
}

func TestConfigForCurrentSchemaDropsHistoricalWorkflowTelemetry(t *testing.T) {
	src := []byte("# retained header\nprefix: example\nvars:\n  name: value\nworkflowTelemetry:\n  retention:\n    maxCompletedEffortAgeDays: 90\nrunner:\n  enabled: true\n")
	got, err := ConfigForCurrentSchema(src, 19)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("workflowTelemetry")) {
		t.Fatalf("retired block remains:\n%s", got)
	}
	for _, want := range []string{"# retained header", "prefix: example", "name: value", "runner:", "enabled: true"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("migration lost unrelated YAML %q:\n%s", want, got)
		}
	}
	again, err := ConfigForCurrentSchema(got, 19)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again, got) {
		t.Fatalf("absent block changed on retry:\n got %s\nwant %s", again, got)
	}
}

func TestConfigForCurrentSchemaRefusesMalformedYAML(t *testing.T) {
	if _, err := ConfigForCurrentSchema([]byte("prefix: [\nworkflowTelemetry: {}\n"), 19); err == nil {
		t.Fatal("malformed YAML accepted")
	}
	if _, err := ConfigForCurrentSchema([]byte("prefix: example\n"), Current()+1); err == nil {
		t.Fatal("ahead schema accepted")
	}
}
