package projectstate

import (
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/resident"
)

// ArtifactRole classifies a loaded target declaration input.
type ArtifactRole string

const (
	// ArtifactConfig identifies an authored configuration input.
	ArtifactConfig ArtifactRole = "config"
	// ArtifactLock identifies the managed project lock.
	ArtifactLock ArtifactRole = "lock"
	// ArtifactManifest identifies manifest authority.
	ArtifactManifest ArtifactRole = "manifest"
	// ArtifactTemplate identifies an embedded template input.
	ArtifactTemplate ArtifactRole = "template"
	// ArtifactConventionPart identifies an authored convention part.
	ArtifactConventionPart ArtifactRole = "convention-part"
	// ArtifactAuthoredData identifies authored sidecar data.
	ArtifactAuthoredData ArtifactRole = "authored-data"
	// ArtifactTopicMetadata identifies authored topic metadata.
	ArtifactTopicMetadata ArtifactRole = "topic-metadata"
	// ArtifactClaimPart identifies an authored current-state claim part.
	ArtifactClaimPart ArtifactRole = "claim-part"
	// ArtifactDecisionRecord identifies an architecture decision record.
	ArtifactDecisionRecord ArtifactRole = "decision-record"
	// ArtifactManagedOutput identifies an existing managed output input.
	ArtifactManagedOutput ArtifactRole = "managed-output"
	// ArtifactProtocolDescriptor identifies a runtime protocol descriptor.
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

// New constructs immutable loaded project facts from validated Loader inputs.
func New(root string, roots resident.Roots, nested bool, cfg *config.Config, selected, complete *catalog.Catalog, targets []Target) (*ProjectState, error) {
	facts, err := config.NewFacts(cfg)
	if err != nil {
		return nil, err
	}
	return &ProjectState{root, roots, nested, facts, catalog.NewProfileView(selected, catalog.ProfileFull), catalog.NewProfileView(complete, catalog.ProfileFull), cloneTargets(targets)}, nil
}

// NewDerived constructs immutable facts for an already-derived operation universe.
func NewDerived(root string, roots resident.Roots, nested bool, selected, complete *catalog.Catalog, targets []Target) *ProjectState {
	return &ProjectState{invokingRoot: root, roots: roots, nested: nested, selectedCat: catalog.NewProfileView(selected, catalog.ProfileFull), completeCat: catalog.NewProfileView(complete, catalog.ProfileFull), targets: cloneTargets(targets)}
}

// Root returns the invoking checkout root.
func (s *ProjectState) Root() string { return s.invokingRoot }

// Roots returns the resolved resident-root facts.
func (s *ProjectState) Roots() resident.Roots { return s.roots }

// Nested reports whether the invoking project is nested below its control root.
func (s *ProjectState) Nested() bool { return s.nested }

// Config returns a defensive copy of the loaded configuration facts.
func (s *ProjectState) Config() *config.Config { return s.facts.Config() }

// Facts returns the immutable configuration snapshot.
func (s *ProjectState) Facts() config.Facts { return s.facts }

// Catalog returns a defensive copy of the selected catalog.
func (s *ProjectState) Catalog() *catalog.Catalog { return s.selectedCat.Catalog() }

// CompleteCatalog returns a defensive copy of the complete catalog.
func (s *ProjectState) CompleteCatalog() *catalog.Catalog { return s.completeCat.Catalog() }

// Targets returns defensive copies of the resolved target declarations.
func (s *ProjectState) Targets() []Target { return cloneTargets(s.targets) }

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
