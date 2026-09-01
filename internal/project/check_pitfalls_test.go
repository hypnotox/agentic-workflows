package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/pitfallcheck"
)

const pitfallsCheckCfg = "prefix: example\nintegrationBranch: main\nvars: {}\ndomains: [rendering]\n"

// An empty unconditional pitfall corpus yields no project-level drift.
func TestCheckPitfallsEmpty(t *testing.T) {
	p, err := loadTestSession(testContext(t), scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := pitfallcheck.Check(p.Config().Domains, mustPitfallCorpus(t, p))
	if err != nil || len(result.Findings()) != 0 || len(result.Information()) != 0 {
		t.Errorf("empty pitfalls must yield no findings, got %#v / %v", result, err)
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
	result, err := pitfallcheck.Check(p.Config().Domains, mustPitfallCorpus(t, p))
	if err != nil {
		t.Fatalf("pitfall check: %v", err)
	}
	findings := result.Findings()
	if len(findings) != 1 || findings[0].Evidence.Kind != "pitfall-domain" || !strings.Contains(findings[0].Evidence.Detail, "bogus") {
		t.Fatalf("want pitfall-domain(bogus) finding, got %#v", findings)
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
