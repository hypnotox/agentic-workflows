package project

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/glossarycheck"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func operationInputs(state *Session, cfg *config.Config) renderInputs {
	return newRenderInputs(state, cfg, state.Reader())
}

// OperationSemantics carries Publisher's direct semantic derivation to residual
// project consumers without coupling project to application coordination.
type OperationSemantics struct {
	Pitfalls        pitfall.Corpus
	Topics          topic.Corpus
	EffectiveSkills map[string]bool
	GeneratedOutput generatedcheck.AdditionalInput
	Glossary        glossarycheck.Input
}

// AdvisoryNotes reports non-blocking project checks from one prepared universe.
func AdvisoryNotes(state *Session, cfg *config.Config, output outputplan.Plan, semantics OperationSemantics) ([]string, error) {
	return advisoryNotes(operationInputs(state, cfg), &output, semantics.Glossary)
}

// BuildCheckReport checks the selected project tree using one prepared universe.
func BuildCheckReport(state *Session, cfg *config.Config, repo *awfgit.Repo, ctx context.Context, output outputplan.Plan, semantics OperationSemantics) (CheckReport, error) {
	return checkReport(operationInputs(state, cfg), repo, ctx, semantics, &output)
}

// BuildListDocument renders the requested project inventory.
func BuildListDocument(state *Session, cfg *config.Config, kindFilter string) (presentation.Document, error) {
	return listDocument(cfg, state.catalog(), kindFilter)
}

// NewPitfall scaffolds one authored pitfall beneath root.
func NewPitfall(root, title string) (presentation.Document, error) { return newPitfall(root, title) }
