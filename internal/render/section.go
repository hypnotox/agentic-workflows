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
	// Heading is the optional awf-owned Markdown ATX heading at the section's
	// leading structural position. It is deliberately separate from the body.
	Heading string
	// Stub marks a section whose template default is a must-replace authoring
	// prompt, declared by the `stub` marker attribute (ADR-0070).
	Stub bool
	// InPlace marks a section declared by the `inplace` marker attribute
	// (ADR-0100); its body is read back from the rendered output rather than a
	// convention part, and preserved across syncs. Mutually exclusive with Stub.
	InPlace bool
}

// The body capture is non-greedy; the optional `\n?` before the closing marker
// absorbs the body's trailing newline so a normal body excludes it, while an
// empty-body block (markers on consecutive lines) captures "". The optional
// ` stub` (ADR-0070) and ` inplace` (ADR-0100) attributes are the only legal
// marker attributes and are mutually exclusive; any other token makes the marker
// unparseable, which CheckResidualMarkers turns into a hard render error instead
// of a silent leak.
var sectionRE = regexp.MustCompile(`(?s)<!-- awf:section (\S+)( stub| inplace)? -->\n(.*?)\n?<!-- awf:end -->`)

// ParseSections splits src into ordered literal and section segments.
// Marker lines are consumed. When markdown is selected, a whole ATX heading on
// the first body line is structural and is stored separately from Text. The
// optional argument preserves the historical Markdown default for direct users.
func ParseSections(src string, markdown ...bool) []Segment {
	isMarkdown := true
	if len(markdown) > 0 {
		isMarkdown = markdown[0]
	}
	var segs []Segment
	idx := sectionRE.FindAllStringSubmatchIndex(src, -1)
	last := 0
	for _, m := range idx {
		// m[0]:m[1] whole match; m[2]:m[3] name; m[4]:m[5] attribute; m[6]:m[7] body
		if m[0] > last {
			segs = append(segs, Segment{Text: src[last:m[0]]})
		}
		attr := ""
		if m[4] >= 0 {
			attr = strings.TrimSpace(src[m[4]:m[5]])
		}
		body := src[m[6]:m[7]]
		heading := ""
		if isMarkdown {
			if end := strings.IndexByte(body, '\n'); end >= 0 {
				if atxHeading(body[:end]) {
					heading, body = body[:end], body[end+1:]
				}
			} else if atxHeading(body) {
				heading, body = body, ""
			}
		}
		segs = append(segs, Segment{
			IsSection: true,
			Name:      src[m[2]:m[3]],
			Stub:      attr == "stub",
			InPlace:   attr == "inplace",
			Heading:   heading,
			Text:      body,
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

// atxHeading recognizes only a complete ATX heading line. Hash-prefixed
// comments in non-Markdown artifacts remain ordinary body text.
func atxHeading(line string) bool {
	if len(line) < 2 || line[0] != '#' {
		return false
	}
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	return i <= 6 && i < len(line) && (line[i] == ' ' || line[i] == '\t')
}

// stubMarkerLine is the whole-line marker a convention part carries to declare
// itself unauthored starter content (ADR-0070). Whole-line matching means prose
// that quotes the marker inline never counts.
const stubMarkerLine = "<!-- awf:stub -->"

// HasStubMarker reports whether a part body contains a line that is exactly the
// awf:stub marker (modulo surrounding whitespace). Detection never mutates the
// body - parts render byte-for-byte verbatim, marker included (ADR-0034, ADR-0070).
// touches-state: rendering/render-engine:stub-part-verbatim - stub-marker detection without mutation; proof in section_test.go
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
// begins with a marker-shaped `awf:section`/`awf:end` comment opener - the
// ADR-0083 whole-line detection behind the part-marker advisory. The prefix
// anchor covers the exact closed marker, an unclosed opener, and a marker with
// trailing text: none has a legitimate quoter, since prose quoting the form
// always precedes it on the line. Inline quoting never fires; the awf:stub
// part marker is out of scope by construction (the pattern names only
// section/end). Callers exclude fenced code before the scan.
// touches-state: rendering/render-engine:part-marker-advisory - whole-line section-marker residue detection; proof in section_test.go
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
// never a bare-identifier scan - a section default may legally quote the bare
// token in prose (ADR-0070 Decision 5).
var residualMarkerRE = regexp.MustCompile(`<!--\s*awf:(section|end)\b`)

// CheckResidualMarkers hard-errors when an assembled skeleton still contains a
// marker-shaped awf:section/awf:end token - a malformed marker (unknown
// attribute, missing name) that ParseSections could not consume and that would
// otherwise leak verbatim into rendered output. It runs pre-Execute: part bodies
// are NUL sentinels and data is uninterpolated, so parts and data that quote the
// full comment form stay out of scope.
// touches-state: rendering/render-engine:no-residual-section-marker - hard error on surviving marker residue; proof in section_test.go
func CheckResidualMarkers(assembled string) error {
	if m := residualMarkerRE.FindString(assembled); m != "" {
		return fmt.Errorf("assembled template still contains a section marker (%q): malformed awf:section/awf:end marker", m)
	}
	return nil
}
