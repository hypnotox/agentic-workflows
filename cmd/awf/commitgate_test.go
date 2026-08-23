package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/commitgateop"
	"github.com/hypnotox/agentic-workflows/internal/commitmsg"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestCommitAuthorizationDiagnostic(t *testing.T) {
	result := currentstatecoord.CommitAuthorizationResult{Category: "operation", Condition: "non-merge", ChangedIndex: true, ChangedMessage: true, ChangedMergeState: true, NextActions: []string{"correct the message trailers", "run git commit"}}
	diagnostic, err := commitgateop.AuthorizationDiagnostic(result)
	if err != nil {
		t.Fatal(err)
	}
	document, err := diagnostic.Document()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := presentation.Render(&out, document); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"index: yes", "message: yes", "merge state: yes", "step 1: correct the message trailers", "step 2: run git commit"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("diagnostic missing %q:\n%s", want, out.String())
		}
	}
}

func TestCommitAuthorizationDiagnosticRejectsInvalidAction(t *testing.T) {
	result := currentstatecoord.CommitAuthorizationResult{Category: "operation", Condition: "refused", NextActions: []string{""}}
	if _, err := commitgateop.AuthorizationDiagnostic(result); err == nil {
		t.Fatal("invalid action accepted")
	}
}

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
	testsupport.WriteAwfConfig(t, root, minimalYAML+"")
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
		if !commitgateop.IsExemptSubject(s) {
			t.Errorf("expected %q exempt", s)
		}
	}
	notExempt := []string{"feat: x", "Merged the configs", "fix: merge handling"}
	for _, s := range notExempt {
		if commitgateop.IsExemptSubject(s) {
			t.Errorf("expected %q not exempt", s)
		}
	}
}

