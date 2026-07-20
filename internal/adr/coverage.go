package adr

import "fmt"

// Anchor is one addressable decision unit: a Decision item or a declared
// invariant slug on a specific ADR. Exactly one of Item/Slug is set.
type Anchor struct {
	ADR  string // 4-digit target ADR number
	Item int    // Decision item number; 0 for a slug anchor
	Slug string // invariant slug; "" for an item anchor
}

// String renders the anchor in the token grammar's own shape, ADR-NNNN#anchor.
func (a Anchor) String() string {
	if a.Slug != "" {
		return "ADR-" + a.ADR + "#" + a.Slug
	}
	return fmt.Sprintf("ADR-%s#%d", a.ADR, a.Item)
}

// coverage is the derived anchor model, built once inside NewCorpus
// (corpus-model-not-rebuilt), so no consumer rebuilds or diverges from it.
type coverage struct {
	anchors map[string][]Anchor // ADR number -> its anchors
}

// buildCoverage derives each ADR's anchor set: its Decision items followed by
// its declared invariant slugs.
func buildCoverage(adrs []ADR) coverage {
	c := coverage{anchors: map[string][]Anchor{}}
	for _, a := range adrs {
		var list []Anchor
		for _, n := range a.DecisionItems() {
			list = append(list, Anchor{ADR: a.Number, Item: n})
		}
		for _, slug := range a.DeclaredSlugs() {
			list = append(list, Anchor{ADR: a.Number, Slug: slug})
		}
		c.anchors[a.Number] = list
	}
	return c
}

// Anchors returns the named ADR's anchors: its Decision items then its declared
// invariant slugs.
func (c Corpus) Anchors(num string) []Anchor { return c.cov.anchors[num] }
