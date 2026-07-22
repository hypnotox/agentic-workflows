package project

import (
	"reflect"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

func TestWithDefaultData(t *testing.T) {
	cases := []struct {
		name     string
		sidecar  map[string]any
		defaults map[string]any
		want     map[string]any
	}{
		{
			name:     "nil defaults leaves sidecar untouched",
			sidecar:  map[string]any{"a": "sidecar"},
			defaults: nil,
			want:     map[string]any{"a": "sidecar"},
		},
		{
			name:     "key only in defaults falls through",
			sidecar:  map[string]any{},
			defaults: map[string]any{"a": "default"},
			want:     map[string]any{"a": "default"},
		},
		{
			name:     "key in both: sidecar wins",
			sidecar:  map[string]any{"a": "sidecar"},
			defaults: map[string]any{"a": "default", "b": "default"},
			want:     map[string]any{"a": "sidecar", "b": "default"},
		},
		{
			name:     "present nil sidecar key suppresses the default",
			sidecar:  map[string]any{"a": nil},
			defaults: map[string]any{"a": "default"},
			want:     map[string]any{"a": nil},
		},
		{
			name:     "present empty-list sidecar key suppresses the default",
			sidecar:  map[string]any{"a": []any{}},
			defaults: map[string]any{"a": []any{"default"}},
			want:     map[string]any{"a": []any{}},
		},
		{
			name:     "nil sidecar data with defaults yields the defaults",
			sidecar:  nil,
			defaults: map[string]any{"a": "default"},
			want:     map[string]any{"a": "default"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := withDefaultData(config.Sidecar{Data: tc.sidecar}, tc.defaults)
			if tc.defaults == nil {
				if !reflect.DeepEqual(got.Data, tc.sidecar) {
					t.Fatalf("nil defaults: got %v, want sidecar %v", got.Data, tc.sidecar)
				}
				return
			}
			// invariant: rendering/project-output-plan:sidecar-key-overrides-default
			if !reflect.DeepEqual(got.Data, tc.want) {
				t.Fatalf("got %v, want %v", got.Data, tc.want)
			}
		})
	}
}

// A change to an artifact's catalog default data must change its lock
// configHash, so awf check flags the artifact stale (ADR-0045).
// invariant: rendering/sync-and-drift:catalog-data-in-confighash
func TestCatalogDataChangesConfigHash(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	hashOf := func() string {
		t.Helper()
		files, err := p.RenderAll()
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if f.TemplateID == "skills/tdd/SKILL.md.tmpl" {
				return f.ConfigHash
			}
		}
		t.Fatal("tdd render not found")
		return ""
	}
	before := hashOf()
	spec := p.Cat.Skills["tdd"]
	spec.Data = map[string]any{"testSurfaces": []any{
		map[string]any{"name": "Changed", "kind": "unit", "location": "here"},
	}}
	p.Cat.Skills["tdd"] = spec
	after := hashOf()
	if before == after {
		t.Fatalf("ConfigHash unchanged after catalog default-data change: %s", before)
	}
}
