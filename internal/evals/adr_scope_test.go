package evals

import (
	"path/filepath"
	"strings"
	"testing"
)

type adrDecisionItem struct {
	text string
	kind string
}

type adrDecisionScopeInput struct {
	items []adrDecisionItem
}

type adrDecisionScopeOutcome struct {
	retained       string
	removed        string
	destination    string
	classification string
}

func TestOverDetailedADRDecisionReviewScenario(t *testing.T) {
	const durableCommitment = "Keep one durable policy owner."
	incidentalKinds := []string{"paths", "commands", "task order", "ordinary test transactions"}
	input := adrDecisionScopeInput{items: []adrDecisionItem{{text: durableCommitment, kind: "durable"}}}
	for _, kind := range incidentalKinds {
		input.items = append(input.items, adrDecisionItem{text: "executor detail for " + kind, kind: kind})
	}
	want := adrDecisionScopeOutcome{
		retained:       durableCommitment,
		removed:        strings.Join(incidentalKinds, ","),
		destination:    "plan-or-direct-execution",
		classification: "reasoned",
	}

	cat := loadCatalog(t)
	for _, target := range []string{"pi", "claude"} {
		t.Run(target, func(t *testing.T) {
			root := cloneFullCatalogForTarget(t, cat, target)
			body := read(t, filepath.Join(root, "."+target, "agents", "adr-reviewer.md"))
			if got := adrDecisionScopeDisposition(body, input); got != want {
				t.Fatalf("over-detailed ADR Decision: got %#v, want %#v", got, want)
			}

			for _, kind := range incidentalKinds {
				single := adrDecisionScopeInput{items: []adrDecisionItem{
					{text: durableCommitment, kind: "durable"},
					{text: "executor detail for " + kind, kind: kind},
				}}
				got := adrDecisionScopeDisposition(body, single)
				if got.retained != durableCommitment || got.removed != kind || got.destination != "plan-or-direct-execution" || got.classification != "reasoned" {
					t.Errorf("%s did not independently affect reviewer disposition: %#v", kind, got)
				}
			}

			mutations := map[string][2]string{
				"durable retention": {"narrowest discrete durable commitment", "remove every commitment"},
				"lifetime test":     {"remains meaningful after implementation", "expires after implementation"},
				"destination":       {"as plan or direct-execution content", "as ADR Decision content"},
				"classification":    {"report a misplaced directive as a reasoned finding", "accept a misplaced directive without a finding"},
			}
			for name, mutation := range mutations {
				mutated := strings.ReplaceAll(body, mutation[0], mutation[1])
				if got := adrDecisionScopeDisposition(mutated, input); got == want {
					t.Errorf("over-detailed ADR Decision accepted contradictory %s guidance %q", name, mutation[1])
				}
			}
			for _, kind := range incidentalKinds {
				mutated := strings.ReplaceAll(body, kind, "retained "+kind)
				if got := adrDecisionScopeDisposition(mutated, input); strings.Contains(got.removed, kind) {
					t.Errorf("reviewer still removed %q after its routing category was contradicted: %#v", kind, got)
				}
			}
		})
	}
}

func adrDecisionScopeDisposition(body string, input adrDecisionScopeInput) adrDecisionScopeOutcome {
	directiveKinds, destination := adrDirectiveRouting(body)
	outcome := adrDecisionScopeOutcome{destination: destination}
	if strings.Contains(body, "report a misplaced directive as a reasoned finding") {
		outcome.classification = "reasoned"
	}
	var retained []string
	var removed []string
	for _, item := range input.items {
		switch item.kind {
		case "durable":
			if strings.Contains(body, "narrowest discrete durable commitment") && strings.Contains(body, "remains meaningful after implementation") {
				retained = append(retained, item.text)
			}
		default:
			if directiveKinds[item.kind] {
				removed = append(removed, item.kind)
			}
		}
	}
	outcome.retained = strings.Join(retained, ",")
	outcome.removed = strings.Join(removed, ",")
	return outcome
}

func adrDirectiveRouting(body string) (map[string]bool, string) {
	const prefix = "Treat "
	const suffix = " as plan or direct-execution content"
	start := strings.Index(body, prefix)
	if start < 0 {
		return nil, ""
	}
	start += len(prefix)
	end := strings.Index(body[start:], suffix)
	if end < 0 {
		return nil, ""
	}
	directive := body[start : start+end]
	kinds := make(map[string]bool)
	for part := range strings.SplitSeq(directive, ",") {
		part = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(part), "and "))
		switch part {
		case "paths", "commands", "task order", "rollout batches", "ordinary test transactions":
			kinds[part] = true
		}
	}
	return kinds, "plan-or-direct-execution"
}
