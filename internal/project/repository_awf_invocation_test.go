package project

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

var bareRepositoryAwfCommand = regexp.MustCompile("`awf (?:context|read|check|render|audit|new|adr|effort|topic|upgrade|init|list|uninstall)")

// Every rendered agent-facing executable command invokes the unconditional
// repository wrapper, while CLI grammar and wrapper resolution remain free to
// use the product name.
// invariant: rendering/workflow-skill-templates:repository-awf-invocation (TestRepositoryAwfInvocation)
func TestRepositoryAwfInvocation(t *testing.T) {
	data := map[string]any{
		"prefix": "example", "vars": map[string]any{}, "layout": testLayout(),
		"data": map[string]any{}, "skills": map[string]bool{},
	}
	for name, skill := range catalog.Standard.Skills {
		// roadmap-graduation needs repository roadmap data unrelated to CLI guidance.
		if name == "roadmap-graduation" {
			continue
		}
		skillData := make(map[string]any, len(data)+1)
		for key, value := range data {
			skillData[key] = value
		}
		skillData["data"] = skill.Data
		body := renderSkillGolden(t, name, skillData)
		if bare := bareRepositoryAwfCommand.FindString(body); bare != "" {
			t.Errorf("%s renders bare repository command %q", name, bare)
		}
	}
	for _, name := range []string{"implementer", "adr-reviewer", "plan-reviewer", "code-reviewer", "explorer", "grounding-checker"} {
		body := renderAgentGolden(t, name, data)
		if bare := bareRepositoryAwfCommand.FindString(body); bare != "" {
			t.Errorf("%s agent renders bare repository command %q", name, bare)
		}
	}
}

// invariant: rendering/guide-and-doc-templates:guide-awf-invocation (TestGuideAwfInvocation)
func TestGuideAwfInvocation(t *testing.T) {
	data := map[string]any{
		"prefix": "example", "vars": map[string]any{}, "layout": testLayout(),
		"data": map[string]any{}, "skills": map[string]bool{},
	}
	guide := renderGolden(t, "agents-doc/AGENTS.md.tmpl", data)
	for _, want := range []string{"run `./awf render` and `./awf check`", "run `./awf check staged`"} {
		if !strings.Contains(guide, want) {
			t.Errorf("guide lacks repository wrapper command %q:\n%s", want, guide)
		}
	}
	if bare := bareRepositoryAwfCommand.FindString(guide); bare != "" {
		t.Errorf("guide renders bare repository command %q", bare)
	}
}
