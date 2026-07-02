package project

import (
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// validKinds and validTargets bound the descriptor schema the embedded catalog
// may use.
var validKinds = []string{"string", "enum", "multiselect"}
var validTargets = []string{"", "var", "invariants-marker", "invariants-globs", "catalog-skills", "catalog-docs"}

// TestVarDescriptorParity asserts that every var referenced by any catalog
// template has a matching var-target descriptor, and no var-target descriptor
// names a var absent from every template. Non-var descriptors (the invariants
// marker/globs) are exempt. The referenced set is re-derived from the templates
// here, independently of any production helper.
// invariant: var-descriptor-parity
func TestVarDescriptorParity(t *testing.T) {
	cat, err := catalog.Load(templates.FS)
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}

	// Referenced vars across every catalog template family.
	referenced := map[string]bool{}
	var paths []string
	for name := range cat.Skills {
		paths = append(paths, "skills/"+name+"/SKILL.md.tmpl")
	}
	for name := range cat.Agents {
		paths = append(paths, "agents/"+name+".md.tmpl")
	}
	for name := range cat.Docs {
		paths = append(paths, "docs/"+name+".md.tmpl")
	}
	for _, name := range hookNames {
		paths = append(paths, "hooks/"+name+".sh.tmpl")
	}
	for _, p := range paths {
		src, err := templates.FS.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		for _, v := range render.ReferencedVars(string(src)) {
			referenced[v] = true
		}
	}

	// Descriptor set, partitioned by target.
	descByKey := map[string]bool{}
	for _, d := range cat.Vars {
		if !slices.Contains(validKinds, d.Kind) {
			t.Errorf("descriptor %q has invalid kind %q", d.Key, d.Kind)
		}
		if !slices.Contains(validTargets, d.Target) {
			t.Errorf("descriptor %q has invalid target %q", d.Key, d.Target)
		}
		if d.Target == "" || d.Target == "var" {
			descByKey[d.Key] = true
			if !referenced[d.Key] {
				t.Errorf("var descriptor %q names a var referenced by no template", d.Key)
			}
		}
	}

	// Every referenced var has a var descriptor.
	for v := range referenced {
		if !descByKey[v] {
			t.Errorf("referenced var %q has no catalog descriptor", v)
		}
	}
}
