package project

import (
	"context"
	"errors"
	"os"
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

// invariant: tooling/commit-policy:exact-commit-enforcement (TestExactCommitEnforcement)
func TestExactCommitEnforcement(t *testing.T) {
	ctx := testContext(t)
	primary := gitfixture.InitNativeAt(t, t.TempDir())
	base := gitfixture.NativeCommitAllowEmpty(t, primary, "base")
	linkedRoot := filepath.Join(t.TempDir(), "linked")
	gitfixture.NativeWorktreeAdd(t, primary, linkedRoot, "linked-policy")
	linked := gitfixture.At(linkedRoot)
	privateKey, publicKey := gitfixture.NativeSSHKey(t)
	signed := gitfixture.NativeSignedCommit(t, linked, "signed", privateKey)
	gitfixture.NativeAnnotatedTag(t, linked, "policy-inner", signed)
	gitfixture.NativeAnnotatedTag(t, linked, "policy-outer", "policy-inner")
	primaryHead := gitfixture.NativeCommitAllowEmpty(t, primary, "primary unsigned")
	primaryConfig := "prefix: x\nintegrationBranch: master\ntargets: [pi]\ncommitPolicy:\n  grandfatheredThrough: " + base + "\n  allowedIdentities:\n    - name: Wrong\n      email: wrong@example.com\n"
	linkedPolicy := func(key string) string {
		return "prefix: x\nintegrationBranch: master\ntargets: [pi]\ncommitPolicy:\n  grandfatheredThrough: " + base + "\n  allowedIdentities:\n    - name: T\n      email: t@example.com\n  requireSignedCommits: true\n  allowedSigners:\n    - principal: t@example.com\n      key: " + key + "\n"
	}
	testsupport.WriteAwfConfig(t, primary.Root(), primaryConfig)
	testsupport.WriteAwfConfig(t, linkedRoot, linkedPolicy(publicKey))
	assertUnchanged := func(targets []string) (string, commitpolicy.Outcome) {
		t.Helper()
		beforeHead := gitfixture.NativeRevParse(t, linked, "HEAD")
		beforeIndex := gitfixture.NativeWriteTree(t, linked)
		configPath := filepath.Join(linkedRoot, ".awf", "config.yaml")
		beforeConfig, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		text, outcome := VerifyCommitPolicyAt(ctx, linkedRoot, targets)
		if afterHead := gitfixture.NativeRevParse(t, linked, "HEAD"); afterHead != beforeHead {
			t.Fatalf("verifier moved HEAD: %s -> %s", beforeHead, afterHead)
		}
		if afterIndex := gitfixture.NativeWriteTree(t, linked); afterIndex != beforeIndex {
			t.Fatalf("verifier changed index: %s -> %s", beforeIndex, afterIndex)
		}
		afterConfig, err := os.ReadFile(configPath)
		if err != nil || string(afterConfig) != string(beforeConfig) {
			t.Fatalf("verifier changed config: %v", err)
		}
		matches, err := filepath.Glob(filepath.Join(linkedRoot, ".awf-allowed-signers-*"))
		if err != nil || len(matches) != 0 {
			t.Fatalf("verifier retained trust material: %v, %v", matches, err)
		}
		return text, outcome
	}
	for _, hooksPath := range []string{".githooks", filepath.Join(t.TempDir(), "hooks")} {
		gitfixture.NativeConfig(t, linked, "core.hooksPath", hooksPath)
		text, outcome := assertUnchanged([]string{"policy-outer", signed, base + ".." + signed})
		if !outcome.OK() || !strings.Contains(text, "conform") {
			t.Fatalf("linked signed policy (%s) = %#v, %q", hooksPath, outcome, text)
		}
	}
	text, outcome := VerifyCommitPolicyAt(ctx, primary.Root(), []string{primaryHead})
	if outcome.OK() || len(outcome.Violations) != 2 || !strings.Contains(text, "identity") {
		t.Fatalf("primary policy = %#v, %q", outcome, text)
	}
	_, wrongKey := gitfixture.NativeSSHKey(t)
	testsupport.WriteAwfConfig(t, linkedRoot, linkedPolicy(wrongKey))
	text, outcome = assertUnchanged([]string{signed})
	if outcome.OK() || len(outcome.Violations) != 1 || outcome.Violations[0].Field != commitpolicy.SignatureField || !strings.Contains(text, "allowed signers") {
		t.Fatalf("wrong signer = %#v, %q", outcome, text)
	}
	testsupport.WriteAwfConfig(t, linkedRoot, linkedPolicy(publicKey))
	text, outcome = assertUnchanged([]string{"missing-target"})
	if outcome.Refusal == nil || outcome.Refusal.Category != commitpolicy.RevisionFailure || outcome.Refusal.RefsChanged || outcome.Refusal.IndexChanged || !strings.Contains(text, "cause:") {
		t.Fatalf("revision refusal = %#v, %q", outcome, text)
	}
	dangling := gitfixture.NativeHashObject(t, linked, "tag", []byte("object "+strings.Repeat("f", len(base))+"\ntype commit\ntag broken\ntagger T <t@example.com> 1 +0000\n\nbroken\n"))
	gitfixture.NativeUpdateRef(t, linked, "refs/tags/broken-policy-tag", dangling)
	_, outcome = assertUnchanged([]string{"broken-policy-tag"})
	if outcome.Refusal == nil || outcome.Refusal.Category != commitpolicy.TagPeelFailure {
		t.Fatalf("tag-peel refusal = %#v", outcome)
	}
	testsupport.WriteAwfConfig(t, linkedRoot, "prefix: x\nintegrationBranch: master\ntargets: [pi]\n")
	text, outcome = VerifyCommitPolicyAt(ctx, linkedRoot, []string{signed})
	if !outcome.Disabled || !outcome.OK() || !strings.Contains(text, "disabled") {
		t.Fatalf("disabled policy = %#v, %q", outcome, text)
	}

	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			fixture := gitfixture.InitNativeObjectFormat(t, t.TempDir(), format)
			formatBase := gitfixture.NativeCommitAllowEmpty(t, fixture, "base")
			formatHead := gitfixture.NativeCommitAllowEmpty(t, fixture, "head")
			repo, err := awfgit.Open(fixture.Root())
			if err != nil {
				t.Fatal(err)
			}
			commits, err := repo.CommitsAfter(ctx, formatBase, []string{"HEAD", formatBase + "..HEAD"})
			if err != nil || len(commits) != 1 || commits[0].ID != formatHead {
				t.Fatalf("%s verifier facts = %#v, %v", format, commits, err)
			}
		})
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
