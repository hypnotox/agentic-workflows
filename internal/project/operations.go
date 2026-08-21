package project

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/commitmsg"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func operationInputs(state *ProjectState, cfg *config.Config) renderInputs {
	return newRenderInputs(state, cfg, filesystemProjectReader{root: state.Root()})
}

// NumberPendingADRs assigns numbers using the selected project tree.
func NumberPendingADRs(state *ProjectState, cfg *config.Config, slugs []string, preparePlan func() (outputplan.Plan, error)) (NumberingReport, error) {
	return numberPendingADRs(operationInputs(state, cfg), slugs, func() (*OutputPlan, error) {
		plan, err := preparePlan()
		return &plan, err
	})
}

// AdvisoryNotes reports non-blocking project checks from the selected tree.
func AdvisoryNotes(state *ProjectState, cfg *config.Config, plan outputplan.Plan) ([]string, error) {
	return advisoryNotes(operationInputs(state, cfg), &plan)
}

// BuildCheckReport checks the selected project tree and its repository state using one Publisher plan.
func BuildCheckReport(state *ProjectState, cfg *config.Config, repo *awfgit.Repo, ctx context.Context, plan outputplan.Plan) (CheckReport, error) {
	return checkReport(operationInputs(state, cfg), repo, ctx, &plan)
}

// CheckCurrentState checks working-tree authority through the supplied repository.
func CheckCurrentState(root string, repo *awfgit.Repo, ctx context.Context) (CurrentStateReport, error) {
	return checkCurrentState(root, repo, ctx)
}

// CheckCommitAuthorization validates one commit message against staged repository state.
func CheckCommitAuthorization(root string, repo *awfgit.Repo, ctx context.Context, msg commitmsg.Message) (CommitAuthorizationResult, error) {
	return checkCommitAuthorization(root, repo, ctx, msg)
}

// InitCollisions reports outputs that would collide during initialization.
func InitCollisions(state *ProjectState, cfg *config.Config, plan outputplan.Plan) ([]string, error) {
	return initCollisions(operationInputs(state, cfg), &plan)
}

// BuildListDocument renders the requested project inventory.
func BuildListDocument(state *ProjectState, cfg *config.Config, kindFilter string) (presentation.Document, error) {
	return listDocument(cfg, state.catalog(), kindFilter)
}

// ReadPlan returns one executable plan projection from root.
func ReadPlan(root, name, selector string) ([]byte, error) { return readPlan(root, name, selector) }

// SyncReport publishes the selected project tree and reports its mutations.
func SyncReport(state *ProjectState, cfg *config.Config, plan outputplan.Plan) ([]Backup, []Change, []string, error) {
	return syncReportOperation(operationInputs(state, cfg), &plan)
}

// InitializeReport publishes an initial project tree under explicit authority.
func InitializeReport(state *ProjectState, cfg *config.Config, seed InitAuthority, plan outputplan.Plan) ([]Backup, []Change, []string, error) {
	return initializeReport(operationInputs(state, cfg), seed, &plan)
}

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
