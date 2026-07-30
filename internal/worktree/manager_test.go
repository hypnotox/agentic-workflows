package worktree

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/effort"
)

const worktreeTestID = "018f47a0-7b3d-4c52-8f1a-123456789abc"

func TestManagedWorktreeAddIntegrateAndRestartableRemove(t *testing.T) {
	// invariant: tooling/effort-management:managed-worktree-lifecycle
	root := initWorktreeRepo(t, "sha1")
	createEffort(t, root, "Managed result")
	manager, err := Open(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	added, err := manager.Add("managed-result", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if !added.ChangedTopology || !strings.Contains(added.String(), "changed topology: yes") {
		t.Fatalf("add result = %#v", added)
	}
	managed := filepath.Join(root, ".awf", "worktrees", "managed-result")
	writeWorktreeFile(t, filepath.Join(managed, "effort.txt"), "effort\n")
	commitWorktree(t, managed, "effort")

	integrated, err := manager.Integrate("managed-result", "")
	if err != nil {
		t.Fatal(err)
	}
	if !integrated.ChangedTopology || !strings.Contains(integrated.Condition, "fast-forwarded") {
		t.Fatalf("integrate result = %#v", integrated)
	}
	already, err := manager.Integrate("managed-result", "")
	if err != nil {
		t.Fatal(err)
	}
	if already.ChangedTopology || !strings.Contains(already.Condition, "already integrated") {
		t.Fatalf("already result = %#v", already)
	}

	removed, err := manager.Remove("managed-result")
	if err != nil {
		t.Fatal(err)
	}
	if !removed.ChangedTopology {
		t.Fatalf("remove result = %#v", removed)
	}
	if _, err := os.Lstat(managed); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("managed path remains: %v", err)
	}
	if commandSucceeds(root, "show-ref", "--verify", "--quiet", "refs/heads/awf/managed-result") {
		t.Fatal("managed branch remains")
	}
	again, err := manager.Remove("managed-result")
	if err != nil {
		t.Fatal(err)
	}
	if again.ChangedTopology {
		t.Fatalf("idempotent remove changed topology: %#v", again)
	}
}

func TestDivergentIntegrationStopsBeforeCommit(t *testing.T) {
	root := initWorktreeRepo(t, "sha1")
	createEffort(t, root, "Divergent result")
	manager, err := Open(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add("divergent-result", "HEAD"); err != nil {
		t.Fatal(err)
	}
	managed := filepath.Join(root, ".awf", "worktrees", "divergent-result")
	writeWorktreeFile(t, filepath.Join(managed, "effort.txt"), "effort\n")
	commitWorktree(t, managed, "effort")
	writeWorktreeFile(t, filepath.Join(root, "target.txt"), "target\n")
	commitWorktree(t, root, "target")

	result, err := manager.Integrate("divergent-result", "")
	if err != nil {
		t.Fatal(err)
	}
	if !result.ChangedTopology || !strings.Contains(result.Condition, "staged without a commit") ||
		!strings.Contains(result.NextAction, "check --staged") || !strings.Contains(result.NextAction, "project gate") ||
		strings.Contains(result.NextAction, "./x gate") {
		t.Fatalf("divergent result = %#v", result)
	}
	mergeHead := strings.TrimSpace(runWorktreeGit(t, root, "rev-parse", "--git-path", "MERGE_HEAD"))
	if !filepath.IsAbs(mergeHead) {
		mergeHead = filepath.Join(root, mergeHead)
	}
	if _, err := os.Stat(mergeHead); err != nil {
		t.Fatalf("MERGE_HEAD absent: %v", err)
	}
	runWorktreeGit(t, root, "merge", "--abort")
}

func TestIntegrationConflictAndUnrelatedHistoryStayVisibleAndActionable(t *testing.T) {
	t.Run("conflict", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		createEffort(t, root, "Conflict result")
		manager, err := Open(context.Background(), root, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := manager.Add("conflict-result", "HEAD"); err != nil {
			t.Fatal(err)
		}
		managed := filepath.Join(root, ".awf", "worktrees", "conflict-result")
		writeWorktreeFile(t, filepath.Join(managed, "tracked.txt"), "effort\n")
		commitWorktree(t, managed, "effort conflict")
		writeWorktreeFile(t, filepath.Join(root, "tracked.txt"), "target\n")
		commitWorktree(t, root, "target conflict")
		_, err = manager.Integrate("conflict-result", "make gate")
		if err == nil || !strings.Contains(err.Error(), "changed topology: yes") || !strings.Contains(err.Error(), "resolve or abort") ||
			!strings.Contains(err.Error(), "`make gate`") || strings.Contains(err.Error(), "./x gate") {
			t.Fatalf("conflict error = %v", err)
		}
		if !commandSucceeds(root, "rev-parse", "--verify", "MERGE_HEAD") {
			t.Fatal("conflict merge state was hidden")
		}
		runWorktreeGit(t, root, "merge", "--abort")
	})

	t.Run("unrelated", func(t *testing.T) {
		root := initWorktreeRepo(t, "sha1")
		createEffort(t, root, "Unrelated result")
		manager, err := Open(context.Background(), root, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := manager.Add("unrelated-result", "HEAD"); err != nil {
			t.Fatal(err)
		}
		managed := filepath.Join(root, ".awf", "worktrees", "unrelated-result")
		runWorktreeGit(t, managed, "checkout", "--orphan", "unrelated-temp")
		runWorktreeGit(t, managed, "rm", "-rf", ".")
		writeWorktreeFile(t, filepath.Join(managed, "orphan.txt"), "orphan\n")
		commitWorktree(t, managed, "orphan")
		runWorktreeGit(t, root, "branch", "-f", "awf/unrelated-result", "unrelated-temp")
		runWorktreeGit(t, managed, "checkout", "awf/unrelated-result")
		before := runWorktreeGit(t, root, "rev-parse", "HEAD")
		_, err = manager.Integrate("unrelated-result", "")
		if err == nil || !strings.Contains(err.Error(), "no proven common ancestor") || !strings.Contains(err.Error(), "changed topology: no") || !strings.Contains(err.Error(), "do not use --allow-unrelated-histories") {
			t.Fatalf("unrelated error = %v", err)
		}
		if after := runWorktreeGit(t, root, "rev-parse", "HEAD"); after != before {
			t.Fatalf("target changed from %s to %s", before, after)
		}
	})
}

func TestRemovalRefusesDirtyAndUnmergedWithoutForce(t *testing.T) {
	root := initWorktreeRepo(t, "sha1")
	createEffort(t, root, "Guard removal")
	manager, err := Open(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add("guard-removal", "HEAD"); err != nil {
		t.Fatal(err)
	}
	managed := filepath.Join(root, ".awf", "worktrees", "guard-removal")
	writeWorktreeFile(t, filepath.Join(managed, "dirty.txt"), "dirty\n")
	_, err = manager.Remove("guard-removal")
	if err == nil || !strings.Contains(err.Error(), "not merged") {
		// With no effort commit the branch is merged, so cleanliness is the first
		// destructive precondition after identity.
		if err == nil || !strings.Contains(err.Error(), "dirty") {
			t.Fatalf("dirty removal error = %v", err)
		}
	}
	if err := os.Remove(filepath.Join(managed, "dirty.txt")); err != nil {
		t.Fatal(err)
	}
	writeWorktreeFile(t, filepath.Join(managed, "effort.txt"), "unmerged\n")
	commitWorktree(t, managed, "unmerged")
	_, err = manager.Remove("guard-removal")
	if err == nil || !strings.Contains(err.Error(), "not merged") || !strings.Contains(err.Error(), "native Git") {
		t.Fatalf("unmerged removal error = %v", err)
	}
	if _, statErr := os.Stat(managed); statErr != nil {
		t.Fatalf("managed worktree was discarded: %v", statErr)
	}
}

func TestAddFailureReportsActualTopologyAndPreservesEffort(t *testing.T) {
	root := initWorktreeRepo(t, "sha1")
	createEffort(t, root, "Partial add")
	runner := func(ctx context.Context, directory string, args ...string) ([]byte, error) {
		out, err := nativeRunner(ctx, directory, args...)
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" && err == nil {
			return nil, errors.New("injected post-add failure")
		}
		return out, err
	}
	manager, err := Open(context.Background(), root, Options{Runner: runner})
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Add("partial-add", "HEAD")
	if err == nil || !strings.Contains(err.Error(), "changed topology: yes") || !strings.Contains(err.Error(), "actual Git topology") {
		t.Fatalf("add failure = %v", err)
	}
	service, err := effort.Open(context.Background(), root, effort.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Show("partial-add"); err != nil {
		t.Fatalf("complete effort changed by add failure: %v", err)
	}
}

func TestResolveAcceptsSHA1AndSHA256ObjectIDs(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			root := initWorktreeRepo(t, format)
			id, err := resolve(context.Background(), nativeRunner, root, "HEAD")
			if err != nil {
				if format == "sha256" && strings.Contains(err.Error(), "unknown value") {
					t.Skip("installed Git lacks SHA-256 repositories")
				}
				t.Fatal(err)
			}
			want := 40
			if format == "sha256" {
				want = 64
			}
			if len(id) != want {
				t.Fatalf("object ID length = %d, want %d", len(id), want)
			}
		})
	}
}

func TestGitHelperErrorAndCleanlinessBranches(t *testing.T) {
	root := initWorktreeRepo(t, "sha1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := nativeRunner(ctx, root, "status"); err == nil {
		t.Fatal("cancelled native runner succeeded")
	}
	failed := func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("runner") }
	if _, err := resolve(context.Background(), failed, ".", "HEAD"); err == nil {
		t.Fatal("resolve runner error hidden")
	}
	invalidID := func(context.Context, string, ...string) ([]byte, error) { return []byte("short\n"), nil }
	if _, err := resolve(context.Background(), invalidID, ".", "HEAD"); err == nil {
		t.Fatal("invalid object ID accepted")
	}
	if err := status(context.Background(), failed, "."); err == nil {
		t.Fatal("status runner error hidden")
	}
	cleanResident := func(context.Context, string, ...string) ([]byte, error) {
		return []byte("?? .awf/efforts/x/state.json\x00"), nil
	}
	if err := status(context.Background(), cleanResident, "."); err != nil {
		t.Fatalf("owned resident treated as dirt: %v", err)
	}
	dirty := func(context.Context, string, ...string) ([]byte, error) { return []byte("?? foreign\x00"), nil }
	if err := status(context.Background(), dirty, "."); err == nil {
		t.Fatal("foreign dirt accepted")
	}
	if exists, err := branchExists(context.Background(), failed, ".", "x"); err == nil || exists {
		t.Fatalf("branch runner result exists=%v err=%v", exists, err)
	}
	if related, err := ancestor(context.Background(), failed, ".", "a", "b"); err == nil || related {
		t.Fatalf("ancestor runner result related=%v err=%v", related, err)
	}
}

