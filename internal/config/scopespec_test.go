package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// invariant: scope-config-dual-form
func TestScopeSpecDualForm(t *testing.T) {
	// A bare string and a {name, meaning} mapping decode in the same list,
	// through the strict config-load path (KnownFields(true)).
	const doc = "- adr\n- {name: rendering, meaning: the render engine}\n"
	var got []ScopeSpec
	dec := yaml.NewDecoder(strings.NewReader(doc))
	dec.KnownFields(true)
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("decode mixed list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0] != (ScopeSpec{Name: "adr"}) {
		t.Errorf("bare element = %+v, want {Name:adr Meaning:}", got[0])
	}
	if got[1] != (ScopeSpec{Name: "rendering", Meaning: "the render engine"}) {
		t.Errorf("mapping element = %+v", got[1])
	}

	// Error cases exercise every UnmarshalYAML branch.
	for _, tc := range []struct {
		name string
		yaml string
	}{
		{"non-scalar-non-mapping", "- [a, b]\n"},                  // sequence element
		{"unknown-key", "- {name: x, foo: y}\n"},                  // explicit key check
		{"undecodable-meaning", "- {name: x, meaning: [a, b]}\n"}, // n.Decode error
		{"missing-name", "- {meaning: y}\n"},                      // r.Name == ""
	} {
		t.Run(tc.name, func(t *testing.T) {
			var bad []ScopeSpec
			if err := yaml.Unmarshal([]byte(tc.yaml), &bad); err == nil {
				t.Errorf("%s: want error, got nil (%+v)", tc.name, bad)
			}
		})
	}
}
