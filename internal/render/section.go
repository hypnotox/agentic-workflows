package render

import "regexp"

type Segment struct {
	IsSection bool
	Name      string
	Text      string
}

var sectionRE = regexp.MustCompile(`(?s)<!-- awf:section (\S+) -->\n(.*?)\n<!-- awf:end -->`)

// ParseSections splits src into ordered literal and section segments.
// Marker lines are consumed; a section segment's Text is the inner body.
func ParseSections(src string) []Segment {
	var segs []Segment
	idx := sectionRE.FindAllStringSubmatchIndex(src, -1)
	last := 0
	for _, m := range idx {
		// m[0]:m[1] whole match; m[2]:m[3] name; m[4]:m[5] body
		if m[0] > last {
			segs = append(segs, Segment{Text: src[last:m[0]]})
		}
		segs = append(segs, Segment{
			IsSection: true,
			Name:      src[m[2]:m[3]],
			Text:      src[m[4]:m[5]],
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
