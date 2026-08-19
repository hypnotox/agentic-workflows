package evals

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestPlanFlexibilityScenarios(t *testing.T) {
	for _, profile := range []string{"core", "full"} {
		t.Run(profile, func(t *testing.T) {
			root := syncPlanFlexibilityProfile(t, profile)
			for _, target := range []string{"pi", "claude"} {
				t.Run(target, func(t *testing.T) {
					if profile == "core" {
						assertCorePlanGovernanceAbsent(t, root, target)
						return
					}
					for _, name := range []string{"writing-plans", "reviewing-plan", "executing-plans", "subagent-driven-development"} {
						assertPlanFlexibilityScenarios(t, target+" "+name, read(t, planSkillPath(root, target, name)))
					}
					for _, name := range []string{"plan-reviewer", "implementer", "code-reviewer"} {
						assertPlanFlexibilityScenarios(t, target+" "+name, read(t, planAgentPath(root, target, name)))
					}
				})
			}
			if profile == "full" {
				assertPlanFlexibilityScenarios(t, "workflow", read(t, filepath.Join(root, "docs", "workflow.md")))
				assertPlanFlexibilityScenarios(t, "plans README", read(t, filepath.Join(root, "docs", "plans", "README.md")))
				assertPlanFlexibilityScenarios(t, "plan template", read(t, filepath.Join(root, "docs", "plans", "template.md")))
			}
		})
	}
}

func syncPlanFlexibilityProfile(t *testing.T, profile string) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: "+evalPrefix+"\nprofile: "+profile+"\nintegrationBranch: main\n")
	p, err := project.Open(testsupport.Context(t), root)
	if err != nil {
		t.Fatalf("open %s profile: %v", profile, err)
	}
	if _, _, _, err := p.InitializeReport(testsupport.Context(t), project.InitAuthority{InitializedWithVersion: project.Version}); err != nil {
		t.Fatalf("initialize %s profile: %v", profile, err)
	}
	return root
}

func planSkillPath(root, target, name string) string {
	if target == "pi" {
		return filepath.Join(root, ".pi", "skills", evalPrefix+"-"+name, "SKILL.md")
	}
	return filepath.Join(root, ".claude", "skills", evalPrefix+"-"+name, "SKILL.md")
}

func planAgentPath(root, target, name string) string {
	return filepath.Join(root, "."+target, "agents", name+".md")
}

func assertPlanFlexibilityScenarios(t *testing.T, name, body string) {
	t.Helper()
	for _, want := range []string{
		"best known route at authoring time, not a binding implementation choreography",
		"merge, split, reorder, add, remove, or replace recorded route detail",
		"A path omitted from the plan is not alone a reason to stop",
		"Reapproval is required only when the protected contract would change",
		"another phase or reviewer could rely on stale material instructions",
		"Inconsequential and independently local edits require no deviation record",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("%s missing plan-flexibility scenario %q", name, want)
		}
	}
	for _, forbidden := range []string{"do not drift from the plan", "commit boundaries match plan phases", "reports every added path as a reasoned deviation", "ADR-0286"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("%s retains route-binding or repository-specific text %q", name, forbidden)
		}
	}
}

func assertCorePlanGovernanceAbsent(t *testing.T, root, target string) {
	t.Helper()
	for _, name := range []string{"writing-plans", "reviewing-plan", "executing-plans", "subagent-driven-development"} {
		path := planSkillPath(root, target, name)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("core %s unexpectedly renders plan skill %s", target, name)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat core %s plan skill %s: %v", target, name, err)
		}
	}
	for _, name := range []string{"implementer", "code-reviewer"} {
		body := read(t, planAgentPath(root, target, name))
		if strings.Contains(body, "best known route at authoring time") || strings.Contains(body, "plan-flexibility") {
			t.Errorf("core %s %s leaks Full plan governance", target, name)
		}
	}
}
