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
	"github.com/hypnotox/agentic-workflows/internal/repositorycheck"
)

func operationInputs(state *Session, cfg *config.Config) renderInputs {
	return newRenderInputs(state, cfg, state.Reader())
}

// AdvisoryNotes reports non-blocking project checks from one prepared universe.
func AdvisoryNotes(state *Session, cfg *config.Config, output outputplan.Plan, glossary glossarycheck.Input) ([]string, error) {
	return advisoryNotes(operationInputs(state, cfg), &output, glossary)
}

// BuildCheckReport checks the selected project tree using already-derived,
// narrow semantic values. Project deliberately has no publisher-operation
// bundle: orchestration remains at the application boundary.
func BuildCheckReport(state *Session, cfg *config.Config, repo *awfgit.Repo, ctx context.Context, output outputplan.Plan, pitfalls pitfall.Corpus, generated generatedcheck.AdditionalInput, glossary glossarycheck.Input) (repositorycheck.Report, error) {
	return checkReport(operationInputs(state, cfg), repo, ctx, pitfalls, generated, glossary, &output)
}

// BuildListDocument renders the requested project inventory.
func BuildListDocument(state *Session, cfg *config.Config, kindFilter string) (presentation.Document, error) {
	return listDocument(cfg, state.catalog(), kindFilter)
}
