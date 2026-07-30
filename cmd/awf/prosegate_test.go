package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	indexformat "github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// proseGateRepo writes an .awf/config.yaml with the given proseGate block, git-
// inits the root, and stages the named files (content keyed by path). It returns
// the root. A staged file is in the index, which is what IndexPaths reads.
func proseGateRepo(t *testing.T, proseGateYAML string, stage map[string]string) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nskills: []\nagents: []\n"+proseGateYAML)
	repo, err := git.PlainInit(root, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add(".awf/config.yaml"); err != nil {
		t.Fatal(err)
	}
	for name, content := range stage {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := wt.Add(name); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestProseGateKnobOff(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	// No .awf at all: config.Load fails.
	if err := runProseGate(ctx, t.TempDir(), io.Discard); err == nil {
		t.Error("no .awf: want a config-load error, got nil")
	}
	// Knob absent, and knob explicitly false: both no-op and return nil.
	for _, y := range []string{"", "proseGate:\n  enabled: false\n"} {
		root := proseGateRepo(t, y, nil)
		if err := runProseGate(ctx, root, io.Discard); err != nil {
			t.Errorf("knob off (%q): want nil, got %v", y, err)
		}
	}
}

func TestProseGateRefusesMissingOrInvalidStagedConfig(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := t.TempDir()
	if _, err := git.PlainInit(root, false); err != nil {
		t.Fatal(err)
	}
	if err := runProseGate(ctx, root, io.Discard); err == nil || !strings.Contains(err.Error(), "staged snapshot has no") {
		t.Fatalf("missing staged config: %v", err)
	}

	root = proseGateRepo(t, "proseGate: [\n", nil)
	if err := runProseGate(ctx, root, io.Discard); err == nil || !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("invalid staged config: %v", err)
	}
}

func TestProseGateBadExemptionCodepoint(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := proseGateRepo(t,
		"proseGate:\n  enabled: true\n  exemptions:\n    - path: x.md\n      codepoint: not-a-codepoint\n",
		map[string]string{"x.md": "clean\n"})
	err := runProseGate(ctx, root, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "exemption for x.md") {
		t.Fatalf("bad codepoint: want an exemption error, got %v", err)
	}
}

func TestProseGateClean(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := proseGateRepo(t, "proseGate:\n  enabled: true\n",
		map[string]string{"a.md": "plain ascii\n"})
	var out strings.Builder
	if err := runProseGate(ctx, root, &out); err != nil {
		t.Fatalf("clean: want nil, got %v", err)
	}
	if !strings.Contains(out.String(), "check prose: clean") {
		t.Errorf("clean: output %q", out.String())
	}
}

func TestProseGateReportsSkippedBinaries(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := proseGateRepo(t, "proseGate:\n  enabled: true\n", map[string]string{
		"z.bin":    "\xff",
		"a.bin":    "\xfe",
		"clean.md": "plain ascii\n",
	})
	var out strings.Builder
	if err := runProseGate(ctx, root, &out); err != nil {
		t.Fatalf("binary exclusions must not fail an otherwise-clean command: %v", err)
	}
	text := out.String()
	first, second := strings.Index(text, "skipped binary: a.bin"), strings.Index(text, "skipped binary: z.bin")
	if first < 0 || second < 0 || first > second {
		t.Errorf("skipped binary paths must be printed in sorted order: %q", text)
	}
	if !strings.Contains(text, "check prose: clean") {
		t.Errorf("clean output missing after binary reports: %q", text)
	}
}

func TestProseGateValidExemptionPermits(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	// A file whose only banned rune is exempted scans clean, exercising the
	// exemption-parse-and-append path.
	root := proseGateRepo(t,
		"proseGate:\n  enabled: true\n  exemptions:\n    - path: depict.md\n      codepoint: U+2014\n",
		map[string]string{"depict.md": "the em dash \u2014 is written about here\n"})
	var out strings.Builder
	if err := runProseGate(ctx, root, &out); err != nil {
		t.Fatalf("valid exemption: want nil, got %v", err)
	}
	if !strings.Contains(out.String(), "check prose: clean") {
		t.Errorf("valid exemption: output %q", out.String())
	}
}

func TestProseGateFindings(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := proseGateRepo(t, "proseGate:\n  enabled: true\n",
		map[string]string{"a.md": "an em dash \u2014 here\n"})
	var out strings.Builder
	err := runProseGate(ctx, root, &out)
	if err == nil || !strings.Contains(err.Error(), "plain punctuation") {
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
		root := proseGateRepo(t, "proseGate:\n  enabled: true\n", map[string]string{"a.md": "staged \u2014\n"})
		if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("worktree clean\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := runProseGate(ctx, root, io.Discard); err == nil {
			t.Fatal("staged banned content must fail even when the worktree copy is clean")
		}
	})
	t.Run("staged file missing from worktree still scans", func(t *testing.T) {
		root := proseGateRepo(t, "proseGate:\n  enabled: true\n", map[string]string{"vanish.md": "clean\n"})
		if err := os.Remove(filepath.Join(root, "vanish.md")); err != nil {
			t.Fatal(err)
		}
		if err := runProseGate(ctx, root, io.Discard); err != nil {
			t.Fatalf("staged clean file: %v", err)
		}
	})
}

