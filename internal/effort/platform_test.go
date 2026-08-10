package effort

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/filepublication"
)

func testPlatformPublicationContract(t *testing.T) {
	t.Helper()
	t.Run("creation refuses raced destination", func(t *testing.T) {
		dir := t.TempDir()
		destination := filepath.Join(dir, "record.json")
		writeEffortFile(t, destination, "unexpected")
		if err := filepublication.Publish(destination, []byte("new"), 0o600); err == nil {
			t.Fatal("creation replaced an existing destination")
		}
		if raw, err := os.ReadFile(destination); err != nil || string(raw) != "unexpected" {
			t.Fatalf("raced creation destination = %q, %v", raw, err)
		}
	})
	t.Run("replacement restores raced destination", func(t *testing.T) {
		dir := t.TempDir()
		destination := filepath.Join(dir, "record.json")
		writeEffortFile(t, destination, "old")
		expected, err := lstatRegular(destination)
		if err != nil {
			t.Fatal(err)
		}
		raced := destination + ".raced"
		writeEffortFile(t, raced, "unexpected")
		if err := os.Rename(raced, destination); err != nil {
			t.Fatal(err)
		}
		temporary := filepath.Join(dir, "temporary")
		writeEffortFile(t, temporary, "new")
		if err := publishAtomic(temporary, destination, &expected); err == nil {
			t.Fatal("replacement accepted a raced destination")
		}
		if raw, err := os.ReadFile(destination); err != nil || string(raw) != "unexpected" {
			t.Fatalf("raced replacement destination = %q, %v", raw, err)
		}
	})
	t.Run("replacement publishes over expected identity", func(t *testing.T) {
		dir := t.TempDir()
		destination := filepath.Join(dir, "record.json")
		writeEffortFile(t, destination, "old")
		expected, err := lstatRegular(destination)
		if err != nil {
			t.Fatal(err)
		}
		temporary := filepath.Join(dir, "temporary")
		writeEffortFile(t, temporary, "new")
		if err := publishAtomic(temporary, destination, &expected); err != nil {
			t.Fatal(err)
		}
		if raw, err := os.ReadFile(destination); err != nil || string(raw) != "new" {
			t.Fatalf("replacement destination = %q, %v", raw, err)
		}
		if err := os.Remove(temporary); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	})
}

func TestPlatformPublicationContract(t *testing.T) {
	testPlatformPublicationContract(t)
}
