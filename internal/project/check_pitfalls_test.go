package project

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

const pitfallsCheckCfg = "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\ndomains: [rendering]\n"

// An empty unconditional pitfall corpus yields no project-level drift.
func TestCheckPitfallsEmpty(t *testing.T) {
	p, err := Open(testContext(t), scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	drift, err := checkPitfalls(renderInputsForTest(p), mustDeriveCorpus(t, p), mustPitfallCorpus(t, p))
	if err != nil || drift != nil {
		t.Errorf("empty pitfalls must yield no drift, got %v / %v", drift, err)
	}
}

// An unknown domain yields pitfall-domain drift, a dangling related ADR yields
// pitfall-adr-link drift, and an entry resolving both yields none.
// invariant: rendering/doc-outputs:pitfall-domains-resolved (TestCheckPitfallsValidatesDomainsAndLinks)
// invariant: rendering/doc-outputs:pitfall-adr-link-resolved (TestCheckPitfallsValidatesDomainsAndLinks)
func TestCheckPitfallsValidatesDomainsAndLinks(t *testing.T) {
	root := scaffoldFiles(t, pitfallsCheckCfg, map[string]string{
		"docs/pitfalls/clean.md":      pitfallSource("Clean", "domains: [rendering]\nrelated: [1]\n"),
		"docs/pitfalls/bad-domain.md": pitfallSource("BadDomain", "domains: [bogus]\n"),
		"docs/pitfalls/bad-link.md":   pitfallSource("BadLink", "related: [42]\n"),
	})
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-real.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-12"),
			testsupport.WithTitle("0001: Real"), testsupport.WithBody("## Context\nx\n")))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := checkPitfalls(renderInputsForTest(p), mustDeriveCorpus(t, p), mustPitfallCorpus(t, p))
	if err != nil {
		t.Fatalf("checkPitfalls: %v", err)
	}
	got := map[string]string{}
	for _, d := range drift {
		got[d.Kind] = d.Detail
	}
	if len(drift) != 2 || !strings.Contains(got["pitfall-domain"], "bogus") || !strings.Contains(got["pitfall-adr-link"], "0042") {
		t.Fatalf("want pitfall-domain(bogus) + pitfall-adr-link(0042) drift, got %#v", drift)
	}
}

// A malformed authored source is a hard corpus-load error before check projection.
func TestCheckPitfallsStructuralError(t *testing.T) {
	p, err := Open(testContext(t), scaffoldFiles(t, pitfallsCheckCfg, map[string]string{
		"docs/pitfalls/bad.md": "---\ntitle: Bad\nunknown: value\n---\nbody\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadPitfallCorpus(renderInputsForTest(p)); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("expected structural error, got %v", err)
	}
}
