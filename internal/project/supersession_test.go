package project

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

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
			testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
				testsupport.ADR("Superseded by ADR-0002", testsupport.WithTitle("0001: A"), testsupport.WithBody(tc.body)))
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
// TestCheckADRRelatedLinksParseError.
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
}
