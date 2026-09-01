package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/glossary"
)

func TestGlossaryRecordsRejectMalformedResidualCheckData(t *testing.T) {
	valid := func(fields map[string]any) []any { return []any{fields} }
	cases := map[string]any{
		"not-list":         "terms",
		"not-map":          []any{"term"},
		"non-string-key":   []any{map[any]any{42: "value"}},
		"missing-term":     valid(map[string]any{"meaning": "m"}),
		"typed-term":       valid(map[string]any{"term": 42, "meaning": "m"}),
		"empty-term":       valid(map[string]any{"term": " ", "meaning": "m"}),
		"newline-term":     valid(map[string]any{"term": "a\nb", "meaning": "m"}),
		"missing-meaning":  valid(map[string]any{"term": "t"}),
		"typed-meaning":    valid(map[string]any{"term": "t", "meaning": 42}),
		"empty-meaning":    valid(map[string]any{"term": "t", "meaning": " "}),
		"newline-meaning":  valid(map[string]any{"term": "t", "meaning": "a\nb"}),
		"typed-domains":    valid(map[string]any{"term": "t", "meaning": "m", "domains": "d"}),
		"bad-domain-entry": valid(map[string]any{"term": "t", "meaning": "m", "domains": []any{42}}),
		"unknown-key":      valid(map[string]any{"term": "t", "meaning": "m", "extra": true}),
		"duplicate":        []any{map[string]any{"term": "Term", "meaning": "one"}, map[string]any{"term": "term", "meaning": "two"}},
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := glossary.Records(raw); err == nil || !strings.Contains(err.Error(), glossary.SidecarPath) {
				t.Fatalf("glossaryRecords(%#v) error = %v", raw, err)
			}
		})
	}
}

func TestGlossaryRecordsAcceptSupportedMappingShapesAndMergeLayers(t *testing.T) {
	records, err := glossary.Records([]any{map[any]any{"term": " t ", "meaning": " m ", "domains": []any{" d "}}})
	if err != nil || len(records) != 1 || records[0].Term != "t" || records[0].Meaning != "m" || len(records[0].Domains) != 1 || records[0].Domains[0] != "d" {
		t.Fatalf("records = %#v, %v", records, err)
	}
	if records, err := glossary.Records(nil); err != nil || records != nil {
		t.Fatalf("nil records = %#v, %v", records, err)
	}

	sidecar := config.Sidecar{Data: map[string]any{
		"standardTerms": []any{map[string]any{"term": "kept", "meaning": "standard"}, map[string]any{"term": "override", "meaning": "standard"}},
		"terms":         []any{map[string]any{"term": "Override", "meaning": "authored"}},
	}}
	merged, err := glossary.Merge(sidecar)
	if err != nil || len(merged) != 2 || merged[0].Term != "kept" || merged[1].Meaning != "authored" {
		t.Fatalf("merged = %#v, %v", merged, err)
	}
	for name, sc := range map[string]config.Sidecar{
		"standard": {Data: map[string]any{"standardTerms": "bad"}},
		"authored": {Data: map[string]any{"terms": "bad"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := glossary.Merge(sc); err == nil {
				t.Fatal("malformed layer accepted")
			}
		})
	}
}
