package publisher

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
	"github.com/hypnotox/agentic-workflows/internal/glossary"
	"github.com/hypnotox/agentic-workflows/internal/glossarycheck"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func TestNewRejectsMissingCompositionDependencies(t *testing.T) {
	state := csRepo(t, sampleYAML, map[string]string{})
	cfg := testConfig(state)
	cases := []struct {
		name string
		new  func()
	}{
		{"state", func() { newPublisher(nil, cfg, memoryProjectReader{}, project.Version) }},
		{"config", func() { newPublisher(state, nil, memoryProjectReader{}, project.Version) }},
		{"reader", func() { newPublisher(state, cfg, nil, project.Version) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("New accepted a missing composition dependency")
				}
			}()
			tc.new()
		})
	}
}

func TestPublisherDefensivelyOwnsConfigurationFacts(t *testing.T) {
	state := csRepo(t, sampleYAML, map[string]string{})
	cfg := testConfig(state)
	publisher := newPublisher(state, cfg, NewFilesystemReader(state.Root()), project.Version)
	before, err := publisher.Plan()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Prefix = "mutated"
	cfg.Vars["gateCmd"] = "mutated"
	cfg.LocalDocs = append(cfg.LocalDocs, config.LocalDoc{Name: "mutated", Title: "Mutated", Description: "Mutated."})
	after, err := publisher.Plan()
	if err != nil {
		t.Fatalf("caller mutation affected existing Publisher: %v", err)
	}
	if !reflect.DeepEqual(before.Outputs(), after.Outputs()) {
		t.Fatal("caller-owned config mutation changed an existing Publisher plan")
	}
}

func TestPublisherOperationIsFrozenWhileNewOperationReadsFreshInPlaceContent(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nlocalDocs:\n  - name: runbooks/incident\n    title: Incident\n    description: Handle incidents.\n")
	const outputPath = "docs/runbooks/incident.md"
	contentAt := func(t *testing.T, operation *Publisher) string {
		t.Helper()
		plan, err := operation.Plan()
		if err != nil {
			t.Fatal(err)
		}
		for _, output := range plan.Outputs() {
			if output.Path() == outputPath {
				return output.Content()
			}
		}
		t.Fatalf("missing output %s", outputPath)
		return ""
	}
	state, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	seed := contentAt(t, New(state, project.Version))
	const marker = "<!-- awf:edit-in-place body -->\n\n"
	beforeOnDisk := strings.Replace(seed, marker, marker+"before\n", 1)
	if beforeOnDisk == seed {
		t.Fatal("local document seed has no in-place marker")
	}
	testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(outputPath)), beforeOnDisk)
	state, err = loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	frozen := New(state, project.Version)
	before := contentAt(t, frozen)
	testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(outputPath)), strings.Replace(before, "before", "after", 1))
	if got := contentAt(t, frozen); got != before || !strings.Contains(got, "before") {
		t.Fatalf("existing operation was not frozen: %q", got)
	}
	freshState, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if got := contentAt(t, New(freshState, project.Version)); !strings.Contains(got, "after") || got == before {
		t.Fatalf("new operation did not read fresh in-place content: %q", got)
	}
}

func TestPublisherRefusesPublicationAfterReadOnlyMaterialization(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\n")
	state, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	operation := New(state, project.Version)
	if _, err := operation.Plan(); err != nil {
		t.Fatal(err)
	}
	if _, err := operation.SyncLeased(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "materialized outside publication") {
		t.Fatalf("stale operation publication error = %v", err)
	}
}

