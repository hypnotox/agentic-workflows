package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// dir is the working-memory prefix; every fixture below builds its citation
// from it rather than writing the shape out, so this file does not carry the
// very thing the gate rejects.
const dir = ".awf/memory/"

// memoryGateRepo writes an .awf/config.yaml with the given memoryCite block,
// git-inits the root, and stages the named files (content keyed by path).
func memoryGateRepo(t *testing.T, memoryCiteYAML string, stage map[string]string) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nskills: []\nagents: []\n"+memoryCiteYAML)
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

func TestMemoryGateKnobOff(t *testing.T) {
	// No .awf at all: config.Load fails.
	if err := runMemoryGate(t.TempDir(), io.Discard); err == nil {
		t.Error("no .awf: want a config-load error, got nil")
	}
	// Knob absent, and knob explicitly false: both no-op and return nil, even
	// with a citing file staged.
	for _, y := range []string{"", "memoryCite:\n  enabled: false\n"} {
		root := memoryGateRepo(t, y, map[string]string{"docs/plans/p.md": dir + "effort.md\n"})
		if err := runMemoryGate(root, io.Discard); err != nil {
			t.Errorf("knob off (%q): want nil, got %v", y, err)
		}
	}
}

func TestMemoryGateRefusesMissingOrInvalidStagedConfig(t *testing.T) {
	root := t.TempDir()
	if _, err := git.PlainInit(root, false); err != nil {
		t.Fatal(err)
	}
	if err := runMemoryGate(root, io.Discard); err == nil || !strings.Contains(err.Error(), "staged snapshot has no") {
		t.Fatalf("missing staged config: %v", err)
	}

	root = memoryGateRepo(t, "memoryCite: [\n", nil)
	if err := runMemoryGate(root, io.Discard); err == nil || !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("invalid staged config: %v", err)
	}
}

func TestMemoryGateClean(t *testing.T) {
	root := memoryGateRepo(t, "memoryCite:\n  enabled: true\n", map[string]string{
		"docs/plans/p.md":     "the file lives under " + dir + " and is named " + dir + "<effort-slug>.md\n",
		"docs/decisions/a.md": "the ignore file " + dir + ".gitignore" + " is fine\n",
	})
	var out strings.Builder
	if err := runMemoryGate(root, &out); err != nil {
		t.Fatalf("clean: want nil, got %v", err)
	}
	if !strings.Contains(out.String(), "memory-gate: clean") {
		t.Errorf("clean: output %q", out.String())
	}
}

func TestMemoryGateFindings(t *testing.T) {
	for _, path := range []string{"docs/decisions/0001-x.md", "docs/plans/2026-01-01-x.md"} {
		root := memoryGateRepo(t, "memoryCite:\n  enabled: true\n",
			map[string]string{path: "intro\n" + dir + "effort.md\n"})
		var out strings.Builder
		err := runMemoryGate(root, &out)
		if err == nil || !strings.Contains(err.Error(), "memoryCite.exemptions") {
			t.Fatalf("%s: want a non-nil error naming the way out, got %v", path, err)
		}
		if !strings.Contains(out.String(), path) || !strings.Contains(out.String(), "line(s) 2") {
			t.Errorf("%s: output %q", path, out.String())
		}
	}
}

func TestMemoryGateScansOnlyDecisionRecords(t *testing.T) {
	root := memoryGateRepo(t, "memoryCite:\n  enabled: true\n", map[string]string{
		"docs/guide.md": dir + "effort.md\n",
		"notes.md":      dir + "effort.md\n",
	})
	var out strings.Builder
	if err := runMemoryGate(root, &out); err != nil {
		t.Fatalf("a citation outside the scanned prefixes must be ignored: %v", err)
	}
	if !strings.Contains(out.String(), "memory-gate: clean") {
		t.Errorf("output %q", out.String())
	}
}

func TestMemoryGateFollowsDocsDir(t *testing.T) {
	root := memoryGateRepo(t, "docsDir: handbook\nmemoryCite:\n  enabled: true\n", map[string]string{
		"handbook/plans/p.md": dir + "effort.md\n",
		"docs/plans/q.md":     dir + "other.md\n",
	})
	var out strings.Builder
	err := runMemoryGate(root, &out)
	if err == nil {
		t.Fatal("a custom docsDir must be scanned")
	}
	if strings.Contains(out.String(), "docs/plans/q.md") {
		t.Errorf("the default docs directory must not be scanned under a custom docsDir: %q", out.String())
	}
}