func TestManagerValidationAndOperationRefusals(t *testing.T) {
	root := initWorktreeRepo(t, "sha1")
	createEffort(t, root, "Validation result")
	manager, err := Open(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	foreign := initWorktreeRepo(t, "sha1")
	if err := manager.validateManagedTarget(foreign); err == nil {
		t.Fatal("foreign managed target accepted")
	}
	original := manager.roots.InvokingRoot
	manager.roots.InvokingRoot = filepath.Join(root, "missing")
	if err := manager.validateLiveInvokingCheckout(); err == nil {
		t.Fatal("missing invoking checkout accepted")
	}
	manager.roots.InvokingRoot = foreign
	if err := manager.validateLiveInvokingCheckout(); err == nil {
		t.Fatal("foreign invoking checkout accepted")
	}
	manager.roots.InvokingRoot = original

	for _, operation := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "rebase-merge", "rebase-apply"} {
		t.Run(operation, func(t *testing.T) {
			candidate := filepath.Join(t.TempDir(), operation)
			if err := os.WriteFile(candidate, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			manager.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
				if args[len(args)-1] == operation {
					return []byte(candidate + "\n"), nil
				}
				return []byte(filepath.Join(t.TempDir(), args[len(args)-1]) + "\n"), nil
			}
			if err := manager.operationFree(root); err == nil || !strings.Contains(err.Error(), "in-progress") {
				t.Fatalf("operation error = %v", err)
			}
		})
	}
	manager.run = failedRunner
	if err := manager.operationFree(root); err == nil {
		t.Fatal("operation probe error hidden")
	}
}

