package outputplan

import (
	"reflect"
	"testing"
)

func TestPlanAndNodesAreDefensiveValues(t *testing.T) {
	outputParts := []string{"part"}
	output := NewOutput(OutputSpec{Path: "out", StubParts: outputParts})
	nodeDeclarers := []string{"publisher"}
	node := NewNode(NodeSpec{Path: "out", Declarers: nodeDeclarers, Output: &output})
	nodes := []Node{node}
	plan := New(nodes)

	outputParts[0], nodeDeclarers[0] = "mutated", "mutated"
	nodes[0] = Node{}

	gotOutput, ok := plan.Nodes()[0].Output()
	if !ok || !reflect.DeepEqual(gotOutput.StubParts(), []string{"part"}) {
		t.Fatalf("output aliases construction input: %#v, %t", gotOutput, ok)
	}

	projectedNodes := plan.Nodes()
	projectedOutputs := plan.Outputs()
	projectedNodes[0], projectedOutputs[0] = Node{}, Output{}
	if plan.Outputs()[0].Path() != "out" {
		t.Fatal("plan projections mutate stored values")
	}
}

func TestNodeAndOutputSliceProjectionsAreDefensive(t *testing.T) {
	output := NewOutput(OutputSpec{Path: "out", StubDefaults: []string{"default"}, StubParts: []string{"part"}, MarkerParts: []string{"marker"}, PartVarRefs: []string{"var"}})
	node := NewNode(NodeSpec{Path: "out", Declarers: []string{"one"}, DeclarerProjections: []string{"projection"}, DependsOn: []string{"dependency"}, Output: &output})

	for _, values := range [][]string{node.Declarers(), output.StubDefaults(), output.StubParts(), output.MarkerParts(), output.PartVarRefs()} {
		values[0] = "mutated"
	}

	if node.Declarers()[0] != "one" {
		t.Fatal("node projection aliases stored slices")
	}
	if output.StubDefaults()[0] != "default" || output.StubParts()[0] != "part" || output.MarkerParts()[0] != "marker" || output.PartVarRefs()[0] != "var" {
		t.Fatal("output projection aliases stored slices")
	}
}
