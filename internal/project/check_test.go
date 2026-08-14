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
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// mustDeriveCorpus derives the operation-owned ADR corpus the way a lifecycle
// entry does, so a helper test exercises the same threaded value production
// passes it (ADR-0180).
func mustDeriveCorpus(t *testing.T, p *Project) adr.Corpus {
	t.Helper()
	corpus, _, _, _, err := p.deriveOperationStateWithPitfalls()
	if err != nil {
		t.Fatalf("derive operation state: %v", err)
	}
	return corpus
}

// mustDeriveTopics derives the operation-owned topic corpus the same way.
func mustDeriveTopics(t *testing.T, p *Project) topic.Corpus {
	t.Helper()
	_, _, topics, _, err := p.deriveOperationStateWithPitfalls()
	if err != nil {
		t.Fatalf("derive operation state: %v", err)
	}
	return topics
}

// mustDeriveSkills derives the operation-owned effective skill set the same way.
func mustDeriveSkills(t *testing.T, p *Project) map[string]bool {
	t.Helper()
	_, _, _, eff, err := p.deriveOperationStateWithPitfalls()
	if err != nil {
		t.Fatalf("derive operation state: %v", err)
	}
	return eff
}

func mustParsePlans(t *testing.T, p *Project) []plan.Plan {
	t.Helper()
	plans, err := plan.ParseDir(filepath.Join(p.Root, config.DocsDir, "plans"))
	if err != nil {
		t.Fatalf("parse plans: %v", err)
	}
	return plans
}

const pitfallsCheckCfg = "prefix: example\nintegrationBranch: main\nvars: {}\ndomains: [rendering]\n"

const commitSubjectCfg = "prefix: example\nintegrationBranch: main\nvars: {}\ndomains: []\naudit:\n  allowedScopes:\n    - name: awf\n"

