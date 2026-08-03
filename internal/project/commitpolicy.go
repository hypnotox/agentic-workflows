package project

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/commitpolicy"
)

// VerifyCommitPolicy evaluates explicit revisions through this invoking project's configured policy.
func (p *Project) VerifyCommitPolicy(ctx context.Context, targets []string) commitpolicy.Outcome {
	if p.Cfg.CommitPolicy == nil {
		return commitpolicy.Outcome{Disabled: true}
	}
	repo, err := p.gitRepo()
	if err != nil {
		return refused(commitpolicy.LinkedWorktreeFailure, "open invoking worktree", err)
	}
	cfg := p.Cfg.CommitPolicy
	policy := commitpolicy.Policy{RequireSigned: cfg.RequireSignedCommits}
	for _, i := range cfg.AllowedIdentities {
		policy.AllowedIdentities = append(policy.AllowedIdentities, commitpolicy.Identity{Name: i.Name, Email: i.Email})
	}
	for _, s := range cfg.AllowedSigners {
		policy.AllowedSigners = append(policy.AllowedSigners, commitpolicy.Signer{Principal: s.Principal, Key: s.Key})
	}
	commits, err := repo.CommitsAfter(ctx, cfg.GrandfatheredThrough, targets)
	if err != nil {
		return refused(commitpolicy.BaselineFailure, "resolve baseline or target", err)
	}
	for i := range commits {
		if !policy.RequireSigned {
			continue
		}
		verdict, verifyErr := repo.VerifySSH(ctx, commits[i].ID, policy.AllowedSigners)
		if verifyErr != nil { // coverage-ignore: native process refusal is independently pinned at the Git boundary
			return refused(commitpolicy.SignatureProcessFailure, "verify SSH signature", verifyErr)
		}
		commits[i].Signature = verdict
	}
	return commitpolicy.Evaluate(policy, commits)
}

func refused(category commitpolicy.Category, observed string, cause error) commitpolicy.Outcome {
	return commitpolicy.Outcome{Refusal: &commitpolicy.Refusal{Category: category, Observed: observed, Cause: cause, Actions: []string{"correct configuration or repository state", "rerun awf check commit-policy with explicit targets"}}}
}

// CommitPolicyText renders one verifier outcome using the project configuration.
func (p *Project) CommitPolicyText(outcome commitpolicy.Outcome) string {
	if p.Cfg.CommitPolicy == nil {
		return commitpolicy.Render(commitpolicy.Policy{}, outcome)
	}
	policy := commitpolicy.Policy{RequireSigned: p.Cfg.CommitPolicy.RequireSignedCommits}
	for _, i := range p.Cfg.CommitPolicy.AllowedIdentities {
		policy.AllowedIdentities = append(policy.AllowedIdentities, commitpolicy.Identity{Name: i.Name, Email: i.Email})
	}
	for _, s := range p.Cfg.CommitPolicy.AllowedSigners {
		policy.AllowedSigners = append(policy.AllowedSigners, commitpolicy.Signer{Principal: s.Principal, Key: s.Key})
	}
	return commitpolicy.Render(policy, outcome)
}
