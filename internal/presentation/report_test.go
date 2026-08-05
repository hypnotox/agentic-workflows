package presentation

import (
	"bytes"
	"testing"
)

func TestReportDocument(t *testing.T) {
	value, err := Prose("ready")
	if err != nil {
		t.Fatal(err)
	}
	context, err := NewField("scope", value)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := NewField("findings", value)
	if err != nil {
		t.Fatal(err)
	}
	record, err := NewRecord(value, value)
	if err != nil {
		t.Fatal(err)
	}
	document, err := (Report{Status: "completed", Context: []Field{context}, Summary: []Field{summary}, Categories: []ReportCategory{{Label: "warnings", Schema: []string{"rule", "detail"}, Records: []Record{record}}}}).Document()
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := Render(&got, document); err != nil {
		t.Fatal(err)
	}
	const want = "status: completed\n\ncontext:\n  scope: ready\n\nsummary:\n  findings: ready\n\nfindings:\n  warnings:\n    ready | ready\n"
	if got.String() != want {
		t.Fatalf("report = %q, want %q", got.String(), want)
	}
	for _, report := range []Report{
		{Status: " "},
		{Status: "ready", Context: []Field{{}}},
		{Status: "ready", Summary: []Field{{}}},
		{Status: "ready", Categories: []ReportCategory{{Label: "Bad", Schema: []string{"detail"}, Records: []Record{record}}}},
	} {
		if _, err := report.Document(); err == nil {
			t.Fatal("invalid report accepted")
		}
	}
}
