package adr

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
)

func TestRenumberPendingRetainsDestinationWhenSourceRetirementFails(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "pending.md")
	if err := os.WriteFile(source, []byte("# ADR-pending: Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("source remove fault")
	files, err := filesystem.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close()
	err = renumberPending(files, "", "pending", 2, func(string, fs.FileInfo) error { return cause })
	var partial *PartialRenumberError
	if !errors.As(err, &partial) || !partial.DestinationPublished || partial.SourceRetired || !errors.Is(err, cause) {
		t.Fatalf("partial = %#v, err = %v", partial, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "0002-pending.md")); err != nil {
		t.Fatalf("destination missing: %v", err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("source missing: %v", err)
	}
}
