package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// The producer matrix keeps reader-facing source guidance bounded to opaque
// generated documents. It deliberately checks rendered content, not recipes or
// inputs: markers are informational and never alter machine dependencies.
// invariant: rendering/doc-outputs:opaque-doc-source-guidance (TestSourceMarkerFamilyMatrix)
func TestSourceMarkerFamilyMatrix(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nskills: []\nagents: []\ndocs: [glossary, pitfalls]\ndomains: [rendering]\ntargets: [claude]\n", map[string]string{
		"domains/rendering.yaml": "paths: ['internal/**']\n",
		"docs/glossary.yaml":     "data:\n  standardTerms:\n  terms:\n",
		"docs/pitfalls.yaml":     "data:\n  pitfalls:\n",
	})
	writeProjectTopic(t, root, "opaque", "Opaque", "applies: global\n")
	writeADR(t, root, "0001-fixture.md", testsupport.ADR("Implemented", testsupport.WithDomains("rendering"), testsupport.WithTitle("0001: Fixture"), testsupport.WithBody("## Decision\n\n1. Fixture.\n")))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = f.Content
	}
	positive := map[string]string{
		"docs/glossary.md":                ".awf/docs/glossary.yaml derived:awf-standard-vocabulary",
		"docs/pitfalls.md":                ".awf/docs/pitfalls.yaml",
		"docs/decisions/INDEX.md":         "derived:authored-adr-corpus",
		"docs/config-reference.md":        "derived:configspec derived:project-configuration",
		"docs/domains/rendering.md":       ".awf/topics/metadata/rendering/*.yaml .awf/topics/parts/rendering/*/current-state.md",
		"docs/topics/rendering/index.md":  ".awf/topics/metadata/rendering/*.yaml .awf/topics/parts/rendering/*/current-state.md",
		"docs/topics/rendering/opaque.md": ".awf/topics/metadata/rendering/opaque.yaml .awf/topics/parts/rendering/opaque/current-state.md",
		"CLAUDE.md":                       "AGENTS.md",
	}
	banner := "<!-- " + bannerText + " -->\n"
	for path, sources := range positive {
		content, ok := byPath[path]
		if !ok {
			t.Errorf("missing marked output %s", path)
			continue
		}
		marker := "<!-- awf:source " + sources + " -->\n"
		if !strings.Contains(content, banner+marker) || strings.Count(content, "<!-- awf:source ") != 1 {
			t.Errorf("%s marker placement/count mismatch:\n%s", path, content)
		}
		if strings.Contains(content, "<no value>") {
			t.Errorf("%s contains unresolved value", path)
		}
	}
	for _, path := range []string{"AGENTS.md", "docs/working-with-awf.md", "docs/decisions/README.md", "docs/plans/README.md", ".awf/hooks/pre-commit.sh"} {
		if content, ok := byPath[path]; ok && strings.Contains(content, "<!-- awf:source ") {
			t.Errorf("unapproved family %s gained a source marker", path)
		}
	}
}
