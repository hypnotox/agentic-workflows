package project

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/commitpolicy"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

type policyRepoStub struct {
	commits   []commitpolicy.Commit
	walkErr   error
	verdict   commitpolicy.SignatureVerdict
	verifyErr error
}

func (s policyRepoStub) CommitsAfter(context.Context, string, []string) ([]commitpolicy.Commit, error) {
	return s.commits, s.walkErr
}

func (s policyRepoStub) VerifySSH(context.Context, string, []commitpolicy.Signer) (commitpolicy.SignatureVerdict, error) {
	return s.verdict, s.verifyErr
}

func TestVerifyCommitPolicyDisabledAndConfigured(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	repo := gitfixture.InitRepo(t)
	base := gitfixture.Commit(t, repo, "base", map[string]string{"a": "a"})
	head := gitfixture.Commit(t, repo, "head", map[string]string{"b": "b"})
	testsupport.WriteAwfConfig(t, repo.Root(), "prefix: x\nintegrationBranch: master\ntargets: [pi]\ncommitPolicy:\n  grandfatheredThrough: "+base+"\n  allowedIdentities:\n    - name: T\n      email: t@example.com\n")
	p, err := Open(ctx, repo.Root())
	if err != nil {
		t.Fatal(err)
	}
	out := p.VerifyCommitPolicy(ctx, []string{"HEAD", "HEAD"})
	if !out.OK() || out.Disabled {
		t.Fatalf("configured outcome = %#v", out)
	}
	if !strings.Contains(p.CommitPolicyText(out), "conform") {
		t.Fatal(p.CommitPolicyText(out))
	}
	p.Cfg.CommitPolicy = nil
	out = p.VerifyCommitPolicy(ctx, []string{head})
	if !out.Disabled || !strings.Contains(p.CommitPolicyText(out), "disabled") {
		t.Fatalf("disabled outcome = %#v", out)
	}
	p.Cfg.CommitPolicy = &config.CommitPolicyConfig{GrandfatheredThrough: base, AllowedIdentities: []config.CommitPolicyIdentity{{Name: "T", Email: "t@example.com"}}}
	out = p.VerifyCommitPolicy(ctx, []string{"missing"})
	if out.Refusal == nil || out.Refusal.Category != commitpolicy.RevisionFailure {
		t.Fatalf("revision refusal = %#v", out)
	}
	gitRepo, err := awfgit.Open(repo.Root())
	if err != nil {
		t.Fatal(err)
	}
	p.repo = gitRepo
	p.Cfg.CommitPolicy.RequireSignedCommits = true
	p.Cfg.CommitPolicy.AllowedSigners = []config.CommitPolicySigner{{Principal: "t@example.com", Key: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIA=="}}
	out = p.VerifyCommitPolicy(ctx, []string{head})
	if len(out.Violations) != 1 || out.Violations[0].Field != commitpolicy.SignatureField {
		t.Fatalf("signature outcome = %#v", out)
	}
	if !strings.Contains(p.CommitPolicyText(out), "allowed signers") {
		t.Fatal(p.CommitPolicyText(out))
	}
	p.repo = nil
	out = p.VerifyCommitPolicy(ctx, []string{head})
	if out.Refusal == nil || out.Refusal.Category != commitpolicy.LinkedWorktreeFailure {
		t.Fatalf("missing repo = %#v", out)
	}
}

func TestCommitPolicyCompositionMapsEveryOperationalRefusal(t *testing.T) {
	ctx := testContext(t)
	policy := commitpolicy.Policy{RequireSigned: true, AllowedSigners: []commitpolicy.Signer{{Principal: "t@example.com", Key: "key"}}}
	categories := []struct {
		kind commitpolicy.Category
		git  awfgit.CommitPolicyErrorKind
	}{
		{commitpolicy.BaselineFailure, awfgit.CommitPolicyBaselineError},
		{commitpolicy.RevisionFailure, awfgit.CommitPolicyRevisionError},
		{commitpolicy.TagPeelFailure, awfgit.CommitPolicyTagPeelError},
		{commitpolicy.TrustFileFailure, awfgit.CommitPolicyTrustError},
		{commitpolicy.SignatureProcessFailure, awfgit.CommitPolicyVerifyError},
	}
	for _, tc := range categories {
		t.Run(string(tc.kind), func(t *testing.T) {
			gitErr := &awfgit.CommitPolicyError{Kind: tc.git, Target: "target", Err: errors.New("fault")}
			stub := policyRepoStub{walkErr: gitErr}
			if tc.git == awfgit.CommitPolicyTrustError || tc.git == awfgit.CommitPolicyVerifyError {
				stub = policyRepoStub{commits: []commitpolicy.Commit{{ID: "abc"}}, verifyErr: gitErr}
			}
			out := verifyCommitPolicy(ctx, policy, strings.Repeat("a", 40), []string{"HEAD"}, stub)
			if out.Refusal == nil || out.Refusal.Category != tc.kind || !errors.Is(out.Refusal.Cause, gitErr) || out.Refusal.RefsChanged || out.Refusal.IndexChanged || len(out.Refusal.Actions) < 2 {
				t.Fatalf("outcome = %#v", out)
			}
		})
	}
	unknown := verifyCommitPolicy(ctx, policy, "base", []string{"HEAD"}, policyRepoStub{walkErr: errors.New("unknown")})
	if unknown.Refusal == nil || unknown.Refusal.Category != commitpolicy.RevisionFailure {
		t.Fatalf("unknown Git refusal = %#v", unknown)
	}
}

func TestVerifyCommitPolicyAtUsesInvokingLinkedWorktreeConfig(t *testing.T) {
	ctx := testContext(t)
	primary := gitfixture.InitNativeAt(t, t.TempDir())
	base := gitfixture.NativeCommitAllowEmpty(t, primary, "base")
	head := gitfixture.NativeCommitAllowEmpty(t, primary, "head")
	linkedRoot := filepath.Join(t.TempDir(), "linked")
	gitfixture.NativeWorktreeAdd(t, primary, linkedRoot, "linked-policy")
	primaryConfig := "prefix: x\nintegrationBranch: master\ntargets: [pi]\ncommitPolicy:\n  grandfatheredThrough: " + base + "\n  allowedIdentities:\n    - name: Wrong\n      email: wrong@example.com\n"
	linkedConfig := "prefix: x\nintegrationBranch: master\ntargets: [pi]\ncommitPolicy:\n  grandfatheredThrough: " + base + "\n  allowedIdentities:\n    - name: T\n      email: t@example.com\n"
	testsupport.WriteAwfConfig(t, primary.Root(), primaryConfig)
	testsupport.WriteAwfConfig(t, linkedRoot, linkedConfig)
	for _, hooksPath := range []string{".githooks", filepath.Join(t.TempDir(), "hooks")} {
		gitfixture.NativeConfig(t, gitfixture.At(linkedRoot), "core.hooksPath", hooksPath)
		text, out := VerifyCommitPolicyAt(ctx, linkedRoot, []string{head})
		if !out.OK() || !strings.Contains(text, "conform") {
			t.Fatalf("linked policy (%s) = %#v, %q", hooksPath, out, text)
		}
	}
	text, out := VerifyCommitPolicyAt(ctx, primary.Root(), []string{head})
	if out.OK() || len(out.Violations) != 2 || !strings.Contains(text, "identity") {
		t.Fatalf("primary policy = %#v, %q", out, text)
	}
}

func TestVerifyCommitPolicyAtReturnsTypedRootAndConfigRefusals(t *testing.T) {
	text, out := VerifyCommitPolicyAt(testContext(t), t.TempDir(), []string{"HEAD"})
	if out.Refusal == nil || out.Refusal.Category != commitpolicy.LinkedWorktreeFailure || !strings.Contains(text, "refs changed: false") {
		t.Fatalf("root refusal = %#v, %q", out, text)
	}
	fixture := gitfixture.InitNativeAt(t, t.TempDir())
	gitfixture.NativeCommitAllowEmpty(t, fixture, "base")
	testsupport.WriteFile(t, filepath.Join(fixture.Root(), ".awf", "config.yaml"), "bad: [")
	text, out = VerifyCommitPolicyAt(testContext(t), fixture.Root(), []string{"HEAD"})
	if out.Refusal == nil || out.Refusal.Category != commitpolicy.ConfigFailure || !strings.Contains(text, "load commitPolicy") {
		t.Fatalf("config refusal = %#v, %q", out, text)
	}
}
