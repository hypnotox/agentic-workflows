package currentstate

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
)

func TestFilenamesQualifyExactIntegrationSubstitutions(t *testing.T) {
	cases := []struct {
		name           string
		parent, result adr.ADR
		want           bool
	}{
		{"equal", adr.ADR{Number: "0200", Filename: "0200-old.md"}, adr.ADR{Number: "0200", Filename: "0200-old.md"}, true},
		{"same identity renamed suffix", adr.ADR{Number: "0200", Filename: "0200-old.md"}, adr.ADR{Number: "0200", Filename: "0200-new.md"}, false},
		{"pending numbering", adr.ADR{Slug: "pending", Filename: "pending.md"}, adr.ADR{Number: "0202", Slug: "pending", Filename: "0202-pending.md"}, true},
		{"pending wrong slug", adr.ADR{Slug: "pending", Filename: "pending.md"}, adr.ADR{Number: "0202", Slug: "other", Filename: "0202-other.md"}, false},
		{"slugless renumber", adr.ADR{Number: "0200", Filename: "0200-old.md"}, adr.ADR{Number: "0201", Filename: "0201-old.md"}, true},
		{"missing result number", adr.ADR{Number: "0200", Filename: "0200-old.md"}, adr.ADR{Filename: "old.md"}, false},
		{"malformed parent filename", adr.ADR{Number: "0200", Filename: "wrong.md"}, adr.ADR{Number: "0201", Filename: "0201-wrong.md"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := filenamesQualify(c.parent, c.result); got != c.want {
				t.Fatalf("filenamesQualify(%#v, %#v) = %v, want %v", c.parent, c.result, got, c.want)
			}
		})
	}
}
