package adr_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestAnchorRendering covers both anchor shapes in both renderings. The token
// form is what the grammar itself writes, so a fault message quoting an anchor
// reads back as something an author can grep for.
func TestAnchorRendering(t *testing.T) {
	item := adr.Anchor{ADR: "0120", Item: 3}
	slug := adr.Anchor{ADR: "0120", Slug: "some-slug"}
	if got, want := item.Describe(), "item 3"; got != want {
		t.Errorf("item Describe = %q, want %q", got, want)
	}
	if got, want := slug.Describe(), "slug `some-slug`"; got != want {
		t.Errorf("slug Describe = %q, want %q", got, want)
	}
	if got, want := item.String(), "ADR-0120#3"; got != want {
		t.Errorf("item String = %q, want %q", got, want)
	}
	if got, want := slug.String(), "ADR-0120#some-slug"; got != want {
		t.Errorf("slug String = %q, want %q", got, want)
	}
}

// TestStateOfUnknownADR pins the fallback: a number the corpus does not hold
// has no anchors anyone could have retired, so it is Live rather than a zero
// value that would read as some other state.
func TestStateOfUnknownADR(t *testing.T) {
	dir := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(dir, "0001-a.md"),
		testsupport.ADR("Accepted", testsupport.WithTitle("0001: A"), testsupport.WithBody("## Decision\n\n1. x.\n")))
	if got := mustCorpus(t, dir).State("9999"); got != adr.StateLive {
		t.Errorf("State of an absent ADR = %q, want Live", got)
	}
}

// TestClaimsSortedByAnchorThenCarrier pins the ordering the renderers depend
// on: item anchors before slug anchors, then by carrier. Rendered output is
// drift-checked by regeneration, so an unstable order would flap the gate.
func TestClaimsSortedByAnchorThenCarrier(t *testing.T) {
	dir := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(dir, "0001-target.md"),
		testsupport.ADR("Accepted", testsupport.WithTitle("0001: Target"), testsupport.WithRelated(2, 3),
			testsupport.WithBody("## Decision\n\n1. a.\n2. b.\n\n## Invariants\n\n- `invariant: z-slug` - x.\n- `invariant: a-slug` - y.\n")))
	// Written slug-first and high-carrier-first, so a stable input order cannot
	// produce the expected output by accident.
	testsupport.WriteFile(t, filepath.Join(dir, "0003-late.md"),
		testsupport.ADR("Implemented", testsupport.WithTitle("0003: Late"),
			testsupport.WithBody("## Decision\n\n1. `supersedes-invariant: ADR-0001#z-slug`, `supersedes: ADR-0001#2`.\n")))
	testsupport.WriteFile(t, filepath.Join(dir, "0002-early.md"),
		testsupport.ADR("Implemented", testsupport.WithTitle("0002: Early"),
			testsupport.WithBody("## Decision\n\n1. `supersedes-invariant: ADR-0001#a-slug`, `supersedes: ADR-0001#1`.\n")))

	var got []string
	for _, claim := range mustCorpus(t, dir).ClaimsOn("0001") {
		got = append(got, claim.Anchor.String()+"<-"+claim.Carrier)
	}
	want := []string{"ADR-0001#1<-0002", "ADR-0001#2<-0003", "ADR-0001#a-slug<-0002", "ADR-0001#z-slug<-0003"}
	if len(got) != len(want) {
		t.Fatalf("claims: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("claims order: got %v, want %v", got, want)
		}
	}
}

// TestClaimOrderingTiebreaks covers the last two comparator arms, both of which
// only fire on inputs the grammar admits but an author would not write:
// one ADR claiming the same anchor twice with different relations, and two ADRs
// each claiming one of their own anchors. Rendered output is drift-checked by
// regeneration, so even a contradictory corpus must order deterministically
// rather than flap the gate on sort.Slice's instability.
func TestClaimOrderingTiebreaks(t *testing.T) {
	dir := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(dir, "0001-target.md"),
		testsupport.ADR("Accepted", testsupport.WithTitle("0001: Target"), testsupport.WithRelated(2),
			testsupport.WithBody("## Decision\n\n1. a.\n")))
	testsupport.WriteFile(t, filepath.Join(dir, "0002-both.md"),
		testsupport.ADR("Implemented", testsupport.WithTitle("0002: Both"), testsupport.WithRelated(2, 3),
			testsupport.WithBody("## Decision\n\n1. b.\n2. `refines: ADR-0001#1`, `supersedes: ADR-0001#1`, `supersedes: ADR-0002#1`.\n")))
	testsupport.WriteFile(t, filepath.Join(dir, "0003-selfer.md"),
		testsupport.ADR("Implemented", testsupport.WithTitle("0003: Selfer"), testsupport.WithRelated(3),
			testsupport.WithBody("## Decision\n\n1. c.\n2. `supersedes: ADR-0003#1`.\n")))
	c := mustCorpus(t, dir)

	// Same anchor, same carrier, differing relation: refinement sorts first.
	claims := c.ClaimsOn("0001")
	if len(claims) != 2 {
		t.Fatalf("want two claims on ADR-0001, got %#v", claims)
	}
	if claims[0].Relation != adr.Refines || claims[1].Relation != adr.Retires {
		t.Errorf("relation tiebreak: got %q then %q, want refines then retires", claims[0].Relation, claims[1].Relation)
	}

	// Two self-claiming ADRs sort by number.
	self, _ := c.GraphFaults()
	if len(self) != 2 {
		t.Fatalf("want two self-claims, got %#v", self)
	}
	if self[0].ADR != "0002" || self[1].ADR != "0003" {
		t.Errorf("self-claims must sort by ADR number, got %s then %s", self[0].ADR, self[1].ADR)
	}
}

