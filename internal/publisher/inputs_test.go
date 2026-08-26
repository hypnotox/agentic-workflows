package publisher

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
	"github.com/hypnotox/agentic-workflows/internal/glossary"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/projectstate"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/vocabularycheck"
)

func TestNewRejectsMissingCompositionDependencies(t *testing.T) {
	state := csRepo(t, sampleYAML, map[string]string{})
	cfg := testConfig(state)
	cases := []struct {
		name string
		new  func()
	}{
		{"state", func() { New(nil, cfg, memoryProjectReader{}, project.Version) }},
		{"config", func() { New(state.OutputState(), nil, memoryProjectReader{}, project.Version) }},
		{"reader", func() { New(state.OutputState(), cfg, nil, project.Version) }},
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
	publisher := New(state.OutputState(), cfg, NewFilesystemReader(state.Root()), project.Version)
	before, err := publisher.Plan()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Prefix = "mutated"
	cfg.Profile = "core"
	cfg.Vars["gateCmd"] = "mutated"
	cfg.LocalDocs = append(cfg.LocalDocs, config.LocalDoc{Name: "mutated", Title: "Mutated", Description: "Mutated."})
	after, err := publisher.Plan()
	if err != nil {
		t.Fatalf("caller mutation affected existing Publisher: %v", err)
	}
	if !reflect.DeepEqual(before.Paths(), after.Paths()) || !reflect.DeepEqual(before.Outputs(), after.Outputs()) {
		t.Fatal("caller-owned config mutation changed an existing Publisher plan")
	}
}

func TestPreparationFreezesGeneratedCheckSources(t *testing.T) {
	checkFrozen := func(t *testing.T, mutate func(string)) {
		t.Helper()
		state := csRepo(t, sampleYAML, map[string]string{".awf/skills/tdd.yaml": "data:\n  stale: before\n"})
		prepared, err := New(state.OutputState(), testConfig(state), NewFilesystemReader(state.Root()), project.Version).Prepare()
		if err != nil {
			t.Fatal(err)
		}
		before, err := generatedcheck.Additional(prepared.GeneratedOutput(), prepared.Plan())
		if err != nil {
			t.Fatal(err)
		}
		mutate(state.Root())
		after, err := generatedcheck.Additional(prepared.GeneratedOutput(), prepared.Plan())
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("prepared generated check changed after source mutation: before=%#v after=%#v", before, after)
		}
	}
	t.Run("sidecar key membership", func(t *testing.T) {
		checkFrozen(t, func(root string) {
			if err := os.WriteFile(filepath.Join(root, ".awf/skills/tdd.yaml"), []byte("data:\n  stale: before\n  added-after-prepare: value\n"), 0o644); err != nil {
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
	lower := state.OutputState()
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
	state, err := Open(testContext(t), root)
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
	prepared, err := New(state.OutputState(), testConfig(state), snapshotTreeReader{tree: staged}, project.Version).Prepare()
	if err != nil {
		t.Fatalf("working-tree authority leaked into selected-tree planning: %v", err)
	}
	if len(prepared.ADRs().All()) != 1 || len(prepared.Topics().All()) != 1 {
		t.Fatalf("selected semantic universe = %d ADRs, %d topics", len(prepared.ADRs().All()), len(prepared.Topics().All()))
	}
}

func TestPublisherResidentMarkerPreparationPropagatesPlanningFailure(t *testing.T) {
	state := csRepo(t, sampleYAML, map[string]string{})
	cfg := testConfig(state)
	lower := lowerWithTargets(state.OutputState(), append(state.Targets(), Target{Outputs: []TargetOutput{{TemplateID: "missing/live-template.tmpl"}}}))
	publisher := New(lower, cfg, NewFilesystemReader(state.Root()), project.Version)
	if _, err := publisher.Prepare(); err == nil {
		t.Fatal("resident-marker preparation hid planning failure")
	}
}

func TestSyncPlanningFailurePrecedesInvalidCommandWiring(t *testing.T) {
	state := csRepo(t, sampleYAML, map[string]string{})
	cfg := testConfig(state)
	delete(cfg.Vars, "gateCmd")
	base := state.OutputState()
	selected := base.Catalog()
	missing := selected.Docs["architecture"]
	missing.TID = "missing/live-template.tmpl"
	selected.Docs["missing-live-fixture"] = missing
	lower := projectstate.NewDerivedWithFacts(base.Root(), base.Roots(), base.Nested(), base.Facts(), selected, base.CompleteCatalog(), base.Targets())
	_, err := New(lower, cfg, NewFilesystemReader(state.Root()), project.Version).Sync()
	if err == nil || !strings.Contains(err.Error(), "missing/live-template.tmpl") {
		t.Fatalf("Sync error = %v, want planning error", err)
	}
	if strings.Contains(err.Error(), "vars.gateCmd") {
		t.Fatalf("Sync error = %v, command-wiring error won over planning", err)
	}
}

type plansReaderForTest struct {
	paths   []string
	pathErr error
	files   map[string][]byte
	readErr error
}

func (r plansReaderForTest) Paths(string) ([]string, error) { return r.paths, r.pathErr }
func (r plansReaderForTest) ReadFile(name string) ([]byte, bool, error) {
	if r.readErr != nil {
		return nil, false, r.readErr
	}
	data, ok := r.files[name]
	return data, ok, nil
}

func TestDerivePlansFiltersTreeInputsAndPropagatesReaderFaults(t *testing.T) {
	cases := []struct {
		name string
		read plansReaderForTest
		want string
	}{
		{"paths", plansReaderForTest{pathErr: os.ErrPermission}, "permission denied"},
		{"read", plansReaderForTest{paths: []string{"docs/plans/2026-08-01-read.md"}, readErr: os.ErrPermission}, "read 2026-08-01-read.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := derivePlans(renderInputs{cfg: &config.Config{}, read: tc.read})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("derivePlans error = %v, want %q", err, tc.want)
			}
		})
	}
	plans, err := derivePlans(renderInputs{cfg: &config.Config{}, read: plansReaderForTest{paths: []string{
		"docs/plans/nested/2026-08-01-hidden.md", "docs/plans/2026-08-01-absent.md",
	}}})
	if err != nil || len(plans) != 0 {
		t.Fatalf("filtered plans = %#v, %v", plans, err)
	}
}

func TestPreparationResidentMarkerRejectsAbsentMarker(t *testing.T) {
	if _, err := (Preparation{}).ResidentMarker("effort-archive"); err == nil {
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

func TestPreparationProjectionsAreDeeplyDefensive(t *testing.T) {
	root := topicProject(t)
	writeProjectTopic(t, root, "immutable", "Immutable", "paths: [\"internal/**\"]\n")
	state, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := New(state.OutputState(), testConfig(state), NewFilesystemReader(root), project.Version).Prepare()
	if err != nil {
		t.Fatal(err)
	}

	records := []adr.ADR{{
		Number: "0001", Slug: "immutable", Domains: []string{"rendering"}, Tags: []string{"tag"},
		Related: []int{2}, Sections: map[string]string{"Decision": "before"},
		Operations: []adr.Operation{{Verb: adr.OpAdd, ID: "rendering/immutable:stable", Slug: "stable"}},
		History:    []adr.HistoryEvent{{Date: "2026-08-21", Status: "Implemented", Operations: []adr.Operation{{Verb: adr.OpAdd, ID: "rendering/immutable:stable"}}}},
	}}
	prepared.adrs, err = adr.NewCorpus(records)
	if err != nil {
		t.Fatal(err)
	}
	prepared.pitfalls = pitfall.New([]pitfall.Entry{{
		Slug: "immutable", Domains: []string{"rendering"}, Tags: []string{"tag"}, Related: []int{1}, Source: []byte("source"),
	}})
	prepared.vocabulary = vocabularycheck.Input{
		GlossaryEnabled: true,
		Authored:        []glossary.Record{{Term: "authored", Meaning: "meaning", Domains: []string{"rendering"}}},
		Merged:          []glossary.Record{{Term: "merged", Meaning: "meaning", Domains: []string{"rendering"}}},
		Domains:         []string{"rendering"}, Tags: map[string]string{"tag": "meaning"}, Pitfalls: prepared.pitfalls,
	}
	prepared.plans = []plan.Plan{{
		ADRs: []plan.ADRLink{{Number: 1}}, Source: []byte("source"), DoD: []plan.DoDItem{{Slug: "done"}},
		CommitSubjects: []string{"subject"}, Phases: []plan.Phase{{
			Advances: []string{"advance"}, Completes: []string{"complete"}, Tasks: []plan.Task{{Fields: plan.TaskFields{
				Paths: []plan.PathEntry{{Value: "path"}}, Applying: []plan.DecisionRef{{ADR: "0001"}}, Context: []plan.DecisionRef{{ADR: "0001"}},
			}}},
		}},
	}}
	focused := ContextPreparation{adrs: prepared.adrs, topics: prepared.topics, plans: prepared.plans, declarations: prepared.Plan().Declarations()}

	beforeADRs, beforePitfalls, beforeTopics := prepared.ADRs(), prepared.Pitfalls(), prepared.Topics()
	beforeSkills, beforePlans, beforePlan := prepared.EffectiveSkills(), prepared.Plans(), prepared.Plan()
	beforeVocabulary := prepared.Vocabulary()
	beforeFocusedADRs, beforeFocusedTopics := focused.ADRs(), focused.Topics()
	beforeFocusedPlans, beforeFocusedDeclarations := focused.Plans(), focused.Declarations()

	projectedADRs := prepared.ADRs().All()
	projectedADRs[0].Domains[0] = "mutated"
	projectedADRs[0].Tags[0] = "mutated"
	projectedADRs[0].Related[0] = 99
	projectedADRs[0].Sections["Decision"] = "mutated"
	projectedADRs[0].Operations[0].ID = "mutated"
	projectedADRs[0].History[0].Status = "mutated"
	projectedADRs[0].History[0].Operations[0].ID = "mutated"

	projectedPitfalls := prepared.Pitfalls().All()
	projectedPitfalls[0].Domains[0] = "mutated"
	projectedPitfalls[0].Tags[0] = "mutated"
	projectedPitfalls[0].Related[0] = 99
	projectedPitfalls[0].Source[0] = 'X'

	projectedTopics := prepared.Topics()
	projectedTopics.DomainPaths["rendering"][0] = "mutated"
	allTopics := projectedTopics.All()
	allTopics[0].Metadata.Paths[0] = "mutated"
	allTopics[0].Claims[0].RevisedBy = append(allTopics[0].Claims[0].RevisedBy, "mutated")
	allTopics[0].Claims[0].References = append(allTopics[0].Claims[0].References, "mutated")

	projectedSkills := prepared.EffectiveSkills()
	for skill := range projectedSkills {
		projectedSkills[skill] = !projectedSkills[skill]
		projectedSkills["mutated"] = true
		break
	}
	projectedPlans := prepared.Plans()
	projectedPlans[0].ADRs[0].Number = 99
	projectedPlans[0].Source[0] = 'X'
	projectedPlans[0].DoD[0].Slug = "mutated"
	projectedPlans[0].CommitSubjects[0] = "mutated"
	projectedPlans[0].Phases[0].Advances[0] = "mutated"
	projectedPlans[0].Phases[0].Completes[0] = "mutated"
	projectedPlans[0].Phases[0].Tasks[0].Fields.Paths[0].Value = "mutated"
	projectedPlans[0].Phases[0].Tasks[0].Fields.Applying[0].ADR = "mutated"
	projectedPlans[0].Phases[0].Tasks[0].Fields.Context[0].ADR = "mutated"

	focusedADRs := focused.ADRs().All()
	focusedADRs[0].Sections["Decision"] = "mutated"
	focusedADRs[0].History[0].Operations[0].ID = "mutated"
	focusedTopics := focused.Topics()
	focusedTopics.DomainPaths["rendering"][0] = "mutated"
	focusedTopics.All()[0].Claims[0].RevisedBy = append(focusedTopics.All()[0].Claims[0].RevisedBy, "mutated")
	focusedPlans := focused.Plans()
	focusedPlans[0].Source[0] = 'X'
	focusedPlans[0].Phases[0].Tasks[0].Fields.Paths[0].Value = "mutated"
	focusedDeclarations := focused.Declarations()
	if len(focusedDeclarations) == 0 {
		t.Fatal("focused preparation has no declarations to test")
	}
	focusedDeclarations[0] = focusedDeclarations[len(focusedDeclarations)-1]
	for _, values := range [][]string{focusedDeclarations[0].Declarers()} {
		if len(values) > 0 {
			values[0] = "mutated"
		}
	}
	inputs := focusedDeclarations[0].Inputs()
	if len(inputs) > 0 {
		inputs[0] = inputs[len(inputs)-1]
	}

	projectedVocabulary := prepared.Vocabulary()
	projectedVocabulary.Authored[0].Domains[0] = "mutated"
	projectedVocabulary.Merged[0].Domains[0] = "mutated"
	projectedVocabulary.Domains[0] = "mutated"
	projectedVocabulary.Tags["tag"] = "mutated"
	projectedVocabulary.Pitfalls.All()[0].Tags[0] = "mutated"

	// outputplan has no exported mutable fields. Every slice-valued query is a
	// defensive copy, so mutate each outward slice and each nested slice query.
	planProjection := prepared.Plan()
	paths := planProjection.Paths()
	if len(paths) > 0 {
		paths[0] = "mutated"
	}
	nodes := planProjection.Nodes()
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
	declarations := planProjection.Declarations()
	if len(declarations) > 0 {
		declarations[0] = declarations[len(declarations)-1]
		for _, values := range [][]string{declarations[0].Declarers()} {
			if len(values) > 0 {
				values[0] = "mutated"
			}
		}
		inputs := declarations[0].Inputs()
		if len(inputs) > 0 {
			inputs[0] = inputs[len(inputs)-1]
		}
	}
	outputs := planProjection.Outputs()
	if len(outputs) > 0 {
		outputs[0] = outputs[len(outputs)-1]
	}

	for name, values := range map[string][2]any{
		"ADRs":                 {prepared.ADRs(), beforeADRs},
		"Pitfalls":             {prepared.Pitfalls(), beforePitfalls},
		"Topics":               {prepared.Topics(), beforeTopics},
		"EffectiveSkills":      {prepared.EffectiveSkills(), beforeSkills},
		"Plans":                {prepared.Plans(), beforePlans},
		"Plan":                 {prepared.Plan(), beforePlan},
		"Vocabulary":           {prepared.Vocabulary(), beforeVocabulary},
		"Context ADRs":         {focused.ADRs(), beforeFocusedADRs},
		"Context Topics":       {focused.Topics(), beforeFocusedTopics},
		"Context Plans":        {focused.Plans(), beforeFocusedPlans},
		"Context Declarations": {focused.Declarations(), beforeFocusedDeclarations},
	} {
		if !reflect.DeepEqual(values[0], values[1]) {
			t.Errorf("mutating the %s projection changed a second projection or Publisher-owned state", name)
		}
	}
}
