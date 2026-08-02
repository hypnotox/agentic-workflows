package project

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/plan"
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

func mustParsePlans(t *testing.T, p *Project) []plan.Plan {
	t.Helper()
	plans, err := plan.ParseDir(filepath.Join(p.Root, p.Cfg.DocsDir, "plans"))
	if err != nil {
		t.Fatalf("parse plans: %v", err)
	}
	return plans
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
// invariant: rendering/doc-outputs:pitfall-domains-resolved (TestCheckPitfallsValidatesDomainsAndLinks)
// invariant: rendering/doc-outputs:pitfall-adr-link-resolved (TestCheckPitfallsValidatesDomainsAndLinks)
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

const glossaryCheckCfg = "prefix: example\nintegrationBranch: main\nvars: {}\nskills: []\nagents: []\ndocs: [glossary]\ndomains: [rendering]\n"

// A disabled glossary doc is never read, so it can yield no drift.
func TestCheckGlossaryDisabled(t *testing.T) {
	p, err := Open(testContext(t), scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\nskills: []\nagents: []\ndocs: []\n"))
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.checkGlossary()
	if err != nil || drift != nil {
		t.Errorf("disabled glossary must yield no drift, got %v / %v", drift, err)
	}
}

// A record naming an unconfigured domain yields glossary-domain drift; records
// resolving their domains, and records carrying none, yield nothing.
// invariant: rendering/doc-outputs:glossary-domains-resolved (TestCheckGlossaryValidatesDomains)
func TestCheckGlossaryValidatesDomains(t *testing.T) {
	p, err := Open(testContext(t), scaffoldFiles(t, glossaryCheckCfg, map[string]string{
		"docs/glossary.yaml": "data:\n  terms:\n" +
			"    - term: clean\n      meaning: resolves\n      domains: [rendering]\n" +
			"    - term: untagged\n      meaning: no domains at all\n" +
			"    - term: bad\n      meaning: names a domain the project does not configure\n      domains: [bogus]\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.checkGlossary()
	if err != nil {
		t.Fatalf("checkGlossary: %v", err)
	}
	if len(drift) != 1 || drift[0].Kind != "glossary-domain" ||
		!strings.Contains(drift[0].Detail, "bogus") || !strings.Contains(drift[0].Detail, "bad") ||
		drift[0].Path != glossarySidecarPath {
		t.Fatalf("want one glossary-domain drift naming term bad and domain bogus, got %#v", drift)
	}
	// Drive the public surface too: the helper finding it is worth nothing if
	// Check drops the slice on the floor. Check reads the lock, so sync first.
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	full, err := p.Check(testContext(t))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !slices.ContainsFunc(full, func(d manifest.Drift) bool {
		return d.Kind == "glossary-domain" && strings.Contains(d.Detail, "bogus")
	}) {
		t.Fatalf("glossary-domain drift did not reach Check's result: %#v", full)
	}
}

// Valid YAML with a bad data.terms shape surfaces the structural error.
func TestCheckGlossaryStructuralError(t *testing.T) {
	p, err := Open(testContext(t), scaffoldFiles(t, glossaryCheckCfg, map[string]string{
		"docs/glossary.yaml": "data:\n  terms: just a string\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.checkGlossary(); err == nil || !strings.Contains(err.Error(), "must be a list") {
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
// invariant: config/configuration:tag-vocabulary-governed (TestCheckTagVocabulary)
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
// invariant: adr-system/adr-lifecycle:adr-related-link-resolved (TestCheckADRRelatedLinks)
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
// invariant: adr-system/adr-lifecycle:adr-related-ascending (TestCheckADRRelatedAscending)
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
// frontmatter-less (grandfathered) plan is skipped. A slug entry resolves
// against a pending record and drifts when it names none (ADR-0202 item 14).
// invariant: adr-system/plan-artifacts:plan-frontmatter-validated (TestCheckPlansValidatesFrontmatterAndLinks)
// invariant: adr-system/plan-artifacts:plan-adr-link-resolved (TestCheckPlansValidatesFrontmatterAndLinks)
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
	// One pending record and one already-numbered record that retained its slug,
	// so both slug resolution paths the claim names are exercised.
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/still-pending.md"), pendingADRFixture("still-pending"))
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0002-was-pending.md"),
		strings.Replace(pendingADRFixture("was-pending"), "# ADR-was-pending:", "# ADR-0002:", 1))

	write("2026-07-12-good.md", "---\ndate: 2026-07-12\nadrs: [1]\nstatus: Proposed\n---\n# Plan: Good\n")
	write("2026-07-12-bad-link.md", "---\ndate: 2026-07-12\nadrs: [42]\nstatus: Proposed\n---\n# Plan: Bad Link\n")
	write("2026-07-12-bad-status.md", "---\ndate: 2026-07-12\nadrs: [1]\nstatus: Draft\n---\n# Plan: Bad Status\n")
	write("2026-07-12-slug-link.md", "---\ndate: 2026-07-12\nadrs: [still-pending]\nstatus: Proposed\n---\n# Plan: Slug Link\n")
	write("2026-07-12-retained-slug-link.md", "---\ndate: 2026-07-12\nadrs: [was-pending]\nstatus: Proposed\n---\n# Plan: Retained Slug Link\n")
	write("2026-07-12-bad-slug-link.md", "---\ndate: 2026-07-12\nadrs: [never-authored]\nstatus: Proposed\n---\n# Plan: Bad Slug Link\n")
	write("2026-06-24-legacy.md", "# Plan: Legacy\n\nNo frontmatter, grandfathered.\n")

	drift := p.checkPlans(mustDeriveCorpus(t, p), mustParsePlans(t, p))

	got := map[string]string{}
	for _, d := range drift {
		got[d.Kind+"@"+filepath.Base(d.Path)] = d.Detail
	}
	if len(drift) != 3 {
		t.Fatalf("expected exactly 3 drifts (bad-link, bad-slug-link, bad-status), got %d: %#v", len(drift), drift)
	}
	if d, ok := got["plan-adr-link@2026-07-12-bad-link.md"]; !ok || d != "ADR-0042" {
		t.Errorf("expected plan-adr-link ADR-0042 drift, got %#v", drift)
	}
	if d, ok := got["plan-adr-link@2026-07-12-bad-slug-link.md"]; !ok || d != "ADR-never-authored" {
		t.Errorf("expected plan-adr-link ADR-never-authored drift, got %#v", drift)
	}
	if _, ok := got["plan-adr-link@2026-07-12-slug-link.md"]; ok {
		t.Errorf("slug link to a pending record must resolve, got %#v", drift)
	}
	if _, ok := got["plan-adr-link@2026-07-12-retained-slug-link.md"]; ok {
		t.Errorf("slug link to a numbered record's retained slug must resolve, got %#v", drift)
	}
	if _, ok := got["plan-frontmatter@2026-07-12-bad-status.md"]; !ok {
		t.Errorf("expected plan-frontmatter drift for bad status, got %#v", drift)
	}

	structured := `---
format: plan-v1
date: 2026-07-12
adrs: []
status: Proposed
---
# Plan: Structured

## Goal

Validate frontmatter.

## Architecture summary

Keep parsing in internal/plan.

## Phase 1: Check

**Execution mode: inline.**

### Task 1.1: Check marker

Parse the marker.

### Phase close

Run the gate.

` + "```commit\ntest(plans): validate frontmatter\n```" + `

## Definition of done

- Frontmatter is validated.
`
	for _, status := range []string{"Proposed", "Implemented"} {
		t.Run("plan-v1 "+status, func(t *testing.T) {
			dir := t.TempDir()
			testsupport.WriteFile(t, filepath.Join(dir, "2026-07-12-structured.md"), strings.Replace(structured, "status: Proposed", "status: "+status, 1))
			plans, err := plan.ParseDir(dir)
			if err != nil || len(plans) != 1 || plans[0].Format != "plan-v1" || plans[0].Status != status {
				t.Fatalf("ParseDir plans=%#v err=%v", plans, err)
			}
		})
	}
	for _, tc := range []struct{ name, body, detail string }{
		{"empty format", strings.Replace(structured, "format: plan-v1", `format: ""`, 1), "format must be a nonempty string"},
		{"unknown format", strings.Replace(structured, "format: plan-v1", "format: plan-v2", 1), "format must be exactly plan-v1"},
		{"duplicate format", strings.Replace(structured, "format: plan-v1", "format: plan-v1\nformat: plan-v1", 1), "duplicate format"},
		{"malformed format", strings.Replace(structured, "format: plan-v1", "format: [plan-v1]", 1), "format must be a nonempty string"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			testsupport.WriteFile(t, filepath.Join(dir, "2026-07-12-structured.md"), tc.body)
			_, err := plan.ParseDir(dir)
			var diagnostic *plan.Diagnostic
			if !errors.As(err, &diagnostic) || diagnostic.Category != "frontmatter" || diagnostic.Detail != tc.detail {
				t.Fatalf("diagnostic=%#v err=%v", diagnostic, err)
			}
		})
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
// not evidence that the record is in the wrong place (ADR-0202 item 7).
// invariant: adr-system/adr-lifecycle:pending-blocked-from-integration-branch (TestCheckPendingADRsFiresOnlyOnPositiveIdentification)
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
// invariant: adr-system/adr-lifecycle:pending-blocked-from-integration-branch (TestCheckReportsPendingADROnIntegrationBranch)
func TestCheckReportsPendingADROnIntegrationBranch(t *testing.T) {
	root := gitScaffold(t, defaultFixtureBranch)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	// Two records, because the claim quantifies over EVERY pending record: a
	// worktree that authored several is the case ADR-0202 item 8 orders by
	// argument, and reporting only the first would leave the rest to be
	// discovered one integration attempt at a time.
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/still-pending.md"), pendingADRFixture("still-pending"))
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/also-pending.md"), pendingADRFixture("also-pending"))
	drift, err := p.Check(testContext(t))
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
// invariant: adr-system/adr-lifecycle:pending-blocked-from-integration-branch (TestCheckPendingADRsSilentOnProbeFailure)
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

func parseCheckSource(t *testing.T) *ast.File {
	t.Helper()
	path := filepath.Join(testsupport.RepoRoot(t), "internal/project/check.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func checkFunc(t *testing.T, file *ast.File, name string) *ast.FuncDecl {
	t.Helper()
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return fn
		}
	}
	t.Fatalf("check.go has no %s function", name)
	return nil
}

func calledMethodCount(fn *ast.FuncDecl, name string) int {
	count := 0
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == name {
			count++
		}
		return true
	})
	return count
}

func hasOutputPlanParameter(fn *ast.FuncDecl) bool {
	for _, field := range fn.Type.Params.List {
		ptr, ok := field.Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		ident, ok := ptr.X.(*ast.Ident)
		if !ok || ident.Name != "OutputPlan" {
			continue
		}
		for _, name := range field.Names {
			if name.Name == "op" {
				return true
			}
		}
	}
	return false
}

func callsMethodWithIdent(fn *ast.FuncDecl, method, argument string) bool {
	found := false
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != method {
			return true
		}
		for _, arg := range call.Args {
			if ident, ok := arg.(*ast.Ident); ok && ident.Name == argument {
				found = true
			}
		}
		return true
	})
	return found
}

func TestCheckReportBuildsOneOutputPlan(t *testing.T) {
	file := parseCheckSource(t)
	report := checkFunc(t, file, "CheckReport")
	check := checkFunc(t, file, "checkWithState")
	advisory := checkFunc(t, file, "advisoryNotesWithState")

	if got := calledMethodCount(report, "outputPlan"); got != 1 {
		t.Errorf("CheckReport constructs %d output plans, want exactly one", got)
	}
	for _, fn := range []*ast.FuncDecl{check, advisory} {
		if !hasOutputPlanParameter(fn) {
			t.Errorf("%s does not receive op *OutputPlan", fn.Name.Name)
		}
		if got := calledMethodCount(fn, "outputPlan"); got != 0 {
			t.Errorf("%s reconstructs %d output plans", fn.Name.Name, got)
		}
		if !callsMethodWithIdent(report, fn.Name.Name, "op") {
			t.Errorf("CheckReport does not pass op to %s", fn.Name.Name)
		}
	}
	for _, producer := range []string{"generateDomainDocs", "generateConfigReference"} {
		if got := calledMethodCount(advisory, producer); got != 0 {
			t.Errorf("advisoryNotesWithState calls %s %d times, want plan write nodes", producer, got)
		}
	}

	root := scaffoldFiles(t,
		"prefix: example\nintegrationBranch: main\nvars: {}\nskills: []\nagents: []\ndomains: [config]\n",
		map[string]string{
			"parts/config-reference/intro.md": "<!-- awf:stub -->\nConfig intro.\n<!-- awf:section bogus -->\n",
		})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	reportValue, err := p.CheckReport(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(reportValue.Notes, "\n")
	for _, want := range []string{
		"docs/domains/config.md has unauthored stub content",
		"docs/config-reference.md has unauthored stub content: stub-marked parts: intro",
	} {
		if got := strings.Count(joined, want); got != 2 {
			t.Errorf("CheckReport notes contain planned write node %q %d times, want compatibility multiplicity 2:\n%s", want, got, joined)
		}
	}
	marker := "part .awf/parts/config-reference/intro.md contains a marker-shaped line"
	if got := strings.Count(joined, marker); got != 1 {
		t.Errorf("CheckReport marker note multiplicity = %d, want deduplicated 1:\n%s", got, joined)
	}
}

func TestCheckReportParsesPlansOnce(t *testing.T) {
	source, err := os.ReadFile(filepath.Join(testsupport.RepoRoot(t), "internal/project/check.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	checkStart := strings.Index(text, "func (p *Project) CheckReport")
	checkEnd := strings.Index(text, "\n// Check is the compatibility projection")
	privateStart := strings.Index(text, "func (p *Project) checkWithState")
	if checkStart < 0 || checkEnd <= checkStart || privateStart < 0 {
		t.Fatal("check.go source boundaries changed")
	}
	if got := strings.Count(text[checkStart:checkEnd], "plan.ParseDir("); got != 1 {
		t.Fatalf("CheckReport has %d plan.ParseDir calls, want exactly one", got)
	}
	if strings.Contains(text[privateStart:], "plan.ParseDir(") {
		t.Fatal("a private check or advisory consumer reparses plans")
	}
	if got := strings.Count(text, "plan.ParseDir("); got != 2 {
		t.Fatalf("check.go has %d plan.ParseDir calls, want CheckReport plus compatibility AdvisoryNotes", got)
	}
}

// TestCheckReportMapsPlanDiagnostics proves malformed plan frontmatter reaches
// the stable drift channel while valid sibling plans remain checkable.
func TestCheckReportMapsPlanDiagnostics(t *testing.T) {
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
	report, err := p.CheckReport(testContext(t))
	if err != nil {
		t.Fatalf("CheckReport: %v", err)
	}
	if !slices.ContainsFunc(report.Drift, func(d manifest.Drift) bool {
		return d.Path == "docs/plans/2026-07-12-broken.md" && d.Kind == "plan-frontmatter" && strings.Contains(d.Detail, "yaml")
	}) {
		t.Fatalf("plan diagnostic did not reach drift: %#v", report.Drift)
	}
}

func TestCheckReportPropagatesPlanDirectoryReadError(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(root, "docs/plans/2026-07-12-directory.md")
	if err := os.Mkdir(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := p.CheckReport(testContext(t)); err == nil || !strings.Contains(err.Error(), "read 2026-07-12-directory.md") {
		t.Fatalf("CheckReport error = %v, want plan read failure", err)
	}
}

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

	drift := p.checkPlans(mustDeriveCorpus(t, p), mustParsePlans(t, p))
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

	notes := p.planCommitScopeNotes(mustParsePlans(t, p))
	if len(notes) != 1 || !strings.Contains(notes[0], "2026-07-14-scope.md") || !strings.Contains(notes[0], "disallowed scope") {
		t.Fatalf("want one scope note, got %#v", notes)
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

// TestCheckProjectsPlanDiagnostics proves Check's compatibility projection
// exposes malformed plan frontmatter as drift rather than a process error.
func TestCheckProjectsPlanDiagnostics(t *testing.T) {
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
	drift, err := p.Check(testContext(t))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !slices.ContainsFunc(drift, func(d manifest.Drift) bool { return d.Kind == "plan-frontmatter" }) {
		t.Fatalf("Check omitted plan-frontmatter drift: %#v", drift)
	}
}

// A vocabulary member equal to a configured domain name is the coarse-tag
// regression, gated exactly; inert when no domains are configured.
// invariant: config/validation:tag-not-domain-name (TestCheckTagVocabularyDomainCollision)
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

// The glossary sibling of TestCheckPropagatesLocalPitfallsError: a local: true
// glossary sidecar is skipped by the render pass before its data.terms transform
// runs, so a structurally invalid record list reaches Check's checkGlossary
// wiring branch rather than failing earlier in the render.
func TestCheckPropagatesLocalGlossaryError(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nvars: {}\nskills: []\nagents: []\ndocs: [glossary]\ndomains: []\n",
		map[string]string{"docs/glossary.yaml": "data:\n  terms:\n    - term: t\n      meaning: m\n"})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, ".awf/docs/glossary.yaml"), "local: true\ndata:\n  terms: just a string\n")
	reopened, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.Check(testContext(t)); err == nil || !strings.Contains(err.Error(), "must be a list") {
		t.Fatalf("expected Check to propagate the local glossary structural error, got %v", err)
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
// fails at corpus load: duplicate identity has one home (ADR-0202 item 4), so
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

// A first adoption accepts an existing intrinsically governed record without
// rewriting it or using its number to select a parser.
func TestInitializeReportAcceptsBrownfieldGovernedRecord(t *testing.T) {
	root := scaffold(t, sampleYAML)
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions", "0001-governed.md"),
		"---\nformat: current-state-v2\nstatus: Proposed\ndate: 2026-07-13\n---\n# ADR-0001: Governed\n\n## Context\n\nC.\n\n## Decision\n\n1. D.\n\n## State changes\n\nNone.\n\n## Consequences\n\nC.\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-07-13: Proposed\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, "docs/decisions", "0001-governed.md"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatalf("initialize governed brownfield: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(root, "docs/decisions", "0001-governed.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("initialization rewrote the existing ADR")
	}
}