func failedRunner(context.Context, string, ...string) ([]byte, error) {
	return nil, errors.New("runner")
}

func TestManagerAuthorityErrorBranches(t *testing.T) {
	if _, err := Open(context.Background(), filepath.Join(t.TempDir(), "missing"), Options{}); err == nil {
		t.Fatal("missing repository accepted")
	}
	m, _ := newManagerWithEffort(t, "Authority errors")
	original := m.roots
	m.roots.PrimaryRoot = "relative"
	if _, err := m.managed("authority-errors"); err == nil {
		t.Fatal("invalid managed root accepted")
	}
	if err := m.validateManagedTarget(t.TempDir()); err == nil {
		t.Fatal("invalid resident authority accepted")
	}
	m.roots = original
	plain := t.TempDir()
	if err := m.validateManagedTarget(plain); err == nil || !strings.Contains(err.Error(), "repository-identity") {
		t.Fatalf("plain managed target error = %v", err)
	}
	m.roots.InvokingRoot = plain
	if err := m.validateLiveInvokingCheckout(); err == nil || !strings.Contains(err.Error(), "repository-identity") {
		t.Fatalf("plain invoking error = %v", err)
	}
}

func TestAddPreconditionAndRunnerFailureBranches(t *testing.T) {
	t.Run("existing path", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "Add path")
		path := filepath.Join(root, ".awf", "worktrees", "add-path")
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := m.Add("add-path", "HEAD"); err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("existing branch", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "Add branch")
		runWorktreeGit(t, root, "branch", "awf/add-branch")
		if _, err := m.Add("add-branch", "HEAD"); err == nil || !strings.Contains(err.Error(), "branch already exists") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("registered branch", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "Add registered")
		base := m.run
		m.run = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
			if strings.Join(args, " ") == "worktree list --porcelain -z" {
				return []byte("worktree " + root + "\x00HEAD abc\x00branch refs/heads/main\x00\x00worktree /foreign\x00HEAD def\x00branch refs/heads/awf/add-registered\x00\x00"), nil
			}
			return base(ctx, dir, args...)
		}
		if _, err := m.Add("add-registered", "HEAD"); err == nil || !strings.Contains(err.Error(), "registration") {
			t.Fatalf("error = %v", err)
		}
	})
	for _, tc := range []struct {
		name, prefix string
	}{
		{"registrations", "worktree list"},
		{"branch probe", "show-ref"},
		{"operation", "rev-parse --git-path"},
		{"base", "rev-parse --verify"},
		{"add", "worktree add"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := newManagerWithEffort(t, "Add "+tc.name)
			base := m.run
			m.run = runnerFailingPrefix(base, tc.prefix)
			if _, err := m.Add("add-"+strings.ReplaceAll(tc.name, " ", "-"), "HEAD"); err == nil {
				t.Fatal("runner fault hidden")
			}
		})
	}
	t.Run("invalid effort and stat fault", func(t *testing.T) {
		m, _ := newManagerWithEffort(t, "Add stat fault")
		if _, err := m.Add("missing-effort", "HEAD"); err == nil {
			t.Fatal("missing effort accepted")
		}
		old := managedLstat
		managedLstat = func(string) (os.FileInfo, error) { return nil, errors.New("stat fault") }
		defer func() { managedLstat = old }()
		if _, err := m.Add("add-stat-fault", "HEAD"); err == nil || !strings.Contains(err.Error(), "stat fault") {
			t.Fatalf("stat error = %v", err)
		}
	})
	t.Run("topology registration probe", func(t *testing.T) {
		m, _ := newManagerWithEffort(t, "Topology probe")
		path, err := m.managed("topology-probe")
		if err != nil {
			t.Fatal(err)
		}
		old := managedLstat
		managedLstat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
		defer func() { managedLstat = old }()
		m.run = func(context.Context, string, ...string) ([]byte, error) {
			return []byte("worktree " + path + "\x00HEAD abc\x00branch refs/heads/awf/topology-probe\x00\x00"), nil
		}
		if !m.topologyPresent("topology-probe", path) {
			t.Fatal("registration topology was not observed")
		}
	})
	t.Run("post-add registration", func(t *testing.T) {
		m, _ := newManagerWithEffort(t, "Add post registration")
		base := m.run
		added := false
		m.run = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
			out, err := base(ctx, dir, args...)
			if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" && err == nil {
				added = true
			}
			if added && strings.Join(args, " ") == "worktree list --porcelain -z" {
				return []byte("worktree " + m.roots.InvokingRoot + "\x00HEAD abc\x00branch refs/heads/main\x00\x00"), nil
			}
			return out, err
		}
		if _, err := m.Add("add-post-registration", "HEAD"); err == nil || !strings.Contains(err.Error(), "exact managed registration") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestIntegratePreconditionAndMutationFailureBranches(t *testing.T) {
	t.Run("managed caller", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "Managed caller")
		if _, err := m.Add("managed-caller", "HEAD"); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, ".awf", "worktrees", "managed-caller")
		m.roots.InvokingRoot = path
		if _, err := m.Integrate("managed-caller", ""); err == nil || !strings.Contains(err.Error(), "receiving checkout") {
			t.Fatalf("error = %v", err)
		}
	})
	for _, tc := range []struct {
		name, prefix string
	}{
		{"registration", "worktree list"},
		{"operation", "rev-parse --git-path"},
		{"status", "status --porcelain"},
		{"detached", "symbolic-ref"},
		{"tip", "rev-parse --verify awf/integrate-tip"},
		{"target", "rev-parse --verify HEAD"},
		{"ancestor", "merge-base --is-ancestor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, root := newManagerWithEffort(t, "Integrate "+tc.name)
			slug := "integrate-" + tc.name
			if _, err := m.Add(slug, "HEAD"); err != nil {
				t.Fatal(err)
			}
			managed := filepath.Join(root, ".awf", "worktrees", slug)
			writeWorktreeFile(t, filepath.Join(managed, "change"), "x")
			commitWorktree(t, managed, "change")
			base := m.run
			m.run = runnerFailingPrefix(base, tc.prefix)
			if _, err := m.Integrate(slug, ""); err == nil {
				t.Fatal("runner fault hidden")
			}
		})
	}
	t.Run("effort target branch", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "Integrate own")
		if _, err := m.Add("integrate-own", "HEAD"); err != nil {
			t.Fatal(err)
		}
		base := m.run
		m.run = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
			if strings.Join(args, " ") == "symbolic-ref -q --short HEAD" {
				return []byte("awf/integrate-own\n"), nil
			}
			return base(ctx, dir, args...)
		}
		if _, err := m.Integrate("integrate-own", ""); err == nil || !strings.Contains(err.Error(), "effort branch") {
			t.Fatalf("error = %v root=%s", err, root)
		}
	})
	t.Run("fast-forward failure", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "Integrate ff failure")
		if _, err := m.Add("integrate-ff-failure", "HEAD"); err != nil {
			t.Fatal(err)
		}
		managed := filepath.Join(root, ".awf", "worktrees", "integrate-ff-failure")
		writeWorktreeFile(t, filepath.Join(managed, "change"), "x")
		commitWorktree(t, managed, "change")
		base := m.run
		m.run = runnerFailingPrefix(base, "merge --ff-only")
		if _, err := m.Integrate("integrate-ff-failure", ""); err == nil || !strings.Contains(err.Error(), "fast-forward failed") {
			t.Fatalf("error = %v", err)
		}
		before := runWorktreeGit(t, root, "rev-parse", "HEAD")
		if m.targetChanged(before) {
			t.Fatal("unchanged target reported changed")
		}
		if !m.targetChanged("0000000000000000000000000000000000000000") {
			t.Fatal("changed target reported unchanged")
		}
	})
}

