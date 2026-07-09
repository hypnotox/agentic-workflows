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
