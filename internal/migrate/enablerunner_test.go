package migrate

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// An absent runner key is seeded enabled: the schema 17 → 18 default-on port
// of the pure awf wrapper (ADR-0156).
// invariant: rendering/companion-scripts:runner-singleton-toggle
func TestEnableRunnerAdds(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, ".awf", "config.yaml")
	testsupport.WriteFile(t, cfg, "prefix: ex\nskills:\n  - tdd\n")
	if err := applyEnableRunner(root, io.Discard); err != nil {
		t.Fatalf("applyEnableRunner: %v", err)
	}
	out, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "runner:") || !strings.Contains(string(out), "enabled: true") {
		t.Errorf("runner.enabled not added:\n%s", out)
	}
	if !strings.Contains(string(out), "prefix: ex") || !strings.Contains(string(out), "- tdd") {
		t.Errorf("untouched keys lost:\n%s", out)
	}
}

// A config already carrying a runner key made a choice - a replay from a
// degraded lock must not override a deliberate opt-out with the upgrade
// default. The genuine 17→18 path has no runner key and still gets true.
// invariant: rendering/companion-scripts:runner-singleton-toggle
func TestEnableRunnerKeepsExplicitOptOut(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, ".awf", "config.yaml")
	src := "prefix: ex\nrunner:\n  enabled: false\n"
	testsupport.WriteFile(t, cfg, src)
	if err := applyEnableRunner(root, io.Discard); err != nil {
		t.Fatalf("applyEnableRunner: %v", err)
	}
	out, _ := os.ReadFile(cfg)
	if string(out) != src {
		t.Errorf("explicit runner opt-out overridden on replay:\n got %q\nwant %q", out, src)
	}
}

func TestEnableRunnerAbsentConfig(t *testing.T) {
	if err := applyEnableRunner(t.TempDir(), io.Discard); err != nil {
		t.Errorf("applyEnableRunner with no .awf/config.yaml should be a no-op, got %v", err)
	}
}

func TestEnableRunnerMalformedConfig(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "skills: [a, b\n")
	if err := applyEnableRunner(root, io.Discard); err == nil {
		t.Error("expected error surfaced from SetMappingScalar for malformed config.yaml")
	}
}

// The enable-runner migration is the schema 18 tip.
func TestEnableRunnerIsCurrent(t *testing.T) {
	if Current() != 18 {
		t.Errorf("Current() = %d, want 18", Current())
	}
}