func TestManagerMutationPropagationBranches(t *testing.T) {
	t.Run("add authority", func(t *testing.T) {
		m, _ := newManagerWithEffort(t, "Add authority")
		m.roots.PrimaryRoot = "relative"
		if _, err := m.Add("add-authority", "HEAD"); err == nil {
			t.Fatal("invalid add authority accepted")
		}
	})
	t.Run("add live identity", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "Add live identity")
		plain := t.TempDir()
		m.roots.InvokingRoot = plain
		base := m.run
		m.run = func(ctx context.Context, _ string, args ...string) ([]byte, error) {
			switch {
			case strings.Join(args, " ") == "worktree list --porcelain -z":
				return []byte("worktree " + root + "\x00HEAD abc\x00branch refs/heads/main\x00\x00"), nil
			case args[0] == "show-ref":
				return base(ctx, root, args...)
			case args[0] == "rev-parse" && args[1] == "--git-path":
				return []byte(filepath.Join(plain, args[2]) + "\n"), nil
			case args[0] == "rev-parse" && args[1] == "--verify":
				return []byte("0000000000000000000000000000000000000000\n"), nil
			default:
				return nil, errors.New("unexpected runner call")
			}
		}
		if _, err := m.Add("add-live-identity", "HEAD"); err == nil || !strings.Contains(err.Error(), "repository-identity") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("integrate authority and fact propagation", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "Integrate propagation")
		if _, err := m.Add("integrate-propagation", "HEAD"); err != nil {
			t.Fatal(err)
		}
		managed := filepath.Join(root, ".awf", "worktrees", "integrate-propagation")
		writeWorktreeFile(t, filepath.Join(managed, "change"), "x")
		commitWorktree(t, managed, "change")
		originalRoots := m.roots
		m.roots.PrimaryRoot = "relative"
		if _, err := m.Integrate("integrate-propagation", ""); err == nil {
			t.Fatal("invalid integrate authority accepted")
		}
		m.roots = originalRoots
		base := m.run
		calls := 0
		m.run = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
			if strings.HasPrefix(strings.Join(args, " "), "rev-parse --verify HEAD") {
				calls++
				if calls == 2 {
					return []byte("0000000000000000000000000000000000000000\n"), nil
				}
			}
			return base(ctx, dir, args...)
		}
		if _, err := m.Integrate("integrate-propagation", ""); err == nil || !strings.Contains(err.Error(), "target HEAD changed") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("validate fact prerequisites", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "Validate prerequisites")
		if _, err := m.Add("validate-prerequisites", "HEAD"); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, ".awf", "worktrees", "validate-prerequisites")
		target := runWorktreeGit(t, root, "rev-parse", "HEAD")
		tip := runWorktreeGit(t, root, "rev-parse", "awf/validate-prerequisites")
		original := m.roots
		m.roots.InvokingRoot = t.TempDir()
		if err := m.validateIntegrationFacts(path, "validate-prerequisites", target, tip); err == nil {
			t.Fatal("invalid invoking checkout accepted")
		}
		m.roots = original
		if err := m.validateIntegrationFacts(filepath.Join(root, "missing"), "validate-prerequisites", target, tip); err == nil {
			t.Fatal("missing managed target accepted")
		}
		base := m.run
		m.run = runnerFailingPrefix(base, "worktree list")
		if err := m.validateIntegrationFacts(path, "validate-prerequisites", target, tip); err == nil {
			t.Fatal("registration probe fault hidden")
		}
	})
	t.Run("remove authority and target", func(t *testing.T) {
		m, _ := newManagerWithEffort(t, "Remove propagation")
		if _, err := m.Remove("missing-effort"); err == nil {
			t.Fatal("missing effort accepted")
		}
		m.roots.PrimaryRoot = "relative"
		if _, err := m.Remove("remove-propagation"); err == nil {
			t.Fatal("invalid remove authority accepted")
		}
		m, root := newManagerWithEffort(t, "Remove target error")
		if _, err := m.Add("remove-target-error", "HEAD"); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, ".awf", "worktrees", "remove-target-error")
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := m.Remove("remove-target-error"); err == nil || !strings.Contains(err.Error(), "repository-identity") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestIntegrationAdditionalRefusalBranches(t *testing.T) {
	t.Run("missing effort and target", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "Integration missing target")
		if _, err := m.Integrate("missing-effort", ""); err == nil {
			t.Fatal("missing effort accepted")
		}
		if _, err := m.Add("integration-missing-target", "HEAD"); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, ".awf", "worktrees", "integration-missing-target")
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
		if _, err := m.Integrate("integration-missing-target", ""); err == nil {
			t.Fatal("missing target accepted")
		}
	})
	t.Run("second ancestry probe", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "Integration second ancestry")
		if _, err := m.Add("integration-second-ancestry", "HEAD"); err != nil {
			t.Fatal(err)
		}
		managed := filepath.Join(root, ".awf", "worktrees", "integration-second-ancestry")
		writeWorktreeFile(t, filepath.Join(managed, "change"), "x")
		commitWorktree(t, managed, "change")
		base := m.run
		calls := 0
		m.run = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
			if strings.HasPrefix(strings.Join(args, " "), "merge-base --is-ancestor") {
				calls++
				if calls == 2 {
					return nil, errors.New("second ancestry")
				}
			}
			return base(ctx, dir, args...)
		}
		if _, err := m.Integrate("integration-second-ancestry", ""); err == nil || !strings.Contains(err.Error(), "second ancestry") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestIntegrationFactDriftBranches(t *testing.T) {
	m, root := newManagerWithEffort(t, "Fact drift")
	if _, err := m.Add("fact-drift", "HEAD"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".awf", "worktrees", "fact-drift")
	target := runWorktreeGit(t, root, "rev-parse", "HEAD")
	tip := runWorktreeGit(t, root, "rev-parse", "awf/fact-drift")
	if err := m.validateIntegrationFacts(path, "fact-drift", "0000000000000000000000000000000000000000", tip); err == nil || !strings.Contains(err.Error(), "target HEAD changed") {
		t.Fatalf("target drift error = %v", err)
	}
	if err := m.validateIntegrationFacts(path, "fact-drift", target, "0000000000000000000000000000000000000000"); err == nil || !strings.Contains(err.Error(), "effort branch changed") {
		t.Fatalf("tip drift error = %v", err)
	}
	base := m.run
	m.run = runnerFailingPrefix(base, "rev-parse --verify HEAD")
	if err := m.validateIntegrationFacts(path, "fact-drift", target, tip); err == nil || !strings.Contains(err.Error(), "target HEAD changed") {
		t.Fatalf("target resolve error = %v", err)
	}
	m.run = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		if strings.HasPrefix(strings.Join(args, " "), "rev-parse --verify awf/fact-drift") {
			return nil, errors.New("tip resolve")
		}
		return base(ctx, dir, args...)
	}
	if err := m.validateIntegrationFacts(path, "fact-drift", target, tip); err == nil || !strings.Contains(err.Error(), "effort branch changed") {
		t.Fatalf("tip resolve error = %v", err)
	}
}

