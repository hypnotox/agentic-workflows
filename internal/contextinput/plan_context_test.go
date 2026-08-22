package contextinput

import (
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/plan"
)

// invariant: tooling/context-and-topic:adr-linked-plan-references (TestPlanContextResolvesAliasesAndOrdersLinks)
func TestPlanContextResolvesAliasesAndOrdersLinks(t *testing.T) {
	corpus, err := adr.NewCorpus([]adr.ADR{
		{Number: "0007", Slug: "retained-pending-slug", Filename: "0007-numbered.md", Status: "Implemented"},
		{Slug: "still-pending", Filename: "still-pending.md", Status: "Proposed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	plans := []plan.Plan{
		{Format: "plan-v2", Status: "Proposed", Filename: "2026-08-02-z.md", Path: "docs/plans/2026-08-02-z.md", ADRs: []plan.ADRLink{{Slug: "retained-pending-slug"}, {Number: 7}, {Slug: "still-pending"}}},
		{Format: "plan-v2", Status: "Implemented", Filename: "2026-08-01-a.md", Path: "docs/plans/2026-08-01-a.md", ADRs: []plan.ADRLink{{Number: 7}}},
		{Format: "plan-v2", Status: "Proposed", Filename: "2026-08-03-unrelated.md", Path: "docs/plans/2026-08-03-unrelated.md", ADRs: []plan.ADRLink{{Slug: "missing"}}},
		{Format: "plan-v1", Status: "Proposed", Filename: "2026-08-04-v1.md", Path: "docs/plans/2026-08-04-v1.md", ADRs: []plan.ADRLink{{Number: 7}}},
		{Filename: "2026-08-05-legacy.md", Path: "docs/plans/2026-08-05-legacy.md", ADRs: []plan.ADRLink{{Number: 7}}},
	}
	context := NewPlanContext(plans, corpus)
	if len(context.Plans) != len(plans) {
		t.Fatalf("parsed plans = %d, want %d", len(context.Plans), len(plans))
	}
	if got, want := context.LinkedPlans("0007"), []string{"docs/plans/2026-08-01-a.md", "docs/plans/2026-08-02-z.md"}; !slices.Equal(got, want) {
		t.Fatalf("numbered links = %#v, want %#v", got, want)
	}
	if got, want := context.LinkedPlans("still-pending"), []string{"docs/plans/2026-08-02-z.md"}; !slices.Equal(got, want) {
		t.Fatalf("pending links = %#v, want %#v", got, want)
	}
	if got := context.LinkedPlans("missing"); len(got) != 0 {
		t.Fatalf("unresolved link = %#v", got)
	}
}
