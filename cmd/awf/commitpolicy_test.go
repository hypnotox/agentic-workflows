package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestRunCommitPolicyDisabledPolicySucceeds(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runCommitPolicy(testContext(t), root, []string{"HEAD"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "commit policy is disabled") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestCommitPolicyRenderedErrorIsTypedWithoutDuplicatePresentation(t *testing.T) {
	for _, kind := range []commitPolicyExit{commitPolicyViolationExit, commitPolicyRefusalExit} {
		err := &renderedCommitPolicyError{kind: kind}
		if !strings.Contains(err.Error(), string(kind)) {
			t.Fatalf("typed error %q does not name %s", err, kind)
		}
	}
}

func TestCommitPolicyCommandArityAndDispatch(t *testing.T) {
	root := scaffoldProject(t)
	var out, errb bytes.Buffer
	if code := runAt(t, root, []string{"awf", "check", "commit-policy"}, &out, &errb); code != 2 {
		t.Fatalf("missing target exit = %d, stderr = %q", code, errb.String())
	}
	out.Reset()
	errb.Reset()
	if code := runAt(t, root, []string{"awf", "check", "commit-policy", "HEAD"}, &out, &errb); code != 0 {
		t.Fatalf("disabled command exit = %d, stderr = %q", code, errb.String())
	}
	if !strings.Contains(out.String(), "disabled") || errb.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", out.String(), errb.String())
	}
}

func TestCommitPolicyCommandUsesRealSignedCommitsAndTypedExits(t *testing.T) {
	root := scaffoldProject(t)
	fixture := gitfixture.At(root)
	baseline := gitfixture.NativeRevParse(t, fixture, "HEAD")
	privateKey, publicKey := gitfixture.NativeSSHKey(t)
	head := gitfixture.NativeSignedCommit(t, fixture, "signed policy fixture", privateKey)
	writePolicy := func(key string) {
		t.Helper()
		testsupport.WriteAwfConfig(t, root, minimalYAML+fmt.Sprintf("commitPolicy:\n  grandfatheredThrough: %s\n  allowedIdentities:\n    - name: T\n      email: t@example.com\n  requireSignedCommits: true\n  allowedSigners:\n    - principal: t@example.com\n      key: %s\n", baseline, key))
	}
	writePolicy(publicKey)
	var out, errb bytes.Buffer
	code := runAt(t, root, []string{"awf", "check", "commit-policy", "HEAD", head, baseline + "..HEAD"}, &out, &errb)
	if code != 0 || !strings.Contains(out.String(), "all selected commits conform") || errb.Len() != 0 {
		t.Fatalf("valid command: exit=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}

	_, wrongKey := gitfixture.NativeSSHKey(t)
	writePolicy(wrongKey)
	out.Reset()
	errb.Reset()
	code = runAt(t, root, []string{"awf", "check", "commit-policy", "HEAD"}, &out, &errb)
	if code != 1 || !strings.Contains(out.String(), "commits must be signed") || !strings.Contains(out.String(), "allowed signers:") || errb.Len() != 0 {
		t.Fatalf("violation command: exit=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}

	writePolicy(publicKey)
	out.Reset()
	errb.Reset()
	code = runAt(t, root, []string{"awf", "check", "commit-policy", "missing-target"}, &out, &errb)
	if code != 1 || !strings.Contains(out.String(), "revision-resolution") || !strings.Contains(out.String(), "refs: false") || errb.Len() != 0 {
		t.Fatalf("refusal command: exit=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}
}