func TestRemovalPartialTopologyAndFailureBranches(t *testing.T) {
	t.Run("target operation", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "Remove operation")
		if _, err := m.Add("remove-operation", "HEAD"); err != nil {
			t.Fatal(err)
		}
		mergeHead := strings.TrimSpace(runWorktreeGit(t, root, "rev-parse", "--git-path", "MERGE_HEAD"))
		if !filepath.IsAbs(mergeHead) {
			mergeHead = filepath.Join(root, mergeHead)
		}
		if err := os.WriteFile(mergeHead, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := m.Remove("remove-operation"); err == nil || !strings.Contains(err.Error(), "in-progress") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("remove and branch failures", func(t *testing.T) {
		for _, prefix := range []string{"worktree remove", "branch -d"} {
			t.Run(prefix, func(t *testing.T) {
				m, root := newManagerWithEffort(t, "Remove "+prefix)
				slug := "remove-" + strings.Trim(strings.ReplaceAll(strings.ReplaceAll(prefix, " ", "-"), "--", "-"), "-")
				if _, err := m.Add(slug, "HEAD"); err != nil {
					t.Fatal(err)
				}
				base := m.run
				if prefix == "branch -d" {
					path := filepath.Join(root, ".awf", "worktrees", slug)
					runWorktreeGit(t, root, "worktree", "remove", path)
				}
				m.run = runnerFailingPrefix(base, prefix)
				if _, err := m.Remove(slug); err == nil {
					t.Fatal("mutation fault hidden")
				}
			})
		}
	})
	t.Run("runner probes", func(t *testing.T) {
		for _, prefix := range []string{"status --porcelain", "worktree list", "show-ref", "merge-base --is-ancestor"} {
			t.Run(prefix, func(t *testing.T) {
				m, root := newManagerWithEffort(t, "Remove probe "+prefix)
				slug := map[string]string{
					"status --porcelain":       "remove-probe-status-porcelain",
					"worktree list":            "remove-probe-worktree-list",
					"show-ref":                 "remove-probe-show-ref",
					"merge-base --is-ancestor": "remove-probe-merge-base-is-ancestor",
				}[prefix]
				if _, err := m.Add(slug, "HEAD"); err != nil {
					t.Fatal(err)
				}
				if prefix == "merge-base --is-ancestor" {
					managed := filepath.Join(root, ".awf", "worktrees", slug)
					writeWorktreeFile(t, filepath.Join(managed, "change"), "x")
					commitWorktree(t, managed, "change")
				}
				m.run = runnerFailingPrefix(m.run, prefix)
				if _, err := m.Remove(slug); err == nil {
					t.Fatal("probe fault hidden")
				}
			})
		}
	})
	t.Run("managed operation and dirt", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "Remove managed checks")
		if _, err := m.Add("remove-managed-checks", "HEAD"); err != nil {
			t.Fatal(err)
		}
		managed := filepath.Join(root, ".awf", "worktrees", "remove-managed-checks")
		base := m.run
		m.run = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
			if filepath.Clean(dir) == filepath.Clean(managed) && strings.HasPrefix(strings.Join(args, " "), "rev-parse --git-path") {
				return nil, errors.New("managed operation probe")
			}
			return base(ctx, dir, args...)
		}
		if _, err := m.Remove("remove-managed-checks"); err == nil {
			t.Fatal("managed operation probe hidden")
		}
		m.run = base
		writeWorktreeFile(t, filepath.Join(managed, "dirty"), "x")
		if _, err := m.Remove("remove-managed-checks"); err == nil || !strings.Contains(err.Error(), "dirty") {
			t.Fatalf("dirty error = %v", err)
		}
	})
	t.Run("foreign registration", func(t *testing.T) {
		m, _ := newManagerWithEffort(t, "Remove foreign registration")
		base := m.run
		m.run = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
			if strings.Join(args, " ") == "worktree list --porcelain -z" {
				return []byte("worktree " + m.roots.InvokingRoot + "\x00HEAD abc\x00branch refs/heads/main\x00\x00worktree /foreign\x00HEAD def\x00branch refs/heads/awf/remove-foreign-registration\x00\x00"), nil
			}
			return base(ctx, dir, args...)
		}
		if _, err := m.Remove("remove-foreign-registration"); err == nil || !strings.Contains(err.Error(), "foreign path") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("managed caller and malformed registration", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "Remove caller")
		if _, err := m.Add("remove-caller", "HEAD"); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, ".awf", "worktrees", "remove-caller")
		m.roots.InvokingRoot = path
		if _, err := m.Remove("remove-caller"); err == nil || !strings.Contains(err.Error(), "target checkout") {
			t.Fatalf("caller error = %v", err)
		}

		m, root = newManagerWithEffort(t, "Remove malformed")
		base := m.run
		path = filepath.Join(root, ".awf", "worktrees", "remove-malformed")
		m.run = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
			if strings.Join(args, " ") == "worktree list --porcelain -z" {
				return []byte("worktree " + root + "\x00HEAD abc\x00branch refs/heads/main\x00\x00worktree " + path + "\x00HEAD def\x00detached\x00\x00"), nil
			}
			return base(ctx, dir, args...)
		}
		if _, err := m.Remove("remove-malformed"); err == nil || !strings.Contains(err.Error(), "not exact") {
			t.Fatalf("registration error = %v", err)
		}
	})
	t.Run("unregistered path cleanup", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "Remove unregistered")
		if _, err := m.Add("remove-unregistered", "HEAD"); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, ".awf", "worktrees", "remove-unregistered")
		base := m.run
		m.run = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
			if strings.Join(args, " ") == "worktree list --porcelain -z" {
				return []byte("worktree " + root + "\x00HEAD abc\x00branch refs/heads/main\x00\x00"), nil
			}
			return base(ctx, dir, args...)
		}
		if _, err := m.Remove("remove-unregistered"); err == nil || !strings.Contains(err.Error(), "branch deletion failed") {
			t.Fatalf("error=%v", err)
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("path remains: %v", err)
		}
	})
	t.Run("prune success and failure", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "Remove prune success")
		if _, err := m.Add("remove-prune-success", "HEAD"); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, ".awf", "worktrees", "remove-prune-success")
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
		if result, err := m.Remove("remove-prune-success"); err != nil || !result.ChangedTopology {
			t.Fatalf("prune result=%#v err=%v", result, err)
		}

		m, root = newManagerWithEffort(t, "Remove prune")
		if _, err := m.Add("remove-prune", "HEAD"); err != nil {
			t.Fatal(err)
		}
		path = filepath.Join(root, ".awf", "worktrees", "remove-prune")
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
		base := m.run
		m.run = runnerFailingPrefix(base, "worktree prune")
		if _, err := m.Remove("remove-prune"); err == nil || !strings.Contains(err.Error(), "prunable") {
			t.Fatalf("error = %v", err)
		}
	})
}

