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

func TestCreationAddFailureRetainsResidentForOrdinaryRetry(t *testing.T) {
	root := applicationRepo(t)
	cause := errors.New("worktree add")
	application := creationApp(t, root, func(runner *faultRunner) {
		runner.add = func(context.Context, string, string, string) error { return cause }
	}, nil)
	record, result, err := application.newEffort(testContext(t), effort.NewInput{Slug: "retained", Title: "Retained"}, "")
	var creation *CreationError
	if !errors.As(err, &creation) || !errors.Is(err, cause) || !creation.ChangedEffort || record.Slug != "retained" || result != (worktree.Result{}) {
		t.Fatalf("creation record=%#v result=%#v outcome=%#v error=%v", record, result, creation, err)
	}
	if creation.Condition != "managed worktree creation failed and the effort resident was retained" {
		t.Fatalf("condition=%q", creation.Condition)
	}
	if _, showErr := application.service.Show("retained"); showErr != nil {
		t.Fatalf("retained resident absent: %v", showErr)
	}
	if len(creation.Steps) < 2 || !strings.Contains(creation.Steps[1], "worktree add retained") {
		t.Fatalf("recovery steps=%#v", creation.Steps)
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
