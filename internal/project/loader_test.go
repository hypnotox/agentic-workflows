package project

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestNewLoaderRejectsMissingDependencies(t *testing.T) {
	load := LoadConfigTree(config.Load)
	resolve := ResolveResidentRoot(func(_ context.Context, root string) string { return root })
	for _, tc := range []struct {
		name     string
		load     LoadConfigTree
		standard *catalog.Catalog
		resolve  ResolveResidentRoot
	}{
		{name: "load config tree", standard: catalog.Standard, resolve: resolve},
		{name: "standard catalog", load: load, resolve: resolve},
		{name: "resolve resident root", load: load, standard: catalog.Standard},
		{name: "git repository", load: load, standard: catalog.Standard, resolve: resolve},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				got := recover()
				if got == nil || !strings.Contains(got.(string), tc.name) {
					t.Fatalf("panic = %v, want text containing %q", got, tc.name)
				}
			}()
			NewLoader(tc.load, tc.standard, tc.resolve, nil)
		})
	}
}

func TestLoaderOpenReturnsLoadError(t *testing.T) {
	root := t.TempDir()
	sentinel := errors.New("load sentinel")
	var gotPath string
	loader := NewLoaderWithoutRepository(func(path string) (*config.Config, error) {
		gotPath = path
		return nil, sentinel
	}, catalog.Standard, func(context.Context, string) string {
		t.Fatal("resident resolver called after load failure")
		return ""
	})
	_, err := loader.Open(testContext(t), root)
	if gotPath != config.RootDir(root) {
		t.Fatalf("load path = %q, want %q", gotPath, config.RootDir(root))
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want sentinel", err)
	}
}

func TestLoaderOpenRejectsNilLoadedConfig(t *testing.T) {
	loader := NewLoaderWithoutRepository(func(string) (*config.Config, error) {
		var cfg *config.Config
		return cfg, nil
	}, catalog.Standard, func(context.Context, string) string {
		t.Fatal("resident resolver called after nil config")
		return ""
	})
	_, err := loader.Open(testContext(t), t.TempDir())
	if err == nil || err.Error() != "project Loader: load config tree returned nil config" {
		t.Fatalf("error = %v", err)
	}
}

func TestLoaderOpenValidatesBeforeResolvingResidentRoot(t *testing.T) {
	loader := NewLoaderWithoutRepository(func(string) (*config.Config, error) {
		return &config.Config{}, nil
	}, catalog.Standard, func(context.Context, string) string {
		t.Fatal("resident resolver called before config validation")
		return ""
	})
	_, err := loader.Open(testContext(t), t.TempDir())
	if err == nil || err.Error() != "prefix must not be empty" {
		t.Fatalf("error = %v", err)
	}
}

func TestLoaderOpenValidatesInjectedStandardWorkflowProfiles(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nintegrationBranch: main\n")
	injectedValue := *catalog.Standard
	injectedValue.Skills = maps.Clone(catalog.Standard.Skills)
	broken := injectedValue.Skills["tdd"]
	broken.Profile.Purpose = ""
	injectedValue.Skills["tdd"] = broken
	loader := NewLoaderWithoutRepository(config.Load, &injectedValue, ResolveResidentRoot(func(_ context.Context, root string) string { return root }))
	_, err := loader.Open(testContext(t), root)
	if err == nil || !strings.Contains(err.Error(), "incomplete workflow profile") {
		t.Fatalf("error = %v, want the injected catalog's incomplete workflow profile", err)
	}
}

func TestOpenFallsBackOnUnsafeResidentRoot(t *testing.T) {
	root := gitfixture.InitRepo(t).Root()
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(root, ".awf")); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(external, "config.yaml"), "prefix: example\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if p.roots.Resident != root {
		t.Fatalf("resident root = %q, want invoking root", p.roots.Resident)
	}
}

func TestLoaderOpenDoesNotMutateStandardCatalog(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nintegrationBranch: main\n")
	injectedValue := *catalog.Standard
	injectedValue.Skills = maps.Clone(catalog.Standard.Skills)
	injectedValue.Agents = maps.Clone(catalog.Standard.Agents)
	injectedValue.Docs = maps.Clone(catalog.Standard.Docs)
	injected := &injectedValue
	snapshotValue := injectedValue
	snapshotValue.Skills = maps.Clone(injected.Skills)
	snapshotValue.Agents = maps.Clone(injected.Agents)
	snapshotValue.Docs = maps.Clone(injected.Docs)
	snapshot := &snapshotValue

	loader := NewLoaderWithoutRepository(config.Load, injected, func(_ context.Context, root string) string { return root })
	if _, err := loader.Open(testContext(t), root); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(injected, snapshot) {
		t.Fatal("Loader.Open mutated the injected standard catalog")
	}
}
