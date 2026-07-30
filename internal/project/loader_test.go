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
	loader := NewLoader(func(path string) (*config.Config, error) {
		gotPath = path
		return nil, sentinel
	}, catalog.Standard, func(context.Context, string) string {
		t.Fatal("resident resolver called after load failure")
		return ""
	}, nil)
	_, err := loader.Open(testContext(t), root)
	if gotPath != config.RootDir(root) {
		t.Fatalf("load path = %q, want %q", gotPath, config.RootDir(root))
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want sentinel", err)
	}
}

func TestLoaderOpenRejectsNilLoadedConfig(t *testing.T) {
	loader := NewLoader(func(string) (*config.Config, error) {
		var cfg *config.Config
		return cfg, nil
	}, catalog.Standard, func(context.Context, string) string {
		t.Fatal("resident resolver called after nil config")
		return ""
	}, nil)
	_, err := loader.Open(testContext(t), t.TempDir())
	if err == nil || err.Error() != "project Loader: load config tree returned nil config" {
		t.Fatalf("error = %v", err)
	}
}

func TestLoaderOpenValidatesBeforeResolvingResidentRoot(t *testing.T) {
	loader := NewLoader(func(string) (*config.Config, error) {
		return &config.Config{}, nil
	}, catalog.Standard, func(context.Context, string) string {
		t.Fatal("resident resolver called before config validation")
		return ""
	}, nil)
	_, err := loader.Open(testContext(t), t.TempDir())
	if err == nil || err.Error() != "prefix must not be empty" {
		t.Fatalf("error = %v", err)
	}
}

func TestLoaderOpenResolvesTargetsBeforeResidentRoot(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nskills: []\nagents: []\ntargets: [unknown]\n")
	loader := NewLoader(config.Load, catalog.Standard, func(context.Context, string) string {
		t.Fatal("resident resolver called before target resolution")
		return ""
	}, nil)
	_, err := loader.Open(testContext(t), root)
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("error = %v, want unknown target", err)
	}
}

// invariant: code-design/dependency-composition:sync-project-loader-wiring
func TestLoaderOpenUsesSemanticResidentRoot(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nskills: []\nagents: []\ntargets: [claude]\n")
	resident := filepath.Join(root, "resident")
	var resolved string
	injectedStandard := catalog.Standard
	loader := NewLoader(config.Load, injectedStandard, func(_ context.Context, got string) string {
		resolved = got
		return resident
	}, nil)
	p, err := loader.Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != root || p.Root != root || p.residentRoot != resident {
		t.Fatalf("roots: resolved=%q root=%q resident=%q", resolved, p.Root, p.residentRoot)
	}
	if len(p.Targets) != 1 || p.Targets[0].Name != "claude" {
		t.Fatalf("targets = %#v", p.Targets)
	}
	if p.Cat == injectedStandard {
		t.Fatal("effective catalog aliases injected standard")
	}
	if !reflect.DeepEqual(p.Cat.Skills["tdd"], injectedStandard.Skills["tdd"]) {
		t.Fatal("effective catalog did not retain standard tdd skill")
	}
}

func TestLoaderOpenReturnsEffectiveCatalogError(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nskills: [local]\nagents: []\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "skills", "local.yaml"), "local: [bad\n")
	loader := NewLoader(config.Load, catalog.Standard, func(_ context.Context, root string) string { return root }, nil)
	_, err := loader.Open(testContext(t), root)
	if err == nil || !strings.Contains(err.Error(), "skills/local.yaml") {
		t.Fatalf("error = %v, want skills/local.yaml", err)
	}
}

func TestLoaderOpenUsesInjectedStandardCatalog(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nskills: [tdd]\nagents: []\n")
	injectedValue := *catalog.Standard
	injectedValue.Skills = maps.Clone(catalog.Standard.Skills)
	delete(injectedValue.Skills, "tdd")
	loader := NewLoader(config.Load, &injectedValue, func(_ context.Context, root string) string { return root }, nil)
	_, err := loader.Open(testContext(t), root)
	if err == nil || !strings.Contains(err.Error(), "tdd") {
		t.Fatalf("error = %v, want injected catalog to reject tdd", err)
	}
}

func TestLoaderOpenReturnsConformanceError(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nskills: [unknown]\nagents: []\n")
	loader := NewLoader(config.Load, catalog.Standard, func(_ context.Context, root string) string { return root }, nil)
	_, err := loader.Open(testContext(t), root)
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("error = %v, want unknown skill", err)
	}
}

func TestOpenFallsBackOnUnsafeResidentRoot(t *testing.T) {
	_, root := gitfixture.InitRepo(t)
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(root, ".awf")); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(external, "config.yaml"), "prefix: example\nskills: []\nagents: []\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if p.residentRoot != root {
		t.Fatalf("resident root = %q, want invoking root", p.residentRoot)
	}
}

func TestLoaderOpenDoesNotMutateStandardCatalog(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nskills: []\nagents: []\n")
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

	loader := NewLoader(config.Load, injected, func(_ context.Context, root string) string { return root }, nil)
	if _, err := loader.Open(testContext(t), root); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(injected, snapshot) {
		t.Fatal("Loader.Open mutated the injected standard catalog")
	}
}
