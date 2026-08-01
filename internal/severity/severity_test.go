package severity_test

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func TestRankString(t *testing.T) {
	for _, tc := range []struct {
		rank severity.Rank
		want string
	}{
		{severity.Error, "error"},
		{severity.Warn, "warn"},
	} {
		if got := tc.rank.String(); got != tc.want {
			t.Fatalf("Rank(%d).String() = %q, want %q", tc.rank, got, tc.want)
		}
	}
}

func TestErrorIsZeroValue(t *testing.T) {
	var zero severity.Rank
	if zero != severity.Error {
		t.Fatalf("zero Rank = %v, want Error", zero)
	}
}

// Every rank-bearing surface renders through the one shared type, so this test
// imports the two producers and asserts their spelling directly rather than
// trusting each package's own tests. It lives here because package
// severity_test may import both without a cycle.
// invariant: tooling/audit-commands:severity-single-spelling (TestOneSpellingAcrossEveryRankSurface)
func TestOneSpellingAcrossEveryRankSurface(t *testing.T) {
	for _, tc := range []struct {
		what string
		got  string
	}{
		{"severity.Error", severity.Error.String()},
		{"severity.Warn", severity.Warn.String()},
		{"audit.Finding", audit.Finding{Severity: severity.Warn}.Severity.String()},
		{"topic.CoverageFinding", topic.CoverageFinding{Severity: severity.Error}.Severity.String()},
	} {
		if tc.got != "error" && tc.got != "warn" {
			t.Errorf("%s renders %q, want error or warn", tc.what, tc.got)
		}
		if tc.got == "warning" {
			t.Errorf("%s renders the retired warning spelling", tc.what)
		}
	}
}
