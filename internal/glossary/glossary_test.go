package glossary

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

func TestRecordsAndMergeOwnGlossarySemantics(t *testing.T) {
	authored := []any{map[string]any{"term": "Alpha", "meaning": "authored", "domains": []any{"rendering"}}}
	records, err := Records(authored)
	if err != nil || len(records) != 1 || records[0].Term != "Alpha" || len(records[0].Domains) != 1 {
		t.Fatalf("records = %#v, err = %v", records, err)
	}
	merged, err := Merge(config.Sidecar{Data: map[string]any{
		"standardTerms": []any{map[string]any{"term": "alpha", "meaning": "standard"}, map[string]any{"term": "Beta", "meaning": "kept"}},
		"terms":         authored,
	}})
	if err != nil || len(merged) != 2 || merged[0].Term != "Beta" || merged[1].Meaning != "authored" {
		t.Fatalf("merged = %#v, err = %v", merged, err)
	}
}

func TestRecordsRejectMalformedLayers(t *testing.T) {
	cases := []struct {
		name string
		raw  any
		want string
	}{
		{"not-list", "bad", "must be a list"},
		{"not-map", []any{"bad"}, "must be a mapping"},
		{"non-string-key", []any{map[any]any{1: "bad"}}, "key 1 is not a string"},
		{"missing-term", []any{map[string]any{"meaning": "x"}}, "missing \"term\""},
		{"bad-meaning", []any{map[string]any{"term": "x", "meaning": ""}}, "meaning is empty"},
		{"bad-domains", []any{map[string]any{"term": "x", "meaning": "x", "domains": "bad"}}, "domains\" must be a list"},
		{"unknown", []any{map[string]any{"term": "x", "meaning": "x", "extra": true}}, "unknown key"},
		{"duplicate", []any{map[string]any{"term": "x", "meaning": "x"}, map[string]any{"term": "X", "meaning": "y"}}, "case-insensitive duplicates"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Records(tc.raw); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
