package publisher

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/plan"
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
func fullProfile(p renderInputs) bool                { return p.cfg == nil || p.cfg.Profile != catalog.ProfileCore }

func deriveOperationStateWithPitfalls(p renderInputs) (adr.Corpus, pitfall.Corpus, topic.Corpus, map[string]bool, error) {
	corpus := adr.Corpus{}
	topics := topic.Corpus{}
	var err error
	if fullProfile(p) {
		corpus, err = adr.LoadCorpusFromTree(p.read, path.Join(config.DocsDir, "decisions"))
		if err != nil {
			return adr.Corpus{}, pitfall.Corpus{}, topic.Corpus{}, nil, err
		}
		topics, err = topic.LoadCorpusFromReader(p.read, p.cfg, corpus)
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

func derivePlans(p renderInputs) ([]plan.Plan, error) {
	if !fullProfile(p) {
		return nil, nil
	}
	prefix := path.Join(config.DocsDir, "plans") + "/"
	paths, err := p.read.Paths(prefix)
	if err != nil {
		return nil, err
	}
	slices.Sort(paths)
	sources := make([]plan.Source, 0, len(paths))
	for _, sourcePath := range paths {
		if path.Dir(sourcePath) != strings.TrimSuffix(prefix, "/") {
			continue
		}
		data, found, err := p.read.ReadFile(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path.Base(sourcePath), err)
		}
		if found {
			sources = append(sources, plan.Source{Filename: path.Base(sourcePath), Path: sourcePath, Bytes: data})
		}
	}
	return plan.ParseSources(sources)
}

// Publisher is the sole output-plan construction and rendering coordinator.
type Publisher struct{ inputs renderInputs }

// Preparation is one Publisher-owned derivation and its direct semantic
// projections for residual consumers.
type Preparation struct {
	plan       outputplan.Plan
	adrs       adr.Corpus
	pitfalls   pitfall.Corpus
	topics     topic.Corpus
	skills     map[string]bool
	plans      []plan.Plan
	plansError error
	generated  generatedcheck.AdditionalInput
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
	plans, plansErr := derivePlans(p.inputs)
	built, err := outputPlanWithPitfalls(p.inputs, adrs, pitfalls, topics, skills)
	if err != nil {
		return Preparation{}, err
	}
	generated, err := generatedSemantics(p.inputs, topics)
	if err != nil {
		return Preparation{}, err
	}
	return Preparation{plan: freezePlan(built), adrs: adrs, pitfalls: pitfalls, topics: topics, skills: maps.Clone(skills), plans: clonePlans(plans), plansError: plansErr, generated: generated}, nil
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

// Plans returns a defensive projection of the operation's parsed plans.
func (p Preparation) Plans() []plan.Plan { return clonePlans(p.plans) }

// PlansError returns diagnostics or another error from parsing the selected plans.
func (p Preparation) PlansError() error { return p.plansError }

// GeneratedOutput returns a defensive prepared projection for generated-output checks.
func (p Preparation) GeneratedOutput() generatedcheck.AdditionalInput {
	return cloneGeneratedOutput(p.generated)
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
		entries[i].Tags = slices.Clone(entries[i].Tags)
		entries[i].Related = slices.Clone(entries[i].Related)
		entries[i].Source = slices.Clone(entries[i].Source)
	}
	return pitfall.New(entries)
}

func clonePlans(plans []plan.Plan) []plan.Plan {
	out := slices.Clone(plans)
	for i := range out {
		out[i].ADRs = slices.Clone(out[i].ADRs)
		out[i].Source = slices.Clone(out[i].Source)
		out[i].Phases = slices.Clone(out[i].Phases)
		for phaseIndex := range out[i].Phases {
			phase := &out[i].Phases[phaseIndex]
			phase.Tasks = slices.Clone(phase.Tasks)
			phase.Advances = slices.Clone(phase.Advances)
			phase.Completes = slices.Clone(phase.Completes)
			for taskIndex := range phase.Tasks {
				fields := &phase.Tasks[taskIndex].Fields
				fields.Paths = slices.Clone(fields.Paths)
				fields.Applying = slices.Clone(fields.Applying)
				fields.Context = slices.Clone(fields.Context)
			}
		}
		out[i].DoD = slices.Clone(out[i].DoD)
		out[i].CommitSubjects = slices.Clone(out[i].CommitSubjects)
	}
	return out
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
