package application

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/effort"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

const creationTestID = "018f47a0-7b3d-4c52-8f1a-123456789abc"

type faultRunner struct {
	worktree.Runner
	add          func(context.Context, string, string, string) error
	worktreeList func(context.Context) ([]awfgit.WorktreeRegistration, error)
	branchExists func(context.Context, string) (bool, error)
}

func (r *faultRunner) WorktreeAdd(ctx context.Context, path, branch, base string) error {
	if r.add != nil {
		return r.add(ctx, path, branch, base)
	}
	return r.Runner.WorktreeAdd(ctx, path, branch, base)
}

func (r *faultRunner) WorktreeList(ctx context.Context) ([]awfgit.WorktreeRegistration, error) {
	if r.worktreeList != nil {
		return r.worktreeList(ctx)
	}
	return r.Runner.WorktreeList(ctx)
}

func (r *faultRunner) BranchExists(ctx context.Context, branch string) (bool, error) {
	if r.branchExists != nil {
		return r.branchExists(ctx, branch)
	}
	return r.Runner.BranchExists(ctx, branch)
}

func creationApp(t *testing.T, root string, apply func(*faultRunner), fault func(string) error) *app {
	t.Helper()
	ctx := testContext(t)
	roots, err := awfgit.ResolveControlRoots(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := awfgit.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	runner := &faultRunner{Runner: repo}
	if apply != nil {
		apply(runner)
	}
	service, err := effort.Open(roots, effort.Dependencies{
		Clock:                 func() time.Time { return time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC) },
		UUID:                  func() (string, error) { return creationTestID, nil },
		Worktrees:             runner.WorktreeList,
		BranchExists:          runner.BranchExists,
		ValidateRef:           repo.ValidateRefName,
		ExpectedArchiveMarker: markerRenderer,
		Fault:                 fault,
	})
	if err != nil {
		t.Fatal(err)
	}
	open := func(checkoutRoot string) (worktree.Runner, error) {
		if sameCreationFixtureCheckout(checkoutRoot, root) {
			return runner, nil
		}
		return awfgit.Open(checkoutRoot)
	}
	openResident := func(name awfgit.ResidentName) (worktree.ResidentHandle, error) {
		residentRoot, rootErr := roots.ResidentRoot(name)
		if rootErr != nil {
			return nil, rootErr
		}
		return filesystem.Open(residentRoot)
	}
	manager, err := worktree.Open(roots, open, openResident)
	if err != nil {
		t.Fatal(err)
	}
	return &app{service: service, manager: manager, gate: integrationGateCommand}
}

func sameCreationFixtureCheckout(left, right string) bool {
	return sameCreationFixtureCheckoutWith(filesystem.NormalizePlatformPath, left, right)
}

func sameCreationFixtureCheckoutWith(normalize func(string) string, left, right string) bool {
	return normalize(left) == normalize(right)
}

func TestCreationFixtureMatchesDarwinVarAlias(t *testing.T) {
	normalizeDarwinAlias := func(value string) string {
		if value == "/var/folders/fixture/repo" {
			return "/private/var/folders/fixture/repo"
		}
		return filepath.Clean(value)
	}
	if !sameCreationFixtureCheckoutWith(normalizeDarwinAlias, "/var/folders/fixture/repo", "/private/var/folders/fixture/repo") {
		t.Fatal("creation fixture did not match Darwin /var alias to canonical /private/var path")
	}
}

func TestCreationRollbackRemovesOnlyWhenTopologyIsProvenAbsent(t *testing.T) {
	t.Run("rolled back", func(t *testing.T) {
		root := applicationRepo(t)
		cause := errors.New("worktree add")
		application := creationApp(t, root, func(runner *faultRunner) {
			runner.add = func(context.Context, string, string, string) error { return cause }
		}, nil)
		_, _, err := application.newEffort(testContext(t), effort.NewInput{Slug: "rolled-back", Title: "Rolled back"}, "")
		var creation *CreationError
		if !errors.As(err, &creation) || !errors.Is(err, cause) || creation.RollbackCause != nil || creation.ChangedEffort || creation.ChangedTopology {
			t.Fatalf("rollback outcome = %#v, error=%v", creation, err)
		}
		if _, statErr := os.Lstat(filepath.Join(root, ".awf", "efforts", "rolled-back")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("rolled-back resident remains: %v", statErr)
		}
	})

	t.Run("retained with topology", func(t *testing.T) {
		root := applicationRepo(t)
		cause := errors.New("post-add failure")
		application := creationApp(t, root, func(runner *faultRunner) {
			runner.add = func(ctx context.Context, path, branch, base string) error {
				if err := runner.Runner.WorktreeAdd(ctx, path, branch, base); err != nil {
					return err
				}
				return cause
			}
		}, nil)
		_, _, err := application.newEffort(testContext(t), effort.NewInput{Slug: "retained-topology", Title: "Retained topology"}, "")
		var creation *CreationError
		if !errors.As(err, &creation) || !errors.Is(err, cause) || !errors.Is(err, effort.ErrManagedTopologyPresent) || !creation.ChangedEffort {
			t.Fatalf("retained outcome = %#v, error=%v", creation, err)
		}
		want := worktree.TopologyEffects{ManagedPath: true, GitRegistration: true, Branch: true}
		if creation.Topology != want {
			t.Fatalf("retained topology = %#v, want %#v", creation.Topology, want)
		}
		if _, showErr := application.service.Show("retained-topology"); showErr != nil {
			t.Fatalf("retained resident absent: %v", showErr)
		}
		var diagnostic interface {
			Diagnostic() (presentation.Diagnostic, error)
		}
		if !errors.As(presentError(err), &diagnostic) {
			t.Fatal("creation error lost outer application diagnostic")
		}
		mapped, mapErr := diagnostic.Diagnostic()
		if mapErr != nil || mapped.Condition != "managed worktree creation failed and topology remains" {
			t.Fatalf("outer diagnostic = %#v, %v", mapped, mapErr)
		}
	})
}

