package adr_test

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
)

// TestAnchorRendering covers both anchor shapes in the token rendering. The
// token form is what the grammar itself writes, so a fault message quoting an
// anchor reads back as something an author can grep for.
func TestAnchorRendering(t *testing.T) {
	item := adr.Anchor{ADR: "0120", Item: 3}
	slug := adr.Anchor{ADR: "0120", Slug: "some-slug"}
	if got, want := item.String(), "ADR-0120#3"; got != want {
		t.Errorf("item String = %q, want %q", got, want)
	}
	if got, want := slug.String(), "ADR-0120#some-slug"; got != want {
		t.Errorf("slug String = %q, want %q", got, want)
	}
}
