package adr

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// StatusEntry is one parsed `## Status history` line (ADR-0135 item 6).
type StatusEntry struct {
	Date        string
	Status      string
	Digest      string // content-sha256; empty for the Proposed scaffold entry
	Sequence    int    // state-sequence; meaningful only when HasSequence
	HasSequence bool
	Rationale   string // Abandoned rationale; empty otherwise
}

// historyHeadRe matches the mandatory `- <date>: <status>` lead of a Status
// history entry; group 3 is the optional `; ...` metadata tail.
var historyHeadRe = regexp.MustCompile(`^- (\d{4}-\d{2}-\d{2}): (Proposed|Accepted|Implemented|Abandoned)(;.*)?$`)

// hexDigestRe matches a lowercase 64-hex-character content-sha256.
var hexDigestRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// parseStatusHistory parses the `## Status history` section into ordered
// entries. It validates each line's shape and metadata tail but leaves
// cross-entry semantics (date monotonicity, the transition matrix, digest
// recomputation, and final-status agreement) to the ADR-level validator.
func parseStatusHistory(body string) ([]StatusEntry, error) {
	lines := nonBlankLines(body)
	if len(lines) == 0 {
		return nil, errors.New("status history is empty")
	}
	entries := make([]StatusEntry, 0, len(lines))
	for _, line := range lines {
		m := historyHeadRe.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("malformed Status history entry: %q", line)
		}
		entry := StatusEntry{Date: m[1], Status: m[2]}
		if err := parseHistoryTail(&entry, m[3]); err != nil {
			return nil, fmt.Errorf("status history entry %q: %w", line, err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// parseHistoryTail consumes the canonical `; content-sha256: ...`,
// `; state-sequence: ...`, and `; rationale: ...` segments in order. Rationale,
// which may itself contain `; `, is terminal and consumes the remainder.
func parseHistoryTail(entry *StatusEntry, tail string) error {
	seenSeq := false
	for tail != "" {
		rest, ok := strings.CutPrefix(tail, "; ")
		if !ok {
			return fmt.Errorf("malformed metadata segment %q", tail)
		}
		switch {
		case strings.HasPrefix(rest, "content-sha256: "):
			if entry.Digest != "" || seenSeq {
				return errors.New("content-sha256 is duplicated or out of order")
			}
			val, more := splitSegment(strings.TrimPrefix(rest, "content-sha256: "))
			if !hexDigestRe.MatchString(val) {
				return fmt.Errorf("content-sha256 is not a 64-hex digest: %q", val)
			}
			entry.Digest, tail = val, more
		case strings.HasPrefix(rest, "state-sequence: "):
			if entry.HasSequence {
				return errors.New("state-sequence is duplicated")
			}
			val, more := splitSegment(strings.TrimPrefix(rest, "state-sequence: "))
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 {
				return fmt.Errorf("state-sequence is not a positive integer: %q", val)
			}
			entry.Sequence, entry.HasSequence, seenSeq, tail = n, true, true, more
		case strings.HasPrefix(rest, "rationale: "):
			// The `rationale: ` prefix requires content after the space; a bare
			// `rationale:` is trimmed to that by nonBlankLines and falls through to
			// the unknown-segment default, so the value here is always nonempty.
			entry.Rationale, tail = strings.TrimPrefix(rest, "rationale: "), ""
		default:
			return fmt.Errorf("unknown metadata segment %q", rest)
		}
	}
	return nil
}

// splitSegment splits value from any following `; `-introduced segment.
func splitSegment(s string) (value, rest string) {
	if i := strings.Index(s, "; "); i >= 0 {
		return s[:i], s[i:]
	}
	return s, ""
}
