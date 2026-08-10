//go:build windows

package adr

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalDecisionsDirectoryCollapsesWindowsAliases(t *testing.T) {
	dir, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	want, err := canonicalDecisionsDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, alias := range []string{strings.ToUpper(dir), `\\?\` + dir} {
		got, err := canonicalDecisionsDirectory(alias)
		if err != nil {
			t.Fatalf("canonicalize alias %q: %v", alias, err)
		}
		if got != want {
			t.Fatalf("canonical identity for %q = %q, want %q", alias, got, want)
		}
	}
}
