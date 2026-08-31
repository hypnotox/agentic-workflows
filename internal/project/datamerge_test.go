package project

import "testing"

// A change to an artifact's catalog default data must change its lock
// configHash, so awf check flags the artifact stale (ADR-0045).
// invariant: rendering/sync-and-drift:catalog-data-in-confighash (TestCatalogDataChangesConfigHash)
func TestCatalogDataChangesConfigHash(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := loadTestSession(testContext(t), root)
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
			if f.TemplateID == "skills/debugging/SKILL.md.tmpl" {
				return f.ConfigHash
			}
		}
		t.Fatal("debugging render not found")
		return ""
	}
	before := hashOf()
	selected := p.catalog()
	spec := selected.Skills["debugging"]
	spec.Data = map[string]any{"testSurfaces": []any{
		map[string]any{"name": "Changed", "kind": "unit", "location": "here"},
	}}
	selected.Skills["debugging"] = spec
	p = testStateWith(p, p.Root(), p.Roots(), p.Nested(), selected, p.Targets())
	after := hashOf()
	if before == after {
		t.Fatalf("ConfigHash unchanged after catalog default-data change: %s", before)
	}
}
