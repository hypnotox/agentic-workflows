package catalog

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/pathglob"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/templates"
	"gopkg.in/yaml.v3"
)

// invariant: rendering/catalog-and-targets:catalog-go-single-source
func TestCatalogIsCompileTimeSingleSource(t *testing.T) {
	if _, err := fs.Stat(templates.FS, "catalog.yaml"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("catalog.yaml must not be embedded; got stat err = %v", err)
	}
	if len(Standard.Skills) == 0 || len(Standard.Agents) == 0 || len(Standard.Docs) == 0 ||
		len(SingletonKinds()) == 0 || len(Standard.Vars) == 0 || len(Standard.DomainDoc.Sections) == 0 {
		t.Fatalf("catalog.Standard is not populated across all kinds")
	}
}

// Catalog default data must be generic: no default names an awf-repo path or
// command (ADR-0045). Walks every spec's Data recursively down to the strings.
// invariant: rendering/catalog-and-targets:catalog-defaults-generic-denylist
func TestCatalogDefaultDataIsGeneric(t *testing.T) {
	cat := Standard
	states, ok := cat.Skills["adr-lifecycle"].Data["adrStates"].([]any)
	if !ok || len(states) != 5 {
		t.Fatalf("representative V2 adrStates = %#v", cat.Skills["adr-lifecycle"].Data["adrStates"])
	}
	wantStates := []string{"Proposed", "Accepted", "Implementing", "Implemented", "Abandoned"}
	for i, state := range states {
		fields, ok := state.(map[string]any)
		if !ok || fields["name"] != wantStates[i] || fields["meaning"] == "" || fields["mutability"] == "" {
			t.Fatalf("V2 adrStates[%d] = %#v", i, state)
		}
	}
	if empty := (map[string]any{})["adrStates"]; empty != nil {
		t.Fatalf("empty catalog override unexpectedly supplies V2 data: %#v", empty)
	}
	denylist := []string{"./x", "hypnotox/agentic-workflows"}
	var walk func(t *testing.T, path string, v any)
	walk = func(t *testing.T, path string, v any) {
		switch val := v.(type) {
		case string:
			for _, banned := range denylist {
				if strings.Contains(val, banned) {
					t.Errorf("%s: default data contains %q: %q", path, banned, val)
				}
			}
		case []any:
			for i, item := range val {
				walk(t, fmt.Sprintf("%s[%d]", path, i), item)
			}
		case map[string]any:
			for k, item := range val {
				walk(t, path+"."+k, item)
			}
		}
	}
	for name, spec := range cat.Skills {
		walk(t, "skills."+name, spec.Data)
	}
	for name, spec := range cat.Agents {
		walk(t, "agents."+name, spec.Data)
	}
	for name, e := range cat.Docs {
		walk(t, "docs."+name, e.Data)
	}
}

func TestAgentsDocSectionsNonEmpty(t *testing.T) {
	cat := Standard
	sections := cat.Docs["agents-doc"].Sections
	if len(sections) == 0 {
		t.Error("expected agents-doc Sections to be non-empty")
	}
	expected := []string{"you-and-this-project", "identity", "invariants", "workflow", "commands", "document-map"}
	for _, s := range expected {
		found := false
		for _, sec := range sections {
			if sec == s {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected section %q in agents-doc Sections, got: %v", s, sections)
		}
	}
}

// Every reviewing skill is a thin dispatcher around one reviewer agent; the
// catalog must pair them so the ADR-0050 validation can enforce it - the
// prefix anchor keeps a future reviewing skill from reopening the blind spot.
// invariant: rendering/catalog-and-targets:reviewing-skill-specs-paired
func TestReviewingSkillSpecsArePaired(t *testing.T) {
	cat := Standard
	for name, spec := range cat.Skills {
		if !strings.HasPrefix(name, "reviewing-") {
			if spec.RequiresAgent != "" {
				t.Errorf("skill %q: requiresAgent %q on a non-reviewing skill (ADR-0050 scopes the field to dispatchers)", name, spec.RequiresAgent)
			}
			continue
		}
		if spec.RequiresAgent == "" {
			t.Errorf("reviewing skill %q carries no requiresAgent", name)
			continue
		}
		if _, ok := cat.Agents[spec.RequiresAgent]; !ok {
			t.Errorf("skill %q requires agent %q, which is not in the catalog agents map", name, spec.RequiresAgent)
		}
	}
}

// TestRequiresSkillsDeclarationsValid rejects a RequiresSkills entry naming a
// non-catalog skill or the artifact itself, and any RequiresSkills on the
// domain-doc spec - today the only TargetSpec use outside the agents map; the
// field is meaningless there and a silent no-op would invite drift (ADR-0080
// Decision 1). The self-naming rejection is an exactness corollary of Decision
// 7: a self-entry could never fail as stale (the frontmatter name always marks
// self as found), so it is refused upfront.
func TestRequiresSkillsDeclarationsValid(t *testing.T) {
	cat := Standard
	for name, spec := range cat.Skills {
		for _, r := range spec.RequiresSkills {
			if _, ok := cat.Skills[r]; !ok {
				t.Errorf("skill %q: requiresSkills entry %q is not a catalog skill", name, r)
			}
			if r == name {
				t.Errorf("skill %q: requiresSkills must not name itself", name)
			}
		}
	}
	for name, spec := range cat.Agents {
		for _, r := range spec.RequiresSkills {
			if _, ok := cat.Skills[r]; !ok {
				t.Errorf("agent %q: requiresSkills entry %q is not a catalog skill", name, r)
			}
		}
	}
	if len(cat.DomainDoc.RequiresSkills) != 0 {
		t.Error("domainDoc: requiresSkills is only valid on skills and agents (ADR-0080 Decision 1)")
	}
}

// invariant: rendering/catalog-and-targets:no-single-marker-init-descriptor
//
// The catalog exposes no single marker/globs var descriptor; qualified markers
// reach config only through currentState.sources.
func TestNoSingleMarkerInitDescriptor(t *testing.T) {
	for _, d := range Standard.Vars {
		if d.Key == "invariantsMarker" || d.Key == "invariantsGlobs" {
			t.Errorf("catalog still declares removed descriptor key %q", d.Key)
		}
		if d.Target == "invariants-marker" || d.Target == "invariants-globs" {
			t.Errorf("catalog still declares removed descriptor target %q", d.Target)
		}
	}

	var live struct {
		CurrentState struct {
			Sources []struct {
				Globs  []string `yaml:"globs"`
				Marker string   `yaml:"marker"`
			} `yaml:"sources"`
		} `yaml:"currentState"`
	}
	configPath := filepath.Join(testsupport.RepoRoot(t), ".awf", "config.yaml")
	body, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(body, &live); err != nil {
		t.Fatal(err)
	}
	const testPath = "internal/catalog/catalog_test.go"
	const qualified = "invariant: rendering/catalog-and-targets:no-single-marker-init-descriptor"
	for _, source := range live.CurrentState.Sources {
		for _, glob := range source.Globs {
			if pathglob.Match(glob, testPath) && source.Marker+" "+qualified == "// "+qualified {
				return
			}
		}
	}
	t.Fatalf("currentState.sources has no configuration route from %s to qualified marker %q", testPath, "// "+qualified)
}
