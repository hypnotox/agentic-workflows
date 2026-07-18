package project

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

const supersessionCfg = "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\n"

// TestCheckDecisionFormat exercises the ADR-0120 item 12 rule: column-0
// numbered Decision items, sequential from 1, regardless of status.
// invariant: decision-items-enumerable
func TestCheckDecisionFormat(t *testing.T) {
	var seq strings.Builder
	seq.WriteString("## Decision\n\n")
	for i := 1; i <= 13; i++ {
		fmt.Fprintf(&seq, "%d. Item.\n", i)
	}
	cases := []struct {
		name       string
		body       string
		wantDetail string // "" = no drift expected
	}{
		{"no items", "## Decision\n\nProse only.\n", "no column-0 numbered items"},
		{"gap", "## Decision\n\n1. a.\n3. b.\n", "item 3 found where 2 expected"},
		{"duplicate", "## Decision\n\n1. a.\n1. b.\n", "item 1 found where 2 expected"},
		{"restart", "## Decision\n\n1. a.\n2. b.\n1. c.\n", "item 1 found where 3 expected"},
		{"multi-digit sequence to 13", seq.String(), ""},
		{"indented sub-list does not count", "## Decision\n\n1. a.\n   1. sub.\n2. b.\n", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffold(t, supersessionCfg)
			// The checked ADR is a plain live record: the format rule applies
			// regardless of status, and keeping the fixture unsuperseded means
			// only the format rule can drift.
			testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
				testsupport.ADR("Implemented", testsupport.WithTitle("0001: A"),
					testsupport.WithBody(tc.body)))
			testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0002-b.md"),
				testsupport.ADR("Implemented", testsupport.WithTitle("0002: B"),
					testsupport.WithBody("## Decision\n\n1. x.\n")))
			p, err := Open(root)
			if err != nil {
				t.Fatal(err)
			}
			drift, err := p.checkSupersessionAll()
			if err != nil {
				t.Fatalf("checkSupersessionAll: %v", err)
			}
			if tc.wantDetail == "" {
				if drift != nil {
					t.Fatalf("want no drift, got %#v", drift)
				}
				return
			}
			if len(drift) != 1 || drift[0].Kind != "adr-decision-format" ||
				drift[0].Path != "docs/decisions/0001-a.md" || !strings.Contains(drift[0].Detail, tc.wantDetail) {
				t.Fatalf("want one adr-decision-format drift containing %q, got %#v", tc.wantDetail, drift)
			}
		})
	}
}

// The adr.ParseDir branch is reachable via a direct call over a malformed ADR
// (pre-empted only inside full Check() by checkPlans) - mirroring
// TestCheckADRRelatedLinksParseError. Both consumers of computeSupersession
// share the branch shape.
func TestCheckSupersessionAllParseError(t *testing.T) {
	root := scaffold(t, supersessionCfg)
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-broken.md"),
		"---\nstatus: [unterminated\n---\n# ADR-0001: Broken\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.checkSupersessionAll(); err == nil {
		t.Fatal("expected adr.ParseDir error, got nil")
	}
	if _, err := p.supersessionNotes(); err == nil {
		t.Fatal("expected adr.ParseDir error from supersessionNotes, got nil")
	}
}

// TestAdvisoryNotesSurfacesSupersessionError covers the supersessionNotes error
// propagation wired into AdvisoryNotes - mirroring
// TestAdvisoryNotesSurfacesPlanCommitError: empty tags keep tagHealthNotes
// inert, no plans keep planCommitScopeNotes inert, and the malformed ADR fails
// supersessionNotes' ParseDir.
func TestAdvisoryNotesSurfacesSupersessionError(t *testing.T) {
	root := scaffold(t, supersessionCfg)
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-broken.md"),
		"---\nstatus: [unterminated\n---\n# ADR-0001: Broken\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.AdvisoryNotes(); err == nil {
		t.Fatal("expected AdvisoryNotes to surface the supersession ParseDir error")
	}
}

// decision is a minimal conforming Decision section for fixtures whose token
// content does not matter; body appends after its single item.
const decision = "## Decision\n\n1. x.\n"

// twoItems is for fixtures whose target must survive a retirement: with a
// single anchor, one `supersedes:` token covers the whole ADR and the coverage
// check then rightly demands the status flip, which is not what those tests
// are about.
const twoItems = "## Decision\n\n1. x.\n2. y.\n"

