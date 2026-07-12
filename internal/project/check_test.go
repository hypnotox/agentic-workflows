package project

import (
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestCheckPlansValidatesFrontmatterAndLinks exercises checkPlans over a
// docs/plans/ set: a plan linking a nonexistent ADR yields plan-adr-link drift,
// a bad status: yields plan-frontmatter drift, a valid plan yields none, and a
// frontmatter-less (grandfathered) plan is skipped.
// invariant: plan-frontmatter-validated
// invariant: plan-adr-link-resolved
func TestCheckPlansValidatesFrontmatterAndLinks(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	// One real ADR (0001) for links to resolve against.
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-real.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-12"),
			testsupport.WithTitle("0001: Real"), testsupport.WithBody("## Context\nx\n")))

	write := func(name, content string) {
		testsupport.WriteFile(t, filepath.Join(root, "docs/plans", name), content)
	}
	write("2026-07-12-good.md", "---\ndate: 2026-07-12\nadrs: [1]\nstatus: Proposed\n---\n# Plan: Good\n")
	write("2026-07-12-bad-link.md", "---\ndate: 2026-07-12\nadrs: [42]\nstatus: Proposed\n---\n# Plan: Bad Link\n")
	write("2026-07-12-bad-status.md", "---\ndate: 2026-07-12\nadrs: [1]\nstatus: Draft\n---\n# Plan: Bad Status\n")
	write("2026-06-24-legacy.md", "# Plan: Legacy\n\nNo frontmatter, grandfathered.\n")

	drift, err := p.checkPlans()
	if err != nil {
		t.Fatalf("checkPlans: %v", err)
	}

	got := map[string]string{}
	for _, d := range drift {
		got[d.Kind+"@"+filepath.Base(d.Path)] = d.Detail
	}
	if len(drift) != 2 {
		t.Fatalf("expected exactly 2 drifts (bad-link, bad-status), got %d: %#v", len(drift), drift)
	}
	if d, ok := got["plan-adr-link@2026-07-12-bad-link.md"]; !ok || d != "ADR-0042" {
		t.Errorf("expected plan-adr-link ADR-0042 drift, got %#v", drift)
	}
	if _, ok := got["plan-frontmatter@2026-07-12-bad-status.md"]; !ok {
		t.Errorf("expected plan-frontmatter drift for bad status, got %#v", drift)
	}
}

// TestCheckPlansPropagatesPlanParseError covers checkPlans' plan.ParseDir error
// branch: malformed plan frontmatter is a hard error (the unparseable-YAML half
// of plan-frontmatter-validated), not silent drift.
func TestCheckPlansPropagatesPlanParseError(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-07-12-broken.md"),
		"---\nstatus: [unterminated\n---\n# Plan: Broken\n")
	if _, err := p.checkPlans(); err == nil {
		t.Fatal("expected plan.ParseDir error for malformed frontmatter, got nil")
	}
}

// TestCheckPlansPropagatesADRParseError covers checkPlans' adr.ParseDir error
// branch: a malformed ADR aborts the check.
func TestCheckPlansPropagatesADRParseError(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-broken.md"),
		"---\nstatus: [unterminated\n---\n# ADR-0001: Broken\n")
	if _, err := p.checkPlans(); err == nil {
		t.Fatal("expected adr.ParseDir error for malformed frontmatter, got nil")
	}
}

// TestCheckPropagatesPlanError covers Check's propagation of a checkPlans error:
// a synced, otherwise-clean project with a malformed plan makes full Check fail.
func TestCheckPropagatesPlanError(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-07-12-broken.md"),
		"---\nstatus: [unterminated\n---\n# Plan: Broken\n")
	if _, err := p.Check(); err == nil {
		t.Fatal("expected Check to propagate the checkPlans parse error, got nil")
	}
}
