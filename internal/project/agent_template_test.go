package project

import (
	"strings"
	"testing"
)

func TestStandardAgentContracts(t *testing.T) {
	contracts := map[string][]string{
		"explorer":        []string{"read-only exploration child", "Do not edit, stage, commit", "searched boundary", "uncertainty"},
		"premise-checker": []string{"read-only adversarial premise checker", "Try to falsify", "counterexamples", "supported**, **revise**, or **unresolved"},
		"implementer":     []string{"same-worktree implementation child", "Do not stage, commit", "The parent owns integration", "closed `completed` or `stopped` receipt"},
		"reviewer":        []string{"fresh report-only reviewer", "Do not edit, stage, commit", "consequence order", "residual uncertainty"},
	}
	for name, wants := range contracts {
		t.Run(name, func(t *testing.T) {
			out := renderAgentGolden(t, name, map[string]any{"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "layout": testLayout()})
			if !strings.Contains(out, "name: "+name) {
				t.Fatalf("rendered %s agent has no matching frontmatter:\n%s", name, out)
			}
			for _, want := range wants {
				if !strings.Contains(out, want) {
					t.Errorf("%s contract missing %q:\n%s", name, want, out)
				}
			}
		})
	}
}
