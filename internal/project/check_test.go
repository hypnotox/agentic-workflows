package project

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

const pitfallsCheckCfg = "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: [pitfalls]\ndomains: [rendering]\n"

const commitSubjectCfg = "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\naudit:\n  allowedScopes:\n    - name: awf\n"

// A disabled pitfalls doc yields no drift and never reads the sidecar.
func TestCheckPitfallsDisabled(t *testing.T) {
	p, err := Open(scaffold(t, "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: []\n"))
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.checkPitfalls()
	if err != nil || drift != nil {
		t.Errorf("disabled pitfalls must yield no drift, got %v / %v", drift, err)
	}
}

// An unknown domain yields pitfall-domain drift, a dangling related ADR yields
// pitfall-adr-link drift, and an entry resolving both yields none.
// invariant: pitfall-domains-resolved
// invariant: pitfall-adr-link-resolved
func TestCheckPitfallsValidatesDomainsAndLinks(t *testing.T) {
	root := scaffoldFiles(t, pitfallsCheckCfg, map[string]string{
		"docs/pitfalls.yaml": "data:\n  pitfalls:\n" +
			"    - title: Clean\n      domains: [rendering]\n      related: [1]\n      body: ok\n" +
			"    - title: BadDomain\n      domains: [bogus]\n      body: ok\n" +
			"    - title: BadLink\n      related: [42]\n      body: ok\n",
	})
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-real.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-12"),
			testsupport.WithTitle("0001: Real"), testsupport.WithBody("## Context\nx\n")))
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.checkPitfalls()
	if err != nil {
		t.Fatalf("checkPitfalls: %v", err)
	}
	got := map[string]string{}
	for _, d := range drift {
		got[d.Kind] = d.Detail
	}
	if len(drift) != 2 || !strings.Contains(got["pitfall-domain"], "bogus") || !strings.Contains(got["pitfall-adr-link"], "0042") {
		t.Fatalf("want pitfall-domain(bogus) + pitfall-adr-link(0042) drift, got %#v", drift)
	}
}

// Valid YAML with a bad data.pitfalls shape surfaces the structural error.
func TestCheckPitfallsStructuralError(t *testing.T) {
	p, err := Open(scaffoldFiles(t, pitfallsCheckCfg, map[string]string{
		"docs/pitfalls.yaml": "data:\n  pitfalls: just a string\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.checkPitfalls(); err == nil || !strings.Contains(err.Error(), "must be a list") {
		t.Fatalf("expected structural error, got %v", err)
	}
}

// A malformed ADR aborts the check via adr.ParseDir.
func TestCheckPitfallsADRParseError(t *testing.T) {
	root := scaffoldFiles(t, pitfallsCheckCfg, map[string]string{
		"docs/pitfalls.yaml": "data:\n  pitfalls:\n    - title: T\n      body: ok\n",
	})
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-broken.md"),
		"---\nstatus: [unterminated\n---\n# ADR-0001: Broken\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.checkPitfalls(); err == nil {
		t.Fatal("expected adr.ParseDir error for malformed frontmatter, got nil")
	}
}

// A non-member tag on an ADR or a pitfall yields tag drift; an empty-meaning
// member yields tag-vocabulary drift; a fully-conforming corpus yields none.
// invariant: tag-vocabulary-governed
func TestCheckTagVocabulary(t *testing.T) {
	cfg := "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: [pitfalls]\ndomains: [rendering]\n" +
		"tags:\n  render-engine: the render engine\n  empty: \"\"\n"
	root := scaffoldFiles(t, cfg, map[string]string{
		"docs/pitfalls.yaml": "data:\n  pitfalls:\n    - title: P\n      tags: [render-engine, ghost]\n      body: ok\n",
	})
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
			testsupport.WithTags("render-engine", "bogus"), testsupport.WithTitle("0001: A"),
			testsupport.WithBody("## Context\nx\n")))
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.checkTagVocabulary()
	if err != nil {
		t.Fatalf("checkTagVocabulary: %v", err)
	}
	got := map[string]string{}
	for _, d := range drift {
		got[d.Kind] = d.Detail
	}
	if len(drift) != 3 || !strings.Contains(got["adr-tag"], "bogus") ||
		!strings.Contains(got["pitfall-tag"], "ghost") || !strings.Contains(got["tag-vocabulary"], "empty") {
		t.Fatalf("want adr-tag(bogus)+pitfall-tag(ghost)+tag-vocabulary(empty), got %#v", drift)
	}
}

// An empty/absent vocabulary makes the membership rule inert.
func TestCheckTagVocabularyInert(t *testing.T) {
	root := scaffold(t, "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
			testsupport.WithTags("anything"), testsupport.WithTitle("0001: A"),
			testsupport.WithBody("## Context\nx\n")))
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.checkTagVocabulary()
	if err != nil || drift != nil {
		t.Fatalf("empty vocabulary must be inert, got %#v / %v", drift, err)
	}
}

// With a non-empty vocabulary but the pitfalls doc disabled, checkTagVocabulary
// proceeds past the ADR loop and pitfallTagEntries short-circuits to no entries;
// a conforming ADR yields no drift.
func TestCheckTagVocabularyPitfallsDisabled(t *testing.T) {
	root := scaffold(t, "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\ntags:\n  rendering: the render engine\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
			testsupport.WithTags("rendering"), testsupport.WithTitle("0001: A"),
			testsupport.WithBody("## Context\nx\n")))
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.checkTagVocabulary()
	if err != nil || drift != nil {
		t.Fatalf("conforming ADR with pitfalls disabled must yield no drift, got %#v / %v", drift, err)
	}
}

