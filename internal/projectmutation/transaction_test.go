package projectmutation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

const fixtureConfig = "prefix: example\nintegrationBranch: master\nvars: {testCmd: go test ./..., gateCmd: make gate}\n"

func fixtureLoader(root string) *project.Loader {
	return project.NewLoaderWithoutRepository(config.Load, catalog.Standard, func(context.Context, string) string { return root })
}

func initializedProject(t *testing.T) (string, *project.Loader) {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	loader := fixtureLoader(root)
	session, err := loader.Load(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.New(session, project.Version).Initialize(publisher.InitAuthority{InitializedWithVersion: project.Version}); err != nil {
		t.Fatal(err)
	}
	return root, loader
}

func TestTransactionLeaseLifetimeAndCancellation(t *testing.T) {
	root := t.TempDir()
	loader := fixtureLoader(root)
	first, err := acquireTransaction(context.Background(), root, loader, TrackedScope, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if _, err := acquireTransaction(ctx, root, loader, TrackedScope, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("contended acquisition = %v, want cancellation", err)
	} else if phase, ok := FailurePhase(err); !ok || phase != PhaseLease {
		t.Fatalf("contended phase = %q, %v", phase, ok)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	second, err := acquireTransaction(context.Background(), root, loader, TrackedScope, nil)
	if err != nil {
		t.Fatalf("lease remained held after release: %v", err)
	}
	if err := second.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestTransactionRejectsNoncoveringLeaseAndReleasesIt(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	loader := fixtureLoader(root)
	released := false
	acquire := func(ctx context.Context, _ string) (*filesystem.Lease, func() error, error) {
		lease, err := filesystem.AcquireTrackedLease(ctx, other)
		if err != nil {
			return nil, nil, err
		}
		return lease, func() error {
			released = true
			return lease.Release()
		}, nil
	}
	if _, err := acquireTransaction(context.Background(), root, loader, TrackedScope, acquire); !errors.Is(err, ErrTrackedLeaseCoverage) {
		t.Fatalf("coverage error = %v", err)
	}
	if !released {
		t.Fatal("rejected lease was not released")
	}
}

func TestTransactionReleasesLeaseWhenAcquirerOmitsReleaseCallback(t *testing.T) {
	root := t.TempDir()
	loader := fixtureLoader(root)
	acquire := func(ctx context.Context, root string) (*filesystem.Lease, func() error, error) {
		lease, err := loader.AcquireProjectLease(ctx, root)
		return lease, nil, err
	}
	if _, err := AcquireProject(context.Background(), root, loader, acquire); !errors.Is(err, ErrProjectLeaseCoverage) {
		t.Fatalf("missing release callback = %v", err)
	}
	lease, err := loader.AcquireProjectLease(context.Background(), root)
	if err != nil {
		t.Fatalf("rejected acquisition leaked its lease: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
}

// invariant: tooling/filesystem-access:focused-project-mutation-transactions (TestSynchronizeReloadsCommittedAuthorityBeforeOnePassPublication)
func TestSynchronizeReloadsCommittedAuthorityBeforeOnePassPublication(t *testing.T) {
	root, loader := initializedProject(t)
	tx, err := AcquireProject(context.Background(), root, loader, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Release() //nolint:errcheck // test cleanup
	files, err := tx.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close() //nolint:errcheck // test cleanup
	session, identity, err := tx.LoadForMutation(files)
	if err != nil {
		t.Fatal(err)
	}
	defer identity.Release() //nolint:errcheck // replacement consumes the identity
	doc := config.LocalDoc{Name: "runbooks/reload", Title: "Reload", Description: "Prove committed reload."}
	updated, err := config.AppendLocalDoc(session.Config().Source(), doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := files.ReplaceExpected(".awf/config.yaml", identity, updated, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := tx.Synchronize()
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasCommittedEffects() {
		t.Fatal("synchronization reported no committed effects")
	}
	if body, err := os.ReadFile(filepath.Join(root, "docs", "runbooks", "reload.md")); err != nil || !strings.Contains(string(body), "# Reload") {
		t.Fatalf("reloaded output = %q, %v", body, err)
	}
}

func TestSynchronizeIsSingleUseAndRefusesAfterRelease(t *testing.T) {
	root, loader := initializedProject(t)
	tx, err := AcquireProject(context.Background(), root, loader, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Synchronize(); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Synchronize(); !errors.Is(err, ErrSynchronizationAttempted) {
		t.Fatalf("second synchronization = %v", err)
	}
	if err := tx.Release(); err != nil {
		t.Fatal(err)
	}

	released, err := AcquireProject(context.Background(), root, loader, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := released.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := released.Synchronize(); !errors.Is(err, ErrProjectLeaseCoverage) {
		t.Fatalf("post-release synchronization = %v", err)
	}
	if _, err := released.Open(); !errors.Is(err, ErrProjectLeaseCoverage) {
		t.Fatalf("post-release open = %v", err)
	}
}

func TestSynchronizePreservesPublisherPartialEffectsAndPhase(t *testing.T) {
	root, loader := initializedProject(t)
	if err := os.Chmod(filepath.Join(root, "AGENTS.md"), 0o600); err != nil {
		t.Fatal(err)
	}
	bridge := filepath.Join(root, "CLAUDE.md")
	if err := os.Remove(bridge); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(bridge, 0o755); err != nil {
		t.Fatal(err)
	}
	tx, err := AcquireProject(context.Background(), root, loader, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Release() //nolint:errcheck // test cleanup
	result, err := tx.Synchronize()
	if err == nil || !result.HasCommittedEffects() {
		t.Fatalf("partial publication = %#v, %v", result, err)
	}
	if phase, ok := FailurePhase(err); !ok || phase != PhasePublication {
		t.Fatalf("partial phase = %q, %v", phase, ok)
	}
	var partial *publisher.PartialError
	if !errors.As(err, &partial) || len(partial.Result.Effects()) != len(result.Effects()) {
		t.Fatalf("publisher partial = %#v, %v", partial, err)
	}
}

type committedOutcome struct{ committed bool }

func TestFinishTypesReleaseFaultWithoutOwningRecoveryPolicy(t *testing.T) {
	fault := errors.New("release sentinel")
	outcome := committedOutcome{committed: true}
	makePartial := func(outcome committedOutcome, cause error, phase Phase) error {
		return &Partial[committedOutcome]{Outcome: outcome, Cause: cause, Recovery: []string{"operation chose " + string(phase) + " recovery"}}
	}
	err := Finish(outcome, nil, &Failure{Phase: PhaseRelease, Cause: fault}, func(outcome committedOutcome) bool { return outcome.committed }, makePartial)
	var partial *Partial[committedOutcome]
	if !errors.As(err, &partial) || !errors.Is(err, fault) {
		t.Fatalf("release partial = %#v, %v", partial, err)
	}
	if len(partial.Recovery) != 1 || partial.Recovery[0] != "operation chose release recovery" {
		t.Fatalf("recovery policy = %#v", partial.Recovery)
	}
	if err.Error() != fault.Error() {
		t.Fatalf("mechanics changed diagnostic text: %q", err)
	}
}
