package project

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

const commitSubjectCfg = "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\ndomains: []\naudit:\n  allowedScopes:\n    - name: awf\n"

// TestCheckPlansCommitSubjectDrift covers the ```commit length/type/shape drift and
// confirms an unknown scope is NOT drift (it is an advisory note instead).
// invariant: adr-system/plan-artifacts:plan-commit-subject-length-checked (TestCheckPlansCommitSubjectDrift)
// invariant: adr-system/plan-artifacts:plan-commit-subject-shape-checked (TestCheckPlansCommitSubjectDrift)
func TestCheckPlansCommitSubjectDrift(t *testing.T) {
	root := scaffold(t, commitSubjectCfg)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	write := func(name, content string) {
		testsupport.WriteFile(t, filepath.Join(root, "docs/plans", name), content)
	}
	fm := "---\ndate: 2026-07-14\nadrs: []\nstatus: Proposed\n---\n# Plan: P\n\n"
	long := "feat(awf): " + strings.Repeat("x", 80)
	write("2026-07-14-long.md", fm+"```commit\n"+long+"\n```\n")
	write("2026-07-14-type.md", fm+"```commit\nzzz(awf): bad type\n```\n")
	write("2026-07-14-shape.md", fm+"```commit\nno conventional shape here\n```\n")
	write("2026-07-14-scope.md", fm+"```commit\nfeat(nope): unknown scope\n```\n")
	write("2026-07-14-ok.md", fm+"```commit\nfeat(awf): fine\n```\n")

	drift := checkPlans(renderInputsForTest(p), mustDeriveCorpus(t, p), mustParsePlans(t, p))
	got := map[string]bool{}
	for _, d := range drift {
		if d.Kind == "plan-commit-subject" {
			got[filepath.Base(d.Path)] = true
		}
	}
	for _, name := range []string{"2026-07-14-long.md", "2026-07-14-type.md", "2026-07-14-shape.md"} {
		if !got[name] {
			t.Errorf("expected plan-commit-subject drift for %s, got %#v", name, drift)
		}
	}
	if got["2026-07-14-scope.md"] {
		t.Errorf("unknown scope must be advisory, not drift: %#v", drift)
	}
	if got["2026-07-14-ok.md"] {
		t.Errorf("valid subject must not drift: %#v", drift)
	}
}

// TestPlanCommitScopeNotes covers the scope advisory: a note for an unknown scope,
// none for an over-length subject (Error, not Warning), a frontmatter-less plan
// skipped, and the ParseDir error branch.
// invariant: adr-system/plan-artifacts:plan-commit-subject-scope-advisory (TestPlanCommitScopeNotes)
func TestPlanCommitScopeNotes(t *testing.T) {
	root := scaffold(t, commitSubjectCfg)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	write := func(name, content string) {
		testsupport.WriteFile(t, filepath.Join(root, "docs/plans", name), content)
	}
	fm := "---\ndate: 2026-07-14\nadrs: []\nstatus: Proposed\n---\n# Plan: P\n\n"
	write("2026-07-14-scope.md", fm+"```commit\nfeat(nope): unknown scope\n```\n")
	write("2026-07-14-long.md", fm+"```commit\nfeat(awf): "+strings.Repeat("x", 80)+"\n```\n")
	// A frontmatter-less plan is skipped (covers the !HasFrontmatter continue); the
	// note count stays 1.
	write("2026-06-24-legacy.md", "# Plan: Legacy\n\nNo frontmatter, grandfathered.\n")

	notes := planCommitScopeNotes(renderInputsForTest(p), mustParsePlans(t, p))
	if len(notes) != 1 || !strings.Contains(notes[0], "2026-07-14-scope.md") || !strings.Contains(notes[0], "disallowed scope") {
		t.Fatalf("want one scope note, got %#v", notes)
	}
}

// TestAdvisoryNotesSurfacesPlanCommitError covers the planCommitScopeNotes error
// propagation wired into AdvisoryNotes. Empty tags keep tagHealthNotes inert (so it
// does not error first); a malformed plan makes planCommitScopeNotes' ParseDir fail.
func TestAdvisoryNotesSurfacesPlanCommitError(t *testing.T) {
	root := scaffold(t, "prefix: awf\nintegrationBranch: main\ndomains: []\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-07-14-broken.md"),
		"---\nstatus: [unterminated\n---\n# Plan: Broken\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := advisoryNotesProject(p); err == nil {
		t.Fatal("expected AdvisoryNotes to surface the plan-commit ParseDir error")
	}
}