// TestRestartCompletesFromPartialTopology proves the restartable property that
// makes the stateless design safe: after a fault between Remove's two
// mutations, a fresh manager reads the real topology and finishes from wherever
// the previous attempt stopped, with no stored evidence of that attempt.
func TestRestartCompletesFromPartialTopology(t *testing.T) {
	// invariant: tooling/effort-management:managed-worktree-lifecycle
	t.Run("branch delete faulted", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "Restart branch delete")
		slug := "restart-branch-delete"
		if _, err := m.Add(slug, "HEAD"); err != nil {
			t.Fatal(err)
		}
		m.run = runnerFailingPrefix(m.run, "branch -d")
		if _, err := m.Remove(slug); err == nil {
			t.Fatal("branch delete fault hidden")
		}
		// Genuinely partial: the checkout is gone, the branch is not.
		if _, err := os.Stat(filepath.Join(root, ".awf", "worktrees", slug)); !os.IsNotExist(err) {
			t.Fatalf("managed path survived the faulted attempt: %v", err)
		}
		if !worktreeBranchExists(t, root, slug) {
			t.Fatal("branch already deleted, so the restart fixture proves nothing")
		}
		if _, err := freshWorktreeManager(t, root).Remove(slug); err != nil {
			t.Fatalf("restart did not complete from partial topology: %v", err)
		}
		if worktreeBranchExists(t, root, slug) {
			t.Fatal("restart left the branch behind")
		}
	})
	t.Run("worktree remove faulted", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "Restart worktree remove")
		slug := "restart-worktree-remove"
		if _, err := m.Add(slug, "HEAD"); err != nil {
			t.Fatal(err)
		}
		m.run = runnerFailingPrefix(m.run, "worktree remove")
		if _, err := m.Remove(slug); err == nil {
			t.Fatal("worktree remove fault hidden")
		}
		managed := filepath.Join(root, ".awf", "worktrees", slug)
		if _, err := os.Stat(managed); err != nil {
			t.Fatalf("faulted removal discarded the checkout anyway: %v", err)
		}
		if _, err := freshWorktreeManager(t, root).Remove(slug); err != nil {
			t.Fatalf("restart did not complete after a failed first mutation: %v", err)
		}
		if _, err := os.Stat(managed); !os.IsNotExist(err) {
			t.Fatalf("restart left the managed path: %v", err)
		}
		if worktreeBranchExists(t, root, slug) {
			t.Fatal("restart left the branch behind")
		}
	})
	t.Run("add faulted before mutating", func(t *testing.T) {
		m, root := newManagerWithEffort(t, "Restart add")
		slug := "restart-add"
		m.run = runnerFailingPrefix(m.run, "worktree add")
		if _, err := m.Add(slug, "HEAD"); err == nil {
			t.Fatal("add fault hidden")
		}
		if _, err := os.Stat(filepath.Join(root, ".awf", "worktrees", slug)); !os.IsNotExist(err) {
			t.Fatalf("failed add left a path: %v", err)
		}
		if _, err := freshWorktreeManager(t, root).Add(slug, "HEAD"); err != nil {
			t.Fatalf("restart could not add after a failed attempt: %v", err)
		}
	})
}

