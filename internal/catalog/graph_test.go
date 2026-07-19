package catalog

import (
	"reflect"
	"slices"
	"sort"
	"testing"
)

// RequiresOf enumerates edges in declaration order: RequiresSkills first, then
// RequiresAgent, then RequiresDoc; docs and unknown (project-local) names are
// leaves (ADR-0081 Decision 1).
func TestRequiresOfEdgeEnumeration(t *testing.T) {
	cases := []struct {
		node Node
		want []Node
	}{
		{Node{Kind: "skill", Name: "reviewing-plan"}, []Node{
			{Kind: "skill", Name: "reviewing-plan-resync"},
			{Kind: "skill", Name: "writing-plans"},
			{Kind: "agent", Name: "plan-reviewer"},
		}},
		{Node{Kind: "skill", Name: "roadmap-graduation"}, []Node{
			{Kind: "doc", Name: "roadmap"},
		}},
		{Node{Kind: "agent", Name: "plan-reviewer"}, []Node{
			{Kind: "skill", Name: "reviewing-plan-resync"},
		}},
		{Node{Kind: "skill", Name: "adr-lifecycle"}, nil},
		{Node{Kind: "skill", Name: "my-local-skill"}, nil},
		{Node{Kind: "agent", Name: "my-local-agent"}, nil},
		{Node{Kind: "doc", Name: "roadmap"}, nil},
	}
	for _, tc := range cases {
		if got := RequiresOf(Standard, tc.node); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("RequiresOf(%v) = %v, want %v", tc.node, got, tc.want)
		}
	}
}

// Closure terminates on a mutually-requiring cycle and returns seeds first.
func TestClosureIsCycleSafe(t *testing.T) {
	cyclic := &Catalog{Skills: map[string]SkillSpec{
		"a": {RequiresSkills: []string{"b"}},
		"b": {RequiresSkills: []string{"a"}},
	}}
	got := Closure(cyclic, []Node{{Kind: "skill", Name: "a"}})
	want := []Node{{Kind: "skill", Name: "a"}, {Kind: "skill", Name: "b"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Closure over a 2-cycle = %v, want %v", got, want)
	}
}

func TestExploringRequirementsAreOneWay(t *testing.T) {
	for _, consumer := range []string{"brainstorming", "debugging", "refactor-coupling-audit"} {
		if !slices.Contains(Standard.Skills[consumer].RequiresSkills, "exploring") {
			t.Errorf("%s does not require exploring", consumer)
		}
	}
	for _, forbidden := range []string{"brainstorming", "debugging", "refactor-coupling-audit"} {
		if slices.Contains(Standard.Skills["exploring"].RequiresSkills, forbidden) {
			t.Errorf("exploring has reciprocal requirement on %s", forbidden)
		}
	}
}

// The Chain seeds' closure is exactly the 11-skill chain unit plus its three
// agents (ADR-0081; counts verified against the catalog on 2026-07-09).
func TestClosureChainUnit(t *testing.T) {
	var seeds []Node
	for name, spec := range Standard.Skills {
		if spec.Chain {
			seeds = append(seeds, Node{Kind: "skill", Name: name})
		}
	}
	var skills, agents []string
	for _, n := range Closure(Standard, seeds) {
		switch n.Kind {
		case "skill":
			skills = append(skills, n.Name)
		case "agent":
			agents = append(agents, n.Name)
		}
	}
	sort.Strings(skills)
	sort.Strings(agents)
	wantSkills := []string{"adr-lifecycle", "brainstorming", "executing-plans", "exploring", "proposing-adr",
		"retrospective", "reviewing-adr", "reviewing-impl", "reviewing-plan",
		"reviewing-plan-resync", "subagent-driven-development", "writing-plans"}
	wantAgents := []string{"adr-reviewer", "code-reviewer", "plan-reviewer"}
	if !reflect.DeepEqual(skills, wantSkills) {
		t.Errorf("chain closure skills = %v, want %v", skills, wantSkills)
	}
	if !reflect.DeepEqual(agents, wantAgents) {
		t.Errorf("chain closure agents = %v, want %v", agents, wantAgents)
	}
}
