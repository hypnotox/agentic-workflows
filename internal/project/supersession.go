package project

import (
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// checkSupersessionAll runs the drift half of the ADR-0120 corpus checks:
// Decision format, full-supersession three-way symmetry, token ref validity,
// partial back-pointers, and flavour exclusivity. The advisory-note half is
// supersessionNotes' (AdvisoryNotes' source); both consume computeSupersession.
func (p *Project) checkSupersessionAll() ([]manifest.Drift, error) {
	corpus, err := p.Corpus()
	if err != nil { // reachable via a direct call over a malformed ADR; pre-empted inside full Check()
		return nil, err
	}
	adrs := corpus.All()
	rel := filepath.ToSlash(filepath.Join(p.Cfg.DocsDir, "decisions"))
	drift := p.checkDecisionFormat(adrs, rel)
	rk, err := checkRetiredKey(corpus, rel)
	if err != nil { // coverage-ignore: ParseDir above just read every scanned path
		return nil, err
	}
	drift = append(drift, rk...)
	d2, _ := computeSupersession(corpus, rel)
	return append(drift, d2...), nil
}

// retiredKeyRe matches the removed retires_invariants: frontmatter key at
// column 0 - the raw form the parsed struct no longer surfaces.
var retiredKeyRe = regexp.MustCompile(`(?m)^retires_invariants:`)

// supersessionKeyRe matches the two frontmatter keys ADR-0128 item 1 removes.
// Same shape as retiredKeyRe and for the same reason: the schema no longer
// carries either field, so non-strict YAML would silently ignore a key an
// author still believed was load-bearing - and for these two, believing it
// would mean believing an ADR was superseded when nothing had superseded it.
var supersessionKeyRe = regexp.MustCompile(`(?m)^(supersedes|superseded_by):`)

// checkRetiredKey refuses the raw retires_invariants: key (ADR-0120 item 7):
// the schema no longer carries the field and non-strict YAML would silently
// ignore it, so only a raw frontmatter scan can catch a stale corpus.
// touches-invariant: retires-invariants-key-refused - the raw-key scan; proof in supersession_test.go
func checkRetiredKey(corpus adr.Corpus, rel string) ([]manifest.Drift, error) {
	adrs := corpus.All()
	var drift []manifest.Drift
	for _, a := range adrs {
		b, err := corpus.Raw(a.Number)
		if err != nil { // coverage-ignore: ParseDir just read this exact path
			return nil, err
		}
		raw := string(b)
		// frontmatter.Parse succeeded in ParseDir, so the closing fence exists;
		// +4 keeps the newline before the fence in the window.
		fmEnd := strings.Index(raw[3:], "\n---") + 3 + 1
		if retiredKeyRe.MatchString(raw[:fmEnd]) {
			drift = append(drift, manifest.Drift{Path: rel + "/" + a.Filename, Kind: "adr-retired-key", Detail: fmt.Sprintf("ADR-%s: retires_invariants: is no longer read; run awf upgrade", a.Number)})
		}
		// touches-invariant: supersession-keys-refused - the raw-key scan; proof in supersession_test.go
		if m := supersessionKeyRe.FindStringSubmatch(raw[:fmEnd]); m != nil {
			drift = append(drift, manifest.Drift{Path: rel + "/" + a.Filename, Kind: "adr-retired-key",
				Detail: fmt.Sprintf("ADR-%s: %s: is no longer read; supersession is derived from inline tokens; run awf upgrade", a.Number, m[1])})
		}
	}
	return drift, nil
}

// supersessionNotes re-parses the corpus and returns the advisory half of
// computeSupersession for AdvisoryNotes' note channel - mirroring how the
// tag-health notes reach cmd/awf's runCheck. The double ParseDir matches the
// corpus checks' existing per-check parse pattern.
func (p *Project) supersessionNotes() ([]string, error) {
	corpus, err := p.Corpus()
	if err != nil { // reachable via a direct call over a malformed ADR; pre-empted inside AdvisoryNotes by tagHealthNotes only when a vocabulary is configured
		return nil, err
	}
	rel := filepath.ToSlash(filepath.Join(p.Cfg.DocsDir, "decisions"))
	_, notes := computeSupersession(corpus, rel)
	return notes, nil
}

// checkDecisionFormat enforces ADR-0120 item 12: every ADR's Decision section
// consists of column-0 numbered items, sequential from 1, regardless of
// status - a Superseded ADR can still be an anchor target.
// touches-invariant: decision-items-enumerable - the format check itself; proof in supersession_test.go
func (p *Project) checkDecisionFormat(adrs []adr.ADR, rel string) []manifest.Drift {
	var drift []manifest.Drift
	for _, a := range adrs {
		items := a.DecisionItems()
		if len(items) == 0 {
			drift = append(drift, manifest.Drift{Path: rel + "/" + a.Filename, Kind: "adr-decision-format", Detail: fmt.Sprintf("ADR-%s: Decision section has no column-0 numbered items", a.Number)})
			continue
		}
		for i, n := range items {
			if n != i+1 {
				drift = append(drift, manifest.Drift{Path: rel + "/" + a.Filename, Kind: "adr-decision-format", Detail: fmt.Sprintf("ADR-%s: Decision item %d found where %d expected (gap, duplicate, or restart)", a.Number, n, i+1)})
				break
			}
		}
	}
	return drift
}

// computeSupersession implements the ADR-0120 supersession checks over one
// corpus pass: the four error checks (full three-way symmetry, token ref
// validity, partial back-pointers, flavour exclusivity) as drift, and the two
// advisories (a token into a fully superseded target, one anchor claimed by
// two live ADRs) as notes. Notes never enter the drift slice.
// touches-invariant: supersession-token-ref-validity - the adr-token-ref checks; proof in supersession_test.go
func computeSupersession(corpus adr.Corpus, rel string) ([]manifest.Drift, []string) {
	adrs := corpus.All()
	byNum := map[string]adr.ADR{}
	for _, a := range adrs {
		byNum[a.Number] = a
	}
	var drift []manifest.Drift
	add := func(a adr.ADR, kind, detail string) {
		drift = append(drift, manifest.Drift{Path: rel + "/" + a.Filename, Kind: kind, Detail: detail})
	}
	var notes []string

	// anchorClaim[anchor] = live (Accepted/Implemented, ADR-0120 item 4)
	// claimant numbers, one per ADR; a Proposed claimant is not yet in force.
	anchorClaim := map[string][]string{}

	for _, a := range adrs {
		num, _ := strconv.Atoi(a.Number) // a.Number is a 4-digit numeral matched by FilenameRe
		live := a.IsLive()
		seenAnchor := map[string]bool{}
		for _, r := range corpus.RefsOf(a.Number) {
			// The claim key is kind-prefixed: the slug grammar admits an
			// all-digit slug, so an item ref #2 and a slug ref #2 into one
			// target are distinct anchors, never a conflict pair.
			anchor := "slug:" + r.Target + "#" + r.Slug
			if r.Slug == "" {
				anchor = "item:" + r.Target + "#" + strconv.Itoa(r.Item)
			}
			// Only RETIREMENTS contest (ADR-0128 item 6). Two ADRs both
			// declaring an anchor dead is a genuine conflict; multiple
			// refinements of one anchor are the normal shape of an evolving
			// decision, and a mixed pair - refined by one ADR, later retired by
			// another - is the ADR-0034 item 1 history the ADR cites as
			// healthy. Neither notes.
			if live && r.Relation == adr.Retires && !seenAnchor[anchor] {
				seenAnchor[anchor] = true
				anchorClaim[anchor] = append(anchorClaim[anchor], a.Number)
			}
			target, ok := byNum[r.Target]
			if !ok {
				add(a, "adr-token-ref", fmt.Sprintf("ADR-%s: token targets ADR-%s, which does not exist", a.Number, r.Target))
				continue
			}
			if target.IsProposed() {
				add(a, "adr-token-ref", fmt.Sprintf("ADR-%s: token targets Proposed ADR-%s, whose body is still mutable", a.Number, r.Target))
				continue
			}
			if r.Slug == "" {
				if items := len(corpus.DecisionItems(r.Target)); r.Item > items { // the format check guarantees 1..N
					add(a, "adr-token-ref", fmt.Sprintf("ADR-%s: token cites ADR-%s#%d, but its Decision has %d items", a.Number, r.Target, r.Item, items))
				}
			} else if !slices.Contains(corpus.DeclaredSlugs(target.Number), r.Slug) {
				add(a, "adr-token-ref", fmt.Sprintf("ADR-%s: token cites ADR-%s#%s, which its Invariants section does not declare", a.Number, r.Target, r.Slug))
			}
			// Back-pointer: a token into a target of ANY status requires the
			// target's related: to name the carrier (ADR-0128 item 5). The live-
			// only guard this replaces would have lost the successor entirely
			// once the status flip stopped naming one: a claimant landing after
			// the flip owed no back-pointer and so became invisible, and bare
			// `Superseded` has nothing else to recover it from.
			if !slices.Contains(target.Related, num) {
				add(a, "adr-token-backpointer", fmt.Sprintf("ADR-%s: token into ADR-%s lacks the related: back-pointer on the target", a.Number, r.Target))
			}
			// The superseded-target advisory is deliberately absent (ADR-0128
			// item 6). It existed to explain a token pointing at a dead ADR,
			// which used to be anomalous. Under coverage-derived supersession it
			// is the normal shape of every completed supersedence - the tokens
			// are what killed the target - so the note would fire on every
			// retirement and mean nothing.
		}
	}

	// Coverage versus status (ADR-0128 items 3 and 4). The status is
	// hand-authored and this is what makes it honest: the flip is refused in
	// both directions, and each finding names the exact edit required rather
	// than merely reporting a mismatch.
	for _, a := range adrs {
		covered := corpus.State(a.Number) == adr.StateCovered
		switch {
		case covered && !a.IsSuperseded():
			// touches-invariant: supersession-coverage-derives-status - the flip-owed half; proof in supersession_test.go
			add(a, "adr-coverage-status", fmt.Sprintf(
				"ADR-%s: every anchor is retired (by %s), so status must be Superseded",
				a.Number, joinADRNums(corpus.Retirers(a.Number))))
		case !covered && a.IsSuperseded():
			uncovered := corpus.UncoveredAnchors(a.Number)
			labels := make([]string, len(uncovered))
			for i, anchor := range uncovered {
				labels[i] = anchor.Describe()
			}
			carry := "carries"
			if len(labels) > 1 {
				carry = "carry"
			}
			add(a, "adr-coverage-status", fmt.Sprintf(
				"ADR-%s: status is Superseded but %s %s no retirement from a shipped ADR",
				a.Number, strings.Join(labels, ", "), carry))
		}
	}

	// Structural faults in the retirement relation (ADR-0129 item 7). Nothing
	// forbade these before: the single-claimant check that stood in for them
	// died with the frontmatter encoding, and either fault derives a
	// coherent-looking state from a contradiction.
	// touches-invariant: supersession-graph-acyclic - the fault report; proof in supersession_test.go
	selfClaims, cycles := corpus.GraphFaults()
	for _, s := range selfClaims {
		if a, ok := corpus.ByNumber(s.ADR); ok {
			add(a, "adr-supersession-graph", fmt.Sprintf("ADR-%s: token claims its own anchor %s", s.ADR, s.Anchor))
		}
	}
	for _, cyc := range cycles {
		if a, ok := corpus.ByNumber(cyc[0]); ok {
			add(a, "adr-supersession-graph", "retirement cycle among fully covered ADRs: "+strings.Join(cyc, " -> ")+" -> "+cyc[0])
		}
	}

	// Advisory: one anchor claimed by two or more live ADRs. The kind prefix
	// is keying only; the note names the anchor as written.
	for _, anchor := range slices.Sorted(maps.Keys(anchorClaim)) {
		cs := anchorClaim[anchor]
		display := anchor[strings.Index(anchor, ":")+1:]
		for i := 1; i < len(cs); i++ {
			notes = append(notes, fmt.Sprintf("anchor ADR-%s claimed by ADR-%s and ADR-%s", display, cs[0], cs[i]))
		}
	}
	return drift, notes
}

// joinADRNums renders a set of ADR numbers for a finding: "ADR-0002, ADR-0003".
func joinADRNums(nums []string) string {
	out := make([]string, len(nums))
	for i, n := range nums {
		out[i] = "ADR-" + n
	}
	return strings.Join(out, ", ")
}
