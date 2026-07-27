package effort

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

// invariant: tooling/effort-management:effort-record-authority
func TestEffortRepairUsesUniqueNativeGitRegistrationTruth(t *testing.T) {
	now := time.Date(2026, 7, 27, 2, 0, 0, 0, time.UTC)
	t.Run("present-checkout-without-authoritative-base-evidence", func(t *testing.T) {
		root := initEffortRepo(t)
		service := openEffortService(t, root, now)
		if _, err := service.New("Repair me", false); err != nil {
			t.Fatal(err)
		}
		managed := service.paths.managedWorktree(idA)
		if err := os.MkdirAll(filepath.Dir(managed), 0o700); err != nil {
			t.Fatal(err)
		}
		runEffortGit(t, "-C", root, "worktree", "add", "-b", "awf/"+idA, managed, "HEAD")
		writeEffortFile(t, filepath.Join(managed, "tip.txt"), "tip\n")
		runEffortGit(t, "-C", managed, "add", "tip.txt")
		runEffortGit(t, "-C", managed, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "advance tip")
		before, err := os.ReadFile(service.paths.record(idA))
		if err != nil {
			t.Fatal(err)
		}
		requireRepairSafety(t, service, "repair-evidence")
		after, err := os.ReadFile(service.paths.record(idA))
		if err != nil || string(after) != string(before) {
			t.Fatalf("refused reconstruction changed record: before=%q after=%q err=%v", before, after, err)
		}
	})

	t.Run("stale-registration-with-absent-managed-path", func(t *testing.T) {
		root := initEffortRepo(t)
		service := openEffortService(t, root, now)
		if _, err := service.New("Repair me", false); err != nil {
			t.Fatal(err)
		}
		managed := service.paths.managedWorktree(idA)
		if err := os.MkdirAll(filepath.Dir(managed), 0o700); err != nil {
			t.Fatal(err)
		}
		runEffortGit(t, "-C", root, "worktree", "add", "-b", "awf/"+idA, managed, "HEAD")
		if err := os.RemoveAll(managed); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(service.paths.record(idA))
		if err != nil {
			t.Fatal(err)
		}
		requireRepairSafety(t, service, "repair-evidence")
		after, err := os.ReadFile(service.paths.record(idA))
		if err != nil || string(after) != string(before) {
			t.Fatalf("stale registration repair changed record: before=%q after=%q err=%v", before, after, err)
		}
	})

	t.Run("stale-registration-retains-existing-authoritative-metadata", func(t *testing.T) {
		root := initEffortRepo(t)
		service := openEffortService(t, root, now)
		if _, err := service.New("Repair me", false); err != nil {
			t.Fatal(err)
		}
		managed := service.paths.managedWorktree(idA)
		if err := os.MkdirAll(filepath.Dir(managed), 0o700); err != nil {
			t.Fatal(err)
		}
		runEffortGit(t, "-C", root, "worktree", "add", "-b", "awf/"+idA, managed, "HEAD")
		writeEffortFile(t, service.paths.record(idA), schemaRecordJSON(now, worktreeJSON(now), "pending"))
		before, err := os.ReadFile(service.paths.record(idA))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(managed); err != nil {
			t.Fatal(err)
		}
		repaired, err := service.Repair(idA)
		if err != nil || len(repaired.Changes) != 0 || repaired.Record.Worktree == nil || repaired.Record.Worktree.Base != strings.Repeat("a", 40) {
			t.Fatalf("stale registration changed existing metadata: result=%#v err=%v", repaired, err)
		}
		after, err := os.ReadFile(service.paths.record(idA))
		if err != nil || string(after) != string(before) {
			t.Fatalf("stale registration rewrote record: before=%q after=%q err=%v", before, after, err)
		}
	})

	t.Run("native-git-moved-managed-branch", func(t *testing.T) {
		root := initEffortRepo(t)
		service := openEffortService(t, root, now)
		if _, err := service.New("Repair me", false); err != nil {
			t.Fatal(err)
		}
		managed := service.paths.managedWorktree(idA)
		if err := os.MkdirAll(filepath.Dir(managed), 0o700); err != nil {
			t.Fatal(err)
		}
		runEffortGit(t, "-C", root, "worktree", "add", "-b", "awf/"+idA, managed, "HEAD")
		writeEffortFile(t, service.paths.record(idA), schemaRecordJSON(now, worktreeJSON(now), "pending"))
		before, err := os.ReadFile(service.paths.record(idA))
		if err != nil {
			t.Fatal(err)
		}
		moved := filepath.Join(root, ".awf", "moved-worktree")
		runEffortGit(t, "-C", root, "worktree", "move", managed, moved)
		requireRepairSafety(t, service, "repository-identity")
		after, err := os.ReadFile(service.paths.record(idA))
		if err != nil || string(after) != string(before) {
			t.Fatalf("moved worktree repair changed record: before=%q after=%q err=%v", before, after, err)
		}
		if info, err := os.Stat(moved); err != nil || !info.IsDir() {
			t.Fatalf("moved worktree did not survive refusal: info=%v err=%v", info, err)
		}
	})

	t.Run("symlinked-managed-path", func(t *testing.T) {
		root := initEffortRepo(t)
		service := openEffortService(t, root, now)
		if _, err := service.New("Repair me", false); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(service.paths.managedWorktree(idA)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(t.TempDir(), service.paths.managedWorktree(idA)); err != nil {
			t.Skip(err)
		}
		requireRepairSafety(t, service, "symlink")
	})

	t.Run("unregistered-directory", func(t *testing.T) {
		root := initEffortRepo(t)
		service := openEffortService(t, root, now)
		if _, err := service.New("Repair me", false); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(service.paths.managedWorktree(idA), 0o700); err != nil {
			t.Fatal(err)
		}
		requireRepairSafety(t, service, "repository-identity")
	})

	t.Run("branch-mismatch", func(t *testing.T) {
		root := initEffortRepo(t)
		service := openEffortService(t, root, now)
		if _, err := service.New("Repair me", false); err != nil {
			t.Fatal(err)
		}
		managed := service.paths.managedWorktree(idA)
		if err := os.MkdirAll(filepath.Dir(managed), 0o700); err != nil {
			t.Fatal(err)
		}
		runEffortGit(t, "-C", root, "worktree", "add", "-b", "not-managed", managed, "HEAD")
		requireRepairSafety(t, service, "repository-identity")
	})

	t.Run("ambiguous-registration", func(t *testing.T) {
		root := initEffortRepo(t)
		managed := filepath.Join(root, ".awf", "worktrees", idA)
		registration := awfgit.WorktreeRegistration{Path: managed, HEAD: strings.Repeat("a", 40), Branch: "refs/heads/awf/" + idA}
		service, err := Open(t.Context(), root, Options{Clock: func() time.Time { return now }, UUID: func() (string, error) { return idA, nil }, Worktrees: func(context.Context, string) ([]awfgit.WorktreeRegistration, error) {
			return []awfgit.WorktreeRegistration{registration, registration}, nil
		}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.New("Repair me", false); err != nil {
			t.Fatal(err)
		}
		requireRepairSafety(t, service, "repository-identity")
	})

	t.Run("registered-path-control-root-refusal", func(t *testing.T) {
		root := initEffortRepo(t)
		managed := filepath.Join(root, ".awf", "worktrees", idA)
		registration := awfgit.WorktreeRegistration{Path: managed, HEAD: strings.Repeat("a", 40), Branch: "refs/heads/awf/" + idA}
		service, err := Open(t.Context(), root, Options{Clock: func() time.Time { return now }, UUID: func() (string, error) { return idA, nil }, Worktrees: func(context.Context, string) ([]awfgit.WorktreeRegistration, error) {
			return []awfgit.WorktreeRegistration{registration}, nil
		}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.New("Repair me", false); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(managed, 0o700); err != nil {
			t.Fatal(err)
		}
		writeEffortFile(t, filepath.Join(managed, ".git"), "not a gitdir pointer\n")
		requireRepairSafety(t, service, "repository-identity")
	})

	t.Run("inaccessible-managed-path", func(t *testing.T) {
		if testCurrentEUID() == 0 {
			t.Skip("root bypasses directory search permissions")
		}
		root := initEffortRepo(t)
		service := openEffortService(t, root, now)
		if _, err := service.New("Repair me", false); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(service.paths.worktrees, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(service.paths.worktrees, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(service.paths.worktrees, 0o700) })
		if _, err := service.Repair(idA); err == nil || !strings.Contains(err.Error(), "lstat managed worktree") {
			t.Fatalf("inaccessible managed path error = %v", err)
		}
	})

	t.Run("foreign-owned-managed-path", func(t *testing.T) {
		if testCurrentEUID() != 0 {
			t.Skip("foreign ownership fixture requires root")
		}
		root := initEffortRepo(t)
		service := openEffortService(t, root, now)
		if _, err := service.New("Repair me", false); err != nil {
			t.Fatal(err)
		}
		managed := service.paths.managedWorktree(idA)
		if err := os.MkdirAll(managed, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := testChown(managed, 1); err != nil {
			t.Skip(err)
		}
		t.Cleanup(func() { _ = testChown(managed, testCurrentEUID()) })
		requireRepairSafety(t, service, "foreign-owner")
	})

	t.Run("repository-identity-mismatch", func(t *testing.T) {
		root := initEffortRepo(t)
		managed := filepath.Join(root, ".awf", "worktrees", idA)
		registration := awfgit.WorktreeRegistration{Path: managed, HEAD: strings.Repeat("a", 40), Branch: "refs/heads/awf/" + idA}
		service, err := Open(t.Context(), root, Options{Clock: func() time.Time { return now }, UUID: func() (string, error) { return idA, nil }, Worktrees: func(context.Context, string) ([]awfgit.WorktreeRegistration, error) {
			return []awfgit.WorktreeRegistration{registration}, nil
		}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.New("Repair me", false); err != nil {
			t.Fatal(err)
		}
		runEffortGit(t, "init", managed)
		writeEffortFile(t, filepath.Join(managed, "foreign.txt"), "foreign\n")
		runEffortGit(t, "-C", managed, "add", "foreign.txt")
		runEffortGit(t, "-C", managed, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "foreign")
		requireRepairSafety(t, service, "repository-identity")
	})
}

func requireRepairSafety(t *testing.T, service *Service, category string) {
	t.Helper()
	_, err := service.Repair(idA)
	var hard *awfgit.HardSafetyError
	if !errors.As(err, &hard) || hard.Category != category || hard.Forceable() {
		t.Fatalf("repair safety error = %T %v", err, err)
	}
}
