package publisher

import (
	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/projectstate"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

type renderInputs struct {
	state   *projectstate.ProjectState
	cfg     *config.Config
	read    ProjectTreeReader
	version string
}

func newRenderInputs(state *projectstate.ProjectState, cfg *config.Config, read ProjectTreeReader, version string) renderInputs {
	return renderInputs{state: state, cfg: cfg, read: read, version: version}
}
func (p renderInputs) root() string              { return p.state.Root() }
func (p renderInputs) targets() []Target         { return p.state.Targets() }
func (p renderInputs) catalog() *catalog.Catalog { return p.state.Catalog() }

func projectCatalog(p renderInputs) *catalog.Catalog { return p.catalog() }
func fullProfile(p renderInputs) bool                { return p.cfg == nil || p.cfg.Profile != catalog.ProfileCore }

func deriveOperationStateWithPitfalls(p renderInputs) (adr.Corpus, pitfall.Corpus, topic.Corpus, map[string]bool, error) {
	corpus := adr.Corpus{}
	topics := topic.Corpus{}
	var err error
	if fullProfile(p) {
		corpus, err = adr.LoadCorpus(decisionsDir(p.root()))
		if err != nil {
			return adr.Corpus{}, pitfall.Corpus{}, topic.Corpus{}, nil, err
		}
		topics, err = topic.LoadCorpus(p.root(), p.cfg, corpus)
		if err != nil {
			return adr.Corpus{}, pitfall.Corpus{}, topic.Corpus{}, nil, err
		}
	}
	pitfalls, err := loadPitfallCorpus(p)
	if err != nil {
		return adr.Corpus{}, pitfall.Corpus{}, topic.Corpus{}, nil, err
	}
	eff, err := effectiveSkills(p)
	if err != nil {
		return adr.Corpus{}, pitfall.Corpus{}, topic.Corpus{}, nil, err
	}
	return corpus, pitfalls, topics, eff, nil
}

// Publisher is the sole output-plan construction and rendering coordinator.
type Publisher struct{ inputs renderInputs }

// New composes a Publisher from immutable loaded facts and an explicit operation tree reader.
func New(state *projectstate.ProjectState, cfg *config.Config, read ProjectTreeReader, version string) *Publisher {
	if state == nil || cfg == nil || read == nil {
		panic("publisher: missing composition dependency")
	}
	return &Publisher{inputs: newRenderInputs(state, cfg, read, version)}
}

// Plan derives exactly one immutable plan for this operation.
func (p *Publisher) Plan() (outputplan.Plan, error) {
	plan, err := outputPlan(p.inputs)
	if err != nil {
		return outputplan.Plan{}, err
	}
	return freezePlan(plan), nil
}

func freezeInputs(inputs []OutputInput) []outputplan.Input {
	out := make([]outputplan.Input, len(inputs))
	for i, input := range inputs {
		out[i] = outputplan.NewInput(input.Path, input.Role)
	}
	return out
}

func freezeOutput(file RenderedFile) outputplan.Output {
	return outputplan.NewOutput(outputplan.OutputSpec{
		Path: file.Path, Content: file.Content, TemplateID: file.TemplateID,
		TemplateHash: file.TemplateHash, ConfigHash: file.ConfigHash,
		RegenChecked: file.RegenChecked, Policy: file.Policy,
		Declarer: file.Declarer, DeclarerProjection: file.DeclarerProjection,
		Encoder: string(file.Encoder), Provenance: int(file.Provenance),
		Assembled: file.assembled, StubDefaults: file.stubDefaults,
		StubParts: file.stubParts, MarkerParts: file.markerParts,
		Kind: file.kind, Artifact: file.artifact, PartVarRefs: file.partVarRefs,
		ConsumedInputs: freezeInputs(file.ConsumedInputs), ObservedTemplateID: file.ObservedTemplateID,
	})
}

func freezePlan(plan *OutputPlan) outputplan.Plan {
	nodes := make([]outputplan.Node, 0, len(plan.Nodes))
	for _, node := range plan.Nodes {
		recipe := outputplan.NewRecipe(node.Recipe.TemplateID, node.Recipe.TemplateHash, node.Recipe.ConfigHash, node.Recipe.Policy, string(node.Recipe.Encoder), node.Recipe.Provenance)
		var output *outputplan.Output
		if node.file != nil {
			value := freezeOutput(*node.file)
			output = &value
		}
		nodes = append(nodes, outputplan.NewNode(outputplan.NodeSpec{
			Path: node.Path, Recipe: recipe, Policy: node.Policy,
			Declarers: node.Declarers, DeclarerProjections: node.DeclarerProjections,
			DependsOn: node.DependsOn, ConsumedInputs: freezeInputs(node.ConsumedInputs),
			ObservedTemplateID: node.ObservedTemplateID, Output: output,
		}))
	}
	declarations := make([]outputplan.Declaration, len(plan.Declarations))
	for i, declaration := range plan.Declarations {
		declarations[i] = outputplan.NewDeclaration(declaration.Path, declaration.TemplateID, declaration.Declarers, freezeInputs(declaration.Inputs), declaration.Dependencies)
	}
	return outputplan.New(nodes, declarations)
}

// BuildConfigReference derives the live configuration reference from this operation's plan.
func (p *Publisher) BuildConfigReference() (ConfigReference, error) {
	return configReferenceModel(p.inputs)
}

// PreflightLocalDoc validates one candidate against the complete output inventory.
func (p *Publisher) PreflightLocalDoc(doc config.LocalDoc) error {
	return preflightLocalDoc(p.inputs, doc)
}

// RenderResidentMarker returns one resident marker from a single operation plan.
func (p *Publisher) RenderResidentMarker(name string) (outputplan.Output, error) {
	file, err := renderResidentMarkerOperation(p.inputs, name)
	if err != nil {
		return outputplan.Output{}, err
	}
	return freezeOutput(file), nil
}
