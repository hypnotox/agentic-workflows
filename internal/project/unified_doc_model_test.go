package project

import (
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/resident"
)

// The whole doc surface derives from the single catalog doc collection: output
// projections use AgentsDoc/Path/TemplateKey metadata while Mandatory chooses
// only the sidecar location, with no independent hand-maintained list (ADR-0061).
// invariant: rendering/catalog-and-targets:unified-doc-model (TestUnifiedDocModelProjections)
// invariant: rendering/singletons-and-payloads:plain-singleton-via-renderkind (TestUnifiedDocModelProjections)
func TestUnifiedDocModelProjections(t *testing.T) {
	cat := catalog.CompleteView().Catalog()
	// (a) SingletonKinds == exactly root and structural output entries.
	var wantSK []string
	for k, e := range cat.Docs {
		if e.AgentsDoc || e.Path != "" {
			wantSK = append(wantSK, k)
		}
	}
	slices.Sort(wantSK)
	if sk := catalog.SingletonKindsFor(cat); !slices.Equal(sk, wantSK) {
		t.Errorf("SingletonKindsFor()=%v, want root/structural entries %v", sk, wantSK)
	}

	// (b) plainSingletons == exactly structural non-generated entries, and
	// no other kind (the generated config reference renders outside RenderAll).
	var got []string
	for _, s := range plainSingletons(cat) {
		got = append(got, s.kind)
	}
	var wantPS []string
	for k, e := range cat.Docs {
		if e.Path != "" && !e.AgentsDoc && !e.Generated {
			wantPS = append(wantPS, k)
		}
	}
	slices.Sort(wantPS)
	if !slices.Equal(got, wantPS) {
		t.Errorf("plainSingletons kinds=%v, want %v", got, wantPS)
	}

	// (c) every structural non-root entry's TemplateKey/Path lands in templateMap
	// at the derived docsDir path.
	state := testStateWith(testState(&config.Config{}), "", resident.NewRoots("", ""), false, cat, nil)
	tm := layout(newRenderInputs(state, &config.Config{}, nil)).templateMap()
	for _, e := range cat.Docs {
		if e.Path == "" || e.AgentsDoc {
			continue
		}
		if v := tm[e.TemplateKey]; v != "docs/"+e.Path {
			t.Errorf("templateMap[%q]=%v, want %q", e.TemplateKey, v, "docs/"+e.Path)
		}
	}
}

// TestSingletonShapeIgnoresMandatory uses deliberately heterogeneous metadata
// so a future Mandatory-based membership shortcut fails even if the standard
// catalog happens to keep those attributes correlated.
func TestSingletonShapeIgnoresMandatory(t *testing.T) {
	entries := map[string]catalog.DocEntry{
		"root":               {AgentsDoc: true, Mandatory: false},
		"structural":         {Path: "structural.md", Mandatory: false},
		"named-with-root-sc": {Mandatory: true},
		"named-with-doc-sc":  {Mandatory: false},
	}
	got := map[string]bool{}
	for name, entry := range entries {
		got[name] = entry.AgentsDoc || entry.Path != ""
	}
	want := map[string]bool{"root": true, "structural": true, "named-with-root-sc": false, "named-with-doc-sc": false}
	if gotKinds := catalog.SingletonKindsFor(&catalog.Catalog{Docs: entries}); !slices.Equal(gotKinds, []string{"root", "structural"}) {
		t.Fatalf("injected singleton kinds = %v", gotKinds)
	}
	for name, expected := range want {
		if got[name] != expected {
			t.Errorf("singleton(%s) = %t, want %t", name, got[name], expected)
		}
	}
}

// invariant: rendering/doc-outputs:layout-docs-profile-projection (TestUnifiedDocModelProjections)
