package publisher

import (
	"fmt"
	"maps"
	"path"
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
	"github.com/hypnotox/agentic-workflows/internal/glossary"
	"github.com/hypnotox/agentic-workflows/internal/glossarycheck"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/projectstate"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

type renderInputs struct {
	state    *projectstate.ProjectState
	cfg      *config.Config
	read     ProjectTreeReader
	version  string
	selected *catalog.Catalog
}

func newRenderInputs(state *projectstate.ProjectState, cfg *config.Config, read ProjectTreeReader, version string) renderInputs {
	return renderInputs{state: state, cfg: cfg, read: read, version: version, selected: state.Catalog()}
}
func (p renderInputs) root() string              { return p.state.Root() }
func (p renderInputs) targets() []Target         { return p.state.Targets() }
func (p renderInputs) catalog() *catalog.Catalog { return p.selected }

func projectCatalog(p renderInputs) *catalog.Catalog { return p.catalog() }

func deriveAuthoritySemantics(p renderInputs) (adr.Corpus, topic.Corpus, error) {
	corpus, err := adr.LoadCorpusFromTree(p.read, path.Join(config.DocsDir, "decisions"))
	if err != nil {
		return adr.Corpus{}, topic.Corpus{}, err
	}
	topics, err := topic.LoadCorpusFromReader(p.read, p.cfg)
	if err != nil {
		return adr.Corpus{}, topic.Corpus{}, err
	}
	return corpus, topics, nil
}

func deriveOperationStateWithPitfalls(p renderInputs) (adr.Corpus, pitfall.Corpus, topic.Corpus, map[string]bool, error) {
	corpus, topics, err := deriveAuthoritySemantics(p)
	if err != nil {
		return adr.Corpus{}, pitfall.Corpus{}, topic.Corpus{}, nil, err
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

// Preparation is one Publisher-owned derivation and its direct semantic
// projections for residual consumers.
type Preparation struct {
	publisher *Publisher
	plan      outputplan.Plan
	adrs      adr.Corpus
	pitfalls  pitfall.Corpus
	topics    topic.Corpus
	skills    map[string]bool
	generated generatedcheck.AdditionalInput
	glossary  glossarycheck.Input
}

// New composes a Publisher from immutable loaded facts and an explicit operation tree reader.
func New(state *projectstate.ProjectState, cfg *config.Config, read ProjectTreeReader, version string) *Publisher {
	if state == nil || cfg == nil || read == nil {
		panic("publisher: missing composition dependency")
	}
	// Retain only the selected tree binding from cfg. Every semantic value is a
	// fresh projection of the state's immutable loaded facts.
	privateConfig := cfg.OperationTree().Bind(state.Facts())
	return &Publisher{inputs: newRenderInputs(state, privateConfig, read, version)}
}

// Prepare derives one operation universe and constructs exactly one immutable plan.
func (p *Publisher) Prepare() (Preparation, error) {
	adrs, pitfalls, topics, skills, err := deriveOperationStateWithPitfalls(p.inputs)
	if err != nil {
		return Preparation{}, err
	}
	built, err := outputPlanWithPitfalls(p.inputs, adrs, pitfalls, topics, skills)
	if err != nil {
		return Preparation{}, err
	}
	glossary, err := glossarySemantics(p.inputs)
	if err != nil {
		return Preparation{}, err
	}
	generated, err := generatedSemantics(p.inputs, topics)
	if err != nil {
		return Preparation{}, err
	}
	return Preparation{publisher: p, plan: freezePlan(built), adrs: adrs, pitfalls: pitfalls, topics: topics, skills: maps.Clone(skills), generated: generated, glossary: glossary}, nil
}

// Plan derives exactly one immutable plan for this operation.
func (p *Publisher) Plan() (outputplan.Plan, error) {
	prepared, err := p.Prepare()
	return prepared.Plan(), err
}

// Plan returns the one immutable plan constructed for this operation.
func (p Preparation) Plan() outputplan.Plan { return p.plan }

// ADRs returns a defensive ADR corpus derived from the selected operation tree.
func (p Preparation) ADRs() adr.Corpus { return p.adrs.Clone() }

// Pitfalls returns a defensive pitfall corpus derived from the selected operation tree.
func (p Preparation) Pitfalls() pitfall.Corpus { return clonePitfallCorpus(p.pitfalls) }

// Topics returns a defensive topic corpus derived from the selected operation tree.
func (p Preparation) Topics() topic.Corpus { return p.topics.Clone() }

// EffectiveSkills returns a defensive projection of the operation's effective skills.
func (p Preparation) EffectiveSkills() map[string]bool { return maps.Clone(p.skills) }

// GeneratedOutput returns a defensive prepared projection for generated-output checks.
func (p Preparation) GeneratedOutput() generatedcheck.AdditionalInput {
	return cloneGeneratedOutput(p.generated)
}

// Glossary returns a defensive prepared glossary projection.
func (p Preparation) Glossary() glossarycheck.Input { return cloneGlossaryInput(p.glossary) }

func glossarySemantics(p renderInputs) (glossarycheck.Input, error) {
	sc, err := p.cfg.Sidecar("docs", "glossary")
	if err != nil {
		return glossarycheck.Input{}, err
	}
	authored, err := glossary.Records(sc.Data["terms"])
	if err != nil {
		return glossarycheck.Input{}, err
	}
	merged, err := glossary.Merge(withDefaultData(sc, projectCatalog(p).Docs["glossary"].Data, glossary.SpecializedListDataKeys("docs", "glossary")...))
	if err != nil {
		return glossarycheck.Input{}, err
	}
	return glossarycheck.Input{Enabled: true, Authored: authored, Merged: merged, Domains: slices.Clone(p.cfg.Domains)}, nil
}
func cloneGlossaryInput(in glossarycheck.Input) glossarycheck.Input {
	out := in
	out.Authored = slices.Clone(in.Authored)
	out.Merged = slices.Clone(in.Merged)
	out.Domains = slices.Clone(in.Domains)
	for i := range out.Authored {
		out.Authored[i].Domains = slices.Clone(out.Authored[i].Domains)
	}
	for i := range out.Merged {
		out.Merged[i].Domains = slices.Clone(out.Merged[i].Domains)
	}
	return out
}

// ResidentMarker selects the marker from this preparation's existing plan.
func (p Preparation) ResidentMarker(name string) (outputplan.Output, error) {
	want := ".awf/" + name + "/.gitignore"
	for _, output := range p.plan.Outputs() {
		if output.Path() == want {
			return output, nil
		}
	}
	return outputplan.Output{}, fmt.Errorf("resident marker %q is not planned", name)
}

func clonePitfallCorpus(corpus pitfall.Corpus) pitfall.Corpus {
	entries := corpus.All()
	for i := range entries {
		entries[i].Domains = slices.Clone(entries[i].Domains)
		entries[i].Source = slices.Clone(entries[i].Source)
	}
	return pitfall.New(entries)
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
		ObservedTemplateID: file.ObservedTemplateID,
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
			DependsOn: node.DependsOn, ObservedTemplateID: node.ObservedTemplateID, Output: output,
		}))
	}
	return outputplan.New(nodes)
}

// BuildConfigReference derives the live configuration reference from this operation's plan.
func (p *Publisher) BuildConfigReference() (ConfigReference, error) {
	return configReferenceModel(p.inputs)
}

// PreflightLocalDoc validates one candidate against the complete output inventory.
func (p *Publisher) PreflightLocalDoc(doc config.LocalDoc) error {
	return preflightLocalDoc(p.inputs, doc)
}
