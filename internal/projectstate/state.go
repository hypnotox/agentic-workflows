package projectstate

import (
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/resident"
)

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
	return &ProjectState{root, roots, nested, facts, catalog.NewView(selected), catalog.NewView(complete), cloneTargets(targets)}, nil
}

// NewDerivedWithFacts constructs an already-derived universe while preserving
// its immutable loaded configuration facts.
func NewDerivedWithFacts(root string, roots resident.Roots, nested bool, facts config.Facts, selected, complete *catalog.Catalog, targets []Target) *ProjectState {
	return &ProjectState{invokingRoot: root, roots: roots, nested: nested, facts: facts, selectedCat: catalog.NewView(selected), completeCat: catalog.NewView(complete), targets: cloneTargets(targets)}
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