func TestPublisherOperationFreezesGeneratedCheckSources(t *testing.T) {
	checkFrozen := func(t *testing.T, mutate func(string)) {
		t.Helper()
		state := csRepo(t, sampleYAML, map[string]string{".awf/skills/awf-maintenance.yaml": "data:\n  stale: before\n"})
		prepared, err := newPublisher(state, testConfig(state), NewFilesystemReader(state.Root()), project.Version).operationState()
		if err != nil {
			t.Fatal(err)
		}
		before, err := generatedcheck.Additional(cloneGeneratedOutput(prepared.generated), prepared.plan)
		if err != nil {
			t.Fatal(err)
		}
		mutate(state.Root())
		after, err := generatedcheck.Additional(cloneGeneratedOutput(prepared.generated), prepared.plan)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("prepared generated check changed after source mutation: before=%#v after=%#v", before, after)
		}
	}
	t.Run("sidecar key membership", func(t *testing.T) {
		checkFrozen(t, func(root string) {
			if err := os.WriteFile(filepath.Join(root, ".awf/skills/awf-maintenance.yaml"), []byte("data:\n  stale: before\n  added-after-prepare: value\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		})
	})
	t.Run("closed tree entries", func(t *testing.T) {
		checkFrozen(t, func(root string) {
			if err := os.WriteFile(filepath.Join(root, ".awf/after-prepare"), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		})
	})
}

func TestRenderInputsSnapshotsCatalogOnce(t *testing.T) {
	state := csRepo(t, sampleYAML, map[string]string{})
	lower := state
	inputs := newRenderInputs(lower, testConfig(state), memoryProjectReader{}, project.Version)

	first := projectCatalog(inputs)
	second := projectCatalog(inputs)
	if first != second {
		t.Fatal("one render operation repeatedly cloned its immutable catalog snapshot")
	}

	projected := lower.Catalog()
	delete(projected.Docs, "architecture")
	if _, ok := projectCatalog(inputs).Docs["architecture"]; !ok {
		t.Fatal("caller-owned catalog projection mutated the render operation snapshot")
	}
}

func TestPublisherStagedTreeOwnsADRAndTopicDerivation(t *testing.T) {
	root := topicProject(t)
	writeProjectTopic(t, root, "selected-tree", "Selected tree", "paths: [\"internal/**\"]\n")
	state, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	filesystem := NewFilesystemReader(root)
	paths, err := filesystem.Paths("")
	if err != nil {
		t.Fatal(err)
	}
	selected := memoryProjectReader{}
	for _, path := range paths {
		data, found, err := filesystem.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if found {
			selected[path] = data
		}
	}
	files := make([]snapshot.File, 0, len(selected))
	for path, data := range selected {
		files = append(files, snapshot.File{Path: path, Mode: snapshot.Regular, Bytes: data})
	}
	staged, err := snapshot.NewTree(files)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs/decisions/0001-topic.md"), []byte("working-tree ADR is malformed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf/topics/metadata/rendering/selected-tree.yaml"), []byte("working: [malformed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prepared, err := newPublisher(state, testConfig(state), snapshotTreeReader{tree: staged}, project.Version).operationState()
	if err != nil {
		t.Fatalf("working-tree authority leaked into selected-tree planning: %v", err)
	}
	if len(prepared.topics.All()) != 1 {
		t.Fatalf("selected topic universe = %d", len(prepared.topics.All()))
	}
}

func TestPublisherResidentMarkerPropagatesPlanningFailure(t *testing.T) {
	state := csRepo(t, sampleYAML, map[string]string{})
	cfg := testConfig(state)
	lower := lowerWithTargets(state, append(state.Targets(), artifactregistry.Target{Name: "broken", SkillDir: ".broken/skills", BridgeFile: "BROKEN.md", BridgeTemplate: "missing/live-template.tmpl"}))
	publisher := newPublisher(lower, cfg, NewFilesystemReader(state.Root()), project.Version)
	if _, err := publisher.operationState(); err == nil {
		t.Fatal("resident-marker preparation hid planning failure")
	}
}

func TestSyncUsesTheSessionsSelectedConfigurationWithoutFactsRebinding(t *testing.T) {
	state := csRepo(t, sampleYAML, map[string]string{})
	cfg := testConfig(state)
	delete(cfg.Vars, "gateCmd")
	base := state
	selected := base.Catalog()
	missing := selected.Docs["architecture"]
	missing.TID = "missing/live-template.tmpl"
	selected.Docs["missing-live-fixture"] = missing
	lower := deriveSession(base, cfg, NewFilesystemReader(state.Root()), selected, base.Targets())
	_, err := New(lower, project.Version).SyncLeased(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "vars.gateCmd") {
		t.Fatalf("Sync error = %v, want selected-session command-wiring error", err)
	}
	if strings.Contains(err.Error(), "missing/live-template.tmpl") {
		t.Fatalf("Sync error = %v, stale state facts overrode the selected Session config", err)
	}
}

func TestPublisherResidentMarkerRejectsAbsentMarker(t *testing.T) {
	state := csRepo(t, sampleYAML, map[string]string{})
	operation := newPublisher(state, testConfig(state), NewFilesystemReader(state.Root()), project.Version)
	if _, err := operation.ResidentMarker("absent"); err == nil {
		t.Fatal("absent resident marker was accepted")
	}
}

func TestFilesystemReaderConfinesPathsToInvokingOperationTree(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		".awf/config.yaml":                       "root config\n",
		"internal/root.go":                       "root marker\n",
		".git/config":                            "root git metadata\n",
		"nested-dir/.git/config":                 "nested git metadata\n",
		"nested-dir/internal/leak.go":            "nested checkout marker\n",
		"nested-file/.git":                       "gitdir: ../metadata\n",
		"nested-file/internal/leak.go":           "linked checkout marker\n",
		"nested-adopter/.awf/config.yaml":        "nested config\n",
		"nested-adopter/.awf/topics/leak.yaml":   "nested authority\n",
		"nested-adopter/internal/leak.go":        "nested adopter marker\n",
		"ordinary/.cache/internal-deliberate.go": "ordinary hidden content\n",
	}
	for name, content := range files {
		testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(name)), content)
	}
	paths, err := NewFilesystemReader(root).Paths("")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".awf/config.yaml", "internal/root.go", "ordinary/.cache/internal-deliberate.go"}
	if !slices.Equal(paths, want) {
		t.Fatalf("operation paths = %q, want %q", paths, want)
	}
}

func TestFilesystemReaderKeepsInvokingGitfileProjectSelected(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".git"), "gitdir: elsewhere\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/config.yaml"), "root config\n")
	testsupport.WriteFile(t, filepath.Join(root, "internal/root.go"), "root marker\n")
	paths, err := NewFilesystemReader(root).Paths("")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{".awf/config.yaml", "internal/root.go"}; !slices.Equal(paths, want) {
		t.Fatalf("linked-checkout operation paths = %q, want %q", paths, want)
	}
}

