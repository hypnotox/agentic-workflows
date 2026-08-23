package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

func TestIndependentWorkflowEscalation(t *testing.T) {
	body := renderGolden(t, "skills/grounding/SKILL.md.tmpl", map[string]any{
		"prefix": "example", "layout": map[string]any{},
	})
	for _, want := range []string{"broad or uncertain repository premises", "advisory, report-only, single-pass, effort-noncreating", "never a workflow-chain prerequisite"} {
		if !strings.Contains(body, want) {
			t.Errorf("grounding missing %q", want)
		}
	}
}

// invariant: rendering/workflow-skill-templates:linked-plan-review-freshness (TestLinkedPlanReviewFreshness)
func TestLinkedPlanReviewFreshness(t *testing.T) {
	data := map[string]any{
		"prefix": "example", "vars": map[string]any{}, "layout": testLayout(),
		"data": map[string]any{}, "skills": map[string]bool{},
	}
	plan := renderSkillGolden(t, "reviewing-plan", data)
	for _, want := range []string{
		"every ADR resolved from parsed plan-level `adrs:`",
		"modification time or session implication",
		"every linked plan whose parsed status remains `Proposed`",
		"inventory completed phases against the changed ADR",
		"renewed implementation assurance for affected landed work",
		"settle the changed decision through brainstorming, then return to ADR amendment and ordinary ADR review",
		"exactly one fresh `plan-reviewer` verify pass",
	} {
		if !strings.Contains(plan, want) {
			t.Errorf("ordinary plan review missing %q", want)
		}
	}
	adr := renderSkillGolden(t, "reviewing-adr", data)
	for _, want := range []string{
		"substantive amendment or correction while an ADR is `Proposed`, `Accepted`, or `Implementing`",
		"Preserve the ADR's entry status",
		"an `Accepted` or `Implementing` record uses `example-adr-lifecycle`",
		"Preserve the nonterminal status with which the ADR entered review",
		"each plan review inventories completed affected phases",
		"renews assurance where the amended decision can affect landed work",
	} {
		if !strings.Contains(adr, want) {
			t.Errorf("ordinary ADR review missing %q", want)
		}
	}
	assertOrderedPhrases(t, adr, "review converges", "After the review settles, run `./awf context --show references <explicit-ADR-path>`", "Invoke ordinary `example-reviewing-plan` separately for every linked plan")
	if _, ok := catalog.Standard.Skills["reviewing-plan-"+"resync"]; ok {
		t.Fatal("retired plan review skill remains in the live catalog")
	}
}

func TestReviewingPlanTemplate(t *testing.T) {
	data := map[string]any{
		"prefix":       "example",
		"vars":         map[string]any{},
		"commitScopes": "`docs(plans)`",
		"layout":       map[string]any{"plansDir": "docs/plans"},
		"data":         map[string]any{},
	}

	out := renderSkillGolden(t, "reviewing-plan", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-reviewing-plan") {
		t.Errorf("expected 'name: example-reviewing-plan' in output:\n%s", out)
	}

	// Assert thin-dispatcher load-bearing phrases
	loadBearing := []string{
		"plan-reviewer",
		"user-decision",
		"explicit uncommitted plan path",
		"selected working-tree snapshot",
		"review that snapshot rather than HEAD",
		"mechanical fixes directly without a durable ledger",
		"substantive reasoned or user-decided findings and dispositions in plan Notes",
		"Later substantive corrections remain separate commits",
		"all universal lenses",
		"per-phase ownership",
		"helper partitions",
		"V4 stable `decision:` selectors",
		"Proposed coverage notes are advisory",
		"historical Decision prose never replaces current-state authority",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}

func TestReviewingAdrTemplate(t *testing.T) {
	data := map[string]any{
		"prefix":       "example",
		"vars":         map[string]any{},
		"commitScopes": "`docs(adr)`",
		"layout":       map[string]any{"adrDir": "docs/decisions", "plansDir": "docs/plans"},
		"data":         map[string]any{},
	}

	out := renderSkillGolden(t, "reviewing-adr", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-reviewing-adr") {
		t.Errorf("expected 'name: example-reviewing-adr' in output:\n%s", out)
	}

	// Assert thin-dispatcher load-bearing phrases
	loadBearing := []string{
		"adr-reviewer",
		"user-decision",
		"example-reviewing-plan",
		"Proposed",
		"user-provenance decision-log",
		"explicitly approved design summary",
		"repository facts do not establish consent",
		"removed unauthorized surplus commitments",
		"semantics-preserving refinements",
		"writing `none` for an empty inventory",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}

func TestRetrospectiveTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"skills": map[string]bool{"reviewing-impl": true, "proposing-adr": true},
		"vars": map[string]any{
			"gateCmd":           "./x gate",
			"invariantTestPath": "./internal/...",
		},
		"layout": map[string]any{
			"docs":        map[string]any{"pitfalls": "docs/pitfalls.md"},
			"workflowRef": "docs/workflow.md",
		},
		"data": map[string]any{},
	}

	out := renderSkillGolden(t, "retrospective", data)

	if !strings.Contains(out, "name: example-retrospective") {
		t.Errorf("expected 'name: example-retrospective' in output:\n%s", out)
	}

	// Load-bearing phrases unique to the retrospective ladder (ADR-0067).
	loadBearing := []string{
		"main thread",
		"promotion ladder",
		"Invariant",
		"example-proposing-adr",
		"docs/pitfalls.md",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}

func TestRefactorCouplingAuditTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"data":   map[string]any{},
	}

	out := renderSkillGolden(t, "refactor-coupling-audit", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-refactor-coupling-audit") {
		t.Errorf("expected 'name: example-refactor-coupling-audit' in output:\n%s", out)
	}

	// Assert load-bearing phrases unique to refactor-coupling-audit
	loadBearing := []string{
		"coupling audit",
		"Context section",
		"Sibling test files",
		"constructor",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}
