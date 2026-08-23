package project

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// A non-member tag on a pitfall yields tag drift and an empty-meaning member
// yields tag-vocabulary drift. Legacy ADR tags remain parsed history and do not
// participate in current vocabulary membership.
// invariant: config/configuration:tag-vocabulary-governed (TestCheckTagVocabulary)
func TestCheckTagVocabulary(t *testing.T) {
	cfg := "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\ndomains: [rendering]\n" +
		"tags:\n  render-engine: the render engine\n  empty: \"\"\n"
	root := scaffoldFiles(t, cfg, map[string]string{
		"docs/pitfalls/p.md": pitfallSource("P", "tags: [render-engine, ghost]\n"),
	})
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
			testsupport.WithTags("render-engine", "legacy-only"), testsupport.WithTitle("0001: A"),
			testsupport.WithBody("## Context\nx\n")))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := checkTagVocabulary(renderInputsForTest(p), mustPitfallCorpus(t, p))
	if err != nil {
		t.Fatalf("checkTagVocabulary: %v", err)
	}
	got := map[string]string{}
	for _, d := range drift {
		got[d.Kind] = d.Detail
	}
	if len(drift) != 2 || !strings.Contains(got["pitfall-tag"], "ghost") ||
		!strings.Contains(got["tag-vocabulary"], "empty") {
		t.Fatalf("want pitfall-tag(ghost)+tag-vocabulary(empty) and no legacy ADR drift, got %#v", drift)
	}
	if _, ok := got["adr-tag"]; ok {
		t.Fatalf("legacy ADR tags must not produce current vocabulary drift: %#v", drift)
	}
}

// An empty/absent vocabulary makes the membership rule inert.
func TestCheckTagVocabularyInert(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\ndomains: []\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
			testsupport.WithTags("anything"), testsupport.WithTitle("0001: A"),
			testsupport.WithBody("## Context\nx\n")))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := checkTagVocabulary(renderInputsForTest(p), mustPitfallCorpus(t, p))
	if err != nil || drift != nil {
		t.Fatalf("empty vocabulary must be inert, got %#v / %v", drift, err)
	}
}

// With a non-empty vocabulary but the pitfalls doc disabled, checkTagVocabulary
// has no current carriers to validate; legacy ADR tags remain historical.
func TestCheckTagVocabularyPitfallsDisabled(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\ndomains: []\ntags:\n  rendering: the render engine\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
			testsupport.WithTags("rendering"), testsupport.WithTitle("0001: A"),
			testsupport.WithBody("## Context\nx\n")))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := checkTagVocabulary(renderInputsForTest(p), mustPitfallCorpus(t, p))
	if err != nil || drift != nil {
		t.Fatalf("legacy ADR with pitfalls disabled must yield no drift, got %#v / %v", drift, err)
	}
}

// A vocabulary member equal to a configured domain name is the coarse-tag
// regression, gated exactly; inert when no domains are configured.
// invariant: config/validation:tag-not-domain-name (TestCheckTagVocabularyDomainCollision)
func TestCheckTagVocabularyDomainCollision(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\ndomains: [rendering]\n"+
		"tags:\n  rendering: coarse\n  narrow: a narrow topic\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := checkTagVocabulary(renderInputsForTest(p), mustPitfallCorpus(t, p))
	if err != nil {
		t.Fatal(err)
	}
	var got string
	for _, d := range drift {
		if d.Kind == "tag-domain-collision" {
			got = d.Detail
		}
	}
	if !strings.Contains(got, "rendering") {
		t.Fatalf("want tag-domain-collision for rendering, got %+v", drift)
	}
	// No domains configured: the collision rule is inert.
	root2 := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\ndomains: []\n"+
		"tags:\n  rendering: fine when no domains\n")
	p2, err := Open(testContext(t), root2)
	if err != nil {
		t.Fatal(err)
	}
	drift2, err := checkTagVocabulary(renderInputsForTest(p2), mustPitfallCorpus(t, p2))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range drift2 {
		if d.Kind == "tag-domain-collision" {
			t.Errorf("no collision expected with no domains; got %+v", drift2)
		}
	}
}
