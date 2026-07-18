package adr

import (
	"fmt"
	"sort"
	"strings"
)

// Anchor is one addressable decision unit: a Decision item or a declared
// invariant slug on a specific ADR. Anchors are the nodes of ADR-0129's model;
// claims are the edges. Exactly one of Item/Slug is set.
type Anchor struct {
	ADR  string // 4-digit target ADR number
	Item int    // Decision item number; 0 for a slug anchor
	Slug string // invariant slug; "" for an item anchor
}

// Describe renders the anchor's local part for a human line: "item N" or
// "slug `<slug>`".
//
// Named Describe rather than Label because ADR-0129's
// supersession-model-single-source requires the identifiers SupersessionIndex,
// Override, and Label to appear nowhere in the tree - a greppable check that
// the retired render index is really gone. A new method reusing that name
// would defeat the grep while looking innocent.
func (a Anchor) Describe() string {
	if a.Slug != "" {
		return "slug `" + a.Slug + "`"
	}
	return fmt.Sprintf("item %d", a.Item)
}

// String renders the anchor in the token grammar's own shape, ADR-NNNN#anchor.
func (a Anchor) String() string {
	if a.Slug != "" {
		return "ADR-" + a.ADR + "#" + a.Slug
	}
	return fmt.Sprintf("ADR-%s#%d", a.ADR, a.Item)
}

// Claim is one edge: an ADR claiming an anchor, carrying the relation and the
// claiming ADR's own Decision item, so the rationale site is addressable
// (ADR-0129 item 2).
type Claim struct {
	Anchor      Anchor
	Carrier     string   // claiming ADR number
	CarrierItem int      // the claiming ADR's Decision item that carries the token
	Relation    Relation // retirement or refinement
}

// State is an ADR's derived supersession state (ADR-0129 item 3). It is
// computed from anchor coverage, never stored: the frontmatter keys that used
// to assert it are gone, and `status: Superseded` is checked against this
// rather than believed.
type State string

const (
	// StateLive is an ADR with no retired anchors.
	StateLive State = "Live"
	// StatePartial is the residual: some anchors retired, not all.
	StatePartial State = "PartiallySuperseded"
	// StateCovered is an ADR every one of whose anchors has been retired.
	StateCovered State = "Covered"
)

// coverage is the derived anchor model, built once inside NewCorpus
// (corpus-model-not-rebuilt).
type coverage struct {
	anchors  map[string][]Anchor // ADR number -> its anchors
	claims   map[Anchor][]Claim  // anchor -> claims on it, carrier-sorted
	byTarget map[string][]Claim  // ADR number -> claims on any of its anchors
	state    map[string]State    // ADR number -> derived state
}

// buildCoverage derives the model. Coverage counts a claim only when it is a
// retirement AND its carrier is Implemented (ADR-0128 items 2 and 6): a
// refinement adapts rather than replaces, and a Proposed successor must not
// kill its predecessor before it ships.
func buildCoverage(adrs []ADR, byNum map[string]ADR) coverage {
	c := coverage{
		anchors:  map[string][]Anchor{},
		claims:   map[Anchor][]Claim{},
		byTarget: map[string][]Claim{},
		state:    map[string]State{},
	}
	for _, a := range adrs {
		var list []Anchor
		for _, n := range a.DecisionItems() {
			list = append(list, Anchor{ADR: a.Number, Item: n})
		}
		for _, slug := range a.DeclaredSlugs() {
			list = append(list, Anchor{ADR: a.Number, Slug: slug})
		}
		c.anchors[a.Number] = list
	}
	for _, a := range adrs {
		for _, r := range a.Refs {
			anchor := Anchor{ADR: r.Target, Item: r.Item, Slug: r.Slug}
			claim := Claim{Anchor: anchor, Carrier: a.Number, CarrierItem: r.CarrierItem, Relation: r.Relation}
			c.claims[anchor] = append(c.claims[anchor], claim)
			c.byTarget[r.Target] = append(c.byTarget[r.Target], claim)
		}
	}
	for _, list := range c.claims {
		sortClaims(list)
	}
	for _, list := range c.byTarget {
		sortClaims(list)
	}
	for _, a := range adrs {
		c.state[a.Number] = deriveState(c, byNum, a)
	}
	return c
}

func sortClaims(list []Claim) {
	sort.Slice(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if a.Anchor != b.Anchor {
			if (a.Anchor.Slug == "") != (b.Anchor.Slug == "") {
				return a.Anchor.Slug == "" // item anchors before slug anchors
			}
			if a.Anchor.Slug == "" {
				return a.Anchor.Item < b.Anchor.Item
			}
			return a.Anchor.Slug < b.Anchor.Slug
		}
		if a.Carrier != b.Carrier {
			return a.Carrier < b.Carrier
		}
		return a.Relation < b.Relation
	})
}

