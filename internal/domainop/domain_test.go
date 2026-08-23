package domainop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

const fixtureConfig = "prefix: example\nprofile: full\nintegrationBranch: master\nvars: {testCmd: go test ./..., gateCmd: make gate}\n"

func operationLoader() *project.Loader {
	return project.NewLoaderWithoutRepository(config.Load, catalog.Standard, awfgit.ProjectResidentRoot)
}

func initializedLoader(t *testing.T, root string) *project.Loader {
	t.Helper()
	loader := operationLoader()
	state, cfg, err := loader.OpenForOperation(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := publisher.New(state.OutputState(), cfg, publisher.NewFilesystemReader(state.Root()), project.Version).Prepare()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepared.Initialize(publisher.InitAuthority{InitializedWithVersion: project.Version}); err != nil {
		t.Fatal(err)
	}
	return loader
}

func TestAddScaffoldsConfiguredDomainAndRemoveReportsOrphan(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	if _, err := Add(context.Background(), root, "payments", initializedLoader(t, root)); err != nil {
		t.Fatal(err)
	}
	part := filepath.Join(root, ".awf", "domains", "parts", "payments", "current-state.md")
	if got, err := os.ReadFile(part); err != nil || !strings.Contains(string(got), "\"payments\" domain") {
		t.Fatalf("part = %q, %v", got, err)
	}
	_, orphaned, err := Remove(context.Background(), root, "payments", operationLoader())
	if err != nil || !orphaned {
		t.Fatalf("remove = orphaned %t, %v", orphaned, err)
	}
}

func TestRemoveRejectsAbsentConfiguredDomain(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	if _, _, err := Remove(context.Background(), root, "payments", initializedLoader(t, root)); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("error = %v", err)
	}
}

func TestDomainOperationsRejectInvalidAndUnavailableInputs(t *testing.T) {
	loader := operationLoader()
	for _, operation := range []func() error{
		func() error { _, err := Add(context.Background(), t.TempDir(), "not valid", loader); return err },
		func() error { _, _, err := Remove(context.Background(), t.TempDir(), "not valid", loader); return err },
		func() error { _, err := Add(context.Background(), t.TempDir(), "payments", loader); return err },
		func() error { _, _, err := Remove(context.Background(), t.TempDir(), "payments", loader); return err },
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
	if _, err := Add(context.Background(), root, "payments", operationLoader()); err == nil {
		t.Fatal("blocked current-state parent accepted")
	}
}

func TestSynchronizeReportsUnavailableProject(t *testing.T) {
	if _, err := synchronize(context.Background(), t.TempDir(), operationLoader()); err == nil {
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
		if _, err := Add(context.Background(), root, "payments", loader); err == nil {
			t.Fatal("add accepted a synchronization failure")
		}
	})

	t.Run("remove", func(t *testing.T) {
		root := t.TempDir()
		testsupport.WriteAwfConfig(t, root, fixtureConfig)
		loader := initializedLoader(t, root)
		if _, err := Add(context.Background(), root, "payments", loader); err != nil {
			t.Fatal(err)
		}
		breakRendering(t, root)
		if _, _, err := Remove(context.Background(), root, "payments", loader); err == nil {
			t.Fatal("remove accepted a synchronization failure")
		}
	})
}

func TestScaffoldCurrentStatePreservesExistingAndReportsFilesystemFailures(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	_, cfg, err := operationLoader().OpenForOperation(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	path := cfg.PartPath("domains", "payments", "current-state")
	testsupport.WriteFile(t, path, "kept")
	if err := scaffoldCurrentState(cfg, "payments"); err != nil {
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
	if err := scaffoldCurrentState(cfg, "payments"); err == nil {
		t.Fatal("self-referential current-state symlink accepted")
	}
	if _, err := hasSidecarOrParts("/dev/null", "payments"); err == nil || !strings.Contains(err.Error(), "inspect authored domain path") {
		t.Fatalf("inspection error = %v", err)
	}
	orphaned, err := hasSidecarOrParts(root, "absent")
	if err != nil || orphaned {
		t.Fatalf("absent authored inputs = %t, %v", orphaned, err)
	}
}
