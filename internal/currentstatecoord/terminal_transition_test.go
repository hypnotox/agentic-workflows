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