// An empty unconditional pitfall corpus yields no project-level drift.
func TestCheckPitfallsEmpty(t *testing.T) {
	p, err := Open(testContext(t), scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.checkPitfalls(mustDeriveCorpus(t, p))
	if err != nil || drift != nil {
		t.Errorf("empty pitfalls must yield no drift, got %v / %v", drift, err)
	}
}

// An unknown domain yields pitfall-domain drift, a dangling related ADR yields
// pitfall-adr-link drift, and an entry resolving both yields none.
// invariant: rendering/doc-outputs:pitfall-domains-resolved (TestCheckPitfallsValidatesDomainsAndLinks)
// invariant: rendering/doc-outputs:pitfall-adr-link-resolved (TestCheckPitfallsValidatesDomainsAndLinks)
func TestCheckPitfallsValidatesDomainsAndLinks(t *testing.T) {
	root := scaffoldFiles(t, pitfallsCheckCfg, map[string]string{
		"docs/pitfalls/clean.md":      pitfallSource("Clean", "domains: [rendering]\nrelated: [1]\n", "ok\n"),
		"docs/pitfalls/bad-domain.md": pitfallSource("BadDomain", "domains: [bogus]\n", "ok\n"),
		"docs/pitfalls/bad-link.md":   pitfallSource("BadLink", "related: [42]\n", "ok\n"),
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

// A malformed authored source is a hard corpus-load error before check projection.
func TestCheckPitfallsStructuralError(t *testing.T) {
	p, err := Open(testContext(t), scaffoldFiles(t, pitfallsCheckCfg, map[string]string{
		"docs/pitfalls/bad.md": "---\ntitle: Bad\nunknown: value\n---\nbody\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.loadPitfallCorpus(); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("expected structural error, got %v", err)
	}
}

const glossaryCheckCfg = "prefix: example\nintegrationBranch: main\nvars: {}\ndomains: [rendering]\n"

// A disabled glossary doc is never read, so it can yield no drift.
func TestCheckGlossaryDisabled(t *testing.T) {
	p, err := Open(testContext(t), scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\n"))
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
	if _, _, _, _, err := p.deriveOperationStateWithPitfalls(); err == nil {
		t.Fatal("expected adr.ParseDir error for malformed frontmatter, got nil")
	}
}

// A non-member tag on an ADR or a pitfall yields tag drift; an empty-meaning
// member yields tag-vocabulary drift; a fully-conforming corpus yields none.
// invariant: config/configuration:tag-vocabulary-governed (TestCheckTagVocabulary)
func TestCheckTagVocabulary(t *testing.T) {
	cfg := "prefix: example\nintegrationBranch: main\nvars: {}\ndomains: [rendering]\n" +
		"tags:\n  render-engine: the render engine\n  empty: \"\"\n"
	root := scaffoldFiles(t, cfg, map[string]string{
		"docs/pitfalls/p.md": pitfallSource("P", "tags: [render-engine, ghost]\n", "ok\n"),
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
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\ndomains: []\n")
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
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\ndomains: []\ntags:\n  rendering: the render engine\n")
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
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\ndomains: []\n")
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
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\ndomains: []\n")
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

// Direct vocabulary projection derives and rejects a malformed corpus source.
func TestCheckTagVocabularyPitfallStructuralError(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nvars: {}\ndomains: []\ntags:\n  rendering: x\n",
		map[string]string{"docs/pitfalls/bad.md": "---\ntitle: Bad\nunknown: value\n---\nbody\n"})
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
			testsupport.WithTitle("0001: A"), testsupport.WithBody("## Context\nx\n")))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := adr.NewCorpus(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.checkTagVocabulary(corpus); err == nil || !strings.Contains(err.Error(), "unknown") {
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
		{"unknown format", strings.Replace(structured, "format: plan-v1", "format: plan-v3", 1), "format must be exactly plan-v1 or plan-v2"},
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

func calledMethodPosition(fn *ast.FuncDecl, name string) token.Pos {
	var position token.Pos
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == name && position == token.NoPos {
			position = call.Pos()
		}
		return true
	})
	return position
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

// invariant: rendering/project-output-plan:check-report-single-plan (TestCheckReportBuildsOneOutputPlan)
func TestCheckReportBuildsOneOutputPlan(t *testing.T) {
	file := parseCheckSource(t)
	report := checkFunc(t, file, "CheckReport")
	directAdvisory := checkFunc(t, file, "AdvisoryNotes")
	check := checkFunc(t, file, "checkWithTrackingState")
	advisory := checkFunc(t, file, "advisoryNotesWithState")

	for _, fn := range []*ast.FuncDecl{report, directAdvisory} {
		if got := calledMethodCount(fn, "outputPlanWithPitfalls"); got != 1 {
			t.Errorf("%s constructs %d output plans, want exactly one", fn.Name.Name, got)
		}
	}
	outputPlanPosition := calledMethodPosition(report, "outputPlanWithPitfalls")
	for _, producer := range []string{"deriveOperationStateWithPitfalls", "ParseDir"} {
		producerPosition := calledMethodPosition(report, producer)
		if producerPosition == token.NoPos || outputPlanPosition == token.NoPos || outputPlanPosition <= producerPosition {
			t.Errorf("CheckReport outputPlan position %d must follow %s position %d", outputPlanPosition, producer, producerPosition)
		}
	}
	if !callsMethodWithIdent(directAdvisory, "advisoryNotesWithState", "op") {
		t.Error("AdvisoryNotes does not pass op to advisoryNotesWithState")
	}
	for _, fn := range []*ast.FuncDecl{check, advisory} {
		if !hasOutputPlanParameter(fn) {
			t.Errorf("%s does not receive op *OutputPlan", fn.Name.Name)
		}
		if got := calledMethodCount(fn, "outputPlanWithPitfalls"); got != 0 {
			t.Errorf("%s reconstructs %d output plans", fn.Name.Name, got)
		}
		if !callsMethodWithIdent(report, fn.Name.Name, "op") {
			t.Errorf("CheckReport does not pass op to %s", fn.Name.Name)
		}
	}
	for _, fn := range []*ast.FuncDecl{check, advisory} {
		for _, producer := range []string{"generateDomainDocs", "generateConfigReference"} {
			if got := calledMethodCount(fn, producer); got != 0 {
				t.Errorf("%s calls %s %d times, want plan write nodes", fn.Name.Name, producer, got)
			}
		}
	}

	root := scaffoldFiles(t,
		"prefix: example\nintegrationBranch: main\nvars: {}\ndomains: [config]\n",
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
	directNotes, err := p.AdvisoryNotes(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	for name, notes := range map[string][]string{"CheckReport": reportValue.Notes, "AdvisoryNotes": directNotes} {
		joined := strings.Join(notes, "\n")
		for _, want := range []string{
			"docs/domains/config.md has unauthored stub content",
			"docs/config-reference.md has unauthored stub content: stub-marked parts: intro",
		} {
			if got := strings.Count(joined, want); got != 2 {
				t.Errorf("%s notes contain planned write node %q %d times, want compatibility multiplicity 2:\n%s", name, want, got, joined)
			}
		}
		marker := "part .awf/parts/config-reference/intro.md contains a marker-shaped line"
		if got := strings.Count(joined, marker); got != 1 {
			t.Errorf("%s marker note multiplicity = %d, want deduplicated 1:\n%s", name, got, joined)
		}
	}
}

func TestCheckReportRequiresGeneratedArtifactsInIndex(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	testsupport.WriteAwfConfig(t, root, withTestGateCmd("prefix: example\nintegrationBranch: main\nvars: {}\n"))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.syncReport(testContext(t), &InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	gitfixture.AddAll(t, repo)
	gitfixture.StageRemoval(t, repo, "AGENTS.md", ".awf/awf.lock")

	report, err := p.CheckReport(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, finding := range report.Drift {
		if finding.Kind == "untracked" {
			got = append(got, finding.Path)
		}
		if (finding.Path == "AGENTS.md" || finding.Path == ".awf/awf.lock") && finding.Kind == "missing" {
			t.Fatalf("missing duplicates untracked root cause: %#v", report.Drift)
		}
	}
	if joined := strings.Join(got, ","); joined != ".awf/awf.lock,AGENTS.md" {
		t.Fatalf("untracked findings = %q", joined)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "index"), []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.CheckReport(testContext(t)); err == nil {
		t.Fatal("corrupt tracking index accepted")
	}
}

func TestCheckLockedFilesSuppressesMissingForUntrackedOutputs(t *testing.T) {
	root := t.TempDir()
	p := &Project{roots: resident.NewRoots(root, root)}
	rendered := map[string]RenderedFile{
		"regen.md":  {Path: "regen.md", Content: "regen", Policy: OutputPolicy{Regenerate: true}},
		"normal.md": {Path: "normal.md", Content: "normal"},
	}
	lock := &manifest.Lock{Files: map[string]manifest.Entry{
		"regen.md":  {},
		"normal.md": {OutputHash: manifest.Hash([]byte("normal"))},
	}}
	tracking := []manifest.Drift{{Path: "regen.md", Kind: "untracked"}, {Path: "normal.md", Kind: "untracked"}}
	if got := p.checkLockedFiles(lock, rendered, tracking); len(got) != 0 {
		t.Fatalf("untracked missing files = %#v", got)
	}
}

func TestCheckGeneratedTrackingNoGitAndNestedResidentExclusion(t *testing.T) {
	t.Run("no Git", func(t *testing.T) {
		p := &Project{Root: t.TempDir()}
		_, notes, err := p.checkGeneratedTracking(testContext(t), &OutputPlan{})
		if err != nil || len(notes) != 1 || !strings.Contains(notes[0], "unavailable outside a Git repository") {
			t.Fatalf("no-Git tracking = notes %q, err %v", notes, err)
		}
	})
	t.Run("nested resident output", func(t *testing.T) {
		fixture := gitfixture.InitRepo(t)
		root := filepath.Join(fixture.Root(), "nested")
		gitfixture.Stage(t, fixture, map[string]string{
			"nested/.awf/awf.lock": "lock\n",
			"nested/tracked.md":    "tracked\n",
		})
		repo, _, err := awfgit.OpenContaining(root)
		if err != nil {
			t.Fatal(err)
		}
		p := &Project{Root: root, nested: true, repo: repo}
		op := &OutputPlan{Nodes: []OutputNode{
			{file: &RenderedFile{Path: "tracked.md"}},
			{file: &RenderedFile{Path: ".awf/efforts/.gitignore"}},
		}}
		drift, _, err := p.checkGeneratedTracking(testContext(t), op)
		if err != nil || len(drift) != 0 {
			t.Fatalf("nested tracking = %#v, %v", drift, err)
		}
	})
}

func TestCheckReportParsesPlansOnce(t *testing.T) {
	source, err := os.ReadFile(filepath.Join(testsupport.RepoRoot(t), "internal/project/check.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	checkStart := strings.Index(text, "func (p *Project) CheckReport")
	checkEnd := strings.Index(text, "\nfunc finishCheckReport")
	privateStart := strings.Index(text, "func (p *Project) checkWithTrackingState")
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
	root := scaffold(t, "prefix: awf\nintegrationBranch: main\ndomains: []\n")
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
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\ndomains: [rendering]\n"+
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
	root2 := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\ndomains: []\n"+
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
// invariant: adr-system/plan-artifacts:plan-v2-assignment-advisories (TestPlanV2AssignmentAdvisories)
func TestPlanV2AssignmentAdvisories(t *testing.T) {
	plans := []plan.Plan{{Filename: "2026-08-02-v2.md", Path: "docs/plans/2026-08-02-v2.md", Format: "plan-v2", Status: "Proposed", DoD: []plan.DoDItem{{Slug: "complete"}}, Phases: []plan.Phase{{Number: 1, Tasks: []plan.Task{{Number: 1, Fields: plan.TaskFields{}}}}}}}
	drift, notes := planArtifactReport(plans, adr.Corpus{})
	if len(drift) != 0 || len(notes) != 1 || !strings.Contains(notes[0], "no outcome assignment") {
		t.Fatalf("planArtifactReport = drift %#v, notes %#v", drift, notes)
	}
	// A Decision assignment in one Proposed plan cannot cover another plan.
	source := "---\nformat: current-state-v4\nstatus: Proposed\ndate: 2026-08-02\nslug: fixture\n---\n# ADR-fixture: Fixture\n\n## Context\n\nContext.\n\n## Decision\n\n1. `decision: first` First.\n\n## State changes\n\nNone.\n\n## Consequences\n\nNone.\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-08-02: Proposed\n"
	record, err := adr.ParseV4("fixture.md", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := adr.NewCorpus([]adr.ADR{record})
	if err != nil {
		t.Fatal(err)
	}
	assigned := plan.Plan{
		Filename: "assigned.md", Path: "docs/plans/assigned.md", Format: "plan-v2", Status: "Proposed",
		ADRs: []plan.ADRLink{{Slug: "fixture"}},
		Phases: []plan.Phase{{
			Number: 1,
			Tasks: []plan.Task{{
				Number: 1,
				Fields: plan.TaskFields{Applying: []plan.DecisionRef{{
					Authored: "fixture:first", ADR: "fixture", Selector: "first", Kind: "Applying",
				}}},
			}},
		}},
	}
	missing := assigned
	missing.Filename, missing.Path = "missing.md", "docs/plans/missing.md"
	missing.Phases = append([]plan.Phase(nil), assigned.Phases...)
	missing.Phases[0].Tasks = append([]plan.Task(nil), assigned.Phases[0].Tasks...)
	missing.Phases[0].Tasks[0].Fields.Applying = nil
	drift, notes = planArtifactReport([]plan.Plan{assigned, missing}, corpus)
	if len(drift) != 0 || !slices.Contains(notes, "missing.md Decision fixture:first has no Applying assignment") {
		t.Fatalf("independent assignments = drift %#v, notes %#v", drift, notes)
	}
}

func TestPlanArtifactReportFindsReferencesAndSortsNotes(t *testing.T) {
	p := plan.Plan{
		Filename: "2026-08-02-v2.md", Path: "docs/plans/2026-08-02-v2.md", Format: "plan-v2", Status: "Proposed",
		ADRs: []plan.ADRLink{{Slug: "missing"}}, DoD: []plan.DoDItem{{Slug: "one"}, {Slug: "two"}},
		Phases: []plan.Phase{
			{Number: 1, Tasks: []plan.Task{{Number: 1, Fields: plan.TaskFields{Applying: []plan.DecisionRef{{Authored: "missing:item", ADR: "missing", Selector: "item", Kind: "Applying"}}}}}},
			{Number: 2, Advances: []string{"one"}, Tasks: []plan.Task{{Number: 1, Fields: plan.TaskFields{Kind: plan.TaskSpike}}}},
		},
	}
	drift, notes := planArtifactReport([]plan.Plan{p}, adr.Corpus{})
	if len(drift) != 2 || !strings.Contains(drift[0].Detail, "ADR not found") || !strings.Contains(drift[1].Detail, "ADR not found") {
		t.Fatalf("hard findings = %#v", drift)
	}
	if !slices.IsSorted(notes) || len(notes) != 2 || !strings.Contains(strings.Join(notes, "\n"), "advanced but has no Completes") || !strings.Contains(strings.Join(notes, "\n"), "no outcome assignment") {
		t.Fatalf("notes = %#v", notes)
	}
}

func TestPlanContextHelpersRejectMissingReferencesAndSelectors(t *testing.T) {
	p := plan.Plan{Filename: "v2.md", Phases: []plan.Phase{{Number: 1, Tasks: []plan.Task{{Number: 1}}}}}
	if _, _, err := selectedRefs(p, "9"); err == nil {
		t.Fatal("missing selector accepted")
	}
	phase, task, err := selectedRefs(p, "1")
	if err != nil || phase.Number != 1 || task.Number != 0 {
		t.Fatalf("phase selected refs = %#v %#v %v", phase, task, err)
	}
	phase, task, err = selectedRefs(p, "1.1")
	if err != nil || phase.Number != 1 || task.Number != 1 {
		t.Fatalf("selected refs = %#v %#v %v", phase, task, err)
	}
	task.Fields.Applying = []plan.DecisionRef{{Authored: "missing:item", ADR: "missing", Selector: "item", Kind: "Applying"}}
	_, _, err = resolveSelectedPlanDecisions(p, adr.Corpus{}, phase, task)
	if err == nil || !strings.Contains(err.Error(), "ADR not found") {
		t.Fatalf("missing reference = %v", err)
	}
}

func TestPlanArtifactReportEnforcesDecisionReferenceContracts(t *testing.T) {
	source := "---\nformat: current-state-v4\nstatus: Proposed\ndate: 2026-08-02\nslug: fixture\n---\n# ADR-fixture: Fixture\n\n## Context\n\nContext.\n\n## Decision\n\n1. `decision: first` First.\n\n## State changes\n\nNone.\n\n## Consequences\n\nNone.\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-08-02: Proposed\n"
	record, err := adr.ParseV4("fixture.md", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := adr.NewCorpus([]adr.ADR{record})
	if err != nil {
		t.Fatal(err)
	}
	p := plan.Plan{
		Filename: "v2.md", Path: "docs/plans/v2.md", Format: "plan-v2", ADRs: []plan.ADRLink{{Slug: "other"}},
		Phases: []plan.Phase{{Number: 1, Tasks: []plan.Task{{Number: 1, Fields: plan.TaskFields{
			Applying: []plan.DecisionRef{{Authored: "fixture:first", ADR: "fixture", Selector: "first", Kind: "Applying"}},
		}}}}},
	}
	drift, _ := planArtifactReport([]plan.Plan{p}, corpus)
	if len(drift) != 2 || !strings.Contains(drift[0].Detail, "ADR not found") || !strings.Contains(drift[1].Detail, "Applying ADR is absent from adrs") {
		t.Fatalf("Applying membership drift = %#v", drift)
	}
	p.ADRs = []plan.ADRLink{{Slug: "fixture"}}
	p.Phases[0].Tasks[0].Fields.Applying = nil
	p.Phases[0].Tasks[0].Fields.Context = []plan.DecisionRef{{Authored: "fixture:missing", ADR: "fixture", Selector: "missing", Kind: "Context"}}
	drift, _ = planArtifactReport([]plan.Plan{p}, corpus)
	if len(drift) != 1 || !strings.Contains(drift[0].Detail, "Context requires frozen ADR") {
		t.Fatalf("context freeze drift = %#v", drift)
	}
}

// invariant: adr-system/plan-artifacts:plan-v2-decision-references (TestResolvePlanDecisionsUsesFrozenCorpusIdentityAndSelector)
func TestResolvePlanDecisionsUsesFrozenCorpusIdentityAndSelector(t *testing.T) {
	v4Source := "---\nformat: current-state-v4\nstatus: Proposed\ndate: 2026-08-02\nslug: fixture\n---\n# ADR-0001: Fixture\n\n## Context\n\nContext.\n\n## Decision\n\n1. `decision: first` First.\n\n2. `decision: zeta` Zeta.\n\n## State changes\n\nNone.\n\n## Consequences\n\nNone.\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-08-02: Proposed\n"
	v4, err := adr.ParseV4("0001-fixture.md", []byte(v4Source))
	if err != nil {
		t.Fatal(err)
	}
	v3Source := strings.Replace(v4Source, "current-state-v4", "current-state-v3", 1)
	v3Source = strings.Replace(v3Source, "1. `decision: first` First.\n\n2. `decision: zeta` Zeta.", "1. First.\n\n2. Zeta.", 1)
	v3, err := adr.ParseV3("0001-fixture.md", []byte(v3Source))
	if err != nil {
		t.Fatal(err)
	}

	resolve := func(record adr.ADR, link plan.ADRLink, ref plan.DecisionRef, context, selectorError bool) ([]plan.ResolvedDecision, error) {
		t.Helper()
		corpus, err := adr.NewCorpus([]adr.ADR{record})
		if err != nil {
			t.Fatal(err)
		}
		p := plan.Plan{Filename: "v2.md", Path: "docs/plans/v2.md", Format: "plan-v2", ADRs: []plan.ADRLink{link}, Phases: []plan.Phase{{Number: 1, Tasks: []plan.Task{{Number: 1, Fields: plan.TaskFields{Applying: []plan.DecisionRef{ref}}}}}}}
		if context {
			p.Phases[0].Tasks[0].Fields.Applying = nil
			p.Phases[0].Tasks[0].Fields.Context = []plan.DecisionRef{ref}
		}
		drift, _ := planArtifactReport([]plan.Plan{p}, corpus)
		switch {
		case context && record.IsContentAmendable():
			if len(drift) != 1 || !strings.Contains(drift[0].Detail, "Context requires frozen ADR") {
				t.Fatalf("amendable Context drift = %#v", drift)
			}
		case !selectorError && len(drift) != 0:
			t.Fatalf("planArtifactReport drift = %#v", drift)
		case selectorError && len(drift) != 1:
			t.Fatalf("selector planArtifactReport drift = %#v", drift)
		}
		phase, task := p.Phases[0], p.Phases[0].Tasks[0]
		applying, selectedContext, err := resolveSelectedPlanDecisions(p, corpus, phase, task)
		if context {
			return selectedContext, err
		}
		return applying, err
	}

	// Numbered links and retained-slug task references resolve the same V4 ADR,
	// and the reverse spelling produces the same projection record.
	applySlug := plan.DecisionRef{Authored: "fixture:first", ADR: "fixture", Selector: "first", Kind: "Applying"}
	applyNumber := plan.DecisionRef{Authored: "0001:first", ADR: "0001", Selector: "first", Kind: "Applying"}
	bySlug, err := resolve(v4, plan.ADRLink{Number: 1}, applySlug, false, false)
	if err != nil || len(bySlug) != 1 {
		t.Fatalf("number link / slug ref = %#v, %v", bySlug, err)
	}
	byNumber, err := resolve(v4, plan.ADRLink{Slug: "fixture"}, applyNumber, false, false)
	if err != nil || len(byNumber) != 1 || bySlug[0] != byNumber[0] {
		t.Fatalf("slug link / number ref = %#v, %v; want %#v", byNumber, err, bySlug)
	}

	// V4 decision IDs are stable while content is amendable, but Context needs a
	// frozen record. Copies model lifecycle states without invalidating fixtures.
	contextRef := plan.DecisionRef{Authored: "fixture:first", ADR: "fixture", Selector: "first", Kind: "Context"}
	if _, err := resolve(v4, plan.ADRLink{Slug: "fixture"}, contextRef, true, false); err == nil || !strings.Contains(err.Error(), "requires frozen ADR") {
		t.Fatalf("amendable V4 Context error = %v", err)
	}
	frozenV4 := v4
	frozenV4.Status = "Implemented"
	if resolved, err := resolve(frozenV4, plan.ADRLink{Slug: "fixture"}, contextRef, true, false); err != nil || len(resolved) != 1 {
		t.Fatalf("frozen V4 Context = %#v, %v", resolved, err)
	}
	frozenCorpus, err := adr.NewCorpus([]adr.ADR{frozenV4})
	if err != nil {
		t.Fatal(err)
	}
	contextTask := plan.Task{Number: 1, Fields: plan.TaskFields{Context: []plan.DecisionRef{contextRef}}}
	contextPhase := plan.Phase{Number: 1, Tasks: []plan.Task{contextTask}}
	contextPlan := plan.Plan{Filename: "v2.md", ADRs: []plan.ADRLink{{Slug: "fixture"}}, Phases: []plan.Phase{contextPhase}}
	applying, selectedContext, err := resolveSelectedPlanDecisions(contextPlan, frozenCorpus, contextPhase, contextTask)
	if err != nil || len(applying) != 0 || len(selectedContext) != 1 || selectedContext[0].Key != "0001:first" {
		t.Fatalf("selected frozen V4 Context = applying %#v, context %#v, err %v", applying, selectedContext, err)
	}
	amendableV4Corpus, err := adr.NewCorpus([]adr.ADR{v4})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := resolveSelectedPlanDecisions(contextPlan, amendableV4Corpus, contextPhase, contextTask); err == nil || !strings.Contains(err.Error(), "task 1.1 Context \"fixture:first\": requires frozen ADR") {
		t.Fatalf("selected amendable Context error = %v", err)
	}
	missingContextRef := plan.DecisionRef{Authored: "fixture:missing", ADR: "fixture", Selector: "missing", Kind: "Context"}
	missingContextTask := plan.Task{Number: 1, Fields: plan.TaskFields{Context: []plan.DecisionRef{missingContextRef}}}
	missingContextPhase := plan.Phase{Number: 1, Tasks: []plan.Task{missingContextTask}}
	_, _, err = resolveSelectedPlanDecisions(contextPlan, frozenCorpus, missingContextPhase, missingContextTask)
	if !errors.Is(err, adr.ErrDecisionSelectorUnknown) || !strings.Contains(err.Error(), "task 1.1 Context \"fixture:missing\"") {
		t.Fatalf("selected Context selector error = %v", err)
	}

	frozenV3 := v3
	frozenV3.Status = "Implemented"
	ordinal := plan.DecisionRef{Authored: "fixture:#1", ADR: "fixture", Selector: "#1", Kind: "Applying"}
	if resolved, err := resolve(frozenV3, plan.ADRLink{Slug: "fixture"}, ordinal, false, false); err != nil || len(resolved) != 1 {
		t.Fatalf("frozen pre-V4 ordinal = %#v, %v", resolved, err)
	}
	amendableV3, err := adr.NewCorpus([]adr.ADR{v3})
	if err != nil {
		t.Fatal(err)
	}
	amendablePlan := plan.Plan{Filename: "v2.md", Path: "docs/plans/v2.md", Format: "plan-v2", ADRs: []plan.ADRLink{{Slug: "fixture"}}, Phases: []plan.Phase{{Number: 1, Tasks: []plan.Task{{Number: 1, Fields: plan.TaskFields{Applying: []plan.DecisionRef{ordinal}}}}}}}
	if drift, _ := planArtifactReport([]plan.Plan{amendablePlan}, amendableV3); len(drift) != 1 || !strings.Contains(drift[0].Detail, adr.ErrDecisionSelectorAmendable.Error()) {
		t.Fatalf("amendable pre-V4 ordinal drift = %#v", drift)
	}
	if _, _, err := resolveSelectedPlanDecisions(amendablePlan, amendableV3, amendablePlan.Phases[0], amendablePlan.Phases[0].Tasks[0]); err == nil || !errors.Is(err, adr.ErrDecisionSelectorAmendable) {
		t.Fatalf("amendable pre-V4 ordinal error = %v", err)
	}

	assertSelector := func(selector string, cause error) {
		t.Helper()
		_, err := resolve(frozenV4, plan.ADRLink{Slug: "fixture"}, plan.DecisionRef{Authored: "fixture:" + selector, ADR: "fixture", Selector: selector, Kind: "Applying"}, false, true)
		var typed *adr.DecisionSelectorError
		if !errors.Is(err, cause) || !errors.As(err, &typed) || !slices.IsSorted(typed.Available) || strings.Join(typed.Available, ",") != "first,zeta" {
			t.Fatalf("selector %q error = %#v", selector, err)
		}
	}
	assertSelector("#1", adr.ErrDecisionSelectorIncompatible)
	assertSelector("missing", adr.ErrDecisionSelectorUnknown)
}

func TestPlanArtifactReportValidatesSelectorsAndAssignments(t *testing.T) {
	source := "---\nformat: current-state-v4\nstatus: Proposed\ndate: 2026-08-02\nslug: fixture\n---\n# ADR-fixture: Fixture\n\n## Context\n\nContext.\n\n## Decision\n\n1. `decision: first` First.\n\n2. `decision: second` Second.\n\n## State changes\n\nNone.\n\n## Consequences\n\nNone.\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-08-02: Proposed\n"
	record, err := adr.ParseV4("fixture.md", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	record.Status = "Implemented"
	corpus, err := adr.NewCorpus([]adr.ADR{record})
	if err != nil {
		t.Fatal(err)
	}
	p := plan.Plan{Filename: "v2.md", Path: "docs/plans/v2.md", Format: "plan-v2", Status: "Proposed", ADRs: []plan.ADRLink{{Slug: "fixture"}}, DoD: []plan.DoDItem{{Slug: "advanced"}, {Slug: "complete"}}, Phases: []plan.Phase{{Number: 1, Advances: []string{"advanced"}, Completes: []string{"complete"}, Tasks: []plan.Task{{Number: 1, Fields: plan.TaskFields{Applying: []plan.DecisionRef{{Authored: "fixture:first", ADR: "fixture", Selector: "first", Kind: "Applying"}}, Context: []plan.DecisionRef{{Authored: "fixture:missing", ADR: "fixture", Selector: "missing", Kind: "Context"}}}}, {Number: 2}}}}}
	drift, notes := planArtifactReport([]plan.Plan{{Format: "plan-v1"}, p}, corpus)
	if len(drift) != 1 || !strings.Contains(drift[0].Detail, "fixture:missing") || len(notes) != 3 || !strings.Contains(strings.Join(notes, "\n"), "task 1.2 has no Applying") || !strings.Contains(strings.Join(notes, "\n"), "fixture:second has no Applying") || !strings.Contains(strings.Join(notes, "\n"), "advanced but has no Completes") {
		t.Fatalf("plan artifact report = drift %#v, notes %#v", drift, notes)
	}
}

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

// invariant: rendering/sync-and-drift:agent-guide-size-advisory (TestAgentGuideSizeAdvisoryBoundary)
func TestAgentGuideSizeAdvisoryBoundary(t *testing.T) {
	for _, tc := range []struct {
		name  string
		bytes int
		want  bool
	}{
		{name: "below", bytes: 12*1024 - 1},
		{name: "boundary", bytes: 12 * 1024},
		{name: "over", bytes: 12*1024 + 1, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nvars: {}\naudit:\n  allowedScopes:\n    - name: awf\n", map[string]string{"parts/agents-doc/identity.md": "x"})
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			op, err := p.OutputPlan(testContext(t))
			if err != nil {
				t.Fatal(err)
			}
			var actual int
			for _, file := range op.writeFiles() {
				if file.Path == "AGENTS.md" {
					actual = len(file.Content)
				}
			}
			testsupport.WriteFile(t, filepath.Join(root, ".awf/parts/agents-doc/identity.md"), strings.Repeat("x", tc.bytes-actual+1))
			p, err = Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			op, err = p.OutputPlan(testContext(t))
			if err != nil {
				t.Fatal(err)
			}
			for _, file := range op.writeFiles() {
				if file.Path == "AGENTS.md" && len(file.Content) != tc.bytes {
					t.Fatalf("expected guide bytes = %d, want %d", len(file.Content), tc.bytes)
				}
			}
			if err := p.Sync(); err != nil {
				t.Fatal(err)
			}
			if tc.want {
				testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-07-14-scope.md"),
					"---\ndate: 2026-07-14\nadrs: []\nstatus: Proposed\n---\n# Plan: Scope\n\n```commit\nfeat(nope): unknown scope\n```\n")
			}
			report, err := p.CheckReport(testContext(t))
			if err != nil {
				t.Fatal(err)
			}
			var notes []string
			for _, note := range report.Notes {
				if strings.Contains(note, "AGENTS.md") && strings.Contains(note, "12288") {
					notes = append(notes, note)
				}
			}
			if tc.want {
				if len(notes) != 1 || !strings.Contains(notes[0], "12289") || !strings.Contains(notes[0], "docs/agents-md-standard.md") {
					t.Fatalf("overage note = %#v", notes)
				}
				ordinaryIndex := slices.IndexFunc(report.Notes, func(note string) bool { return strings.Contains(note, "disallowed scope") })
				sizeIndex := slices.Index(report.Notes, notes[0])
				if ordinaryIndex < 0 || sizeIndex < 0 || ordinaryIndex >= sizeIndex {
					t.Fatalf("CheckReport notes do not place ordinary advisory before size advisory: %#v", report.Notes)
				}
				direct, err := p.AdvisoryNotes(testContext(t))
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(strings.Join(direct, "\n"), "12288") {
					t.Fatalf("AdvisoryNotes included aggregate-only size note: %#v", direct)
				}
				for _, resident := range []string{"missing", "stale"} {
					if resident == "missing" {
						if err := os.Remove(filepath.Join(root, "AGENTS.md")); err != nil {
							t.Fatal(err)
						}
					} else {
						testsupport.WriteFile(t, filepath.Join(root, "AGENTS.md"), "stale")
					}
					residentReport, err := p.CheckReport(testContext(t))
					if err != nil {
						t.Fatal(err)
					}
					if got := strings.Count(strings.Join(residentReport.Notes, "\n"), "12289"); got != 1 {
						t.Fatalf("%s resident size notes = %d, want 1: %#v", resident, got, residentReport.Notes)
					}
				}
				return
			}
			if len(notes) != 0 {
				t.Fatalf("notes = %#v, want none", notes)
			}
		})
	}
}
