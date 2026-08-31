package publisher

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
	"github.com/hypnotox/agentic-workflows/internal/glossary"
	"github.com/hypnotox/agentic-workflows/internal/glossarycheck"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// ProjectSession is the read-only project selection consumed by Publisher.
// The concrete authority owner is project.Session.
type ProjectSession interface {
	Root() string
	Roots() resident.Roots
	Config() *config.Config
	Reader() ProjectTreeReader
	Catalog() *catalog.Catalog
	Targets() []Target
}

type renderInputs struct {
	session  ProjectSession
	cfg      *config.Config
	read     ProjectTreeReader
	version  string
	selected *catalog.Catalog
}

func renderInputsFromSession(session ProjectSession, version string) renderInputs {
	return renderInputs{session: session, cfg: session.Config(), read: session.Reader(), version: version, selected: session.Catalog()}
}
func (p renderInputs) root() string              { return p.session.Root() }
func (p renderInputs) targets() []Target         { return p.session.Targets() }
func (p renderInputs) catalog() *catalog.Catalog { return p.selected }

func projectCatalog(p renderInputs) *catalog.Catalog { return p.catalog() }

func deriveAuthoritySemantics(p renderInputs) (topic.Corpus, error) {
	topics, err := topic.LoadCorpusFromReader(p.read, p.cfg)
	if err != nil {
		return topic.Corpus{}, err
	}
	return topics, nil
}

func deriveOperationStateWithPitfalls(p renderInputs) (pitfall.Corpus, topic.Corpus, map[string]bool, error) {
	topics, err := deriveAuthoritySemantics(p)
	if err != nil {
		return pitfall.Corpus{}, topic.Corpus{}, nil, err
	}
	pitfalls, err := loadPitfallCorpus(p)
	if err != nil {
		return pitfall.Corpus{}, topic.Corpus{}, nil, err
	}
	eff, err := effectiveSkills(p)
	if err != nil {
		return pitfall.Corpus{}, topic.Corpus{}, nil, err
	}
	return pitfalls, topics, eff, nil
}

// Publisher is the sole output-plan construction and rendering coordinator.
// One Publisher is one immutable operation: all readers share its one derived
// universe, and publication may be attempted only once.
type Publisher struct {
	inputs        renderInputs
	once          sync.Once
	op            operation
	opErr         error
	mu            sync.Mutex
	used          bool
	opStarted     bool
	publishing    bool
	planningReady bool
}

type operation struct {
	plan      outputplan.Plan
	files     []RenderedFile
	pitfalls  pitfall.Corpus
	topics    topic.Corpus
	skills    map[string]bool
	generated generatedcheck.AdditionalInput
	glossary  glossarycheck.Input
}

// New composes a Publisher from one authoritative project Session.
func New(session ProjectSession, version string) *Publisher {
	if session == nil || session.Config() == nil || session.Reader() == nil {
		panic("publisher: missing composition dependency")
	}
	return &Publisher{inputs: renderInputsFromSession(session, version)}
}

func (p *Publisher) operationState() (*operation, error) {
	p.mu.Lock()
	if p.publishing && !p.planningReady {
		p.mu.Unlock()
		return nil, errors.New("publisher: publication planning is not ready")
	}
	p.opStarted = true
	p.mu.Unlock()
	p.once.Do(func() {
		pitfalls, topics, skills, err := deriveOperationStateWithPitfalls(p.inputs)
		if err != nil {
			p.opErr = err
			return
		}
		built, err := outputPlanWithPitfalls(p.inputs, pitfalls, topics, skills)
		if err != nil {
			p.opErr = err
			return
		}
		glossary, err := glossarySemantics(p.inputs)
		if err != nil {
			p.opErr = err
			return
		}
		generated, err := generatedSemantics(p.inputs, topics)
		if err != nil {
			p.opErr = err
			return
		}
		p.op = operation{
			plan: freezePlan(built), files: built.writeFiles(),
			pitfalls: clonePitfallCorpus(pitfalls), topics: topics.Clone(), skills: maps.Clone(skills),
			generated: cloneGeneratedOutput(generated), glossary: cloneGlossaryInput(glossary),
		}
	})
	if p.opErr != nil {
		return nil, p.opErr
	}
	return &p.op, nil
}

// beginMutation reserves this operation's sole publication attempt.  It is
// deliberately before lease acquisition: a failed attempt is still an attempt
// against this operation's frozen authority.
func (p *Publisher) beginMutation() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.used {
		return errors.New("publisher: operation already used for publication")
	}
	if p.opStarted {
		return errors.New("publisher: operation already materialized outside publication")
	}
	p.used = true
	p.publishing = true
	return nil
}

func (p *Publisher) allowPublicationPlanning() {
	p.mu.Lock()
	p.planningReady = true
	p.mu.Unlock()
}

// Plan returns the one immutable desired-output plan for this operation.
func (p *Publisher) Plan() (outputplan.Plan, error) {
	op, err := p.operationState()
	if err != nil {
		return outputplan.Plan{}, err
	}
	return op.plan, nil
}

// Pitfalls returns the operation's frozen pitfall corpus. These narrow accessors
// share Plan's cached derivation rather than starting a second planning pass.
func (p *Publisher) Pitfalls() (pitfall.Corpus, error) {
	op, err := p.operationState()
	if err != nil {
		return pitfall.Corpus{}, err
	}
	return clonePitfallCorpus(op.pitfalls), nil
}

// EffectiveSkills returns the operation's frozen effective skill projection.
func (p *Publisher) EffectiveSkills() (map[string]bool, error) {
	op, err := p.operationState()
	if err != nil {
		return nil, err
	}
	return maps.Clone(op.skills), nil
}

// GeneratedOutput returns the operation's frozen generated-output input.
func (p *Publisher) GeneratedOutput() (generatedcheck.AdditionalInput, error) {
	op, err := p.operationState()
	if err != nil {
		return generatedcheck.AdditionalInput{}, err
	}
	return cloneGeneratedOutput(op.generated), nil
}

// Glossary returns the operation's frozen glossary input.
func (p *Publisher) Glossary() (glossarycheck.Input, error) {
	op, err := p.operationState()
	if err != nil {
		return glossarycheck.Input{}, err
	}
	return cloneGlossaryInput(op.glossary), nil
}

// ResidentMarker selects a resident marker from this operation's one plan.
func (p *Publisher) ResidentMarker(name string) (outputplan.Output, error) {
	plan, err := p.Plan()
	if err != nil {
		return outputplan.Output{}, err
	}
	want := ".awf/" + name + "/.gitignore"
	for _, output := range plan.Outputs() {
		if output.Path() == want {
			return output, nil
		}
	}
	return outputplan.Output{}, fmt.Errorf("resident marker %q is not planned", name)
}

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

// BuildConfigReference derives the live configuration reference from this operation's files.
func (p *Publisher) BuildConfigReference() (ConfigReference, error) {
	op, err := p.operationState()
	if err != nil {
		return ConfigReference{}, err
	}
	return configReferenceRows(p.inputs, slices.Clone(op.files))
}

// PreflightLocalDoc validates one candidate against the complete output inventory.
func (p *Publisher) PreflightLocalDoc(doc config.LocalDoc) error {
	return preflightLocalDoc(p.inputs, doc)
}
