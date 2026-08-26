package localdocop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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
	if _, err := publisher.New(state.OutputState(), cfg, publisher.NewFilesystemReader(state.Root()), project.Version).Initialize(publisher.InitAuthority{InitializedWithVersion: project.Version}); err != nil {
		t.Fatal(err)
	}
	return loader
}

func TestRunAddsDeclarationAndSynchronizes(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	doc := config.LocalDoc{Name: "runbooks/api", Title: "API", Description: "Operate API"}
	if err := Run(context.Background(), root, doc, initializedLoader(t, root)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "runbooks", "api.md")); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(config.RootDir(root))
	if err != nil || len(cfg.LocalDocs) != 1 || cfg.LocalDocs[0] != doc {
		t.Fatalf("local docs = %#v, %v", cfg.LocalDocs, err)
	}
}

func TestRunRejectsUnavailableAndInvalidInputs(t *testing.T) {
	doc := config.LocalDoc{Name: "runbooks/api", Title: "API", Description: "Operate API"}
	if err := Run(context.Background(), t.TempDir(), doc, operationLoader()); err == nil {
		t.Fatal("uninitialized project accepted")
	}
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	if err := Run(context.Background(), root, config.LocalDoc{Name: "bad name", Title: "API", Description: "Operate API"}, initializedLoader(t, root)); err == nil {
		t.Fatal("invalid local document accepted")
	}
}

func TestRunRetainsDeclarationWhenSynchronizationFails(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	loader := initializedLoader(t, root)
	beforeLock, err := os.ReadFile(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	invalid := strings.Replace(fixtureConfig, ", gateCmd: make gate", "", 1)
	if err := os.WriteFile(config.ConfigPath(root), []byte(invalid), 0o644); err != nil {
		t.Fatal(err)
	}

	doc := config.LocalDoc{Name: "runbooks/api", Title: "API", Description: "Operate API"}
	err = Run(context.Background(), root, doc, loader)
	if err == nil || !strings.Contains(err.Error(), "vars.gateCmd") {
		t.Fatalf("synchronization error = %v", err)
	}
	cfg, loadErr := config.Load(config.RootDir(root))
	if loadErr != nil || len(cfg.LocalDocs) != 1 || cfg.LocalDocs[0] != doc {
		t.Fatalf("retryable declaration = %#v, %v", cfg.LocalDocs, loadErr)
	}
	afterLock, readErr := os.ReadFile(config.LockPath(root))
	if readErr != nil || string(afterLock) != string(beforeLock) {
		t.Fatalf("lock changed after failed synchronization: %v", readErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, "docs", "runbooks", "api.md")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("local document output exists after failed synchronization: %v", statErr)
	}
}

func TestRunReportsDestinationInspectionFailure(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	testsupport.WriteFile(t, filepath.Join(root, "docs", "runbooks"), "not a directory")
	err := Run(context.Background(), root, config.LocalDoc{Name: "runbooks/api", Title: "API", Description: "Operate API"}, initializedLoader(t, root))
	if err == nil || !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("error = %v", err)
	}
}

func TestRunRefusesExistingDestinationBeforeWritingConfig(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	path := filepath.Join(root, "docs", "runbooks", "api.md")
	testsupport.WriteFile(t, path, "foreign")
	before, err := os.ReadFile(config.ConfigPath(root))
	if err != nil {
		t.Fatal(err)
	}
	err = Run(context.Background(), root, config.LocalDoc{Name: "runbooks/api", Title: "API", Description: "Operate API"}, initializedLoader(t, root))
	if err == nil || !errors.Is(err, os.ErrExist) && err.Error() != "local document destination already exists: docs/runbooks/api.md" {
		t.Fatalf("error = %v", err)
	}
	after, readErr := os.ReadFile(config.ConfigPath(root))
	if readErr != nil || string(after) != string(before) {
		t.Fatalf("config mutated: %q, %v", after, readErr)
	}
}
