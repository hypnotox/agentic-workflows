package effort

import (
	"bytes"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestListDocumentUsesSemanticEffortsList(t *testing.T) {
	for _, test := range []struct {
		name    string
		records []Record
		want    string
	}{
		{
			name:    "nonempty",
			records: []Record{{Slug: "first", Title: "First effort"}, {Slug: "second", Title: "Second effort"}},
			want:    "effort list:\n  efforts:\n    first: First effort\n    second: Second effort\n",
		},
		{name: "empty", want: "efforts: none\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			document, err := ListDocument(test.records)
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := presentation.Render(&out, document); err != nil {
				t.Fatal(err)
			}
			if out.String() != test.want {
				t.Fatalf("list document = %q, want %q", out.String(), test.want)
			}
		})
	}
}
