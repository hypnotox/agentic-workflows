package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// mustDeriveCorpus derives the operation-owned ADR corpus the way a lifecycle
// entry does, so a helper test exercises the same threaded value production
// passes it (ADR-0180).
func mustDeriveCorpus(t *testing.T, p *Project) adr.Corpus {
	t.Helper()
	corpus, _, _, err := p.deriveOperationState()
	if err != nil {
		t.Fatalf("derive operation state: %v", err)
	}
	return corpus
}

// mustDeriveTopics derives the operation-owned topic corpus the same way.
func mustDeriveTopics(t *testing.T, p *Project) topic.Corpus {
	t.Helper()
	_, topics, _, err := p.deriveOperationState()
	if err != nil {
		t.Fatalf("derive operation state: %v", err)
	}
	return topics
}

// mustDeriveSkills derives the operation-owned effective skill set the same way.
func mustDeriveSkills(t *testing.T, p *Project) map[string]bool {
	t.Helper()
	_, _, eff, err := p.deriveOperationState()
	if err != nil {
		t.Fatalf("derive operation state: %v", err)
	}
	return eff
}

const pitfallsCheckCfg = "prefix: example\nintegrationBranch: main\nvars: {}\nskills: []\nagents: []\ndocs: [pitfalls]\ndomains: [rendering]\n"

const commitSubjectCfg = "prefix: example\nintegrationBranch: main\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\naudit:\n  allowedScopes:\n    - name: awf\n"

// A disabled pitfalls doc yields no drift and never reads the sidecar.
func TestCheckPitfallsDisabled(t *testing.T) {
	p, err := Open(testContext(t), scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\nskills: []\nagents: []\ndocs: []\n"))
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.checkPitfalls(mustDeriveCorpus(t, p))
	if err != nil || drift != nil {
		t.Errorf("disabled pitfalls must yield no drift, got %v / %v", drift, err)
	}
}

// An unknown domain yields pitfall-domain drift, a dangling related ADR yields
// pitfall-adr-link drift, and an entry resolving both yields none.
// invariant: rendering/doc-outputs:pitfall-domains-resolved
// invariant: rendering/doc-outputs:pitfall-adr-link-resolved
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
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.checkPitfalls(mustDeriveCorpus(t, p))
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
	p, err := Open(testContext(t), scaffoldFiles(t, pitfallsCheckCfg, map[string]string{
		"docs/pitfalls.yaml": "data:\n  pitfalls: just a string\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.checkPitfalls(mustDeriveCorpus(t, p)); err == nil || !strings.Contains(err.Error(), "must be a list") {
		t.Fatalf("expected structural error, got %v", err)
	}
}

// A malformed ADR aborts the check via adr.ParseDir.
func TestDeriveOperationStateSurfacesMalformedADR(t *testing.T) {
	root := scaffoldFiles(t, pitfallsCheckCfg, map[string]string{
		"docs/pitfalls.yaml": "data:\n  pitfalls:\n    - title: T\n      body: ok\n",
	})
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-broken.md"),
		"---\nstatus: [unterminated\n---\n# ADR-0001: Broken\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.deriveOperationState(); err == nil {
		t.Fatal("expected adr.ParseDir error for malformed frontmatter, got nil")
	}
}

// A non-member tag on an ADR or a pitfall yields tag drift; an empty-meaning
// member yields tag-vocabulary drift; a fully-conforming corpus yields none.
// invariant: config/configuration:tag-vocabulary-governed
func TestCheckTagVocabulary(t *testing.T) {
	cfg := "prefix: example\nintegrationBranch: main\nvars: {}\nskills: []\nagents: []\ndocs: [pitfalls]\ndomains: [rendering]\n" +
		"tags:\n  render-engine: the render engine\n  empty: \"\"\n"
	root := scaffoldFiles(t, cfg, map[string]string{
		"docs/pitfalls.yaml": "data:\n  pitfalls:\n    - title: P\n      tags: [render-engine, ghost]\n      body: ok\n",
	})
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
			testsupport.WithTags("render-engine", "bogus"), testsupport.WithTitle("0001: A"),
			testsupport.WithBody("## Context\nx\n")))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.checkTagVocabulary(mustDeriveCorpus(t, p))
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
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
			testsupport.WithTags("anything"), testsupport.WithTitle("0001: A"),
			testsupport.WithBody("## Context\nx\n")))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.checkTagVocabulary(mustDeriveCorpus(t, p))
	if err != nil || drift != nil {
		t.Fatalf("empty vocabulary must be inert, got %#v / %v", drift, err)
	}
}

