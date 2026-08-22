package vocabularycheck

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/glossary"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

func TestCheckClassifiesGlossaryAndTagPolicy(t *testing.T) {
	pitfalls, err := pitfall.Load([]pitfall.SourceFile{
		{Path: pitfall.SourceDir + "/tagged.md", Regular: true, Bytes: []byte("---\ntitle: Tagged\ntags: [known, ghost]\n---\nbody\n")},
		{Path: pitfall.SourceDir + "/untagged.md", Regular: true, Bytes: []byte("---\ntitle: Untagged\n---\nbody\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	results, err := Evaluate(Input{
		GlossaryEnabled: true,
		Authored:        []glossary.Record{{Term: "bad", Meaning: "meaning", Domains: []string{"missing"}}},
		Merged:          []glossary.Record{{Term: "long", Meaning: strings.Repeat("x", glossary.MeaningMax+1)}},
		Domains:         []string{"rendering"},
		Tags:            map[string]string{"known": "Known", "rendering": "collision", "empty": ""},
		Pitfalls:        pitfalls,
	})
	if err != nil {
		t.Fatal(err)
	}
	findings := append(results.Glossary.Findings(), results.Tags.Findings()...)
	var kinds []string
	for _, finding := range findings {
		kinds = append(kinds, finding.Evidence.Kind)
		if finding.Property != PropertyCorrectness && finding.Property != PropertyHeuristic {
			t.Fatalf("property = %q", finding.Property)
		}
		if finding.Rank != severity.Error && finding.Rank != severity.Warn {
			t.Fatalf("rank = %q", finding.Rank)
		}
	}
	joined := strings.Join(kinds, ",")
	for _, want := range []string{"glossary-domain", "advisory", "tag-vocabulary", "tag-domain-collision", "pitfall-tag"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("kinds = %v, missing %s", kinds, want)
		}
	}
	var warnings []string
	for _, finding := range findings {
		if finding.Rank == severity.Warn {
			warnings = append(warnings, finding.Evidence.Detail)
		}
	}
	if len(warnings) != 3 || !strings.Contains(strings.Join(warnings, "\n"), "carries no tags") || !strings.Contains(strings.Join(warnings, "\n"), "1/1 tagged pitfalls") {
		t.Fatalf("warnings = %v", warnings)
	}
}

func TestCheckIsInertWithoutEnabledVocabulary(t *testing.T) {
	results, err := Evaluate(Input{})
	findings := append(results.Glossary.Findings(), results.Tags.Findings()...)
	if err != nil || len(findings) != 0 {
		t.Fatalf("result = %#v, err = %v", findings, err)
	}
}