func TestProseGateUsesStagedConfig(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := proseGateRepo(t, "proseGate:\n  enabled: true\n", map[string]string{"a.md": "staged \u2014\n"})
	testsupport.WriteAwfConfig(t, root, "prefix: example\nskills: []\nagents: []\nproseGate:\n  enabled: false\n")
	if err := runProseGate(ctx, root, io.Discard); err == nil {
		t.Fatal("worktree disabled knob must not override staged enabled config")
	}
}

func TestProseGateUsesStagedExemption(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := proseGateRepo(t,
		"proseGate:\n  enabled: true\n  exemptions:\n    - path: depict.md\n      codepoint: U+2014\n",
		map[string]string{"depict.md": "staged \u2014\n"})
	testsupport.WriteAwfConfig(t, root, "prefix: example\nskills: []\nagents: []\nproseGate:\n  enabled: true\n")
	if err := runProseGate(ctx, root, io.Discard); err != nil {
		t.Fatalf("staged exemption must control despite worktree config: %v", err)
	}
}

func TestProseGateSkipsStagedGitlink(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := proseGateRepo(t, "proseGate:\n  enabled: true\n", map[string]string{"a.md": "clean\n"})
	repo, err := git.PlainOpen(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := repo.Storer.Index()
	if err != nil {
		t.Fatal(err)
	}
	idx.Entries = append(idx.Entries, &indexformat.Entry{Name: "submodule", Mode: filemode.Submodule, Hash: plumbing.NewHash("0123456789012345678901234567890123456789")})
	if err := repo.Storer.SetIndex(idx); err != nil {
		t.Fatal(err)
	}
	if err := runProseGate(ctx, root, io.Discard); err != nil {
		t.Fatalf("gitlink must not block regular staged files: %v", err)
	}
}

func TestProseGateDispatch(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	// Drive the command through run() so the dispatch handler closure is
	// exercised, not just runProseGate directly.
	root := proseGateRepo(t, "proseGate:\n  enabled: true\n",
		map[string]string{"a.md": "plain ascii\n"})
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb strings.Builder
	if code := run([]string{"awf", "check", "prose"}, &out, &errb); code != 0 {
		t.Fatalf("check prose exited %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "check prose: clean") {
		t.Errorf("dispatch: output %q", out.String())
	}
}

// invariant: tooling/quality-gates:prose-gate-refuses-without-git
func TestProseGateRefusesOutsideAGitRepo(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	// An adopted tree outside a git repository has no staged snapshot, so the
	// command refuses rather than reporting a clean tree it could not see.
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nskills: []\nagents: []\nproseGate:\n  enabled: true\n")
	err := runProseGate(ctx, root, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "cannot read staged files") {
		t.Fatalf("outside git: want a refusal naming the enumeration failure, got %v", err)
	}
}
