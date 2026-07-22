package project

import (
	"maps"
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

func planNodes(plan []PlanOp) map[catalog.Node]bool {
	got := make(map[catalog.Node]bool)
	for _, op := range plan {
		got[op.Node] = true
	}
	return got
}

func closureNodes(nodes []catalog.Node) map[catalog.Node]bool {
	got := make(map[catalog.Node]bool, len(nodes))
	for _, node := range nodes {
		got[node] = true
	}
	return got
}

// openChainProject opens a project with the full derived chain closure and
// its three agents enabled (the drift-test fixture builder).
func openChainProject(t *testing.T) *Project {
	t.Helper()
	p, err := Open(scaffold(t, chainClosureConfig("awf")))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Cascade members are seed-dependent (ADR-0081 Decision 5). Pin names, not
// corpus sizes, so an unrelated closure addition cannot silently shift scope.
// invariant: tooling/init-and-enablement:remove-refuses-dependents
func TestResolveDisableCascadeSizes(t *testing.T) {
	p := openChainProject(t)
	cases := []struct {
		seed string
		want []string
	}{
		{"brainstorming", []string{"skill brainstorming"}},
		{"reviewing-plan", []string{"agent plan-reviewer", "skill brainstorming", "skill proposing-adr", "skill reviewing-adr", "skill reviewing-plan", "skill reviewing-plan-resync", "skill writing-plans"}},
		{"executing-plans", []string{"agent plan-reviewer", "skill brainstorming", "skill executing-plans", "skill proposing-adr", "skill reviewing-adr", "skill reviewing-impl", "skill reviewing-plan", "skill reviewing-plan-resync", "skill subagent-driven-development", "skill writing-plans"}},
		{"retrospective", []string{"agent plan-reviewer", "skill brainstorming", "skill executing-plans", "skill proposing-adr", "skill retrospective", "skill reviewing-adr", "skill reviewing-impl", "skill reviewing-plan", "skill reviewing-plan-resync", "skill subagent-driven-development", "skill writing-plans"}},
	}
	for _, tc := range cases {
		plan := p.ResolveDisable("skill", tc.seed)
		got := make([]string, 0)
		for _, op := range plan {
			got = append(got, op.Node.Kind+" "+op.Node.Name)
		}
		slices.Sort(got)
		if !slices.Equal(got, tc.want) {
			t.Errorf("ResolveDisable(%q) = %v, want %v", tc.seed, got, tc.want)
		}
	}
}

// The add plan on an empty config is the seed's full forward closure.
// invariant: tooling/init-and-enablement:add-applies-closure-plan
func TestResolveEnableClosurePlan(t *testing.T) {
	p, err := Open(scaffold(t, "prefix: example\nskills: []\nagents: []\n"))
	if err != nil {
		t.Fatal(err)
	}
	plan := p.ResolveEnable("skill", "brainstorming")
	closure := catalog.Closure(catalog.Standard, []catalog.Node{{Kind: "skill", Name: "brainstorming"}})
	if got, want := planNodes(plan), closureNodes(closure); !maps.Equal(got, want) {
		t.Fatalf("ResolveEnable(brainstorming) nodes = %v, want %v", got, want)
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
	if leafPlan := p2.ResolveEnable("skill", "tdd"); len(leafPlan) != 1 {
		t.Errorf("adding a leaf to a closed config plans %d ops, want 1: %v", len(leafPlan), leafPlan)
	}
	// Enabled-subtree skip mid-walk never re-adds an enabled member.
	p3, err := Open(scaffold(t, "prefix: example\nskills: [executing-plans, retrospective, reviewing-impl, subagent-driven-development]\nagents: [code-reviewer]\n"))
	if err != nil {
		t.Fatal(err)
	}
	plan3 := p3.ResolveEnable("skill", "brainstorming")
	enabled := map[catalog.Node]bool{
		{Kind: "skill", Name: "executing-plans"}:             true,
		{Kind: "skill", Name: "retrospective"}:               true,
		{Kind: "skill", Name: "reviewing-impl"}:              true,
		{Kind: "skill", Name: "subagent-driven-development"}: true,
		{Kind: "agent", Name: "code-reviewer"}:               true,
	}
	want3 := map[catalog.Node]bool{}
	for _, node := range closure {
		if !enabled[node] {
			want3[node] = true
		}
	}
	if got := planNodes(plan3); !maps.Equal(got, want3) {
		t.Fatalf("partial-closure add nodes = %v, want %v", got, want3)
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
	disablePlan := p.ResolveDisable("skill", "retrospective")
	if len(disablePlan) != 1 {
		t.Errorf("local reviewing-impl must not join the plan, got %d ops: %v", len(disablePlan), disablePlan)
	}
}
