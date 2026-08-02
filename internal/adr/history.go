package adr

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// HistoryEventKind distinguishes lifecycle events from operation-application
// events in current-state-v2 Status history.
type HistoryEventKind uint8

const (
	HistoryStatus HistoryEventKind = iota + 1
	HistoryApplied
	HistoryReapplied
	HistoryAmended
)

// HistoryEvent is one parsed `## Status history` line. LegacySequence records
// that a retired state-sequence segment was tolerated and discarded, so the
// check layer can direct the project to awf upgrade (ADR-0191).
type HistoryEvent struct {
	Kind           HistoryEventKind
	Date           string
	Status         string
	Digest         string
	LegacySequence bool
	Rationale      string
	Operations     []Operation
}

// StatusEntry preserves the source-compatible V1 name while ADR.History uses
// the common event representation.
type StatusEntry = HistoryEvent

var (
	v1HistoryHeadRe = regexp.MustCompile(`^- (\d{4}-\d{2}-\d{2}): (Proposed|Accepted|Implemented|Abandoned)(;.*)?$`)
	v2HistoryHeadRe = regexp.MustCompile(`^- (\d{4}-\d{2}-\d{2}): (Proposed|Accepted|Implementing|Implemented|Abandoned)(;.*)?$`)
	appliedHeadRe   = regexp.MustCompile(`^- (\d{4}-\d{2}-\d{2}): Applied; (state-sequence: [1-9][0-9]*; )?operations: (.+)$`)
	reappliedHeadRe = regexp.MustCompile(`^- (\d{4}-\d{2}-\d{2}): Reapplied; operations: (.+)$`)
	amendedHeadRe   = regexp.MustCompile(`^- (\d{4}-\d{2}-\d{2}): Amended; content-sha256: ([0-9a-f]{64})$`)
	appliedOpRe     = regexp.MustCompile("^(add|update|remove) `(" + slugPart + "/" + slugPart + ":" + slugPart + ")`$")
)

// hexDigestRe matches a lowercase 64-hex-character content-sha256.
var hexDigestRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

func parseStatusHistory(body string) ([]HistoryEvent, error) {
	return parseHistory(body, CurrentStateV1, nil)
}

func parseV2History(body string, declared []Operation) ([]HistoryEvent, error) {
	return parseHistory(body, CurrentStateV2, declared)
}

// parseHistory validates each line's exact grammar. Cross-event lifecycle,
// digest, date, and cardinality rules are enforced by the format validator.
func parseHistory(body string, format Format, declared []Operation) ([]HistoryEvent, error) {
	lines := nonBlankLines(body)
	if format == CurrentStateV2 {
		lines = exactNonBlankLines(body)
	}
	if len(lines) == 0 {
		return nil, errors.New("status history is empty")
	}
	entries := make([]HistoryEvent, 0, len(lines))
	for _, line := range lines {
		if format == CurrentStateV2 {
			if m := appliedHeadRe.FindStringSubmatch(line); m != nil {
				ops, err := parseAppliedOperations(m[3], declared)
				if err != nil {
					return nil, fmt.Errorf("Status history entry %q: %w", line, err)
				}
				entries = append(entries, HistoryEvent{Kind: HistoryApplied, Date: m[1], LegacySequence: m[2] != "", Operations: ops})
				continue
			}
			if m := reappliedHeadRe.FindStringSubmatch(line); m != nil {
				ops, err := parseQualifiedOperations(m[2], declared, "Reapplied")
				if err != nil {
					return nil, fmt.Errorf("Status history entry %q: %w", line, err)
				}
				entries = append(entries, HistoryEvent{Kind: HistoryReapplied, Date: m[1], Operations: ops})
				continue
			}
			if m := amendedHeadRe.FindStringSubmatch(line); m != nil {
				entries = append(entries, HistoryEvent{Kind: HistoryAmended, Date: m[1], Digest: m[2]})
				continue
			}
		}
		head := v1HistoryHeadRe
		if format == CurrentStateV2 {
			head = v2HistoryHeadRe
		}
		m := head.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("malformed Status history entry: %q", line)
		}
		entry := HistoryEvent{Kind: HistoryStatus, Date: m[1], Status: m[2]}
		if err := parseHistoryTail(&entry, m[3]); err != nil {
			return nil, fmt.Errorf("status history entry %q: %w", line, err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func exactNonBlankLines(body string) []string {
	var lines []string
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func parseAppliedOperations(list string, declared []Operation) ([]Operation, error) {
	return parseQualifiedOperations(list, declared, "Applied")
}

func parseQualifiedOperations(list string, declared []Operation, event string) ([]Operation, error) {
	parts := strings.Split(list, ", ")
	declaredAt := make(map[Operation]int, len(declared))
	for i, op := range declared {
		declaredAt[op] = i
	}
	seen := map[Operation]bool{}
	previous := -1
	ops := make([]Operation, 0, len(parts))
	for _, part := range parts {
		m := appliedOpRe.FindStringSubmatch(part)
		if m == nil {
			return nil, fmt.Errorf("malformed %s operation %q", event, part)
		}
		op := Operation{Verb: OpVerb(m[1]), ID: m[2], Slug: localSlug(m[2])}
		position, ok := declaredAt[op]
		if !ok {
			return nil, fmt.Errorf("%s operation %s `%s` is not declared", strings.ToLower(event), op.Verb, op.ID)
		}
		if seen[op] {
			return nil, fmt.Errorf("%s operation %s `%s` is duplicated", strings.ToLower(event), op.Verb, op.ID)
		}
		if position <= previous {
			return nil, fmt.Errorf("%s operations do not follow State changes declaration order", strings.ToLower(event))
		}
		seen[op] = true
		previous = position
		ops = append(ops, op)
	}
	return ops, nil
}

// parseHistoryTail consumes canonical metadata in digest, retired
// state-sequence (tolerated and discarded, ADR-0191), rationale order.
// Rationale is terminal and may itself contain `; `.
func parseHistoryTail(entry *HistoryEvent, tail string) error {
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
			if entry.LegacySequence {
				return errors.New("state-sequence is duplicated")
			}
			val, more := splitSegment(strings.TrimPrefix(rest, "state-sequence: "))
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 {
				return fmt.Errorf("state-sequence is not a positive integer: %q", val)
			}
			entry.LegacySequence, seenSeq, tail = true, true, more
		case strings.HasPrefix(rest, "rationale: "):
			entry.Rationale, tail = strings.TrimPrefix(rest, "rationale: "), ""
		default:
			return fmt.Errorf("unknown metadata segment %q", rest)
		}
	}
	return nil
}

func splitSegment(s string) (value, rest string) {
	if i := strings.Index(s, "; "); i >= 0 {
		return s[:i], s[i:]
	}
	return s, ""
}
