package contextinput

import (
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/plan"
)

func TestNewDefensivelyCopiesMutableSemanticProjections(t *testing.T) {
	layout := Layout{Docs: map[string]string{"guide": "docs/guide.md"}, Singletons: map[string]string{"guide": "docs/guide.md"}}
	plans := []plan.Plan{{Phases: []plan.Phase{{Tasks: []plan.Task{{Fields: plan.TaskFields{Paths: []plan.PathEntry{{Value: "a"}}, Applying: []plan.DecisionRef{{ADR: "0001"}}, Context: []plan.DecisionRef{{ADR: "0002"}}}}}, Advances: []string{"advance"}, Completes: []string{"complete"}}}}}
	lock := &manifest.Lock{Files: map[string]manifest.Entry{"generated.md": {}}, BridgeAttestation: &manifest.BridgeAttestation{LegacyADRGaps: []int{1}}}
	declarations := []outputplan.Declaration{outputplan.NewDeclaration("docs/guide.md", "guide", nil, nil, nil)}
	input := New(layout, currentstate.Loaded{}, NewPlanContext(plans, adr.Corpus{}), nil, lock, declarations, []string{"eligible"}, []string{"ignore"})

	layout.Docs["guide"] = "mutated"
	layout.Singletons["guide"] = "mutated"
	plans[0].Phases[0].Tasks[0].Fields.Paths[0].Value = "mutated"
	plans[0].Phases[0].Tasks[0].Fields.Applying[0].ADR = "mutated"
	plans[0].Phases[0].Tasks[0].Fields.Context[0].ADR = "mutated"
	plans[0].Phases[0].Advances[0], plans[0].Phases[0].Completes[0] = "mutated", "mutated"
	lock.Files["mutated"] = manifest.Entry{}
	lock.BridgeAttestation.LegacyADRGaps[0] = 9
	declarations[0] = outputplan.NewDeclaration("mutated", "mutated", nil, nil, nil)

	if input.Layout.Docs["guide"] != "docs/guide.md" || input.Layout.Singletons["guide"] != "docs/guide.md" || input.PlanState.Plans[0].Phases[0].Tasks[0].Fields.Paths[0].Value != "a" || input.PlanState.Plans[0].Phases[0].Tasks[0].Fields.Applying[0].ADR != "0001" || input.PlanState.Plans[0].Phases[0].Tasks[0].Fields.Context[0].ADR != "0002" || input.PlanState.Plans[0].Phases[0].Advances[0] != "advance" || input.PlanState.Plans[0].Phases[0].Completes[0] != "complete" || input.Lock.Files["mutated"] != (manifest.Entry{}) || input.Lock.BridgeAttestation.LegacyADRGaps[0] != 1 || input.Declarations[0].Path() != "docs/guide.md" {
		t.Fatalf("input retained mutable caller state: %#v", input)
	}
	eligible, ignores := []string{"eligible"}, []string{"ignore"}
	input = New(Layout{}, currentstate.Loaded{}, PlanContext{}, nil, nil, nil, eligible, ignores)
	eligible[0], ignores[0] = "mutated", "mutated"
	if input.Eligible[0] != "eligible" || input.ContextIgnore[0] != "ignore" {
		t.Fatalf("input aliases eligibility projections: %#v", input)
	}
}

func TestNewPlanContextDefensivelyProjectsAndOrdersLinks(t *testing.T) {
	corpus, err := adr.NewCorpus([]adr.ADR{{Number: "0007", Slug: "example", Filename: "0007-example.md", Status: "Implemented"}})
	if err != nil {
		t.Fatal(err)
	}
	plans := []plan.Plan{
		{Format: "plan-v2", Path: "docs/plans/z.md", ADRs: []plan.ADRLink{{Number: 7}}},
		{Format: "plan-v2", Path: "docs/plans/a.md", ADRs: []plan.ADRLink{{Slug: "example"}}},
	}
	input := NewPlanContext(plans, corpus)
	plans[0].Path = "mutated"
	if got, want := input.LinkedPlans("0007"), []string{"docs/plans/a.md", "docs/plans/z.md"}; !slices.Equal(got, want) {
		t.Fatalf("links = %#v, want %#v", got, want)
	}
	got := input.LinkedPlans("0007")
	got[0] = "mutated"
	if again := input.LinkedPlans("0007"); again[0] != "docs/plans/a.md" {
		t.Fatalf("returned projection aliased input: %#v", again)
	}
}

func TestNewCopiesLoadedSources(t *testing.T) {
	loaded := currentstate.Loaded{Sources: map[string][]byte{"source": []byte("original")}}
	input := New(Layout{}, loaded, PlanContext{}, nil, nil, nil, nil, nil)
	loaded.Sources["source"][0] = 'm'
	if got := string(input.Loaded.Sources["source"]); got != "original" {
		t.Fatalf("loaded source = %q, want defensive copy", got)
	}
}
