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
		testsupport.WithDomains("tooling"),
		testsupport.WithRetiresInvariants("old-slug"),
		testsupport.WithBody("## Context\nbody\n"),
	)
	want := "---\nstatus: Implemented\ndate: 2026-06-25\ntags: [x, y]\ndomains: [tooling]\nretires_invariants: [old-slug]\n---\n# ADR-0002: Full\n## Context\nbody\n"
	if got != want {
		t.Errorf("ADR(full) =\n%q\nwant\n%q", got, want)
	}
}
