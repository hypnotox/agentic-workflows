package project

import (
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/glossary"
	"github.com/hypnotox/agentic-workflows/internal/glossarycheck"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

const glossaryCheckCfg = "prefix: example\nintegrationBranch: main\nvars: {}\ndomains: [rendering]\n"

func evaluateGlossaryForTest(p *Session) (checkresult.Result, error) {
	input, err := testPublisher(renderInputsForTest(p)).Glossary()
	if err != nil {
		return checkresult.Result{}, err
	}
	return glossarycheck.Evaluate(input)
}

// A disabled glossary doc is never read, so it can yield no drift.
func TestCheckGlossaryDisabled(t *testing.T) {
	p, err := loadTestSession(testContext(t), scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := evaluateGlossaryForTest(p)
	if err != nil || len(result.Findings()) != 0 || len(result.Information()) != 0 {
		t.Errorf("disabled glossary must yield no findings, got %#v / %v", result, err)
	}
}

// A record naming an unconfigured domain yields glossary-domain drift; records
// resolving their domains, and records carrying none, yield nothing.
// invariant: rendering/doc-outputs:glossary-domains-resolved (TestCheckGlossaryValidatesDomains)
func TestCheckGlossaryValidatesDomains(t *testing.T) {
	p, err := loadTestSession(testContext(t), scaffoldFiles(t, glossaryCheckCfg, map[string]string{
		"docs/glossary.yaml": "data:\n  terms:\n" +
			"    - term: clean\n      meaning: resolves\n      domains: [rendering]\n" +
			"    - term: untagged\n      meaning: no domains at all\n" +
			"    - term: bad\n      meaning: names a domain the project does not configure\n      domains: [bogus]\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	result, err := evaluateGlossaryForTest(p)
	if err != nil {
		t.Fatalf("glossary check: %v", err)
	}
	findings := result.Findings()
	if len(findings) != 1 || findings[0].Evidence.Kind != "glossary-domain" ||
		!strings.Contains(findings[0].Evidence.Detail, "bogus") || !strings.Contains(findings[0].Evidence.Detail, "bad") ||
		findings[0].Evidence.Path != glossary.SidecarPath {
		t.Fatalf("want one glossary-domain finding naming term bad and domain bogus, got %#v", findings)
	}
	// Drive the public surface too: the helper finding it is worth nothing if
	// Check drops the slice on the floor. Check reads the lock, so sync first.
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	full, err := checkProject(p, testContext(t))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !slices.ContainsFunc(full, func(d manifest.Drift) bool {
		return d.Kind == "glossary-domain" && strings.Contains(d.Detail, "bogus")
	}) {
		t.Fatalf("glossary-domain drift did not reach Check's result: %#v", full)
	}
}

// Valid YAML with a bad data.terms shape surfaces the structural error.
func TestCheckGlossaryStructuralError(t *testing.T) {
	p, err := loadTestSession(testContext(t), scaffoldFiles(t, glossaryCheckCfg, map[string]string{
		"docs/glossary.yaml": "data:\n  terms: just a string\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := evaluateGlossaryForTest(p); err == nil || !strings.Contains(err.Error(), "must be a list") {
		t.Fatalf("expected structural error, got %v", err)
	}
}
