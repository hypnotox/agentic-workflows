package filepublication

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestMoveNoReplacePreservesDirectoryOnCollision(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	destination := filepath.Join(dir, "destination")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := MoveNoReplace(source, destination); !errors.Is(err, os.ErrExist) {
		t.Fatalf("collision error = %v", err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("collision changed source: %v", err)
	}
	if err := os.Remove(destination); err != nil {
		t.Fatal(err)
	}
	if err := MoveNoReplace(source, destination); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(destination); err != nil {
		t.Fatalf("move destination absent: %v", err)
	}
}

// invariant: tooling/file-publication:exclusive-file-publication-single-home (TestPublishCompleteExclusiveFile)
func TestPublishCompleteExclusiveFile(t *testing.T) {
	t.Run("prepares complete file with requested mode", func(t *testing.T) {
		destination := filepath.Join(t.TempDir(), "artifact")
		if err := Publish(destination, []byte("complete bytes"), 0o640); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(destination)
		if err != nil || string(raw) != "complete bytes" {
			t.Fatalf("published bytes = %q, %v", raw, err)
		}
		info, err := os.Stat(destination)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o640 {
			t.Fatalf("published mode = %o, want 640", got)
		}
	})

	t.Run("reports a missing parent without creating a destination", func(t *testing.T) {
		destination := filepath.Join(t.TempDir(), "missing", "artifact")
		err := Publish(destination, []byte("complete bytes"), 0o644)
		if !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), "create publication temporary") {
			t.Fatalf("missing-parent error = %v; want wrapped not-exist identity with context", err)
		}
		if _, statErr := os.Stat(destination); !errors.Is(statErr, fs.ErrNotExist) {
			t.Fatalf("missing-parent destination exists or has unexpected error: %v", statErr)
		}
	})

	t.Run("refuses existing destination without changing bytes and cleans temporary", func(t *testing.T) {
		dir := t.TempDir()
		destination := filepath.Join(dir, "artifact")
		if err := os.WriteFile(destination, []byte("winner"), 0o600); err != nil {
			t.Fatal(err)
		}
		err := Publish(destination, []byte("loser"), 0o644)
		if !errors.Is(err, os.ErrExist) {
			t.Fatalf("destination collision error = %v", err)
		}
		raw, readErr := os.ReadFile(destination)
		if readErr != nil || string(raw) != "winner" {
			t.Fatalf("existing bytes = %q, %v", raw, readErr)
		}
		entries, readErr := os.ReadDir(dir)
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".filepublication-") {
				t.Fatalf("temporary %s remains after failed publication", entry.Name())
			}
		}
	})
}

// invariant: tooling/file-publication:exclusive-file-publication-single-home (TestPublishConcurrentPublishersLeaveOneCompleteWinner)
type confinedFixtureRoot struct {
	dir       string
	openErr   error
	linkErr   error
	removeErr error
	openCalls int
}

func (r *confinedFixtureRoot) OpenFile(name string, flag int, mode os.FileMode) (*os.File, error) {
	r.openCalls++
	if r.openErr != nil {
		return nil, r.openErr
	}
	return os.OpenFile(filepath.Join(r.dir, filepath.FromSlash(name)), flag, mode)
}

func (r *confinedFixtureRoot) Link(oldname, newname string) error {
	if r.linkErr != nil {
		return r.linkErr
	}
	return os.Link(filepath.Join(r.dir, filepath.FromSlash(oldname)), filepath.Join(r.dir, filepath.FromSlash(newname)))
}

func (r *confinedFixtureRoot) Remove(name string) error {
	removeErr := os.Remove(filepath.Join(r.dir, filepath.FromSlash(name)))
	if r.removeErr != nil {
		return r.removeErr
	}
	return removeErr
}

// invariant: tooling/file-publication:exclusive-file-publication-single-home (TestPublishConfinedCompleteExclusiveFile)
func TestPublishConfinedCompleteExclusiveFile(t *testing.T) {
	t.Run("preparation failure leaves destination absent", func(t *testing.T) {
		dir := t.TempDir()
		destination := "artifact"
		preparationErr := errors.New("preparation failed")
		err := PublishConfined(&confinedFixtureRoot{dir: dir, openErr: preparationErr}, destination, []byte("loser"), 0o644)
		if !errors.Is(err, preparationErr) {
			t.Fatalf("preparation error = %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, destination)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed preparation left destination: %v", err)
		}
	})

	t.Run("collision preserves winner and exact mode is selected", func(t *testing.T) {
		dir := t.TempDir()
		root := &confinedFixtureRoot{dir: dir}
		destination := "artifact"
		if err := PublishConfined(root, destination, []byte("winner"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := PublishConfined(root, destination, []byte("loser"), 0o600); !errors.Is(err, os.ErrExist) {
			t.Fatalf("collision error = %v", err)
		}
		raw, err := os.ReadFile(filepath.Join(dir, destination))
		if err != nil || string(raw) != "winner" {
			t.Fatalf("winner bytes = %q, %v", raw, err)
		}
		info, err := os.Stat(filepath.Join(dir, destination))
		if err != nil || info.Mode().Perm() != 0o644 {
			t.Fatalf("winner mode = %v, %v", info, err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Name() != destination {
			t.Fatalf("publication residue = %v", entries)
		}
	})
}

func TestPublishConfinedNegativeBackendPaths(t *testing.T) {
	t.Run("joins publication and cleanup failures without leaking", func(t *testing.T) {
		dir := t.TempDir()
		publishErr := errors.New("publish failed")
		cleanupErr := errors.New("cleanup failed")
		root := &confinedFixtureRoot{dir: dir, linkErr: publishErr, removeErr: cleanupErr}
		err := PublishConfined(root, "artifact", []byte("complete"), 0o644)
		if !errors.Is(err, publishErr) || !errors.Is(err, cleanupErr) {
			t.Fatalf("joined error = %v; want publication and cleanup identities", err)
		}
		entries, readErr := os.ReadDir(dir)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if len(entries) != 0 {
			t.Fatalf("failed publication residue = %v", entries)
		}
	})

	t.Run("exhausts temporary collisions without leaking", func(t *testing.T) {
		dir := t.TempDir()
		root := &confinedFixtureRoot{dir: dir, openErr: os.ErrExist}
		err := PublishConfined(root, "artifact", []byte("complete"), 0o644)
		if err == nil || !strings.Contains(err.Error(), "temporary name collisions exhausted") {
			t.Fatalf("collision exhaustion error = %v", err)
		}
		if root.openCalls != 100 {
			t.Fatalf("temporary open attempts = %d, want 100", root.openCalls)
		}
		entries, readErr := os.ReadDir(dir)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if len(entries) != 0 {
			t.Fatalf("collision exhaustion residue = %v", entries)
		}
	})
}

func TestPublishConcurrentPublishersLeaveOneCompleteWinner(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "artifact")
	contents := []string{"first complete artifact", "second complete artifact"}
	start := make(chan struct{})
	results := make(chan error, len(contents))
	var ready sync.WaitGroup
	ready.Add(len(contents))
	for _, body := range contents {
		go func() {
			ready.Done()
			<-start
			results <- Publish(destination, []byte(body), 0o644)
		}()
	}
	ready.Wait()
	close(start)

	var successes int
	for range contents {
		err := <-results
		if err == nil {
			successes++
			continue
		}
		if !errors.Is(err, os.ErrExist) {
			t.Fatalf("publisher error = %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful publishers = %d, want 1", successes)
	}
	raw, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != contents[0] && string(raw) != contents[1] {
		t.Fatalf("winner bytes = %q", raw)
	}
}
