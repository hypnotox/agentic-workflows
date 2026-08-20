package project

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/commitmsg"
	"github.com/hypnotox/agentic-workflows/internal/commitpolicy"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// NumberPendingADRs forwards through the temporary compatibility facade;
// Phase 3 removes this bridge with its command consumers.
func (p *Project) NumberPendingADRs(ctx context.Context, slugs []string) (NumberingReport, error) {
	return numberPendingADRs(newRenderInputs(p.state, p.Cfg, p.read), ctx, slugs)
}
func (p *Project) AdvisoryNotes(ctx context.Context) ([]string, error) {
	return advisoryNotes(newRenderInputs(p.state, p.Cfg, p.read), ctx)
}
func (p *Project) CheckReport(ctx context.Context) (CheckReport, error) {
	return checkReport(newRenderInputs(p.state, p.Cfg, p.read), p.repo, ctx)
}
func (p *Project) VerifyCommitPolicy(ctx context.Context, targets []string) commitpolicy.Outcome {
	return verifyCommitPolicyFacade(p.Cfg, p.Root, p.repo, ctx, targets)
}
func (p *Project) CommitPolicyPresentation(outcome commitpolicy.Outcome) (presentation.Document, error) {
	return commitPolicyPresentation(p.Cfg, outcome)
}
func (p *Project) ConfigReferenceModel(ctx context.Context) (ConfigReference, error) {
	return configReferenceModel(newRenderInputs(p.state, p.Cfg, p.read), ctx)
}
func (p *Project) ContextState(ctx context.Context) (ContextState, error) {
	return contextState(p.state, p.repo, ctx)
}
func (p *Project) CheckCurrentState(ctx context.Context) (CurrentStateReport, error) {
	return checkCurrentState(p.Root, p.repo, ctx)
}
func (p *Project) CheckCommitAuthorization(ctx context.Context, msg commitmsg.Message) (CommitAuthorizationResult, error) {
	return checkCommitAuthorization(p.Root, p.repo, ctx, msg)
}
func (p *Project) InitCollisions(ctx context.Context) ([]string, error) {
	return initCollisions(newRenderInputs(p.state, p.Cfg, p.read), ctx)
}
func (p *Project) ListDocument(kindFilter string) (presentation.Document, error) {
	return listDocument(p.Cfg, p.state.catalog(), kindFilter)
}
func (p *Project) OutputPlan(ctx context.Context) (*OutputPlan, error) {
	return outputPlan(newRenderInputs(p.state, p.Cfg, p.read), ctx)
}
func (p *Project) PreflightLocalDoc(ctx context.Context, doc config.LocalDoc) error {
	return preflightLocalDoc(newRenderInputs(p.state, p.Cfg, p.read), ctx, doc)
}
func (p *Project) PlannedOutputs(ctx context.Context) ([]string, error) {
	op, err := p.OutputPlan(ctx)
	if err != nil {
		return nil, err
	}
	return plannedOutputPaths(op), nil
}
func (p *Project) ReadPlan(name, selector string) ([]byte, error) {
	return readPlan(p.Root, name, selector)
}
func (p *Project) SyncReport(ctx context.Context) ([]Backup, []Change, []string, error) {
	return syncReportOperation(newRenderInputs(p.state, p.Cfg, p.read), ctx)
}
func (p *Project) InitializeReport(ctx context.Context, seed InitAuthority) ([]Backup, []Change, []string, error) {
	return initializeReport(newRenderInputs(p.state, p.Cfg, p.read), ctx, seed)
}
func (p *Project) Audit(ctx context.Context, base, head string) ([]audit.Finding, int, error) {
	return auditOperation(p.Root, p.Cfg, ctx, base, head)
}
func (p *Project) NewADR(ctx context.Context, title string) (string, error) {
	return newADR(p.Root, p.Cfg, p.repo, ctx, title)
}
func (p *Project) NewPlan(title string) (string, error) {
	return newPlan(p.Root, title)
}
func (p *Project) RenderResidentMarker(ctx context.Context, name string) (RenderedFile, error) {
	return renderResidentMarkerOperation(newRenderInputs(p.state, p.Cfg, p.read), ctx, name)
}
func (p *Project) NewPitfall(title string) (presentation.Document, error) {
	return newPitfall(p.Root, title)
}
func (p *Project) QueryTopic(ctx context.Context, selector string, opts topic.QueryOptions) (topic.QueryResult, error) {
	return queryTopic(p.Root, p.repo, ctx, selector, opts)
}