// runSupersession scaffolds a corpus from files, runs both computeSupersession
// consumers, and returns (drift, notes).
func runSupersession(t *testing.T, files map[string]string) ([]manifest.Drift, []string) {
	t.Helper()
	root := scaffold(t, supersessionCfg)
	for name, content := range files {
		testsupport.WriteFile(t, filepath.Join(root, "docs/decisions", name), content)
	}
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.checkSupersessionAll()
	if err != nil {
		t.Fatalf("checkSupersessionAll: %v", err)
	}
	notes, err := p.supersessionNotes()
	if err != nil {
		t.Fatalf("supersessionNotes: %v", err)
	}
	return drift, notes
}

// hasKindDetail reports whether drift contains a d of the given kind whose
// detail contains want.
func hasKindDetail(drift []manifest.Drift, kind, want string) bool {
	for _, d := range drift {
		if d.Kind == kind && strings.Contains(d.Detail, want) {
			return true
		}
	}
	return false
}

// TestCheckRetiredKey covers the raw-key refusal (ADR-0120 item 7): the
// removed retires_invariants: key drifts whether empty or non-empty; a clean
// file does not.
// invariant: retires-invariants-key-refused
func TestCheckRetiredKey(t *testing.T) {
	cases := []struct {
		name  string
		fm    string
		wantN int
	}{
		{"empty key drifts", "retires_invariants: []\n", 1},
		{"non-empty key drifts", "retires_invariants: [some-slug]\n", 1},
		{"clean file does not", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffold(t, supersessionCfg)
			testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
				"---\nstatus: Implemented\n"+tc.fm+"---\n# ADR-0001: A\n\n"+decision)
			p, err := Open(root)
			if err != nil {
				t.Fatal(err)
			}
			drift, err := p.checkSupersessionAll()
			if err != nil {
				t.Fatalf("checkSupersessionAll: %v", err)
			}
			var got []manifest.Drift
			for _, d := range drift {
				if d.Kind == "adr-retired-key" {
					got = append(got, d)
				}
			}
			if len(got) != tc.wantN {
				t.Fatalf("want %d adr-retired-key drift(s), got %#v", tc.wantN, drift)
			}
			if tc.wantN == 1 && !strings.Contains(got[0].Detail, "run awf upgrade") {
				t.Errorf("detail must route to awf upgrade, got %q", got[0].Detail)
			}
		})
	}
}

// TestTokenRefValidity covers the adr-token-ref check: a dangling target, an
// out-of-range item, an unknown slug, and a Proposed target each fail.
// invariant: supersession-token-ref-validity
func TestTokenRefValidity(t *testing.T) {
	cases := []struct {
		name       string
		files      map[string]string
		wantDetail string
	}{
		{
			name: "dangling item ref",
			files: map[string]string{
				"0002-carrier.md": testsupport.ADR("Implemented", testsupport.WithTitle("0002: Carrier"),
					testsupport.WithBody("## Decision\n\n1. Overrides `supersedes: ADR-0042#1`.\n")),
			},
			wantDetail: "token targets ADR-0042, which does not exist",
		},
		{
			name: "out-of-range item",
			files: map[string]string{
				"0001-target.md": testsupport.ADR("Accepted", testsupport.WithTitle("0001: Target"),
					testsupport.WithRelated(2), testsupport.WithBody(decision)),
				"0002-carrier.md": testsupport.ADR("Implemented", testsupport.WithTitle("0002: Carrier"),
					testsupport.WithBody("## Decision\n\n1. Overrides `supersedes: ADR-0001#2`.\n")),
			},
			wantDetail: "token cites ADR-0001#2, but its Decision has 1 items",
		},
		{
			name: "unknown slug",
			files: map[string]string{
				"0001-target.md": testsupport.ADR("Accepted", testsupport.WithTitle("0001: Target"),
					testsupport.WithRelated(2), testsupport.WithBody(decision+"## Invariants\n- `invariant: real-slug` - x.\n")),
				"0002-carrier.md": testsupport.ADR("Implemented", testsupport.WithTitle("0002: Carrier"),
					testsupport.WithBody("## Decision\n\n1. Retires `supersedes-invariant: ADR-0001#ghost-slug`.\n")),
			},
			wantDetail: "token cites ADR-0001#ghost-slug, which its Invariants section does not declare",
		},
		{
			name: "Proposed target",
			files: map[string]string{
				"0001-target.md": testsupport.ADR("Proposed", testsupport.WithTitle("0001: Target"),
					testsupport.WithRelated(2), testsupport.WithBody(decision)),
				"0002-carrier.md": testsupport.ADR("Implemented", testsupport.WithTitle("0002: Carrier"),
					testsupport.WithBody("## Decision\n\n1. Overrides `supersedes: ADR-0001#1`.\n")),
			},
			wantDetail: "token targets Proposed ADR-0001, whose body is still mutable",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			drift, _ := runSupersession(t, tc.files)
			if !hasKindDetail(drift, "adr-token-ref", tc.wantDetail) {
				t.Fatalf("want adr-token-ref drift containing %q, got %#v", tc.wantDetail, drift)
			}
		})
	}
}