// A dangling ADR related: number yields adr-related-link drift; a resolving one
// yields none. Unconditional (no vocabulary configured here).
// invariant: adr-related-link-resolved
func TestCheckADRRelatedLinks(t *testing.T) {
	root := scaffold(t, "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
			testsupport.WithRelated(1, 42), testsupport.WithTitle("0001: A"),
			testsupport.WithBody("## Context\nx\n")))
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.checkADRRelatedLinks()
	if err != nil {
		t.Fatalf("checkADRRelatedLinks: %v", err)
	}
	if len(drift) != 1 || drift[0].Kind != "adr-related-link" || !strings.Contains(drift[0].Detail, "0042") {
		t.Fatalf("want one adr-related-link(0042) drift, got %#v", drift)
	}
}

// The two methods' adr.ParseDir branches are reachable via direct calls (they
// are pre-empted only inside full Check() by checkPlans), so they are tested,
// not coverage-ignored — mirroring TestCheckPitfallsADRParseError.
func TestCheckTagVocabularyADRParseError(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\ntags:\n  rendering: x\n", nil)
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-broken.md"),
		"---\nstatus: [unterminated\n---\n# ADR-0001: Broken\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.checkTagVocabulary(); err == nil {
		t.Fatal("expected adr.ParseDir error, got nil")
	}
}

func TestCheckADRRelatedLinksParseError(t *testing.T) {
	root := scaffold(t, "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-broken.md"),
		"---\nstatus: [unterminated\n---\n# ADR-0001: Broken\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.checkADRRelatedLinks(); err == nil {
		t.Fatal("expected adr.ParseDir error, got nil")
	}
}

// checkTagVocabulary's pitfallTagEntries branch surfaces a malformed pitfalls
// sidecar (valid ADRs so ParseDir succeeds first; non-empty vocabulary so the
// method proceeds past the len==0 guard) — reachable, tested not ignored.
func TestCheckTagVocabularyPitfallStructuralError(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: [pitfalls]\ndomains: []\ntags:\n  rendering: x\n",
		map[string]string{"docs/pitfalls.yaml": "data:\n  pitfalls: just a string\n"})
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
			testsupport.WithTitle("0001: A"), testsupport.WithBody("## Context\nx\n")))
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.checkTagVocabulary(); err == nil || !strings.Contains(err.Error(), "must be a list") {
		t.Fatalf("expected pitfalls structural error, got %v", err)
	}
}

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

// TestCheckPlansCommitSubjectDrift covers the ```commit length/type/shape drift and
// confirms an unknown scope is NOT drift (it is an advisory note instead).
// invariant: plan-commit-subject-length-checked
// invariant: plan-commit-subject-shape-checked
func TestCheckPlansCommitSubjectDrift(t *testing.T) {
	root := scaffold(t, commitSubjectCfg)
	p, err := Open(root)
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

	drift, err := p.checkPlans()
	if err != nil {
		t.Fatalf("checkPlans: %v", err)
	}
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
// invariant: plan-commit-subject-scope-advisory
func TestPlanCommitScopeNotes(t *testing.T) {
	root := scaffold(t, commitSubjectCfg)
	p, err := Open(root)
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

	notes, err := p.planCommitScopeNotes()
	if err != nil {
		t.Fatalf("planCommitScopeNotes: %v", err)
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "2026-07-14-scope.md") || !strings.Contains(notes[0], "disallowed scope") {
		t.Fatalf("want one scope note, got %#v", notes)
	}

	// A malformed plan makes ParseDir fail.
	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-07-14-broken.md"),
		"---\nstatus: [unterminated\n---\n# Plan: Broken\n")
	if _, err := p.planCommitScopeNotes(); err == nil {
		t.Fatal("expected ParseDir error for malformed frontmatter, got nil")
	}
}

// TestAdvisoryNotesSurfacesPlanCommitError covers the planCommitScopeNotes error
// propagation wired into AdvisoryNotes. Empty tags keep tagHealthNotes inert (so it
// does not error first); a malformed plan makes planCommitScopeNotes' ParseDir fail.
func TestAdvisoryNotesSurfacesPlanCommitError(t *testing.T) {
	root := scaffold(t, "prefix: awf\nskills: []\nagents: []\ndocs: []\ndomains: []\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-07-14-broken.md"),
		"---\nstatus: [unterminated\n---\n# Plan: Broken\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.AdvisoryNotes(); err == nil {
		t.Fatal("expected AdvisoryNotes to surface the plan-commit ParseDir error")
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

// A vocabulary member equal to a configured domain name is the coarse-tag
// regression, gated exactly; inert when no domains are configured.
// invariant: tag-not-domain-name
func TestCheckTagVocabularyDomainCollision(t *testing.T) {
	root := scaffold(t, "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: [rendering]\n"+
		"tags:\n  rendering: coarse\n  narrow: a narrow topic\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.checkTagVocabulary()
	if err != nil {
		t.Fatal(err)
	}
	var got string
	for _, d := range drift {
		if d.Kind == "tag-domain-collision" {
			got = d.Detail
		}
	}
	if !strings.Contains(got, "rendering") {
		t.Fatalf("want tag-domain-collision for rendering, got %+v", drift)
	}
	// No domains configured: the collision rule is inert.
	root2 := scaffold(t, "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\n"+
		"tags:\n  rendering: fine when no domains\n")
	p2, err := Open(root2)
	if err != nil {
		t.Fatal(err)
	}
	drift2, err := p2.checkTagVocabulary()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range drift2 {
		if d.Kind == "tag-domain-collision" {
			t.Errorf("no collision expected with no domains; got %+v", drift2)
		}
	}
}
