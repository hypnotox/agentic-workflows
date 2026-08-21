// Package outputplan owns immutable semantic output declarations and plans.
package outputplan

import "slices"

// TreeReader is the read-only operation-tree authority used during planning.
type TreeReader interface {
	ReadFile(path string) ([]byte, bool, error)
	Paths(prefix string) ([]string, error)
}

// ArtifactRole classifies an input to a planned output.
type ArtifactRole string

const (
	ArtifactConfig             ArtifactRole = "config"
	ArtifactLock               ArtifactRole = "lock"
	ArtifactManifest           ArtifactRole = "manifest"
	ArtifactTemplate           ArtifactRole = "template"
	ArtifactConventionPart     ArtifactRole = "convention-part"
	ArtifactAuthoredData       ArtifactRole = "authored-data"
	ArtifactTopicMetadata      ArtifactRole = "topic-metadata"
	ArtifactClaimPart          ArtifactRole = "claim-part"
	ArtifactDecisionRecord     ArtifactRole = "decision-record"
	ArtifactManagedOutput      ArtifactRole = "managed-output"
	ArtifactProtocolDescriptor ArtifactRole = "protocol-descriptor"
)

// Policy is the complete lifecycle policy attached to an output.
type Policy struct {
	ValidateFrontmatter bool
	ScanReferences      bool
	ScanSkillReferences bool
	Regenerate          bool
}

// Input is one semantic input consumed by an output.
type Input struct {
	path string
	role ArtifactRole
}

func NewInput(path string, role ArtifactRole) Input { return Input{path: path, role: role} }
func (i Input) Path() string                        { return i.path }
func (i Input) Role() ArtifactRole                  { return i.role }

// Declaration records one deterministic output and its semantic inputs.
type Declaration struct {
	path, templateID string
	declarers        []string
	inputs           []Input
	dependencies     []string
}

func NewDeclaration(path, templateID string, declarers []string, inputs []Input, dependencies []string) Declaration {
	return Declaration{path: path, templateID: templateID, declarers: slices.Clone(declarers), inputs: slices.Clone(inputs), dependencies: slices.Clone(dependencies)}
}
func (d Declaration) Path() string        { return d.path }
func (d Declaration) TemplateID() string  { return d.templateID }
func (d Declaration) Declarers() []string { return slices.Clone(d.declarers) }
func (d Declaration) Inputs() []Input     { return slices.Clone(d.inputs) }

// Recipe is the normalized output-affecting identity used for coalescing.
type Recipe struct {
	templateID, templateHash, configHash string
	policy                               Policy
	encoder, provenance                  string
}

func NewRecipe(templateID, templateHash, configHash string, policy Policy, encoder, provenance string) Recipe {
	return Recipe{templateID: templateID, templateHash: templateHash, configHash: configHash, policy: policy, encoder: encoder, provenance: provenance}
}

// OutputSpec is the construction projection for one rendered output. Publisher
// consumes it immediately; Output stores every slice defensively.
type OutputSpec struct {
	Path, Content, TemplateID, TemplateHash, ConfigHash string
	RegenChecked                                        bool
	Policy                                              Policy
	Declarer, DeclarerProjection, Encoder               string
	Provenance                                          int
	Assembled, Kind, Artifact                           string
	StubDefaults, StubParts, MarkerParts, PartVarRefs   []string
	ConsumedInputs                                      []Input
	ObservedTemplateID                                  string
}

// Output is one immutable rendered output carried by a plan.
type Output struct {
	spec OutputSpec
}

func NewOutput(spec OutputSpec) Output {
	spec.StubDefaults = slices.Clone(spec.StubDefaults)
	spec.StubParts = slices.Clone(spec.StubParts)
	spec.MarkerParts = slices.Clone(spec.MarkerParts)
	spec.PartVarRefs = slices.Clone(spec.PartVarRefs)
	spec.ConsumedInputs = slices.Clone(spec.ConsumedInputs)
	return Output{spec: spec}
}
func (o Output) Path() string               { return o.spec.Path }
func (o Output) Content() string            { return o.spec.Content }
func (o Output) TemplateID() string         { return o.spec.TemplateID }
func (o Output) TemplateHash() string       { return o.spec.TemplateHash }
func (o Output) ConfigHash() string         { return o.spec.ConfigHash }
func (o Output) RegenChecked() bool         { return o.spec.RegenChecked }
func (o Output) Policy() Policy             { return o.spec.Policy }
func (o Output) Declarer() string           { return o.spec.Declarer }
func (o Output) DeclarerProjection() string { return o.spec.DeclarerProjection }
func (o Output) Encoder() string            { return o.spec.Encoder }
func (o Output) Provenance() int            { return o.spec.Provenance }
func (o Output) Assembled() string          { return o.spec.Assembled }
func (o Output) StubDefaults() []string     { return slices.Clone(o.spec.StubDefaults) }
func (o Output) StubParts() []string        { return slices.Clone(o.spec.StubParts) }
func (o Output) MarkerParts() []string      { return slices.Clone(o.spec.MarkerParts) }
func (o Output) Kind() string               { return o.spec.Kind }
func (o Output) Artifact() string           { return o.spec.Artifact }
func (o Output) PartVarRefs() []string      { return slices.Clone(o.spec.PartVarRefs) }

// NodeSpec is Publisher's construction projection for one plan node.
type NodeSpec struct {
	Path                string
	Recipe              Recipe
	Policy              Policy
	Declarers           []string
	DeclarerProjections []string
	DependsOn           []string
	ConsumedInputs      []Input
	ObservedTemplateID  string
	Output              *Output
}

// Node is one immutable path in a Plan.
type Node struct{ spec NodeSpec }

func NewNode(spec NodeSpec) Node {
	spec.Declarers = slices.Clone(spec.Declarers)
	spec.DeclarerProjections = slices.Clone(spec.DeclarerProjections)
	spec.DependsOn = slices.Clone(spec.DependsOn)
	spec.ConsumedInputs = slices.Clone(spec.ConsumedInputs)
	if spec.Output != nil {
		copy := *spec.Output
		spec.Output = &copy
	}
	return Node{spec: spec}
}
func (n Node) Path() string        { return n.spec.Path }
func (n Node) Declarers() []string { return slices.Clone(n.spec.Declarers) }
func (n Node) Output() (Output, bool) {
	if n.spec.Output == nil {
		return Output{}, false
	}
	return *n.spec.Output, true
}

// Plan is the immutable desired-output authority for one operation.
type Plan struct {
	nodes        []Node
	declarations []Declaration
}

func New(nodes []Node, declarations []Declaration) Plan {
	return Plan{nodes: slices.Clone(nodes), declarations: slices.Clone(declarations)}
}
func (p Plan) Nodes() []Node               { return slices.Clone(p.nodes) }
func (p Plan) Declarations() []Declaration { return slices.Clone(p.declarations) }
func (p Plan) Paths() []string {
	out := make([]string, 0, len(p.nodes))
	for _, node := range p.nodes {
		out = append(out, node.Path())
	}
	return out
}
func (p Plan) Outputs() []Output {
	out := make([]Output, 0, len(p.nodes))
	for _, node := range p.nodes {
		if output, ok := node.Output(); ok {
			out = append(out, output)
		}
	}
	return out
}
