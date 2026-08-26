package effort

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
)

func TestRollbackCreationRetainsSameNameSuccessor(t *testing.T) {
	root := initEffortRepo(t)
	var reservation, original string
	service := openTestService(t, root, func(deps *Dependencies) {
		noTopology(deps)
		deps.UUID = func() (string, error) { return testIDA, nil }
		deps.Fault = func(stage string) error {
			if stage != "rollback.delete" {
				return nil
			}
			original = reservation + ".original"
			if err := os.Rename(reservation, original); err != nil {
				return err
			}
			if err := os.Mkdir(reservation, 0o700); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(reservation, "successor"), []byte("keep"), 0o600)
		}
	})
	record, err := service.New(testContext(t), NewInput{Slug: "rollback-successor", Title: "Rollback successor"})
	if err != nil {
		t.Fatal(err)
	}
	reservation = filepath.Join(root, ".awf", "efforts", tombstoneName(record))
	result, err := service.RollbackCreation(testContext(t), record)
	if !errors.Is(err, filesystem.ErrIdentityChanged) || !result.Reserved || result.Removed {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if got, readErr := os.ReadFile(filepath.Join(reservation, "successor")); readErr != nil || string(got) != "keep" {
		t.Fatalf("successor mutated: bytes=%q err=%v", got, readErr)
	}
	if _, statErr := os.Stat(original); statErr != nil {
		t.Fatalf("original reservation lost: %v", statErr)
	}
}
