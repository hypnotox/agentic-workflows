package catalog

import (
	"reflect"
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
		{Node{Kind: "skill", Name: "reviewing-plan"}, []Node{{Kind: "agent", Name: "plan-reviewer"}}},
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

// Advisory profile neighbors are deliberately absent from structural closure.
func TestWorkflowProfileNeighborsDoNotCloseRequirements(t *testing.T) {
	got := Closure(Standard, []Node{{Kind: "skill", Name: "brainstorming"}})
	if !reflect.DeepEqual(got, []Node{{Kind: "skill", Name: "brainstorming"}}) {
		t.Errorf("brainstorming closure = %v, want no advisory neighbors", got)
	}
}