// TestTokenBackpointer covers the adr-token-backpointer check: a token into a
// target without the related: back-pointer fails; with it, the corpus is clean.
// Since ADR-0128 item 5 the target's status is irrelevant - see the superseded
// subtest, which is the case the live-only guard used to let through.
// invariant: supersession-backpointer-any-status
func TestTokenBackpointer(t *testing.T) {
	carrier := testsupport.ADR("Implemented", testsupport.WithTitle("0002: Carrier"),
		testsupport.WithBody("## Decision\n\n1. Overrides `supersedes: ADR-0001#1`.\n"))
	t.Run("live target without back-pointer fails", func(t *testing.T) {
		drift, _ := runSupersession(t, map[string]string{
			"0001-target.md": testsupport.ADR("Accepted", testsupport.WithTitle("0001: Target"),
				testsupport.WithBody(decision)),
			"0002-carrier.md": carrier,
		})
		if !hasKindDetail(drift, "adr-token-backpointer", "ADR-0002: token into ADR-0001 lacks the related: back-pointer") {
			t.Fatalf("want adr-token-backpointer drift, got %#v", drift)
		}
	})
	t.Run("with back-pointer passes", func(t *testing.T) {
		drift, notes := runSupersession(t, map[string]string{
			"0001-target.md": testsupport.ADR("Accepted", testsupport.WithTitle("0001: Target"),
				testsupport.WithRelated(2), testsupport.WithBody(twoItems)),
			"0002-carrier.md": carrier,
		})
		if drift != nil || len(notes) != 0 {
			t.Fatalf("want clean corpus, got drift %#v notes %#v", drift, notes)
		}
	})
}

