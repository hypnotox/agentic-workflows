package adr

import (
	"regexp"
	"strconv"
	"strings"
)

// Citation is one citation of another ADR's anchor inside a Decision item
// (ADR-0131 item 2). HasVerb records whether an override verb occurs in the
// same Decision item, which is what separates a mention from a claim the
// record is expected to encode.
//
// Extraction lives here rather than in internal/project because ADR-0130's
// corpus-owns-field-reads confines Sections reads to this package: consumers
// take parsed citations and never touch raw section text.
type Citation struct {
	Carrier     string // citing ADR's number
	CarrierItem int    // the citing ADR's own Decision item; 0 before the first
	Anchor      Anchor // the cited anchor
	HasVerb     bool   // an override verb occurs in the same Decision item
}

var (
	// adrRef matches the ADR reference an item citation hangs off, including
	// the two wrappers the corpus writes it in (ADR-0131 item 2): the
	// possessive "ADR-0022's Decision item 4" and the markdown link
	// "[ADR-0040](0040-....md) Decision item 1". A bare prefix match sees
	// neither. The possessive was a spec-to-code gap - this ADR's Context
	// measured the corpus with `('s)?` while the implementation dropped it -
	// and the link shape carries ADR-0047 Decision 1, the one claim the
	// Context records the enumeration as having missed.
	adrRef = `\[?ADR-([0-9]{4})(?:\]\([^)\s]*\))?(?:'s)?`
	// citeItemRe covers three of the four item-citation shapes, longest
	// alternative first so "Decision item 4" does not match as "Decision 4".
	// Recognizing only the first shape would see 59 of the corpus's 186
	// citations (ADR-0131 item 2).
	citeItemRe = regexp.MustCompile(adrRef + `\s+(?:Decision\s+item|Decision|item)\s+([0-9]+)`)
	// citeDRe is the fourth shape, "ADR-NNNN DN".
	citeDRe = regexp.MustCompile(adrRef + `\s+D([0-9]+)`)
	// citeSlugRe matches both slug spellings (ADR-0131 item 2). It reads raw
	// text, not masked: a slug citation is always written inside a code span,
	// so masking would erase every one of them. Section scoping is what makes
	// the spelling unambiguous - the same string inside `## Invariants` is a
	// declaration parsed by declRe, and only the Decision section reaches here.
	citeSlugRe = regexp.MustCompile("`(?:inv|invariant):\\s*([a-z0-9-]+)`")
	// adrRefRe finds the ADR reference a slug citation hangs off.
	adrRefRe = regexp.MustCompile(`ADR-([0-9]{4})`)
	// verbRe is the enumerated override-verb surface list of ADR-0131 item 2,
	// whole-word matched. Enumerated, never stem-plus-suffix: that rule elides
	// the e in every e-final participle (`replacing`, not `replaceing`) and
	// misses nominalizations entirely, and the corpus leans on exactly those
	// forms - 55 occurrences of `supersedence` alone. Enumeration also keeps
	// `narrower`, `narrowest` and `overridable` out, which a stem match admits.
	verbRe = regexp.MustCompile(`\b(?:` + strings.Join([]string{
		"supersede", "supersedes", "superseded", "superseding", "supersedence",
		"override", "overrides", "overrode", "overridden", "overriding",
		"replace", "replaces", "replaced", "replacing", "replacement",
		"reverse", "reverses", "reversed", "reversing", "reversal",
		"amend", "amends", "amended", "amending", "amendment",
		"revise", "revises", "revised", "revising", "revision",
		"narrow", "narrows", "narrowed", "narrowing",
		"generalize", "generalizes", "generalized", "generalizing", "generalization",
	}, "|") + `)\b`)
)

// maskCodeSpans blanks the contents of inline code spans, preserving length so
// that every offset taken from the masked text still addresses the original.
// ADR-0131 item 5 exempts citations and verbs written inside a code span,
// which is what lets an ADR discuss the grammar without tripping the check.
//
// This is deliberately not shared with parseRefs. Relation tokens are
// themselves always written inside backticks, so masking token recognition
// would delete every token in the corpus - and the resulting output would look
// like a clean corpus rather than a bug.
func maskCodeSpans(s string) string {
	out := []byte(s)
	inSpan := false
	for i := 0; i < len(out); i++ {
		switch {
		case out[i] == '\n':
			// A code span does not cross a line, so a newline closes an
			// unterminated one rather than masking the rest of the document.
			inSpan = false
		case out[i] == '`':
			inSpan = !inSpan
			out[i] = ' '
		case inSpan:
			out[i] = ' '
		}
	}
	return string(out)
}

// Citations returns every anchor citation in the named ADR's Decision section,
// in no guaranteed order beyond shape grouping. An absent ADR, or one with no
// Decision section, cites nothing.
func (c Corpus) Citations(num string) []Citation {
	a, ok := c.byNum[num]
	if !ok {
		return nil
	}
	decision := a.Sections["Decision"]
	spans := decisionItemSpans(decision)
	masked := maskCodeSpans(decision)

	// A verb anywhere in a Decision item arms every citation in that item.
	verbItems := map[int]bool{}
	for _, m := range verbRe.FindAllStringIndex(masked, -1) {
		verbItems[itemAt(spans, m[0])] = true
	}

	var out []Citation
	add := func(off int, anchor Anchor) {
		item := itemAt(spans, off)
		out = append(out, Citation{Carrier: num, CarrierItem: item, Anchor: anchor, HasVerb: verbItems[item]})
	}
	for _, re := range []*regexp.Regexp{citeItemRe, citeDRe} {
		for _, m := range re.FindAllStringSubmatchIndex(masked, -1) {
			n, _ := strconv.Atoi(masked[m[4]:m[5]]) // the regex admits only digits
			add(m[0], Anchor{ADR: masked[m[2]:m[3]], Item: n})
		}
	}
	for _, m := range citeSlugRe.FindAllStringSubmatchIndex(decision, -1) {
		// The slug names no ADR, so it hangs off the nearest preceding ADR
		// reference in its own Decision item ("ADR-0009 ... and `inv: foo`").
		target, ok := precedingADR(masked, spans, m[0])
		if !ok {
			continue
		}
		add(m[0], Anchor{ADR: target, Slug: decision[m[2]:m[3]]})
	}
	return out
}

// precedingADR returns the last ADR reference at or before off that sits in the
// same Decision item. A slug citation with no ADR reference to hang off names
// no anchor, so it is not a citation of another ADR at all.
func precedingADR(masked string, spans [][2]int, off int) (string, bool) {
	item := itemAt(spans, off)
	num, found := "", false
	for _, m := range adrRefRe.FindAllStringSubmatchIndex(masked, -1) {
		if m[0] > off {
			break
		}
		if itemAt(spans, m[0]) == item {
			num, found = masked[m[2]:m[3]], true
		}
	}
	return num, found
}
