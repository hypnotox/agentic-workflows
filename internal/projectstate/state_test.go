package projectstate

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/resident"
)

func TestProjectStateDefensivelyOwnsLoadedFacts(t *testing.T) {
	cfg := &config.Config{Vars: map[string]any{"key": "original"}}
	targets := []Target{{Capabilities: []Capability{CapabilitySubagentTools}, Outputs: []TargetOutput{{Inputs: []TargetOutputInput{{Path: "input", Role: ArtifactTemplate}}}}}}
	state, err := New("root", resident.NewRoots("root", "resident"), true, cfg, catalog.Standard, catalog.Standard, targets)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Vars["key"] = "changed"
	targets[0].Capabilities[0] = CapabilitySessionHandoff
	targets[0].Outputs[0].Inputs[0].Path = "changed"
	if got := state.Config().Vars["key"]; got != "original" {
		t.Fatalf("config alias = %v, want original", got)
	}
	got := state.Targets()
	if got[0].Capabilities[0] != CapabilitySubagentTools || got[0].Outputs[0].Inputs[0].Path != "input" {
		t.Fatalf("loaded targets = %#v", got)
	}
	got[0].Capabilities[0] = CapabilitySessionHandoff
	got[0].Outputs[0].Inputs[0].Path = "returned-copy"
	again := state.Targets()
	if again[0].Capabilities[0] != CapabilitySubagentTools || again[0].Outputs[0].Inputs[0].Path != "input" {
		t.Fatalf("returned targets alias state = %#v", again)
	}
	if state.Root() != "root" || !state.Nested() || state.Roots() != resident.NewRoots("root", "resident") {
		t.Fatalf("loaded facts = root=%q nested=%t roots=%#v", state.Root(), state.Nested(), state.Roots())
	}
	if state.Catalog() == state.CompleteCatalog() {
		t.Fatal("selected and complete catalogs share one mutable snapshot")
	}
}

func TestTargetAccessorsExposeDeclaredFacts(t *testing.T) {
	target := BuiltinTarget("pi")
	if target.Name != "pi" || !HasCapability(target, CapabilitySubagentTools) {
		t.Fatalf("built-in target = %#v", target)
	}
}

func TestNewDerivedCopiesTargets(t *testing.T) {
	targets := []Target{{Capabilities: []Capability{CapabilitySubagentTools}}}
	state := NewDerived("", resident.Roots{}, false, catalog.Standard, catalog.Standard, targets)
	targets[0].Capabilities[0] = CapabilitySessionHandoff
	if got := state.Targets()[0].Capabilities[0]; got != CapabilitySubagentTools {
		t.Fatalf("derived targets alias = %q", got)
	}
}
