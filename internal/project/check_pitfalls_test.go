package project

import (
	"strings"
	"testing"
)

const pitfallsCheckCfg = "prefix: example\nintegrationBranch: main\nvars: {}\ndomains: [rendering]\n"

// An empty unconditional pitfall corpus yields no project-level drift.
func TestCheckPitfallsEmpty(t *testing.T) {
	p, err := loadTestSession(testContext(t), scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	drift, err := checkPitfalls(renderInputsForTest(p), mustPitfallCorpus(t, p))
	if err != nil || drift != nil {
		t.Errorf("empty pitfalls must yield no drift, got %v / %v", drift, err)
	}
}

// invariant: rendering/doc-outputs:pitfall-domains-resolved (TestCheckPitfallsValidatesDomains)
func TestCheckPitfallsValidatesDomains(t *testing.T) {
	root := scaffoldFiles(t, pitfallsCheckCfg, map[string]string{
		"docs/pitfalls/clean.md":      pitfallSource("Clean", "domains: [rendering]\n"),
		"docs/pitfalls/bad-domain.md": pitfallSource("BadDomain", "domains: [bogus]\n"),
	})
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := checkPitfalls(renderInputsForTest(p), mustPitfallCorpus(t, p))
	if err != nil {
		t.Fatalf("checkPitfalls: %v", err)
	}
	if len(drift) != 1 || drift[0].Kind != "pitfall-domain" || !strings.Contains(drift[0].Detail, "bogus") {
		t.Fatalf("want pitfall-domain(bogus) drift, got %#v", drift)
	}
}

// A malformed authored source is a hard corpus-load error before check projection.
func TestCheckPitfallsStructuralError(t *testing.T) {
	p, err := loadTestSession(testContext(t), scaffoldFiles(t, pitfallsCheckCfg, map[string]string{
		"docs/pitfalls/bad.md": "---\ntitle: Bad\nunknown: value\n---\nbody\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadPitfallCorpus(renderInputsForTest(p)); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("expected structural error, got %v", err)
	}
}
