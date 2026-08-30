package pitfallcheck

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

func TestCheckClassifiesUnknownDomain(t *testing.T) {
	corpus, err := pitfall.Load([]pitfall.SourceFile{{
		Path: pitfall.SourceDir + "/bad.md", Regular: true,
		Bytes: []byte("---\ntitle: Bad\ndomains: [missing]\n---\nbody\n"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Check([]string{"rendering"}, corpus)
	if err != nil {
		t.Fatal(err)
	}
	findings := result.Findings()
	if len(findings) != 1 || findings[0].Evidence.Kind != "pitfall-domain" {
		t.Fatalf("findings = %#v", findings)
	}
	finding := findings[0]
	if finding.Rank != severity.Error || finding.Property != PropertyCorrectness || finding.Evidence.Path != pitfall.SourceDir+"/bad.md" {
		t.Fatalf("classification = %#v", finding)
	}
}