// TestSupersessionAdvisories covers the surviving note channel: one anchor
// claimed by two live ADRs' RETIREMENTS. The superseded-target note is gone
// with ADR-0128 item 6 - it is now the normal shape of every completed
// supersedence.
// invariant: supersession-contested-anchor-advisory
func TestSupersessionAdvisories(t *testing.T) {
	t.Run("same anchor claimed by two live ADRs is a note", func(t *testing.T) {
		token := "## Decision\n\n1. Overrides `supersedes: ADR-0001#1`.\n"
		_, notes := runSupersession(t, map[string]string{
			"0001-target.md": testsupport.ADR("Accepted", testsupport.WithTitle("0001: Target"),
				testsupport.WithRelated(2, 3), testsupport.WithBody(decision)),
			"0002-first.md":  testsupport.ADR("Implemented", testsupport.WithTitle("0002: First"), testsupport.WithBody(token)),
			"0003-second.md": testsupport.ADR("Accepted", testsupport.WithTitle("0003: Second"), testsupport.WithBody(token)),
		})
		want := "anchor ADR-0001#1 claimed by ADR-0002 and ADR-0003"
		if len(notes) != 1 || notes[0] != want {
			t.Fatalf("want %q, got %#v", want, notes)
		}
	})
	t.Run("an item ref and an all-digit slug ref are distinct anchors", func(t *testing.T) {
		// The slug grammar admits an all-digit slug ("2"); the conflict
		// advisory keys anchors by kind, so item #2 and slug #2 into one
		// target never pair.
		_, notes := runSupersession(t, map[string]string{
			"0001-target.md": testsupport.ADR("Accepted", testsupport.WithTitle("0001: Target"),
				testsupport.WithRelated(2, 3),
				testsupport.WithBody("## Decision\n\n1. a.\n2. b.\n\n## Invariants\n\n- `invariant: 2` - an all-digit slug.\n")),
			"0002-first.md": testsupport.ADR("Implemented", testsupport.WithTitle("0002: First"),
				testsupport.WithBody("## Decision\n\n1. Overrides `supersedes: ADR-0001#2`.\n")),
			"0003-second.md": testsupport.ADR("Accepted", testsupport.WithTitle("0003: Second"),
				testsupport.WithBody("## Decision\n\n1. Retires `supersedes-invariant: ADR-0001#2`.\n")),
		})
		if len(notes) != 0 {
			t.Fatalf("want no conflict note across anchor kinds, got %#v", notes)
		}
	})
	t.Run("two refinements of one anchor do not contest", func(t *testing.T) {
		// Refinements do not contest (ADR-0128 item 6): several ADRs adapting
		// one decision is the normal shape of an evolving decision, not two
		// ADRs declaring it dead.
		token := "## Decision\n\n1. Adapts `refines: ADR-0001#1`.\n"
		_, notes := runSupersession(t, map[string]string{
			"0001-target.md": testsupport.ADR("Accepted", testsupport.WithTitle("0001: Target"),
				testsupport.WithRelated(2, 3), testsupport.WithBody(decision)),
			"0002-first.md":  testsupport.ADR("Implemented", testsupport.WithTitle("0002: First"), testsupport.WithBody(token)),
			"0003-second.md": testsupport.ADR("Accepted", testsupport.WithTitle("0003: Second"), testsupport.WithBody(token)),
		})
		if len(notes) != 0 {
			t.Fatalf("refinements must not contest, got %#v", notes)
		}
	})
	t.Run("a refinement and a later retirement do not contest", func(t *testing.T) {
		// The mixed pair ADR-0128 item 6 names as healthy, and the shape this
		// repo's own corpus carries on ADR-0034 item 1 (refined by ADR-0057,
		// retired by ADR-0121). It emitted a spurious note until the advisory
		// was scoped to retirements.
		_, notes := runSupersession(t, map[string]string{
			"0001-target.md": testsupport.ADR("Accepted", testsupport.WithTitle("0001: Target"),
				testsupport.WithRelated(2, 3), testsupport.WithBody(decision)),
			"0002-refiner.md": testsupport.ADR("Implemented", testsupport.WithTitle("0002: Refiner"),
				testsupport.WithBody("## Decision\n\n1. Adapts `refines: ADR-0001#1`.\n")),
			"0003-retirer.md": testsupport.ADR("Accepted", testsupport.WithTitle("0003: Retirer"),
				testsupport.WithBody("## Decision\n\n1. Retires `supersedes: ADR-0001#1`.\n")),
		})
		if len(notes) != 0 {
			t.Fatalf("a refine-then-retire pair must not contest, got %#v", notes)
		}
	})
	t.Run("a Proposed claimant is not in force", func(t *testing.T) {
		token := "## Decision\n\n1. Overrides `supersedes: ADR-0001#1`.\n"
		_, notes := runSupersession(t, map[string]string{
			"0001-target.md": testsupport.ADR("Accepted", testsupport.WithTitle("0001: Target"),
				testsupport.WithRelated(2, 3), testsupport.WithBody(decision)),
			"0002-first.md":  testsupport.ADR("Implemented", testsupport.WithTitle("0002: First"), testsupport.WithBody(token)),
			"0003-second.md": testsupport.ADR("Proposed", testsupport.WithTitle("0003: Second"), testsupport.WithBody(token)),
		})
		if len(notes) != 0 {
			t.Fatalf("want no conflict note with a Proposed claimant, got %#v", notes)
		}
	})
}

