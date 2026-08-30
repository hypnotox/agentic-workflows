package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// The pending-record block fires on a positive integration-branch
// identification and on nothing else: another branch, a detached HEAD, and a
// tree with no readable repository all pass, because an indeterminate answer is
// not evidence that the record is in the wrong place (ADR-0202 item 7).
func TestCheckPendingADRsFiresOnlyOnPositiveIdentification(t *testing.T) {
	for _, tc := range []struct {
		name      string
		wantDrift bool
		setup     func(t *testing.T) string
	}{
		{
			name: "on the integration branch blocks", wantDrift: true,
			setup: func(t *testing.T) string { return gitScaffold(t, defaultFixtureBranch) },
		},
		{
			name:  "another branch passes",
			setup: func(t *testing.T) string { return gitScaffold(t, "effort/side") },
		},
		{
			name: "detached HEAD passes",
			setup: func(t *testing.T) string {
				root := gitScaffold(t, defaultFixtureBranch)
				repo := gitfixture.At(root)
				gitfixture.NativeCheckout(t, repo, gitfixture.NativeRevParse(t, repo, "HEAD"))
				return root
			},
		},
		{
			name:  "an unreadable repository passes",
			setup: func(t *testing.T) string { return scaffold(t, gitSampleYAML) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := tc.setup(t)
			testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/still-pending.md"), pendingADRFixture("still-pending"))
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			drift := checkPendingADRs(renderInputsForTest(p), testRepo(p), testContext(t), mustDeriveCorpus(t, p))
			if !tc.wantDrift {
				if len(drift) != 0 {
					t.Fatalf("expected no drift, got %#v", drift)
				}
				return
			}
			if len(drift) != 1 {
				t.Fatalf("expected exactly one pending-record drift, got %#v", drift)
			}
			if drift[0].Kind != "pending-adr-on-integration-branch" || drift[0].Detail != "still-pending" || drift[0].Path != "docs/decisions/still-pending.md" {
				t.Errorf("drift = %#v", drift[0])
			}
		})
	}
}

// The block reaches awf check, not just its own helper. Without this the whole
// check could be unwired from Check and every helper-level test above would
// still pass.
func TestCheckReportsPendingADROnIntegrationBranch(t *testing.T) {
	root := gitScaffold(t, defaultFixtureBranch)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := initializeReportProject(p, InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	// Two records, because the claim quantifies over EVERY pending record: a
	// worktree that authored several is the case ADR-0202 item 8 orders by
	// argument, and reporting only the first would leave the rest to be
	// discovered one integration attempt at a time.
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/still-pending.md"), pendingADRFixture("still-pending"))
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/also-pending.md"), pendingADRFixture("also-pending"))
	drift, err := checkProject(p, testContext(t))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	reported := map[string]bool{}
	for _, d := range drift {
		if d.Kind == "pending-adr-on-integration-branch" {
			reported[d.Detail] = true
		}
	}
	if !reported["still-pending"] || !reported["also-pending"] {
		t.Fatalf("awf check did not report every pending record, got %v in %#v", reported, drift)
	}
}

// A branch probe that fails outright is the fourth indeterminate outcome, and
// it is distinct from the no-repository one: the handle exists, the git call
// itself errors. Removing the control directory under a live handle is the way
// to produce it; the block must stay silent rather than report a record it has
// no evidence is misplaced.
func TestCheckPendingADRsSilentOnProbeFailure(t *testing.T) {
	root := gitScaffold(t, defaultFixtureBranch)
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/still-pending.md"), pendingADRFixture("still-pending"))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	corpus := mustDeriveCorpus(t, p)
	// Sanity: with the repository intact this same corpus is blocked, so a
	// silent result below is the probe failure and not an empty corpus.
	if drift := checkPendingADRs(renderInputsForTest(p), testRepo(p), testContext(t), corpus); len(drift) != 1 {
		t.Fatalf("fixture does not block before the probe breaks: %#v", drift)
	}
	if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	if drift := checkPendingADRs(renderInputsForTest(p), testRepo(p), testContext(t), corpus); len(drift) != 0 {
		t.Fatalf("a failed branch probe must emit nothing, got %#v", drift)
	}
}

// A numbered corpus on the integration branch produces no block: the check
// reports the pending records, not every record.
func TestCheckPendingADRsIgnoresNumberedRecords(t *testing.T) {
	root := gitScaffold(t, defaultFixtureBranch)
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-real.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-12"),
			testsupport.WithTitle("0001: Real"), testsupport.WithBody("## Context\nx\n")))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if drift := checkPendingADRs(renderInputsForTest(p), testRepo(p), testContext(t), mustDeriveCorpus(t, p)); len(drift) != 0 {
		t.Fatalf("a numbered corpus must not be blocked, got %#v", drift)
	}
}