func TestCreationRollbackPreservesUncertaintyAndInterruptionStates(t *testing.T) {
	t.Run("unknown topology", func(t *testing.T) {
		root := applicationRepo(t)
		probe := errors.New("topology probe")
		application := creationApp(t, root, func(runner *faultRunner) {
			runner.worktreeList = func(context.Context) ([]awfgit.WorktreeRegistration, error) { return nil, probe }
			runner.branchExists = func(context.Context, string) (bool, error) { return false, probe }
		}, nil)
		_, _, err := application.newEffort(testContext(t), effort.NewInput{Slug: "unknown-creation", Title: "Unknown creation"}, "")
		var creation *CreationError
		if !errors.As(err, &creation) || !creation.Topology.Uncertain || !creation.ChangedEffort {
			t.Fatalf("uncertain creation = %#v, error=%v", creation, err)
		}
	})

	for _, test := range []struct {
		stage         string
		wantCondition string
		residentGone  bool
		reservation   bool
	}{
		{"rollback.rename", "managed worktree creation failed and effort rollback failed", false, false},
		{"rollback.root-fsync", "managed worktree creation failed and effort deletion rollback was interrupted", true, true},
		{"rollback.delete-fsync", "managed worktree creation failed after effort deletion with durability uncertainty", true, false},
	} {
		t.Run(test.stage, func(t *testing.T) {
			root := applicationRepo(t)
			application := creationApp(t, root, func(runner *faultRunner) {
				runner.add = func(context.Context, string, string, string) error { return errors.New("worktree add") }
			}, func(stage string) error {
				if stage == test.stage {
					return errors.New("injected " + stage)
				}
				return nil
			})
			_, _, err := application.newEffort(testContext(t), effort.NewInput{Slug: "faulted-rollback", Title: "Faulted rollback"}, "")
			var creation *CreationError
			if !errors.As(err, &creation) || creation.Condition != test.wantCondition || creation.RollbackCause == nil {
				t.Fatalf("faulted creation = %#v, error=%v", creation, err)
			}
			active := filepath.Join(root, ".awf", "efforts", "faulted-rollback")
			_, activeErr := os.Lstat(active)
			if errors.Is(activeErr, os.ErrNotExist) != test.residentGone {
				t.Fatalf("active resident error=%v, want gone=%t", activeErr, test.residentGone)
			}
			reservation := filepath.Join(root, ".awf", "efforts", ".finishing-"+creationTestID+"-faulted-rollback")
			_, reservationErr := os.Lstat(reservation)
			if (!errors.Is(reservationErr, os.ErrNotExist)) != test.reservation {
				t.Fatalf("reservation error=%v, want present=%t", reservationErr, test.reservation)
			}
		})
	}
}

func TestCreationValidatesResidentBeforeWorktreeMutation(t *testing.T) {
	root := applicationRepo(t)
	added := false
	application := creationApp(t, root, func(runner *faultRunner) {
		runner.add = func(context.Context, string, string, string) error {
			added = true
			return nil
		}
	}, nil)
	for _, input := range []effort.NewInput{{Slug: "invalid-title", Title: "  "}, {Slug: strings.Repeat("s", 33), Title: "Overlong slug"}} {
		record, result, err := application.newEffort(testContext(t), input, "")
		if err == nil || record != (effort.Record{}) || result != (worktree.Result{}) || added {
			t.Fatalf("resident validation record=%#v result=%#v err=%v added=%t", record, result, err, added)
		}
	}
}

func TestAddWorktreeRevalidatesResidentBeforeTopologyMutation(t *testing.T) {
	root := applicationRepo(t)
	added := false
	application := creationApp(t, root, func(runner *faultRunner) {
		runner.add = func(context.Context, string, string, string) error {
			added = true
			return nil
		}
	}, nil)
	record, err := application.service.New(testContext(t), effort.NewInput{Slug: "invalidated", Title: "Invalidated"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, record.MemoryPath)); err != nil {
		t.Fatal(err)
	}
	if _, err := application.addWorktree(testContext(t), record.Slug, ""); err == nil || added {
		t.Fatalf("invalidated resident add error=%v added=%t", err, added)
	}
}
