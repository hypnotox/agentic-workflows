package project

import (
	"reflect"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
)

// invariant: rendering/project-output-plan:catalog-list-data-layering (TestWithDefaultData)
// invariant: config/configuration:sidecar-data-defaults-control (TestWithDefaultData)
func TestWithDefaultData(t *testing.T) {
	cases := []struct {
		name         string
		sidecar      map[string]any
		dataDefaults map[string]bool
		defaults     map[string]any
		want         map[string]any
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
			name:     "present nil non-list key replaces the default",
			sidecar:  map[string]any{"a": nil},
			defaults: map[string]any{"a": "default"},
			want:     map[string]any{"a": nil},
		},
		{
			name:     "absent authored list keeps catalog list",
			defaults: map[string]any{"a": []any{"default"}},
			want:     map[string]any{"a": []any{"default"}},
		},
		{
			name:     "empty authored list keeps catalog list",
			sidecar:  map[string]any{"a": []any{}},
			defaults: map[string]any{"a": []any{"default"}},
			want:     map[string]any{"a": []any{"default"}},
		},
		{
			name:         "authored lists preserve both orders and duplicates",
			sidecar:      map[string]any{"a": []any{"project-one", map[string]any{"name": "duplicate"}, "project-two"}},
			dataDefaults: map[string]bool{"a": true},
			defaults:     map[string]any{"a": []any{"default-one", map[string]any{"name": "duplicate"}, "default-two"}},
			want:         map[string]any{"a": []any{"default-one", map[string]any{"name": "duplicate"}, "default-two", "project-one", map[string]any{"name": "duplicate"}, "project-two"}},
		},
		{
			name:         "explicit false suppresses catalog list",
			sidecar:      map[string]any{"a": []any{"project"}},
			dataDefaults: map[string]bool{"a": false},
			defaults:     map[string]any{"a": []any{"default"}},
			want:         map[string]any{"a": []any{"project"}},
		},
		{
			name:         "explicit false without authored list yields empty",
			dataDefaults: map[string]bool{"a": false},
			defaults:     map[string]any{"a": []any{"default"}},
			want:         map[string]any{"a": []any{}},
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
			sc := config.Sidecar{Data: tc.sidecar, DataDefaults: tc.dataDefaults}
			got := withDefaultData(sc, tc.defaults)
			if tc.defaults == nil {
				if !reflect.DeepEqual(got.Data, tc.sidecar) {
					t.Fatalf("nil defaults: got %v, want sidecar %v", got.Data, tc.sidecar)
				}
				return
			}
			// invariant: rendering/project-output-plan:sidecar-key-overrides-default (TestWithDefaultData)
			if !reflect.DeepEqual(got.Data, tc.want) {
				t.Fatalf("got %v, want %v", got.Data, tc.want)
			}
		})
	}

	defaults := []any{map[string]any{"name": "default"}}
	authored := []any{map[string]any{"name": "project"}}
	got := withDefaultData(config.Sidecar{Data: map[string]any{"a": authored}}, map[string]any{"a": defaults})
	merged := got.Data["a"].([]any)
	merged[0].(map[string]any)["name"] = "changed default"
	merged[1].(map[string]any)["name"] = "changed project"
	if defaults[0].(map[string]any)["name"] != "default" || authored[0].(map[string]any)["name"] != "project" {
		t.Fatal("effective list aliases catalog or authored input")
	}
	defaults[0].(map[string]any)["name"] = "changed source default"
	authored[0].(map[string]any)["name"] = "changed source project"
	if merged[0].(map[string]any)["name"] != "changed default" || merged[1].(map[string]any)["name"] != "changed project" {
		t.Fatal("source list aliases effective catalog or authored records")
	}

	specialized := withDefaultData(
		config.Sidecar{Data: map[string]any{"standardTerms": []any{"project"}}},
		map[string]any{"standardTerms": []any{"catalog"}},
		specializedListDataKeys("docs", "glossary")...,
	)
	if !reflect.DeepEqual(specialized.Data["standardTerms"], []any{"project"}) {
		t.Fatalf("specialized glossary list entered generic composition: %v", specialized.Data)
	}
}

// A change to an artifact's catalog default data must change its lock
// configHash, so awf check flags the artifact stale (ADR-0045).
// invariant: rendering/sync-and-drift:catalog-data-in-confighash (TestCatalogDataChangesConfigHash)
func TestCatalogDataChangesConfigHash(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	hashOf := func() string {
		t.Helper()
		files, err := renderAll(p)
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
	selected := p.catalog()
	spec := selected.Skills["tdd"]
	spec.Data = map[string]any{"testSurfaces": []any{
		map[string]any{"name": "Changed", "kind": "unit", "location": "here"},
	}}
	selected.Skills["tdd"] = spec
	state := *p
	state.selectedCat = catalog.NewView(selected)
	p = &state
	after := hashOf()
	if before == after {
		t.Fatalf("ConfigHash unchanged after catalog default-data change: %s", before)
	}
}
