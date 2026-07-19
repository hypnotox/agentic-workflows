package project

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// checkCitations reports a supersession claim stated in prose and never
// encoded (ADR-0131 item 2). It fires when an override verb occurs in the same
// Decision item as a citation of another ADR's anchor and that Decision item
// carries no relation token for that anchor.
//
// It lives in its own file rather than in supersession.go, which already
// carries the coverage model, per ADR-0131 item 8.
func (p *Project) checkCitations() ([]manifest.Drift, error) {
	corpus, err := p.Corpus()
	if err != nil { // reachable via a direct call over a malformed ADR; pre-empted inside full Check()
		return nil, err
	}
	rel := filepath.ToSlash(filepath.Join(p.Cfg.DocsDir, "decisions"))
	return computeCitations(corpus, rel), nil
}

// computeCitations is the pure half, so tests drive a corpus rather than a
// project tree.
func computeCitations(corpus adr.Corpus, rel string) []manifest.Drift {
	var drift []manifest.Drift
	for _, a := range corpus.All() {
		// encoded[item][anchor] - a relation token of any kind at that
		// rationale site, cites: included, since a citation token's whole
		// purpose is to answer this check (ADR-0131 item 4).
		encoded := map[int]map[string]bool{}
		// retired[anchor] - a slug anchor this carrier retires anywhere in its
		// Decision section. Item claims are scoped per Decision item, slug
		// claims per carrier (citation-check-slug-claims-per-carrier): a slug
		// is atomic and dies once, so a carrier that already retires it has
		// said everything there is to say, and there is no truthful second
		// token to write. Only the retirement key carries across items; a
		// cites-invariant: stays in `encoded` and so suppresses at its own item
		// alone, or an informational mention at item 3 could silence a genuine
		// unencoded retirement at item 9 (ADR-0131 item 2).
		retired := map[string]bool{}
		for _, r := range corpus.RefsOf(a.Number) {
			anchor := adr.Anchor{ADR: r.Target, Item: r.Item, Slug: r.Slug}
			if encoded[r.CarrierItem] == nil {
				encoded[r.CarrierItem] = map[string]bool{}
			}
			encoded[r.CarrierItem][anchor.String()] = true
			if r.Slug != "" && r.Relation == adr.Retires {
				retired[anchor.String()] = true
			}
		}
		// One finding per (item, anchor), not per citation occurrence: an item
		// that names the same anchor twice has one claim to encode, and one
		// token settles it.
		reported := map[string]bool{}
		for _, c := range corpus.Citations(a.Number) {
			key := strconv.Itoa(c.CarrierItem) + "#" + c.Anchor.String()
			if !c.HasVerb || encoded[c.CarrierItem][c.Anchor.String()] || retired[c.Anchor.String()] || reported[key] {
				continue
			}
			// Exemptions are structural, never a marker (ADR-0131 item 5).
			// A self-citation claims nothing about another decision, and a
			// Proposed target is still mutable so nothing is settled to
			// supersede. Code spans and non-Decision text are excluded
			// upstream, in extraction.
			if c.Anchor.ADR == a.Number {
				continue
			}
			target, ok := corpus.ByNumber(c.Anchor.ADR)
			if !ok || target.IsProposed() {
				continue
			}
			// A cited invariant bullet that declares no slug has no anchor to
			// address: Anchor addresses a slug only by string, with no ordinal
			// form, so there is nothing a token could name.
			if c.Anchor.Slug != "" && !declaresSlug(corpus, c.Anchor) {
				continue
			}
			reported[key] = true
			drift = append(drift, manifest.Drift{
				Path:   rel + "/" + a.Filename,
				Kind:   "adr-unencoded-claim",
				Detail: unencodedDetail(a.Number, c),
			})
		}
	}
	return drift
}

// unencodedDetail names the satisfying token shapes for the anchor's kind. It
// names `cites:` explicitly because that token's absence is the silent case:
// an author who does not know it exists reaches for a relation token and
// records a supersession that never happened (ADR-0131's Consequences).
func unencodedDetail(carrier string, c adr.Citation) string {
	anchor := c.Anchor.String()
	shapes := fmt.Sprintf("`supersedes: %s`, `refines: %s`, or `cites: %s`", anchor, anchor, anchor)
	if c.Anchor.Slug != "" {
		// A slug is atomic, so it has no refinement form (ADR-0128 item 2).
		shapes = fmt.Sprintf("`supersedes-invariant: %s` or `cites-invariant: %s`", anchor, anchor)
	}
	return fmt.Sprintf(
		"ADR-%s Decision item %d states an override of %s but encodes no relation token for it at that item; add %s if the citation asserts no claim",
		carrier, c.CarrierItem, anchor, shapes)
}

// declaresSlug reports whether the cited ADR actually declares the slug.
func declaresSlug(corpus adr.Corpus, anchor adr.Anchor) bool {
	for _, s := range corpus.DeclaredSlugs(anchor.ADR) {
		if s == anchor.Slug {
			return true
		}
	}
	return false
}
