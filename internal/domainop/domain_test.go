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

func TestAddRemoveAndPreflightRefusal(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	loader := initializedLoader(t, root)
	outcome, err := Add(context.Background(), root, "payments", loader, nil)
	if err != nil || !outcome.ConfigReplaced || !outcome.ScaffoldCreated {
		t.Fatalf("add=%#v %v", outcome, err)
	}
	part := filepath.Join(root, ".awf/domains/parts/payments/current-state.md")
	if got, err := os.ReadFile(part); err != nil || !strings.Contains(string(got), `"payments" domain`) {
		t.Fatalf("part=%q %v", got, err)
	}
	removed, err := Remove(context.Background(), root, "payments", loader, nil)
	if err != nil || !removed.Orphaned {
		t.Fatalf("remove=%#v %v", removed, err)
	}

	root = t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	loader = initializedLoader(t, root)
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
	outcome, err = Add(context.Background(), root, "payments", loader, nil)
	if err == nil || outcome.ConfigReplaced || outcome.ScaffoldCreated {
		t.Fatalf("preflight refusal=%#v %v", outcome, err)
	}
	after, readErr := os.ReadFile(config.ConfigPath(root))
	if readErr != nil || string(after) != string(before) {
		t.Fatalf("config changed before publication preflight: %v", readErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".awf/domains/parts/payments/current-state.md")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("scaffold created before publication preflight: %v", statErr)
	}
}

func TestAddAndRemoveResumeAlreadyCommittedConfigState(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	loader := initializedLoader(t, root)
	cfg, err := config.Load(config.RootDir(root))
	if err != nil {
		t.Fatal(err)
	}
	updated, err := config.SetArrayMember(cfg.Source(), "domains", "payments", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigPath(root), updated, 0o644); err != nil {
		t.Fatal(err)
	}
	added, err := Add(context.Background(), root, "payments", loader, nil)
	if err != nil || added.ConfigReplaced || !added.ScaffoldCreated {
		t.Fatalf("resumed add=%#v %v", added, err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs/domains/payments.md")); err != nil {
		t.Fatal(err)
	}

	cfg, err = config.Load(config.RootDir(root))
	if err != nil {
		t.Fatal(err)
	}
	updated, err = config.SetArrayMember(cfg.Source(), "domains", "payments", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigPath(root), updated, 0o644); err != nil {
		t.Fatal(err)
	}
	removed, err := Remove(context.Background(), root, "payments", loader, nil)
	if err != nil || removed.ConfigReplaced || !removed.Orphaned {
		t.Fatalf("resumed remove=%#v %v", removed, err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs/domains/payments.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("retired domain output remains: %v", err)
	}
}

func TestAddRefusesDifferingScaffoldBeforeConfigMutation(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fixtureConfig)
	loader := initializedLoader(t, root)
	path := filepath.Join(root, ".awf/domains/parts/payments/current-state.md")
	testsupport.WriteFile(t, path, "foreign")
	before, _ := os.ReadFile(config.ConfigPath(root))
	_, err := Add(context.Background(), root, "payments", loader, nil)
	after, _ := os.ReadFile(config.ConfigPath(root))
	if err == nil || string(before) != string(after) {
		t.Fatalf("collision=%v config changed=%v", err, string(before) != string(after))
	}
}

func TestLeasePrecedesAuthorityAndReleaseErrorIsOrdinary(t *testing.T) {
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
	outcome, err := Add(context.Background(), root, "payments", loader, acquire)
	if !outcome.ConfigReplaced || !errors.Is(err, fault) {
		t.Fatalf("outcome=%#v error=%v", outcome, err)
	}
}