func TestPublisherOperationProjectionsAreDeeplyDefensive(t *testing.T) {
	root := topicProject(t)
	writeProjectTopic(t, root, "immutable", "Immutable", "paths: [\"internal/**\"]\n")
	state, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	publisher := newPublisher(state, testConfig(state), NewFilesystemReader(root), project.Version)
	prepared, err := publisher.operationState()
	if err != nil {
		t.Fatal(err)
	}

	prepared.pitfalls = pitfall.New([]pitfall.Entry{{
		Slug: "immutable", Domains: []string{"rendering"}, Source: []byte("source"),
	}})
	prepared.glossary = glossarycheck.Input{
		Enabled:  true,
		Authored: []glossary.Record{{Term: "authored", Meaning: "meaning", Domains: []string{"rendering"}}},
		Merged:   []glossary.Record{{Term: "merged", Meaning: "meaning", Domains: []string{"rendering"}}},
		Domains:  []string{"rendering"},
	}
	pitfallsProjection := func() pitfall.Corpus {
		value, err := publisher.Pitfalls()
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	topicsProjection := func() topic.Corpus { return prepared.topics.Clone() }
	planProjection := func() outputplan.Plan {
		value, err := publisher.Plan()
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	glossaryProjection := func() glossarycheck.Input {
		value, err := publisher.Glossary()
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	beforePitfalls, beforeTopics := pitfallsProjection(), topicsProjection()
	beforePlan := planProjection()
	beforeGlossary := glossaryProjection()

	projectedPitfalls := pitfallsProjection().All()
	projectedPitfalls[0].Domains[0] = "mutated"
	projectedPitfalls[0].Source[0] = 'X'

	projectedTopics := topicsProjection()
	projectedTopics.DomainPaths["rendering"][0] = "mutated"
	allTopics := projectedTopics.All()
	allTopics[0].Metadata.Paths[0] = "mutated"
	allTopics[0].Claims[0].References = append(allTopics[0].Claims[0].References, "mutated")

	projectedGlossary := glossaryProjection()
	projectedGlossary.Authored[0].Domains[0] = "mutated"
	projectedGlossary.Merged[0].Domains[0] = "mutated"
	projectedGlossary.Domains[0] = "mutated"

	// outputplan has no exported mutable fields. Every slice-valued query is a
	// defensive copy, so mutate each outward slice and each nested slice query.
	projectedPlan := planProjection()
	nodes := projectedPlan.Nodes()
	if len(nodes) > 0 {
		nodes[0] = nodes[len(nodes)-1]
		declarers := nodes[0].Declarers()
		if len(declarers) > 0 {
			declarers[0] = "mutated"
		}
		if output, ok := nodes[0].Output(); ok {
			for _, values := range [][]string{output.StubDefaults(), output.StubParts(), output.MarkerParts(), output.PartVarRefs()} {
				if len(values) > 0 {
					values[0] = "mutated"
				}
			}
		}
	}
	outputs := projectedPlan.Outputs()
	if len(outputs) > 0 {
		outputs[0] = outputs[len(outputs)-1]
	}

	for name, values := range map[string][2]any{
		"Pitfalls": {pitfallsProjection(), beforePitfalls},
		"Topics":   {topicsProjection(), beforeTopics},
		"Plan":     {planProjection(), beforePlan},
		"Glossary": {glossaryProjection(), beforeGlossary},
	} {
		if !reflect.DeepEqual(values[0], values[1]) {
			t.Errorf("mutating the %s projection changed a second projection or Publisher-owned state", name)
		}
	}
}
