package currentstatecoord

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

type publicationOutcomeForTest struct {
	committed bool
	mutation  presentation.Mutation
}

func (o publicationOutcomeForTest) HasCommittedEffects() bool { return o.committed }
func (o publicationOutcomeForTest) PartialMutation() (presentation.Mutation, error) {
	return o.mutation, nil
}

// TestNumberPendingADRsSeparatesPreAndPostMutationUniverses proves the
// coordinator selects the pre-mutation corpus before its effects, then leaves
// post-mutation loading to its callback rather than sharing the first corpus.
func TestNumberPendingADRsSeparatesPreAndPostMutationUniverses(t *testing.T) {
	root := t.TempDir()
	decisions := filepath.Join(root, "docs", "decisions")
	testsupport.WriteFile(t, filepath.Join(decisions, "0001-seed.md"),
		testsupport.ADR("Implemented", testsupport.WithDate("2026-07-31"), testsupport.WithTitle("0001: Seed"), testsupport.WithBody("## Context\nx\n## Consequences\nc\n")))
	testsupport.WriteFile(t, filepath.Join(decisions, "pending.md"), pendingNumberingRecord(t, "pending"))

	var callbackSawPostMutation bool
	lease, err := filesystem.AcquireTrackedLease(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	releaseNumberingLease(t, lease)
	report, err := NumberPendingADRsLeased(root, nil, func() (PublicationOutcome, error) {
		corpus, err := adr.LoadCorpus(decisions)
		if err != nil {
			return nil, err
		}
		_, callbackSawPostMutation = corpus.BySlug("pending")
		return nil, nil
	}, lease)
	if err != nil {
		t.Fatalf("NumberPendingADRs: %v", err)
	}
	if got, want := report.Assignments, []NumberAssignment{{Slug: "pending", Number: "0002"}}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("assignments = %#v, want %#v", got, want)
	}
	if !callbackSawPostMutation {
		t.Fatal("post-mutation callback did not load the numbered corpus")
	}
	if _, err := os.Stat(filepath.Join(decisions, "0002-pending.md")); err != nil {
		t.Fatalf("numbered ADR missing: %v", err)
	}
}