func TestMemoryGateExemptionPermits(t *testing.T) {
	root := memoryGateRepo(t,
		"memoryCite:\n  enabled: true\n  exemptions:\n    - path: docs/plans/p.md\n",
		map[string]string{"docs/plans/p.md": dir + "effort.md\n"})
	var out strings.Builder
	if err := runMemoryGate(root, &out); err != nil {
		t.Fatalf("exempt path: want nil, got %v", err)
	}
	if !strings.Contains(out.String(), "memory-gate: clean") {
		t.Errorf("exempt path: output %q", out.String())
	}
}

func TestMemoryGateSkipsStagedSymlink(t *testing.T) {
	// A staged symlink's bytes are its target path, not document text, so even a
	// target of exactly the flagged shape is not a citation.
	root := memoryGateRepo(t, "memoryCite:\n  enabled: true\n",
		map[string]string{"docs/plans/p.md": "clean\n"})
	if err := os.Symlink(dir+"effort.md", filepath.Join(root, "docs/plans/link.md")); err != nil {
		t.Fatal(err)
	}
	repo, err := git.PlainOpen(root)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("docs/plans/link.md"); err != nil {
		t.Fatal(err)
	}
	if err := runMemoryGate(root, io.Discard); err != nil {
		t.Fatalf("a staged symlink must not block regular staged files: %v", err)
	}
}

func TestMemoryGateUsesStagedBytesWhenWorktreeDiffers(t *testing.T) {
	t.Run("citation cleaned without restaging remains a finding", func(t *testing.T) {
		root := memoryGateRepo(t, "memoryCite:\n  enabled: true\n",
			map[string]string{"docs/plans/p.md": dir + "effort.md\n"})
		if err := os.WriteFile(filepath.Join(root, "docs/plans/p.md"), []byte("worktree clean\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := runMemoryGate(root, io.Discard); err == nil {
			t.Fatal("staged citation must fail even when the worktree copy is clean")
		}
	})
	t.Run("citation added without staging is not a finding", func(t *testing.T) {
		root := memoryGateRepo(t, "memoryCite:\n  enabled: true\n",
			map[string]string{"docs/plans/p.md": "clean\n"})
		if err := os.WriteFile(filepath.Join(root, "docs/plans/p.md"), []byte(dir+"effort.md\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := runMemoryGate(root, io.Discard); err != nil {
			t.Fatalf("an unstaged citation must not fail the gate: %v", err)
		}
	})
}

func TestMemoryGateUsesStagedConfig(t *testing.T) {
	root := memoryGateRepo(t, "memoryCite:\n  enabled: true\n",
		map[string]string{"docs/plans/p.md": dir + "effort.md\n"})
	testsupport.WriteAwfConfig(t, root, "prefix: example\nskills: []\nagents: []\nmemoryCite:\n  enabled: false\n")
	if err := runMemoryGate(root, io.Discard); err == nil {
		t.Fatal("worktree disabled knob must not override staged enabled config")
	}
}

func TestMemoryGateDispatch(t *testing.T) {
	// Drive the command through run() so the dispatch handler closure is
	// exercised, not just runMemoryGate directly.
	root := memoryGateRepo(t, "memoryCite:\n  enabled: true\n",
		map[string]string{"docs/plans/p.md": "clean\n"})
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb strings.Builder
	if code := run([]string{"awf", "memory-gate"}, &out, &errb); code != 0 {
		t.Fatalf("memory-gate exited %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "memory-gate: clean") {
		t.Errorf("dispatch: output %q", out.String())
	}
}

func TestMemoryGateRefusesOutsideAGitRepo(t *testing.T) {
	// An adopted tree outside a git repository has no staged snapshot, so the
	// command refuses rather than reporting a clean tree it could not see.
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nskills: []\nagents: []\nmemoryCite:\n  enabled: true\n")
	err := runMemoryGate(root, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "cannot read staged files") {
		t.Fatalf("outside git: want a refusal naming the enumeration failure, got %v", err)
	}
}
