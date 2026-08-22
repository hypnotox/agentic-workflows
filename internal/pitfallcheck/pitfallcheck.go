// Package pitfallcheck owns pitfall domain and ADR-link validity policy.
package pitfallcheck

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

// PropertyCorrectness is the protected property for pitfall findings.
const PropertyCorrectness checkresult.Property = "correctness"

// Check evaluates a prepared pitfall corpus against immutable domain and ADR facts.
func Check(domains []string, corpus pitfall.Corpus, adrs adr.Corpus) (checkresult.Result, error) {
	known := map[string]bool{}
	for _, d := range domains {
		known[d] = true
	}
	var findings []checkresult.Finding
	for _, e := range corpus.All() {
		for _, d := range e.Domains {
			if !known[d] {
				findings = append(findings, finding(e.SourcePath, "pitfall-domain", fmt.Sprintf("%s (%q): unknown domain %q", e.Slug, e.Title, d)))
			}
		}
		for _, n := range e.Related {
			if !adrs.Has(fmt.Sprintf("%04d", n)) {
				findings = append(findings, finding(e.SourcePath, "pitfall-adr-link", fmt.Sprintf("%s (%q): ADR-%04d", e.Slug, e.Title, n)))
			}
		}
	}
	return checkresult.New(findings, nil)
}
func finding(path, kind, detail string) checkresult.Finding {
	return checkresult.Finding{Rank: severity.Error, Property: PropertyCorrectness, Evidence: checkresult.Evidence{Path: path, Kind: kind, Detail: detail}}
}
