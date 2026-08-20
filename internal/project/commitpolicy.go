package project

import (
	"context"
	"errors"

	"github.com/hypnotox/agentic-workflows/internal/commitpolicy"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

type commitPolicyRepository interface {
	CommitsAfter(context.Context, string, []string) ([]commitpolicy.Commit, error)
	VerifySSH(context.Context, string, []commitpolicy.Signer) (commitpolicy.SignatureVerdict, error)
}

// VerifyCommitPolicy evaluates explicit revisions through this invoking project's configured policy.
func verifyCommitPolicyFacade(cfg *config.Config, root string, repository *awfgit.Repo, ctx context.Context, targets []string) commitpolicy.Outcome {
	if cfg.CommitPolicy == nil {
		return commitpolicy.Outcome{Disabled: true}
	}
	repo, err := gitRepo(root, repository)
	if err != nil {
		return refused(commitpolicy.LinkedWorktreeFailure, "open invoking worktree", err)
	}
	return verifyCommitPolicy(ctx, policyFromConfig(cfg.CommitPolicy), cfg.CommitPolicy.GrandfatheredThrough, targets, repo)
}

func verifyCommitPolicy(ctx context.Context, policy commitpolicy.Policy, baseline string, targets []string, repo commitPolicyRepository) commitpolicy.Outcome {
	commits, err := repo.CommitsAfter(ctx, baseline, targets)
	if err != nil {
		return gitRefusal(err)
	}
	for i := range commits {
		if !policy.RequireSigned {
			continue
		}
		verdict, verifyErr := repo.VerifySSH(ctx, commits[i].ID, policy.AllowedSigners)
		if verifyErr != nil {
			return gitRefusal(verifyErr)
		}
		commits[i].Signature = verdict
	}
	return commitpolicy.Evaluate(policy, commits)
}

func gitRefusal(err error) commitpolicy.Outcome {
	var policyErr *awfgit.CommitPolicyError
	if !errors.As(err, &policyErr) {
		return refused(commitpolicy.RevisionFailure, "read repository commit facts", err)
	}
	category := commitpolicy.RevisionFailure
	switch policyErr.Kind {
	case awfgit.CommitPolicyBaselineError:
		category = commitpolicy.BaselineFailure
	case awfgit.CommitPolicyRevisionError:
		category = commitpolicy.RevisionFailure
	case awfgit.CommitPolicyTagPeelError:
		category = commitpolicy.TagPeelFailure
	case awfgit.CommitPolicyTrustError:
		category = commitpolicy.TrustFileFailure
	case awfgit.CommitPolicyVerifyError:
		category = commitpolicy.SignatureProcessFailure
	}
	return refused(category, policyErr.Error(), err)
}

func refused(category commitpolicy.Category, observed string, cause error) commitpolicy.Outcome {
	actions := []string{"correct repository state or the explicit target and rerun awf check commit-policy"}
	switch category {
	case commitpolicy.ConfigFailure:
		actions = []string{"correct commitPolicy in the invoking worktree", "rerun awf check commit-policy with the same explicit targets"}
	case commitpolicy.BaselineFailure:
		actions = []string{"set commitPolicy.grandfatheredThrough to one full commit object ID", "rerun awf check commit-policy with the same explicit targets"}
	case commitpolicy.RevisionFailure:
		actions = []string{"correct the missing or invalid explicit revision or range", "rerun awf check commit-policy with explicit targets"}
	case commitpolicy.TagPeelFailure:
		actions = []string{"select a revision or tag whose recursively peeled target is a commit", "rerun awf check commit-policy with explicit targets"}
	case commitpolicy.LinkedWorktreeFailure:
		actions = []string{"restore the invoking linked worktree registration and checkout path", "rerun from that worktree"}
	case commitpolicy.TrustFileFailure:
		actions = []string{"restore safe temporary-file access in the invoking worktree", "rerun awf check commit-policy with the same explicit targets"}
	case commitpolicy.SignatureProcessFailure:
		actions = []string{"correct the Git SSH signature verification failure", "rerun awf check commit-policy with the same explicit targets"}
	}
	return commitpolicy.Outcome{Refusal: &commitpolicy.Refusal{Category: category, Observed: observed, RefsChanged: false, IndexChanged: false, Actions: actions, Cause: cause}}
}

func policyFromConfig(cfg *config.CommitPolicyConfig) commitpolicy.Policy {
	policy := commitpolicy.Policy{RequireSigned: cfg.RequireSignedCommits}
	for _, identity := range cfg.AllowedIdentities {
		policy.AllowedIdentities = append(policy.AllowedIdentities, commitpolicy.Identity{Name: identity.Name, Email: identity.Email})
	}
	for _, signer := range cfg.AllowedSigners {
		policy.AllowedSigners = append(policy.AllowedSigners, commitpolicy.Signer{Principal: signer.Principal, Key: signer.Key})
	}
	return policy
}

// CommitPolicyPresentation maps one verifier outcome using the project configuration.
func commitPolicyPresentation(cfg *config.Config, outcome commitpolicy.Outcome) (presentation.Document, error) {
	if cfg.CommitPolicy == nil {
		return commitpolicy.Presentation(commitpolicy.Policy{}, outcome)
	}
	return commitpolicy.Presentation(policyFromConfig(cfg.CommitPolicy), outcome)
}

// VerifyCommitPolicyAt resolves the invoking worktree and returns its typed outcome
// together with its model-owned presentation.
func VerifyCommitPolicyAt(ctx context.Context, root string, targets []string) (presentation.Document, commitpolicy.Outcome, error) {
	roots, err := awfgit.ResolveControlRoots(ctx, root)
	if err != nil {
		outcome := refused(commitpolicy.LinkedWorktreeFailure, "resolve invoking worktree", err)
		document, presentationErr := commitpolicy.Presentation(commitpolicy.Policy{}, outcome)
		return document, outcome, presentationErr
	}
	p, err := Open(ctx, roots.InvokingRoot)
	if err != nil {
		outcome := refused(commitpolicy.ConfigFailure, "load commitPolicy from "+roots.InvokingRoot, err)
		document, presentationErr := commitpolicy.Presentation(commitpolicy.Policy{}, outcome)
		return document, outcome, presentationErr
	}
	outcome := p.VerifyCommitPolicy(ctx, targets)
	document, presentationErr := p.CommitPolicyPresentation(outcome)
	return document, outcome, presentationErr
}
