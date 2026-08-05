package severity_test

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func TestRankString(t *testing.T) {
	for _, tc := range []struct {
		rank severity.Rank
		want string
	}{
		{severity.Error, "error"},
		{severity.Warn, "warn"},
	} {
		if got := tc.rank.String(); got != tc.want {
			t.Fatalf("Rank(%d).String() = %q, want %q", tc.rank, got, tc.want)
		}
	}
}

func TestErrorIsZeroValue(t *testing.T) {
	var zero severity.Rank
	if zero != severity.Error {
		t.Fatalf("zero Rank = %v, want Error", zero)
	}
}

// Every rank-bearing surface renders through the one shared type. Report
// categories are separately closed and ordered by presentation.Report.Document.
// invariant: tooling/audit-commands:severity-single-spelling (TestOneSpellingAcrossEveryRankSurface)
func TestOneSpellingAcrossEveryRankSurface(t *testing.T) {
	for _, tc := range []struct {
		what string
		got  string
		want string
	}{
		{"severity.Error", severity.Error.String(), "error"},
		{"severity.Warn", severity.Warn.String(), "warn"},
		{"audit.Finding", audit.Finding{Severity: severity.Warn}.Severity.String(), "warn"},
		{"topic.CoverageFinding", topic.CoverageFinding{Severity: severity.Error}.Severity.String(), "error"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s renders %q, want %q", tc.what, tc.got, tc.want)
		}
	}

	value, err := presentation.Prose("sentinel")
	if err != nil {
		t.Fatal(err)
	}
	record, err := presentation.NewRecord(value)
	if err != nil {
		t.Fatal(err)
	}
	category := func(label string) presentation.ReportCategory {
		return presentation.ReportCategory{Label: label, Schema: []string{"detail"}, Records: []presentation.Record{record}}
	}
	if _, err := (presentation.Report{Status: "ready", Categories: []presentation.ReportCategory{category("errors"), category("warnings")}}).Document(); err != nil {
		t.Fatalf("canonical report categories rejected: %v", err)
	}
	for _, categories := range [][]presentation.ReportCategory{
		{category("error")},
		{category("warn")},
		{category("warnings"), category("errors")},
	} {
		if _, err := (presentation.Report{Status: "ready", Categories: categories}).Document(); err == nil {
			t.Fatalf("noncanonical report categories accepted: %#v", categories)
		}
	}
}
