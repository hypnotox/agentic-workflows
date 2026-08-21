package projectstate

import (
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/resident"
)

// ArtifactRole classifies a loaded target declaration input.
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

// OutputPolicy is the resolved declaration data attached to an output.
type OutputPolicy struct{ ValidateFrontmatter, ScanReferences, ScanSkillReferences, Regenerate bool }

// ProjectState is the immutable loaded project fact snapshot.
type ProjectState struct {
	invokingRoot string
	roots        resident.Roots
	nested       bool
	facts        config.Facts
	selectedCat  catalog.View
	completeCat  catalog.View
	targets      []Target
}

func New(root string, roots resident.Roots, nested bool, cfg *config.Config, selected, complete *catalog.Catalog, targets []Target) (*ProjectState, error) {
	facts, err := config.NewFacts(cfg)
	if err != nil {
		return nil, err
	}
	return &ProjectState{root, roots, nested, facts, catalog.NewProfileView(selected, catalog.ProfileFull), catalog.NewProfileView(complete, catalog.ProfileFull), cloneTargets(targets)}, nil
}
func NewDerived(root string, roots resident.Roots, nested bool, selected, complete *catalog.Catalog, targets []Target) *ProjectState {
	return &ProjectState{invokingRoot: root, roots: roots, nested: nested, selectedCat: catalog.NewProfileView(selected, catalog.ProfileFull), completeCat: catalog.NewProfileView(complete, catalog.ProfileFull), targets: cloneTargets(targets)}
}
func (s *ProjectState) Root() string                      { return s.invokingRoot }
func (s *ProjectState) Roots() resident.Roots             { return s.roots }
func (s *ProjectState) Nested() bool                      { return s.nested }
func (s *ProjectState) Config() *config.Config            { return s.facts.Config() }
func (s *ProjectState) Facts() config.Facts               { return s.facts }
func (s *ProjectState) Catalog() *catalog.Catalog         { return s.selectedCat.Catalog() }
func (s *ProjectState) CompleteCatalog() *catalog.Catalog { return s.completeCat.Catalog() }
func (s *ProjectState) Targets() []Target                 { return cloneTargets(s.targets) }

func cloneTargets(source []Target) []Target {
	out := make([]Target, len(source))
	copy(out, source)
	for i := range out {
		out[i].Capabilities = append([]Capability(nil), out[i].Capabilities...)
		out[i].Outputs = append([]TargetOutput(nil), out[i].Outputs...)
		for j := range out[i].Outputs {
			out[i].Outputs[j].Inputs = append([]TargetOutputInput(nil), out[i].Outputs[j].Inputs...)
		}
	}
	return out
}
