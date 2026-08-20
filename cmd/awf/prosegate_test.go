package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// proseGateRepo writes an .awf/config.yaml with the given proseGate block, git-
// inits the root, and stages the named files (content keyed by path). It returns
// the root. A staged file is in the index, which is what IndexPaths reads.
func proseGateRepo(t *testing.T, proseGateYAML string, stage map[string]string) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nintegrationBranch: main\n"+proseGateYAML)
	repo := gitfixture.InitRepoAt(t, root)
	gitfixture.Add(t, repo, ".awf/config.yaml")
	gitfixture.Stage(t, repo, stage)
	return root
}

func TestProseGateRequiresProjectConfig(t *testing.T) {
	if err := runProseGate(testContext(t), t.TempDir(), io.Discard); err == nil {
		t.Error("no .awf: want a config-load error, got nil")
	}
}

func TestProseGateRefusesMissingOrInvalidWorkingConfig(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := gitfixture.InitRepo(t).Root()
	if err := runProseGate(ctx, root, io.Discard); err == nil || !strings.Contains(err.Error(), "not an awf project") {
		t.Fatalf("missing working config: %v", err)
	}

	root = proseGateRepo(t, "proseGate: [\n", nil)
	if err := runProseGate(ctx, root, io.Discard); err == nil || !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("invalid working config: %v", err)
	}
}

func TestProseGateBadExemptionCodepoint(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := proseGateRepo(t,
		"proseGate:\n  exemptions:\n    - path: x.md\n      codepoint: not-a-codepoint\n",
		map[string]string{"x.md": "clean\n"})
	err := runProseGate(ctx, root, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "exemption for x.md") {
		t.Fatalf("bad codepoint: want an exemption error, got %v", err)
	}
}

func TestProseGateClean(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := proseGateRepo(t, "",
		map[string]string{"a.md": "plain ascii\n"})
	var out strings.Builder
	if err := runProseGate(ctx, root, &out); err != nil {
		t.Fatalf("clean: want nil, got %v", err)
	}
	if out.String() != completedCheckReport {
		t.Errorf("clean: output %q", out.String())
	}
}

func TestProseGateSilentlySkipsBinaries(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := proseGateRepo(t, "", map[string]string{
		"z.bin":    "\xff",
		"a.bin":    "\xfe",
		"clean.md": "plain ascii\n",
	})
	var out strings.Builder
	if err := runProseGate(ctx, root, &out); err != nil {
		t.Fatalf("binary exclusions must not fail an otherwise-clean command: %v", err)
	}
	if out.String() != completedCheckReport {
		t.Errorf("binary exclusions must be silent: %q", out.String())
	}
}

func TestProseGateValidExemptionPermits(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	// A file whose only banned rune is exempted scans clean, exercising the
	// exemption-parse-and-append path.
	root := proseGateRepo(t,
		"proseGate:\n  exemptions:\n    - path: depict.md\n      codepoint: U+2013\n",
		map[string]string{"depict.md": "the en dash \u2013 is written about here\n"})
	var out strings.Builder
	if err := runProseGate(ctx, root, &out); err != nil {
		t.Fatalf("valid exemption: want nil, got %v", err)
	}
	if out.String() != completedCheckReport {
		t.Errorf("valid exemption: output %q", out.String())
	}
}

func TestProseGateFindings(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := proseGateRepo(t, "",
		map[string]string{"a.md": "three \u2014 em \u2014 dashes \u2014 here\n"})
	var out strings.Builder
	err := runProseGate(ctx, root, &out)
	if err == nil || !strings.Contains(err.Error(), "punctuation restraint") {
		t.Fatalf("findings: want a non-nil error, got %v", err)
	}
	if !strings.Contains(out.String(), "a.md") || !strings.Contains(out.String(), "em-dash") {
		t.Errorf("findings: output %q", out.String())
	}
}

func TestProseGateUsesStagedBytesWhenWorktreeDiffers(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	t.Run("banned content cleaned without restaging remains a finding", func(t *testing.T) {
		root := proseGateRepo(t, "", map[string]string{"a.md": "staged \u2013\n"})
		if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("worktree clean\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := runProseGate(ctx, root, io.Discard); err == nil {
			t.Fatal("staged banned content must fail even when the worktree copy is clean")
		}
	})
	t.Run("staged file missing from worktree still scans", func(t *testing.T) {
		root := proseGateRepo(t, "", map[string]string{"vanish.md": "clean\n"})
		if err := os.Remove(filepath.Join(root, "vanish.md")); err != nil {
			t.Fatal(err)
		}
		if err := runProseGate(ctx, root, io.Discard); err != nil {
			t.Fatalf("staged clean file: %v", err)
		}
	})
}

func TestProseGateUsesWorkingConfigExemption(t *testing.T) {
	ctx := testContext(t)
	root := proseGateRepo(t, "", map[string]string{"depict.md": "staged \u2013\n"})
	testsupport.WriteAwfConfig(t, root, "prefix: example\nintegrationBranch: main\nproseGate:\n  exemptions:\n    - path: depict.md\n      codepoint: U+2013\n")
	if err := runProseGate(ctx, root, io.Discard); err != nil {
		t.Fatalf("working-config exemption must control the staged corpus: %v", err)
	}
}

func TestProseGateSkipsStagedGitlink(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := proseGateRepo(t, "", map[string]string{"a.md": "clean\n"})
	gitfixture.StageGitlink(t, gitfixture.At(root), "submodule")
	if err := runProseGate(ctx, root, io.Discard); err != nil {
		t.Fatalf("gitlink must not block regular staged files: %v", err)
	}
}

func TestProseGateDispatch(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	// Drive the command through run() so the dispatch handler closure is
	// exercised, not just runProseGate directly.
	root := proseGateRepo(t, "",
		map[string]string{"a.md": "plain ascii\n"})
	if err := initializeProject(ctx, root, io.Discard); err != nil {
		t.Fatal(err)
	}
	gitfixture.AddAll(t, gitfixture.At(root))
	gitfixture.Commit(t, gitfixture.At(root), "fixture", nil)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb strings.Builder
	if code := run([]string{"awf", "check", "repo", "prose"}, &out, &errb); code != 0 {
		t.Fatalf("check repo prose exited %d: %s", code, errb.String())
	}
	if out.String() != completedCheckReport {
		t.Errorf("dispatch: output %q", out.String())
	}
}

// invariant: tooling/quality-gates:prose-gate-refuses-without-git (TestProseGateRefusesOutsideAGitRepo)
func TestProseGateRefusesOutsideAGitRepo(t *testing.T) {
	ctx := testContext(t)
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nintegrationBranch: main\n")
	err := runProseGate(ctx, root, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "cannot read staged files") {
		t.Fatalf("unconditional gate outside git: want a refusal naming the enumeration failure, got %v", err)
	}
}
