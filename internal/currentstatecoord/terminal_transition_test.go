package currentstatecoord

import (
	"context"
	"strings"
	"testing"
	"time"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// TestSelectedTerminalEvidenceUsesRepositoryHistory proves that a status-only
// index transition cannot substitute for the plan-selected implementation range.
func terminalTransitionPlan(status, reconciliation string) string {
	return "---\nformat: plan-v2\ndate: 2026-08-27\nadrs: []\nstatus: " + status + "\n---\n" +
		"# Plan: Terminal fixture\n\n## Goal\n\nProve selected history.\n\n## Architecture summary\n\nUse the staged transition owner.\n\n" +
		"## Phase 1: Land\n\n**Execution mode: inline.**\n\nCompletes: [\"landed\"]\n\n### Task 1.1: Land\n\nLand it.\n\n" +
		"### Phase close\n\n```commit\nfix(plans): land fixture\n```\n\n## Definition of done\n\n- `dod: landed` Landed.\n\n## Notes\n\n" + reconciliation
}

// invariant: adr-system/plan-artifacts:terminal-plan-history-frozen (TestCheckStagedBindsTerminalTransitionToSelectedHistory)
func TestCheckStagedBindsTerminalTransitionToSelectedHistory(t *testing.T) {
	fixture := gitfixture.InitRepo(t)
	planPath := "docs/plans/2026-08-27-terminal-fixture.md"
	base := gitfixture.Commit(t, fixture, "base", map[string]string{
		".awf/config.yaml": "prefix: x\nprofile: core\nintegrationBranch: main\n",
		".awf/awf.lock":    `{"awfVersion":"0.39.2","schemaVersion":46,"files":{"prior":{}}}`,
		planPath:           terminalTransitionPlan("Proposed", "Mutable.\n"),
	})
	head := gitfixture.Commit(t, fixture, "implementation", map[string]string{"internal/landed.go": "package internal\n"})
	reconciliation := func(path string) string {
		return "### Terminal reconciliation\nImplementation range: " + base + ".." + head + "\nTouched paths:\n- \"" + path + "\"\nMaterial deviations:\n- none\n"
	}
	repo, err := awfgit.Open(fixture.Root())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gitfixture.Stage(t, fixture, map[string]string{planPath: terminalTransitionPlan("Implemented", reconciliation("internal/landed.go"))})
	report, err := CheckStaged(fixture.Root(), repo, ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, drift := range report.PlanDrift {
		if drift.Kind == "plan-terminal-transition" {
			t.Fatalf("matching selected history rejected: %#v", drift)
		}
	}

	gitfixture.Stage(t, fixture, map[string]string{planPath: terminalTransitionPlan("Implemented", reconciliation("internal/guessed.go"))})
	report, err = CheckStaged(fixture.Root(), repo, ctx)
	if err != nil {
		t.Fatal(err)
	}
	matched := false
	for _, drift := range report.PlanDrift {
		matched = matched || drift.Kind == "plan-terminal-transition" && strings.Contains(drift.Detail, "does not equal selected touched paths")
	}
	if !matched {
		t.Fatalf("mismatched selected history did not reach terminal policy: %#v", report.PlanDrift)
	}
}

func TestSelectedTerminalEvidenceUsesRepositoryHistory(t *testing.T) {
	fixture := gitfixture.InitRepo(t)
	base := gitfixture.Commit(t, fixture, "base", map[string]string{"internal/landed.go": "package internal\n"})
	gitfixture.Commit(t, fixture, "implementation", map[string]string{"internal/landed.go": "package internal\n// landed\n"})
	head := gitfixture.Commit(t, fixture, "restore", map[string]string{"internal/landed.go": "package internal\n"})
	repo, err := awfgit.Open(fixture.Root())
	if err != nil {
		t.Fatal(err)
	}
	before := plan.Plan{Filename: "p.md", Path: "docs/plans/p.md", Status: "Proposed"}
	after := before
	after.Status = "Implemented"
	after.TerminalReconciliation = &plan.TerminalReconciliation{ImplementationRange: base + ".." + head}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	selected, err := selectedTerminalEvidence([]plan.Plan{before}, []plan.Plan{after}, repo, ctx)
	if err != nil || len(selected["p.md"]) != 1 || selected["p.md"][0] != "internal/landed.go" {
		t.Fatalf("selected implementation evidence = %#v, %v", selected, err)
	}

	after.TerminalReconciliation = nil
	selected, err = selectedTerminalEvidence([]plan.Plan{before}, []plan.Plan{after}, repo, ctx)
	if err != nil || len(selected) != 0 {
		t.Fatalf("missing reconciliation selected evidence = %#v, %v", selected, err)
	}

	for _, selector := range []string{"missing.." + head, base + "..missing", head + ".." + base, head + ".." + head} {
		after.TerminalReconciliation = &plan.TerminalReconciliation{ImplementationRange: selector}
		if _, err := selectedTerminalEvidence([]plan.Plan{before}, []plan.Plan{after}, repo, ctx); err == nil || !strings.Contains(err.Error(), "docs/plans/p.md") {
			t.Fatalf("malformed, reversed, or empty selector %q accepted: %v", selector, err)
		} else if strings.HasPrefix(selector, "missing..") && !strings.Contains(err.Error(), "resolve selected implementation range base:") {
			t.Fatalf("repository base failure lost context: %v", err)
		} else if strings.HasSuffix(selector, "..missing") && !strings.Contains(err.Error(), "resolve selected implementation range head:") {
			t.Fatalf("repository head failure lost context: %v", err)
		}
	}

	gitfixture.CheckoutNewBranch(t, fixture, "divergent", base)
	divergent := gitfixture.Commit(t, fixture, "divergent", map[string]string{"side.go": "side\n"})
	gitfixture.NativeCheckout(t, fixture, "master")
	after.TerminalReconciliation = &plan.TerminalReconciliation{ImplementationRange: divergent + ".." + head}
	if _, err := selectedTerminalEvidence([]plan.Plan{before}, []plan.Plan{after}, repo, ctx); err == nil || !strings.Contains(err.Error(), "not an ancestor") {
		t.Fatalf("divergent selected history accepted: %v", err)
	}

	stale := gitfixture.Commit(t, fixture, "later", map[string]string{"later.go": "later\n"})
	after.TerminalReconciliation = &plan.TerminalReconciliation{ImplementationRange: base + ".." + head}
	if _, err := selectedTerminalEvidence([]plan.Plan{before}, []plan.Plan{after}, repo, ctx); err == nil || !strings.Contains(err.Error(), "current checkout HEAD") {
		t.Fatalf("stale selected head accepted after %s: %v", stale, err)
	}
}
