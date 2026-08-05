package main

import (
	"errors"
	"strings"
	"testing"
)

const completedCheckReport = "status: completed\n\nsummary:\n  findings: 0 errors, 0 warnings\n"

func requireCheckReport(t *testing.T, got string) {
	t.Helper()
	if got != completedCheckReport {
		t.Fatalf("report = %q, want %q", got, completedCheckReport)
	}
}

func TestCheckReportRejectsInvalidPresentationValues(t *testing.T) {
	for _, collection := range []struct {
		notes    []string
		findings []checkFinding
	}{
		{notes: []string{"\n"}},
		{findings: []checkFinding{{severity: "warn", check: "\n", detail: "detail"}}},
		{findings: []checkFinding{{severity: "error", check: "check", detail: "\n"}}},
	} {
		if _, err := checkReport(collection.notes, collection.findings); err == nil {
			t.Fatal("invalid presentation value accepted")
		}
	}
}

func TestRenderCheckCollectionPropagatesConstructionAndWriterFailures(t *testing.T) {
	var clean strings.Builder
	if err := renderCheckCollection(&clean, checkCollection{}); err != nil {
		t.Fatal(err)
	}
	requireCheckReport(t, clean.String())
	if err := renderCheckCollection(&failOnWrite{failAt: 1, err: errors.New("writer failed")}, checkCollection{}); err == nil {
		t.Fatal("writer failure accepted")
	}
	if err := renderCheckCollection(&failOnWrite{failAt: 1, err: errors.New("unused")}, checkCollection{notes: []string{"\n"}}); err == nil {
		t.Fatal("invalid report fact accepted")
	}
}

func TestCheckCollectionAppendDeduplicatesReportFacts(t *testing.T) {
	first := checkCollection{notes: []string{"same"}, findings: []checkFinding{{severity: "warn", check: "check", detail: "same"}}}
	second := checkCollection{notes: []string{"same", "next"}, findings: []checkFinding{{severity: "warn", check: "check", detail: "same"}, {severity: "error", check: "check", detail: "next"}}}
	got := first.append(second)
	if len(got.notes) != 2 || len(got.findings) != 2 {
		t.Fatalf("deduplicated collection = %#v", got)
	}
}
