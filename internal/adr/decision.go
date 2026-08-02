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
var ordinalSelectorRe = regexp.MustCompile(`^#[1-9][0-9]*$`)

func retainDecisionItems(a *ADR) {
	if a.decisionStart == 0 && a.decisionEnd == 0 {
		return
	}
	section := a.source[a.decisionStart:a.decisionEnd]
	newline := strings.IndexByte(section, '\n')
	if newline < 0 { // coverage-ignore: a retained section range always includes its heading newline
		return
	}
	bodyOffset := a.decisionStart + newline + 1
	var starts []int
	var fence byte
	var fenceLen int
	pos := bodyOffset
	for _, raw := range rangeLines(a.source[bodyOffset:a.decisionEnd]) {
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
		end := a.decisionEnd
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		m := decisionItemRe.FindStringSubmatch(a.source[start:end])
		n, _ := strconv.Atoi(m[1])
		a.decisions = append(a.decisions, decisionItem{ordinal: n, source: a.source[start:end]})
	}
}

func validateV4Decisions(a *ADR) error {
	a.decisionBySlug = make(map[string]int, len(a.decisions))
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
		if _, err := a.decisionBySelector(m[2]); err == nil {
			return fmt.Errorf("duplicate decision slug %q", m[2])
		}
		a.decisionBySlug[m[2]] = i
		item.slug = m[2]
	}
	return nil
}

// decisionBySelector is deliberately private until a cross-package production
// consumer needs it. V4 selects only stable slugs; frozen older formats retain
// canonical ordinal navigation, while amendable ordinal meanings are refused.
func (a ADR) decisionBySelector(selector string) (decisionItem, error) {
	if a.IsV4() {
		i, ok := a.decisionBySlug[selector]
		if !ok {
			return decisionItem{}, fmt.Errorf("unknown V4 Decision selector %q", selector)
		}
		return a.decisions[i], nil
	}
	if !ordinalSelectorRe.MatchString(selector) {
		return decisionItem{}, fmt.Errorf("decision selector %q is incompatible with pre-V4 ordinal navigation", selector)
	}
	if a.IsContentAmendable() {
		return decisionItem{}, fmt.Errorf("decision selector %q targets amendable pre-V4 content", selector)
	}
	ordinal, _ := strconv.Atoi(strings.TrimPrefix(selector, "#"))
	for _, item := range a.decisions {
		if item.ordinal == ordinal {
			return item, nil
		}
	}
	return decisionItem{}, fmt.Errorf("unknown pre-V4 Decision selector %q", selector)
}
