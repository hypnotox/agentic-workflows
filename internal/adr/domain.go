package adr

import (
	"fmt"
	"strings"
)

// RenderDomainIndex renders the per-domain ADR index for corpus: every ADR whose domains frontmatter includes domain, grouped by status in
// the same order as ACTIVE.md, with links relative to docs/domains/ (one dir over)
// and each superseded entry annotated with its successor. Returns a placeholder
// line when no ADR matches, so the rendered section is never empty.
func RenderDomainIndex(corpus Corpus, domain string) string {
	groups, ordered := groupByStatus(corpus.All(), func(a ADR) bool {
		for _, d := range a.Domains {
			if d == domain {
				return true
			}
		}
		return false
	})
	if len(groups) == 0 {
		return "_No decisions recorded for this domain yet._\n"
	}

	var sb strings.Builder
	for i, status := range ordered {
		if i > 0 {
			sb.WriteString("\n")
		}
		fmt.Fprintf(&sb, "### %s\n\n", status)
		for _, a := range groups[status] {
			fmt.Fprintf(&sb, "- [%s](../decisions/%s)", a.Title, a.Filename)
			// Claimant numbers only, never individual anchors (ADR-0129 item 5).
			// The scalar successor this replaces could name one ADR; coverage
			// may split across several, and a per-anchor list would turn a
			// domain index into a supersession report.
			// touches-invariant: domain-index-surfaces-partial - the claimant render; proof in domain_test.go
			if claimants := corpus.Retirers(a.Number); len(claimants) > 0 {
				fmt.Fprintf(&sb, " \u2192 superseded by %s", joinADRs(claimants))
			}
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
