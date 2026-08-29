package publisher

import (
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/render"
)

// inPlaceBoundary owns the renderer's in-place read-back boundary. It recognizes
// the current section's registered pointer, the following registered pointers,
// and the awf-owned structural framing around the adopter-owned body.
type inPlaceBoundary struct {
	name            string
	declared        []string
	style           render.CommentStyle
	expectedHeading string
	expectedSymbols map[string]string
}

func newInPlaceBoundary(name string, declared []string, style render.CommentStyle, expectedHeading string, expectedSymbols map[string]string) inPlaceBoundary {
	return inPlaceBoundary{
		name:            name,
		declared:        declared,
		style:           style,
		expectedHeading: expectedHeading,
		expectedSymbols: expectedSymbols,
	}
}

// readBody extracts the adopter-owned body within the boundary. A missing own
// pointer returns found=false so assembly retains the template default.
func (b inPlaceBoundary) readBody(output string) (body string, found bool) {
	lines := strings.Split(output, "\n")
	ownPrefixes := render.PointerLinePrefixes(b.name, b.style)
	start := -1
	for i, line := range lines {
		if hasAnyPrefix(strings.TrimSpace(line), ownPrefixes) {
			start = i
			break
		}
	}
	if start < 0 {
		return "", false
	}

	var boundaryPrefixes []string
	for _, declared := range b.declared {
		if declared != b.name {
			boundaryPrefixes = append(boundaryPrefixes, render.PointerLinePrefixes(declared, b.style)...)
		}
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if hasAnyPrefix(strings.TrimSpace(lines[i]), boundaryPrefixes) {
			end = i
			// Only the exact renderer-owned symbol for this registered next
			// section is framing. Lookalikes remain adopter body content.
			if i > start+1 {
				for _, declared := range b.declared {
					if declared != b.name && hasAnyPrefix(strings.TrimSpace(lines[i]), render.PointerLinePrefixes(declared, b.style)) && strings.TrimSpace(lines[i-1]) == b.expectedSymbols[declared] {
						end = i - 1
						break
					}
				}
			}
			break
		}
	}

	bodyLines := lines[start+1 : end]
	if b.expectedHeading != "" && len(bodyLines) > 0 {
		// A structural slot is awf-owned. Any ATX heading occupying it is tamper,
		// regardless of level; a body heading is preserved only when that slot is
		// genuinely absent.
		if bodyLines[0] == b.expectedHeading || atxHeadingLine(strings.TrimSpace(bodyLines[0])) {
			bodyLines = bodyLines[1:]
		}
	}
	return trimBlankFraming(bodyLines), true
}

func atxHeadingLine(s string) bool {
	i := 0
	for i < len(s) && s[i] == '#' {
		i++
	}
	return i > 0 && i <= 6 && i < len(s) && (s[i] == ' ' || s[i] == '\t')
}

// trimBlankFraming drops leading and trailing blank (whitespace-only) lines - the
// awf-owned framing - and returns the interior lines joined verbatim.
func trimBlankFraming(lines []string) string {
	lo, hi := 0, len(lines)
	for lo < hi && strings.TrimSpace(lines[lo]) == "" {
		lo++
	}
	for hi > lo && strings.TrimSpace(lines[hi-1]) == "" {
		hi--
	}
	return strings.Join(lines[lo:hi], "\n")
}

// hasAnyPrefix reports whether s begins with any of the given prefixes.
func hasAnyPrefix(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}
