package project

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/commitmsg"
	"github.com/hypnotox/agentic-workflows/internal/commitpolicy"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// Project is a temporary compatibility facade. Each method forwards to its
// operation owner; Phase 3 removes this bridge with its command consumers.
func (p *Project) NumberPendingADRs(ctx context.Context, slugs []string) (NumberingReport, error) {
	return NumberPendingADRsOperation(p, ctx, slugs)
}
func (p *Project) AdvisoryNotes(ctx context.Context) ([]string, error) {
	return AdvisoryNotesOperation(p, ctx)
}
func (p *Project) CheckReport(ctx context.Context) (CheckReport, error) {
	return CheckReportOperation(p, ctx)
}
func (p *Project) VerifyCommitPolicy(ctx context.Context, targets []string) commitpolicy.Outcome {
	return VerifyCommitPolicyOperation(p, ctx, targets)
}
func (p *Project) CommitPolicyPresentation(outcome commitpolicy.Outcome) (presentation.Document, error) {
	return CommitPolicyPresentationOperation(p, outcome)
}
func (p *Project) ConfigReferenceModel(ctx context.Context) (ConfigReference, error) {
	return ConfigReferenceModelOperation(p, ctx)
}
func (p *Project) ContextState(ctx context.Context) (ContextState, error) {
	return ContextStateOperation(p, ctx)
}
func (p *Project) CheckCurrentState(ctx context.Context) (CurrentStateReport, error) {
	return CheckCurrentStateOperation(p, ctx)
}
func (p *Project) CheckStaged(ctx context.Context) (CurrentStateReport, error) {
	return CheckStagedOperation(p, ctx)
}
func (p *Project) CheckCommitAuthorization(ctx context.Context, msg commitmsg.Message) (CommitAuthorizationResult, error) {
	return CheckCommitAuthorizationOperation(p, ctx, msg)
}
func (p *Project) InitCollisions(ctx context.Context) ([]string, error) {
	return InitCollisionsOperation(p, ctx)
}
func (p *Project) ListDocument(kindFilter string) (presentation.Document, error) {
	return ListDocumentOperation(p, kindFilter)
}
func (p *Project) OutputPlan(ctx context.Context) (*OutputPlan, error) {
	return OutputPlanOperation(p, ctx)
}
func (p *Project) PreflightLocalDoc(ctx context.Context, doc config.LocalDoc) error {
	return PreflightLocalDocOperation(p, ctx, doc)
}
func (p *Project) PlannedOutputs(ctx context.Context) ([]string, error) {
	return PlannedOutputsOperation(p, ctx)
}
func (p *Project) ReadPlan(name, selector string) ([]byte, error) {
	return ReadPlanOperation(p, name, selector)
}
func (p *Project) SyncReport(ctx context.Context) ([]Backup, []Change, []string, error) {
	return SyncReportOperation(p, ctx)
}
func (p *Project) InitializeReport(ctx context.Context, seed InitAuthority) ([]Backup, []Change, []string, error) {
	return InitializeReportOperation(p, ctx, seed)
}
func (p *Project) Audit(ctx context.Context, base, head string) ([]audit.Finding, int, error) {
	return AuditOperation(p, ctx, base, head)
}
func (p *Project) NewADR(ctx context.Context, title string) (string, error) {
	return NewADROperation(p, ctx, title)
}
func (p *Project) NewPlan(title string) (string, error) { return NewPlanOperation(p, title) }
func (p *Project) RenderResidentMarker(ctx context.Context, name string) (RenderedFile, error) {
	return RenderResidentMarkerOperation(p, ctx, name)
}
func (p *Project) NewPitfall(title string) (presentation.Document, error) {
	return NewPitfallOperation(p, title)
}
func (p *Project) CheckStagedDrift(ctx context.Context) ([]manifest.Drift, error) {
	return CheckStagedDriftOperation(p, ctx)
}
func (p *Project) QueryTopic(ctx context.Context, selector string, opts topic.QueryOptions) (topic.QueryResult, error) {
	return QueryTopicOperation(p, ctx, selector, opts)
}
