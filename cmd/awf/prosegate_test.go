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
	// No .awf at all: config.Load fails.
	if err := runProseGate(t.TempDir(), io.Discard); err == nil {
		t.Error("no .awf: want a config-load error, got nil")
	}
	// Knob absent, and knob explicitly false: both no-op and return nil.
	for _, y := range []string{"", "proseGate:\n  enabled: false\n"} {
		root := t.TempDir()
		testsupport.WriteAwfConfig(t, root, "prefix: example\nskills: []\nagents: []\n"+y)
		if err := runProseGate(root, io.Discard); err != nil {
			t.Errorf("knob off (%q): want nil, got %v", y, err)
		}
	}
}

func TestProseGateBadExemptionCodepoint(t *testing.T) {
	root := proseGateRepo(t,
		"proseGate:\n  enabled: true\n  exemptions:\n    - path: x.md\n      codepoint: not-a-codepoint\n",
		map[string]string{"x.md": "clean\n"})
	err := runProseGate(root, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "exemption for x.md") {
		t.Fatalf("bad codepoint: want an exemption error, got %v", err)
	}
}

func TestProseGateClean(t *testing.T) {
	root := proseGateRepo(t, "proseGate:\n  enabled: true\n",
		map[string]string{"a.md": "plain ascii\n"})
	var out strings.Builder
	if err := runProseGate(root, &out); err != nil {
		t.Fatalf("clean: want nil, got %v", err)
	}
	if !strings.Contains(out.String(), "prose-gate: clean") {
		t.Errorf("clean: output %q", out.String())
	}
}

func TestProseGateValidExemptionPermits(t *testing.T) {
	// A file whose only banned rune is exempted scans clean, exercising the
	// exemption-parse-and-append path.
	root := proseGateRepo(t,
		"proseGate:\n  enabled: true\n  exemptions:\n    - path: depict.md\n      codepoint: U+2014\n",
		map[string]string{"depict.md": "the em dash \u2014 is written about here\n"})
	var out strings.Builder
	if err := runProseGate(root, &out); err != nil {
		t.Fatalf("valid exemption: want nil, got %v", err)
	}
	if !strings.Contains(out.String(), "prose-gate: clean") {
		t.Errorf("valid exemption: output %q", out.String())
	}
}

func TestProseGateFindings(t *testing.T) {
	root := proseGateRepo(t, "proseGate:\n  enabled: true\n",
		map[string]string{"a.md": "an em dash \u2014 here\n"})
	var out strings.Builder
	err := runProseGate(root, &out)
	if err == nil || !strings.Contains(err.Error(), "plain punctuation") {
		t.Fatalf("findings: want a non-nil error, got %v", err)
	}
	if !strings.Contains(out.String(), "a.md") || !strings.Contains(out.String(), "em-dash") {
		t.Errorf("findings: output %q", out.String())
	}
}

func TestProseGateScanReadError(t *testing.T) {
	root := proseGateRepo(t, "proseGate:\n  enabled: true\n",
		map[string]string{"vanish.md": "clean\n"})
	// Staged in the index, then removed from disk without staging the deletion:
	// IndexPaths still returns it, and Scan's ReadFile fails.
	if err := os.Remove(filepath.Join(root, "vanish.md")); err != nil {
		t.Fatal(err)
	}
	if err := runProseGate(root, io.Discard); err == nil || !strings.Contains(err.Error(), "prose-gate:") {
		t.Fatalf("scan read error: want a prose-gate error, got %v", err)
	}
}

func TestProseGateDispatch(t *testing.T) {
	// Drive the command through run() so the dispatch handler closure is
	// exercised, not just runProseGate directly.
	root := proseGateRepo(t, "proseGate:\n  enabled: true\n",
		map[string]string{"a.md": "plain ascii\n"})
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb strings.Builder
	if code := run([]string{"awf", "prose-gate"}, &out, &errb); code != 0 {
		t.Fatalf("prose-gate exited %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "prose-gate: clean") {
		t.Errorf("dispatch: output %q", out.String())
	}
}

// invariant: prose-gate-refuses-without-git
func TestProseGateRefusesOutsideAGitRepo(t *testing.T) {
	// An adopted tree (the knob is on) that is not a git repository: config.Load
	// succeeds, the knob check passes, and IndexPaths is the thing that fails, so
	// the command refuses rather than reporting a clean tree it could not see.
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nskills: []\nagents: []\nproseGate:\n  enabled: true\n")
	err := runProseGate(root, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "enumerate tracked files") {
		t.Fatalf("outside git: want a refusal naming the enumeration failure, got %v", err)
	}
}
