package project

import (
	"testing"
)

// deadSkillRefs renders the project and runs the dead-skill-reference scan
// over the rendered set (INDEX.md/domain docs are irrelevant to these fixtures).
func deadSkillRefs(t *testing.T, files map[string]string) []string {
	t.Helper()
	p, err := Open(testContext(t), scaffoldFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\n", files))
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := renderAll(p)
	if err != nil {
		t.Fatal(err)
	}
	var details []string
	for _, d := range checkDeadSkillRefs(renderInputsForTest(p), rendered, mustDeriveSkills(t, p)) {
		if d.Kind != "dead-skill-reference" {
			t.Fatalf("unexpected drift kind %q", d.Kind)
		}
		details = append(details, d.Detail)
	}
	return details
}

// A managed rendered artifact referencing a known skill outside the effective
// set fails check; enabling the skill clears it.
func TestCatalogSkillReferenceIsNeverDead(t *testing.T) {
	part := map[string]string{"parts/agents-doc/workflow.md": "Use `example-tdd` for test-first work.\n"}
	if got := deadSkillRefs(t, part); len(got) != 0 {
		t.Fatalf("full catalog reference was flagged dead: %v", got)
	}
}

// Prefix-adjacent tokens that name no known skill, and references inside
// fenced code blocks, produce no findings.
// invariant: rendering/doc-outputs:skill-ref-unknown-ignored (TestSkillRefScannerIgnoresUnknownAndFenced)
func TestSkillRefScannerIgnoresUnknownAndFenced(t *testing.T) {
	got := deadSkillRefs(t, map[string]string{
		"parts/agents-doc/workflow.md": "This is example-specific prose about example-bootstrap.sh.\n\n```\nexample-tdd\n```\n",
	})
	if len(got) != 0 {
		t.Fatalf("expected no findings, got %v", got)
	}
}

// Whole-token matching: the retired reviewing-plan-resync token is unknown
// and never degrades into a false dead hit on the surviving reviewing-plan.
func TestSkillRefScannerIgnoresRetiredUnknownToken(t *testing.T) {
	got := deadSkillRefs(t, map[string]string{"parts/agents-doc/workflow.md": "Resync via `example-reviewing-plan-resync`.\n"})
	if len(got) != 0 {
		t.Fatalf("retired unknown token findings = %v", got)
	}
}

// Whole-token matching is boundary-anchored on the left too: a prefix embedded
// in a larger word (nonexample-tdd) is not a reference.
func TestSkillRefScannerRequiresLeftBoundary(t *testing.T) {
	got := deadSkillRefs(t,
		map[string]string{
			"parts/agents-doc/workflow.md": "Prose about nonexample-tdd tooling.\n",
		})
	if len(got) != 0 {
		t.Fatalf("expected no findings for an embedded prefix, got %v", got)
	}
}

// A chain-less config that enables only task skills renders with zero dead
// skill references: every chain-skill mention in a task skill is guarded with
// generic fallback prose (ADR-0045, ADR-0046).
func TestTaskSkillsOnlyConfigHasNoDeadRefs(t *testing.T) {
	got := deadSkillRefs(t,
		nil)
	if len(got) != 0 {
		t.Fatalf("expected no dead skill references, got %v", got)
	}
}

// invariant: rendering/project-output-plan:profile-projected-render (TestEffectiveSkillsMembership)
func TestEffectiveSkillsMembership(t *testing.T) {
	p, err := Open(testContext(t), scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	eff, err := effectiveSkills(renderInputsForTest(p))
	if err != nil {
		t.Fatal(err)
	}
	for name := range projectCatalog(renderInputsForTest(p)).Skills {
		if !eff[name] {
			t.Errorf("catalog skill %q missing from effective set", name)
		}
	}
}