// TestTokenBackpointerAnyStatus pins ADR-0128 item 5 directly: a token into a
// Superseded target owes the back-pointer just as a live one does. Without
// this, a claimant landing after the target's flip owed nothing and became
// invisible - and bare `Superseded` names no successor, so the back-pointer is
// the only thing left that can recover the claimant.
// invariant: supersession-backpointer-any-status
func TestTokenBackpointerAnyStatus(t *testing.T) {
	carrier := testsupport.ADR("Implemented", testsupport.WithTitle("0002: Carrier"),
		testsupport.WithBody("## Decision\n\n1. Overrides `supersedes: ADR-0001#1`.\n"))
	// Proposed is deliberately absent: a token into a Proposed target is
	// refused earlier as adr-token-ref (its body is still mutable) and never
	// reaches the back-pointer check.
	for _, status := range []string{"Superseded", "Superseded by ADR-0009"} {
		t.Run("target "+status+" without back-pointer fails", func(t *testing.T) {
			drift, _ := runSupersession(t, map[string]string{
				"0001-target.md": testsupport.ADR(status, testsupport.WithTitle("0001: Target"),
					testsupport.WithBody(decision)),
				"0002-carrier.md": carrier,
			})
			if !hasKindDetail(drift, "adr-token-backpointer", "ADR-0001") {
				t.Fatalf("want a back-pointer drift for a %s target, got %#v", status, drift)
			}
		})
	}
	t.Run("superseded target with back-pointer is clean", func(t *testing.T) {
		drift, _ := runSupersession(t, map[string]string{
			"0001-target.md": testsupport.ADR("Superseded", testsupport.WithTitle("0001: Target"),
				testsupport.WithRelated(2), testsupport.WithBody(decision)),
			"0002-carrier.md": carrier,
		})
		if hasKindDetail(drift, "adr-token-backpointer", "ADR-0001") {
			t.Fatalf("want no back-pointer drift, got %#v", drift)
		}
	})
}

// TestSupersessionGraphFaults covers ADR-0129 item 7: irreflexivity, and
// acyclicity scoped to covered ADRs. A cycle among live ADRs is deliberately
// legal - two current ADRs may each refine or retire some of the other's
// anchors - so the check fires only when every member is fully retired, where
// the loop asserts each is dead because the other is.
// invariant: supersession-graph-acyclic
func TestSupersessionGraphFaults(t *testing.T) {
	t.Run("an ADR claiming its own anchor is drift", func(t *testing.T) {
		drift, _ := runSupersession(t, map[string]string{
			"0001-self.md": testsupport.ADR("Implemented", testsupport.WithTitle("0001: Self"),
				testsupport.WithRelated(1),
				testsupport.WithBody("## Decision\n\n1. a.\n2. Retires `supersedes: ADR-0001#1`.\n")),
		})
		if !hasKindDetail(drift, "adr-supersession-graph", "claims its own anchor ADR-0001#1") {
			t.Fatalf("want a self-claim drift, got %#v", drift)
		}
	})

	t.Run("a cycle between two fully covered ADRs is drift", func(t *testing.T) {
		// Both carriers are Implemented, which is what makes their retirements
		// count; each ADR's single item is then retired by the other, so both
		// derive Covered and the loop closes.
		drift, _ := runSupersession(t, map[string]string{
			"0001-a.md": testsupport.ADR("Implemented", testsupport.WithTitle("0001: A"),
				testsupport.WithRelated(2),
				testsupport.WithBody("## Decision\n\n1. Retires `supersedes: ADR-0002#1`.\n")),
			"0002-b.md": testsupport.ADR("Implemented", testsupport.WithTitle("0002: B"),
				testsupport.WithRelated(1),
				testsupport.WithBody("## Decision\n\n1. Retires `supersedes: ADR-0001#1`.\n")),
		})
		if !hasKindDetail(drift, "adr-supersession-graph", "retirement cycle") {
			t.Fatalf("want a cycle drift, got %#v", drift)
		}
	})

	t.Run("mutual claims between live ADRs are legal", func(t *testing.T) {
		// Each retires one of the other's two items, so neither is covered and
		// the loop is a legitimate pair of partial supersessions.
		drift, _ := runSupersession(t, map[string]string{
			"0001-a.md": testsupport.ADR("Implemented", testsupport.WithTitle("0001: A"),
				testsupport.WithRelated(2),
				testsupport.WithBody("## Decision\n\n1. a.\n2. Retires `supersedes: ADR-0002#1`.\n")),
			"0002-b.md": testsupport.ADR("Implemented", testsupport.WithTitle("0002: B"),
				testsupport.WithRelated(1),
				testsupport.WithBody("## Decision\n\n1. b.\n2. Retires `supersedes: ADR-0001#1`.\n")),
		})
		if hasKindDetail(drift, "adr-supersession-graph", "retirement cycle") {
			t.Fatalf("mutual partial claims between live ADRs must be legal, got %#v", drift)
		}
	})
}

