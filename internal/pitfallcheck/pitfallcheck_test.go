package pitfallcheck

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

func TestCheckClassifiesDomainBeforeADRLink(t *testing.T) {
	corpus, err := pitfall.Load([]pitfall.SourceFile{{
		Path: pitfall.SourceDir + "/bad.md", Regular: true,
		Bytes: []byte("---\ntitle: Bad\ndomains: [missing]\nrelated: [42]\n---\nbody\n"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Check([]string{"rendering"}, corpus, adr.Corpus{})
	if err != nil {
		t.Fatal(err)
	}
	findings := result.Findings()
	if len(findings) != 2 || findings[0].Evidence.Kind != "pitfall-domain" || findings[1].Evidence.Kind != "pitfall-adr-link" {
		t.Fatalf("findings = %#v", findings)
	}
	for _, finding := range findings {
		if finding.Rank != severity.Error || finding.Property != PropertyCorrectness || finding.Evidence.Path != pitfall.SourceDir+"/bad.md" {
			t.Fatalf("classification = %#v", finding)
		}
	}
}
