package adr

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Decision is one resolved ADR Decision item. Markdown retains its exact source.
type Decision struct {
	Key         string
	ADRIdentity string
	Title       string
	Status      string
	Markdown    string
}

type decisionItem struct {
	ordinal int
	slug    string
	source  string
}

// ErrDecisionSelectorIncompatible reports a selector that cannot address the
// target ADR format.
var ErrDecisionSelectorIncompatible = errors.New("incompatible Decision selector")

// ErrDecisionSelectorAmendable reports an ordinal selector against amendable
// pre-V4 Decision content.
var ErrDecisionSelectorAmendable = errors.New("amendable Decision content")

// ErrDecisionSelectorUnknown reports a compatible selector absent from a record.
var ErrDecisionSelectorUnknown = errors.New("unknown Decision selector")

// DecisionSelectorError describes a typed Decision lookup failure. Available is
// sorted and contains every selector currently supported by the ADR.
type DecisionSelectorError struct {
	Selector  string
	Available []string
	cause     error
}

func (e *DecisionSelectorError) Error() string {
	return fmt.Sprintf("%v; available: %s", e.cause, strings.Join(e.Available, ", "))
}

func (e *DecisionSelectorError) Unwrap() error { return e.cause }

var decisionItemRe = regexp.MustCompile(`(?m)^([0-9]+)\. `)
var v4DecisionLeadRe = regexp.MustCompile(`^([0-9]+)\. ` + "`decision: ([a-z0-9]+(?:-[a-z0-9]+)*)`" + `(?:[ \t]+)(\S.*)$`)
var ordinalSelectorRe = regexp.MustCompile(`^#[1-9][0-9]*$`)

func retainDecisionItems(a *ADR) {
	if a.decisionStart == 0 && a.decisionEnd == 0 {
		return
	}
	section := a.source[a.decisionStart:a.decisionEnd]
	newline := strings.IndexByte(section, '\n')
	if newline < 0 {
		return
	}
	bodyOffset := a.decisionStart + newline + 1
	var starts []int
	var fence byte
	var fenceLen, pos = 0, bodyOffset
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
func (a ADR) decisionBySelector(selector string) (decisionItem, error) {
	if a.IsV4() {
		if ordinalSelectorRe.MatchString(selector) {
			return decisionItem{}, fmt.Errorf("%w %q for V4 slug navigation", ErrDecisionSelectorIncompatible, selector)
		}
		i, ok := a.decisionBySlug[selector]
		if !ok {
			return decisionItem{}, fmt.Errorf("unknown V4 Decision selector %q: %w", selector, ErrDecisionSelectorUnknown)
		}
		return a.decisions[i], nil
	}
	if !ordinalSelectorRe.MatchString(selector) {
		return decisionItem{}, fmt.Errorf("%w %q for pre-V4 ordinal navigation", ErrDecisionSelectorIncompatible, selector)
	}
	if a.IsContentAmendable() {
		return decisionItem{}, fmt.Errorf("%w for selector %q", ErrDecisionSelectorAmendable, selector)
	}
	ordinal, _ := strconv.Atoi(strings.TrimPrefix(selector, "#"))
	for _, item := range a.decisions {
		if item.ordinal == ordinal {
			return item, nil
		}
	}
	return decisionItem{}, fmt.Errorf("unknown pre-V4 Decision selector %q: %w", selector, ErrDecisionSelectorUnknown)
}

// DecisionSelectors returns the stable selectors supported by this ADR in sorted order.
func (a ADR) DecisionSelectors() []string {
	out := make([]string, 0, len(a.decisions))
	if a.IsV4() {
		for _, item := range a.decisions {
			out = append(out, item.slug)
		}
	} else if !a.IsContentAmendable() {
		for _, item := range a.decisions {
			out = append(out, "#"+strconv.Itoa(item.ordinal))
		}
	}
	sort.Strings(out)
	return out
}

// Decisions returns every Decision item addressable by this ADR in source order.
// Amendable pre-V4 records have no stable item identity and therefore return none.
func (a ADR) Decisions() []Decision {
	if !a.IsV4() && a.IsContentAmendable() {
		return nil
	}
	out := make([]Decision, 0, len(a.decisions))
	for _, item := range a.decisions {
		selector := item.slug
		if !a.IsV4() {
			selector = "#" + strconv.Itoa(item.ordinal)
		}
		out = append(out, Decision{Key: a.Identity() + ":" + selector, ADRIdentity: a.Identity(), Title: a.Title, Status: a.Status, Markdown: item.source})
	}
	return out
}

// LookupDecision resolves one compatible selector and preserves its exact Markdown.
func (a ADR) LookupDecision(selector string) (Decision, error) {
	item, err := a.decisionBySelector(selector)
	if err != nil {
		return Decision{}, &DecisionSelectorError{Selector: selector, Available: a.DecisionSelectors(), cause: err}
	}
	key := a.Identity() + ":" + selector
	return Decision{Key: key, ADRIdentity: a.Identity(), Title: a.Title, Status: a.Status, Markdown: item.source}, nil
}