// TestPreMutationRefusalInvokesNoDestructiveCommand pins the negative half of
// the refusal contract: a refusal that reports no topology change must not have
// run a destructive command at all.
func TestPreMutationRefusalInvokesNoDestructiveCommand(t *testing.T) {
	m, root := newManagerWithEffort(t, "Refusal invokes nothing")
	slug := "refusal-invokes-nothing"
	if _, err := m.Add(slug, "HEAD"); err != nil {
		t.Fatal(err)
	}
	managed := filepath.Join(root, ".awf", "worktrees", slug)
	writeWorktreeFile(t, filepath.Join(managed, "effort.txt"), "unmerged\n")
	commitWorktree(t, managed, "unmerged")

	var invoked []string
	m.run = recordingWorktreeRunner(m.run, &invoked)
	_, err := m.Remove(slug)
	if err == nil || !strings.Contains(err.Error(), "not merged") {
		t.Fatalf("unmerged removal error = %v", err)
	}
	if !strings.Contains(err.Error(), "changed topology: no") {
		t.Fatalf("refusal must report an unchanged topology: %v", err)
	}
	// Without this the negative loop below would pass on an empty log.
	if len(invoked) == 0 {
		t.Fatal("no git command was recorded, so the negative assertion proves nothing")
	}
	for _, args := range invoked {
		for _, destructive := range []string{"worktree remove", "branch -d", "branch -D"} {
			if strings.HasPrefix(args, destructive) {
				t.Errorf("refusal invoked %q", args)
			}
		}
	}
	if _, statErr := os.Stat(managed); statErr != nil {
		t.Fatalf("refusal discarded the managed worktree: %v", statErr)
	}
	if !worktreeBranchExists(t, root, slug) {
		t.Fatal("refusal deleted the branch")
	}
}

