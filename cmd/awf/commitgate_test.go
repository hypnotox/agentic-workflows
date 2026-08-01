package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/commitmsg"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
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
	ctx := testContext(t)
	_ = ctx
	cases := []struct{ name, in, want string }{
		{"plain", "feat: x\n\nbody here\n", "feat: x"},
		{"leading comment", "# please enter a message\nfeat: y\n", "feat: y"},
		{"blank then comment", "  \n# c\nfix: z\n", "fix: z"},
		{"trailing spaces", "feat: t   \n", "feat: t"},
		{"crlf", "feat: w\r\n\r\nbody\r\n", "feat: w"},
		{"comment only", "# a\n# b\n", ""},
		{"scissors stops scan", "# msg\n# ------------------------ >8 ------------------------\nfeat: belowscissors\n", ""},
		{"ordinary greater-than comment", "# threshold >8\nfeat: kept\n", "feat: kept"},
		{"subject before scissors", "feat: above\n# ------------------------ >8 ------------------------\ndiff\n", "feat: above"},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := commitmsg.Clean([]byte(c.in)).Subject; got != c.want {
				t.Errorf("Clean(%q).Subject = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestIsExemptSubject(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
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
	ctx := testContext(t)
	_ = ctx
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runCommitGate(ctx, root, writeMsg(t, "feat: a clean subject\n"), nil, &out); err != nil {
		t.Fatalf("conforming subject must pass: %v (out=%q)", err, out.String())
	}
}

// invariant: adr-system/adr-lifecycle:older-format-incoming-parent-sanction (TestCheckCommitAuthorizesOlderFormatIncomingParent)
func TestCheckCommitAuthorizesOlderFormatIncomingParent(t *testing.T) {
	root := scaffoldProject(t)
	fixture := gitfixture.At(root)
	gitfixture.AddAll(t, fixture)
	base := gitfixture.Commit(t, fixture, "chore: scaffold", nil)
	v2Incoming := `---
format: current-state-v2
status: Proposed
date: 2026-01-01
---
# ADR-0190: Old format

## Context

Original context.

## Decision

1. Keep V2.

## State changes

None.

## Consequences

The format stays authored.

## Alternatives Considered

None.

## Status history

- 2026-01-01: Proposed
`
	v2Result := strings.Replace(v2Incoming, "# ADR-0190:", "# ADR-0191:", 1)
	v1Body := `---
format: current-state-v1
status: Proposed
date: 2026-01-01
---
# ADR-0189: V1 format

## Context

V1 context.

## Decision

1. Keep V1.

## State changes

None.

## Consequences

The format stays authored.

## Alternatives Considered

None.

## Status history

- 2026-01-01: Proposed
`
	legacyBody, err := os.ReadFile(filepath.Join("..", "..", "docs", "decisions", "0001-template-overlay-rendering-engine.md"))
	if err != nil {
		t.Fatal(err)
	}
	v2IncomingPath := "docs/decisions/0190-old-format.md"
	v2ResultPath := "docs/decisions/0191-old-format.md"
	v1Path := "docs/decisions/0189-v1-format.md"
	legacyPath := "docs/decisions/0001-template-overlay-rendering-engine.md"
	gitfixture.CheckoutNewBranch(t, fixture, "old-v2", base)
	v2Head := gitfixture.Commit(t, fixture, "docs: old v2", map[string]string{v2IncomingPath: v2Incoming})
	gitfixture.CheckoutNewBranch(t, fixture, "old-v1", base)
	v1Head := gitfixture.Commit(t, fixture, "docs: old v1", map[string]string{v1Path: v1Body})
	gitfixture.CheckoutNewBranch(t, fixture, "old-legacy", base)
	legacyHead := gitfixture.Commit(t, fixture, "docs: old legacy", map[string]string{legacyPath: string(legacyBody)})
	gitfixture.CheckoutNewBranch(t, fixture, "integration", base)
	gitfixture.Stage(t, fixture, map[string]string{v2ResultPath: v2Result, v1Path: v1Body, legacyPath: string(legacyBody)})
	mergeHeadPath := filepath.Join(root, ".git", "MERGE_HEAD")
	testsupport.WriteFile(t, mergeHeadPath, v2Head+"\n"+v1Head+"\n"+legacyHead+"\n")
	observed := "merge with MERGE_HEAD " + v2Head + "," + v1Head + "," + legacyHead
	refusalSuffix := "; changed index: no; changed message: no; changed merge state: no; next actions: 1. correct the message trailers 2. run git commit to finish the existing merge\n"

	indexBefore, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	mergeBefore, err := os.ReadFile(mergeHeadPath)
	if err != nil {
		t.Fatal(err)
	}
	unstampedPath := writeMsg(t, "Merge old branches\n")
	messageBefore, err := os.ReadFile(unstampedPath)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runCommitGate(testContext(t), root, unstampedPath, nil, &out); err == nil {
		t.Fatal("unstamped older-format merge succeeded")
	}
	wantMissing := "operation: " + observed + ": missing authorization version legacy for ADR-0001" + refusalSuffix
	if out.String() != wantMissing {
		t.Fatalf("unstamped refusal = %q, want %q", out.String(), wantMissing)
	}
	out.Reset()
	if err := runCommitGate(testContext(t), root, writeMsg(t, ""), nil, &out); err == nil {
		t.Fatal("empty-message older-format merge succeeded")
	}
	if out.String() != wantMissing {
		t.Fatalf("empty-message refusal = %q, want %q", out.String(), wantMissing)
	}
	indexAfter, _ := os.ReadFile(filepath.Join(root, ".git", "index"))
	mergeAfter, _ := os.ReadFile(mergeHeadPath)
	messageAfter, _ := os.ReadFile(unstampedPath)
	if !bytes.Equal(indexBefore, indexAfter) || !bytes.Equal(mergeBefore, mergeAfter) || !bytes.Equal(messageBefore, messageAfter) {
		t.Fatal("refusal changed the staged index, message, or MERGE_HEAD")
	}

	wrongVersion := "Merge old branches\n\nAWF-Allow-Version: legacy\nAWF-Allow-Reason: preserve legacy\nAWF-Allow-Version: current-state-v1\nAWF-Allow-Reason: preserve V1\nAWF-Allow-Version: current-state-v3\nAWF-Allow-Reason: wrong version for V2\n"
	out.Reset()
	if err := runCommitGate(testContext(t), root, writeMsg(t, wrongVersion), nil, &out); err == nil {
		t.Fatal("wrong-version authorization succeeded")
	}
	wantWrong := "operation: " + observed + ": missing authorization version current-state-v2 for ADR-0191" + refusalSuffix
	if out.String() != wantWrong {
		t.Fatalf("wrong-version refusal = %q, want %q", out.String(), wantWrong)
	}

	valid := "Merge old branches\n\nAWF-Allow-Version: current-state-v2\nAWF-Allow-Reason: preserve reviewed V2 history\nAWF-Allow-Version: current-state-v1\nAWF-Allow-Reason: preserve reviewed V1 history\nAWF-Allow-Version: legacy\nAWF-Allow-Reason: preserve reviewed legacy history\nAWF-Allow-Version: legacy\nAWF-Allow-Reason: redundant but harmless\n"
	out.Reset()
	if err := runCommitGate(testContext(t), root, writeMsg(t, valid), nil, &out); err != nil {
		t.Fatalf("qualified octopus merge refused: %v\n%s", err, out.String())
	}
	wantSuccess := "operation: stale merge authorization satisfied; changed index: no; changed message: no; changed merge state: no; next actions: none\n"
	if out.String() != wantSuccess {
		t.Fatalf("success outcome = %q, want %q", out.String(), wantSuccess)
	}

	evilV2 := strings.Replace(v2Result, "Original context.", "Evil merge context.", 1)
	gitfixture.Stage(t, fixture, map[string]string{v2ResultPath: evilV2})
	out.Reset()
	if err := runCommitGate(testContext(t), root, writeMsg(t, valid), nil, &out); err == nil {
		t.Fatal("evil merge edit succeeded")
	}
	wantEvil := "operation: " + observed + ": unqualified incoming-parent record ADR-0191" + refusalSuffix
	if out.String() != wantEvil {
		t.Fatalf("evil-merge refusal = %q, want %q", out.String(), wantEvil)
	}
	gitfixture.Stage(t, fixture, map[string]string{v2ResultPath: v2Result})

	unrelatedPath := "docs/decisions/0191-unrelated.md"
	gitfixture.StageRemoval(t, fixture, v2ResultPath)
	gitfixture.Stage(t, fixture, map[string]string{unrelatedPath: v2Result})
	out.Reset()
	if err := runCommitGate(testContext(t), root, writeMsg(t, valid), nil, &out); err == nil {
		t.Fatal("unrelated filename substitution succeeded")
	}
	if out.String() != wantEvil {
		t.Fatalf("filename refusal = %q, want %q", out.String(), wantEvil)
	}
	gitfixture.StageRemoval(t, fixture, unrelatedPath)
	gitfixture.Stage(t, fixture, map[string]string{v2ResultPath: v2Result})

	out.Reset()
	malformed := "Merge old branches\n\nAWF-Allow-Version: legacy\nAWF-Allow-Reason: \n"
	err = runCommitGate(testContext(t), root, writeMsg(t, malformed), nil, &out)
	var syntax *commitmsg.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("malformed error = %#v", err)
	}
	wantMalformed := "operation: " + observed + ": malformed reserved trailer at cleaned line 4: AWF-Allow-Reason must be nonempty" + refusalSuffix
	if out.String() != wantMalformed {
		t.Fatalf("malformed refusal = %q, want %q", out.String(), wantMalformed)
	}

	if err := os.Remove(mergeHeadPath); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := runCommitGate(testContext(t), root, writeMsg(t, "feat: ordinary stale import\n\nAWF-Allow-Version: legacy\nAWF-Allow-Reason: not a merge\n"), nil, &out); err == nil {
		t.Fatal("non-merge provisional import succeeded")
	}
	wantNonMerge := "operation: non-merge: provisional older-format introduction without merge parents" + refusalSuffix
	if out.String() != wantNonMerge {
		t.Fatalf("non-merge refusal = %q, want %q", out.String(), wantNonMerge)
	}
}

func TestRunCommitGateRejectsMalformedAuthorizationByIdentity(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer
	err := runCommitGate(testContext(t), root, writeMsg(t, "feat: x\n\nAWF-Allow-Version: legacy\n"), nil, &out)
	var syntax *commitmsg.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("error = %#v, want SyntaxError; output=%q", err, out.String())
	}
	if !strings.Contains(out.String(), "changed index: no; changed message: no; changed merge state: no") {
		t.Fatalf("missing non-mutation outcome: %q", out.String())
	}
	gitfixture.StageUnmerged(t, gitfixture.At(root), "conflict.md")
	out.Reset()
	err = runCommitGate(testContext(t), root, writeMsg(t, "feat: x\n"), nil, &out)
	if err == nil || errors.As(err, &syntax) {
		t.Fatalf("infrastructure error = %#v, want non-SyntaxError", err)
	}
}

