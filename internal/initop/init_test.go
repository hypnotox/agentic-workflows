package initop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

func loaderFor(root string) LoadProject {
	return func(string) (*project.Loader, error) {
		return project.NewLoaderWithoutRepository(config.Load, catalog.Standard, func(context.Context, string) string { return root }), nil
	}
}

func TestRunWaitsForProjectLeaseBeforeMutableReads(t *testing.T) {
	root := t.TempDir()
	held, err := filesystem.AcquireProjectLease(context.Background(), root, root)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := Run(context.Background(), Input{Root: root, Answers: map[string]string{"testCmd": "go test ./...", "gateCmd": "make gate"}}, loaderFor(root), func(context.Context, string) error { return nil })
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("init completed before covering lease release: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if _, err := os.Stat(config.ConfigPath(root)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("config read or created before lease: %v", err)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestRunConvergesAfterGateFailureLeavesConfigWithoutLock(t *testing.T) {
	root := t.TempDir()
	fault := errors.New("gate sentinel")
	outcome, err := Run(context.Background(), Input{Root: root}, loaderFor(root), func(context.Context, string) error { return fault })
	if !errors.Is(err, fault) || outcome.ConfigPath != config.ConfigPath(root) {
		t.Fatalf("first outcome=%#v error=%v", outcome, err)
	}
	before, err := os.ReadFile(config.ConfigPath(root))
	if err != nil {
		t.Fatalf("read retained config: %v", err)
	}
	if _, statErr := os.Stat(config.LockPath(root)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("lock created before gate passed: %v", statErr)
	}

	outcome, err = Run(context.Background(), Input{Root: root}, loaderFor(root), func(context.Context, string) error { return nil })
	if err != nil || !outcome.ExistingConfig {
		t.Fatalf("rerun outcome=%#v error=%v", outcome, err)
	}
	after, err := os.ReadFile(config.ConfigPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("rerun overwrote retained config")
	}
	if _, found, err := manifest.LoadOptional(config.LockPath(root)); err != nil || !found {
		t.Fatalf("load completed lock: found=%v error=%v", found, err)
	}
}

func TestRunAdoptsExactOutputsAfterInterruptedLockPublication(t *testing.T) {
	root := t.TempDir()
	gate := func(context.Context, string) error { return nil }
	if _, err := Run(context.Background(), Input{Root: root}, loaderFor(root), gate); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	outcome, err := Run(context.Background(), Input{Root: root}, loaderFor(root), gate)
	if err != nil || !outcome.ExistingConfig {
		t.Fatalf("rerun outcome=%#v error=%v", outcome, err)
	}
	if _, err := os.Stat(config.LockPath(root)); err != nil {
		t.Fatalf("replacement lock missing: %v", err)
	}
}

func TestRunIsIdempotentForInitializedProject(t *testing.T) {
	root := t.TempDir()
	gate := func(context.Context, string) error { return nil }
	if _, err := Run(context.Background(), Input{Root: root, Answers: map[string]string{"testCmd": "go test ./...", "gateCmd": "make gate"}}, loaderFor(root), gate); err != nil {
		t.Fatal(err)
	}
	outcome, err := Run(context.Background(), Input{Root: root, Answers: map[string]string{"testCmd": "ignored"}}, loaderFor(root), gate)
	if err != nil || !outcome.ExistingConfig || !outcome.IgnoredAnswers {
		t.Fatalf("rerun outcome=%#v error=%v", outcome, err)
	}
}

func TestRunRefusesPermanentLockWithoutConfig(t *testing.T) {
	root := t.TempDir()
	gate := func(context.Context, string) error { return nil }
	if _, err := Run(context.Background(), Input{Root: root}, loaderFor(root), gate); err != nil {
		t.Fatal(err)
	}
	lockPath := config.LockPath(root)
	before, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(config.ConfigPath(root)); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), Input{Root: root}, loaderFor(root), gate); err == nil {
		t.Fatal("lock without config accepted")
	}
	if _, statErr := os.Stat(config.ConfigPath(root)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("config recreated beside orphaned lock: %v", statErr)
	}
	after, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("orphaned lock changed")
	}
}

func TestRunRefusesCollisionWithoutCreatingConfig(t *testing.T) {
	root := t.TempDir()
	foreign := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(foreign, []byte("foreign"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Run(context.Background(), Input{Root: root}, loaderFor(root), func(context.Context, string) error { return nil })
	if err == nil {
		t.Fatal("collision accepted")
	}
	if _, statErr := os.Stat(config.ConfigPath(root)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("config created: %v", statErr)
	}
	got, _ := os.ReadFile(foreign)
	if string(got) != "foreign" {
		t.Fatalf("foreign file changed: %q", got)
	}
}

func TestRunRefusesInvalidExistingConfigWithoutClobberingIt(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := config.ConfigPath(root)
	invalid := []byte("prefix: [unterminated\n")
	if err := os.WriteFile(path, invalid, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Run(context.Background(), Input{Root: root}, loaderFor(root), func(context.Context, string) error { return nil })
	if err == nil {
		t.Fatal("invalid config accepted")
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(invalid) {
		t.Fatalf("config overwritten: %q", got)
	}
	if _, statErr := os.Stat(config.LockPath(root)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("lock created for invalid config: %v", statErr)
	}
}

func TestReleaseFailureIsOrdinaryAndRetainsOutcome(t *testing.T) {
	root := t.TempDir()
	fault := errors.New("release sentinel")
	outcome, err := runWithDependencies(context.Background(), Input{Root: root}, loaderFor(root), func(context.Context, string) error { return errors.New("stop after config") }, func(*project.Session, *config.Config, *publisher.Publisher) ([]string, error) { return nil, nil }, func(lease *filesystem.Lease) error { return errors.Join(lease.Release(), fault) })
	if outcome.ConfigPath == "" || !errors.Is(err, fault) {
		t.Fatalf("outcome=%#v error=%v", outcome, err)
	}
}
