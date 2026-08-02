package adr

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
	"gopkg.in/yaml.v3"
)

// Format markers are the exact governed `format:` frontmatter values.
const (
	V1FormatMarker = "current-state-v1"
	V2FormatMarker = "current-state-v2"
	V3FormatMarker = "current-state-v3"
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

// v3Frontmatter is the closed V3 frontmatter: the governed three keys plus the
// mandatory slug identity. V1 and V2 keep the narrower closed struct, so a
// `slug:` key in a pre-V3 record is still an unknown-field rejection.
type v3Frontmatter struct {
	Format string `yaml:"format"`
	Status string `yaml:"status"`
	Date   string `yaml:"date"`
	Slug   string `yaml:"slug"`
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

// ParseV3 parses and validates one current-state-v3 ADR in either identity
// form: numbered `NNNN-<slug>.md` with a `# ADR-NNNN: <Title>` heading, or
// pending `<slug>.md` with a `# ADR-<slug>: <Title>` heading. The record's body
// rules, history grammar, and digest coverage are V2's exactly; what V3 adds is
// the mandatory `slug:` key and its agreement with the filename and the heading
// (ADR-0202 items 1 to 3).
func ParseV3(name string, data []byte) (ADR, error) {
	a, err := parseGoverned(name, data, CurrentStateV3)
	if err != nil {
		return ADR{}, err
	}
	if a.Slug == "" {
		return ADR{}, errors.New("frontmatter slug is required for a current-state-v3 record")
	}
	if canonical, err := slugify(a.Slug); err != nil || canonical != a.Slug {
		return ADR{}, fmt.Errorf("frontmatter slug %q is not in slug form", a.Slug)
	}
	fileSlug := strings.TrimSuffix(name, ".md")
	identity := fileSlug
	if m := FilenameRe.FindStringSubmatch(name); m != nil {
		fileSlug, identity = strings.TrimSuffix(strings.TrimPrefix(name, m[1]+"-"), ".md"), m[1]
	}
	if fileSlug != a.Slug {
		return ADR{}, fmt.Errorf("filename %q does not carry the frontmatter slug %q", name, a.Slug)
	}
	prefix := "ADR-" + identity + ": "
	if !strings.HasPrefix(a.Title, prefix) || strings.TrimSpace(strings.TrimPrefix(a.Title, prefix)) == "" {
		return ADR{}, fmt.Errorf("heading must be `# %s<Title>`, got %q", prefix, "# "+a.Title)
	}
	return a, nil
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
	a := ADR{Format: format, Status: fm.Status, Date: fm.Date, Slug: fm.Slug, Sections: parsed.bodies, Filename: name}
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
	if a.HasV2Semantics() {
		a.History, err = parseV2History(a.Sections["Status history"], ops)
	} else {
		a.History, err = parseStatusHistory(a.Sections["Status history"])
	}
	if err != nil {
		return ADR{}, err
	}
	if a.HasV2Semantics() {
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
// A V1 record is editable only while Proposed; a V2 record is editable until it
// reaches a terminal status. Past its freeze point a record's five
// content-sha256 sections are locked at their before-state digest (ADR-0188).
func FrozenContentEqual(before, after ADR) bool {
	if before.HasV2Semantics() {
		return !terminalStatus(before.Status) || ContentDigest(before.Sections) == ContentDigest(after.Sections)
	}
	return before.Status == statusProposed || ContentDigest(before.Sections) == ContentDigest(after.Sections)
}

// HistoryTransitionValidAggregate reports whether a merge's Status history is a
// legal ordered append: the before history survives as an exact prefix of the
// after history. The fixed one-or-two-event shape HistoryTransitionValid
// enforces is deliberately not applied, because that shape encodes "one commit
// is one authoring step", which a merge is not (ADR-0182 item 7).
//
// Prefix preservation is the whole obligation, and that is not a weakening.
// validateV2History already replays every parsed record's complete history:
// each transition must be legal in the state its predecessors produce, every
// digest and sequence must be well-formed, and the terminal event must match the
// frontmatter status. The appended events are therefore already proven a legal
// ordered chain by parsing, and what a pair uniquely establishes is that nothing
// preceding them was rewritten.
func HistoryTransitionValidAggregate(before, after ADR) bool {
	return len(after.History) >= len(before.History) &&
		historiesEqual(before.History, after.History[:len(before.History)])
}

// HistoryTransitionValid reports whether a pair preserves append-only Status
// history: equal histories at the same status, one same-status Applied batch
// while Implementing, one same-status Amended event while Accepted or
// Implementing (ADR-0188), or an exact before prefix plus the required event
// shape when the status follows a legal lifecycle edge. One same-status
// Reapplied event is also legal while Implementing.
func HistoryTransitionValid(before, after ADR) bool {
	if !after.HasV2Semantics() {
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
		if len(added) == 0 {
			return true
		}
		if len(added) != 1 {
			return false
		}
		switch added[0].Kind {
		case HistoryApplied, HistoryReapplied:
			return before.Status == statusImplementing
		case HistoryAmended:
			return before.Status == statusAccepted || before.Status == statusImplementing
		default:
			return false
		}
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
		if before.Status == statusImplementing {
			return len(added) == 2 && added[0].Kind == HistoryApplied && added[1].Kind == HistoryStatus
		}
		return len(added) == 1 && added[0].Kind == HistoryStatus
	}
	return false // coverage-ignore: every legal V2 transition target is handled by the closed switch
}

// HistoriesEqual reports whether a pair's Status history is byte-identical.
// The numbering transition takes this rather than either append-tolerant
// variant: numbering touches no history event, so the pair's history must not
// move at all (ADR-0202 item 9).
func HistoriesEqual(before, after ADR) bool {
	return historiesEqual(before.History, after.History)
}

func historiesEqual(a, b []HistoryEvent) bool {
	return slices.EqualFunc(a, b, func(x, y HistoryEvent) bool {
		return x.Kind == y.Kind && x.Date == y.Date && x.Status == y.Status &&
			x.Digest == y.Digest && x.LegacySequence == y.LegacySequence && x.Rationale == y.Rationale &&
			slices.Equal(x.Operations, y.Operations)
	})
}

// formatActivation is one authored ADR format's exact marker and schema
// generation. Its order is the historical activation order and its final entry
// is the format new records use.
type formatActivation struct {
	format     Format
	marker     string
	generation int
}

var formatActivations = []formatActivation{
	{CurrentStateV1, V1FormatMarker, 14},
	{CurrentStateV2, V2FormatMarker, 15},
	{CurrentStateV3, V3FormatMarker, 29},
}

// CurrentFormat returns the format newly authored ADRs use.
func CurrentFormat() Format { return formatActivations[len(formatActivations)-1].format }

// CurrentFormatMarker returns the exact marker newly authored ADRs use.
func CurrentFormatMarker() string { return formatActivations[len(formatActivations)-1].marker }

// KnownFormatMarker reports whether marker names one registered governed ADR
// format.
func KnownFormatMarker(marker string) bool {
	for _, activation := range formatActivations {
		if marker == activation.marker {
			return true
		}
	}
	return false
}

// FormatAtGeneration returns the format active at generation. It reports false
// before the first governed format activation.
func FormatAtGeneration(generation int) (Format, bool) {
	var format Format
	found := false
	for _, activation := range formatActivations {
		if activation.generation > generation {
			break
		}
		format, found = activation.format, true
	}
	return format, found
}

// ParseRecord routes an ADR by its authored format marker. Marker absence is
// the sole legacy route; invalid frontmatter and every nonempty unregistered
// marker are refusals rather than legacy fallbacks.
func ParseRecord(name string, data []byte) (ADR, error) {
	marker, declared, err := declaredFormatMarker(data)
	if err != nil {
		return ADR{}, err
	}
	if !declared {
		if FilenameRe.MatchString(name) {
			a, _, err := ParseBytes(name, data)
			if err != nil {
				return ADR{}, err
			}
			a.Format = Legacy
			return a, nil
		}
		return ADR{}, ErrNotADRRecord(name)
	}
	if marker == "" {
		return ADR{}, errors.New("empty governed ADR format marker")
	}
	switch marker {
	case V1FormatMarker:
		if !FilenameRe.MatchString(name) {
			return ADR{}, ErrNotADRRecord(name)
		}
		return ParseV1(name, data)
	case V2FormatMarker:
		if !FilenameRe.MatchString(name) {
			return ADR{}, ErrNotADRRecord(name)
		}
		return ParseV2(name, data)
	case V3FormatMarker:
		return ParseV3(name, data)
	default:
		return ADR{}, fmt.Errorf("unknown governed ADR format marker %q", marker)
	}
}

// declaredFormatMarker reads the routing marker while distinguishing a missing
// key (legacy) from an explicitly empty key (invalid). The initial strict
// frontmatter parse detects malformed YAML and duplicate keys before routing.
func declaredFormatMarker(data []byte) (string, bool, error) {
	block, _, found := frontmatter.Split(data)
	if !found {
		firstLine := data
		if newline := bytes.IndexByte(firstLine, '\n'); newline >= 0 {
			firstLine = firstLine[:newline]
		}
		if strings.TrimRight(string(firstLine), "\r") == "---" {
			return "", false, errors.New("unterminated frontmatter")
		}
		return "", false, nil
	}
	var document yaml.Node
	if err := yaml.Unmarshal(block, &document); err != nil {
		return "", false, fmt.Errorf("frontmatter: %w", err)
	}
	if len(document.Content) == 0 || document.Content[0].Kind != yaml.MappingNode {
		return "", false, errors.New("frontmatter must be a mapping")
	}
	mapping := document.Content[0]
	for i := 0; i < len(mapping.Content); i += 2 {
		key, value := mapping.Content[i], mapping.Content[i+1]
		if key.Value != "format" {
			continue
		}
		if value.Kind != yaml.ScalarNode {
			return "", true, errors.New("malformed governed ADR format marker")
		}
		return value.Value, true, nil
	}
	return "", false, nil
}

// parseV1Frontmatter strictly decodes the closed frontmatter, rejecting any
// unknown key, an absent or wrong format marker, an unknown status, or a
// non-`YYYY-MM-DD` date.
func parseGovernedFrontmatter(data []byte, format Format) (v3Frontmatter, []byte, error) {
	block, body, found := frontmatter.Split(data)
	if !found {
		return v3Frontmatter{}, nil, errors.New("missing frontmatter")
	}
	dec := yaml.NewDecoder(bytes.NewReader(block))
	dec.KnownFields(true)
	var fm v3Frontmatter
	if format == CurrentStateV3 {
		if err := dec.Decode(&fm); err != nil {
			return v3Frontmatter{}, nil, fmt.Errorf("frontmatter: %w", err)
		}
	} else {
		var narrow governedFrontmatter
		if err := dec.Decode(&narrow); err != nil {
			return v3Frontmatter{}, nil, fmt.Errorf("frontmatter: %w", err)
		}
		fm = v3Frontmatter{Format: narrow.Format, Status: narrow.Status, Date: narrow.Date}
	}
	marker := V1FormatMarker
	known := v1StatusKnown
	if format != CurrentStateV1 {
		marker = FormatMarker(format)
		known = func(status string) bool { return v2Statuses[status] }
	}
	if fm.Format != marker {
		return v3Frontmatter{}, nil, fmt.Errorf("frontmatter format must be %q, got %q", marker, fm.Format)
	}
	if !known(fm.Status) {
		return v3Frontmatter{}, nil, fmt.Errorf("invalid status %q", fm.Status)
	}
	if _, err := time.Parse("2006-01-02", fm.Date); err != nil {
		return v3Frontmatter{}, nil, fmt.Errorf("invalid date %q", fm.Date)
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
// per-status digest/rationale rules, and final-status agreement with the
// frontmatter (ADR-0135 items 6 and 7).
func validateV1History(a ADR) error {
	h := a.History
	digest := ContentDigest(a.Sections)
	first := h[0]
	if first.Status != statusProposed || first.Digest != "" || first.Rationale != "" {
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
		if err := validateHistoryEntry(e, digest); err != nil {
			return err
		}
	}
	if last := h[len(h)-1]; last.Status != a.Status {
		return fmt.Errorf("final Status history status %s does not match frontmatter status %s", last.Status, a.Status)
	}
	return nil
}

// validateV2History enforces one governed record's event stream: scaffold,
// transitions, dates, stamps, and application cardinality.
func validateV2History(a ADR) error {
	h := a.History
	digest := ContentDigest(a.Sections)
	first := h[0]
	if first.Kind != HistoryStatus || first.Status != statusProposed || first.Digest != "" || first.Rationale != "" {
		return errors.New("first Status history entry must be the `- <date>: Proposed` scaffold")
	}
	applied := map[Operation]bool{}
	current := ""
	explicit := false
	lastStatus := ""
	lastStamp := ""
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
		if event.Kind == HistoryReapplied {
			if current != statusImplementing {
				return errors.New("reapplied event is allowed only while Implementing")
			}
			for _, op := range event.Operations {
				if !applied[op] {
					return fmt.Errorf("reapplied operation %s `%s` requires an earlier Applied occurrence", op.Verb, op.ID)
				}
				if op.Verb == OpRemove {
					return fmt.Errorf("reapplied operation %s `%s` is invalid; only add or update may be reapplied", op.Verb, op.ID)
				}
			}
			continue
		}
		if event.Kind == HistoryAmended {
			if current != statusAccepted && current != statusImplementing {
				return errors.New("amended event is allowed only while Accepted or Implementing")
			}
			if event.Digest == lastStamp {
				return errors.New("amended event must record a digest different from the preceding stamp")
			}
			lastStamp = event.Digest
			continue
		}
		if event.Kind != HistoryStatus { // coverage-ignore: the parser constructs only the four closed event kinds
			return errors.New("Status history contains an unknown event kind")
		}
		if i > 0 && !v2TransitionLegal(current, event.Status) {
			return fmt.Errorf("illegal Status history transition %s -> %s", current, event.Status)
		}
		if err := validateV2StatusEntry(event); err != nil {
			return err
		}
		if event.Status != statusProposed {
			if lastStamp == "" {
				lastStamp = event.Digest
			} else if event.Digest != lastStamp {
				return fmt.Errorf("%s entry content-sha256 %q does not repeat the preceding stamp %q", event.Status, event.Digest, lastStamp)
			}
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
			if i == 0 || h[i-1].Kind != HistoryApplied {
				return errors.New("explicit Implemented transition requires a final Applied event immediately before it")
			}
		}
	}
	if lastStatus != a.Status {
		return fmt.Errorf("latest Status history status %s does not match frontmatter status %s", lastStatus, a.Status)
	}
	if lastStamp != "" && lastStamp != digest {
		return fmt.Errorf("latest stamped content-sha256 %q does not match the computed digest %q", lastStamp, digest)
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

func validateV2StatusEntry(e HistoryEvent) error {
	switch e.Status {
	case statusProposed:
		return nil // the first-entry scaffold check owns Proposed metadata
	case statusAccepted, statusImplementing:
		if e.Rationale != "" {
			return fmt.Errorf("%s entry carries a rationale it must not", e.Status)
		}
	case statusImplemented:
		if e.Rationale != "" {
			return errors.New("implemented entry must not carry a rationale")
		}
	case statusAbandoned:
		if e.Rationale == "" {
			return errors.New("abandoned entry must end with a nonempty rationale")
		}
	}
	if e.Digest == "" {
		return fmt.Errorf("%s entry must carry a content-sha256", e.Status)
	}
	return nil
}

func validateHistoryEntry(e HistoryEvent, digest string) error {
	switch e.Status {
	case statusProposed:
		return nil // the scaffold: no digest or rationale (shape checked once, above)
	case statusAccepted:
		if e.Rationale != "" {
			return errors.New("accepted entry carries a rationale it must not")
		}
	case statusImplemented:
		if e.Rationale != "" {
			return errors.New("implemented entry must not carry a rationale")
		}
	case statusAbandoned:
		if e.Rationale == "" {
			return errors.New("abandoned entry must end with a nonempty rationale")
		}
	}
	if e.Digest != digest {
		return fmt.Errorf("%s entry content-sha256 %q does not match the computed digest %q", e.Status, e.Digest, digest)
	}
	return nil
}
