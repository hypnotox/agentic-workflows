package migrate

import (
	"reflect"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

func TestHistoricalRequiresOfPreservesFrozenClosure(t *testing.T) {
	cat := &catalog.Catalog{
		Skills: map[string]catalog.SkillSpec{
			"root": {
				RequiresSkills: []string{"dependency"},
				RequiresAgent:  "reviewer",
				RequiresDoc:    "guide",
			},
		},
		Agents: map[string]catalog.AgentSpec{
			"reviewer": {RequiresSkills: []string{"agent-dependency"}},
		},
	}
	if got, want := historicalRequiresOf(cat, historicalNode{Kind: "skill", Name: "root"}), []historicalNode{
		{Kind: "skill", Name: "dependency"},
		{Kind: "agent", Name: "reviewer"},
		{Kind: "doc", Name: "guide"},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("skill requirements = %#v, want %#v", got, want)
	}
	if got, want := historicalRequiresOf(cat, historicalNode{Kind: "agent", Name: "reviewer"}), []historicalNode{
		{Kind: "skill", Name: "agent-dependency"},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("agent requirements = %#v, want %#v", got, want)
	}
	if got := historicalRequiresOf(cat, historicalNode{Kind: "doc", Name: "guide"}); len(got) != 0 {
		t.Fatalf("doc requirements = %#v, want none", got)
	}
}
