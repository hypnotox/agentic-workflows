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

func runOperation(ctx context.Context, root string, doc config.LocalDoc, loader *project.Loader) (outcome Outcome, returnErr error) {
	tx, err := projectmutation.AcquireProject(ctx, root, loader, nil)
	if err != nil {
		return Outcome{}, err
	}
	defer func() { returnErr = errors.Join(returnErr, tx.Release()) }()
	return Run(ctx, doc, tx)
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

func TestRunAddsDeclarationAndSynchronizes(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	doc := config.LocalDoc{Name: "runbooks/api", Title: "API", Description: "Operate API"}
	if _, err := runOperation(context.Background(), root, doc, initializedLoader(t, root)); err != nil {
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
	if _, err := runOperation(context.Background(), t.TempDir(), doc, operationLoader()); err == nil {
		t.Fatal("uninitialized project accepted")
	}
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	if _, err := runOperation(context.Background(), root, config.LocalDoc{Name: "bad name", Title: "API", Description: "Operate API"}, initializedLoader(t, root)); err == nil {
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
	_, err = runOperation(context.Background(), root, doc, loader)
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
	_, err := runOperation(context.Background(), root, config.LocalDoc{Name: "runbooks/api", Title: "API", Description: "Operate API"}, initializedLoader(t, root))
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
	_, err = runOperation(context.Background(), root, config.LocalDoc{Name: "runbooks/api", Title: "API", Description: "Operate API"}, initializedLoader(t, root))
	if err == nil || !errors.Is(err, os.ErrExist) && err.Error() != "local document destination already exists: docs/runbooks/api.md" {
		t.Fatalf("error = %v", err)
	}
	after, readErr := os.ReadFile(config.ConfigPath(root))
	if readErr != nil || string(after) != string(before) {
		t.Fatalf("config mutated: %q, %v", after, readErr)
	}
}

func TestRunRetainsTypedPartialOutcomeAfterDeclaration(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	loader := initializedLoader(t, root)
	invalid := strings.Replace(fixtureConfig, ", gateCmd: make gate", "", 1)
	if err := os.WriteFile(config.ConfigPath(root), []byte(invalid), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := runOperation(context.Background(), root, config.LocalDoc{Name: "runbooks/api", Title: "API", Description: "Operate API"}, loader)
	var partial *PartialError
	if !errors.As(err, &partial) || !partial.Outcome.DeclarationReplaced {
		t.Fatalf("partial = %#v, err = %v", partial, err)
	}
}

func TestFinishTypesReleaseFaultWithCommittedOutcome(t *testing.T) {
	fault := errors.New("release sentinel")
	outcome := Outcome{DocumentPath: "docs/runbooks/api.md", DeclarationReplaced: true}
	err := Finish(outcome, nil, fault)
	var partial *PartialError
	if !errors.As(err, &partial) || !errors.Is(err, fault) || !partial.Outcome.DeclarationReplaced {
		t.Fatalf("release partial = %#v, %v", partial, err)
	}
	if len(partial.Recovery) != 1 || !strings.Contains(partial.Recovery[0], "lease-release fault") {
		t.Fatalf("release recovery = %#v", partial.Recovery)
	}
}

func TestPartialErrorDocumentRetainsDeclarationAndDefaultRecovery(t *testing.T) {
	partial := newPartial(Outcome{DocumentPath: "docs/runbooks/api.md", DeclarationReplaced: true}, nil)
	document, err := partial.Document()
	if err != nil {
		t.Fatal(err)
	}
	var rendered strings.Builder
	if err := presentation.Render(&rendered, document); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"local document partially committed", "local-document declaration replacement: true", "local document: docs/runbooks/api.md", "inspect the reported cause, then retry awf new doc"} {
		if !strings.Contains(rendered.String(), want) {
			t.Errorf("document omitted %q: %s", want, rendered.String())
		}
	}
}
