package project

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// citeProject writes each named ADR body into a scaffolded decisions dir and
// opens the project. Fixtures are real files parsed by the real parser, so a
// test proves the whole path (parse -> Citations -> computeCitations) rather
// than the pure half alone.
func citeProject(t *testing.T, adrs map[string]string) *Project {
	t.Helper()
	root := scaffold(t, "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\n")
	for name, body := range adrs {
		testsupport.WriteFile(t, filepath.Join(root, "docs/decisions", name), body)
	}
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// citeDrift runs the check and returns only its findings.
func citeDrift(t *testing.T, p *Project) []manifest.Drift {
	t.Helper()
	drift, err := p.checkCitations()
	if err != nil {
		t.Fatalf("checkCitations: %v", err)
	}
	return drift
}

// citeTarget is a settled ADR others may cite, with an optional Invariants
// section.
func citeTarget(invariants string) string {
	body := "## Context\nx\n\n## Decision\n\n1. **A settled decision.** It says a thing.\n"
	if invariants != "" {
		body += "\n## Invariants\n\n" + invariants
	}
	return testsupport.ADR("Implemented", testsupport.WithDate("2026-07-13"),
		testsupport.WithTitle("0001: A"), testsupport.WithBody(body))
}

// citeCarrier is an ADR whose Decision section holds the text under test.
func citeCarrier(decision string) string {
	return testsupport.ADR("Implemented", testsupport.WithDate("2026-07-13"),
		testsupport.WithTitle("0002: B"),
		testsupport.WithBody("## Context\nx\n\n## Decision\n\n"+decision))
}

func detailsOf(drift []manifest.Drift) string {
	var b strings.Builder
	for _, d := range drift {
		b.WriteString(d.Detail)
		b.WriteString("\n")
	}
	return b.String()
}

// The check reads only `## Decision`. A verb and a citation together anywhere
// else are inert, which is what lets a Context section narrate an override
// history without owing a token. The positive control is load-bearing: without
// it the test passes just as well against a check that never fires at all.
// invariant: citation-check-decision-scoped
func TestCitationsDecisionScoped(t *testing.T) {
	outside := testsupport.ADR("Implemented", testsupport.WithDate("2026-07-13"),
		testsupport.WithTitle("0002: B"), testsupport.WithBody(
			"## Context\nThis supersedes ADR-0001 Decision item 1 outright.\n\n"+
				"## Decision\n\n1. **Something unrelated.** No citation here.\n\n"+
				"## Consequences\nIt also overrides ADR-0001 Decision item 2.\n"))
	p := citeProject(t, map[string]string{"0001-a.md": citeTarget(""), "0002-b.md": outside})
	if d := citeDrift(t, p); len(d) != 0 {
		t.Fatalf("a verb and citation outside ## Decision must be inert, got %s", detailsOf(d))
	}

	inside := citeCarrier("1. **Something.** This supersedes ADR-0001 Decision item 1 outright.\n")
	p = citeProject(t, map[string]string{"0001-a.md": citeTarget(""), "0002-b.md": inside})
	if d := citeDrift(t, p); len(d) != 1 {
		t.Fatalf("positive control: the same sentence inside ## Decision must fire once, got %s", detailsOf(d))
	}
}

// All six item-citation spellings the corpus uses resolve to the same anchor
// kind: the four bare shapes, plus the possessive and markdown-link wrappers
// ADR-0131 item 2 admits. A wrapper the grammar misses reads as covered while
// silently exempting every claim written that way.
// invariant: citation-check-item-shapes
func TestCitationsItemShapes(t *testing.T) {
	dec := "1. **It overrides several anchors.**\n" +
		"   ADR-0001 Decision item 1, ADR-0001 Decision 2, ADR-0001 item 3, ADR-0001 D4,\n" +
		"   ADR-0001's Decision item 5, and [ADR-0001](0001-a.md) Decision item 6.\n"
	p := citeProject(t, map[string]string{
		"0001-a.md": citeTarget(""),
		"0002-b.md": citeCarrier(dec),
	})
	got := detailsOf(citeDrift(t, p))
	for _, want := range []string{"ADR-0001#1", "ADR-0001#2", "ADR-0001#3", "ADR-0001#4", "ADR-0001#5", "ADR-0001#6"} {
		if !strings.Contains(got, want) {
			t.Errorf("shape for %s not recognized; findings were:\n%s", want, got)
		}
	}
	if n := strings.Count(got, "\n"); n != 6 {
		t.Errorf("want exactly six findings, got %d:\n%s", n, got)
	}
}

// Both slug spellings are citations when they sit outside `## Invariants`;
// inside it the same string is a declaration, parsed by declRe.
// invariant: citation-check-slug-spellings
func TestCitationsSlugSpellings(t *testing.T) {
	dec := "1. **It narrows the first.** ADR-0001 `inv: some-slug` is overridden here.\n\n" +
		"2. **It narrows it again.** ADR-0001 `invariant: some-slug` is overridden here too.\n"
	p := citeProject(t, map[string]string{
		"0001-a.md": citeTarget("- `invariant: some-slug`: the target's sentence.\n"),
		"0002-b.md": citeCarrier(dec),
	})
	// One finding per spelling, each at its own Decision item. Counting
	// findings, not anchor occurrences: a detail names the anchor three times,
	// once in the claim and twice in the token shapes it suggests.
	drift := citeDrift(t, p)
	got := detailsOf(drift)
	if len(drift) != 2 || !strings.Contains(got, "item 1") || !strings.Contains(got, "item 2") {
		t.Errorf("both `inv:` and `invariant:` spellings must be recognized, got:\n%s", got)
	}
}

// The verb list is enumerated, never stem-plus-suffix: the participles and
// nominalizations the corpus leans on must match, and the ordinary adjectives
// that share a stem must not. Both directions are asserted, since a rule that
// over-matches is as wrong as one that under-matches.
// invariant: citation-check-verb-forms
func TestCitationsVerbForms(t *testing.T) {
	fires := []string{"overridden", "overriding", "replacing", "supersedence", "amendment", "generalization"}
	inert := []string{"narrower", "narrowest", "overridable"}
	for _, v := range fires {
		p := citeProject(t, map[string]string{
			"0001-a.md": citeTarget(""),
			"0002-b.md": citeCarrier("1. **X.** ADR-0001 Decision item 1 is " + v + " by this.\n"),
		})
		if d := citeDrift(t, p); len(d) != 1 {
			t.Errorf("%q must arm the check, got %d findings", v, len(d))
		}
	}
	for _, w := range inert {
		p := citeProject(t, map[string]string{
			"0001-a.md": citeTarget(""),
			"0002-b.md": citeCarrier("1. **X.** ADR-0001 Decision item 1 is " + w + " than this.\n"),
		})
		if d := citeDrift(t, p); len(d) != 0 {
			t.Errorf("%q is ordinary description and must not arm the check, got %s", w, detailsOf(d))
		}
	}
}

// A Proposed target is still mutable, so its item numbers are not yet anchors
// and nothing about it is settled enough to supersede.
// invariant: citation-check-exempts-proposed-target
func TestCitationsExemptsProposedTarget(t *testing.T) {
	proposed := testsupport.ADR("Proposed", testsupport.WithDate("2026-07-13"),
		testsupport.WithTitle("0001: A"),
		testsupport.WithBody("## Context\nx\n\n## Decision\n\n1. **Still mutable.** x\n"))
	p := citeProject(t, map[string]string{
		"0001-a.md": proposed,
		"0002-b.md": citeCarrier("1. **X.** This supersedes ADR-0001 Decision item 1.\n"),
	})
	if d := citeDrift(t, p); len(d) != 0 {
		t.Fatalf("a Proposed target must be exempt, got %s", detailsOf(d))
	}
}

// An ADR citing its own items claims nothing about another decision, so it
// owes no token however it phrases the reference.
// invariant: citation-check-exempts-self-citation
func TestCitationsExemptsSelfCitation(t *testing.T) {
	p := citeProject(t, map[string]string{
		"0002-b.md": citeCarrier(
			"1. **X.** This supersedes ADR-0002 Decision item 2 of this very ADR.\n\n2. **Y.** x\n"),
	})
	if d := citeDrift(t, p); len(d) != 0 {
		t.Fatalf("a self-citation must be exempt, got %s", detailsOf(d))
	}
}

// A slug citation naming something the target never declares addresses no
// anchor: Anchor carries a slug only by string and has no ordinal form, so
// there is no token an author could write to satisfy it.
// invariant: citation-check-exempts-unslugged-bullet
func TestCitationsExemptsUnsluggedBullet(t *testing.T) {
	p := citeProject(t, map[string]string{
		"0001-a.md": citeTarget("- a prose bullet declaring no slug at all\n"),
		"0002-b.md": citeCarrier("1. **X.** ADR-0001 `inv: never-declared` is overridden here.\n"),
	})
	if d := citeDrift(t, p); len(d) != 0 {
		t.Fatalf("a slug the target never declares must be exempt, got %s", detailsOf(d))
	}
}

// An item citation or an override verb inside an inline code span is a
// specimen, not a claim, which is what lets an ADR discuss the citation grammar
// without self-triggering; ADR-0131's own Decision section depends on it. The
// exemption stops there. A slug citation's backticks ARE its syntax, carried as
// literal characters of the pattern that finds it, so it is scanned raw and
// masking would recognize none of them; it fires inside a span by construction,
// and an author quoting one writes `cites-invariant:`. A fenced block is
// excluded upstream, so that form exempts every anchor kind.
// invariant: citation-check-exempts-code-spans
func TestCitationsExemptsCodeSpans(t *testing.T) {
	t.Run("item citation inside a span is a specimen", func(t *testing.T) {
		p := citeProject(t, map[string]string{
			"0001-a.md": citeTarget(""),
			"0002-b.md": citeCarrier(
				"1. **Discussing the grammar.** A citation is written `ADR-0001 Decision item 1` and is\n" +
					"   overridden only when it appears as prose.\n"),
		})
		if d := citeDrift(t, p); len(d) != 0 {
			t.Fatalf("an item citation inside a code span must be exempt, got %s", detailsOf(d))
		}
	})

	t.Run("a verb inside a span does not arm the item", func(t *testing.T) {
		// Only the verb is quoted; the citation is bare prose. Nothing may fire.
		// This is the clause that keeps an ADR from arming itself by naming a
		// verb as data, which item 2 does for every form it enumerates.
		p := citeProject(t, map[string]string{
			"0001-a.md": citeTarget(""),
			"0002-b.md": citeCarrier(
				"1. **Naming a verb as data.** The word `overridden` is one of the listed forms, and\n" +
					"   ADR-0001 Decision item 1 is merely mentioned beside it.\n"),
		})
		if d := citeDrift(t, p); len(d) != 0 {
			t.Fatalf("a verb inside a code span must not arm the item, got %s", detailsOf(d))
		}
	})

	t.Run("a slug citation fires inside a span, and is exempt only when fenced", func(t *testing.T) {
		inv := "- `invariant: some-slug`: the target's sentence.\n"
		p := citeProject(t, map[string]string{
			"0001-a.md": citeTarget(inv),
			"0002-b.md": citeCarrier("1. **X.** ADR-0001 `inv: some-slug` is overridden here.\n"),
		})
		if d := citeDrift(t, p); len(d) != 1 {
			t.Fatalf("a slug citation is recognized inside its own backticks, got %s", detailsOf(d))
		}
		p = citeProject(t, map[string]string{
			"0001-a.md": citeTarget(inv),
			"0002-b.md": citeCarrier(
				"1. **X.** An example:\n\n```\nADR-0001 `inv: some-slug` is overridden here.\n```\n"),
		})
		if d := citeDrift(t, p); len(d) != 0 {
			t.Fatalf("a fenced slug citation must be exempt, got %s", detailsOf(d))
		}
	})
}

// A `cites:` token answers the check for the anchor it names and for no other.
// Suppression is anchor-scoped, so an item citing two anchors and tokenizing
// one still owes the other; a token that suppressed its whole item would let
// one inert mention silence every real claim beside it.
// invariant: cites-token-suppresses-citation-check
func TestCitesTokenSuppressesCitationCheck(t *testing.T) {
	dec := "1. **It overrides one and mentions another.** ADR-0001 Decision item 1\n" +
		"   (`cites: ADR-0001#1`) is only mentioned, while ADR-0001 Decision item 2 is\n" +
		"   overridden outright.\n"
	p := citeProject(t, map[string]string{
		"0001-a.md": citeTarget(""),
		"0002-b.md": citeCarrier(dec),
	})
	got := detailsOf(citeDrift(t, p))
	if strings.Contains(got, "ADR-0001#1 ") {
		t.Errorf("the tokenized anchor must be suppressed, got:\n%s", got)
	}
	if !strings.Contains(got, "ADR-0001#2") {
		t.Errorf("suppression must not reach a different anchor in the same item, got:\n%s", got)
	}
}

// slugScopingCorpus builds a two-ADR corpus: an Implemented target declaring
// one slug, and a carrier whose Decision states a slug override at item 1 and
// again at item 9. The carrier's tokens are supplied per case, which is the
// only variable the scoping rule turns on.
func slugScopingCorpus(refs []adr.SupersessionRef) adr.Corpus {
	tgt := adr.ADR{
		Number:   "0002",
		Filename: "0002-target.md",
		Status:   "Implemented",
		Sections: map[string]string{
			"Invariants": "- `invariant: doomed-slug`: the target's declared sentence.\n",
		},
	}
	car := adr.ADR{
		Number:   "0003",
		Filename: "0003-carrier.md",
		Status:   "Implemented",
		Sections: map[string]string{
			"Decision": "1. The first item narrows ADR-0002 `inv: doomed-slug` in passing.\n\n" +
				"9. The ninth item supersedes ADR-0002 `inv: doomed-slug` outright.\n",
		},
		Refs: refs,
	}
	return adr.NewCorpus([]adr.ADR{tgt, car})
}

// TestCitationsSlugClaimsScopePerCarrier pins both halves of ADR-0131 item 2's
// scoping split. The retirement key carries across items because a slug is
// atomic and dies once; the citation key does not, because letting it carry
// would let an informational mention at item 1 suppress a genuine unencoded
// retirement at item 9. The second half is what keeps the wider scope for the
// first from costing recall.
//
// invariant: citation-check-slug-claims-per-carrier
func TestCitationsSlugClaimsScopePerCarrier(t *testing.T) {
	retirement := adr.SupersessionRef{
		Target: "0002", Slug: "doomed-slug", Relation: adr.Retires, CarrierItem: 9,
	}
	citation := adr.SupersessionRef{
		Target: "0002", Slug: "doomed-slug", Relation: adr.Cites, CarrierItem: 1,
	}

	t.Run("retirement token satisfies the slug claim at every item", func(t *testing.T) {
		drift := computeCitations(slugScopingCorpus([]adr.SupersessionRef{retirement}), "docs/decisions")
		if len(drift) != 0 {
			t.Errorf("a retirement at item 9 must answer the item 1 claim too, got %d finding(s): %v", len(drift), drift)
		}
	})

	t.Run("citation token suppresses only at its own item", func(t *testing.T) {
		drift := computeCitations(slugScopingCorpus([]adr.SupersessionRef{citation}), "docs/decisions")
		if len(drift) != 1 {
			t.Fatalf("a cites-invariant at item 1 must leave the item 9 claim standing, got %d finding(s): %v", len(drift), drift)
		}
		if !strings.Contains(drift[0].Detail, "Decision item 9") {
			t.Errorf("surviving finding must name item 9, got %q", drift[0].Detail)
		}
	})
}

// A malformed ADR aborts the check via adr.ParseDir. Reachable through a
// direct call; inside full Check() it is pre-empted by checkPlans, which
// parses first - mirroring TestCheckADRRelatedLinksParseError.
func TestCheckCitationsADRParseError(t *testing.T) {
	root := scaffold(t, "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-broken.md"),
		"---\nstatus: [unterminated\n---\n# ADR-0001: Broken\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.checkCitations(); err == nil {
		t.Fatal("expected adr.ParseDir error for malformed frontmatter, got nil")
	}
}
