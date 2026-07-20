package adr

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// OpVerb is a current-state-v1 State-changes verb.
type OpVerb string

const (
	// OpAdd introduces a new claim.
	OpAdd OpVerb = "add"
	// OpUpdate revises an existing claim.
	OpUpdate OpVerb = "update"
	// OpRemove retires an existing claim.
	OpRemove OpVerb = "remove"
)

// Operation is one parsed `## State changes` entry: a verb over a qualified
// claim ID `<domain>/<topic>:<local-slug>` (ADR-0135 item 3).
type Operation struct {
	Verb OpVerb
	ID   string // qualified claim ID
	Slug string // local slug component of ID
}

// slugPart is the shared identifier grammar for a domain, topic, or local slug
// (ADR-0134 item 1): lowercase alphanumerics in hyphen-separated groups.
const slugPart = `[a-z0-9]+(?:-[a-z0-9]+)*`

// stateOpRe matches exactly `- <verb> ` + one inline-code qualified ID and
// nothing else on the line.
var stateOpRe = regexp.MustCompile("^- (add|update|remove) `(" + slugPart + "/" + slugPart + ":" + slugPart + ")`$")

// qualifiedIDRe validates a bare qualified claim ID.
var qualifiedIDRe = regexp.MustCompile("^" + slugPart + "/" + slugPart + ":(" + slugPart + ")$")

// parseStateChanges parses the `## State changes` section body. It returns the
// operations and whether the section is the exclusive `None.` form. Exactly one
// of the two forms is legal: a nonempty operation list, or the single paragraph
// `None.`. Each claim ID may appear at most once (ADR-0135 item 3).
func parseStateChanges(body string) (ops []Operation, none bool, err error) {
	lines := nonBlankLines(body)
	if len(lines) == 0 {
		return nil, false, errors.New("state changes is empty; use `None.` or a list of operations")
	}
	if len(lines) == 1 && lines[0] == "None." {
		return nil, true, nil
	}
	seen := map[string]bool{}
	for _, line := range lines {
		m := stateOpRe.FindStringSubmatch(line)
		if m == nil {
			return nil, false, fmt.Errorf("malformed State changes entry: %q", line)
		}
		id := m[2]
		if seen[id] {
			return nil, false, fmt.Errorf("state changes names %s more than once", id)
		}
		seen[id] = true
		ops = append(ops, Operation{Verb: OpVerb(m[1]), ID: id, Slug: localSlug(id)})
	}
	return ops, false, nil
}

// localSlug returns the local-slug component of a qualified claim ID.
func localSlug(id string) string {
	if m := qualifiedIDRe.FindStringSubmatch(id); m != nil {
		return m[1]
	}
	return "" // coverage-ignore: every caller passes an ID stateOpRe already validated
}

// nonBlankLines returns the section's non-blank, whitespace-trimmed lines.
func nonBlankLines(body string) []string {
	var out []string
	for _, raw := range strings.Split(body, "\n") {
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