// With a non-empty vocabulary but the pitfalls doc disabled, checkTagVocabulary
// proceeds past the ADR loop and pitfallTagEntries short-circuits to no entries;
// a conforming ADR yields no drift.
func TestCheckTagVocabularyPitfallsDisabled(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\ntags:\n  rendering: the render engine\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
			testsupport.WithTags("rendering"), testsupport.WithTitle("0001: A"),
			testsupport.WithBody("## Context\nx\n")))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.checkTagVocabulary(mustDeriveCorpus(t, p))
	if err != nil || drift != nil {
		t.Fatalf("conforming ADR with pitfalls disabled must yield no drift, got %#v / %v", drift, err)
	}
}

// A dangling ADR related: number yields adr-related-link drift; a resolving one
// yields none. Unconditional (no vocabulary configured here).
// invariant: adr-system/adr-lifecycle:adr-related-link-resolved
func TestCheckADRRelatedLinks(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
			testsupport.WithRelated(1, 42), testsupport.WithTitle("0001: A"),
			testsupport.WithBody("## Context\nx\n")))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift := p.checkADRRelatedLinks(mustDeriveCorpus(t, p))
	if len(drift) != 1 || drift[0].Kind != "adr-related-link" || !strings.Contains(drift[0].Detail, "0042") {
		t.Fatalf("want one adr-related-link(0042) drift, got %#v", drift)
	}
}

// Every clause of the slug is exercised: a descending array reports
// adr-related-order and an ascending one does not; the finding names the FIRST
// descent; and resolution stays independent of ordering, so a descending array
// still has every entry checked against the corpus. The last two are what the
// separate-loops implementation exists for - a merged loop that aborts the
// resolution scan at the first descent, or a missing break, each passes a test
// that only checks the simple case.
// invariant: adr-system/adr-lifecycle:adr-related-ascending
func TestCheckADRRelatedAscending(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\n")
	write := func(name, title string, related ...int) {
		opts := []testsupport.ADROption{testsupport.WithDate("2026-07-13"),
			testsupport.WithTitle(title), testsupport.WithBody("## Context\nx\n")}
		if len(related) > 0 {
			opts = append(opts, testsupport.WithRelated(related...))
		}
		testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/"+name),
			testsupport.ADR("Accepted", opts...))
	}
	write("0001-a.md", "0001: A", 42, 2)        // one descent, every entry resolves
	write("0002-b.md", "0002: B", 1, 42)        // ascending
	write("0003-c.md", "0003: C", 42, 2, 77)    // descent AND a dangling entry after it
	write("0004-d.md", "0004: D", 42, 2, 43, 3) // two descents; only the first is reported
	write("0042-e.md", "0042: E")
	write("0043-f.md", "0043: F")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift := p.checkADRRelatedLinks(mustDeriveCorpus(t, p))
	kinds := map[string][]string{}
	for _, d := range drift {
		kinds[d.Kind] = append(kinds[d.Kind], d.Detail)
	}
	order, link := kinds["adr-related-order"], kinds["adr-related-link"]

	// 0002 ascends and must contribute nothing; 0001, 0003 and 0004 each
	// contribute exactly one ordering finding.
	if len(order) != 3 {
		t.Fatalf("want three adr-related-order findings (0001, 0003, 0004), got %d: %q", len(order), order)
	}
	joined := strings.Join(order, "\n")
	if strings.Contains(joined, "ADR-0002") {
		t.Errorf("an ascending array must not be reported, got %q", joined)
	}
	for _, want := range []string{"ADR-0001", "ADR-0003", "ADR-0004"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing ordering finding for %s in %q", want, joined)
		}
	}

	// The dangling entry sits AFTER the descent in 0003, so a loop that stops
	// scanning at the first descent would never reach it.
	if len(link) != 1 || !strings.Contains(link[0], "ADR-0077") {
		t.Errorf("a descent must not suppress the dangling-link scan for later entries, got %q", link)
	}

	// 0004 descends twice; exactly one finding, naming the first descent.
	var d4 string
	for _, d := range order {
		if strings.Contains(d, "ADR-0004") {
			d4 = d
		}
	}
	if !strings.Contains(d4, "descends at 2 after 42") {
		t.Errorf("finding must name the first descent (2 after 42), got %q", d4)
	}
	if strings.Contains(d4, "after 43") {
		t.Errorf("only the first descent is reported, got %q", d4)
	}
}

