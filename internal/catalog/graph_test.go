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
		{Node{Kind: "agent", Name: "plan-reviewer"}, nil},
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
	custom := &Catalog{Agents: map[string]AgentSpec{"reviewer": {RequiresSkills: []string{"reviewing-plan"}}}}
	if got := RequiresOf(custom, Node{Kind: "agent", Name: "reviewer"}); !reflect.DeepEqual(got, []Node{{Kind: "skill", Name: "reviewing-plan"}}) {
		t.Errorf("custom agent requirements = %v", got)
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
// Grounding owns the checker edge; brainstorming's optional prose reference is
// not structural.
func TestWorkflowProfileNeighborsDoNotCloseRequirements(t *testing.T) {
	got := Closure(Standard, []Node{{Kind: "skill", Name: "brainstorming"}})
	want := []Node{{Kind: "skill", Name: "brainstorming"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("brainstorming closure = %v, want %v", got, want)
	}
	got = Closure(Standard, []Node{{Kind: "skill", Name: "grounding"}})
	want = []Node{{Kind: "skill", Name: "grounding"}, {Kind: "agent", Name: "grounding-checker"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("grounding closure = %v, want %v", got, want)
	}
}
