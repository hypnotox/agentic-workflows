package filepublication

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

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
func TestPublishConcurrentPublishersLeaveOneCompleteWinner(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "artifact")
	publications := make([]*prepared, 0, 2)
	for _, contents := range []string{"first complete artifact", "second complete artifact"} {
		publication, err := prepare(destination, []byte(contents), 0o644)
		if err != nil {
			t.Fatal(err)
		}
		publications = append(publications, publication)
		defer func() {
			if err := publication.cleanup(); err != nil {
				t.Error(err)
			}
		}()
	}

	start := make(chan struct{})
	results := make(chan error, len(publications))
	var ready sync.WaitGroup
	ready.Add(len(publications))
	for _, publication := range publications {
		go func() {
			ready.Done()
			<-start
			results <- publication.publish()
		}()
	}
	ready.Wait()
	close(start)

	var successes int
	for range publications {
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
	if string(raw) != "first complete artifact" && string(raw) != "second complete artifact" {
		t.Fatalf("winner bytes = %q", raw)
	}
}
