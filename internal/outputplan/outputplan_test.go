package outputplan

import (
	"reflect"
	"testing"
)

func TestPlanAndDeclarationsAreDefensiveValues(t *testing.T) {
	declarers := []string{"publisher"}
	inputs := []Input{NewInput("source", ArtifactTemplate)}
	dependencies := []string{"dependency"}
	declaration := NewDeclaration("out", "template", declarers, inputs, dependencies)
	outputParts := []string{"part"}
	output := NewOutput(OutputSpec{Path: "out", StubParts: outputParts, ConsumedInputs: inputs})
	nodeDeclarers := []string{"publisher"}
	node := NewNode(NodeSpec{Path: "out", Declarers: nodeDeclarers, ConsumedInputs: inputs, Output: &output})
	nodes := []Node{node}
	declarations := []Declaration{declaration}
	plan := New(nodes, declarations)

	declarers[0], inputs[0], dependencies[0], outputParts[0], nodeDeclarers[0] = "mutated", NewInput("mutated", ArtifactConfig), "mutated", "mutated", "mutated"
	nodes[0], declarations[0] = Node{}, Declaration{}

	if got := plan.Paths(); !reflect.DeepEqual(got, []string{"out"}) {
		t.Fatalf("paths after input mutation = %v", got)
	}
	gotDeclaration := plan.Declarations()[0]
	if !reflect.DeepEqual(gotDeclaration.Declarers(), []string{"publisher"}) || gotDeclaration.Inputs()[0].Path() != "source" {
		t.Fatalf("declaration aliases construction input: %#v", gotDeclaration)
	}
	gotOutput, ok := plan.Nodes()[0].Output()
	if !ok || !reflect.DeepEqual(gotOutput.StubParts(), []string{"part"}) {
		t.Fatalf("output aliases construction input: %#v, %t", gotOutput, ok)
	}

	projectedNodes := plan.Nodes()
	projectedDeclarations := plan.Declarations()
	projectedPaths := plan.Paths()
	projectedOutputs := plan.Outputs()
	projectedNodes[0], projectedDeclarations[0], projectedPaths[0], projectedOutputs[0] = Node{}, Declaration{}, "mutated", Output{}
	if plan.Paths()[0] != "out" || plan.Declarations()[0].Path() != "out" || plan.Outputs()[0].Path() != "out" {
		t.Fatal("plan projections mutate stored values")
	}
}

func TestDeclarationTemplateIDProjectsConstructionValue(t *testing.T) {
	declaration := NewDeclaration("declared", "declaration-template", nil, nil, nil)
	if declaration.TemplateID() != "declaration-template" {
		t.Fatalf("declaration template id = %q", declaration.TemplateID())
	}
}

func TestNodeAndOutputSliceProjectionsAreDefensive(t *testing.T) {
	output := NewOutput(OutputSpec{Path: "out", StubDefaults: []string{"default"}, StubParts: []string{"part"}, MarkerParts: []string{"marker"}, PartVarRefs: []string{"var"}, ConsumedInputs: []Input{NewInput("source", ArtifactTemplate)}})
	node := NewNode(NodeSpec{Path: "out", Declarers: []string{"one"}, DeclarerProjections: []string{"projection"}, DependsOn: []string{"dependency"}, ConsumedInputs: []Input{NewInput("source", ArtifactTemplate)}, Output: &output})

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
