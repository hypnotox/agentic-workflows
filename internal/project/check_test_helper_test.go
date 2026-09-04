package project

import (
	"context"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/repositorycheck"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// checkProject preserves the terse drift-only shape used by project tests.
func checkProject(p *Session, ctx context.Context) ([]manifest.Drift, error) {
	report, err := checkReportProject(p, ctx)
	return reportDrift(report), err
}

func reportDrift(report repositorycheck.Report) []manifest.Drift {
	var drift []manifest.Drift
	for _, finding := range report.DirectResult.Findings() {
		drift = append(drift, manifest.Drift{Kind: finding.Evidence.Kind, Path: finding.Evidence.Path, Detail: finding.Evidence.Detail})
	}
	for _, information := range report.DirectResult.Information() {
		drift = append(drift, manifest.Drift{Kind: information.Evidence.Kind, Path: information.Evidence.Path, Detail: information.Evidence.Detail})
	}
	return drift
}

func reportNotes(report repositorycheck.Report) []string {
	result := report.AggregateAdvisoryResult()
	notes := make([]string, 0, len(result.Information())+len(result.Findings()))
	for _, information := range result.Information() {
		notes = append(notes, information.Evidence.Detail)
	}
	for _, finding := range result.Findings() {
		notes = append(notes, finding.Evidence.Detail)
	}
	return notes
}

// mustDeriveCorpus derives the operation-owned ADR corpus the way a lifecycle
// entry does, so a helper test exercises the same threaded value production
// passes it (ADR-0180).
func mustPitfallCorpus(t *testing.T, p *Session) pitfall.Corpus {
	t.Helper()
	corpus, err := loadPitfallCorpus(renderInputsForTest(p))
	if err != nil {
		t.Fatal(err)
	}
	return corpus
}

// mustDeriveTopics derives the operation-owned topic corpus the same way.
func mustDeriveTopics(t *testing.T, p *Session) topic.Corpus {
	t.Helper()
	_, topics, err := deriveOperationStateWithPitfalls(renderInputsForTest(p))
	if err != nil {
		t.Fatalf("derive operation state: %v", err)
	}
	return topics
}
