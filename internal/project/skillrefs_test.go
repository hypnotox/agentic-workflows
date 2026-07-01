package project

import (
	"testing"
)

// deadSkillRefs renders the project and runs the dead-skill-reference scan
// over the rendered set (ACTIVE.md/domain docs are irrelevant to these fixtures).
func deadSkillRefs(t *testing.T, configYAML string, files map[string]string) []string {
	t.Helper()
	p, err := Open(scaffoldFiles(t, configYAML, files))
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	var details []string
	for _, d := range p.checkDeadSkillRefs(rendered, RenderedFile{}, nil, p.effSkills) {
		if d.Kind != "dead-skill-reference" {
			t.Fatalf("unexpected drift kind %q", d.Kind)
		}
		details = append(details, d.Detail)
	}
	return details
}

// A managed rendered artifact referencing a known skill outside the effective
// set fails check; enabling the skill clears it.
// invariant: skill-ref-dead-fails
func TestDeadSkillReferenceFlagged(t *testing.T) {
	part := map[string]string{
		"parts/agents-doc/workflow.md": "Use `example-tdd` for test-first work.\n",
	}
	got := deadSkillRefs(t, "prefix: example\nvars: {}\nskills: []\nagents: []\n", part)
	if len(got) != 1 || got[0] != "example-tdd" {
		t.Fatalf("expected one example-tdd finding, got %v", got)
	}
	if got := deadSkillRefs(t, "prefix: example\nvars: {}\nskills: [tdd]\nagents: []\n", part); len(got) != 0 {
		t.Fatalf("expected clean with tdd enabled, got %v", got)
	}
}

// Prefix-adjacent tokens that name no known skill, and references inside
// fenced code blocks, produce no findings.
// invariant: skill-ref-unknown-ignored
func TestSkillRefScannerIgnoresUnknownAndFenced(t *testing.T) {
	got := deadSkillRefs(t, "prefix: example\nvars: {}\nskills: []\nagents: []\n", map[string]string{
		"parts/agents-doc/workflow.md": "This is example-specific prose about example-bootstrap.sh.\n\n```\nexample-tdd\n```\n",
	})
	if len(got) != 0 {
		t.Fatalf("expected no findings, got %v", got)
	}
}

// Whole-token matching: a dead reference to reviewing-plan-resync is flagged
// as the full token, never as a substring hit on reviewing-plan.
func TestSkillRefScannerWholeToken(t *testing.T) {
	got := deadSkillRefs(t,
		"prefix: example\nvars: {}\nskills: []\nagents: []\n",
		map[string]string{
			"parts/agents-doc/workflow.md": "Resync via `example-reviewing-plan-resync`.\n",
		})
	if len(got) != 1 || got[0] != "example-reviewing-plan-resync" {
		t.Fatalf("expected exactly the full-token finding, got %v", got)
	}
}

// The effective set is enabled minus doc-gate-suppressed, with local-declared
// skills always kept.
// invariant: skills-context-effective-set
func TestEffectiveSkillsMembership(t *testing.T) {
	p, err := Open(scaffoldFiles(t,
		"prefix: example\nvars: {}\nskills: [tdd, roadmap-graduation, brainstorming]\nagents: []\n",
		map[string]string{
			"skills/brainstorming.yaml": "local: true\n",
		}))
	if err != nil {
		t.Fatal(err)
	}
	// Make brainstorming BOTH local and doc-gated in the catalog to prove
	// local wins over the gate.
	spec := p.Cat.Skills["brainstorming"]
	spec.RequiresDoc = "roadmap"
	p.Cat.Skills["brainstorming"] = spec
	eff, err := p.effectiveSkills()
	if err != nil {
		t.Fatal(err)
	}
	if !eff["tdd"] {
		t.Error("plain enabled skill missing from effective set")
	}
	if eff["roadmap-graduation"] {
		t.Error("doc-gated skill with disabled doc must be excluded")
	}
	if !eff["brainstorming"] {
		t.Error("local-declared skill must be kept despite the doc gate")
	}
}
