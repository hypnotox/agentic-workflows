package adr

import "testing"

// Citations answers for an ADR the corpus does not hold, and for one whose
// Decision section names no anchor at all. Both are the "nothing to say" paths
// a consumer hits before any citation exists.
func TestCitationsAbsentOrEmpty(t *testing.T) {
	c := NewCorpus([]ADR{{
		Number:   "0001",
		Filename: "0001-a.md",
		Status:   "Implemented",
		Sections: map[string]string{"Decision": "1. **A decision.** It overrides nothing.\n"},
	}})
	if got := c.Citations("9999"); got != nil {
		t.Errorf("an absent ADR cites nothing, got %v", got)
	}
	if got := c.Citations("0001"); got != nil {
		t.Errorf("a Decision section naming no anchor cites nothing, got %v", got)
	}
}

// A slug citation names no ADR of its own, so it hangs off the nearest
// preceding ADR reference in the same Decision item. With no such reference
// the citation addresses no anchor and is dropped rather than guessed at: a
// reference in a *different* item must not capture it.
func TestCitationsSlugWithoutPrecedingADRReference(t *testing.T) {
	c := NewCorpus([]ADR{{
		Number:   "0002",
		Filename: "0002-b.md",
		Status:   "Implemented",
		Sections: map[string]string{"Decision": "" +
			"1. **Mentions an ADR.** ADR-0001 is discussed here.\n\n" +
			"2. **Names a slug alone.** The `inv: orphan-slug` is overridden here.\n"},
	}})
	for _, got := range c.Citations("0002") {
		if got.Anchor.Slug == "orphan-slug" {
			t.Errorf("a slug with no ADR reference in its own item must be dropped, got %+v", got)
		}
	}
}
