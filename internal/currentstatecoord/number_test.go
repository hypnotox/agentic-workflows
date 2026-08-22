package currentstatecoord

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

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
	report, err := NumberPendingADRs(root, nil, func() error {
		corpus, err := adr.LoadCorpus(decisions)
		if err != nil {
			return err
		}
		_, callbackSawPostMutation = corpus.BySlug("pending")
		return nil
	})
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
