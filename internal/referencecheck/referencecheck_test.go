package referencecheck

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/outputplan"
)

func TestCheckPreservesReferenceEvidence(t *testing.T) {
	out := outputplan.NewOutput(outputplan.OutputSpec{Path: "docs/a.md", Content: "[bad](missing.md) demo-tdd", Policy: outputplan.Policy{ScanReferences: true, ScanSkillReferences: true}})
	plan := outputplan.New([]outputplan.Node{outputplan.NewNode(outputplan.NodeSpec{Path: "docs/a.md", Output: &out})})
	got, err := Check(plan, "demo", map[string]bool{}, map[string]bool{"tdd": true}, func(string) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	findings := got.Findings()
	if len(findings) != 2 || findings[0].Evidence.Kind != "dead-reference" || findings[0].Evidence.Path != "docs/a.md" || findings[1].Evidence.Kind != "dead-skill-reference" {
		t.Fatalf("findings=%#v", findings)
	}
}
