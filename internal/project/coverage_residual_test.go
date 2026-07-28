package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
)

func TestAdvisoryRenderingAndResolverFallbacks(t *testing.T) {
	p := &Project{Cfg: &config.Config{Skills: []string{"local", "dep"}}, Cat: &catalog.Catalog{Skills: map[string]catalog.SkillSpec{
		"local": {Profile: catalog.WorkflowProfile{}, RequiresSkills: []string{"dep"}},
		"dep":   {Profile: catalog.WorkflowProfile{}},
	}}}
	row := p.taskSkillRows()
	if !strings.Contains(row, "project-local skill") || !strings.Contains(row, "WorkflowTask") && !strings.Contains(row, "task") {
		t.Fatalf("fallback row = %q", row)
	}
	old := catalog.Standard.Skills["brainstorming"]
	patched := old
	patched.RequiresSkills = []string{"exploring"}
	catalog.Standard.Skills["brainstorming"] = patched
	defer func() { catalog.Standard.Skills["brainstorming"] = old }()
	pick := []string{"brainstorming"}
	if _, _, err := ScaffoldConfig("example", nil, &config.CatalogTrim{Skills: &pick}, nil); err != nil {
		t.Fatal(err)
	}
	if artifactSourceLabel(ArtifactProtocolDescriptor) != "protocol descriptor" || artifactSourceLabel(ArtifactRole("unknown")) != "unknown" {
		t.Fatal("unknown artifact role not preserved")
	}
	if got := p.ResolveEnable("skill", "local"); len(got) != 1 {
		t.Fatalf("resolver plan = %#v", got)
	}
	localRoot := scaffoldFiles(t, "prefix: example\nskills: [local]\nagents: []\n", map[string]string{"skills/local.yaml": "data:\n  description: local\n"})
	localProject, err := Open(localRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := localProject.effectiveCatalog(); err != nil {
		t.Fatal(err)
	}
	pool := map[string]catalog.SkillSpec{}
	if err := synthesizeLocals(localProject, pool, []string{"local"}, "skills", func(string) catalog.SkillSpec { return catalog.SkillSpec{} }); err != nil {
		t.Fatal(err)
	}
}
