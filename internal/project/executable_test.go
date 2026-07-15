package project

import (
	"os"
	"path/filepath"
	"testing"
)

// A rendered #!-shebang script is written executable (0755); every other rendered
// file stays 0644. The mode is enforced on every sync - a pre-existing file's mode
// is corrected, not only set at creation (ADR-0100 Decision 8).
// invariant: shebang-rendered-executable
func TestShebangRenderedExecutable(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\ndocs: []\ndomains: []\nhooks:\n  enabled: true\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	shebang := filepath.Join(root, ".awf/hooks/pre-commit.sh") // "#!/usr/bin/env bash…"
	markdown := filepath.Join(root, "AGENTS.md")               // "<!-- GENERATED… -->…"
	assertPerm(t, shebang, 0o755)
	assertPerm(t, markdown, 0o644)

	// A pre-existing file with the wrong mode is corrected on the next sync.
	if err := os.Chmod(shebang, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	assertPerm(t, shebang, 0o755)
}

func assertPerm(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != want {
		t.Errorf("%s mode = %o, want %o", path, got, want)
	}
}