func freshWorktreeManager(t *testing.T, root string) *Manager {
	t.Helper()
	manager, err := Open(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func worktreeBranchExists(t *testing.T, root, slug string) bool {
	t.Helper()
	return runWorktreeGit(t, root, "branch", "--list", "awf/"+slug) != ""
}

func recordingWorktreeRunner(base Runner, log *[]string) Runner {
	return func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		*log = append(*log, strings.Join(args, " "))
		return base(ctx, dir, args...)
	}
}

func newManagerWithEffort(t *testing.T, title string) (*Manager, string) {
	t.Helper()
	root := initWorktreeRepo(t, "sha1")
	createEffort(t, root, title)
	manager, err := Open(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return manager, root
}

func runnerFailingPrefix(base Runner, prefix string) Runner {
	return func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		if strings.HasPrefix(strings.Join(args, " "), prefix) {
			return nil, errors.New("injected " + prefix)
		}
		return base(ctx, dir, args...)
	}
}

func createEffort(t *testing.T, root, title string) {
	t.Helper()
	service, err := effort.Open(context.Background(), root, effort.Options{UUID: func() (string, error) { return worktreeTestID, nil }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.New(title); err != nil {
		t.Fatal(err)
	}
}

func initWorktreeRepo(t *testing.T, format string) string {
	t.Helper()
	root := t.TempDir()
	args := []string{"init"}
	if format == "sha256" {
		args = append(args, "--object-format=sha256")
	}
	args = append(args, root)
	command := exec.Command("git", args...)
	if output, err := command.CombinedOutput(); err != nil {
		if format == "sha256" {
			t.Skipf("installed Git lacks SHA-256 repositories: %v: %s", err, output)
		}
		t.Fatalf("git init: %v: %s", err, output)
	}
	runWorktreeGit(t, root, "config", "user.name", "Test")
	runWorktreeGit(t, root, "config", "user.email", "test@example.com")
	writeWorktreeFile(t, filepath.Join(root, "tracked.txt"), "base\n")
	commitWorktree(t, root, "base")
	return root
}

func writeWorktreeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func commitWorktree(t *testing.T, root, message string) {
	t.Helper()
	runWorktreeGit(t, root, "add", "-A", "--", ".", ":(exclude).awf")
	runWorktreeGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", message)
}

func runWorktreeGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	fixed := append([]string{"-C", root}, args...)
	command := exec.Command("git", fixed...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", fixed, err, output)
	}
	return strings.TrimSpace(string(output))
}

func commandSucceeds(root string, args ...string) bool {
	fixed := append([]string{"-C", root}, args...)
	return exec.Command("git", fixed...).Run() == nil
}
