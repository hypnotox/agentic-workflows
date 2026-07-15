package project

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

// openChainProject opens a project with the full 11-skill chain closure and
// its three agents enabled (the drift-test fixture builder).
func openChainProject(t *testing.T) *Project {
	t.Helper()
	p, err := Open(scaffold(t, chainClosureConfig("awf")))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Cascade sizes are seed-dependent (ADR-0081 Decision 5): the closure has two
// mutually-requiring cores (planning 5, execution 3) with edges only
// planning→execution, brainstorming a pure source, and retrospective/
// adr-lifecycle sinks. Counts verified against the catalog on 2026-07-09.
// invariant: remove-refuses-dependents
func TestResolveDisableCascadeSizes(t *testing.T) {
	p := openChainProject(t)
	cases := []struct {
		seed string
		ops  int
	}{
		{"brainstorming", 1},    // pure source: nothing requires it
		{"reviewing-plan", 7},   // planning core + brainstorming + plan-reviewer
		{"executing-plans", 10}, // both cores + brainstorming + plan-reviewer
		{"retrospective", 11},   // worst case: 10 skills + plan-reviewer
	}
	for _, tc := range cases {
		if plan := p.ResolveDisable("skill", tc.seed); len(plan) != tc.ops {
			t.Errorf("ResolveDisable(%q) = %d ops, want %d: %v", tc.seed, len(plan), tc.ops, plan)
		}
	}
}

// The add plan on an empty config is the seed's full forward closure - the
// 11-skill chain unit plus its three agents from the brainstorming seed.
// invariant: add-applies-closure-plan
func TestResolveEnableClosurePlan(t *testing.T) {
	p, err := Open(scaffold(t, "prefix: example\nskills: []\nagents: []\n"))
	if err != nil {
		t.Fatal(err)
	}
	plan := p.ResolveEnable("skill", "brainstorming")
	if len(plan) != 14 {
		t.Fatalf("ResolveEnable(brainstorming) = %d ops, want 14 (11 skills + 3 agents): %v", len(plan), plan)
	}
	if plan[0].Node != (catalog.Node{Kind: "skill", Name: "brainstorming"}) || plan[0].RequiredBy != "" {
		t.Errorf("plan must lead with the requested node, got %+v", plan[0])
	}
	for _, op := range plan[1:] {
		if op.RequiredBy == "" {
			t.Errorf("closure op %v lacks required-by provenance", op.Node)
		}
	}
	// An enabled dependency is skipped with its whole subtree.
	p2 := openChainProject(t)
	if plan := p2.ResolveEnable("skill", "tdd"); len(plan) != 1 {
		t.Errorf("adding a leaf to a closed config plans %d ops, want 1: %v", len(plan), plan)
	}
	// Enabled-subtree skip mid-walk: with the execution core already enabled,
	// brainstorming's closure plans only the planning side (7 skills incl. the
	// seed + adr-reviewer + plan-reviewer = 9 ops), never re-adding members.
	p3, err := Open(scaffold(t, "prefix: example\nskills: [executing-plans, retrospective, reviewing-impl, subagent-driven-development]\nagents: [code-reviewer]\n"))
	if err != nil {
		t.Fatal(err)
	}
	plan3 := p3.ResolveEnable("skill", "brainstorming")
	if len(plan3) != 9 {
		t.Fatalf("partial-closure add = %d ops, want 9: %v", len(plan3), plan3)
	}
	for _, op := range plan3 {
		if op.Node.Name == "reviewing-impl" || op.Node.Name == "executing-plans" {
			t.Errorf("enabled dependency %v must be skipped", op.Node)
		}
	}
}

// A local-sidecar artifact carries no catalog edges and is never pulled into
// a remove plan, mirroring the validator's skip.
func TestResolveDisableSkipsLocalDependents(t *testing.T) {
	p, err := Open(scaffoldFiles(t,
		"prefix: example\nskills: [reviewing-impl, executing-plans, retrospective, subagent-driven-development]\nagents: [code-reviewer]\n",
		map[string]string{"skills/reviewing-impl.yaml": "local: true\n"}))
	if err != nil {
		t.Fatal(err)
	}
	plan := p.ResolveDisable("skill", "retrospective")
	if len(plan) != 1 {
		t.Errorf("local reviewing-impl must not join the plan, got %d ops: %v", len(plan), plan)
	}
}
