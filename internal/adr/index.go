package adr

import (
	"fmt"
	"sort"
	"strings"
)

// RenderIndexMD renders the decisions/INDEX.md index for corpus (ADR-0135 item
// 8). It replaces the status-partitioned ACTIVE.md with two sections: "In
// flight" lists the Proposed and Accepted ADRs whose adoption is still under
// way, and "History" is a compact roll of the terminal Implemented and
// Abandoned decisions kept only as rationale. Both sections always render, with
// a placeholder line when empty, so the content is never blank and its
// document-map link resolves out of the box. The content carries no
// generated-by banner - that is the caller's job (internal/project's
// generateIndexMD, via injectBanner).
func RenderIndexMD(corpus Corpus) string {
	var inflight, history []ADR
	for _, a := range corpus.All() {
		if a.IsInflight() {
			inflight = append(inflight, a)
		} else {
			history = append(history, a)
		}
	}

	var sb strings.Builder
	renderIndexSection(&sb, "In flight", inflight, "_No decisions are in flight._")
	sb.WriteString("\n")
	renderIndexSection(&sb, "History", history, "_No decisions recorded yet._")
	return sb.String()
}

// renderIndexSection writes one INDEX.md section: a heading, then either one
// number-sorted bullet per ADR or the empty-section placeholder.
func renderIndexSection(sb *strings.Builder, heading string, adrs []ADR, empty string) {
	fmt.Fprintf(sb, "## %s\n\n", heading)
	if len(adrs) == 0 {
		sb.WriteString(empty)
		sb.WriteString("\n")
		return
	}
	sort.Slice(adrs, func(i, j int) bool { return adrs[i].Number < adrs[j].Number })
	for _, a := range adrs {
		fmt.Fprintf(sb, "- [%s](%s) (%s)\n", a.Title, a.Filename, a.Status)
	}
}
