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

// dir is the working-memory prefix; every fixture below builds its citation
// from it rather than writing the shape out, so this file does not carry the
// very thing the gate rejects.
const dir = ".awf/efforts/"

// memoryGateRepo writes an .awf/config.yaml with the given memoryCite block,
// git-inits the root, and stages the named files (content keyed by path).
func memoryGateRepo(t *testing.T, memoryCiteYAML string, stage map[string]string) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nintegrationBranch: main\nskills: []\nagents: []\n"+memoryCiteYAML)
	repo := gitfixture.InitRepoAt(t, root)
	gitfixture.Add(t, repo, ".awf/config.yaml")
	gitfixture.Stage(t, repo, stage)
	return root
}

func TestMemoryGateKnobOff(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	// A bare directory has no project config, so the knob cannot be consulted.
	if err := runMemoryGate(ctx, t.TempDir(), io.Discard); err == nil {
		t.Error("bare directory: want a staged-snapshot read error, got nil")
	}
	// Knob absent and knob explicitly false both disclose the disabled child and
	// return nil without scanning, even with a citing file staged.
	for _, y := range []string{"", "memoryCite:\n  enabled: false\n"} {
		root := memoryGateRepo(t, y, map[string]string{"docs/plans/p.md": cite() + "\n"})
		if err := runMemoryGate(ctx, root, io.Discard); err != nil {
			t.Errorf("knob off (%q): want nil, got %v", y, err)
		}
	}
}

func TestMemoryGateRefusesMissingOrInvalidWorkingConfig(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := gitfixture.InitRepo(t).Root()
	if err := runMemoryGate(ctx, root, io.Discard); err == nil || !strings.Contains(err.Error(), "not an awf project") {
		t.Fatalf("missing working config: %v", err)
	}

	root = memoryGateRepo(t, "memoryCite: [\n", nil)
	if err := runMemoryGate(ctx, root, io.Discard); err == nil || !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("invalid working config: %v", err)
	}
}

func TestMemoryGateClean(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := memoryGateRepo(t, "memoryCite:\n  enabled: true\n", map[string]string{
		"docs/plans/p.md":     "the file lives under " + dir + " and is named " + dir + "<effort-slug>/memory.md\n",
		"docs/decisions/a.md": "the ignore file " + dir + ".gitignore" + " is fine\n",
	})
	var out strings.Builder
	if err := runMemoryGate(ctx, root, &out); err != nil {
		t.Fatalf("clean: want nil, got %v", err)
	}
	if !strings.Contains(out.String(), "check repo memory: clean") {
		t.Errorf("clean: output %q", out.String())
	}
}

// invariant: tooling/quality-gates:memory-citation-gate (TestMemoryGateFindings)
func TestMemoryGateFindings(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	for _, path := range []string{"docs/decisions/0001-x.md", "docs/plans/2026-01-01-x.md"} {
		root := memoryGateRepo(t, "memoryCite:\n  enabled: true\n",
			map[string]string{path: "intro\n" + cite() + "\n"})
		var out strings.Builder
		err := runMemoryGate(ctx, root, &out)
		if err == nil || !strings.Contains(err.Error(), "memoryCite.exemptions") {
			t.Fatalf("%s: want a non-nil error naming the way out, got %v", path, err)
		}
		if !strings.Contains(out.String(), path) || !strings.Contains(out.String(), "line(s) 2") {
			t.Errorf("%s: output %q", path, out.String())
		}
	}
}

func TestMemoryGateScansOnlyDecisionRecords(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := memoryGateRepo(t, "memoryCite:\n  enabled: true\n", map[string]string{
		"docs/guide.md": cite() + "\n",
		"notes.md":      cite() + "\n",
	})
	var out strings.Builder
	if err := runMemoryGate(ctx, root, &out); err != nil {
		t.Fatalf("a citation outside the scanned prefixes must be ignored: %v", err)
	}
	if !strings.Contains(out.String(), "check repo memory: clean") {
		t.Errorf("output %q", out.String())
	}
}

func TestMemoryGateFollowsDocsDir(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := memoryGateRepo(t, "docsDir: handbook\nmemoryCite:\n  enabled: true\n", map[string]string{
		"handbook/plans/p.md": cite() + "\n",
		"docs/plans/q.md":     dir + "other/memory.md\n",
	})
	var out strings.Builder
	err := runMemoryGate(ctx, root, &out)
	if err == nil {
		t.Fatal("a custom docsDir must be scanned")
	}
	if strings.Contains(out.String(), "docs/plans/q.md") {
		t.Errorf("the default docs directory must not be scanned under a custom docsDir: %q", out.String())
	}
}

