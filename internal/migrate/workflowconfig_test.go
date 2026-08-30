package migrate

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"gopkg.in/yaml.v3"
)

func TestRetireWorkflowConfigBytesRemovesProfileAndUnsetRetiredVars(t *testing.T) {
	source := []byte("# keep\nprefix: example\nprofile: full\nintegrationBranch: main\nvars:\n  gateCmd: make gate\n  gateCmdFull: \"\"\n  commitGateCmd: null\n  activeMdRegenCmd: '  '\n  invariantTestPath:\n")
	got, removed, err := retireWorkflowConfigBytes(source)
	if err != nil {
		t.Fatal(err)
	}
	wantRemoved := []string{"vars.activeMdRegenCmd", "vars.commitGateCmd", "vars.gateCmdFull", "vars.invariantTestPath", "profile"}
	if !slices.Equal(removed, wantRemoved) {
		t.Fatalf("removed=%v, want %v", removed, wantRemoved)
	}
	for _, forbidden := range []string{"profile:", "gateCmdFull:", "commitGateCmd:", "activeMdRegenCmd:", "invariantTestPath:"} {
		if bytes.Contains(got, []byte(forbidden)) {
			t.Fatalf("retired key %q remains:\n%s", forbidden, got)
		}
	}
	if !bytes.Contains(got, []byte("# keep")) || !bytes.Contains(got, []byte("gateCmd: make gate")) {
		t.Fatalf("retained config lost:\n%s", got)
	}
	again, removedAgain, err := retireWorkflowConfigBytes(got)
	if err != nil || len(removedAgain) != 0 || !bytes.Equal(again, got) {
		t.Fatalf("idempotence removed=%v err=%v\ngot:\n%s", removedAgain, err, again)
	}
}

func TestRetireWorkflowConfigBytesRefusesMeaningfulRetiredOverrides(t *testing.T) {
	for _, key := range retiredWorkflowVars {
		t.Run(key, func(t *testing.T) {
			source := []byte("prefix: example\nprofile: core\nintegrationBranch: main\nvars:\n  " + key + ": custom behavior\n")
			got, removed, err := retireWorkflowConfigBytes(source)
			if err == nil || !strings.Contains(err.Error(), "vars."+key+" has a meaningful retired override") {
				t.Fatalf("got=%q removed=%v err=%v, want actionable refusal", got, removed, err)
			}
			if got != nil || removed != nil {
				t.Fatalf("refusal planned mutation: got=%q removed=%v", got, removed)
			}
		})
	}
}

func TestRetireWorkflowConfigMigrationPreservesModeAndDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	path := config.ConfigPath(root)
	source := "prefix: example\nprofile: core\nintegrationBranch: main\nvars:\n  gateCmdFull: \"\"\n  gateCmd: make gate\n"
	writeLock(t, root, 48)
	testsupport.WriteFile(t, path, source)
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}

	applied, changes, mutations, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(applied, []string{retireWorkflowConfigName, retirePitfallRelationsName}) || len(changes) != 2 || len(mutations) != 1 {
		t.Fatalf("applied=%v changes=%v mutations=%#v", applied, changes, mutations)
	}
	mutation := mutations[0]
	if mutation.Path != ".awf/config.yaml" || mutation.Mode != 0o640 || strings.Contains(string(mutation.Content), "profile:") || strings.Contains(string(mutation.Content), "gateCmdFull:") {
		t.Fatalf("mutation=%#v", mutation)
	}
	got, err := os.ReadFile(filepath.Clean(path))
	if err != nil || string(got) != source {
		t.Fatalf("Build wrote config: %q, %v", got, err)
	}
}

func TestRetiredWorkflowArtifactsUseFrozenSchema48IdentitySet(t *testing.T) {
	var got []string
	for kind, artifacts := range retiredWorkflowArtifacts {
		for name := range artifacts {
			got = append(got, kind+"/"+name)
		}
	}
	slices.Sort(got)
	want := []string{
		"agents/adr-reviewer", "agents/code-reviewer", "agents/grounding-checker",
		"skills/adr-lifecycle", "skills/bugfix", "skills/executing-direct", "skills/exploring", "skills/grounding", "skills/orienting", "skills/proposing-adr", "skills/refactor-coupling-audit", "skills/retrospective", "skills/reviewing-adr", "skills/reviewing-impl", "skills/roadmap-graduation", "skills/tdd", "skills/writing-docs",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("retired artifacts = %v, want %v", got, want)
	}
}

