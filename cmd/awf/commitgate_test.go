package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func writeMsg(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// citingProject is a scaffolded project with the citation knob on; the gate
// reads the worktree config through project.Open, so rewriting it after the
// scaffold sync is enough.
func citingProject(t *testing.T) string {
	t.Helper()
	root := scaffoldProject(t)
	testsupport.WriteAwfConfig(t, root, minimalYAML+"memoryCite:\n  enabled: true\n")
	return root
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
	// invariant: tooling/audit-and-snapshots:commit-gate-shared-rule
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
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
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

func TestCleanCommitLines(t *testing.T) {
	// The extraction must preserve blank lines and leave every surviving line
	// untrimmed, which is what keeps cleanCommitSubject's existing behaviour and
	// makes body line numbers match what the author sees.
	got := cleanCommitLines("feat: x  \r\n\n# a comment\nbody\n# ---- >8 ----\ndiff\n")
	want := []string{"feat: x  ", "", "body"}
	if len(got) != len(want) {
		t.Fatalf("cleanCommitLines = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// cite is the working-memory prefix followed by a concrete name, built rather
// than written out so this file does not carry the shape the gate rejects.
func cite() string { return dir + "effort.md" }

// invariant: tooling/quality-gates:memory-citation-gate
func TestRunCommitGateCitationScan(t *testing.T) {
	t.Run("knob off accepts a citing body", func(t *testing.T) {
		root := scaffoldProject(t)
		var out bytes.Buffer
		if err := runCommitGate(root, writeMsg(t, "feat: a\n\nsee "+cite()+"\n"), nil, &out); err != nil {
			t.Fatalf("knob off must not scan: %v (out=%q)", err, out.String())
		}
	})
	t.Run("knob on accepts a clean body", func(t *testing.T) {
		root := citingProject(t)
		var out bytes.Buffer
		if err := runCommitGate(root, writeMsg(t, "feat: a\n\nthe file lives under "+dir+"\n"), nil, &out); err != nil {
			t.Fatalf("clean body must pass: %v (out=%q)", err, out.String())
		}
	})
	t.Run("knob on rejects a citing body", func(t *testing.T) {
		root := citingProject(t)
		var out bytes.Buffer
		err := runCommitGate(root, writeMsg(t, "feat: a\n\nsee "+cite()+"\n"), nil, &out)
		if err == nil || !strings.Contains(err.Error(), "working-memory file") {
			t.Fatalf("citing body must be rejected: %v", err)
		}
		if !strings.Contains(out.String(), "line 3") || !strings.Contains(out.String(), "effort.md") {
			t.Errorf("diagnostic must name the reference: %q", out.String())
		}
	})
	t.Run("an exemption does not suppress the message scan", func(t *testing.T) {
		// An exemption is keyed by path and a commit message has none, so
		// configuring one - even under the synthetic label the diagnostic uses -
		// must leave the scan blocking.
		root := scaffoldProject(t)
		testsupport.WriteAwfConfig(t, root,
			minimalYAML+"memoryCite:\n  enabled: true\n  exemptions:\n    - path: commit message\n")
		var out bytes.Buffer
		// Assert the citation error specifically: a bare non-nil check would also
		// pass if the config never parsed, which is the opposite of the point.
		err := runCommitGate(root, writeMsg(t, "feat: a\n\nsee "+cite()+"\n"), nil, &out)
		if err == nil || !strings.Contains(err.Error(), "working-memory file") {
			t.Errorf("a configured exemption must not reach the commit-message scan: %v", err)
		}
	})
	t.Run("a comment-only citation is accepted", func(t *testing.T) {
		// git discards the comment, so it is never recorded.
		root := citingProject(t)
		var out bytes.Buffer
		if err := runCommitGate(root, writeMsg(t, "feat: a\n\n# see "+cite()+"\n"), nil, &out); err != nil {
			t.Fatalf("a citation git discards must not block: %v", err)
		}
	})
	t.Run("a citation below the scissors line is accepted", func(t *testing.T) {
		// `git commit -v` appends the staged diff, which may legitimately touch a
		// file whose fixtures name a working-memory file.
		root := citingProject(t)
		msg := "feat: a\n\nbody\n# ------------------------ >8 ------------------------\ndiff --git\n+" + cite() + "\n"
		var out bytes.Buffer
		if err := runCommitGate(root, writeMsg(t, msg), nil, &out); err != nil {
			t.Fatalf("a verbose diff must not block: %v", err)
		}
	})
}

func TestRunCommitGateCitationScanIgnoresSubjectExemption(t *testing.T) {
	root := citingProject(t)
	// The exemption exists because git writes the subject; a person may still
	// edit the body, so the citation scan applies to a merge and an autosquash.
	for _, subject := range []string{"Merge branch 'topic'", "fixup! feat: a"} {
		var out bytes.Buffer
		if err := runCommitGate(root, writeMsg(t, subject+"\n\nsee "+cite()+"\n"), nil, &out); err == nil {
			t.Errorf("%q: a citing body must be rejected under an exempt subject", subject)
		}
	}
	// The exemption still governs what it was for: a merge subject is not a
	// Conventional Commits violation.
	var out bytes.Buffer
	if err := runCommitGate(root, writeMsg(t, "Merge branch 'topic'\n\nclean body\n"), nil, &out); err != nil {
		t.Errorf("merge subject with a clean body must stay exempt: %v", err)
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
	// A git-generated subject reaches project.Open too, because the citation scan
	// sits above the subject exemption (ADR-0158 Decision 6). This is the accepted
	// behavioural cost of that ordering: the call returned nil here before.
	if err := runCommitGate(bare, writeMsg(t, "Merge branch 'x'\n"), nil, &out); err == nil {
		t.Fatal("a merge subject outside an awf project must error too")
	}
}