// TestSupersessionKeysRefused pins ADR-0128 item 1: neither removed frontmatter
// key is silently ignored. Non-strict YAML would drop them, so an author who
// still believed `supersedes:` was load-bearing would believe an ADR was
// superseded when nothing had superseded it. The finding must carry upgrade
// guidance, exactly as the retires_invariants: refusal does.
// invariant: supersession-keys-refused
func TestSupersessionKeysRefused(t *testing.T) {
	for _, key := range []string{"supersedes: [1]", `superseded_by: "0002"`} {
		t.Run(key, func(t *testing.T) {
			root := scaffold(t, supersessionCfg)
			testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
				"---\nstatus: Implemented\ndate: 2026-01-01\ntags: [tooling]\nrelated: []\ndomains: []\n"+key+
					"\n---\n# ADR-0001: A\n\n## Decision\n\n1. x.\n")
			p, err := Open(root)
			if err != nil {
				t.Fatal(err)
			}
			drift, err := p.checkSupersessionAll()
			if err != nil {
				t.Fatal(err)
			}
			if !hasKindDetail(drift, "adr-retired-key", "is no longer read") {
				t.Fatalf("want a refusal for %q, got %#v", key, drift)
			}
			if !hasKindDetail(drift, "adr-retired-key", "awf upgrade") {
				t.Fatalf("the refusal must name awf upgrade as the remedy, got %#v", drift)
			}
		})
	}
}

// TestCoverageDerivesStatus pins ADR-0128 items 3, 4 and 6: the hand-written
// status is checked against derived coverage in both directions, only
// retirements from Implemented carriers count, and refinements count toward
// nothing.
// invariant: supersession-coverage-derives-status
// invariant: supersession-coverage-implemented-only
// invariant: refines-token-never-covers
func TestCoverageDerivesStatus(t *testing.T) {
	target := func(status string) string {
		return testsupport.ADR(status, testsupport.WithTitle("0001: Target"),
			testsupport.WithRelated(2), testsupport.WithBody(twoItems))
	}
	carrier := func(status, body string) string {
		return testsupport.ADR(status, testsupport.WithTitle("0002: Carrier"), testsupport.WithBody(body))
	}
	const retiresBoth = "## Decision\n\n1. Retires `supersedes: ADR-0001#1`, `supersedes: ADR-0001#2`.\n"
	const refinesBoth = "## Decision\n\n1. Adapts `refines: ADR-0001#1`, `refines: ADR-0001#2`.\n"

	cases := []struct {
		name       string
		targetSt   string
		carrierSt  string
		carrierAt  string
		wantDetail string
	}{
		{
			name: "fully covered but not flipped is drift", targetSt: "Implemented",
			carrierSt: "Implemented", carrierAt: retiresBoth,
			wantDetail: "so status must be Superseded",
		},
		{
			name: "flipped without full coverage is drift", targetSt: "Superseded",
			carrierSt: "Implemented", carrierAt: "## Decision\n\n1. Retires `supersedes: ADR-0001#1`.\n",
			wantDetail: "carry no retirement from an Implemented ADR",
		},
		{
			// A Proposed successor must not kill its predecessor: it has not
			// shipped, so the predecessor is still the current guidance.
			name: "retirements from a Proposed carrier do not cover", targetSt: "Superseded",
			carrierSt: "Proposed", carrierAt: retiresBoth,
			wantDetail: "carry no retirement from an Implemented ADR",
		},
		{
			// Refinements adapt rather than replace, so an ADR whose every item
			// is refined is still live guidance.
			name: "refinements never cover", targetSt: "Superseded",
			carrierSt: "Implemented", carrierAt: refinesBoth,
			wantDetail: "carry no retirement from an Implemented ADR",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			drift, _ := runSupersession(t, map[string]string{
				"0001-target.md":  target(tc.targetSt),
				"0002-carrier.md": carrier(tc.carrierSt, tc.carrierAt),
			})
			if !hasKindDetail(drift, "adr-coverage-status", tc.wantDetail) {
				t.Fatalf("want %q, got %#v", tc.wantDetail, drift)
			}
		})
	}

	t.Run("covered and flipped is clean", func(t *testing.T) {
		drift, _ := runSupersession(t, map[string]string{
			"0001-target.md":  target("Superseded"),
			"0002-carrier.md": carrier("Implemented", retiresBoth),
		})
		if hasKindDetail(drift, "adr-coverage-status", "") {
			t.Fatalf("want no coverage drift, got %#v", drift)
		}
	})
}
