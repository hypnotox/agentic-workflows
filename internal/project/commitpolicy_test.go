package project

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/commitpolicy"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func documentText(t *testing.T, document presentation.Document) string {
	t.Helper()
	var output bytes.Buffer
	if err := presentation.Render(&output, document); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func commitPolicyPresentationText(t *testing.T, cfg *config.Config, outcome commitpolicy.Outcome) string {
	t.Helper()
	document, err := commitPolicyPresentation(cfg, outcome)
	if err != nil {
		t.Fatal(err)
	}
	return documentText(t, document)
}

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
	testsupport.WriteAwfConfig(t, repo.Root(), "prefix: x\nintegrationBranch: master\ncommitPolicy:\n  grandfatheredThrough: "+base+"\n  allowedIdentities:\n    - name: T\n      email: t@example.com\n")
	state, err := Open(ctx, repo.Root())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(config.RootDir(repo.Root()))
	if err != nil {
		t.Fatal(err)
	}
	gitRepo, err := awfgit.Open(repo.Root())
	if err != nil {
		t.Fatal(err)
	}
	out := verifyCommitPolicyOperation(cfg, state.Root(), gitRepo, ctx, []string{"HEAD", "HEAD"})
	if !out.OK() || out.Disabled {
		t.Fatalf("configured outcome = %#v", out)
	}
	if !strings.Contains(commitPolicyPresentationText(t, cfg, out), "conform") {
		t.Fatal(commitPolicyPresentationText(t, cfg, out))
	}
	cfg.CommitPolicy = nil
	out = verifyCommitPolicyOperation(cfg, state.Root(), gitRepo, ctx, []string{head})
	if !out.Disabled || !strings.Contains(commitPolicyPresentationText(t, cfg, out), "disabled") {
		t.Fatalf("disabled outcome = %#v", out)
	}
	cfg.CommitPolicy = &config.CommitPolicyConfig{GrandfatheredThrough: base, AllowedIdentities: []config.CommitPolicyIdentity{{Name: "T", Email: "t@example.com"}}}
	out = verifyCommitPolicyOperation(cfg, state.Root(), gitRepo, ctx, []string{"missing"})
	if out.Refusal == nil || out.Refusal.Category != commitpolicy.RevisionFailure {
		t.Fatalf("revision refusal = %#v", out)
	}
	cfg.CommitPolicy.RequireSignedCommits = true
	cfg.CommitPolicy.AllowedSigners = []config.CommitPolicySigner{{Principal: "t@example.com", Key: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIA=="}}
	out = verifyCommitPolicyOperation(cfg, state.Root(), gitRepo, ctx, []string{head})
	if len(out.Violations) != 1 || out.Violations[0].Field != commitpolicy.SignatureField {
		t.Fatalf("signature outcome = %#v", out)
	}
	if !strings.Contains(commitPolicyPresentationText(t, cfg, out), "signature") {
		t.Fatal(commitPolicyPresentationText(t, cfg, out))
	}
	out = verifyCommitPolicyOperation(cfg, state.Root(), nil, ctx, []string{head})
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
	tagPrivate, tagPublic := gitfixture.NativeSSHKey(t)
	directPrivate, directPublic := gitfixture.NativeSSHKey(t)
	rangePrivate, rangePublic := gitfixture.NativeSSHKey(t)
	tagCommit := gitfixture.NativeSignedCommitAs(t, linked, "tag-only", tagPrivate, "Tag", "tag@example.com")
	gitfixture.NativeAnnotatedTag(t, linked, "policy-inner", tagCommit)
	gitfixture.NativeAnnotatedTag(t, linked, "policy-outer", "policy-inner")
	gitfixture.NativeCheckoutNewBranch(t, linked, "direct-only", base)
	directCommit := gitfixture.NativeSignedCommitAs(t, linked, "direct-only", directPrivate, "Direct", "direct@example.com")
	gitfixture.NativeCheckoutNewBranch(t, linked, "range-only", base)
	rangeCommitOne := gitfixture.NativeSignedCommitAs(t, linked, "range-one", rangePrivate, "Range", "range@example.com")
	rangeCommitTwo := gitfixture.NativeSignedCommitAs(t, linked, "range-two", rangePrivate, "Range", "range@example.com")
	primaryHead := gitfixture.NativeCommitAllowEmpty(t, primary, "primary unsigned")
	primaryConfig := "prefix: x\nintegrationBranch: master\ncommitPolicy:\n  grandfatheredThrough: " + base + "\n  allowedIdentities:\n    - name: Wrong\n      email: wrong@example.com\n"
	type allowedProvenance struct{ name, email, key string }
	tagPolicy := allowedProvenance{"Tag", "tag@example.com", tagPublic}
	directPolicy := allowedProvenance{"Direct", "direct@example.com", directPublic}
	rangePolicy := allowedProvenance{"Range", "range@example.com", rangePublic}
	linkedPolicy := func(values ...allowedProvenance) string {
		var body strings.Builder
		body.WriteString("prefix: x\nintegrationBranch: master\ncommitPolicy:\n  grandfatheredThrough: " + base + "\n  allowedIdentities:\n")
		for _, value := range values {
			body.WriteString("    - name: " + value.name + "\n      email: " + value.email + "\n")
		}
		body.WriteString("  requireSignedCommits: true\n  allowedSigners:\n")
		for _, value := range values {
			body.WriteString("    - principal: " + value.email + "\n      key: " + value.key + "\n")
		}
		return body.String()
	}
	testsupport.WriteAwfConfig(t, primary.Root(), primaryConfig)
	testsupport.WriteAwfConfig(t, linkedRoot, linkedPolicy(tagPolicy, directPolicy, rangePolicy))
	assertUnchanged := func(targets []string) (string, commitpolicy.Outcome) {
		t.Helper()
		beforeHead := gitfixture.NativeRevParse(t, linked, "HEAD")
		beforeIndex := gitfixture.NativeWriteTree(t, linked)
		configPath := filepath.Join(linkedRoot, ".awf", "config.yaml")
		beforeConfig, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		document, outcome, err := VerifyCommitPolicyAt(ctx, linkedRoot, targets)
		if err != nil {
			t.Fatal(err)
		}
		text := documentText(t, document)
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
		for _, check := range []struct {
			name    string
			policy  string
			targets []string
		}{
			{"recursive-tag", linkedPolicy(tagPolicy), []string{"policy-outer"}},
			{"direct", linkedPolicy(directPolicy), []string{directCommit}},
			{"range", linkedPolicy(rangePolicy), []string{base + ".." + rangeCommitTwo}},
			{"combined", linkedPolicy(tagPolicy, directPolicy, rangePolicy), []string{"policy-outer", directCommit, base + ".." + rangeCommitTwo}},
		} {
			testsupport.WriteAwfConfig(t, linkedRoot, check.policy)
			text, outcome := assertUnchanged(check.targets)
			if !outcome.OK() || !strings.Contains(text, "conform") {
				t.Fatalf("linked signed %s policy (%s) = %#v, %q", check.name, hooksPath, outcome, text)
			}
		}
	}
	if rangeCommitOne == rangeCommitTwo || tagCommit == directCommit {
		t.Fatal("distinct target fixtures collapsed")
	}
	document, outcome, err := VerifyCommitPolicyAt(ctx, primary.Root(), []string{primaryHead})
	if err != nil {
		t.Fatal(err)
	}
	text := documentText(t, document)
	if outcome.OK() || len(outcome.Violations) != 2 || !strings.Contains(text, "author") {
		t.Fatalf("primary policy = %#v, %q", outcome, text)
	}
	_, wrongKey := gitfixture.NativeSSHKey(t)
	testsupport.WriteAwfConfig(t, linkedRoot, linkedPolicy(allowedProvenance{"Direct", "direct@example.com", wrongKey}))
	text, outcome = assertUnchanged([]string{directCommit})
	if outcome.OK() || len(outcome.Violations) != 1 || outcome.Violations[0].Field != commitpolicy.SignatureField || !strings.Contains(text, "signature") {
		t.Fatalf("wrong signer = %#v, %q", outcome, text)
	}
	testsupport.WriteAwfConfig(t, linkedRoot, linkedPolicy(directPolicy))
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
	testsupport.WriteAwfConfig(t, linkedRoot, "prefix: x\nintegrationBranch: master\n")
	text, outcome = assertUnchanged([]string{directCommit})
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
			testsupport.WriteAwfConfig(t, fixture.Root(), "prefix: x\nintegrationBranch: master\ncommitPolicy:\n  grandfatheredThrough: "+formatBase+"\n  allowedIdentities:\n    - name: T\n      email: t@example.com\n")
			beforeHead := gitfixture.NativeRevParse(t, fixture, "HEAD")
			beforeIndex := gitfixture.NativeWriteTree(t, fixture)
			document, outcome, err := VerifyCommitPolicyAt(ctx, fixture.Root(), []string{"HEAD", formatBase + "..HEAD"})
			if err != nil {
				t.Fatal(err)
			}
			text := documentText(t, document)
			if !outcome.OK() || !strings.Contains(text, "conform") {
				t.Fatalf("%s project verifier = %#v, %q", format, outcome, text)
			}
			if gitfixture.NativeRevParse(t, fixture, "HEAD") != beforeHead || gitfixture.NativeWriteTree(t, fixture) != beforeIndex || beforeHead != formatHead {
				t.Fatalf("%s project verifier mutated repository", format)
			}
			commits, err := repo.CommitsAfter(ctx, formatBase, []string{"HEAD", formatBase + "..HEAD"})
			if err != nil || len(commits) != 1 || commits[0].ID != formatHead {
				t.Fatalf("%s verifier facts = %#v, %v", format, commits, err)
			}
		})
	}
}