func TestRunCommitGateRejectsLongSubject(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := scaffoldProject(t)
	long := "feat: " + strings.Repeat("x", 80)
	var out bytes.Buffer
	err := runCommitGate(ctx, root, writeMsg(t, long+"\n"), nil, &out)
	if err == nil {
		t.Fatal("an 80+ char subject must be rejected")
	}
	if !strings.Contains(out.String(), "chars > 72") {
		t.Errorf("expected length violation on stdout, got %q", out.String())
	}
}

func TestRunCommitGateRejectsNonConventional(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := scaffoldProject(t)
	var out bytes.Buffer
	// invariant: tooling/audit-and-snapshots:commit-gate-shared-rule (TestRunCommitGateRejectsNonConventional)
	if err := runCommitGate(ctx, root, writeMsg(t, "just some words\n"), nil, &out); err == nil {
		t.Fatal("a non-Conventional-Commits subject must be rejected")
	}
}

func TestRunCommitGateExemptAndEmptySkip(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := scaffoldProject(t)
	var out bytes.Buffer
	// Merge subject is exempt; an all-comments message has an empty subject.
	if err := runCommitGate(ctx, root, writeMsg(t, "Merge branch 'topic'\n"), nil, &out); err != nil {
		t.Errorf("merge subject must be exempt: %v", err)
	}
	if err := runCommitGate(ctx, root, writeMsg(t, "# nothing but a comment\n"), nil, &out); err != nil {
		t.Errorf("empty subject must skip: %v", err)
	}
}