// checkTagVocabulary's pitfallTagEntries branch surfaces a malformed pitfalls
// sidecar (valid ADRs so ParseDir succeeds first; non-empty vocabulary so the
// method proceeds past the len==0 guard) - reachable, tested not ignored.
func TestCheckTagVocabularyPitfallStructuralError(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nvars: {}\nskills: []\nagents: []\ndocs: [pitfalls]\ndomains: []\ntags:\n  rendering: x\n",
		map[string]string{"docs/pitfalls.yaml": "data:\n  pitfalls: just a string\n"})
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
			testsupport.WithTitle("0001: A"), testsupport.WithBody("## Context\nx\n")))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.checkTagVocabulary(mustDeriveCorpus(t, p)); err == nil || !strings.Contains(err.Error(), "must be a list") {
		t.Fatalf("expected pitfalls structural error, got %v", err)
	}
}

// TestCheckPlansValidatesFrontmatterAndLinks exercises checkPlans over a
// docs/plans/ set: a plan linking a nonexistent ADR yields plan-adr-link drift,
// a bad status: yields plan-frontmatter drift, a valid plan yields none, and a
// frontmatter-less (grandfathered) plan is skipped.
// invariant: adr-system/plan-artifacts:plan-frontmatter-validated
// invariant: adr-system/plan-artifacts:plan-adr-link-resolved
func TestCheckPlansValidatesFrontmatterAndLinks(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
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

	drift, err := p.checkPlans(mustDeriveCorpus(t, p))
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

// The pending-record block fires on a positive integration-branch
// identification and on nothing else: another branch, a detached HEAD, and a
// tree with no readable repository all pass, because an indeterminate answer is
// not evidence that the record is in the wrong place (ADR-0194 item 7).
// invariant: adr-system/adr-lifecycle:pending-blocked-from-integration-branch
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
			drift := p.checkPendingADRs(testContext(t), mustDeriveCorpus(t, p))
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
// invariant: adr-system/adr-lifecycle:pending-blocked-from-integration-branch
func TestCheckReportsPendingADROnIntegrationBranch(t *testing.T) {
	root := gitScaffold(t, defaultFixtureBranch)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/still-pending.md"), pendingADRFixture("still-pending"))
	drift, err := p.Check(testContext(t))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	found := false
	for _, d := range drift {
		if d.Kind == "pending-adr-on-integration-branch" && d.Detail == "still-pending" {
			found = true
		}
	}
	if !found {
		t.Fatalf("awf check did not report the pending record: %#v", drift)
	}
}

// A branch probe that fails outright is the fourth indeterminate outcome, and
// it is distinct from the no-repository one: the handle exists, the git call
// itself errors. Removing the control directory under a live handle is the way
// to produce it; the block must stay silent rather than report a record it has
// no evidence is misplaced.
// invariant: adr-system/adr-lifecycle:pending-blocked-from-integration-branch
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
	if drift := p.checkPendingADRs(testContext(t), corpus); len(drift) != 1 {
		t.Fatalf("fixture does not block before the probe breaks: %#v", drift)
	}
	if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	if drift := p.checkPendingADRs(testContext(t), corpus); len(drift) != 0 {
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
	if drift := p.checkPendingADRs(testContext(t), mustDeriveCorpus(t, p)); len(drift) != 0 {
		t.Fatalf("a numbered corpus must not be blocked, got %#v", drift)
	}
}

// TestCheckPlansPropagatesPlanParseError covers checkPlans' plan.ParseDir error
// branch: malformed plan frontmatter is a hard error (the unparseable-YAML half
// of plan-frontmatter-validated), not silent drift.
func TestCheckPlansPropagatesPlanParseError(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-07-12-broken.md"),
		"---\nstatus: [unterminated\n---\n# Plan: Broken\n")
	if _, err := p.checkPlans(mustDeriveCorpus(t, p)); err == nil {
		t.Fatal("expected plan.ParseDir error for malformed frontmatter, got nil")
	}
}

// TestCheckPlansCommitSubjectDrift covers the ```commit length/type/shape drift and
// confirms an unknown scope is NOT drift (it is an advisory note instead).
// invariant: adr-system/plan-artifacts:plan-commit-subject-length-checked
// invariant: adr-system/plan-artifacts:plan-commit-subject-shape-checked
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

	drift, err := p.checkPlans(mustDeriveCorpus(t, p))
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
// invariant: adr-system/plan-artifacts:plan-commit-subject-scope-advisory
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
	root := scaffold(t, "prefix: awf\nintegrationBranch: main\nskills: []\nagents: []\ndocs: []\ndomains: []\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-07-14-broken.md"),
		"---\nstatus: [unterminated\n---\n# Plan: Broken\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.AdvisoryNotes(testContext(t)); err == nil {
		t.Fatal("expected AdvisoryNotes to surface the plan-commit ParseDir error")
	}
}