func TestMemoryGateExemptionPermits(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := memoryGateRepo(t,
		"memoryCite:\n  enabled: true\n  exemptions:\n    - path: docs/plans/p.md\n",
		map[string]string{"docs/plans/p.md": cite() + "\n"})
	var out strings.Builder
	if err := runMemoryGate(ctx, root, &out); err != nil {
		t.Fatalf("exempt path: want nil, got %v", err)
	}
	if !strings.Contains(out.String(), "check repo memory: clean") {
		t.Errorf("exempt path: output %q", out.String())
	}
}

func TestMemoryGateSkipsStagedSymlink(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	// A staged symlink's bytes are its target path, not document text, so even a
	// target of exactly the flagged shape is not a citation.
	root := memoryGateRepo(t, "memoryCite:\n  enabled: true\n",
		map[string]string{"docs/plans/p.md": "clean\n"})
	if err := os.Symlink(cite(), filepath.Join(root, "docs/plans/link.md")); err != nil {
		t.Fatal(err)
	}
	gitfixture.Add(t, gitfixture.At(root), "docs/plans/link.md")
	if err := runMemoryGate(ctx, root, io.Discard); err != nil {
		t.Fatalf("a staged symlink must not block regular staged files: %v", err)
	}
}

func TestMemoryGateUsesStagedBytesWhenWorktreeDiffers(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	t.Run("citation cleaned without restaging remains a finding", func(t *testing.T) {
		root := memoryGateRepo(t, "memoryCite:\n  enabled: true\n",
			map[string]string{"docs/plans/p.md": cite() + "\n"})
		if err := os.WriteFile(filepath.Join(root, "docs/plans/p.md"), []byte("worktree clean\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := runMemoryGate(ctx, root, io.Discard); err == nil {
			t.Fatal("staged citation must fail even when the worktree copy is clean")
		}
	})
	t.Run("citation added without staging is not a finding", func(t *testing.T) {
		root := memoryGateRepo(t, "memoryCite:\n  enabled: true\n",
			map[string]string{"docs/plans/p.md": "clean\n"})
		if err := os.WriteFile(filepath.Join(root, "docs/plans/p.md"), []byte(cite()+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := runMemoryGate(ctx, root, io.Discard); err != nil {
			t.Fatalf("an unstaged citation must not fail the gate: %v", err)
		}
	})
}

func TestMemoryGateUsesWorkingConfigKnob(t *testing.T) {
	ctx := testContext(t)
	root := memoryGateRepo(t, "memoryCite:\n  enabled: true\n",
		map[string]string{"docs/plans/p.md": cite() + "\n"})
	testsupport.WriteAwfConfig(t, root, "prefix: example\nintegrationBranch: main\nskills: []\nagents: []\nmemoryCite:\n  enabled: false\n")
	if err := runMemoryGate(ctx, root, io.Discard); err != nil {
		t.Fatalf("worktree-disabled knob must return before reading the staged corpus: %v", err)
	}
}

func TestMemoryGateDispatch(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	// Drive the command through run() so the dispatch handler closure is
	// exercised, not just runMemoryGate directly.
	root := memoryGateRepo(t, "memoryCite:\n  enabled: true\n",
		map[string]string{"docs/plans/p.md": "clean\n"})
	if err := initializeProject(ctx, root, io.Discard); err != nil {
		t.Fatal(err)
	}
	gitfixture.AddAll(t, gitfixture.At(root))
	gitfixture.Commit(t, gitfixture.At(root), "fixture", nil)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb strings.Builder
	if code := run([]string{"awf", "check", "repo", "memory"}, &out, &errb); code != 0 {
		t.Fatalf("check repo memory exited %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "check repo memory: clean") {
		t.Errorf("dispatch: output %q", out.String())
	}
}

func TestMemoryGateRefusesOutsideAGitRepo(t *testing.T) {
	ctx := testContext(t)
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nintegrationBranch: main\nskills: []\nagents: []\nmemoryCite:\n  enabled: false\n")
	if err := runMemoryGate(ctx, root, io.Discard); err != nil {
		t.Fatalf("disabled outside git must return before reading the index: %v", err)
	}
	testsupport.WriteAwfConfig(t, root, "prefix: example\nintegrationBranch: main\nskills: []\nagents: []\nmemoryCite:\n  enabled: true\n")
	err := runMemoryGate(ctx, root, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "cannot read staged files") {
		t.Fatalf("enabled outside git: want a refusal naming the enumeration failure, got %v", err)
	}
}