// deriveState computes one ADR's state. Partial is the residual by
// construction, so no anchor set can fall outside the three states. An ADR with
// zero anchors is Live rather than vacuously Covered: "every anchor retired" is
// true of an empty set, and reading that as a dead decision would retire an ADR
// nobody superseded (ADR-0129 item 3).
func deriveState(c coverage, byNum map[string]ADR, a ADR) State {
	anchors := c.anchors[a.Number]
	if len(anchors) == 0 {
		return StateLive
	}
	retired := 0
	for _, anchor := range anchors {
		if isRetired(c, byNum, anchor) {
			retired++
		}
	}
	switch retired {
	case 0:
		return StateLive
	case len(anchors):
		return StateCovered
	default:
		return StatePartial
	}
}

// hasShipped reports whether a carrier's claims are in force. Implemented is
// the obvious case; Superseded counts because superseding an ADR does not
// un-supersede what that ADR superseded - the retirement it asserts remains
// part of the corpus. Proposed and Accepted do not: a successor that has not
// shipped must not kill its predecessor.
func hasShipped(a ADR) bool { return a.IsImplemented() || a.IsSuperseded() }

// isRetired reports whether an anchor carries a counting retirement: a
// `supersedes:` claim from a carrier that has shipped. A refinement never
// counts, however many there are.
//
// A carrier counts when it is Implemented OR Superseded. Superseding an ADR
// does not un-supersede what that ADR superseded: a Superseded record is not
// void in meaning, and the retirement it asserts remains part of the corpus and
// remains in force. Excluding it would break every chain of more than two
// generations - in A retires B retires C, flipping B to Superseded would revive
// C, whose own status could then never be made consistent by any edit to C.
//
// This is a predicate over present corpus state, not over history: no carrier
// is asked what status it once held, only what it holds now. Proposed and
// Accepted carriers still do not count, which is the rule's original point -
// a successor that has not shipped must not kill its predecessor.
func isRetired(c coverage, byNum map[string]ADR, anchor Anchor) bool {
	for _, claim := range c.claims[anchor] {
		if claim.Relation != Retires {
			continue
		}
		if carrier, ok := byNum[claim.Carrier]; ok && hasShipped(carrier) {
			return true
		}
	}
	return false
}

// State returns the ADR's derived supersession state. An unknown number is
// Live: it has no anchors anyone could have retired.
func (c Corpus) State(num string) State {
	if s, ok := c.cov.state[num]; ok {
		return s
	}
	return StateLive
}

// ClaimsOn returns the claims made on the named ADR's anchors, ordered by
// anchor then carrier. This is the "who claims this" question ACTIVE.md's
// chains and the domain index both ask.
func (c Corpus) ClaimsOn(num string) []Claim { return c.cov.byTarget[num] }

// Retirers returns the numbers of the Implemented ADRs that retired at least
// one of the named ADR's anchors, sorted and deduplicated. This is what renders
// in place of the scalar successor the frontmatter used to carry: coverage may
// split across several carriers, so the answer is a set, not a name.
func (c Corpus) Retirers(num string) []string {
	seen := map[string]bool{}
	var out []string
	for _, claim := range c.cov.byTarget[num] {
		if claim.Relation != Retires || seen[claim.Carrier] {
			continue
		}
		if carrier, ok := c.byNum[claim.Carrier]; !ok || !hasShipped(carrier) {
			continue
		}
		seen[claim.Carrier] = true
		out = append(out, claim.Carrier)
	}
	sort.Strings(out)
	return out
}

// Anchors returns the named ADR's anchors: its Decision items then its declared
// invariant slugs.
func (c Corpus) Anchors(num string) []Anchor { return c.cov.anchors[num] }

// UncoveredAnchors returns the named ADR's anchors that carry no counting
// retirement, in anchor order. The coverage-versus-status check names these
// when a `Superseded` ADR is not in fact fully covered, so the author is told
// exactly which decision still needs a successor rather than merely that the
// status is wrong.
func (c Corpus) UncoveredAnchors(num string) []Anchor {
	var out []Anchor
	for _, anchor := range c.cov.anchors[num] {
		if !isRetired(c.cov, c.byNum, anchor) {
			out = append(out, anchor)
		}
	}
	return out
}

// Chain is one supersedence relationship for the ACTIVE.md index: a covered
// predecessor and every ADR that retired one of its anchors. The shape is
// one-to-many (ADR-0129 item 6) because coverage may split across successors,
// which is exactly why the scalar `superseded_by:` field could not express it.
type Chain struct {
	Predecessor string
	Successors  []string
}

// Chains returns every fully-covered ADR paired with its retirers, sorted by
// predecessor.

