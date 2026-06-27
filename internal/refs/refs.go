// Package refs extracts internal markdown link targets from rendered content.
// It is pure and stdlib-only: it performs no I/O and resolves no paths — callers
// resolve and stat the returned targets. (ADR-0020)
package refs

import "strings"

// Links returns the relative-path targets of inline markdown links — [text](target)
// — in content, in order of appearance. It skips: external targets (http://,
// https://, mailto:, tel:) and bare #fragment anchors; and any link inside a fenced
// code block (opened by ``` or ~~~). A trailing #anchor and a (target "title") title
// are stripped, leaving the bare relative path. Reference-style links ([text][id])
// are out of scope.
func Links(content string) []string {
	var out []string
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
			out = append(out, lineLinks(line)...)
		}
	}
	return out
}

// lineLinks extracts the target of every [text](target) on a single line.
func lineLinks(line string) []string {
	var out []string
	for {
		open := strings.IndexByte(line, '[')
		if open < 0 {
			return out
		}
		rest := line[open+1:]
		mid := strings.Index(rest, "](")
		if mid < 0 {
			return out
		}
		dest := rest[mid+2:]
		end := strings.IndexByte(dest, ')')
		if end < 0 {
			line = rest
			continue
		}
		if t := normalizeTarget(dest[:end]); t != "" {
			out = append(out, t)
		}
		line = dest[end+1:]
	}
}

// normalizeTarget strips an optional title and trailing #anchor, unwraps an
// <...> destination, and returns "" for external or anchor-only targets.
func normalizeTarget(dest string) string {
	dest = strings.TrimSpace(dest)
	if i := strings.IndexAny(dest, " \t"); i >= 0 {
		dest = dest[:i]
	}
	dest = strings.TrimPrefix(dest, "<")
	dest = strings.TrimSuffix(dest, ">")
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
