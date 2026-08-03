package main

import (
	"bytes"
	"strings"
	"testing"
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
