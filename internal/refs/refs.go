// Package refs extracts internal markdown link targets from rendered content.
// It is pure and stdlib-only: it performs no I/O and resolves no paths — callers
// resolve and stat the returned targets. (ADR-0020)
package refs

import "strings"

// Links returns the relative-path targets of inline markdown links — [text](target)
// — in content, in order of appearance. Image-link destinations (![alt](target)) are
// extracted too, so a dead image counts as a dead reference. It skips: external
// targets (http://, https://, mailto:, tel:) and bare #fragment anchors; links inside
// a fenced code block (opened by ``` or ~~~); and links inside an inline code span
// (text between paired `-backtick delimiters). A trailing #anchor and a (target
// "title") title are stripped, leaving the bare relative path. Reference-style links
// ([text][id]) and 4-space-indented code blocks are out of scope.
func Links(content string) []string {
	var out []string
	for _, line := range strings.Split(WithoutFences(content), "\n") {
		out = append(out, lineLinks(stripCodeSpans(line))...)
	}
	return out
}

// WithoutFences returns content minus fenced-code-block lines (opened by ``` or
// ~~~; delimiter lines dropped too). Inline code spans are deliberately kept —
// scanners like the skill-reference check (ADR-0046) match tokens that
// legitimately render inside single-backtick spans.
func WithoutFences(content string) string {
	var kept []string
	inFence := false
	fence := ""
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if inFence {
			if strings.HasPrefix(trimmed, fence) {
				inFence = false
			}
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "```"):
			inFence, fence = true, "```"
		case strings.HasPrefix(trimmed, "~~~"):
			inFence, fence = true, "~~~"
		default:
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// stripCodeSpans removes the content of inline code spans — text between paired
// `-backtick delimiters — so link syntax inside a code span is not parsed as a link.
// An unpaired trailing backtick is not a span: its segment is kept intact.
func stripCodeSpans(line string) string {
	parts := strings.Split(line, "`")
	var b strings.Builder
	for i, p := range parts {
		switch {
		case i%2 == 0:
			b.WriteString(p) // outside any span
		case i == len(parts)-1:
			b.WriteString("`" + p) // unpaired trailing backtick — keep literally
		}
		// odd index that is not the last: inside a paired span — dropped
	}
	return b.String()
}

// lineLinks extracts the target of every [text](target) on a single line. The
// closing ] is matched with bracket nesting so a link whose text is itself a
// link — a badge, [![alt](img)](target) — yields both the inner image target
// and the outer destination, not just the inner one.
func lineLinks(line string) []string {
	var out []string
	for i := 0; i < len(line); {
		open := strings.IndexByte(line[i:], '[')
		if open < 0 {
			return out
		}
		open += i
		closing := matchingBracket(line, open)
		if closing < 0 || closing+1 >= len(line) || line[closing+1] != '(' {
			i = open + 1 // not a link — nested candidates start after this [
			continue
		}
		end := strings.IndexByte(line[closing+2:], ')')
		if end < 0 {
			i = open + 1
			continue
		}
		end += closing + 2
		out = append(out, lineLinks(line[open+1:closing])...) // e.g. a badge image
		if t := normalizeTarget(line[closing+2 : end]); t != "" {
			out = append(out, t)
		}
		i = end + 1
	}
	return out
}

// matchingBracket returns the index of the ] closing the [ at open, tracking
// nested bracket pairs; -1 when unclosed.
func matchingBracket(line string, open int) int {
	depth := 0
	for j := open; j < len(line); j++ {
		switch line[j] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// normalizeTarget strips an optional title and trailing #anchor, unwraps an
// <...> destination, and returns "" for external or anchor-only targets.
func normalizeTarget(dest string) string {
	dest = strings.TrimSpace(dest)
	if strings.HasPrefix(dest, "<") {
		// A <...> destination is markdown's only way to write a target containing
		// spaces — unwrap it before the whitespace/title cut, not after.
		dest = dest[1:]
		if end := strings.IndexByte(dest, '>'); end >= 0 {
			dest = dest[:end]
		}
	} else if i := strings.IndexAny(dest, " \t"); i >= 0 {
		dest = dest[:i]
	}
	if i := strings.IndexByte(dest, '#'); i >= 0 {
		dest = dest[:i]
	}
	if dest == "" {
		return ""
	}
	for _, scheme := range []string{"http://", "https://", "mailto:", "tel:"} {
		if strings.HasPrefix(dest, scheme) {
			return ""
		}
	}
	return dest
}