func TestRunCommitGateCoreSkipsFullAuthorization(t *testing.T) {
	dependencies := defaultCommitGateDependencies()
	dependencies.openProject = func(context.Context, string) (*config.Config, *awfgit.Repo, error) {
		return &config.Config{Profile: catalog.ProfileCore}, nil, nil
	}
	called := false
	dependencies.authorize = func(context.Context, string, *awfgit.Repo, commitmsg.Message) (currentstatecoord.CommitAuthorizationResult, error) {
		called = true
		return currentstatecoord.CommitAuthorizationResult{}, errors.New("Full authorization reached")
	}
	var out bytes.Buffer
	err := runCommitGateWithDependencies(testContext(t), t.TempDir(), writeMsg(t, "feat: Core commit\n\nAWF-Allow-Version: legacy\n"), nil, &out, dependencies)
	if err != nil {
		t.Fatalf("Core commit gate consulted Full authorization: %v", err)
	}
	if called || out.Len() != 0 {
		t.Fatalf("Core authorization called=%v output=%q", called, out.String())
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
	want := "condition: stale merge authorization satisfied\nstate: operation\n\ndiagnostic:\n  changed:\n    index: no\n    message: no\n    merge state: no\n"
	if out.String() != want {
		t.Fatalf("success output = %q, want %q", out.String(), want)
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
	wantMissing := "condition: " + observed + ": missing authorization version legacy for ADR-0001"
	if !strings.Contains(out.String(), wantMissing) || !strings.Contains(out.String(), "step 1: correct the message trailers") {
		t.Fatalf("unstamped refusal = %q", out.String())
	}
	out.Reset()
	if err := runCommitGate(testContext(t), root, writeMsg(t, ""), nil, &out); err == nil {
		t.Fatal("empty-message older-format merge succeeded")
	}
	if !strings.Contains(out.String(), wantMissing) {
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
	wantWrong := "condition: " + observed + ": missing authorization version current-state-v2 for ADR-0191"
	if !strings.Contains(out.String(), wantWrong) {
		t.Fatalf("wrong-version refusal = %q", out.String())
	}

	valid := "Merge old branches\n\nAWF-Allow-Version: current-state-v2\nAWF-Allow-Reason: preserve reviewed V2 history\nAWF-Allow-Version: current-state-v1\nAWF-Allow-Reason: preserve reviewed V1 history\nAWF-Allow-Version: legacy\nAWF-Allow-Reason: preserve reviewed legacy history\nAWF-Allow-Version: legacy\nAWF-Allow-Reason: redundant but harmless\n"
	out.Reset()
	if err := runCommitGate(testContext(t), root, writeMsg(t, valid), nil, &out); err != nil {
		t.Fatalf("qualified octopus merge refused: %v\n%s", err, out.String())
	}
	wantSuccess := "condition: stale merge authorization satisfied\nstate: operation"
	if !strings.Contains(out.String(), wantSuccess) {
		t.Fatalf("success outcome = %q, want %q", out.String(), wantSuccess)
	}

	evilV2 := strings.Replace(v2Result, "Original context.", "Evil merge context.", 1)
	gitfixture.Stage(t, fixture, map[string]string{v2ResultPath: evilV2})
	out.Reset()
	if err := runCommitGate(testContext(t), root, writeMsg(t, valid), nil, &out); err == nil {
		t.Fatal("evil merge edit succeeded")
	}
	wantEvil := "condition: " + observed + ": unqualified incoming-parent record ADR-0191"
	if !strings.Contains(out.String(), wantEvil) {
		t.Fatalf("evil-merge refusal = %q", out.String())
	}
	gitfixture.Stage(t, fixture, map[string]string{v2ResultPath: v2Result})

	unrelatedPath := "docs/decisions/0191-unrelated.md"
	gitfixture.StageRemoval(t, fixture, v2ResultPath)
	gitfixture.Stage(t, fixture, map[string]string{unrelatedPath: v2Result})
	out.Reset()
	if err := runCommitGate(testContext(t), root, writeMsg(t, valid), nil, &out); err == nil {
		t.Fatal("unrelated filename substitution succeeded")
	}
	if !strings.Contains(out.String(), wantEvil) {
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
	wantMalformed := "condition: " + observed + ": malformed reserved trailer at cleaned line 4: AWF-Allow-Reason must be nonempty"
	if !strings.Contains(out.String(), wantMalformed) {
		t.Fatalf("malformed refusal = %q", out.String())
	}

	if err := os.Remove(mergeHeadPath); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := runCommitGate(testContext(t), root, writeMsg(t, "feat: ordinary stale import\n\nAWF-Allow-Version: legacy\nAWF-Allow-Reason: not a merge\n"), nil, &out); err == nil {
		t.Fatal("non-merge provisional import succeeded")
	}
	wantNonMerge := "condition: non-merge: provisional older-format introduction without merge parents"
	if !strings.Contains(out.String(), wantNonMerge) {
		t.Fatalf("non-merge refusal = %q", out.String())
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
	want := "condition: non-merge: malformed reserved trailer at cleaned line 3: AWF-Allow-Version must be immediately followed by AWF-Allow-Reason\nstate: operation\n\ndiagnostic:\n  changed:\n    index: no\n    message: no\n    merge state: no\n  steps:\n    step 1: correct the message trailers\n    step 2: run git commit to finish the existing merge\n"
	if out.String() != want {
		t.Fatalf("malformed authorization output = %q, want %q", out.String(), want)
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
	want := "check staged commit:\n  errors:\n    subject is not Conventional Commits (type(scope)?: subject)\n"
	if out.String() != want {
		t.Fatalf("refusal output = %q, want %q", out.String(), want)
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
	repo := gitfixture.At(root)
	gitfixture.AddAll(t, repo)
	gitfixture.Commit(t, repo, "fixture", nil)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "check", "staged", "commit", writeMsg(t, "feat: via dispatch\n")}, &out, &errb); code != 0 {
		t.Fatalf("dispatch check staged commit should accept a clean subject: code=%d err=%s", code, errb.String())
	}
	out.Reset()
	errb.Reset()
	if code := run([]string{"awf", "check", "staged", "commit", writeMsg(t, "nope not conventional\n")}, &out, &errb); code != 1 {
		t.Fatalf("dispatch refusal exit = %d, want 1", code)
	}
	if out.String() != "check staged commit:\n  errors:\n    subject is not Conventional Commits (type(scope)?: subject)\n" || errb.String() != "condition: awf: check staged commit: rejected \"nope not conventional\"\n" {
		t.Fatalf("dispatch refusal streams stdout=%q stderr=%q", out.String(), errb.String())
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
	t.Run("unconditional gate rejects a citing body", func(t *testing.T) {
		root := scaffoldProject(t)
		var out bytes.Buffer
		if err := runCommitGate(ctx, root, writeMsg(t, "feat: a\n\nsee "+cite()+"\n"), nil, &out); err == nil || !strings.Contains(err.Error(), "effort-owned memory file") {
			t.Fatalf("unconditional gate must scan: %v (out=%q)", err, out.String())
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
			minimalYAML+"memoryCite:\n  exemptions:\n    - path: commit-message\n")
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

func TestRunCommitGateMechanismFailuresPreserveIdentity(t *testing.T) {
	root := scaffoldProject(t)
	failure := errors.New("injected failure")
	message := func() string { return writeMsg(t, "feat: seam\n") }
	assertFailure := func(t *testing.T, dependencies commitGateDependencies, messagePath string, stdin io.Reader) {
		t.Helper()
		err := runCommitGateWithDependencies(testContext(t), root, messagePath, stdin, &bytes.Buffer{}, dependencies)
		if !errors.Is(err, failure) {
			t.Fatalf("error = %v, want identity %v", err, failure)
		}
	}
	authorizationResult := func() commitGateDependencies {
		dependencies := defaultCommitGateDependencies()
		dependencies.authorize = func(context.Context, string, *awfgit.Repo, commitmsg.Message) (currentstatecoord.CommitAuthorizationResult, error) {
			return currentstatecoord.CommitAuthorizationResult{Category: "operation", Condition: "refused"}, nil
		}
		return dependencies
	}
	t.Run("message file", func(t *testing.T) {
		dependencies := defaultCommitGateDependencies()
		dependencies.readFile = func(string) ([]byte, error) { return nil, failure }
		assertFailure(t, dependencies, message(), nil)
	})
	t.Run("message stdin", func(t *testing.T) {
		dependencies := defaultCommitGateDependencies()
		dependencies.readStdin = func(io.Reader) ([]byte, error) { return nil, failure }
		assertFailure(t, dependencies, "", strings.NewReader("feat: seam\n"))
	})
	t.Run("config loading", func(t *testing.T) {
		dependencies := defaultCommitGateDependencies()
		dependencies.openProject = func(context.Context, string) (*config.Config, *awfgit.Repo, error) { return nil, nil, failure }
		assertFailure(t, dependencies, message(), nil)
	})
	t.Run("policy and staged transition loading", func(t *testing.T) {
		dependencies := defaultCommitGateDependencies()
		dependencies.authorize = func(context.Context, string, *awfgit.Repo, commitmsg.Message) (currentstatecoord.CommitAuthorizationResult, error) {
			return currentstatecoord.CommitAuthorizationResult{}, failure
		}
		assertFailure(t, dependencies, message(), nil)
	})
	t.Run("diagnostic construction", func(t *testing.T) {
		dependencies := authorizationResult()
		dependencies.diagnostic = func(currentstatecoord.CommitAuthorizationResult) (presentation.Diagnostic, error) {
			return presentation.Diagnostic{}, failure
		}
		assertFailure(t, dependencies, message(), nil)
	})
	t.Run("diagnostic document", func(t *testing.T) {
		dependencies := authorizationResult()
		dependencies.diagnosticDocument = func(presentation.Diagnostic) (presentation.Document, error) { return presentation.Document{}, failure }
		assertFailure(t, dependencies, message(), nil)
	})
	t.Run("diagnostic render", func(t *testing.T) {
		dependencies := authorizationResult()
		dependencies.render = func(io.Writer, presentation.Document) error { return failure }
		assertFailure(t, dependencies, message(), nil)
	})
	t.Run("conventional report render", func(t *testing.T) {
		dependencies := defaultCommitGateDependencies()
		dependencies.render = func(io.Writer, presentation.Document) error { return failure }
		assertFailure(t, dependencies, writeMsg(t, "not conventional\n"), nil)
	})
	t.Run("citation report render", func(t *testing.T) {
		dependencies := defaultCommitGateDependencies()
		dependencies.render = func(io.Writer, presentation.Document) error { return failure }
		citationRoot := citingProject(t)
		err := runCommitGateWithDependencies(testContext(t), citationRoot, writeMsg(t, "feat: cite\n\nsee "+cite()+"\n"), nil, &bytes.Buffer{}, dependencies)
		if !errors.Is(err, failure) {
			t.Fatalf("error = %v, want identity %v", err, failure)
		}
	})
}

func TestRunCommitGateProjectOpenError(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	// A directory with no .awf config: a conforming-looking but non-exempt subject
	// proceeds to project.Open, which fails.
	bare := t.TempDir()
	var out bytes.Buffer
	if err := runCommitGate(ctx, bare, writeMsg(t, "feat: needs a project\n"), nil, &out); err == nil {
		t.Fatal("check staged commit outside an awf project must error")
	}
	// A git-generated subject reaches project.Open too, because the citation scan
	// sits above the subject exemption (ADR-0158 Decision 6). This is the accepted
	// behavioural cost of that ordering: the call returned nil here before.
	if err := runCommitGate(ctx, bare, writeMsg(t, "Merge branch 'x'\n"), nil, &out); err == nil {
		t.Fatal("a merge subject outside an awf project must error too")
	}
}
