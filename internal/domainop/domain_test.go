package domainop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/projectmutation"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

const fixtureConfig = "prefix: example\nintegrationBranch: master\nvars: {testCmd: go test ./..., gateCmd: make gate}\n"

func operationLoader() *project.Loader {
	return project.NewLoaderWithoutRepository(config.Load, catalog.Standard, awfgit.ProjectResidentRoot)
}

func runAdd(ctx context.Context, root, name string, loader *project.Loader) (outcome Outcome, returnErr error) {
	tx, err := projectmutation.AcquireProject(ctx, root, loader, nil)
	if err != nil {
		return Outcome{}, err
	}
	defer func() { returnErr = errors.Join(returnErr, tx.Release()) }()
	return Add(ctx, name, tx)
}

func runRemove(ctx context.Context, root, name string, loader *project.Loader) (outcome Outcome, returnErr error) {
	tx, err := projectmutation.AcquireProject(ctx, root, loader, nil)
	if err != nil {
		return Outcome{}, err
	}
	defer func() { returnErr = errors.Join(returnErr, tx.Release()) }()
	return Remove(ctx, name, tx)
}

func initializedLoader(t *testing.T, root string) *project.Loader {
	t.Helper()
	loader := operationLoader()
	state, err := loader.Load(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.New(state, project.Version).Initialize(publisher.InitAuthority{InitializedWithVersion: project.Version}); err != nil {
		t.Fatal(err)
	}
	return loader
}

func TestAddScaffoldsConfiguredDomainAndRemoveReportsOrphan(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	if _, err := runAdd(context.Background(), root, "payments", initializedLoader(t, root)); err != nil {
		t.Fatal(err)
	}
	part := filepath.Join(root, ".awf", "domains", "parts", "payments", "current-state.md")
	if got, err := os.ReadFile(part); err != nil || !strings.Contains(string(got), "\"payments\" domain") {
		t.Fatalf("part = %q, %v", got, err)
	}
	outcome, err := runRemove(context.Background(), root, "payments", operationLoader())
	if err != nil || !outcome.Orphaned {
		t.Fatalf("remove = orphaned %t, %v", outcome.Orphaned, err)
	}
}

func TestRemoveRejectsAbsentConfiguredDomain(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	if _, err := runRemove(context.Background(), root, "payments", initializedLoader(t, root)); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("error = %v", err)
	}
}

func TestDomainOperationsRejectInvalidAndUnavailableInputs(t *testing.T) {
	loader := operationLoader()
	for _, operation := range []func() error{
		func() error { _, err := runAdd(context.Background(), t.TempDir(), "not valid", loader); return err },
		func() error { _, err := runRemove(context.Background(), t.TempDir(), "not valid", loader); return err },
		func() error { _, err := runAdd(context.Background(), t.TempDir(), "payments", loader); return err },
		func() error { _, err := runRemove(context.Background(), t.TempDir(), "payments", loader); return err },
	} {
		if err := operation(); err == nil {
			t.Fatal("invalid or unavailable operation succeeded")
		}
	}
}

func TestAddReportsBlockedCurrentStateParent(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	blocked := filepath.Join(root, ".awf", "domains", "parts", "payments")
	testsupport.WriteFile(t, blocked, "not a directory")
	if _, err := runAdd(context.Background(), root, "payments", operationLoader()); err == nil {
		t.Fatal("blocked current-state parent accepted")
	}
}

