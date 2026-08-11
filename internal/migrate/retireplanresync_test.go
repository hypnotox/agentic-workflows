package migrate

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"gopkg.in/yaml.v3"
)

func TestRetirePlanResyncGenerationRegistration(t *testing.T) {
	if Current() != templateSourceGeneration {
		t.Fatalf("Current() = %d, want %d", Current(), templateSourceGeneration)
	}
	last := registry[len(registry)-1]
	if last.To != templateSourceGeneration || last.Name != "template-source-root" {
		t.Fatalf("last migration = %#v", last)
	}
}

// invariant: config/migrations-and-locks:retired-plan-resync-selection-migration (TestRemovePlanResyncSelection)
func TestRemovePlanResyncSelection(t *testing.T) {
	tests := []struct {
		name, src, want string
		removed         bool
	}{
		{"block", "prefix: ex\nskills:\n  - reviewing-plan\n  - reviewing-plan-resync\n  - writing-plans\nvars:\n  literal: keep\n", "prefix: ex\nskills:\n  - reviewing-plan\n  - writing-plans\nvars:\n  literal: keep\n", true},
		{"flow", "prefix: ex\nskills: [reviewing-plan, reviewing-plan-resync, writing-plans]\n", "prefix: ex\nskills:\n  - reviewing-plan\n  - writing-plans\n", true},
		{"sole", "prefix: ex\nskills: [reviewing-plan-resync]\n", "prefix: ex\nskills: []\n", true},
		{"duplicates", "prefix: ex\nskills: [reviewing-plan-resync, reviewing-plan, reviewing-plan-resync]\n", "prefix: ex\nskills:\n  - reviewing-plan\n", true},
		{"empty", "prefix: ex\nskills: []\n", "prefix: ex\nskills: []\n", false},
		{"absent member", "prefix: ex\nskills: [reviewing-plan]\n", "prefix: ex\nskills: [reviewing-plan]\n", false},
		{"absent key", "prefix: ex\nvars: {literal: keep}\n", "prefix: ex\nvars: {literal: keep}\n", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, removed, err := removePlanResyncSelection([]byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want || removed != tc.removed {
				t.Fatalf("got (%t) %q, want (%t) %q", removed, got, tc.removed, tc.want)
			}
			again, removed, err := removePlanResyncSelection(got)
			if err != nil || removed || string(again) != tc.want {
				t.Fatalf("repeat got (%t) %q, err %v", removed, again, err)
			}
		})
	}
	for _, malformed := range []string{"prefix: [\n", "prefix: ex\nskills: reviewing-plan-resync\n"} {
		if _, _, err := removePlanResyncSelection([]byte(malformed)); err == nil {
			t.Fatalf("malformed input accepted: %q", malformed)
		}
	}
}

func TestRemovePlanResyncSelectionResolvesAliases(t *testing.T) {
	for _, tc := range []struct {
		name, src string
	}{
		{"anchor in skills", "prefix: ex\nskills: [&retired reviewing-plan-resync, *retired, reviewing-plan]\nvars:\n  literal: *retired\n"},
		{"anchor outside skills", "prefix: ex\nvars: {literal: &retired reviewing-plan-resync}\nskills: [*retired, reviewing-plan]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, removed, err := removePlanResyncSelection([]byte(tc.src))
			if err != nil || !removed {
				t.Fatalf("remove = %t, %v", removed, err)
			}
			var decoded struct {
				Skills []string          `yaml:"skills"`
				Vars   map[string]string `yaml:"vars"`
			}
			if err := yaml.Unmarshal(got, &decoded); err != nil {
				t.Fatalf("invalid migrated YAML %q: %v", got, err)
			}
			if !reflect.DeepEqual(decoded.Skills, []string{"reviewing-plan"}) || decoded.Vars["literal"] != retiredPlanResyncSkill {
				t.Fatalf("decoded = %#v", decoded)
			}
			again, removed, err := removePlanResyncSelection(got)
			if err != nil || removed || !reflect.DeepEqual(again, got) {
				t.Fatalf("repeat = %t, %q, %v", removed, again, err)
			}
		})
	}
}

