//go:build !windows

package adr

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestCanonicalDecisionsDirectoryRejectsDeletedWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	_, err := canonicalDecisionsDirectory(".")
	if err == nil || !strings.Contains(err.Error(), "make absolute") || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("deleted working directory error = %v; want wrapped not-exist identity with context", err)
	}
}
