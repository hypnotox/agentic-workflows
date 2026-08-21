package publisher

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
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
