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
// touches-invariant: supersession-full-symmetry - the forward+reverse symmetry checks; proof in supersession_test.go
// touches-invariant: supersession-token-ref-validity - the adr-token-ref checks; proof in supersession_test.go
// touches-invariant: supersession-backpointer - the adr-token-backpointer check; proof in supersession_test.go
// touches-invariant: supersession-flavour-exclusive - the adr-token-exclusive check; proof in supersession_test.go
// touches-invariant: supersession-conflict-advisory - the live same-anchor note; proof in supersession_test.go
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

	// claimants[target] = numbers of the ADRs whose supersedes: names target,
	// in corpus (number) order.
	claimants := map[string][]string{}
	for _, a := range adrs {
		for _, n := range a.Supersedes {
			t := fmt.Sprintf("%04d", n)
			claimants[t] = append(claimants[t], a.Number)
		}
	}

	// anchorClaim[anchor] = live (Accepted/Implemented, ADR-0120 item 4)
	// claimant numbers, one per ADR; a Proposed claimant is not yet in force.
	anchorClaim := map[string][]string{}

	for _, a := range adrs {
		// Full symmetry, forward direction: each supersedes: entry must point at
		// an existing target carrying the matching suffixed status and scalar
		// back-pointer.
		for _, n := range a.Supersedes {
			tn := fmt.Sprintf("%04d", n)
			target, ok := byNum[tn]
			if !ok {
				add(a, "adr-supersession", fmt.Sprintf("ADR-%s: supersedes ADR-%s, which does not exist", a.Number, tn))
				continue
			}
			if want := "Superseded by ADR-" + a.Number; target.Status != want {
				add(a, "adr-supersession", fmt.Sprintf("ADR-%s: supersedes ADR-%s, whose status is %q (want %q)", a.Number, tn, target.Status, want))
			}
			if target.SupersededBy != a.Number {
				add(a, "adr-supersession", fmt.Sprintf("ADR-%s: supersedes ADR-%s, whose superseded_by is %q (want %q)", a.Number, tn, target.SupersededBy, a.Number))
			}
		}
		// Full symmetry, reverse direction: a superseded record must carry the
		// matching suffixed/scalar pair and have exactly one claimant.
		if a.IsSuperseded() || a.SupersededBy != "" {
			if a.SupersededBy == "" || a.Status != "Superseded by ADR-"+a.SupersededBy {
				add(a, "adr-supersession", fmt.Sprintf("ADR-%s: status %q and superseded_by %q are not the matching suffixed/scalar pair", a.Number, a.Status, a.SupersededBy))
			}
			cs := claimants[a.Number]
			if a.SupersededBy != "" && !slices.Contains(cs, a.SupersededBy) {
				add(a, "adr-supersession", fmt.Sprintf("ADR-%s: superseded_by names ADR-%s, whose supersedes does not claim it", a.Number, a.SupersededBy))
			}
			for i := 1; i < len(cs); i++ { // a second claimant is a drift on the higher-numbered one
				add(byNum[cs[i]], "adr-supersession", fmt.Sprintf("ADR-%s: second full-supersession claim on ADR-%s (already claimed by ADR-%s)", cs[i], a.Number, cs[0]))
			}
		}

		num, _ := strconv.Atoi(a.Number) // a.Number is a 4-digit numeral matched by FilenameRe
		live := a.IsLive()
		seenAnchor := map[string]bool{}
		for _, r := range corpus.RefsOf(a.Number) {
			// Flavour exclusivity: a token into a target the same ADR also fully
			// supersedes (independent of the target's existence or status).
			tn, _ := strconv.Atoi(r.Target) // the token regex admits only digits
			if slices.Contains(a.Supersedes, tn) {
				add(a, "adr-token-exclusive", fmt.Sprintf("ADR-%s: token into ADR-%s, which it also fully supersedes", a.Number, r.Target))
			}
			// The claim key is kind-prefixed: the slug grammar admits an
			// all-digit slug, so an item ref #2 and a slug ref #2 into one
			// target are distinct anchors, never a conflict pair.
			anchor := "slug:" + r.Target + "#" + r.Slug
			if r.Slug == "" {
				anchor = "item:" + r.Target + "#" + strconv.Itoa(r.Item)
			}
			if live && !seenAnchor[anchor] {
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
			// Back-pointer: a token into a live target requires the predecessor's
			// related: to name the carrier.
			if target.IsLive() && !slices.Contains(target.Related, num) {
				add(a, "adr-token-backpointer", fmt.Sprintf("ADR-%s: token into ADR-%s lacks the related: back-pointer on the target", a.Number, r.Target))
			}
			// Advisory, not drift: a token whose target was later fully superseded
			// is immutable prose that would otherwise go permanently red.
			if target.IsSuperseded() {
				notes = append(notes, fmt.Sprintf("ADR-%s token targets ADR-%s, which was fully superseded", a.Number, r.Target))
			}
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
