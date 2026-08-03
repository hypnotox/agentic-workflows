package project

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/commitpolicy"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

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
	if out.Refusal == nil || out.Refusal.Category != commitpolicy.BaselineFailure {
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
