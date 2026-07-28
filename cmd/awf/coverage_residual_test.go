package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

func TestAdvisoryAgentNoteCoverage(t *testing.T) {
	root := scaffoldProject(t)
	p, err := project.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	p.Cfg.Agents = []string{"code-reviewer"}
	p.Cat.Skills["removed"] = catalog.SkillSpec{RequiresAgent: "code-reviewer"}
	p.Cfg.Skills = []string{"removed", "other"}
	var out bytes.Buffer
	noteUnrequiredAgents(p, []project.PlanOp{{Node: catalog.Node{Kind: "skill", Name: "removed"}}, {Node: catalog.Node{Kind: "skill", Name: "other"}}}, &out)
	if !strings.Contains(out.String(), "code-reviewer") {
		t.Fatalf("note=%q", out.String())
	}
}
