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

// Format markers are the exact governed `format:` frontmatter values.
const (
	V1FormatMarker = "current-state-v1"
	V2FormatMarker = "current-state-v2"
)

// v1SectionOrder is the required exact, ordered section set of a
// current-state-v1 ADR (ADR-0135 item 2).
var v1SectionOrder = []string{"Context", "Decision", "State changes", "Consequences", "Alternatives Considered", "Status history"}

// governedFrontmatter is the closed governed frontmatter: exactly format,
// status, and date. Number and title come from the filename and heading.
type governedFrontmatter struct {
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
	return parseGoverned(name, data, CurrentStateV1)
}

// ParseV2 parses and validates one current-state-v2 ADR.
func ParseV2(name string, data []byte) (ADR, error) {
	return parseGoverned(name, data, CurrentStateV2)
}

func parseGoverned(name string, data []byte, format Format) (ADR, error) {
	fm, body, err := parseGovernedFrontmatter(data, format)
	if err != nil {
		return ADR{}, err
	}
	if err := validateV1SectionOrder(string(body)); err != nil {
		return ADR{}, err
	}
	parsed := sections(string(body), len(data)-len(body))
	a := ADR{Format: format, Status: fm.Status, Date: fm.Date, Sections: parsed.bodies, Filename: name}
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
	if format == CurrentStateV2 {
		a.History, err = parseV2History(a.Sections["Status history"], ops)
	} else {
		a.History, err = parseStatusHistory(a.Sections["Status history"])
	}
	if err != nil {
		return ADR{}, err
	}
	if format == CurrentStateV2 {
		err = validateV2History(a)
	} else {
		err = validateV1History(a)
	}
	if err != nil {
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
	if after.Format != CurrentStateV2 {
		if before.Status == after.Status {
			return historiesEqual(before.History, after.History)
		}
		return v1TransitionLegal(before.Status, after.Status) &&
			len(after.History) == len(before.History)+1 &&
			historiesEqual(before.History, after.History[:len(before.History)])
	}
	if len(after.History) < len(before.History) || !historiesEqual(before.History, after.History[:len(before.History)]) {
		return false
	}
	added := after.History[len(before.History):]
	if before.Status == after.Status {
		return before.Status == statusImplementing && len(added) == 1 && added[0].Kind == HistoryApplied
	}
	if !v2TransitionLegal(before.Status, after.Status) {
		return false
	}
	switch after.Status {
	case statusAccepted, statusAbandoned:
		return len(added) == 1 && added[0].Kind == HistoryStatus
	case statusImplementing:
		return len(added) == 2 && added[0].Kind == HistoryStatus && added[1].Kind == HistoryApplied
	case statusImplemented:
		if len(added) == 1 {
			return added[0].Kind == HistoryStatus
		}
		return len(added) == 2 && added[0].Kind == HistoryApplied && added[1].Kind == HistoryStatus
	}
	return false // coverage-ignore: every legal V2 transition target is handled by the closed switch
}

func historiesEqual(a, b []HistoryEvent) bool {
	return slices.EqualFunc(a, b, func(x, y HistoryEvent) bool {
		return x.Kind == y.Kind && x.Date == y.Date && x.Status == y.Status &&
			x.Digest == y.Digest && x.Sequence == y.Sequence &&
			x.HasSequence == y.HasSequence && x.Rationale == y.Rationale &&
			slices.Equal(x.Operations, y.Operations)
	})
}

// ParseRecord routes by the V1 cutoff and optional V2 cutoff. Missing V2
// keeps every governed record in the V1 region.
func ParseRecord(name string, data []byte, cutoff int, v2Cutoff ...int) (ADR, error) {
	num := 0
	if m := FilenameRe.FindStringSubmatch(name); m != nil {
		num, _ = strconv.Atoi(m[1]) // the regex admits only four digits
	}
	v2 := 0
	if len(v2Cutoff) > 0 {
		v2 = v2Cutoff[0]
	}
	if v2 > 0 && num >= v2 {
		return ParseV2(name, data)
	}
	if cutoff > 0 && num >= cutoff {
		return ParseV1(name, data)
	}
	a, _, err := ParseBytes(name, data)
	if err != nil {
		return ADR{}, err
	}
	if block, _, found := frontmatter.Split(data); found {
		for _, marker := range []string{V1FormatMarker, V2FormatMarker} {
			if bytes.Contains(block, []byte("format: "+marker)) {
				return ADR{}, fmt.Errorf("ADR-%s is below the format cutoff %d but declares %s", a.Number, cutoff, marker)
			}
		}
	}
	a.Format = Legacy
	return a, nil
}

// parseV1Frontmatter strictly decodes the closed frontmatter, rejecting any
// unknown key, an absent or wrong format marker, an unknown status, or a
// non-`YYYY-MM-DD` date.
func parseGovernedFrontmatter(data []byte, format Format) (governedFrontmatter, []byte, error) {
	block, body, found := frontmatter.Split(data)
	if !found {
		return governedFrontmatter{}, nil, errors.New("missing frontmatter")
	}
	dec := yaml.NewDecoder(bytes.NewReader(block))
	dec.KnownFields(true)
	var fm governedFrontmatter
	if err := dec.Decode(&fm); err != nil {
		return governedFrontmatter{}, nil, fmt.Errorf("frontmatter: %w", err)
	}
	marker := V1FormatMarker
	known := v1StatusKnown
	if format == CurrentStateV2 {
		marker = V2FormatMarker
		known = func(status string) bool { return v2Statuses[status] }
	}
	if fm.Format != marker {
		return governedFrontmatter{}, nil, fmt.Errorf("frontmatter format must be %q, got %q", marker, fm.Format)
	}
	if !known(fm.Status) {
		return governedFrontmatter{}, nil, fmt.Errorf("invalid status %q", fm.Status)
	}
	if _, err := time.Parse("2006-01-02", fm.Date); err != nil {
		return governedFrontmatter{}, nil, fmt.Errorf("invalid date %q", fm.Date)
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
func validateV2History(a ADR) error {
	h := a.History
	digest := ContentDigest(a.Sections)
	first := h[0]
	if first.Kind != HistoryStatus || first.Status != statusProposed || first.Digest != "" || first.HasSequence || first.Rationale != "" {
		return errors.New("first Status history entry must be the `- <date>: Proposed` scaffold")
	}
	applied := map[Operation]bool{}
	current := ""
	explicit := false
	lastStatus := ""
	for i, event := range h {
		if i > 0 && event.Date < h[i-1].Date {
			return fmt.Errorf("status history dates must not descend: %s after %s", event.Date, h[i-1].Date)
		}
		if event.Kind == HistoryApplied {
			explicit = true
			if current != statusImplementing {
				return errors.New("applied event is allowed only while Implementing")
			}
			for _, op := range event.Operations {
				if applied[op] {
					return fmt.Errorf("applied operation %s `%s` was already applied", op.Verb, op.ID)
				}
				applied[op] = true
			}
			continue
		}
		if event.Kind != HistoryStatus { // coverage-ignore: the parser constructs only the two closed event kinds
			return errors.New("Status history contains an unknown event kind")
		}
		if i > 0 && !v2TransitionLegal(current, event.Status) {
			return fmt.Errorf("illegal Status history transition %s -> %s", current, event.Status)
		}
		if err := validateV2StatusEntry(event, digest); err != nil {
			return err
		}
		current, lastStatus = event.Status, event.Status
		if event.Status == statusImplementing {
			if len(a.Operations) < 2 {
				return errors.New("implementing requires at least two declared operations")
			}
			if i+1 >= len(h) || h[i+1].Kind != HistoryApplied {
				return errors.New("implementing status event must be followed by the first Applied event")
			}
		}
		if event.Status == statusImplemented && explicit {
			if event.HasSequence {
				return errors.New("V2 ADR cannot mix explicit Applied events with implicit terminal sequencing")
			}
			if i == 0 || h[i-1].Kind != HistoryApplied { // coverage-ignore: legal lifecycle ordering leaves no intervening status after Implementing
				return errors.New("explicit Implemented transition requires a final Applied event immediately before it")
			}
		}
	}
	if lastStatus != a.Status {
		return fmt.Errorf("latest Status history status %s does not match frontmatter status %s", lastStatus, a.Status)
	}
	if !explicit && a.Status == statusImplemented {
		terminal := h[len(h)-1]
		if len(a.Operations) > 0 && !terminal.HasSequence {
			return errors.New("implemented ADR with operations must record a state-sequence")
		}
		if len(a.Operations) == 0 && terminal.HasSequence {
			return errors.New("implemented `None.` ADR must not record a state-sequence")
		}
	}
	switch a.Status {
	case statusImplementing:
		if len(applied) == 0 || len(applied) >= len(a.Operations) {
			return errors.New("implementing requires at least one applied and one remaining operation")
		}
	case statusImplemented:
		if explicit && len(applied) != len(a.Operations) {
			return errors.New("implemented requires every declared operation to be applied")
		}
	case statusAbandoned:
		if explicit && len(applied) >= len(a.Operations) {
			return errors.New("abandoned explicit history requires at least one canceled operation")
		}
	}
	return nil
}

func validateV2StatusEntry(e HistoryEvent, digest string) error {
	switch e.Status {
	case statusProposed:
		return nil // the first-entry scaffold check owns Proposed metadata
	case statusAccepted, statusImplementing:
		if e.HasSequence || e.Rationale != "" {
			return fmt.Errorf("%s entry carries a sequence or rationale it must not", e.Status)
		}
	case statusImplemented:
		if e.Rationale != "" {
			return errors.New("implemented entry must not carry a rationale")
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

func validateHistoryEntry(a ADR, e HistoryEvent, digest string) error {
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
