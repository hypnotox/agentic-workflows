package effort

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

func TestEffortRepairUsesUniqueNativeGitRegistrationTruth(t *testing.T) {
	now := time.Date(2026, 7, 27, 2, 0, 0, 0, time.UTC)
	t.Run("reconstruction-registered-present-and-missing-path-retention", func(t *testing.T) {
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
		head := stringsTrim(runEffortGitOutput(t, "-C", managed, "rev-parse", "HEAD"))
		repaired, err := service.Repair(idA)
		if err != nil {
			t.Fatal(err)
		}
		if repaired.Record.Worktree == nil || repaired.Record.Worktree.Base != head || !repaired.Record.Worktree.AttachedAt.Equal(now) || repaired.Record.Integration != IntegrationPending || len(repaired.Changes) != 2 {
			t.Fatalf("reconstruction = %#v", repaired)
		}
		if err := os.RemoveAll(managed); err != nil {
			t.Fatal(err)
		}
		retained, err := service.Repair(idA)
		if err != nil || retained.Record.Worktree == nil || len(retained.Changes) != 0 {
			t.Fatalf("registered missing-path retention = %#v, %v", retained, err)
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
		registration := awfgit.WorktreeRegistration{Path: managed, HEAD: stringsRepeat("a", 40), Branch: "refs/heads/awf/" + idA}
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
		registration := awfgit.WorktreeRegistration{Path: managed, HEAD: stringsRepeat("a", 40), Branch: "refs/heads/awf/" + idA}
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
		if os.Geteuid() == 0 {
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
		if os.Geteuid() != 0 {
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
		if err := os.Chown(managed, 1, -1); err != nil {
			t.Skip(err)
		}
		t.Cleanup(func() { _ = os.Chown(managed, os.Geteuid(), -1) })
		requireRepairSafety(t, service, "foreign-owner")
	})

	t.Run("repository-identity-mismatch", func(t *testing.T) {
		root := initEffortRepo(t)
		managed := filepath.Join(root, ".awf", "worktrees", idA)
		registration := awfgit.WorktreeRegistration{Path: managed, HEAD: stringsRepeat("a", 40), Branch: "refs/heads/awf/" + idA}
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

func runEffortGitOutput(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(output)
}

func stringsTrim(value string) string              { return strings.TrimSpace(value) }
func stringsRepeat(value string, count int) string { return strings.Repeat(value, count) }
