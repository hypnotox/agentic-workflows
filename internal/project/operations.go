package project

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/commitmsg"
	"github.com/hypnotox/agentic-workflows/internal/config"
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

// NumberPendingADRs assigns numbers using the selected project tree.
func NumberPendingADRs(state *ProjectState, cfg *config.Config, slugs []string, publish func() error) (NumberingReport, error) {
	return numberPendingADRs(operationInputs(state, cfg), slugs, publish)
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

// CheckCurrentState checks working-tree authority through the supplied repository.
func CheckCurrentState(root string, repo *awfgit.Repo, ctx context.Context) (CurrentStateReport, error) {
	return checkCurrentState(root, repo, ctx)
}

// CheckCommitAuthorization validates one commit message against staged repository state.
func CheckCommitAuthorization(root string, repo *awfgit.Repo, ctx context.Context, msg commitmsg.Message) (CommitAuthorizationResult, error) {
	return checkCommitAuthorization(root, repo, ctx, msg)
}

// BuildListDocument renders the requested project inventory.
func BuildListDocument(state *ProjectState, cfg *config.Config, kindFilter string) (presentation.Document, error) {
	return listDocument(cfg, state.catalog(), kindFilter)
}

// ReadPlan returns one executable plan projection from root.
func ReadPlan(root, name, selector string) ([]byte, error) { return readPlan(root, name, selector) }

// Audit evaluates the selected repository history against project configuration.
func Audit(root string, cfg *config.Config, ctx context.Context, base, head string) ([]audit.Finding, int, error) {
	return auditOperation(root, cfg, ctx, base, head)
}

// NewADR scaffolds one branch-aware ADR through the supplied repository.
func NewADR(root string, cfg *config.Config, repo *awfgit.Repo, ctx context.Context, title string) (string, error) {
	return newADR(root, cfg, repo, ctx, title)
}

// NewPlan scaffolds one plan beneath root.
func NewPlan(root, title string) (string, error) { return newPlan(root, title) }

// NewPitfall scaffolds one authored pitfall beneath root.
func NewPitfall(root, title string) (presentation.Document, error) { return newPitfall(root, title) }

// QueryTopic runs one topic query through the supplied repository.
func QueryTopic(root string, repo *awfgit.Repo, ctx context.Context, selector string, opts topic.QueryOptions) (topic.QueryResult, error) {
	return queryTopic(root, repo, ctx, selector, opts)
}
