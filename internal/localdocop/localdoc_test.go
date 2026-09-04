package localdocop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

const fixtureConfig = "prefix: example\nintegrationBranch: master\nvars: {testCmd: go test ./..., gateCmd: make gate}\n"

func operationLoader() *project.Loader {
	return project.NewLoaderWithoutRepository(config.Load, catalog.Standard, awfgit.ProjectResidentRoot)
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

func TestRunSynchronizesAndRefusesCollisionBeforeConfig(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	loader := initializedLoader(t, root)
	doc := config.LocalDoc{Name: "runbooks/api", Title: "API", Description: "Operate API"}
	outcome, err := Run(context.Background(), root, doc, loader, nil)
	if err != nil || !outcome.DeclarationReplaced {
		t.Fatalf("outcome=%#v error=%v", outcome, err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs/runbooks/api.md")); err != nil {
		t.Fatal(err)
	}

	root = t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	loader = initializedLoader(t, root)
	testsupport.WriteFile(t, filepath.Join(root, "docs/runbooks/api.md"), "foreign")
	before, _ := os.ReadFile(config.ConfigPath(root))
	_, err = Run(context.Background(), root, doc, loader, nil)
	after, _ := os.ReadFile(config.ConfigPath(root))
	if err == nil || string(before) != string(after) {
		t.Fatalf("collision=%v config changed=%v", err, string(before) != string(after))
	}
}

func TestRunPreflightsPublicationBeforeDeclarationMutation(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	loader := initializedLoader(t, root)
	blocked := filepath.Join(root, ".claude/skills/awf-maintenance/SKILL.md")
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(config.ConfigPath(root))
	if err != nil {
		t.Fatal(err)
	}
	doc := config.LocalDoc{Name: "runbooks/api", Title: "API", Description: "Operate API"}
	outcome, err := Run(context.Background(), root, doc, loader, nil)
	if err == nil || outcome.DeclarationReplaced {
		t.Fatalf("outcome=%#v error=%v", outcome, err)
	}
	after, readErr := os.ReadFile(config.ConfigPath(root))
	if readErr != nil || string(after) != string(before) {
		t.Fatalf("declaration changed before publication preflight: %v", readErr)
	}
}

func TestRunResumesMatchingCommittedDeclaration(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	loader := initializedLoader(t, root)
	doc := config.LocalDoc{Name: "runbooks/api", Title: "API", Description: "Operate API"}
	cfg, err := config.Load(config.RootDir(root))
	if err != nil {
		t.Fatal(err)
	}
	updated, err := config.AppendLocalDoc(cfg.Source(), doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigPath(root), updated, 0o644); err != nil {
		t.Fatal(err)
	}
	outcome, err := Run(context.Background(), root, doc, loader, nil)
	if err != nil || outcome.DeclarationReplaced || outcome.DocumentPath != "docs/runbooks/api.md" {
		t.Fatalf("resumed outcome=%#v error=%v", outcome, err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs/runbooks/api.md")); err != nil {
		t.Fatal(err)
	}
}

func TestLeasePrecedesReadAndReleaseErrorIsOrdinary(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	initializer := initializedLoader(t, root)
	acquired := false
	observing := false
	loader := project.NewLoaderWithoutRepository(func(path string) (*config.Config, error) {
		if observing && !acquired {
			t.Fatal("read before lease")
		}
		return config.Load(path)
	}, catalog.Standard, awfgit.ProjectResidentRoot)
	fault := errors.New("release sentinel")
	acquire := func(ctx context.Context, root string) (*filesystem.Lease, func() error, error) {
		acquired = true
		lease, err := initializer.AcquireProjectLease(ctx, root)
		return lease, func() error { return errors.Join(lease.Release(), fault) }, err
	}
	observing = true
	outcome, err := Run(context.Background(), root, config.LocalDoc{Name: "api", Title: "API", Description: "Operate"}, loader, acquire)
	if !outcome.DeclarationReplaced || !errors.Is(err, fault) {
		t.Fatalf("outcome=%#v error=%v", outcome, err)
	}
}
