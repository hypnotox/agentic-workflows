package adr

import (
	"fmt"
	"sort"
	"strings"
)

// RenderDomainIndex renders the per-domain ADR index for the decisions directory
// dir: every ADR whose domains frontmatter includes domain, grouped by status in
// the same order as ACTIVE.md, with links relative to docs/domains/ (one dir over)
// and each superseded entry annotated with its successor. Returns a placeholder
// line when no ADR matches, so the rendered section is never empty.
func RenderDomainIndex(dir, domain string) (string, error) {
	adrs, err := ParseDir(dir)
	if err != nil {
		return "", err
	}
	groups := make(map[string][]ADR)
	for _, a := range adrs {
		for _, d := range a.Domains {
			if d == domain {
				groups[a.Status] = append(groups[a.Status], a)
				break
			}
		}
	}
	if len(groups) == 0 {
		return "_No decisions recorded for this domain yet._\n", nil
	}
	for k := range groups {
		sort.Slice(groups[k], func(i, j int) bool { return groups[k][i].Number < groups[k][j].Number })
	}
	var ordered []string
	seen := map[string]bool{}
	for _, s := range statusOrder {
		if len(groups[s]) > 0 {
			ordered = append(ordered, s)
			seen[s] = true
		}
	}
	var extra []string
	for k := range groups {
		if !seen[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	ordered = append(ordered, extra...)

	var sb strings.Builder
	for i, status := range ordered {
		if i > 0 {
			sb.WriteString("\n")
		}
		fmt.Fprintf(&sb, "### %s\n\n", status)
		for _, a := range groups[status] {
			fmt.Fprintf(&sb, "- [%s](../decisions/%s)", a.Title, a.Filename)
			if a.SupersededBy != "" {
				fmt.Fprintf(&sb, " → superseded by ADR-%s", a.SupersededBy)
			}
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}
