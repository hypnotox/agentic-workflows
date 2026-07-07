package project

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// projectWithInvariants builds a bare *Project carrying the given invariant
// sources (nil = no invariants configured), for the pure marker-mapping methods.
func projectWithInvariants(sources []config.InvariantSource) *Project {
	var inv *config.InvariantConfig
	if sources != nil {
		inv = &config.InvariantConfig{Sources: sources}
	}
	return &Project{Cfg: &config.Config{Invariants: inv}}
}

func TestInvariantMarkersDisplay(t *testing.T) {
	cases := []struct {
		name    string
		sources []config.InvariantSource
		want    string
	}{
		{"nil", nil, ""},
		{"empty", []config.InvariantSource{}, ""},
		{"single", []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}, "`*.go` → `//`"},
		{"polyglot-multiglob", []config.InvariantSource{
			{Globs: []string{"*.go"}, Marker: "//"},
			{Globs: []string{"*.py", "*.pyi"}, Marker: "#"},
		}, "`*.go` → `//`, `*.py`, `*.pyi` → `#`"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := projectWithInvariants(tc.sources).invariantMarkersDisplay(); got != tc.want {
				t.Errorf("display = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestInvariantMarkerSentence(t *testing.T) {
	if got := projectWithInvariants(nil).invariantMarkerSentence(); got != "" {
		t.Errorf("nil sentence = %q, want empty", got)
	}
	got := projectWithInvariants([]config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}).invariantMarkerSentence()
	if got != "Its marker follows the file's type: `*.go` → `//`; the marker comment must open its line (indentation aside)." {
		t.Errorf("sentence = %q", got)
	}
}

func TestInvariantMarkerTable(t *testing.T) {
	if got := projectWithInvariants(nil).invariantMarkerTable(); got != "" {
		t.Errorf("nil table = %q, want empty", got)
	}
	got := projectWithInvariants([]config.InvariantSource{
		{Globs: []string{"*.go"}, Marker: "//"},
		{Globs: []string{"*.py", "*.pyi"}, Marker: "#"},
	}).invariantMarkerTable()
	want := "| files | marker |\n|---|---|\n| `*.go` | `//` |\n| `*.py`, `*.pyi` | `#` |"
	if got != want {
		t.Errorf("table =\n%q\nwant\n%q", got, want)
	}
}
