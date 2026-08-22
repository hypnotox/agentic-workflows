package referencecheck

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/outputplan"
)

func TestADRRelatedAcceptsResolvedLink(t *testing.T) {
	got, err := ADRRelated([]ADR{{Number: "0001", Filename: "one.md"}, {Number: "0002", Filename: "two.md", Related: []int{1}}})
	if err != nil || len(got.Findings()) != 0 {
		t.Fatalf("findings=%#v err=%v", got, err)
	}
}
func TestADRRelatedPreservesLinkAndOrderEvidence(t *testing.T) {
	got, err := ADRRelated([]ADR{{Number: "0002", Filename: "two.md", Related: []int{3, 1}}})
	if err != nil {
		t.Fatal(err)
	}
	findings := got.Findings()
	if len(findings) != 3 || findings[0].Evidence.Kind != "adr-related-link" || findings[2].Evidence.Kind != "adr-related-order" {
		t.Fatalf("findings=%#v", findings)
	}
}
func TestCheckPreservesReferenceEvidence(t *testing.T) {
	out := outputplan.NewOutput(outputplan.OutputSpec{Path: "docs/a.md", Content: "[bad](missing.md) demo-tdd", Policy: outputplan.Policy{ScanReferences: true, ScanSkillReferences: true}})
	plan := outputplan.New([]outputplan.Node{outputplan.NewNode(outputplan.NodeSpec{Path: "docs/a.md", Output: &out})}, nil)
	got, err := Check(plan, "demo", map[string]bool{}, map[string]bool{"tdd": true}, func(string) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	findings := got.Findings()
	if len(findings) != 2 || findings[0].Evidence.Kind != "dead-reference" || findings[0].Evidence.Path != "docs/a.md" || findings[1].Evidence.Kind != "dead-skill-reference" {
		t.Fatalf("findings=%#v", findings)
	}
}
