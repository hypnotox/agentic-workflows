package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeMsg(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCleanCommitSubject(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"plain", "feat: x\n\nbody here\n", "feat: x"},
		{"leading comment", "# please enter a message\nfeat: y\n", "feat: y"},
		{"blank then comment", "  \n# c\nfix: z\n", "fix: z"},
		{"trailing spaces", "feat: t   \n", "feat: t"},
		{"crlf", "feat: w\r\n\r\nbody\r\n", "feat: w"},
		{"comment only", "# a\n# b\n", ""},
		{"scissors stops scan", "# msg\n# ------------------------ >8 ------------------------\nfeat: belowscissors\n", ""},
		{"subject before scissors", "feat: above\n# ------ >8 ------\ndiff\n", "feat: above"},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := cleanCommitSubject(c.in); got != c.want {
				t.Errorf("cleanCommitSubject(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestIsExemptSubject(t *testing.T) {
	exempt := []string{"Merge branch 'x'", "fixup! feat: a", "squash! fix: b", "amend! docs: c"}
	for _, s := range exempt {
		if !isExemptSubject(s) {
			t.Errorf("expected %q exempt", s)
		}
	}
	notExempt := []string{"feat: x", "Merged the configs", "fix: merge handling"}
	for _, s := range notExempt {
		if isExemptSubject(s) {
			t.Errorf("expected %q not exempt", s)
		}
	}
}

func TestRunCommitGateAccepts(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runCommitGate(root, writeMsg(t, "feat: a clean subject\n"), nil, &out); err != nil {
		t.Fatalf("conforming subject must pass: %v (out=%q)", err, out.String())
	}
}

func TestRunCommitGateRejectsLongSubject(t *testing.T) {
	root := scaffoldProject(t)
	long := "feat: " + strings.Repeat("x", 80)
	var out bytes.Buffer
	err := runCommitGate(root, writeMsg(t, long+"\n"), nil, &out)
	if err == nil {
		t.Fatal("an 80+ char subject must be rejected")
	}
	if !strings.Contains(out.String(), "chars > 72") {
		t.Errorf("expected length violation on stdout, got %q", out.String())
	}
}

func TestRunCommitGateRejectsNonConventional(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runCommitGate(root, writeMsg(t, "just some words\n"), nil, &out); err == nil {
		t.Fatal("a non-Conventional-Commits subject must be rejected")
	}
}

func TestRunCommitGateExemptAndEmptySkip(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer
	// Merge subject is exempt; an all-comments message has an empty subject.
	if err := runCommitGate(root, writeMsg(t, "Merge branch 'topic'\n"), nil, &out); err != nil {
		t.Errorf("merge subject must be exempt: %v", err)
	}
	if err := runCommitGate(root, writeMsg(t, "# nothing but a comment\n"), nil, &out); err != nil {
		t.Errorf("empty subject must skip: %v", err)
	}
}

func TestRunCommitGateReadsStdin(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runCommitGate(root, "", strings.NewReader("feat: from stdin\n"), &out); err != nil {
		t.Fatalf("stdin message must be read and pass: %v", err)
	}
}

func TestRunCommitGateReadError(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runCommitGate(root, filepath.Join(root, "does-not-exist"), nil, &out); err == nil {
		t.Fatal("an unreadable message path must error")
	}
}

func TestDispatchCommitGate(t *testing.T) {
	root := scaffoldProject(t)
	swapGetwd(t, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "commit-gate", writeMsg(t, "feat: via dispatch\n")}, &out, &errb); code != 0 {
		t.Fatalf("dispatch commit-gate should accept a clean subject: code=%d err=%s", code, errb.String())
	}
	out.Reset()
	errb.Reset()
	if code := run([]string{"awf", "commit-gate", writeMsg(t, "nope not conventional\n")}, &out, &errb); code == 0 {
		t.Fatal("dispatch commit-gate should block a non-conforming subject")
	}
}

func TestRunCommitGateProjectOpenError(t *testing.T) {
	// A directory with no .awf config: a conforming-looking but non-exempt subject
	// proceeds to project.Open, which fails.
	bare := t.TempDir()
	var out bytes.Buffer
	if err := runCommitGate(bare, writeMsg(t, "feat: needs a project\n"), nil, &out); err == nil {
		t.Fatal("commit-gate outside an awf project must error")
	}
}
