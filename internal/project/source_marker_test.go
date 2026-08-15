package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// The producer matrix keeps reader-facing source guidance bounded to opaque
// generated documents. It deliberately checks rendered content, not recipes or
// inputs: markers are informational and never alter machine dependencies.
// invariant: rendering/doc-outputs:opaque-doc-source-guidance (TestSourceMarkerFamilyMatrix)
func TestSourceMarkerFamilyMatrix(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars:\n  gateCmd: ./x gate\ndomains: [rendering]\n", map[string]string{
		"domains/rendering.yaml":       "paths: ['internal/**']\n",
		"docs/glossary.yaml":           "data:\n  standardTerms:\n  terms:\n",
		"docs/pitfalls/fixture.md":     pitfallSource("Fixture pitfall", "domains: [rendering]\n", "first body\n"),
		"docs/pitfalls/second-kind.md": pitfallSource("Second heterogeneous pitfall", "tags: [proof]\nrelated: [1]\n", "second body with different metadata\n"),
	})
	testsupport.WriteFile(t, filepath.Join(root, "docs", "plans", "2026-08-07-fixture.md"), "---\nformat: plan-v2\ndate: 2026-08-07\nadrs: []\nstatus: Proposed\n---\n# Plan: Fixture\n")
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
		"docs/pitfalls.md":                ".awf/docs/pitfalls/*.md",
		"docs/pitfalls/fixture.md":        ".awf/docs/pitfalls/fixture.md",
		"docs/pitfalls/second-kind.md":    ".awf/docs/pitfalls/second-kind.md",
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
	index := byPath["docs/pitfalls.md"]
	if strings.Contains(index, ".awf/docs/pitfalls/fixture.md") || strings.Contains(index, ".awf/docs/pitfalls/second-kind.md") {
		t.Errorf("pitfall index names individual sources instead of only corpus guidance:\n%s", index)
	}
	for _, slug := range []string{"fixture", "second-kind"} {
		path := "docs/pitfalls/" + slug + ".md"
		want := "<!-- awf:source .awf/docs/pitfalls/" + slug + ".md -->"
		if !strings.Contains(byPath[path], want) {
			t.Errorf("%s lacks its own exact source marker: %s", path, byPath[path])
		}
	}
	for _, path := range []string{"AGENTS.md", "docs/working-with-awf.md"} {
		if !strings.Contains(byPath[path], "<!-- awf:edit ") {
			t.Errorf("actionable output %s lost its awf:edit guidance", path)
		}
	}
	for _, path := range []string{
		"AGENTS.md",
		"docs/working-with-awf.md",
		"docs/decisions/README.md",
		"docs/decisions/template.md",
		"docs/plans/README.md",
		"docs/plans/template.md",
		".claude/skills/example-tdd/SKILL.md",
		".pi/skills/example-tdd/SKILL.md",
		".claude/agents/code-reviewer.md",
		".pi/agents/code-reviewer.md",
		".pi/extensions/awf-subagents/index.ts",
		".awf/hooks/pre-commit.sh",
	} {
		content, ok := byPath[path]
		if !ok {
			t.Errorf("missing representative unmarked output %s", path)
			continue
		}
		if strings.Contains(content, "<!-- awf:source ") {
			t.Errorf("unapproved family %s gained a source marker", path)
		}
	}
	for _, path := range []string{
		"docs/decisions/0001-fixture.md",
		"docs/plans/2026-08-07-fixture.md",
	} {
		if _, ok := byPath[path]; ok {
			t.Errorf("authored artifact %s was rendered", path)
		}
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("read authored artifact %s: %v", path, err)
		}
		if strings.Contains(string(content), bannerText) || strings.Contains(string(content), "<!-- awf:source ") {
			t.Errorf("authored artifact %s gained generated provenance", path)
		}
	}
}
