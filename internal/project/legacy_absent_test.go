package project

import (
	"slices"
	"testing"
)

// TestLegacyActiveMDIndexNotPlanned is the behavioral half of the current-state
// legacy-authority denylist (its source-scan half is
// currentstate.TestLegacyAuthorityAbsent, kept there to avoid a project import
// cycle). It asserts the ADR-projected ACTIVE.md index is no longer a planned
// output: INDEX.md replaced it (ADR-0135). This behavioral check keeps the
// absence pinned independently of the source denylist.
func TestLegacyActiveMDIndexNotPlanned(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\ndomains: [rendering]\ntargets: [claude]\n",
		map[string]string{"domains/rendering.yaml": "paths: ['internal/**']\n"})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	planned, err := p.PlannedOutputs()
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(planned, "docs/decisions/ACTIVE.md") {
		t.Error("docs/decisions/ACTIVE.md is a planned output; the ADR-projected index is retired for INDEX.md (ADR-0135)")
	}
	// Positive control: INDEX.md must be planned, so the ACTIVE.md assertion is
	// not passing vacuously over a plan that generates neither index.
	if !slices.Contains(planned, "docs/decisions/INDEX.md") {
		t.Errorf("docs/decisions/INDEX.md is not planned; the positive control failed:\n%v", planned)
	}
}
