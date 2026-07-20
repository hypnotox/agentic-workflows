package testsupport_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestWriteFileCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c.txt")
	testsupport.WriteFile(t, path, "hello\n")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello\n" {
		t.Errorf("WriteFile content = %q, want %q", got, "hello\n")
	}
}

func TestWriteAwfConfig(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\n")
	got, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "prefix: example\n" {
		t.Errorf("WriteAwfConfig content = %q", got)
	}
}

func TestSwapVar(t *testing.T) {
	seam := 1
	t.Run("swap", func(t *testing.T) {
		testsupport.SwapVar(t, &seam, 2)
		if seam != 2 {
			t.Fatalf("seam = %d, want 2", seam)
		}
	})
	if seam != 1 {
		t.Errorf("seam not restored after subtest, got %d, want 1", seam)
	}
}

func TestWriteGoModule(t *testing.T) {
	dir := t.TempDir()
	testsupport.WriteGoModule(t, dir, "example.com/m", "package m\nfunc F() {}\n")
	mod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mod), "module example.com/m") || !strings.Contains(string(mod), "go 1.26") {
		t.Errorf("go.mod = %q", mod)
	}
	src, err := os.ReadFile(filepath.Join(dir, "f.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(src) != "package m\nfunc F() {}\n" {
		t.Errorf("f.go = %q", src)
	}
}

func TestWriteProfile(t *testing.T) {
	dir := t.TempDir()
	path := testsupport.WriteProfile(t, dir, "example.com/m/f.go:2.1,2.5 1 1\n")
	if path != filepath.Join(dir, "cover.out") {
		t.Errorf("WriteProfile path = %q", path)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(got), "mode: set\n") {
		t.Errorf("profile missing mode header: %q", got)
	}
}

func TestADRMinimal(t *testing.T) {
	got := testsupport.ADR("Proposed")
	want := "---\nstatus: Proposed\n---\n# ADR-0001: T\n"
	if got != want {
		t.Errorf("ADR(minimal) = %q, want %q", got, want)
	}
}

func TestADREveryOption(t *testing.T) {
	got := testsupport.ADR("Implemented",
		testsupport.WithTitle("0002: Full"),
		testsupport.WithDate("2026-06-25"),
		testsupport.WithTags("x", "y"),
		testsupport.WithRelated(1, 5),
		testsupport.WithDomains("tooling"),
		testsupport.WithBody("## Context\nbody\n"),
	)
	want := "---\nstatus: Implemented\ndate: 2026-06-25\ntags: [x, y]\nrelated: [1, 5]\ndomains: [tooling]\n---\n# ADR-0002: Full\n## Context\nbody\n"
	if got != want {
		t.Errorf("ADR(full) =\n%q\nwant\n%q", got, want)
	}
}

// WalkRepoSources owns awf's repo-walk boundary, so its pruning is pinned here
// rather than rediscovered by each scanner that uses it.
func TestWalkRepoSourcesBoundary(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, "keep.go"), "package x\n")
	testsupport.WriteFile(t, filepath.Join(root, "skip_test.go"), "package x\n")
	testsupport.WriteFile(t, filepath.Join(root, "notes.md"), "not go\n")
	testsupport.WriteFile(t, filepath.Join(root, "pkg", "nested.go"), "package y\n")
	// A hidden tree: .claude/worktrees/ holds session checkouts of this repo.
	testsupport.WriteFile(t, filepath.Join(root, ".claude", "worktrees", "s", "foreign.go"), "package z\n")
	// A non-hidden nested checkout, marked by its own .git entry.
	testsupport.WriteFile(t, filepath.Join(root, "vendored", "other.go"), "package w\n")
	testsupport.WriteFile(t, filepath.Join(root, "vendored", ".git"), "gitdir: elsewhere\n")

	seen := map[string]bool{}
	testsupport.WalkRepoSources(t, root, func(rel string, body []byte) {
		seen[rel] = true
		if len(body) == 0 {
			t.Errorf("%s: body must be read", rel)
		}
	})
	want := map[string]bool{"keep.go": true, "pkg/nested.go": true}
	for w := range want {
		if !seen[w] {
			t.Errorf("production source %q was not walked", w)
		}
	}
	for _, skipped := range []string{
		"skip_test.go", "notes.md",
		".claude/worktrees/s/foreign.go", "vendored/other.go",
	} {
		if seen[skipped] {
			t.Errorf("%q must be outside the repo-walk boundary", skipped)
		}
	}

	var markdown []string
	testsupport.WalkRepoFiles(t, root, func(rel string) bool {
		return filepath.Ext(rel) == ".md"
	}, func(rel string, _ []byte) {
		markdown = append(markdown, rel)
	})
	if got := strings.Join(markdown, ","); got != "notes.md" {
		t.Fatalf("repository Markdown = %q, want notes.md", got)
	}
}
