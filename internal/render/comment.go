package render

import (
	"fmt"
	"strings"
)

// commentOpen is the exact authoring-comment literal (ADR-0121). The strip and
// the documented invariants.sources marker must stay byte-identical, so a
// comment that strips here is exactly a comment the scanner can read; a
// whitespace variant is not the directive and passes through visibly.
const commentOpen = "<!-- awf:comment"

// StripAuthoringComments removes whole-line awf:comment authoring directives
// from src: a line whose trimmed form opens with the exact commentOpen literal
// at a token boundary (followed by a space, a tab, "-->", or the end of the
// line) and ends with "-->" is removed together with its trailing newline.
// Fenced code blocks are preserved verbatim, so a part or template can
// demonstrate the syntax. A whole line outside a fence that opens at the
// boundary but does not end with "-->" - a missing close, the bare opener, or
// text trailing the close - is a hard error; the input is returned unchanged
// alongside it. Mid-line occurrences and prefix-sharing tokens (awf:commentary)
// never fire.
func StripAuthoringComments(src string) (string, error) {
	var kept []string
	inFence := false
	fence := ""
	for i, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if inFence {
			if strings.HasPrefix(trimmed, fence) {
				inFence = false
			}
			kept = append(kept, line)
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "```"):
			inFence, fence = true, "```"
			kept = append(kept, line)
		case strings.HasPrefix(trimmed, "~~~"):
			inFence, fence = true, "~~~"
			kept = append(kept, line)
		case opensAuthoringComment(trimmed):
			if !strings.HasSuffix(trimmed, "-->") {
				return src, fmt.Errorf("line %d: malformed awf:comment (the whole line must end with \"-->\"): %s", i+1, trimmed)
			}
			// A directive line: dropped, its newline consumed by the join.
		default:
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n"), nil
}

// opensAuthoringComment reports whether a trimmed line opens with the exact
// directive literal at a token boundary: followed by whitespace, "-->", or
// the end of the line. "<!-- awf:commentary" fails the boundary.
func opensAuthoringComment(trimmed string) bool {
	rest, ok := strings.CutPrefix(trimmed, commentOpen)
	if !ok {
		return false
	}
	return rest == "" || strings.HasPrefix(rest, " ") || strings.HasPrefix(rest, "\t") || strings.HasPrefix(rest, "-->")
}
