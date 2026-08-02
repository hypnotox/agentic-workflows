package plan

import (
	"strings"
	"testing"
)

func TestProjectionContextUnionAndOutcomeNewline(t *testing.T) {
	p := Plan{
		Format: "plan-v2", Filename: "p.md", Preamble: "pre\n", Goal: "goal\n",
		ArchitectureSummary: "architecture\n", DefinitionOfDone: "done\n",
		Phases: []Phase{{
			Number: 1, Prefix: "phase\n", Close: "close\n", Advances: []string{"advance"},
			Tasks: []Task{{Number: 1, Content: "task\n"}},
		}},
		DoD: []DoDItem{{Slug: "advance", Content: "- advance"}},
	}
	input := ProjectionInput{
		Plan: p, Selector: "1",
		Applying: []ResolvedDecision{{Key: "a", ADRIdentity: "a", Title: "A", Status: "Accepted", Markdown: "A"}},
		Context:  []ResolvedDecision{{Key: "b", ADRIdentity: "b", Title: "B", Status: "Accepted", Markdown: "B"}},
	}
	got, err := RenderProjectionInput(input)
	if err != nil || !strings.Contains(string(got), "Context decisions") || !strings.Contains(string(got), "- advance\n") {
		t.Fatalf("projection = %q, %v", got, err)
	}
}
