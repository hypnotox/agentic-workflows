package adr

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// decisionItem is the package-owned identity and exact source of one Decision
// list item. It intentionally remains private until a production consumer needs
// semantic lookup.
type decisionItem struct {
	ordinal int
	slug    string
	source  string
}

var decisionItemRe = regexp.MustCompile(`(?m)^([0-9]+)\. `)
var v4DecisionLeadRe = regexp.MustCompile(`^([0-9]+)\. ` + "`decision: ([a-z0-9]+(?:-[a-z0-9]+)*)`" + `(?:[ \t]+)(\S.*)$`)

func retainDecisionItems(a *ADR) {
	if a.DecisionStart == 0 && a.DecisionEnd == 0 {
		return
	}
	section := a.Source[a.DecisionStart:a.DecisionEnd]
	newline := strings.IndexByte(section, '\n')
	if newline < 0 { // coverage-ignore: a retained section range always includes its heading newline
		return
	}
	bodyOffset := a.DecisionStart + newline + 1
	var starts []int
	var fence byte
	var fenceLen int
	pos := bodyOffset
	for _, raw := range rangeLines(a.Source[bodyOffset:a.DecisionEnd]) {
		line := strings.TrimSuffix(raw, "\n")
		if marker, n, ok := fenceMarker(line); ok {
			if fence == 0 {
				fence, fenceLen = marker, n
			} else if marker == fence && n >= fenceLen && fenceCloser(line, n) {
				fence, fenceLen = 0, 0
			}
			pos += len(raw)
			continue
		}
		if fence == 0 && decisionItemRe.MatchString(line) {
			starts = append(starts, pos)
		}
		pos += len(raw)
	}
	a.decisions = make([]decisionItem, 0, len(starts))
	for i, start := range starts {
		end := a.DecisionEnd
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		m := decisionItemRe.FindStringSubmatch(a.Source[start:end])
		n, _ := strconv.Atoi(m[1])
		a.decisions = append(a.decisions, decisionItem{ordinal: n, source: a.Source[start:end]})
	}
}

func validateV4Decisions(a *ADR) error {
	seen := map[string]bool{}
	for i := range a.decisions {
		item := &a.decisions[i]
		line := item.source
		if n := strings.IndexByte(line, '\n'); n >= 0 {
			line = line[:n]
		}
		m := v4DecisionLeadRe.FindStringSubmatch(line)
		if m == nil || m[0] != line {
			return fmt.Errorf("decision item %d must begin %q followed by nonempty commitment prose", item.ordinal, "N. `decision: lowercase-kebab-slug`")
		}
		if seen[m[2]] {
			return fmt.Errorf("duplicate decision slug %q", m[2])
		}
		seen[m[2]] = true
		item.slug = m[2]
	}
	return nil
}
