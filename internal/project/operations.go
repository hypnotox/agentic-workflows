package project

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/commitmsg"
	"github.com/hypnotox/agentic-workflows/internal/commitpolicy"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func operationInputs(state *ProjectState, cfg *config.Config) renderInputs {
	return newRenderInputs(state, cfg, nil)
}

func NumberPendingADRs(state *ProjectState, cfg *config.Config, ctx context.Context, slugs []string) (NumberingReport, error) {
	return numberPendingADRs(operationInputs(state, cfg), ctx, slugs)
}
func AdvisoryNotes(state *ProjectState, cfg *config.Config, ctx context.Context) ([]string, error) {
	return advisoryNotes(operationInputs(state, cfg), ctx)
}
func BuildCheckReport(state *ProjectState, cfg *config.Config, repo *awfgit.Repo, ctx context.Context) (CheckReport, error) {
	return checkReport(operationInputs(state, cfg), repo, ctx)
}
func VerifyCommitPolicy(cfg *config.Config, root string, repo *awfgit.Repo, ctx context.Context, targets []string) commitpolicy.Outcome {
	return verifyCommitPolicyOperation(cfg, root, repo, ctx, targets)
}
func BuildCommitPolicyPresentation(cfg *config.Config, outcome commitpolicy.Outcome) (presentation.Document, error) {
	return commitPolicyPresentation(cfg, outcome)
}
func BuildConfigReference(state *ProjectState, cfg *config.Config, ctx context.Context) (ConfigReference, error) {
	return configReferenceModel(operationInputs(state, cfg), ctx)
}
func BuildContextState(state *ProjectState, repo *awfgit.Repo, ctx context.Context) (ContextState, error) {
	return contextState(state, repo, ctx)
}
func CheckCurrentState(root string, repo *awfgit.Repo, ctx context.Context) (CurrentStateReport, error) {
	return checkCurrentState(root, repo, ctx)
}
func CheckCommitAuthorization(root string, repo *awfgit.Repo, ctx context.Context, msg commitmsg.Message) (CommitAuthorizationResult, error) {
	return checkCommitAuthorization(root, repo, ctx, msg)
}
func InitCollisions(state *ProjectState, cfg *config.Config, ctx context.Context) ([]string, error) {
	return initCollisions(operationInputs(state, cfg), ctx)
}
func BuildListDocument(state *ProjectState, cfg *config.Config, kindFilter string) (presentation.Document, error) {
	return listDocument(cfg, state.catalog(), kindFilter)
}
func BuildOutputPlan(state *ProjectState, cfg *config.Config, ctx context.Context) (*OutputPlan, error) {
	return outputPlan(operationInputs(state, cfg), ctx)
}
func PreflightLocalDoc(state *ProjectState, cfg *config.Config, ctx context.Context, doc config.LocalDoc) error {
	return preflightLocalDoc(operationInputs(state, cfg), ctx, doc)
}
func PlannedOutputs(state *ProjectState, cfg *config.Config, ctx context.Context) ([]string, error) {
	op, err := BuildOutputPlan(state, cfg, ctx)
	if err != nil {
		return nil, err
	}
	return plannedOutputPaths(op), nil
}
func ReadPlan(root, name, selector string) ([]byte, error) { return readPlan(root, name, selector) }
func SyncReport(state *ProjectState, cfg *config.Config, ctx context.Context) ([]Backup, []Change, []string, error) {
	return syncReportOperation(operationInputs(state, cfg), ctx)
}
func InitializeReport(state *ProjectState, cfg *config.Config, ctx context.Context, seed InitAuthority) ([]Backup, []Change, []string, error) {
	return initializeReport(operationInputs(state, cfg), ctx, seed)
}
func Audit(root string, cfg *config.Config, ctx context.Context, base, head string) ([]audit.Finding, int, error) {
	return auditOperation(root, cfg, ctx, base, head)
}
func NewADR(root string, cfg *config.Config, repo *awfgit.Repo, ctx context.Context, title string) (string, error) {
	return newADR(root, cfg, repo, ctx, title)
}
func NewPlan(root, title string) (string, error) { return newPlan(root, title) }
func RenderResidentMarker(state *ProjectState, cfg *config.Config, ctx context.Context, name string) (RenderedFile, error) {
	return renderResidentMarkerOperation(operationInputs(state, cfg), ctx, name)
}
func NewPitfall(root, title string) (presentation.Document, error) { return newPitfall(root, title) }
func QueryTopic(root string, repo *awfgit.Repo, ctx context.Context, selector string, opts topic.QueryOptions) (topic.QueryResult, error) {
	return queryTopic(root, repo, ctx, selector, opts)
}
