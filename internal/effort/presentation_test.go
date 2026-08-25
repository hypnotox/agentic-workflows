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

func TestMemoryDocumentMapsSuccessesAndRefusals(t *testing.T) {
	metadata := &MemoryMetadata{Effort: "demo", Phase: "phase", Next: "next", Updated: "2026-08-05T12:00:00Z"}
	next := 3
	line := 7
	for _, result := range []MemoryOperationResult{
		{Condition: MemoryRead, Memory: metadata, Content: "one\n", Range: &MemoryRange{StartLine: 2, EndLine: 2, TotalLines: 3, NextOffset: &next, TruncatedBy: "limit"}},
		{Condition: MemoryEdited, Memory: metadata, ReplacementCount: 1, Diff: &MemoryDiff{Text: "diff", FirstChangedLine: &line}},
		{Condition: MemoryUpdated, Memory: metadata},
		{Condition: MemoryNotOwner, Outcome: &MemoryOutcome{Category: "operation", Condition: "another owner is active", NextActions: []RecoveryAction{{Text: "attach again"}}}},
		{Condition: MemoryFailure, Outcome: &MemoryOutcome{Category: "operation", Condition: "publication uncertain", Cause: "disk failure", NextActions: []RecoveryAction{{Text: "read first"}}}},
	} {
		document, err := result.MemoryDocument()
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err := presentation.Render(&out, document); err != nil {
			t.Fatal(err)
		}
		if out.Len() == 0 {
			t.Fatal("memory document rendered empty")
		}
	}
	for _, result := range []MemoryOperationResult{
		{},
		{Condition: MemoryUpdated},
		{Condition: MemoryRead, Memory: metadata},
		{Condition: MemoryEdited, Memory: metadata},
		{Condition: MemoryUpdated, Memory: &MemoryMetadata{Effort: "bad\nvalue", Phase: "phase", Next: "next", Updated: "time"}},
		{Condition: MemoryNotOwner, Outcome: &MemoryOutcome{Category: "operation", Condition: "", NextActions: []RecoveryAction{{Text: "recover"}}}},
		{Condition: MemoryNotOwner, Outcome: &MemoryOutcome{Category: "", Condition: "owner", NextActions: []RecoveryAction{{Text: "recover"}}}},
		{Condition: MemoryNotOwner, Outcome: &MemoryOutcome{Category: "operation", Condition: "owner", NextActions: []RecoveryAction{{Text: "bad\naction"}}}},
	} {
		if _, err := result.MemoryDocument(); err == nil {
			t.Fatalf("malformed memory result accepted: %#v", result)
		}
	}
}

func TestMemoryDocumentRetainsEveryTypedPresentationFact(t *testing.T) {
	metadata := &MemoryMetadata{Effort: "demo", Phase: "phase", Next: "next", Updated: "2026-08-05T12:00:00Z"}
	next := 8
	line := 7
	cases := []struct {
		name   string
		result MemoryOperationResult
		facts  []string
	}{
		{"read", MemoryOperationResult{Condition: MemoryRead, Memory: metadata, Content: "body\n", Range: &MemoryRange{StartLine: 7, EndLine: 7, TotalLines: 9, NextOffset: &next, TruncatedBy: "bytes"}}, []string{"start line: 7", "end line: 7", "total lines: 9", "next offset: 8", "truncated by: bytes", `content: "body\n"`}},
		{"read-null", MemoryOperationResult{Condition: MemoryRead, Memory: metadata, Content: "body", Range: &MemoryRange{StartLine: 7, EndLine: 7, TotalLines: 7, TruncatedBy: "none"}}, []string{"next offset: null"}},
		{"edit", MemoryOperationResult{Condition: MemoryEdited, Memory: metadata, ReplacementCount: 2, Diff: &MemoryDiff{Text: "before/after", FirstChangedLine: &line, Truncated: true}}, []string{"replacements: 2", `diff: "before/after"`, "first changed line: 7", "diff truncated: yes"}},
		{"edit-null", MemoryOperationResult{Condition: MemoryEdited, Memory: metadata, ReplacementCount: 1, Diff: &MemoryDiff{}}, []string{"first changed line: null"}},
		{"offset", MemoryOperationResult{Condition: MemoryOffsetOutOfRange, Outcome: &MemoryOutcome{Category: "operation", Condition: "outside", NextActions: []RecoveryAction{{Text: "retry"}}}, Offset: &MemoryOffsetFact{Offset: 10, TotalLines: 9}}, []string{"changed memory: no", "offset: 10", "total lines: 9"}},
		{"no-match", MemoryOperationResult{Condition: MemoryNoMatch, Outcome: &MemoryOutcome{Category: "operation", Condition: "absent", NextActions: []RecoveryAction{{Text: "retry"}}}, Edit: &MemoryEditFact{Index: 3}}, []string{"edit index: 3"}},
		{"ambiguous", MemoryOperationResult{Condition: MemoryAmbiguousMatch, Outcome: &MemoryOutcome{Category: "operation", Condition: "repeated", NextActions: []RecoveryAction{{Text: "retry"}}}, Edit: &MemoryEditFact{Index: 4, Occurrences: 2}}, []string{"edit index: 4", "occurrences: 2"}},
		{"overlap", MemoryOperationResult{Condition: MemoryOverlappingEdits, Outcome: &MemoryOutcome{Category: "operation", Condition: "overlap", NextActions: []RecoveryAction{{Text: "retry"}}}, Overlap: &MemoryOverlapFact{FirstIndex: 2, SecondIndex: 5}}, []string{"first edit index: 2", "second edit index: 5"}},
		{"size", MemoryOperationResult{Condition: MemoryResultTooLarge, Outcome: &MemoryOutcome{Category: "operation", Condition: "large", NextActions: []RecoveryAction{{Text: "retry"}}}, Size: &MemorySizeFact{Bytes: 51201, MaxBytes: 51200}}, []string{"bytes: 51201", "maximum bytes: 51200"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document, err := tc.result.MemoryDocument()
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := presentation.Render(&out, document); err != nil {
				t.Fatal(err)
			}
			for _, fact := range tc.facts {
				if !bytes.Contains(out.Bytes(), []byte(fact)) {
					t.Fatalf("output %q omitted %q", out.String(), fact)
				}
			}
		})
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
