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
			// The checked ADR is Superseded (a symmetric pair, so only the format
			// rule can drift): the format check applies regardless of status.
			testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
				testsupport.ADR("Superseded by ADR-0002", testsupport.WithTitle("0001: A"),
					testsupport.WithSupersededBy("0002"), testsupport.WithBody(tc.body)))
			testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0002-b.md"),
				testsupport.ADR("Implemented", testsupport.WithTitle("0002: B"),
					testsupport.WithSupersedes(1), testsupport.WithBody("## Decision\n\n1. x.\n")))
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

// TestFullSupersessionSymmetry covers the three-way symmetry check: a
// symmetric pair passes; each one-sided form fails; a second full claimant
// fails on the higher-numbered claimant.
// invariant: supersession-full-symmetry
func TestFullSupersessionSymmetry(t *testing.T) {
	symmetricOld := testsupport.ADR("Superseded by ADR-0002", testsupport.WithTitle("0001: Old"),
		testsupport.WithSupersededBy("0002"), testsupport.WithBody(decision))
	cases := []struct {
		name       string
		files      map[string]string
		wantDetail string // "" = no drift expected
	}{
		{
			name: "symmetric pair passes",
			files: map[string]string{
				"0001-old.md": symmetricOld,
				"0002-new.md": testsupport.ADR("Implemented", testsupport.WithTitle("0002: New"),
					testsupport.WithSupersedes(1), testsupport.WithBody(decision)),
			},
		},
		{
			name: "claim without status flip",
			files: map[string]string{
				"0001-old.md": testsupport.ADR("Accepted", testsupport.WithTitle("0001: Old"), testsupport.WithBody(decision)),
				"0002-new.md": testsupport.ADR("Implemented", testsupport.WithTitle("0002: New"),
					testsupport.WithSupersedes(1), testsupport.WithBody(decision)),
			},
			wantDetail: `whose status is "Accepted"`,
		},
		{
			name: "claim of a missing target",
			files: map[string]string{
				"0002-new.md": testsupport.ADR("Implemented", testsupport.WithTitle("0002: New"),
					testsupport.WithSupersedes(1), testsupport.WithBody(decision)),
			},
			wantDetail: "which does not exist",
		},
		{
			name: "status flip without claimant",
			files: map[string]string{
				"0001-old.md": symmetricOld,
				"0002-new.md": testsupport.ADR("Implemented", testsupport.WithTitle("0002: New"), testsupport.WithBody(decision)),
			},
			wantDetail: "whose supersedes does not claim it",
		},
		{
			name: "suffixed status without superseded_by",
			files: map[string]string{
				"0001-old.md": testsupport.ADR("Superseded by ADR-0002", testsupport.WithTitle("0001: Old"), testsupport.WithBody(decision)),
				"0002-new.md": testsupport.ADR("Implemented", testsupport.WithTitle("0002: New"),
					testsupport.WithSupersedes(1), testsupport.WithBody(decision)),
			},
			wantDetail: "not the matching suffixed/scalar pair",
		},
		{
			name: "superseded_by without suffixed status",
			files: map[string]string{
				"0001-old.md": testsupport.ADR("Accepted", testsupport.WithTitle("0001: Old"),
					testsupport.WithSupersededBy("0002"), testsupport.WithBody(decision)),
				"0002-new.md": testsupport.ADR("Implemented", testsupport.WithTitle("0002: New"),
					testsupport.WithSupersedes(1), testsupport.WithBody(decision)),
			},
			wantDetail: "not the matching suffixed/scalar pair",
		},
		{
			name: "second claimant drifts on the higher-numbered one",
			files: map[string]string{
				"0001-old.md": symmetricOld,
				"0002-new.md": testsupport.ADR("Implemented", testsupport.WithTitle("0002: New"),
					testsupport.WithSupersedes(1), testsupport.WithBody(decision)),
				"0003-again.md": testsupport.ADR("Implemented", testsupport.WithTitle("0003: Again"),
					testsupport.WithSupersedes(1), testsupport.WithBody(decision)),
			},
			wantDetail: "ADR-0003: second full-supersession claim on ADR-0001 (already claimed by ADR-0002)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			drift, notes := runSupersession(t, tc.files)
			if len(notes) != 0 {
				t.Errorf("want no notes, got %#v", notes)
			}
			if tc.wantDetail == "" {
				if drift != nil {
					t.Fatalf("want no drift, got %#v", drift)
				}
				return
			}
			if !hasKindDetail(drift, "adr-supersession", tc.wantDetail) {
				t.Fatalf("want adr-supersession drift containing %q, got %#v", tc.wantDetail, drift)
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
// live target without the related: back-pointer fails; with it, the corpus is
// clean.
// invariant: supersession-backpointer
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
				testsupport.WithRelated(2), testsupport.WithBody(decision)),
			"0002-carrier.md": carrier,
		})
		if drift != nil || len(notes) != 0 {
			t.Fatalf("want clean corpus, got drift %#v notes %#v", drift, notes)
		}
	})
}

// TestTokenFlavourExclusive covers adr-token-exclusive: one successor may not
// both fully and partially supersede the same target.
// invariant: supersession-flavour-exclusive
func TestTokenFlavourExclusive(t *testing.T) {
	drift, _ := runSupersession(t, map[string]string{
		"0001-old.md": testsupport.ADR("Superseded by ADR-0002", testsupport.WithTitle("0001: Old"),
			testsupport.WithSupersededBy("0002"), testsupport.WithBody(decision)),
		"0002-new.md": testsupport.ADR("Implemented", testsupport.WithTitle("0002: New"),
			testsupport.WithSupersedes(1),
			testsupport.WithBody("## Decision\n\n1. Also overrides `supersedes: ADR-0001#1`.\n")),
	})
	if !hasKindDetail(drift, "adr-token-exclusive", "ADR-0002: token into ADR-0001, which it also fully supersedes") {
		t.Fatalf("want adr-token-exclusive drift, got %#v", drift)
	}
}

// TestSupersessionAdvisories covers the two note channels: a token into a
// fully superseded target yields a note and no drift; one anchor claimed by
// two live ADRs yields a note.
// invariant: supersession-conflict-advisory
func TestSupersessionAdvisories(t *testing.T) {
	t.Run("token into a superseded target is a note, not drift", func(t *testing.T) {
		drift, notes := runSupersession(t, map[string]string{
			"0001-old.md": testsupport.ADR("Superseded by ADR-0002", testsupport.WithTitle("0001: Old"),
				testsupport.WithSupersededBy("0002"), testsupport.WithBody(decision)),
			"0002-new.md": testsupport.ADR("Implemented", testsupport.WithTitle("0002: New"),
				testsupport.WithSupersedes(1), testsupport.WithBody(decision)),
			"0003-citer.md": testsupport.ADR("Implemented", testsupport.WithTitle("0003: Citer"),
				testsupport.WithBody("## Decision\n\n1. Cites `supersedes: ADR-0001#1`.\n")),
		})
		if drift != nil {
			t.Fatalf("want no drift, got %#v", drift)
		}
		if len(notes) != 1 || notes[0] != "ADR-0003 token targets ADR-0001, which was fully superseded" {
			t.Fatalf("want the superseded-target note, got %#v", notes)
		}
	})
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
