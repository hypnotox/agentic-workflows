package project

import (
	"slices"
	"testing"
)

// TestDecisionIndexesNotPlanned pins that historical decision files are not
// publisher-managed projections.
func TestDecisionIndexesNotPlanned(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\ndomains: [rendering]\n",
		map[string]string{"domains/rendering.yaml": "paths: ['internal/**']\n"})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	planned, err := plannedOutputsProject(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"docs/decisions/ACTIVE.md", "docs/decisions/INDEX.md", "docs/decisions/README.md", "docs/decisions/template.md"} {
		if slices.Contains(planned, path) {
			t.Errorf("historical decision path %s remains planned: %v", path, planned)
		}
	}
}