// TestMultiGenerationChain pins the property that a two-generation-only model
// cannot express: A retires B, B retires C, and B is itself Superseded. B's
// retirement of C stays in force, so C remains Covered and its `Superseded`
// status stays consistent. Under the original Implemented-only rule C revived
// the moment B was flipped, and no edit to C could have made it consistent
// again - the drift was unfixable at its own file.
// invariant: supersession-model-derives-state
func TestMultiGenerationChain(t *testing.T) {
	dir := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(dir, "0003-c.md"),
		testsupport.ADR("Superseded", testsupport.WithTitle("0003: C"),
			testsupport.WithRelated(2), testsupport.WithBody("## Decision\n\n1. c.\n")))
	testsupport.WriteFile(t, filepath.Join(dir, "0002-b.md"),
		testsupport.ADR("Superseded", testsupport.WithTitle("0002: B"),
			testsupport.WithRelated(1),
			testsupport.WithBody("## Decision\n\n1. Retires `supersedes: ADR-0003#1`.\n")))
	testsupport.WriteFile(t, filepath.Join(dir, "0001-a.md"),
		testsupport.ADR("Implemented", testsupport.WithTitle("0001: A"),
			testsupport.WithBody("## Decision\n\n1. Retires `supersedes: ADR-0002#1`.\n")))
	c := mustCorpus(t, dir)

	for _, tc := range []struct {
		num   string
		state adr.State
	}{
		{"0001", adr.StateLive},    // nothing claims A
		{"0002", adr.StateCovered}, // retired by A
		{"0003", adr.StateCovered}, // retired by B, which is itself Superseded
	} {
		if got := c.State(tc.num); got != tc.state {
			t.Errorf("ADR-%s state = %q, want %q", tc.num, got, tc.state)
		}
	}
	if got := c.Retirers("0003"); len(got) != 1 || got[0] != "0002" {
		t.Errorf("ADR-0003 retirers = %v, want [0002] - a Superseded carrier still retires", got)
	}
}

// TestCitesTokenIsUncounted pins the citation token's inertness in the coverage
// model: ADR-0002 names every one of ADR-0001's anchors, but only cites them, so
// ADR-0001 keeps every anchor and stays live. A negative relation filter
// anywhere in the coverage path would read these as retirements and kill a live
// ADR, which is the failure this pins.
//
// It drives both keys deliberately. `cites-invariant:` has no corpus instance -
// every real citation is item-anchored - so without a synthetic slug-anchored
// token citesInvRe would be executed but never behaviourally exercised, and a
// mistake in it would not fail.
// invariant: cites-token-uncounted
func TestCitesTokenIsUncounted(t *testing.T) {
	dir := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(dir, "0001-target.md"),
		testsupport.ADR("Accepted", testsupport.WithTitle("0001: Target"), testsupport.WithRelated(2),
			testsupport.WithBody("## Decision\n\n1. a.\n\n## Invariants\n\n- `invariant: a-slug` - x.\n")))
	testsupport.WriteFile(t, filepath.Join(dir, "0002-carrier.md"),
		testsupport.ADR("Implemented", testsupport.WithTitle("0002: Carrier"),
			testsupport.WithBody("## Decision\n\n1. Mentions `cites: ADR-0001#1` and `cites-invariant: ADR-0001#a-slug`.\n")))
	c := mustCorpus(t, dir)

	if got := c.State("0001"); got != adr.StateLive {
		t.Errorf("ADR-0001 state = %q, want Live - a citation retires nothing", got)
	}
	if got := c.Retirers("0001"); len(got) != 0 {
		t.Errorf("ADR-0001 retirers = %v, want none - a citation is not a retirement", got)
	}
	// Both tokens must actually have parsed, or the assertions above would pass
	// vacuously against a corpus that simply saw no tokens at all.
	var relations []adr.Relation
	for _, claim := range c.ClaimsOn("0001") {
		relations = append(relations, claim.Relation)
	}
	if len(relations) != 2 || relations[0] != adr.Cites || relations[1] != adr.Cites {
		t.Fatalf("claims on ADR-0001 = %v, want two Cites claims - both keys must parse", relations)
	}
}

// TestCitesTokenIsUnrendered pins that a citation reaches neither rendered
// surface. AnnotatedAnchors is the sole seam: RenderActiveMD and awf context
// both consume it, and Claim.Verb renders anything that is not Refines as
// "superseded by", so an unfiltered citation would render as a retirement it
// never claimed.
// invariant: cites-token-unrendered
func TestCitesTokenIsUnrendered(t *testing.T) {
	dir := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(dir, "0001-target.md"),
		testsupport.ADR("Accepted", testsupport.WithTitle("0001: Target"), testsupport.WithRelated(2),
			testsupport.WithBody("## Decision\n\n1. a.\n\n## Invariants\n\n- `invariant: a-slug` - x.\n")))
	testsupport.WriteFile(t, filepath.Join(dir, "0002-carrier.md"),
		testsupport.ADR("Implemented", testsupport.WithTitle("0002: Carrier"),
			testsupport.WithBody("## Decision\n\n1. Mentions `cites: ADR-0001#1` and `cites-invariant: ADR-0001#a-slug`.\n")))
	c := mustCorpus(t, dir)

	if got := c.AnnotatedAnchors(); len(got) != 0 {
		t.Errorf("AnnotatedAnchors = %v, want none - a citation annotates nothing", got)
	}
	out := adr.RenderActiveMD(c)
	for _, unwanted := range []string{"Superseded anchors on live ADRs", "superseded by", "refined by"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("ACTIVE.md contains %q over a cites-only corpus:\n%s", unwanted, out)
		}
	}
}
