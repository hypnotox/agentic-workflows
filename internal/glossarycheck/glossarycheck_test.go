package glossarycheck

import (
	"github.com/hypnotox/agentic-workflows/internal/glossary"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"strings"
	"testing"
)

func TestEvaluateGlossary(t *testing.T) {
	long := strings.Repeat("x", glossary.MeaningMax+1)
	result, err := Evaluate(Input{Enabled: true, Domains: []string{"known"}, Authored: []glossary.Record{{Term: "Bad", Meaning: "ok", Domains: []string{"missing"}}}, Merged: []glossary.Record{{Term: "Long", Meaning: long}}})
	if err != nil {
		t.Fatal(err)
	}
	findings := result.Findings()
	if len(findings) != 2 || findings[0].Rank != severity.Error || findings[1].Rank != severity.Warn {
		t.Fatalf("findings=%#v", findings)
	}
}
func TestEvaluateDisabled(t *testing.T) {
	result, err := Evaluate(Input{})
	if err != nil || len(result.Findings()) != 0 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}