func TestRunCommitGateReadsStdin(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runCommitGate(ctx, root, "", strings.NewReader("feat: from stdin\n"), &out); err != nil {
		t.Fatalf("stdin message must be read and pass: %v", err)
	}
}

func TestRunCommitGateReadError(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runCommitGate(ctx, root, filepath.Join(root, "does-not-exist"), nil, &out); err == nil {
		t.Fatal("an unreadable message path must error")
	}
}

func TestDispatchCommitGate(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := scaffoldProject(t)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "check", "commit", writeMsg(t, "feat: via dispatch\n")}, &out, &errb); code != 0 {
		t.Fatalf("dispatch check commit should accept a clean subject: code=%d err=%s", code, errb.String())
	}
	out.Reset()
	errb.Reset()
	if code := run([]string{"awf", "check", "commit", writeMsg(t, "nope not conventional\n")}, &out, &errb); code == 0 {
		t.Fatal("dispatch check commit should block a non-conforming subject")
	}
}

func TestCleanCommitLines(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	// The extraction must preserve blank lines and leave every surviving line
	// untrimmed, which is what keeps cleanCommitSubject's existing behaviour and
	// makes body line numbers match what the author sees.
	got := strings.Split(commitmsg.Clean([]byte("feat: x  \r\n\n# a comment\nbody\n# ------------------------ >8 ------------------------\ndiff\n")).Text, "\n")
	want := []string{"feat: x  ", "", "body"}
	if len(got) != len(want) {
		t.Fatalf("Clean lines = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// cite is the working-memory prefix followed by a concrete name, built rather
// than written out so this file does not carry the shape the gate rejects.
func cite() string { return dir + "concrete-effort/memory.md" }

// invariant: tooling/quality-gates:memory-citation-gate (TestRunCommitGateCitationScan)
func TestRunCommitGateCitationScan(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	t.Run("knob off accepts a citing body", func(t *testing.T) {
		root := scaffoldProject(t)
		var out bytes.Buffer
		if err := runCommitGate(ctx, root, writeMsg(t, "feat: a\n\nsee "+cite()+"\n"), nil, &out); err != nil {
			t.Fatalf("knob off must not scan: %v (out=%q)", err, out.String())
		}
	})
	t.Run("knob on accepts a clean body", func(t *testing.T) {
		root := citingProject(t)
		var out bytes.Buffer
		if err := runCommitGate(ctx, root, writeMsg(t, "feat: a\n\nthe file lives under "+dir+"\n"), nil, &out); err != nil {
			t.Fatalf("clean body must pass: %v (out=%q)", err, out.String())
		}
	})
	t.Run("knob on rejects a citing body", func(t *testing.T) {
		root := citingProject(t)
		var out bytes.Buffer
		err := runCommitGate(ctx, root, writeMsg(t, "feat: a\n\nsee "+cite()+"\n"), nil, &out)
		if err == nil || !strings.Contains(err.Error(), "effort-owned memory file") {
			t.Fatalf("citing body must be rejected: %v", err)
		}
		if !strings.Contains(out.String(), "line 3") || !strings.Contains(out.String(), "concrete-effort/memory.md") {
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
		err := runCommitGate(ctx, root, writeMsg(t, "feat: a\n\nsee "+cite()+"\n"), nil, &out)
		if err == nil || !strings.Contains(err.Error(), "effort-owned memory file") {
			t.Errorf("a configured exemption must not reach the commit-message scan: %v", err)
		}
	})
	t.Run("a comment-only citation is accepted", func(t *testing.T) {
		// git discards the comment, so it is never recorded.
		root := citingProject(t)
		var out bytes.Buffer
		if err := runCommitGate(ctx, root, writeMsg(t, "feat: a\n\n# see "+cite()+"\n"), nil, &out); err != nil {
			t.Fatalf("a citation git discards must not block: %v", err)
		}
	})
	t.Run("a citation below the scissors line is accepted", func(t *testing.T) {
		// `git commit -v` appends the staged diff, which may legitimately touch a
		// file whose fixtures name a working-memory file.
		root := citingProject(t)
		msg := "feat: a\n\nbody\n# ------------------------ >8 ------------------------\ndiff --git\n+" + cite() + "\n"
		var out bytes.Buffer
		if err := runCommitGate(ctx, root, writeMsg(t, msg), nil, &out); err != nil {
			t.Fatalf("a verbose diff must not block: %v", err)
		}
	})
}

func TestRunCommitGateCitationScanIgnoresSubjectExemption(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := citingProject(t)
	// The exemption exists because git writes the subject; a person may still
	// edit the body, so the citation scan applies to a merge and an autosquash.
	for _, subject := range []string{"Merge branch 'topic'", "fixup! feat: a"} {
		var out bytes.Buffer
		if err := runCommitGate(ctx, root, writeMsg(t, subject+"\n\nsee "+cite()+"\n"), nil, &out); err == nil {
			t.Errorf("%q: a citing body must be rejected under an exempt subject", subject)
		}
	}
	// The exemption still governs what it was for: a merge subject is not a
	// Conventional Commits violation.
	var out bytes.Buffer
	if err := runCommitGate(ctx, root, writeMsg(t, "Merge branch 'topic'\n\nclean body\n"), nil, &out); err != nil {
		t.Errorf("merge subject with a clean body must stay exempt: %v", err)
	}
}

func TestRunCommitGateProjectOpenError(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	// A directory with no .awf config: a conforming-looking but non-exempt subject
	// proceeds to project.Open, which fails.
	bare := t.TempDir()
	var out bytes.Buffer
	if err := runCommitGate(ctx, bare, writeMsg(t, "feat: needs a project\n"), nil, &out); err == nil {
		t.Fatal("check commit outside an awf project must error")
	}
	// A git-generated subject reaches project.Open too, because the citation scan
	// sits above the subject exemption (ADR-0158 Decision 6). This is the accepted
	// behavioural cost of that ordering: the call returned nil here before.
	if err := runCommitGate(ctx, bare, writeMsg(t, "Merge branch 'x'\n"), nil, &out); err == nil {
		t.Fatal("a merge subject outside an awf project must error too")
	}
}