func (c Corpus) Chains() []Chain {
	merged := map[string]map[string]bool{}
	add := func(pred, succ string) {
		if merged[pred] == nil {
			merged[pred] = map[string]bool{}
		}
		merged[pred][succ] = true
	}
	for _, a := range c.all {
		if c.State(a.Number) == StateCovered {
			for _, r := range c.Retirers(a.Number) {
				add(a.Number, r)
			}
		}
	}
	out := make([]Chain, 0, len(merged))
	for pred, succs := range merged {
		list := make([]string, 0, len(succs))
		for s := range succs {
			list = append(list, s)
		}
		sort.Strings(list)
		out = append(out, Chain{Predecessor: pred, Successors: list})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Predecessor < out[j].Predecessor })
	return out
}

// AnnotatedAnchors returns the claimed anchors of ADRs that are still live -
// the "superseded anchors on live ADRs" view. A covered ADR is excluded: its
// whole record is retired, so per-anchor annotation would be noise, and the
// chains subsection already names its retirers.
func (c Corpus) AnnotatedAnchors() []Claim {
	var out []Claim
	for _, a := range c.all {
		if !a.IsLive() || c.State(a.Number) == StateCovered {
			continue
		}
		for _, claim := range c.ClaimsOn(a.Number) {
			// A citation claims nothing, so it annotates nothing (ADR-0131
			// item 4). This is the sole seam: Verb() renders anything that is
			// not Refines as "superseded by", so an unfiltered Cites claim
			// would read as a retirement in both consumers.
			if claim.Relation == Cites {
				continue
			}
			// A claim from a Superseded carrier is not annotation-worthy: the
			// carrier's own record is dead.
			if carrier, ok := c.byNum[claim.Carrier]; ok && !carrier.IsSuperseded() {
				out = append(out, claim)
			}
		}
	}
	return out
}

// Verb renders the claim's relation for a human line: a retired anchor reads
// "superseded by", an adapted one "refined by".
func (c Claim) Verb() string {
	if c.Relation == Refines {
		return "refined by"
	}
	return "superseded by"
}

// joinADRs renders a successor set as "ADR-0002, ADR-0003".
func joinADRs(nums []string) string {
	out := make([]string, len(nums))
	for i, n := range nums {
		out[i] = "ADR-" + n
	}
	return strings.Join(out, ", ")
}

// Cycle is one retirement cycle among covered ADRs, as the sequence of ADR
// numbers that closes back on its first element.
type Cycle []string

// SelfClaim is an ADR whose own Decision section claims one of its own anchors.
type SelfClaim struct {
	ADR    string
	Anchor Anchor
}

// GraphFaults returns the irreflexivity and acyclicity violations in the
// retirement relation (ADR-0129 item 7). Nothing previously forbade either:
// the single-claimant check that used to stand in for this died with the
// frontmatter encoding, and a self-targeting token or an A-to-B-to-A cycle
// would otherwise derive a coherent-looking state from a contradiction.
//
// Acyclicity is scoped to ADRs the model classifies as Covered. A cycle among
// live ADRs is not yet a contradiction: partial claims in both directions are
// legitimate, since two ADRs may each refine or retire some of the other's
// anchors while both remain current. Only when the cycle's members are all
// fully retired does it assert that each is dead because the other is.
func (c Corpus) GraphFaults() ([]SelfClaim, []Cycle) {
	var self []SelfClaim
	for _, a := range c.all {
		for _, r := range a.Refs {
			if r.Target == a.Number {
				self = append(self, SelfClaim{ADR: a.Number, Anchor: Anchor{ADR: r.Target, Item: r.Item, Slug: r.Slug}})
			}
		}
	}
	sort.Slice(self, func(i, j int) bool { return self[i].ADR < self[j].ADR })

	// Edge set restricted to covered ADRs: predecessor -> its retirers.
	edges := map[string][]string{}
	for _, a := range c.all {
		if c.State(a.Number) != StateCovered {
			continue
		}
		for _, r := range c.Retirers(a.Number) {
			if c.State(r) == StateCovered {
				edges[a.Number] = append(edges[a.Number], r)
			}
		}
	}
	var cycles []Cycle
	const (
		unvisited = 0
		onStack   = 1
		done      = 2
	)
	mark := map[string]int{}
	var stack []string
	var walk func(string)
	walk = func(n string) {
		mark[n] = onStack
		stack = append(stack, n)
		for _, next := range edges[n] {
			switch mark[next] {
			case unvisited:
				walk(next)
			case onStack:
				// Report the cycle from its entry point, so the same loop
				// reported from two start nodes reads identically.
				for i, s := range stack {
					if s == next {
						cycles = append(cycles, append(Cycle{}, stack[i:]...))
						break
					}
				}
			}
		}
		stack = stack[:len(stack)-1]
		mark[n] = done
	}
	for _, a := range c.all {
		if mark[a.Number] == unvisited && len(edges[a.Number]) > 0 {
			walk(a.Number)
		}
	}
	return self, cycles
}