func TestRetireWorkflowArtifactsRemovesOnlyDefaultEquivalentState(t *testing.T) {
	root := schema48WorkflowRoot(t)
	files := map[string]string{
		".awf/skills/grounding.yaml":                        "data: {}\ndataDefaults: {unused: true}\nsections:\n  notes: {drop: false}\npaths: []\n",
		".awf/agents/adr-reviewer.yaml":                     "{}\n",
		".awf/skills/parts/grounding/notes.md":              "{{=awf:sectionDefault}}\n",
		".awf/skills/parts/debugging/debugging-surfaces.md": "{{=awf:sectionDefault}}\n",
		".awf/skills/parts/debugging/oracle-and-handoff.md": "current section override\n",
	}
	for path, body := range files {
		testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(path)), body)
	}
	_, _, mutations, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	var removed []string
	for _, mutation := range mutations {
		if mutation.Remove {
			removed = append(removed, mutation.Path)
		}
	}
	want := []string{".awf/agents/adr-reviewer.yaml", ".awf/skills/grounding.yaml", ".awf/skills/parts/debugging/debugging-surfaces.md", ".awf/skills/parts/grounding/notes.md"}
	if !slices.Equal(removed, want) {
		t.Fatalf("removed = %v, want %v", removed, want)
	}
	for path, body := range files {
		got, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if readErr != nil || string(got) != body {
			t.Fatalf("Build mutated %s: %q, %v", path, got, readErr)
		}
	}
}

func TestRetireWorkflowArtifactsRefusesMeaningfulOrUnknownSidecars(t *testing.T) {
	cases := map[string]string{
		"data":             "data: {value: custom}\n",
		"default-disabled": "dataDefaults: {value: false}\n",
		"section-dropped":  "sections: {notes: {drop: true}}\n",
		"unknown-section":  "sections: {other: {drop: false}}\n",
		"paths":            "paths: [src/**]\n",
		"unknown-field":    "custom: value\n",
		"malformed":        "sections: [\n",
		"multiple-docs":    "{}\n---\n{}\n",
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			root := schema48WorkflowRoot(t)
			path := filepath.Join(root, ".awf", "skills", "grounding.yaml")
			testsupport.WriteFile(t, path, source)
			_, _, mutations, err := Build(context.Background(), root)
			if err == nil || !strings.Contains(err.Error(), ".awf/skills/grounding.yaml has a meaningful retired override") {
				t.Fatalf("mutations=%#v err=%v, want retired-override refusal", mutations, err)
			}
			got, readErr := os.ReadFile(path)
			if readErr != nil || string(got) != source {
				t.Fatalf("refusal mutated source: %q, %v", got, readErr)
			}
		})
	}
}

func TestRetireWorkflowArtifactsRefusesMeaningfulOrUnknownParts(t *testing.T) {
	cases := map[string]string{
		"notes.md":                  "",
		"boundaries.md":             "  {{=awf:sectionDefault}}\n",
		"finding-classification.md": "{{=awf:sectionDefault}}",
		"procedure.md":              "custom body\n",
		"unknown.md":                "{{=awf:sectionDefault}}\n",
		"notes/extra.md":            "{{=awf:sectionDefault}}\n",
		"notes.txt":                 "{{=awf:sectionDefault}}\n",
	}
	for rel, source := range cases {
		t.Run(strings.ReplaceAll(rel, "/", "-"), func(t *testing.T) {
			root := schema48WorkflowRoot(t)
			path := filepath.Join(root, ".awf", "skills", "parts", "grounding", filepath.FromSlash(rel))
			testsupport.WriteFile(t, path, source)
			_, _, mutations, err := Build(context.Background(), root)
			if err == nil || !strings.Contains(err.Error(), "reconcile it before upgrade") {
				t.Fatalf("mutations=%#v err=%v, want part refusal", mutations, err)
			}
			got, readErr := os.ReadFile(path)
			if readErr != nil || string(got) != source {
				t.Fatalf("refusal mutated source: %q, %v", got, readErr)
			}
		})
	}
}