func TestSynchronizeReportsUnavailableProject(t *testing.T) {
	root := t.TempDir()
	tx, err := projectmutation.AcquireProject(context.Background(), root, operationLoader(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Release() //nolint:errcheck // test cleanup
	if _, err := tx.Synchronize(); err == nil {
		t.Fatal("unavailable project synchronized")
	}
}

func TestDomainOperationsReportSynchronizationFailure(t *testing.T) {
	breakRendering := func(t *testing.T, root string) {
		t.Helper()
		testsupport.WriteFile(t, filepath.Join(root, ".awf", "parts", "workflow", "commit-discipline.md"), "{{=awf:unknown-placeholder}}\n")
	}

	t.Run("add", func(t *testing.T) {
		root := t.TempDir()
		testsupport.WriteAwfConfig(t, root, fixtureConfig)
		loader := initializedLoader(t, root)
		breakRendering(t, root)
		if _, err := runAdd(context.Background(), root, "payments", loader); err == nil {
			t.Fatal("add accepted a synchronization failure")
		}
	})

	t.Run("remove", func(t *testing.T) {
		root := t.TempDir()
		testsupport.WriteAwfConfig(t, root, fixtureConfig)
		loader := initializedLoader(t, root)
		if _, err := runAdd(context.Background(), root, "payments", loader); err != nil {
			t.Fatal(err)
		}
		breakRendering(t, root)
		if _, err := runRemove(context.Background(), root, "payments", loader); err == nil {
			t.Fatal("remove accepted a synchronization failure")
		}
	})
}

func scaffoldCurrentStateForTest(root string, cfg *config.Config, name string) error {
	files, err := filesystem.Open(root)
	if err != nil {
		return err
	}
	defer files.Close()
	_, err = scaffoldCurrentStateConfined(files, root, cfg, name)
	return err
}

func TestScaffoldCurrentStatePreservesExistingAndReportsFilesystemFailures(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	session, err := operationLoader().Load(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	cfg := session.Config()
	path := cfg.PartPath("domains", "payments", "current-state")
	testsupport.WriteFile(t, path, "kept")
	if err := scaffoldCurrentStateForTest(root, cfg, "payments"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "kept" {
		t.Fatalf("existing part = %q, %v", got, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(path), path); err != nil {
		t.Fatal(err)
	}
	if err := scaffoldCurrentStateForTest(root, cfg, "payments"); err == nil {
		t.Fatal("self-referential current-state symlink accepted")
	}
	blocked, openErr := filesystem.Open("/dev/null")
	if openErr == nil {
		defer blocked.Close()
		if _, err := hasSidecarOrParts(blocked, "payments"); err == nil || !strings.Contains(err.Error(), "inspect authored domain path") {
			t.Fatalf("inspection error = %v", err)
		}
	}
	files, err := filesystem.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close()
	orphaned, err := hasSidecarOrParts(files, "absent")
	if err != nil || orphaned {
		t.Fatalf("absent authored inputs = %t, %v", orphaned, err)
	}
}

func TestAddRetainsTypedPartialOutcomeAfterSynchronizationFailure(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	loader := initializedLoader(t, root)
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "parts", "workflow", "commit-discipline.md"), "{{=awf:unknown-placeholder}}\n")
	_, err := runAdd(context.Background(), root, "payments", loader)
	var partial *PartialError
	if !errors.As(err, &partial) || !partial.Outcome.ConfigReplaced || !partial.Outcome.ScaffoldCreated {
		t.Fatalf("partial outcome = %#v, err = %v", partial, err)
	}
}

func TestFinishTypesReleaseFaultWithCommittedOutcome(t *testing.T) {
	fault := errors.New("release sentinel")
	outcome := Outcome{ConfigReplaced: true}
	err := Finish(outcome, nil, fault)
	var partial *PartialError
	if !errors.As(err, &partial) || !errors.Is(err, fault) || !partial.Outcome.ConfigReplaced {
		t.Fatalf("release partial = %#v, %v", partial, err)
	}
	if len(partial.Recovery) != 1 || !strings.Contains(partial.Recovery[0], "lease-release fault") {
		t.Fatalf("release recovery = %#v", partial.Recovery)
	}
}

func TestPartialErrorDocumentRetainsEffectsAndDefaultRecovery(t *testing.T) {
	partial := newPartial(Outcome{ConfigReplaced: true, ScaffoldCreated: true, Orphaned: true, Publisher: publisher.Result{}}, nil)
	document, err := partial.Document()
	if err != nil {
		t.Fatal(err)
	}
	var rendered strings.Builder
	if err := presentation.Render(&rendered, document); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"domain mutation partially committed", "config replacement: true", "authored scaffold: true", "orphaned authored inputs: true", "inspect the reported cause, then retry the domain command"} {
		if !strings.Contains(rendered.String(), want) {
			t.Errorf("document omitted %q: %s", want, rendered.String())
		}
	}
}