// invariant: config/migrations-and-locks:retired-plan-resync-selection-migration (TestRetirePlanResyncMigrationReportsAndStamps)
func TestRetirePlanResyncMigrationReportsAndStamps(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "prefix: ex\nintegrationBranch: main\nvars: {}\nskills: [reviewing-plan-resync, reviewing-plan, reviewing-plan-resync]\nagents: [plan-reviewer]\ndocs: [workflow]\ntargets: [claude]\ndocsDir: docs\n")
	skillSidecar := filepath.Join(root, ".awf", "skills", "tdd.yaml")
	docSidecar := filepath.Join(root, ".awf", "docs", "guide.yaml")
	testsupport.WriteFile(t, skillSidecar, "local: false\npurpose: keep\n")
	testsupport.WriteFile(t, docSidecar, "local: false\npath: guide.md\n")
	if err := os.Chmod(skillSidecar, 0o640); err != nil {
		t.Fatal(err)
	}
	stampLockAt(t, filepath.Join(root, ".awf", "awf.lock"), 39)
	applied, changes, err := Upgrade(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(applied, []string{"retire-plan-resync-selection", "global-topic-path-ownership", "effort-archive-root", "pitfall-corpus", "template-source-root"}) {
		t.Fatalf("applied = %v", applied)
	}
	texts := make([]string, len(changes))
	for i, change := range changes {
		texts[i] = change.Text
	}
	if !reflect.DeepEqual(texts, []string{
		"retire-plan-resync: removed reviewing-plan-resync from skills",
		"drop-selection: removed skills",
		"drop-selection: removed agents",
		"drop-selection: removed docs",
		"drop-selection: removed targets",
		"drop-selection: removed docsDir",
		"drop-selection: removed local from " + filepath.ToSlash(skillSidecar),
		"drop-selection: removed local from " + filepath.ToSlash(docSidecar),
		"schema-stamp: updated awf.lock schema version",
	}) {
		t.Fatalf("changes = %v", texts)
	}
	got, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, retired := range append([]string{retiredPlanResyncSkill}, selectionRetiredKeys...) {
		if strings.Contains(string(got), retired) {
			t.Fatalf("config retained %q: %s", retired, got)
		}
	}
	if _, err := config.Load(filepath.Join(root, ".awf")); err != nil {
		t.Fatalf("strict-load generation-40 config: %v", err)
	}
	for _, sidecar := range []string{skillSidecar, docSidecar} {
		body, err := os.ReadFile(sidecar)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "local:") {
			t.Fatalf("sidecar retained local: %s", body)
		}
	}
	if info, err := os.Stat(skillSidecar); err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("skill sidecar mode = %v, %v", info, err)
	}
	applied, changes, err = Upgrade(context.Background(), root)
	if err != nil || len(applied) != 0 || len(changes) != 0 {
		t.Fatalf("repeat = %v, %v, %v", applied, changes, err)
	}
}

func TestRetirePlanResyncMigrationMalformedIsAtomic(t *testing.T) {
	for _, tc := range []struct {
		name    string
		config  string
		sidecar string
	}{
		{name: "config", config: "prefix: ex\nskills: reviewing-plan-resync\n"},
		{name: "sidecar preflight", config: "prefix: ex\nskills: [reviewing-plan-resync]\n", sidecar: "local: [\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, ".awf", "config.yaml")
			testsupport.WriteFile(t, path, tc.config)
			if tc.sidecar != "" {
				testsupport.WriteFile(t, filepath.Join(root, ".awf", "skills", "tdd.yaml"), tc.sidecar)
			}
			changes := &Changes{}
			if err := applyRetirePlanResync(root, changes); err == nil {
				t.Fatal("malformed input accepted")
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.config || len(changes.Items()) != 0 {
				t.Fatalf("mutation on failure: %q, %v", got, changes.Items())
			}
		})
	}
}

// invariant: config/migrations-and-locks:retired-plan-resync-selection-migration (TestConfigForCurrentSchemaAlwaysRetiresPlanResync)
func TestConfigForCurrentSchemaAlwaysRetiresPlanResync(t *testing.T) {
	src := []byte("prefix: ex\nintegrationBranch: main\nvars: {}\nskills: [reviewing-plan-resync, reviewing-plan]\nagents: [plan-reviewer]\ndocs: [workflow]\ntargets: [claude]\ndocsDir: docs\n")
	for from := 1; from <= Current(); from++ {
		got, err := ConfigForCurrentSchema(src, from)
		if err != nil {
			t.Fatalf("from %d: %v", from, err)
		}
		for _, retired := range append([]string{retiredPlanResyncSkill}, selectionRetiredKeys...) {
			if strings.Contains(string(got), retired) {
				t.Fatalf("from %d retained %q: %s", from, retired, got)
			}
		}
		root := t.TempDir()
		testsupport.WriteFile(t, filepath.Join(root, "config.yaml"), string(got))
		if _, err := config.Load(root); err != nil {
			t.Fatalf("from %d strict load: %v", from, err)
		}
	}
	aliased := []byte("prefix: ex\nintegrationBranch: main\nskills: [&retired reviewing-plan-resync, *retired, reviewing-plan]\nagents: [plan-reviewer]\nvars:\n  literal: *retired\n")
	got, err := ConfigForCurrentSchema(aliased, 37)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Skills []string          `yaml:"skills"`
		Vars   map[string]string `yaml:"vars"`
	}
	if err := yaml.Unmarshal(got, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Skills != nil || decoded.Vars["literal"] != retiredPlanResyncSkill {
		t.Fatalf("aliased forward port = %#v", decoded)
	}
}
