package project

import (
	"context"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// checkProject preserves the terse drift-only shape used by project tests while
// production consumers retain CheckReport and its tracking advisories.
func checkProject(p *ProjectState, ctx context.Context) ([]manifest.Drift, error) {
	report, err := checkReportProject(p, ctx)
	return report.Drift, err
}

// mustDeriveCorpus derives the operation-owned ADR corpus the way a lifecycle
// entry does, so a helper test exercises the same threaded value production
// passes it (ADR-0180).
func mustPitfallCorpus(t *testing.T, p *ProjectState) pitfall.Corpus {
	t.Helper()
	corpus, err := loadPitfallCorpus(renderInputsForTest(p))
	if err != nil {
		t.Fatal(err)
	}
	return corpus
}

// mustDeriveTopics derives the operation-owned topic corpus the same way.
func mustDeriveTopics(t *testing.T, p *ProjectState) topic.Corpus {
	t.Helper()
	_, topics, _, err := deriveOperationStateWithPitfalls(renderInputsForTest(p))
	if err != nil {
		t.Fatalf("derive operation state: %v", err)
	}
	return topics
}

// mustDeriveSkills derives the operation-owned effective skill set the same way.
func mustDeriveSkills(t *testing.T, p *ProjectState) map[string]bool {
	t.Helper()
	_, _, eff, err := deriveOperationStateWithPitfalls(renderInputsForTest(p))
	if err != nil {
		t.Fatalf("derive operation state: %v", err)
	}
	return eff
}
