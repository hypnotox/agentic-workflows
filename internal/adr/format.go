package adr

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
	"gopkg.in/yaml.v3"
)

// V1FormatMarker is the exact `format:` frontmatter value of a current-state-v1
// ADR (ADR-0135 item 1).
const V1FormatMarker = "current-state-v1"

// v1SectionOrder is the required exact, ordered section set of a
// current-state-v1 ADR (ADR-0135 item 2).
var v1SectionOrder = []string{"Context", "Decision", "State changes", "Consequences", "Alternatives Considered", "Status history"}

// v1Frontmatter is the closed current-state-v1 frontmatter: exactly format,
// status, and date. Number and title come from the filename and heading, and
// no topics/domains/tags/related are allowed (ADR-0135 items 1 and 4).
type v1Frontmatter struct {
	Format string `yaml:"format"`
	Status string `yaml:"status"`
	Date   string `yaml:"date"`
}

// ParseV1 parses and validates one current-state-v1 ADR. name is the base
// filename (Number is derived from it); Title comes from the first `# ` heading.
// It enforces the exact frontmatter, status enum, section order, sequential
// Decision items, State-changes and Status-history grammar, and the per-ADR
// lifecycle and digest rules. Cross-ADR facts (sequence contiguity, ID reuse,
// claim provenance) are validated at the corpus level.
func ParseV1(name string, data []byte) (ADR, error) {
	fm, body, err := parseV1Frontmatter(data)
	if err != nil {
		return ADR{}, err
	}
	if err := validateV1SectionOrder(string(body)); err != nil {
		return ADR{}, err
	}
	parsed := sections(string(body), len(data)-len(body))
	a := ADR{Format: CurrentStateV1, Status: fm.Status, Date: fm.Date, Sections: parsed.bodies, Filename: name}
	if decision, ok := parsed.ranges["Decision"]; ok {
		a.DecisionStart, a.DecisionEnd = decision.start, decision.end
	}
	for _, line := range strings.Split(string(body), "\n") {
		if title, ok := strings.CutPrefix(line, "# "); ok {
			a.Title = title
			break
		}
	}
	if m := FilenameRe.FindStringSubmatch(name); m != nil {
		a.Number = m[1]
	}
	if err := validateDecisionItems(a); err != nil {
		return ADR{}, err
	}
	ops, none, err := parseStateChanges(a.Sections["State changes"])
	if err != nil {
		return ADR{}, err
	}
	a.Operations, a.NoneState = ops, none
	hist, err := parseStatusHistory(a.Sections["Status history"])
	if err != nil {
		return ADR{}, err
	}
	a.History = hist
	if err := validateV1History(a); err != nil {
		return ADR{}, err
	}
	return a, nil
}

// FrozenContentEqual reports whether a pair preserves canonical ADR content.
// Proposed records remain editable; every later status freezes the five
// content-sha256 sections at their before-state digest.
func FrozenContentEqual(before, after ADR) bool {
	return before.Status == statusProposed || ContentDigest(before.Sections) == ContentDigest(after.Sections)
}

// HistoryTransitionValid reports whether a pair preserves append-only Status
// history: equal histories at the same status, or an exact before prefix plus
// one entry when the status follows a legal lifecycle edge.
func HistoryTransitionValid(before, after ADR) bool {
	if before.Status == after.Status {
		return slices.Equal(before.History, after.History)
	}
	return v1TransitionLegal(before.Status, after.Status) &&
		len(after.History) == len(before.History)+1 &&
		slices.Equal(before.History, after.History[:len(before.History)])
}

// ParseRecord parses one ADR, routing by its filename number against cutoff
// (the lock's adrFormatV1From). A number at or above cutoff must be
// current-state-v1; a lower number is legacy identity-only and must not declare
// the v1 format marker. A cutoff of zero or below treats every ADR as legacy,
// which is the pre-cutover state where no v1 ADR exists.
func ParseRecord(name string, data []byte, cutoff int) (ADR, error) {
	num := 0
	if m := FilenameRe.FindStringSubmatch(name); m != nil {
		num, _ = strconv.Atoi(m[1]) // the regex admits only four digits
	}
	if cutoff > 0 && num >= cutoff {
		return ParseV1(name, data)
	}
	a, _, err := ParseBytes(name, data)
	if err != nil {
		return ADR{}, err
	}
	if block, _, found := frontmatter.Split(data); found && bytes.Contains(block, []byte("format: "+V1FormatMarker)) {
		return ADR{}, fmt.Errorf("ADR-%s is below the format cutoff %d but declares %s", a.Number, cutoff, V1FormatMarker)
	}
	a.Format = Legacy
	return a, nil
}