// TestCheckPropagatesPlanError covers Check's propagation of a checkPlans error:
// a synced, otherwise-clean project with a malformed plan makes full Check fail.
func TestCheckPropagatesPlanError(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-07-12-broken.md"),
		"---\nstatus: [unterminated\n---\n# Plan: Broken\n")
	if _, err := p.Check(testContext(t)); err == nil {
		t.Fatal("expected Check to propagate the checkPlans parse error, got nil")
	}
}

// A vocabulary member equal to a configured domain name is the coarse-tag
// regression, gated exactly; inert when no domains are configured.
// invariant: config/validation:tag-not-domain-name
func TestCheckTagVocabularyDomainCollision(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: [rendering]\n"+
		"tags:\n  rendering: coarse\n  narrow: a narrow topic\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.checkTagVocabulary(mustDeriveCorpus(t, p))
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
	root2 := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\n"+
		"tags:\n  rendering: fine when no domains\n")
	p2, err := Open(testContext(t), root2)
	if err != nil {
		t.Fatal(err)
	}
	drift2, err := p2.checkTagVocabulary(mustDeriveCorpus(t, p2))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range drift2 {
		if d.Kind == "tag-domain-collision" {
			t.Errorf("no collision expected with no domains; got %+v", drift2)
		}
	}
}

// A local: true pitfalls sidecar is skipped by the render pass before its
// data.pitfalls transform runs, but checkPitfalls reads it regardless, so a
// structurally invalid entry list reaches Check's wiring branch rather than
// failing earlier in the render.
func TestCheckPropagatesLocalPitfallsError(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nvars: {}\nskills: []\nagents: []\ndocs: [pitfalls]\ndomains: []\n",
		map[string]string{"docs/pitfalls.yaml": "data:\n  pitfalls:\n    - title: T\n      body: B\n"})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, ".awf/docs/pitfalls.yaml"), "local: true\ndata:\n  pitfalls: just a string\n")
	reopened, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.Check(testContext(t)); err == nil || !strings.Contains(err.Error(), "must be a list") {
		t.Fatalf("expected Check to propagate the local pitfalls structural error, got %v", err)
	}
}

// AdvisoryNotes and ConfigReferenceModel both forward the operation
// derivation's fault; a malformed ADR reaches each one's wiring branch.
func TestAdvisoryNotesAndConfigReferenceSurfaceMalformedADR(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-broken.md"),
		"---\nstatus: [unterminated\n---\n# ADR-0001: Broken\n")
	if _, err := p.AdvisoryNotes(testContext(t)); err == nil {
		t.Fatal("expected AdvisoryNotes to surface the malformed ADR, got nil")
	}
	if _, err := p.ConfigReferenceModel(testContext(t)); err == nil {
		t.Fatal("expected ConfigReferenceModel to surface the malformed ADR, got nil")
	}
}

// A first adoption whose decisions dir carries two ADRs with the same number
// fails at corpus load: duplicate identity has one home (ADR-0194 item 4), so
// the refusal precedes every consumer rather than being re-derived by each.
func TestInitializeReportSurfacesDuplicateADRIdentity(t *testing.T) {
	root := scaffold(t, sampleYAML)
	for _, name := range []string{"0001-alpha.md", "0001-beta.md"} {
		testsupport.WriteFile(t, filepath.Join(root, "docs/decisions", name),
			testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
				testsupport.WithTitle("0001: A"), testsupport.WithBody("## Context\nx\n")))
	}
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err == nil ||
		!strings.Contains(err.Error(), "ADR number 0001 is declared by more than one file") {
		t.Fatalf("expected duplicate ADR identity to fail the corpus load, got %v", err)
	}
}

// A first adoption whose decisions dir parses cleanly but declares a governed
// format inside the brownfield legacy set fails authority sealing, after the
// entry derivation succeeded.
func TestInitializeReportSurfacesBrownfieldGovernedRecord(t *testing.T) {
	root := scaffold(t, sampleYAML)
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions", "0001-governed.md"),
		"---\nformat: current-state-v2\nstatus: Proposed\ndate: 2026-07-13\n---\n# ADR-0001: Governed\n\n## Context\n\nC.\n\n## Decision\n\n1. D.\n\n## State changes\n\nNone.\n\n## Consequences\n\nC.\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-07-13: Proposed\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err == nil ||
		!strings.Contains(err.Error(), "seal first-adoption ADR authority") {
		t.Fatalf("expected a brownfield governed record to fail authority sealing, got %v", err)
	}
}