func TestVerifyCommitPolicyAtReturnsTypedRootAndConfigRefusals(t *testing.T) {
	document, out, err := VerifyCommitPolicyAt(testContext(t), t.TempDir(), []string{"HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	text := documentText(t, document)
	if out.Refusal == nil || out.Refusal.Category != commitpolicy.LinkedWorktreeFailure || !strings.Contains(text, "refs: false") {
		t.Fatalf("root refusal = %#v, %q", out, text)
	}
	fixture := gitfixture.InitNativeAt(t, t.TempDir())
	gitfixture.NativeCommitAllowEmpty(t, fixture, "base")
	testsupport.WriteFile(t, filepath.Join(fixture.Root(), ".awf", "config.yaml"), "bad: [")
	document, out, err = VerifyCommitPolicyAt(testContext(t), fixture.Root(), []string{"HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	text = documentText(t, document)
	if out.Refusal == nil || out.Refusal.Category != commitpolicy.ConfigFailure || !strings.Contains(text, "load commitPolicy") {
		t.Fatalf("config refusal = %#v, %q", out, text)
	}
	testsupport.WriteAwfConfig(t, fixture.Root(), "prefix: x\nprofile: unknown\nintegrationBranch: master\n")
	document, out, err = VerifyCommitPolicyAt(testContext(t), fixture.Root(), []string{"HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	text = documentText(t, document)
	if out.Refusal == nil || out.Refusal.Category != commitpolicy.ConfigFailure || !strings.Contains(text, "load commitPolicy") {
		t.Fatalf("validation refusal = %#v, %q", out, text)
	}
}

func TestCommitPolicyManifestProjection(t *testing.T) {
	const base = "prefix: example\nintegrationBranch: main\nvars: {gateCmd: make gate}\n"
	root := scaffold(t, base)
	syncAndLoad := func() *manifest.Lock {
		t.Helper()
		p, err := Open(testContext(t), root)
		if err != nil {
			t.Fatal(err)
		}
		if err := syncProject(p); err != nil {
			t.Fatal(err)
		}
		lock, err := manifest.Load(lockFile(root))
		if err != nil {
			t.Fatal(err)
		}
		return lock
	}
	absent := syncAndLoad()
	absentAgain := syncAndLoad()
	if !reflect.DeepEqual(absent, absentAgain) {
		t.Fatal("repeated absent-policy sync changed the manifest")
	}
	consumerPath := "docs/architecture.md"
	unrelatedPath := "AGENTS.md"
	consumerBefore, ok := absent.Files[consumerPath]
	if !ok {
		t.Fatalf("manifest missing consumer %s", consumerPath)
	}
	unrelatedBefore, ok := absent.Files[unrelatedPath]
	if !ok {
		t.Fatalf("manifest missing unrelated output %s", unrelatedPath)
	}
	generateKey := func(name string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), name)
		if output, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", path).CombinedOutput(); err != nil {
			t.Fatalf("generate SSH key: %v: %s", err, output)
		}
		body, err := os.ReadFile(path + ".pub")
		if err != nil {
			t.Fatal(err)
		}
		fields := strings.Fields(string(body))
		if len(fields) < 2 {
			t.Fatalf("generated public key = %q", body)
		}
		return strings.Join(fields[:2], " ")
	}
	key1, key2 := generateKey("one"), generateKey("two")
	policyYAML := func(baseline, name, email string, signed bool, principal, key string) string {
		body := fmt.Sprintf("%scommitPolicy:\n  grandfatheredThrough: %s\n  allowedIdentities:\n    - name: %s\n      email: %s\n", base, baseline, name, email)
		if signed {
			body += fmt.Sprintf("  requireSignedCommits: true\n  allowedSigners:\n    - principal: %s\n      key: %s\n", principal, key)
		}
		return body
	}
	variants := []string{
		policyYAML(strings.Repeat("a", 40), "Ada", "ada@example.test", true, "ada@example.test", key1),
		policyYAML(strings.Repeat("b", 40), "Ada", "ada@example.test", true, "ada@example.test", key1),
		policyYAML(strings.Repeat("b", 40), "Grace", "grace@example.test", true, "ada@example.test", key1),
		policyYAML(strings.Repeat("b", 40), "Grace", "grace@example.test", false, "", ""),
		policyYAML(strings.Repeat("b", 40), "Grace", "grace@example.test", true, "grace@example.test", key2),
	}
	previous := consumerBefore.ConfigHash
	for i, policy := range variants {
		testsupport.WriteAwfConfig(t, root, policy)
		lock := syncAndLoad()
		if lock.Files[consumerPath].ConfigHash == previous {
			t.Fatalf("normalized policy mutation %d did not change consumer manifest hash", i)
		}
		if lock.Files[unrelatedPath].ConfigHash != unrelatedBefore.ConfigHash {
			t.Fatalf("unrelated manifest hash changed with policy mutation %d", i)
		}
		previous = lock.Files[consumerPath].ConfigHash
	}
}
