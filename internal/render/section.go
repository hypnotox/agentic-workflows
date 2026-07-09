package render

import (
	"fmt"
	"regexp"
	"strings"
)

type Segment struct {
	IsSection bool
	Name      string
	Text      string
	// Stub marks a section whose template default is a must-replace authoring
	// prompt, declared by the `stub` marker attribute (ADR-0070).
	Stub bool
}

// The body capture is non-greedy; the optional `\n?` before the closing marker
// absorbs the body's trailing newline so a normal body excludes it, while an
// empty-body block (markers on consecutive lines) captures "". The optional
// ` stub` attribute (ADR-0070) is the only legal marker attribute; any other
// token makes the marker unparseable, which CheckResidualMarkers turns into a
// hard render error instead of a silent leak.
var sectionRE = regexp.MustCompile(`(?s)<!-- awf:section (\S+)( stub)? -->\n(.*?)\n?<!-- awf:end -->`)

// ParseSections splits src into ordered literal and section segments.
// Marker lines are consumed; a section segment's Text is the inner body.
func ParseSections(src string) []Segment {
	var segs []Segment
	idx := sectionRE.FindAllStringSubmatchIndex(src, -1)
	last := 0
	for _, m := range idx {
		// m[0]:m[1] whole match; m[2]:m[3] name; m[4]:m[5] stub attribute; m[6]:m[7] body
		if m[0] > last {
			segs = append(segs, Segment{Text: src[last:m[0]]})
		}
		segs = append(segs, Segment{
			IsSection: true,
			Name:      src[m[2]:m[3]],
			Stub:      m[4] >= 0,
			Text:      src[m[6]:m[7]],
		})
		last = m[1]
	}
	if last < len(src) {
		segs = append(segs, Segment{Text: src[last:]})
	}
	if len(segs) == 0 {
		segs = append(segs, Segment{Text: src})
	}
	return segs
}

// stubMarkerLine is the whole-line marker a convention part carries to declare
// itself unauthored starter content (ADR-0070). Whole-line matching means prose
// that quotes the marker inline never counts.
const stubMarkerLine = "<!-- awf:stub -->"

// HasStubMarker reports whether a part body contains a line that is exactly the
// awf:stub marker (modulo surrounding whitespace). Detection never mutates the
// body — parts render byte-for-byte verbatim, marker included (ADR-0034, ADR-0070).
// invariant: stub-part-verbatim
func HasStubMarker(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == stubMarkerLine {
			return true
		}
	}
	return false
}

// markerLineRE anchors HasMarkerLine's detection: a trimmed line beginning with
// a marker-shaped awf:section/awf:end comment opener.
var markerLineRE = regexp.MustCompile(`^<!--\s*awf:(section|end)\b`)

// HasMarkerLine reports whether body contains a line that, after trimming,
// begins with a marker-shaped `awf:section`/`awf:end` comment opener — the
// ADR-0083 whole-line detection behind the part-marker advisory. The prefix
// anchor covers the exact closed marker, an unclosed opener, and a marker with
// trailing text: none has a legitimate quoter, since prose quoting the form
// always precedes it on the line. Inline quoting never fires; the awf:stub
// part marker is out of scope by construction (the pattern names only
// section/end). Callers exclude fenced code before the scan.
// invariant: part-marker-advisory
func HasMarkerLine(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		if markerLineRE.MatchString(strings.TrimSpace(line)) {
			return true
		}
	}
	return false
}

// residualMarkerRE matches a marker-shaped comment opener that survived section
// assembly: `<!--` + optional whitespace + awf:section/awf:end. Comment-anchored,
// never a bare-identifier scan — a section default may legally quote the bare
// token in prose (ADR-0070 Decision 5).
var residualMarkerRE = regexp.MustCompile(`<!--\s*awf:(section|end)\b`)

// CheckResidualMarkers hard-errors when an assembled skeleton still contains a
// marker-shaped awf:section/awf:end token — a malformed marker (unknown
// attribute, missing name) that ParseSections could not consume and that would
// otherwise leak verbatim into rendered output. It runs pre-Execute: part bodies
// are NUL sentinels and data is uninterpolated, so parts and data that quote the
// full comment form stay out of scope.
// invariant: no-residual-section-marker
func CheckResidualMarkers(assembled string) error {
	if m := residualMarkerRE.FindString(assembled); m != "" {
		return fmt.Errorf("assembled template still contains a section marker (%q) — malformed awf:section/awf:end marker", m)
	}
	return nil
}
