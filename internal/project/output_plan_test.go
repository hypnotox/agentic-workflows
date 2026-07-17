package project

import (
	"testing"
)

// invariant: output-plan-complete
// invariant: shared-output-coalesced
// invariant: target-capabilities-closed
func TestOutputPlanContainsWritesGeneratedNodesAndReservations(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: [mine]\nagents: []\ndomains: [rendering]\ntargets: [pi]\n", map[string]string{"skills/mine.yaml": "local: true\n"})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	op, err := p.OutputPlan()
	if err != nil {
		t.Fatal(err)
	}
	var reservation, cref bool
	for _, n := range op.Nodes {
		if n.Reservation && n.Path == ".pi/skills/example-mine/SKILL.md" {
			reservation = true
		}
		if n.Path == "docs/config-reference.md" {
			cref = true
			if len(n.DependsOn) == 0 {
				t.Error("config reference has no dependencies")
			}
			for _, dep := range n.DependsOn {
				if dep == n.Path {
					t.Error("config reference has self dependency")
				}
			}
		}
	}
	if !reservation || !cref {
		t.Fatalf("plan missing reservation=%v config-reference=%v: %#v", reservation, cref, op.Nodes)
	}
	files := op.writeFiles()
	for _, f := range files {
		if f.Path == ".pi/skills/example-mine/SKILL.md" {
			t.Fatal("reservation was rendered")
		}
	}
}

func TestTargetDescriptorValidation(t *testing.T) {
	for _, target := range []Target{
		{Name: "bad", BridgeFile: "X"},
		{Name: "bad", Capabilities: []Capability{"unknown"}},
		{Name: "bad", Outputs: []TargetOutput{{Path: "../bad", TemplateID: "x"}}},
	} {
		if err := target.validate(); err == nil {
			t.Fatalf("invalid target %#v was accepted", target)
		}
	}
	if got := piTarget.targetTemplateData()["targetSubagentTools"]; got != true {
		t.Fatalf("Pi capability projection = %#v", got)
	}
	if _, err := resolveTargets([]string{"nope"}); err == nil {
		t.Fatal("unknown target resolved")
	}
}

// invariant: output-policy-explicit
func TestOutputPolicyIsExplicit(t *testing.T) {
	if got := policyFor("agents", "anything", false); !got.ValidateFrontmatter || !got.ScanReferences {
		t.Fatalf("agent policy = %#v", got)
	}
	if got := policyFor("target-output", "anything", false); got.ScanReferences {
		t.Fatalf("target output policy = %#v", got)
	}
	if got := policyFor("docs", memoryTID, false); got.ScanReferences {
		t.Fatalf("memory policy = %#v", got)
	}
	if (OutputPolicy{}).ScanReferences {
		t.Fatal("zero policy must not scan")
	}
}
