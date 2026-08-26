package project

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/topic"
	"github.com/hypnotox/agentic-workflows/internal/vocabularycheck"
)

func operationInputs(state *ProjectState, cfg *config.Config) renderInputs {
	return newRenderInputs(state, cfg, filesystemProjectReader{root: state.Root()})
}

// OperationSemantics carries Publisher's direct semantic derivation to residual
// project consumers without coupling project to application coordination.
type OperationSemantics struct {
	ADRs            adr.Corpus
	Pitfalls        pitfall.Corpus
	Topics          topic.Corpus
	EffectiveSkills map[string]bool
	Plans           []plan.Plan
	PlansError      error
	GeneratedOutput generatedcheck.AdditionalInput
	Vocabulary      vocabularycheck.Input
}

// AdvisoryNotes reports non-blocking project checks from one prepared universe.
func AdvisoryNotes(state *ProjectState, cfg *config.Config, output outputplan.Plan, semantics OperationSemantics) ([]string, error) {
	return advisoryNotes(operationInputs(state, cfg), semantics.Plans, semantics.PlansError, &output, semantics.Vocabulary)
}

// BuildCheckReport checks the selected project tree using one prepared universe.
func BuildCheckReport(state *ProjectState, cfg *config.Config, repo *awfgit.Repo, ctx context.Context, output outputplan.Plan, semantics OperationSemantics) (CheckReport, error) {
	return checkReport(operationInputs(state, cfg), repo, ctx, semantics, &output)
}

// BuildListDocument renders the requested project inventory.
func BuildListDocument(state *ProjectState, cfg *config.Config, kindFilter string) (presentation.Document, error) {
	return listDocument(cfg, state.catalog(), kindFilter)
}

// NewADRLeased scaffolds one branch-aware ADR through the caller-held
// selected-root capability. The caller retains its tracked lease through
// presentation.
func NewADRLeased(root string, cfg *config.Config, repo *awfgit.Repo, ctx context.Context, title string, lease *filesystem.Lease, files *filesystem.Handle) (string, error) {
	return newADRLeased(root, cfg, repo, ctx, title, lease, files)
}

// NewPlanLeased scaffolds one plan through the caller-held selected-root
// capability. The caller retains its tracked lease through presentation.
func NewPlanLeased(root, title string, lease *filesystem.Lease, files *filesystem.Handle) (string, error) {
	return newPlanLeased(root, title, lease, files)
}

// NewPitfall scaffolds one authored pitfall beneath root.
func NewPitfall(root, title string) (presentation.Document, error) { return newPitfall(root, title) }
