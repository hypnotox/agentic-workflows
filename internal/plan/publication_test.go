package plan

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/filepublication"
)

func TestNewFileReturnsNonCollisionPublicationError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte("date: YYYY-MM-DD\n# Plan: Title\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := now
	now = func() time.Time { return time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { now = previous })
	publicationFailure := errors.New("publication failure")
	_, err := newFile(dir, "Failure", func(string, []byte, os.FileMode) error { return publicationFailure })
	if !errors.Is(err, publicationFailure) {
		t.Fatalf("NewFile error = %v, want publication failure", err)
	}
}

func TestNewFileRequestsScaffoldMode(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte("date: YYYY-MM-DD\n# Plan: Title\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := now
	now = func() time.Time { return time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { now = previous })
	var requested os.FileMode
	_, err := newFile(dir, "Mode", func(path string, contents []byte, mode os.FileMode) error {
		requested = mode
		return filepublication.Publish(path, contents, mode)
	})
	if err != nil {
		t.Fatal(err)
	}
	if requested.Perm() != 0o644 {
		t.Fatalf("publication mode = %o, want 644", requested.Perm())
	}
}

// TestNewFileRefusesPublicationCollision makes the competing winner appear at
// the publication boundary, rather than relying on scheduling between a probe
// and write. The scaffold must retain its established refusal presentation.
// invariant: adr-system/plan-artifacts:plan-new-unnumbered (TestNewFileRefusesPublicationCollision)
func TestNewFileRefusesPublicationCollision(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte("date: YYYY-MM-DD\n# Plan: Title\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := now
	now = func() time.Time { return time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { now = previous })

	var destination string
	publish := func(path string, contents []byte, mode os.FileMode) error {
		destination = path
		if err := os.WriteFile(path, []byte("concurrent winner"), 0o600); err != nil {
			t.Fatal(err)
		}
		return filepublication.Publish(path, contents, mode)
	}
	_, err := newFile(dir, "Collision", publish)
	if err == nil || err.Error() != "plan: "+destination+" already exists" {
		t.Fatalf("NewFile collision error = %v, want established overwrite refusal", err)
	}
	winner, readErr := os.ReadFile(destination)
	if readErr != nil || string(winner) != "concurrent winner" {
		t.Fatalf("winner bytes = %q, read error = %v", winner, readErr)
	}
}
