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
	"github.com/hypnotox/agentic-workflows/internal/resident"
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
		return &config.Config{Profile: catalog.ProfileFull}, nil
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
	testsupport.WriteAwfConfig(t, root, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	injectedValue := *catalog.Standard
	injectedValue.Skills = maps.Clone(catalog.Standard.Skills)
	broken := injectedValue.Skills["tdd"]
	broken.Profile.Purpose = ""
	injectedValue.Skills["tdd"] = broken
	loader := NewLoaderWithoutRepository(config.Load, &injectedValue, ResolveResidentRoot(func(_ context.Context, root string) string { return root }))
	_, err := loader.Open(testContext(t), root)
	if err == nil || !strings.Contains(err.Error(), "incomplete workflow metadata") {
		t.Fatalf("error = %v, want the injected catalog's incomplete workflow metadata", err)
	}
}

func TestOpenFallsBackOnUnsafeResidentRoot(t *testing.T) {
	root := gitfixture.InitRepo(t).Root()
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(root, ".awf")); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(external, "config.yaml"), "prefix: example\nprofile: full\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if p.roots().Resident != root {
		t.Fatalf("resident root = %q, want invoking root", p.roots().Resident)
	}
}

func TestLoaderOpenOwnsInjectedCompleteView(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	injected := *catalog.Standard
	injected.Skills = maps.Clone(catalog.Standard.Skills)
	loader := NewLoaderWithoutRepository(config.Load, &injected, func(_ context.Context, root string) string { return root })
	p, err := loader.Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if p.catalog() == &injected || !reflect.DeepEqual(p.catalog(), &injected) {
		t.Fatal("ProjectState did not own one equivalent injected catalog snapshot")
	}
	second, err := loader.Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	firstSkill := p.catalog().Skills["tdd"]
	firstSkill.Sections[0] = "changed first project"
	p.catalog().Skills["tdd"] = firstSkill
	if projectCatalog(renderInputsForTest(second)).Skills["tdd"].Sections[0] == "changed first project" {
		t.Fatal("Loader-opened projects share a mutable catalog snapshot")
	}
}

func TestProjectStateDefensivelyOwnsTargetSnapshots(t *testing.T) {
	source := []Target{{
		Name: "target", Capabilities: []Capability{CapabilitySubagentTools},
		Outputs: []TargetOutput{{Path: "output", Inputs: []TargetOutputInput{{Path: "input", Role: ArtifactTemplate}}}},
	}}
	state, err := newProjectState("root", resident.NewRoots("root", "resident"), false, &config.Config{}, catalog.Standard, catalog.Standard, source)
	if err != nil {
		t.Fatal(err)
	}
	source[0].Capabilities[0] = CapabilitySessionHandoff
	source[0].Outputs[0].Path = "mutated"
	source[0].Outputs[0].Inputs[0].Path = "mutated"
	first := state.Targets()
	if first[0].Capabilities[0] != CapabilitySubagentTools || first[0].Outputs[0].Path != "output" || first[0].Outputs[0].Inputs[0].Path != "input" {
		t.Fatalf("project state retained a target construction alias: %#v", first)
	}
	first[0].Capabilities[0] = CapabilityEffortSessions
	first[0].Outputs[0].Path = "returned mutation"
	first[0].Outputs[0].Inputs[0].Path = "returned mutation"
	second := state.Targets()
	if second[0].Capabilities[0] != CapabilitySubagentTools || second[0].Outputs[0].Path != "output" || second[0].Outputs[0].Inputs[0].Path != "input" {
		t.Fatalf("resolvedTargets returned a nested alias: %#v", second)
	}
}

func TestLoaderOpenDoesNotMutateStandardCatalog(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nprofile: full\nintegrationBranch: main\n")
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

func TestLoaderRejectsUnsupportedConfigFactData(t *testing.T) {
	type unsupported struct{ values []string }
	loader := NewLoaderWithoutRepository(func(string) (*config.Config, error) {
		return &config.Config{Prefix: "example", Profile: catalog.ProfileFull, IntegrationBranch: "main", Vars: map[string]any{"bad": unsupported{values: []string{"mutable"}}}}, nil
	}, catalog.Standard, func(_ context.Context, root string) string { return root })
	_, err := loader.Open(testContext(t), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "unsupported semantic data type") {
		t.Fatalf("Loader.Open error = %v", err)
	}
}

func TestLoaderStateDefensivelyOwnsLoadedFacts(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nprofile: full\nintegrationBranch: main\ndomains: [tooling]\ntags: {tag: meaning}\nvars: {nested: {items: [original]}}\n")
	p, err := NewLoaderWithoutRepository(config.Load, catalog.Standard, func(_ context.Context, root string) string { return root }).Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	p.Config().Domains[0] = "mutated"
	p.Config().Tags["tag"] = "mutated"
	p.Config().Vars["nested"].(map[string]any)["items"].([]any)[0] = "mutated"
	p.Targets()[0].Capabilities = append(p.Targets()[0].Capabilities, CapabilitySubagentTools)
	p.catalog().Skills["tdd"] = catalog.SkillSpec{Sections: []string{"mutated"}}
	got := p.Config()
	if got.Domains[0] != "tooling" || got.Tags["tag"] != "meaning" || got.Vars["nested"].(map[string]any)["items"].([]any)[0] != "original" {
		t.Fatalf("state retained a Loader input alias: %#v", got)
	}
	returnedTargets := p.Targets()
	wantCapabilities := len(returnedTargets[1].Capabilities)
	returnedTargets[1].Capabilities = append(returnedTargets[1].Capabilities, CapabilitySubagentTools)
	if len(p.Targets()[1].Capabilities) != wantCapabilities {
		t.Fatal("resolved target accessor returned an alias")
	}
	returnedCatalog := p.catalog()
	first := returnedCatalog.Skills["tdd"]
	first.Sections[0] = "returned mutation"
	returnedCatalog.Skills["tdd"] = first
	if p.catalog().Skills["tdd"].Sections[0] == "returned mutation" {
		t.Fatal("catalog accessor returned an alias")
	}
	complete := p.completeCatalog()
	complete.Skills["tdd"] = catalog.SkillSpec{Sections: []string{"complete mutation"}}
	if p.completeCatalog().Skills["tdd"].Sections[0] == "complete mutation" {
		t.Fatal("complete catalog accessor returned an alias")
	}
	if p.Root() != root || p.roots().Tracked != root || p.nested() || p.Config().Source() != nil {
		t.Fatal("Loader did not construct the expected fact state")
	}
}
