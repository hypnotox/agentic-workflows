package effort

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEffortServiceRefusalsAndRepairWorktreeTruth(t *testing.T) {
	root := initEffortRepo(t)
	now := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	service := openEffortService(t, root, now)
	if _, err := service.New("Active", false); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reopen(idA); err == nil {
		t.Fatal("active reopen accepted")
	}
	if _, err := service.Complete(idA); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Complete(idA); err == nil {
		t.Fatal("terminal completion accepted")
	}
	if _, err := service.Rename(idA, " "); err == nil {
		t.Fatal("blank rename accepted")
	}

	recordPath := filepath.Join(root, ".awf", "efforts", idA+".json")
	writeEffortFile(t, recordPath, schemaRecordJSON(now, worktreeJSON(now), "pending"))
	repaired, err := service.Repair(idA)
	if err != nil {
		t.Fatal(err)
	}
	if len(repaired.Changes) != 2 || repaired.Record.Worktree != nil || repaired.Record.Integration != IntegrationNone {
		t.Fatalf("missing-worktree repair = %#v", repaired)
	}
	writeEffortFile(t, recordPath, schemaRecordJSON(now, worktreeJSON(now), "pending"))
	managed := filepath.Join(root, ".awf", "worktrees", idA)
	if err := os.MkdirAll(filepath.Dir(managed), 0o700); err != nil {
		t.Fatal(err)
	}
	runEffortGit(t, "-C", root, "worktree", "add", "-b", "awf/"+idA, managed, "HEAD")
	unchanged, err := service.Repair(idA)
	if err != nil || len(unchanged.Changes) != 0 || unchanged.Record.Worktree == nil {
		t.Fatalf("present worktree repair = %#v, %v", unchanged, err)
	}
	runEffortGit(t, "-C", root, "worktree", "remove", managed)
	if err := os.WriteFile(managed, []byte("unsafe"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Repair(idA); err == nil {
		t.Fatal("unsafe worktree path accepted")
	}
}

func TestEffortAllocationAndAssignmentRefusals(t *testing.T) {
	root := initEffortRepo(t)
	empty := openEffortService(t, root, time.Now().UTC())
	if records, err := empty.List(); err != nil || len(records) != 0 {
		t.Fatalf("empty list = %#v, %v", records, err)
	}
	service, err := Open(t.Context(), root, Options{UUID: func() (string, error) { return "", errors.New("entropy") }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.New("Entropy", false); err == nil || !strings.Contains(err.Error(), "entropy") {
		t.Fatalf("allocator error = %v", err)
	}
	service, err = Open(t.Context(), root, Options{UUID: func() (string, error) { return "bad", nil }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.New("Invalid UUID", false); err == nil {
		t.Fatal("invalid allocator UUID accepted")
	}
	collisionRoot := initEffortRepo(t)
	collisionService := openEffortService(t, collisionRoot, time.Now().UTC())
	if _, err := collisionService.New("First", false); err != nil {
		t.Fatal(err)
	}
	if _, err := collisionService.New("Exhaust", false); err == nil || !strings.Contains(err.Error(), "128 collisions") {
		t.Fatalf("collision exhaustion = %v", err)
	}
}