func TestPartialNumberingDocumentRetainsExactPublisherAndPathEffects(t *testing.T) {
	publisherEffect, err := presentation.Literal("output-replaced AGENTS.md; recovery: inspect AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	publisherNext, err := presentation.Prose("inspect AGENTS.md, then rerun awf render")
	if err != nil {
		t.Fatal(err)
	}
	partial := &PartialNumberingError{
		Report: NumberingReport{Assignments: []NumberAssignment{{Slug: "pending", Number: "0002"}}},
		Effects: []NumberingEffect{
			{Kind: "destination-published", Path: "docs/decisions/0002-pending.md"},
			{Kind: "source-retired", Path: "docs/decisions/pending.md"},
			{Kind: "provenance-replaced", Path: ".awf/topics/parts/d/t/current-state.md"},
		},
		Publication: publicationOutcomeForTest{committed: true, mutation: presentation.Mutation{
			Changes:     []presentation.MutationChange{{Label: "committed effects", Values: []presentation.Value{publisherEffect}}},
			NextActions: []presentation.Value{publisherNext},
		}},
		Recovery: []string{"repair publication, then run awf render"},
	}
	document, err := partial.Document()
	if err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	if err := presentation.Render(&output, document); err != nil {
		t.Fatal(err)
	}
	for _, fact := range []string{
		"pending -> 0002",
		"destination-published docs/decisions/0002-pending.md",
		"source-retired docs/decisions/pending.md",
		"provenance-replaced .awf/topics/parts/d/t/current-state.md",
		"output-replaced AGENTS.md; recovery: inspect AGENTS.md",
		"inspect AGENTS.md, then rerun awf render",
		"repair publication, then run awf render",
	} {
		if !strings.Contains(output.String(), fact) {
			t.Errorf("partial presentation omitted %q:\n%s", fact, output.String())
		}
	}
}

func TestNumberPendingADRsRejectsSymlinkedAuthorityThroughConfinedCorpus(t *testing.T) {
	root := t.TempDir()
	decisions := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(decisions, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "pending.md")
	testsupport.WriteFile(t, outside, pendingNumberingRecord(t, "pending"))
	if err := os.Symlink(outside, filepath.Join(decisions, "pending.md")); err != nil {
		t.Fatal(err)
	}
	lease, err := filesystem.AcquireTrackedLease(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	releaseNumberingLease(t, lease)
	published := false
	report, err := NumberPendingADRsLeased(root, nil, func() (PublicationOutcome, error) {
		published = true
		return nil, nil
	}, lease)
	if err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("symlink authority error = %v", err)
	}
	if len(report.Assignments) != 0 || published {
		t.Fatalf("refusal mutated or published: report=%#v published=%t", report, published)
	}
	if got, readErr := os.ReadFile(outside); readErr != nil || !strings.Contains(string(got), "# ADR-pending:") {
		t.Fatalf("outside authority changed: %q, %v", got, readErr)
	}
}

func TestNumberingReportDocumentValidatesAssignments(t *testing.T) {
	for _, report := range []NumberingReport{
		{Assignments: []NumberAssignment{{Slug: "bad\nslug", Number: "0002"}}},
		{Assignments: []NumberAssignment{{Slug: "slug", Number: "bad\nnumber"}}},
	} {
		if _, err := report.Document(); err == nil {
			t.Fatal("invalid numbering assignment produced a presentation")
		}
	}
	if _, err := (NumberingReport{}).Document(); err != nil {
		t.Fatalf("empty report: %v", err)
	}
}

func releaseNumberingLease(t *testing.T, lease *filesystem.Lease) {
	t.Helper()
	t.Cleanup(func() {
		if err := lease.Release(); err != nil {
			t.Errorf("release numbering lease: %v", err)
		}
	})
}

func pendingNumberingRecord(t *testing.T, slug string) string {
	t.Helper()
	body := func(digest string) string {
		return "---\nformat: current-state-v3\nslug: " + slug + "\nstatus: Implemented\ndate: 2026-07-31\n---\n" +
			"# ADR-" + slug + ": Pending\n\n## Context\n\nx\n\n## Decision\n\n1. Pending.\n\n## State changes\n\nNone.\n\n## Consequences\n\nc\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-07-31: Proposed\n" +
			"- 2026-07-31: Accepted; content-sha256: " + digest + "\n" +
			"- 2026-07-31: Implemented; content-sha256: " + digest + "\n"
	}
	parsed, _, err := adr.ParseBytes(slug+".md", []byte(body("placeholder")))
	if err != nil {
		t.Fatalf("build pending ADR: %v", err)
	}
	return body(adr.ContentDigest(parsed.Sections))
}

func TestNumberPendingADRsRequiresCoveringLeaseAndRefusesADR9999Exhaustion(t *testing.T) {
	t.Run("noncovering lease", func(t *testing.T) {
		root := t.TempDir()
		lease, err := filesystem.AcquireTrackedLease(context.Background(), t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		releaseNumberingLease(t, lease)
		if _, err := NumberPendingADRsLeased(root, nil, nil, lease); err == nil || !strings.Contains(err.Error(), "requires a covering tracked lease") {
			t.Fatalf("lease refusal = %v", err)
		}
	})

	t.Run("exhausted identity space", func(t *testing.T) {
		root := t.TempDir()
		decisions := filepath.Join(root, "docs", "decisions")
		testsupport.WriteFile(t, filepath.Join(decisions, "9999-max.md"), testsupport.ADR("Implemented", testsupport.WithDate("2026-07-31"), testsupport.WithTitle("9999: Max"), testsupport.WithBody("## Context\nx\n## Consequences\nc\n")))
		testsupport.WriteFile(t, filepath.Join(decisions, "pending.md"), pendingNumberingRecord(t, "pending"))
		lease, err := filesystem.AcquireTrackedLease(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		releaseNumberingLease(t, lease)
		if _, err := NumberPendingADRsLeased(root, nil, func() (PublicationOutcome, error) { return nil, nil }, lease); err == nil || !strings.Contains(err.Error(), "identity space exhausted") {
			t.Fatalf("exhaustion refusal = %v", err)
		}
		if _, err := os.Stat(filepath.Join(decisions, "pending.md")); err != nil {
			t.Fatalf("pending source changed on exhaustion: %v", err)
		}
		if matches, err := filepath.Glob(filepath.Join(decisions, "10000-*.md")); err != nil || len(matches) != 0 {
			t.Fatalf("numbered destinations on exhaustion = %v, %v", matches, err)
		}
	})
}

func TestNumberPendingADRsRetainsProvenanceEffectsAfterLaterWalkFailure(t *testing.T) {
	root := t.TempDir()
	decisions := filepath.Join(root, "docs", "decisions")
	testsupport.WriteFile(t, filepath.Join(decisions, "0001-seed.md"), testsupport.ADR("Implemented", testsupport.WithDate("2026-07-31"), testsupport.WithTitle("0001: Seed"), testsupport.WithBody("## Context\nx\n## Consequences\nc\n")))
	testsupport.WriteFile(t, filepath.Join(decisions, "pending.md"), pendingNumberingRecord(t, "pending"))
	part := filepath.Join(root, ".awf", "topics", "parts", "d", "t", "current-state.md")
	testsupport.WriteFile(t, part, "Origin: ADR-pending\n")
	outside := filepath.Join(t.TempDir(), "outside")
	testsupport.WriteFile(t, outside, "outside\n")
	if err := os.Symlink(outside, filepath.Join(root, ".awf", "topics", "parts", "z-link")); err != nil {
		t.Fatal(err)
	}
	lease, err := filesystem.AcquireTrackedLease(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	releaseNumberingLease(t, lease)
	report, err := NumberPendingADRsLeased(root, nil, func() (PublicationOutcome, error) { return nil, nil }, lease)
	var partial *PartialNumberingError
	if !errors.As(err, &partial) || len(report.Assignments) != 1 || len(partial.Effects) != 3 || partial.Effects[2].Kind != "provenance-replaced" {
		t.Fatalf("partial = %#v, err = %v", partial, err)
	}
	if got, readErr := os.ReadFile(part); readErr != nil || !strings.Contains(string(got), "ADR-0002") {
		t.Fatalf("provenance = %q, %v", got, readErr)
	}
}
