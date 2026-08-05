package effort

import (
	"bytes"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestRecordPresentationPreservesMemoryPath(t *testing.T) {
	record := Record{Slug: "demo", Title: "Demo", MemoryPath: "/primary  root/.awf/efforts/demo/memory.md"}
	detail, err := record.Detail()
	if err != nil {
		t.Fatal(err)
	}
	document, err := detail.Document()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := presentation.Render(&out, document); err != nil {
		t.Fatal(err)
	}
	const detailWant = "slug: demo\ntitle: Demo\nmemory: /primary  root/.awf/efforts/demo/memory.md\n"
	if out.String() != detailWant {
		t.Fatalf("detail = %q, want %q", out.String(), detailWant)
	}

	mutation, err := record.NewEffortMutation(presentation.Mutation{Status: "completed"})
	if err != nil {
		t.Fatal(err)
	}
	document, err = mutation.Document()
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := presentation.Render(&out, document); err != nil {
		t.Fatal(err)
	}
	const mutationWant = "status: completed\n\nmutation:\n  identity:\n    effort: demo\n    title: Demo\n    memory: /primary  root/.awf/efforts/demo/memory.md\n"
	if out.String() != mutationWant {
		t.Fatalf("mutation = %q, want %q", out.String(), mutationWant)
	}

	invalid := Record{Slug: "demo", Title: "Demo", MemoryPath: "/primary\nroot/memory.md"}
	if _, err := invalid.Detail(); err == nil {
		t.Fatal("detail accepted a memory path with a line break")
	}
	if _, err := invalid.NewEffortMutation(presentation.Mutation{Status: "completed"}); err == nil {
		t.Fatal("mutation accepted a memory path with a line break")
	}
}

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