// parseV1Frontmatter strictly decodes the closed frontmatter, rejecting any
// unknown key, an absent or wrong format marker, an unknown status, or a
// non-`YYYY-MM-DD` date.
func parseV1Frontmatter(data []byte) (v1Frontmatter, []byte, error) {
	block, body, found := frontmatter.Split(data)
	if !found {
		return v1Frontmatter{}, nil, errors.New("missing frontmatter")
	}
	dec := yaml.NewDecoder(bytes.NewReader(block))
	dec.KnownFields(true)
	var fm v1Frontmatter
	if err := dec.Decode(&fm); err != nil {
		return v1Frontmatter{}, nil, fmt.Errorf("frontmatter: %w", err)
	}
	if fm.Format != V1FormatMarker {
		return v1Frontmatter{}, nil, fmt.Errorf("frontmatter format must be %q, got %q", V1FormatMarker, fm.Format)
	}
	if !v1StatusKnown(fm.Status) {
		return v1Frontmatter{}, nil, fmt.Errorf("invalid status %q", fm.Status)
	}
	if _, err := time.Parse("2006-01-02", fm.Date); err != nil {
		return v1Frontmatter{}, nil, fmt.Errorf("invalid date %q", fm.Date)
	}
	return fm, body, nil
}

// validateV1SectionOrder requires the six sections to appear exactly once each,
// in the canonical order, with no extra or missing `## ` heading.
func validateV1SectionOrder(body string) error {
	got := v1Headings(body)
	if !slices.Equal(got, v1SectionOrder) {
		return fmt.Errorf("sections must be exactly %v in order, got %v", v1SectionOrder, got)
	}
	return nil
}

// v1Headings returns the ordered `## ` heading names of body, skipping headings
// inside fenced code blocks (mirrors sections()).
func v1Headings(body string) []string {
	var names []string
	var fence byte
	var fenceLen int
	for _, raw := range rangeLines(body) {
		line := strings.TrimSuffix(raw, "\n")
		if marker, n, ok := fenceMarker(line); ok {
			if fence == 0 {
				fence, fenceLen = marker, n
				continue
			}
			if marker == fence && n >= fenceLen && fenceCloser(line, n) {
				fence, fenceLen = 0, 0
			}
			continue
		}
		if fence != 0 {
			continue
		}
		if h, ok := strings.CutPrefix(line, "## "); ok {
			names = append(names, strings.TrimSpace(h))
		}
	}
	return names
}

// validateDecisionItems requires at least one column-zero numbered Decision
// item and strict 1..n sequencing (ADR-0135 item 2).
func validateDecisionItems(a ADR) error {
	items := a.DecisionItems()
	if len(items) == 0 {
		return errors.New("decision has no numbered items")
	}
	for i, n := range items {
		if n != i+1 {
			return fmt.Errorf("decision items must be sequential from 1; position %d is item %d", i+1, n)
		}
	}
	return nil
}

// validateV1History enforces the per-ADR Status-history semantics: a Proposed
// scaffold first entry, legal adjacent transitions, non-descending dates,
// per-status digest/sequence/rationale rules, and final-status agreement with
// the frontmatter (ADR-0135 items 6 and 7).
func validateV1History(a ADR) error {
	h := a.History
	digest := ContentDigest(a.Sections)
	first := h[0]
	if first.Status != statusProposed || first.Digest != "" || first.HasSequence || first.Rationale != "" {
		return errors.New("first Status history entry must be the `- <date>: Proposed` scaffold")
	}
	for i, e := range h {
		if i > 0 {
			if !v1TransitionLegal(h[i-1].Status, e.Status) {
				return fmt.Errorf("illegal Status history transition %s -> %s", h[i-1].Status, e.Status)
			}
			if e.Date < h[i-1].Date {
				return fmt.Errorf("status history dates must not descend: %s after %s", e.Date, h[i-1].Date)
			}
		}
		if err := validateHistoryEntry(a, e, digest); err != nil {
			return err
		}
	}
	if last := h[len(h)-1]; last.Status != a.Status {
		return fmt.Errorf("final Status history status %s does not match frontmatter status %s", last.Status, a.Status)
	}
	return nil
}

// validateHistoryEntry enforces one entry's digest, sequence, and rationale
// rules for its status.
func validateHistoryEntry(a ADR, e StatusEntry, digest string) error {
	switch e.Status {
	case statusProposed:
		return nil // the scaffold: no digest, sequence, or rationale (shape checked once, above)
	case statusAccepted:
		if e.HasSequence || e.Rationale != "" {
			return errors.New("accepted entry carries a sequence or rationale it must not")
		}
	case statusImplemented:
		if e.Rationale != "" {
			return errors.New("implemented entry must not carry a rationale")
		}
		if len(a.Operations) > 0 && !e.HasSequence {
			return errors.New("implemented ADR with operations must record a state-sequence")
		}
		if len(a.Operations) == 0 && e.HasSequence {
			return errors.New("implemented `None.` ADR must not record a state-sequence")
		}
	case statusAbandoned:
		if e.HasSequence {
			return errors.New("abandoned entry must not record a state-sequence")
		}
		if e.Rationale == "" {
			return errors.New("abandoned entry must end with a nonempty rationale")
		}
	}
	if e.Digest != digest {
		return fmt.Errorf("%s entry content-sha256 %q does not match the computed digest %q", e.Status, e.Digest, digest)
	}
	return nil
}
