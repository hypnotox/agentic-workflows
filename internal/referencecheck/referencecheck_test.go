package referencecheck

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/outputplan"
)

func TestCheckPreservesReferenceEvidence(t *testing.T) {
	out := outputplan.NewOutput(outputplan.OutputSpec{Path: "docs/a.md", Content: "[bad](missing.md) demo-debugging", Policy: outputplan.Policy{ScanReferences: true, ScanSkillReferences: true}})
	plan := outputplan.New([]outputplan.Node{outputplan.NewNode(outputplan.NodeSpec{Path: "docs/a.md", Output: &out})})
	got, err := Check(plan, "demo", map[string]bool{"awf-effort": true}, func(string) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	findings := got.Findings()
	if len(findings) != 2 || findings[0].Evidence.Kind != "dead-reference" || findings[0].Evidence.Path != "docs/a.md" || findings[1].Evidence.Kind != "dead-skill-reference" {
		t.Fatalf("findings=%#v", findings)
	}
}

func TestCheckRecognizesFixedAndRetiredAWFSkillReferences(t *testing.T) {
	out := outputplan.NewOutput(outputplan.OutputSpec{
		Path:    "AGENTS.md",
		Content: "awf-effort demo-debugging agentic-debugging",
		Policy:  outputplan.Policy{ScanSkillReferences: true},
	})
	plan := outputplan.New([]outputplan.Node{outputplan.NewNode(outputplan.NodeSpec{Path: "AGENTS.md", Output: &out})})
	got, err := Check(plan, "demo", map[string]bool{"awf-effort": true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	findings := got.Findings()
	if len(findings) != 1 || findings[0].Evidence.Detail != "demo-debugging" {
		t.Fatalf("findings = %#v", findings)
	}
}

func TestCheckGivesCanonicalAgenticSkillsPrecedenceOverLegacyPrefix(t *testing.T) {
	out := outputplan.NewOutput(outputplan.OutputSpec{
		Path:    "AGENTS.md",
		Content: "agentic-debugging agentic-planning",
		Policy:  outputplan.Policy{ScanSkillReferences: true},
	})
	plan := outputplan.New([]outputplan.Node{outputplan.NewNode(outputplan.NodeSpec{Path: "AGENTS.md", Output: &out})})
	got, err := Check(plan, "agentic", map[string]bool{"awf-effort": true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if findings := got.Findings(); len(findings) != 0 {
		t.Fatalf("canonical external skills were reinterpreted as legacy AWF skills: %#v", findings)
	}
}