func TestRetireWorkflowArtifactsMigratesRetainedSidecarsWithoutLosingCurrentOverrides(t *testing.T) {
	root := schema48WorkflowRoot(t)
	tests := []struct {
		kind, name, retired, current string
		removed                      bool
	}{
		{"skills", "brainstorming", "preamble", "procedure", false},
		{"skills", "using-awf", "procedure", "upgrades", false},
		{"skills", "debugging", "debugging-surfaces", "oracle-and-handoff", false},
		{"agents", "explorer", "identity", "scope", false},
		{"agents", "implementer", "identity", "", true},
	}
	sources := map[string]string{}
	for _, test := range tests {
		sections := "  " + test.retired + ": {drop: false}\n"
		if test.current != "" {
			sections += "  " + test.current + ": {drop: true}\n"
		}
		source := "data: {}\ndataDefaults: {legacy: true}\nsections:\n" + sections + "paths: []\n"
		rel := ".awf/" + test.kind + "/" + test.name + ".yaml"
		sources[rel] = source
		testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(rel)), source)
	}
	debugPath := filepath.Join(root, ".awf", "skills", "debugging.yaml")
	if err := os.Chmod(debugPath, 0o640); err != nil {
		t.Fatal(err)
	}

	_, _, mutations, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	planned := map[string]FileMutation{}
	for _, mutation := range mutations {
		planned[mutation.Path] = mutation
	}
	for _, test := range tests {
		rel := ".awf/" + test.kind + "/" + test.name + ".yaml"
		mutation, ok := planned[rel]
		if !ok || mutation.Remove != test.removed {
			t.Fatalf("%s mutation = %#v, found %v", rel, mutation, ok)
		}
		if test.name == "debugging" && mutation.Mode != 0o640 {
			t.Fatalf("debugging mode = %v", mutation.Mode)
		}
		if !mutation.Remove {
			var got config.Sidecar
			if err := yaml.Unmarshal(mutation.Content, &got); err != nil {
				t.Fatal(err)
			}
			if len(got.Data) != 0 || len(got.DataDefaults) != 0 || len(got.Paths) != 0 || len(got.Sections) != 1 || !got.Sections[test.current].Drop {
				t.Fatalf("%s retained sidecar = %#v", rel, got)
			}
		}
		original, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if readErr != nil || string(original) != sources[rel] {
			t.Fatalf("Build mutated %s: %q, %v", rel, original, readErr)
		}
	}

	for _, mutation := range mutations {
		path := filepath.Join(root, filepath.FromSlash(mutation.Path))
		if mutation.Remove {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(path, mutation.Content, mutation.Mode); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := config.Load(filepath.Join(root, ".awf"))
	if err != nil {
		t.Fatalf("load migrated config: %v", err)
	}
	for _, test := range tests {
		got, err := loaded.Sidecar(test.kind, test.name)
		if err != nil {
			t.Fatalf("load %s/%s: %v", test.kind, test.name, err)
		}
		if test.removed {
			if len(got.Sections) != 0 {
				t.Fatalf("removed %s/%s sidecar = %#v", test.kind, test.name, got)
			}
		} else if len(got.Sections) != 1 || !got.Sections[test.current].Drop {
			t.Fatalf("migrated %s/%s sidecar = %#v", test.kind, test.name, got)
		}
	}
}

func TestRetireWorkflowArtifactsRefusesMeaningfulRetainedSidecars(t *testing.T) {
	cases := map[string]string{
		".awf/skills/brainstorming.yaml": "data: {loadBearingExamples: [{item: custom}]}\n",
		".awf/skills/using-awf.yaml":     "sections: {procedure: {drop: true}}\n",
		".awf/skills/debugging.yaml":     "sections: {debugging-surfaces: {drop: true}}\n",
		".awf/agents/explorer.yaml":      "sections: {identity: {drop: true}}\n",
		".awf/agents/implementer.yaml":   "dataDefaults: {prohibitedShortcuts: false}\n",
	}
	for rel, source := range cases {
		t.Run(strings.ReplaceAll(rel, "/", "-"), func(t *testing.T) {
			root := schema48WorkflowRoot(t)
			testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(rel)), source)
			_, _, mutations, err := Build(context.Background(), root)
			if err == nil || !strings.Contains(err.Error(), "meaningful retired override") {
				t.Fatalf("mutations=%#v err=%v, want retained-sidecar refusal", mutations, err)
			}
		})
	}
}

func TestRetireWorkflowArtifactsRefusesDogfoodOverrides(t *testing.T) {
	cases := map[string]string{
		".awf/agents/adr-reviewer.yaml":                         "data: {focusItems: [custom]}\n",
		".awf/agents/code-reviewer.yaml":                        "data: {focusItems: [custom]}\n",
		".awf/skills/proposing-adr.yaml":                        "dataDefaults: {adrTriggers: false}\n",
		".awf/skills/refactor-coupling-audit.yaml":              "sections: {category-4-codegen: {drop: true}}\n",
		".awf/skills/tdd.yaml":                                  "data: {testSurfaces: [custom]}\n",
		".awf/skills/parts/bugfix/pitfalls-check.md":            "custom\n",
		".awf/skills/parts/debugging/debugging-surfaces.md":     "custom\n",
		".awf/skills/parts/retrospective/procedure.md":          "custom\n",
		".awf/skills/parts/reviewing-impl/run-audit.md":         "custom\n",
		".awf/skills/parts/roadmap-graduation/failure-modes.md": "custom\n",
	}
	for rel, source := range cases {
		t.Run(strings.ReplaceAll(rel, "/", "-"), func(t *testing.T) {
			root := schema48WorkflowRoot(t)
			testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(rel)), source)
			_, _, mutations, err := Build(context.Background(), root)
			if err == nil || !strings.Contains(err.Error(), "reconcile it before upgrade") {
				t.Fatalf("mutations=%#v err=%v, want dogfood override refusal", mutations, err)
			}
		})
	}
}

func schema48WorkflowRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeLock(t, root, 48)
	testsupport.WriteFile(t, config.ConfigPath(root), "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {gateCmd: make gate}\n")
	return root
}

func TestConfigBytesForGenerationChainsWorkflowConfigRetirement(t *testing.T) {
	source := []byte("prefix: example\nprofile: full\nintegrationBranch: main\ntags: {old: metadata}\nvars: {gateCmd: make gate, gateCmdFull: \"\"}\n")
	got, err := ConfigBytesForGeneration(LiveSchemaFloor, source)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"profile:", "tags:", "gateCmdFull:"} {
		if bytes.Contains(got, []byte(forbidden)) {
			t.Fatalf("retired key %q remains:\n%s", forbidden, got)
		}
	}
	if !bytes.Contains(got, []byte("gateCmd: make gate")) {
		t.Fatalf("retained var lost:\n%s", got)
	}
}
