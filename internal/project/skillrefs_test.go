package project

import (
	"testing"
)

// deadSkillRefs renders the project and runs the dead-skill-reference scan
// over the rendered set (INDEX.md/domain docs are irrelevant to these fixtures).
func deadSkillRefs(t *testing.T, files map[string]string) []string {
	t.Helper()
	p, err := loadTestSession(testContext(t), scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nvars: {}\n", files))
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
	part := map[string]string{"parts/agents-doc/workflow.md": "Use `awf-maintenance` for test-first work.\n"}
	if got := deadSkillRefs(t, part); len(got) != 0 {
		t.Fatalf("full catalog reference was flagged dead: %v", got)
	}
}

// Prefix-adjacent tokens that name no known skill, and references inside
// fenced code blocks, produce no findings.
// invariant: rendering/doc-outputs:skill-ref-unknown-ignored (TestSkillRefScannerIgnoresUnknownAndFenced)
func TestSkillRefScannerIgnoresUnknownAndFenced(t *testing.T) {
	got := deadSkillRefs(t, map[string]string{
		"parts/agents-doc/workflow.md": "This is example-specific prose about example-bootstrap.sh.\n\n```\nawf-maintenance\n```\n",
	})
	if len(got) != 0 {
		t.Fatalf("expected no findings, got %v", got)
	}
}

// Whole-token matching keeps an unknown retired skill token from degrading
// into a false dead reference to a similarly named live skill.
func TestSkillRefScannerIgnoresRetiredUnknownToken(t *testing.T) {
	got := deadSkillRefs(t, map[string]string{"parts/agents-doc/workflow.md": "Resync via `example-reviewing-impl-resync`.\n"})
	if len(got) != 0 {
		t.Fatalf("retired unknown token findings = %v", got)
	}
}

// Whole-token matching is boundary-anchored on the left too: a prefix embedded
// in a larger word (nonawf-maintenance) is not a reference.
func TestSkillRefScannerRequiresLeftBoundary(t *testing.T) {
	got := deadSkillRefs(t,
		map[string]string{
			"parts/agents-doc/workflow.md": "Prose about nonawf-maintenance tooling.\n",
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
