package evals

import (
	"path/filepath"
	"strings"
	"testing"
)

type adrDecisionScopeInput struct {
	durableCommitment string
	incidentalDetails []string
}

type adrDecisionScopeOutcome struct {
	retained       string
	removed        string
	destination    string
	classification string
}

func TestOverDetailedADRDecisionReviewScenario(t *testing.T) {
	input := adrDecisionScopeInput{
		durableCommitment: "Keep one durable policy owner.",
		incidentalDetails: []string{"paths", "commands", "task order", "ordinary test transactions"},
	}
	governing := []string{
		"narrowest discrete durable commitment",
		"remains meaningful after implementation",
		"Treat paths, commands, task order, rollout batches, ordinary test transactions, and comparable executor instructions as plan or direct-execution content",
		"report a misplaced directive as a reasoned finding",
	}
	want := adrDecisionScopeOutcome{
		retained:       input.durableCommitment,
		removed:        strings.Join(input.incidentalDetails, ","),
		destination:    "plan-or-direct-execution",
		classification: "reasoned",
	}

	cat := loadCatalog(t)
	for _, target := range []string{"pi", "claude"} {
		t.Run(target, func(t *testing.T) {
			root := syncFullCatalogForTarget(t, cat, target)
			body := read(t, filepath.Join(root, "."+target, "agents", "adr-reviewer.md"))
			if got := adrDecisionScopeDisposition(body, input, governing...); got != want {
				t.Fatalf("over-detailed ADR Decision: got %#v, want %#v", got, want)
			}
			for _, clause := range governing {
				mutated := strings.ReplaceAll(body, clause, "missing-governing-clause")
				if got := adrDecisionScopeDisposition(mutated, input, governing...); got == want {
					t.Errorf("over-detailed ADR Decision still yields the complete reviewer disposition without %q", clause)
				}
			}
		})
	}
}

func adrDecisionScopeDisposition(body string, input adrDecisionScopeInput, governing ...string) adrDecisionScopeOutcome {
	for _, clause := range governing {
		if !strings.Contains(body, clause) {
			return adrDecisionScopeOutcome{}
		}
	}
	return adrDecisionScopeOutcome{
		retained:       input.durableCommitment,
		removed:        strings.Join(input.incidentalDetails, ","),
		destination:    "plan-or-direct-execution",
		classification: "reasoned",
	}
}
