package render

import (
	"fmt"
	"strings"
)

// commentOpen is the exact authoring-comment literal (ADR-0121). The strip and
// the documented currentState.sources marker must stay byte-identical, so a
// comment that strips here is exactly a comment the scanner can read; a
// whitespace variant is not the directive and passes through visibly.
const commentOpen = "<!-- awf:comment"

// StripAuthoringCommentsSource removes authoring directives while retaining each
// kept byte range's source span. It never coalesces adjacent spans, since an
// include boundary remains meaningful even when a comment is stripped nearby.
func StripAuthoringCommentsSource(src SourceText) (SourceText, error) {
	text := src.AuthoredText()
	kept, err := authoringCommentRanges(text)
	if err != nil {
		return src, err
	}
	out := SourceText{Root: src.Root}
	for _, r := range kept {
		out.appendSource(src.slice(r[0], r[1]))
	}
	return out, nil
}

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
	stripped, err := StripAuthoringCommentsSource(SourceText{Spans: []SourceSpan{{Text: src}}})
	if err != nil {
		return src, err
	}
	return stripped.AuthoredText(), nil
}

// authoringCommentRanges returns the source offsets retained by comment stripping.
func authoringCommentRanges(src string) ([][2]int, error) {
	var kept [][2]int
	inFence := false
	fence := ""
	offset := 0
	for i, raw := range strings.SplitAfter(src, "\n") {
		line := strings.TrimSuffix(raw, "\n")
		trimmed := strings.TrimSpace(line)
		keep := true
		if inFence {
			if strings.HasPrefix(trimmed, fence) {
				inFence = false
			}
		} else {
			switch {
			case strings.HasPrefix(trimmed, "```"):
				inFence, fence = true, "```"
			case strings.HasPrefix(trimmed, "~~~"):
				inFence, fence = true, "~~~"
			case opensAuthoringComment(trimmed):
				if !strings.HasSuffix(trimmed, "-->") {
					return nil, fmt.Errorf("line %d: malformed awf:comment (the whole line must end with \"-->\"): %s", i+1, trimmed)
				}
				keep = false
			}
		}
		if keep {
			kept = append(kept, [2]int{offset, offset + len(raw)})
		} else if !strings.HasSuffix(raw, "\n") && offset+len(raw) == len(src) && len(kept) > 0 {
			// Split-and-join stripping historically removed the separator before
			// a stripped final physical line. Preserve that output contract while
			// retaining source ranges for every surviving byte.
			last := &kept[len(kept)-1]
			if last[1] > last[0] && src[last[1]-1] == '\n' {
				last[1]--
			}
		}
		offset += len(raw)
	}
	return kept, nil
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
