package catalog

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/templates"
)

// invariant: catalog-go-single-source
func TestCatalogIsCompileTimeSingleSource(t *testing.T) {
	if _, err := fs.Stat(templates.FS, "catalog.yaml"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("catalog.yaml must not be embedded; got stat err = %v", err)
	}
	if len(Standard.Skills) == 0 || len(Standard.Agents) == 0 || len(Standard.Docs) == 0 ||
		len(Standard.Singletons) == 0 || len(Standard.Vars) == 0 || len(Standard.DomainDoc.Sections) == 0 {
		t.Fatalf("catalog.Standard is not populated across all kinds")
	}
}

// Catalog default data must be generic: no default names an awf-repo path or
// command (ADR-0045). Walks every spec's Data recursively down to the strings.
// invariant: catalog-defaults-generic-denylist
func TestCatalogDefaultDataIsGeneric(t *testing.T) {
	cat := Standard
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
	for name, spec := range cat.Singletons {
		walk(t, "singletons."+name, spec.Data)
	}
}

func TestAgentsDocSectionsNonEmpty(t *testing.T) {
	cat := Standard
	sections := cat.Singletons["agents-doc"].Sections
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
// catalog must pair them so the ADR-0050 validation can enforce it — the
// prefix anchor keeps a future reviewing skill from reopening the blind spot.
// invariant: reviewing-skill-specs-paired
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
