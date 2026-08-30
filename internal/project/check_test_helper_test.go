package project

import (
	"context"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
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

func mustDeriveCorpus(t *testing.T, p *ProjectState) adr.Corpus {
	t.Helper()
	corpus, _, _, _, err := deriveOperationStateWithPitfalls(renderInputsForTest(p))
	if err != nil {
		t.Fatalf("derive operation state: %v", err)
	}
	return corpus
}

// mustDeriveTopics derives the operation-owned topic corpus the same way.
func mustDeriveTopics(t *testing.T, p *ProjectState) topic.Corpus {
	t.Helper()
	_, _, topics, _, err := deriveOperationStateWithPitfalls(renderInputsForTest(p))
	if err != nil {
		t.Fatalf("derive operation state: %v", err)
	}
	return topics
}

// mustDeriveSkills derives the operation-owned effective skill set the same way.
func mustDeriveSkills(t *testing.T, p *ProjectState) map[string]bool {
	t.Helper()
	_, _, _, eff, err := deriveOperationStateWithPitfalls(renderInputsForTest(p))
	if err != nil {
		t.Fatalf("derive operation state: %v", err)
	}
	return eff
}

// pendingADRFixture is a valid Proposed pending current-state-v3 record: slug
// identity, no number, and the slug-form heading.
func pendingADRFixture(slug string) string {
	return "---\nformat: current-state-v3\nslug: " + slug + "\nstatus: Proposed\ndate: 2026-07-31\n---\n" +
		"# ADR-" + slug + ": A decision\n\n" +
		"## Context\n\nBackground prose.\n\n" +
		"## Decision\n\n1. The only decision.\n\n" +
		"## State changes\n\nNone.\n\n" +
		"## Consequences\n\nConsequence prose.\n\n" +
		"## Alternatives Considered\n\nNone considered.\n\n" +
		"## Status history\n\n- 2026-07-31: Proposed\n"
}
