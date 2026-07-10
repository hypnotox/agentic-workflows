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
var validTargets = []string{"", "var", "catalog-skills", "catalog-docs", "audit-scopes"}

// TestVarDescriptorParity asserts that every var referenced by any catalog
// template has a matching var-target descriptor, and no var-target descriptor
// names a var absent from every template. Non-var descriptors (catalog trim,
// audit scopes) are exempt. The referenced set is re-derived from the templates
// here, independently of any production helper.
// invariant: var-descriptor-parity
func TestVarDescriptorParity(t *testing.T) {
	cat := catalog.Standard

	// Referenced vars across every catalog template family.
	referenced := map[string]bool{}
	var paths []string
	for name := range cat.Skills {
		paths = append(paths, "skills/"+name+"/SKILL.md.tmpl")
	}
	for name := range cat.Agents {
		paths = append(paths, "agents/"+name+".md.tmpl")
	}
	for _, e := range cat.Docs {
		paths = append(paths, e.TID) // merged-in singletons render from non-docs/ templates
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

// functionalVarKeys pins the catalog's value-carrying descriptor set to the
// eight functional keys ADR-0084 enumerates. Extending this list is a
// successor-ADR act: a descriptor exists only for a value the rendered
// artifacts or the tooling execute or enforce, never to tune prose wording.
var functionalVarKeys = []string{
	"gateCmd", "gateCmdFull", "checkCmd", "commitGateCmd",
	"testCmd", "commitScopes", "activeMdRegenCmd", "invariantTestPath",
}

// TestVarDescriptorSetPinned asserts the catalog's value-carrying descriptors
// (every kind but the catalog-trim multiselects) are exactly the pinned
// functional set, and the multiselects are exactly the two catalog trims — so
// a prose knob cannot re-enter under any kind without a successor ADR.
// Extending this pin is also where ADR-0087's seed-on-introduction contract
// bites: the release adding a catalog var must ship a one-time schema-migration
// seed (`<key>: ""` where absent), or absent-key acknowledgement silently
// swallows the new var's advisory for every existing adopter.
// invariant: var-descriptor-set-pinned
func TestVarDescriptorSetPinned(t *testing.T) {
	var got, multiselects []string
	for _, d := range catalog.Standard.Vars {
		if d.Kind == "multiselect" {
			multiselects = append(multiselects, d.Key)
			continue
		}
		got = append(got, d.Key)
	}
	want := slices.Clone(functionalVarKeys)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("value-carrying descriptor keys = %v, want the ADR-0084 pinned set %v", got, want)
	}
	slices.Sort(multiselects)
	if !slices.Equal(multiselects, []string{"docs", "skills"}) {
		t.Errorf("multiselect descriptor keys = %v, want exactly the catalog trims [docs skills]", multiselects)
	}
}
