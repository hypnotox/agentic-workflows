package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

const localSkillYAML = `prefix: example
targets:
  - claude
  - cursor
skills:
  - my-skill
`

func localSkillFiles() map[string]string {
	return map[string]string{
		"skills/my-skill.yaml":             "data:\n  description: Do the thing when X.\n",
		"skills/parts/my-skill/content.md": "Body line for the local skill.\n",
	}
}

func findByTID(files []RenderedFile, tid string) []RenderedFile {
	var out []RenderedFile
	for _, f := range files {
		if f.TemplateID == tid {
			out = append(out, f)
		}
	}
	return out
}

func TestLocalSkillRendersFromBasePerTarget(t *testing.T) {
	root := scaffoldFiles(t, localSkillYAML, localSkillFiles())
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	got := findByTID(files, "skills/_base/SKILL.md.tmpl")
	if len(got) != 2 { // one per enabled target (claude + cursor)
		t.Fatalf("expected 2 base-rendered files, got %d", len(got))
	}
	for _, f := range got {
		for _, want := range []string{"name: example-my-skill", "Do the thing when X.", "Body line for the local skill."} {
			if !strings.Contains(f.Content, want) {
				t.Errorf("missing %q in:\n%s", want, f.Content)
			}
		}
		if strings.Contains(f.Content, "<no value>") {
			t.Errorf("publication-unsafe <no value>:\n%s", f.Content)
		}
	}
}

func TestLocalAgentRendersFromBase(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nagents:\n  - my-agent\n", map[string]string{
		"agents/my-agent.yaml":             "data:\n  description: Reviews the frobnicator.\n",
		"agents/parts/my-agent/content.md": "Agent body here.\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	got := findByTID(files, "agents/_base.md.tmpl")
	if len(got) != 1 { // default target claude only
		t.Fatalf("expected 1 base agent file, got %d", len(got))
	}
	for _, want := range []string{"name: my-agent", "Reviews the frobnicator.", "Agent body here."} {
		if !strings.Contains(got[0].Content, want) {
			t.Errorf("missing %q in:\n%s", want, got[0].Content)
		}
	}
}

func TestLocalSynthesisDoesNotMutateStandard(t *testing.T) {
	before := len(catalog.Standard.Skills)
	root := scaffoldFiles(t, localSkillYAML, localSkillFiles())
	if _, err := Open(root); err != nil {
		t.Fatal(err)
	}
	if got := len(catalog.Standard.Skills); got != before {
		t.Errorf("catalog.Standard.Skills mutated: before %d, after %d", before, got)
	}
	if _, ok := catalog.Standard.Skills["my-skill"]; ok {
		t.Error("local entry leaked into catalog.Standard")
	}
}

func TestLocalUndeclaredNameFailsOpen(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills:\n  - ghost\n", nil)
	_, err := Open(root)
	if err == nil || !strings.Contains(err.Error(), "is not in the catalog") {
		t.Fatalf("expected not-in-catalog error, got %v", err)
	}
}

func TestLocalReservedNameFailsOpen(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills:\n  - _x\n", map[string]string{
		"skills/_x.yaml": "data:\n  description: nope\n",
	})
	_, err := Open(root)
	if err == nil || !strings.Contains(err.Error(), "kebab-case") {
		t.Fatalf("expected invalid-name error, got %v", err)
	}
}

// TestLocalAgentReservedNameFailsOpen exercises the agents-side synthesis
// error-return in effectiveCatalog (skills stay clean, agents fail).
func TestLocalAgentReservedNameFailsOpen(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nagents:\n  - _a\n", map[string]string{
		"agents/_a.yaml": "data:\n  description: x\n",
	})
	_, err := Open(root)
	if err == nil || !strings.Contains(err.Error(), "kebab-case") {
		t.Fatalf("expected invalid-name error from agents synthesis, got %v", err)
	}
}

func TestLocalMalformedSidecarFailsOpen(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills:\n  - my-skill\n", map[string]string{
		"skills/my-skill.yaml": "data: [unterminated\n",
	})
	_, err := Open(root)
	if err == nil || !strings.Contains(err.Error(), "sidecar") {
		t.Fatalf("expected sidecar parse error, got %v", err)
	}
}

func TestLocalNameShadowingStandardStaysStandard(t *testing.T) {
	root := scaffoldFiles(t, sampleYAML, map[string]string{
		"skills/parts/tdd/content.md": "should be ignored\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if p.Cat.Skills["tdd"].Base {
		t.Error("Standard skill tdd was shadowed by a local synthesis")
	}
	if secs := p.Cat.Skills["tdd"].Sections; len(secs) == 1 && secs[0] == "content" {
		t.Errorf("tdd sections overwritten to [content]: %v", secs)
	}
}

func TestLocalHandAuthoredSkipped(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills:\n  - hand\n", map[string]string{
		"skills/hand.yaml": "local: true\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.Cat.Skills["hand"]; ok {
		t.Error("a local:true name must not be synthesized into the catalog")
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	if got := findByTID(files, "skills/_base/SKILL.md.tmpl"); len(got) != 0 {
		t.Errorf("a local:true skill must not be base-rendered, got %d files", len(got))
	}
}
