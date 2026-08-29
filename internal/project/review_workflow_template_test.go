package project

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/templates"
)

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

// invariant: rendering/workflow-skill-templates:authority-guided-review-remediation (TestAuthorityGuidedReviewRemediation)
// TestAuthorityGuidedReviewRemediation pins the authority-guided review
// remediation boundary: the shared review spine stays the single semantic home
// of finding classification, one variable-free partial carries the
// dispatcher-side routing obligation into all reviewing skills, routes plan
// contradictions back through ADR review, and removes automatic residual escalation.
func TestAuthorityGuidedReviewRemediation(t *testing.T) {
	const (
		stopCriterion   = "every viable correct remediation would contradict or change a settled user-approved design or decision, or would require an unauthorized change to an active current-state claim"
		nonTriggers     = "Ambiguity, competing clean options, severity, structural character, or survival after an earlier correction"
		residualOpening = "Diagnose every residual finding under the same boundary"
	)

	partial, err := fs.ReadFile(templates.FS, "partials/review-remediation-autonomy.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(partial), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			t.Errorf("shared review-remediation partial must not interrupt consumer structure with heading %q", line)
		}
	}

	riskPartial, err := fs.ReadFile(templates.FS, "partials/review-maintainability-risk.md")
	if err != nil {
		t.Fatal(err)
	}
	riskRoutingClauses := []string{
		"reject a risk-free preference as a non-admissible reviewer-contract violation",
		"select autonomously among competing clean local remedies",
		"Route a genuinely new material choice or changed approved boundary through brainstorming independently of severity",
		"only a true authority deviation is a `user-decision` finding",
	}
	for _, clause := range riskRoutingClauses {
		if !strings.Contains(string(riskPartial), clause) {
			t.Errorf("maintainability dispatcher contract missing %q", clause)
		}
		mutated := strings.ReplaceAll(string(riskPartial), clause, "missing-risk-routing-clause")
		if strings.Contains(mutated, clause) {
			t.Errorf("maintainability dispatcher mutation failed to remove %q", clause)
		}
	}
	adrSkill, err := fs.ReadFile(templates.FS, "skills/reviewing-adr/SKILL.md.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(adrSkill), "review-maintainability-risk") || strings.Contains(string(adrSkill), "non-admissible reviewer-contract violation") {
		t.Errorf("ADR review unexpectedly carries dispatcher concrete-risk contract")
	}
	// Exactly the three tokens ExpandIncludes rejects inside a partial.
	for _, token := range []string{"awf:section", "awf:end", "awf:include"} {
		if strings.Contains(string(partial), token) {
			t.Errorf("shared review-remediation partial carries the rejected token %q", token)
		}
	}

	retired := []string{
		"Escalate any residual structural findings as `user-decision` items",
		"the step-2 return edge applies to initial-dispatch findings only",
		// Pinned from the second character: lowercase in the plan and ADR
		// skills, capitalized in resync.
		"o not loop further without explicit user direction",
		"a genuine design fork or unresolved ambiguity that should not be decided unilaterally",
		"present a genuine unresolved `user-decision` fork or consensus deviation and stop",
		"(return edge, step 2)",
	}

	dispatcherWants := []string{
		"**Rule.** Apply mechanical corrections directly.",
		"Apply reasoned corrections autonomously with a concise rationale.",
		"single semantic home",
		"routes that classification without redefining it",
		"**Flexible details.**",
		nonTriggers,
		"None transfers the choice to the user.",
		"**Stop when.**",
		"**Required evidence.**",
		"retain exactly one fresh verify-pass dispatch",
		"Do not dispatch another same-artifact review loop.",
		"A consensus deviation remains a user decision.",
	}
	fullDispatcherWants := []string{
		stopCriterion,
		"active current-state claim; cite the affected authority",
		"A new material decision or changed approved boundary follows the brainstorming route before ADR mutation",
		"pauses at brainstorming's pre-artifact outline approval boundary",
		"remain implementation detail for this workflow to resolve",
		"not the unresolved design fork named by an implementation stop condition",
		"A plan correction that would contradict linked ADR authority returns to ADR amendment and independent review",
	}
	coreDispatcherWants := []string{
		"every viable correct remediation would contradict or change the approved boundary",
		"Route a new material decision or changed boundary through brainstorming",
		"wait at its outline approval boundary",
		"Resolve competing clean options inside the approved boundary as implementation detail",
	}

	// The pinned non-trigger enumeration deliberately starts at "competing
	// clean options" because the leading word's case differs between the two
	// prose homes, so assert the dropped word separately rather than leaving
	// the claim's ambiguity clause proven by nothing.
	assertNamesAmbiguity := func(t *testing.T, label, out string) {
		t.Helper()
		if !strings.Contains(strings.ToLower(out), "ambiguity") {
			t.Errorf("%s never names ambiguity as a non-trigger", label)
		}
	}

	// Every Latitude: exact routing replacement needs a positive pin. Absence
	// of the retired predecessor alone lets the replacement be degraded back
	// toward an unconditioned escalation without turning the suite red.
	routingWants := map[string]string{
		"reviewing-plan": "route the finding through `example-brainstorming` with the cited affected authority and wait at its pre-artifact approval boundary",
		"reviewing-adr":  "route the finding through `example-brainstorming` with the cited affected authority and wait at its pre-artifact approval boundary",
		"reviewing-impl": "a `user-decision` finding or consensus deviation remains. Route it through `example-brainstorming` with the cited affected authority",
	}

	skillVariants := map[string]map[string]any{
		"full-configured": {
			"prefix": "example", "profile": "full", "vars": map[string]any{"gateCmd": "./x gate"}, "layout": testLayout(),
			"commitScopes":        "`docs(plans)`",
			"skills":              map[string]bool{"effort-workflow": true, "adr-lifecycle": true, "reviewing-impl": true},
			"targetSubagentTools": true,
		},
		"full-empty": {
			"prefix": "example", "profile": "full", "vars": map[string]any{}, "layout": testLayout(),
			"data": map[string]any{}, "skills": map[string]bool{},
		},
		"core-configured": {
			"prefix": "example", "profile": "core", "vars": map[string]any{"gateCmd": "./x gate"}, "layout": testLayout(),
			"skills": map[string]bool{}, "targetSubagentTools": true,
		},
		"core-empty": {
			"prefix": "example", "profile": "core", "vars": map[string]any{}, "layout": testLayout(),
			"data": map[string]any{}, "skills": map[string]bool{},
		},
	}

	for _, skill := range []string{"reviewing-plan", "reviewing-adr", "reviewing-impl"} {
		raw, err := fs.ReadFile(templates.FS, "skills/"+skill+"/SKILL.md.tmpl")
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(string(raw), "<!-- awf:include review-remediation-autonomy -->"); got != 1 {
			t.Errorf("skills/%s has %d review-remediation includes, want 1", skill, got)
		}
		for _, reject := range append(retired,
			"present to the user with the cited affected authority and wait",
			"present a `user-decision` finding with the cited affected authority") {
			if strings.Contains(string(raw), reject) {
				t.Errorf("skills/%s source retains retired escalation phrase %q", skill, reject)
			}
		}
		for variant, data := range skillVariants {
			out := renderSkillGolden(t, skill, data)
			assertNoLeaks(t, out)
			for _, want := range dispatcherWants {
				if !strings.Contains(out, want) {
					t.Errorf("%s/%s missing dispatcher clause %q", variant, skill, want)
				}
			}
			profileWants := fullDispatcherWants
			if strings.HasPrefix(variant, "core-") {
				profileWants = coreDispatcherWants
			}
			for _, want := range profileWants {
				if !strings.Contains(out, want) {
					t.Errorf("%s/%s missing profile dispatcher clause %q", variant, skill, want)
				}
			}
			for _, reject := range retired {
				if strings.Contains(out, reject) {
					t.Errorf("%s/%s retains retired escalation phrase %q", variant, skill, reject)
				}
			}
			assertNamesAmbiguity(t, variant+"/"+skill, out)
			residualWant := residualOpening
			if strings.HasPrefix(variant, "core-") {
				residualWant = "Diagnose residual findings under the same boundary"
			}
			if !strings.Contains(out, residualWant) {
				t.Errorf("%s/%s missing residual re-review replacement %q", variant, skill, residualWant)
			}
			if want := routingWants[skill]; !strings.Contains(out, want) {
				t.Errorf("%s/%s missing routing replacement %q", variant, skill, want)
			}
			if skill == "reviewing-adr" {
				for _, forbidden := range riskRoutingClauses {
					if strings.Contains(out, forbidden) {
						t.Errorf("%s/%s unexpectedly carries concrete-risk dispatcher clause %q", variant, skill, forbidden)
					}
				}
			} else {
				for _, want := range riskRoutingClauses {
					if !strings.Contains(out, want) {
						t.Errorf("%s/%s missing concrete-risk dispatcher clause %q", variant, skill, want)
					}
				}
			}
		}
	}

	reviewerData := map[string]map[string]any{
		"adr-reviewer": {
			"prefix": "example",
			"vars": map[string]any{
				"invariantTestPath": "internal/adrtools/invariants_test.go",
				"activeMdRegenCmd":  "go test ./internal/adrtools/",
			},
			"layout": map[string]any{"adrDir": "docs/decisions", "indexMd": "docs/decisions/INDEX.md"},
			"data": map[string]any{
				"focusItems": []map[string]any{
					{
						"name":        "context-grounding",
						"description": "Verify factual claims in the Context section against named files, ADRs, and state docs; flag stale claims and drift since brainstorm.",
					},
				},
			},
		},
		"plan-reviewer": {
			"prefix": "example",
			"vars":   map[string]any{},
			"layout": map[string]any{"plansDir": "docs/plans"},
			"data": map[string]any{
				"focusItems": []map[string]any{
					{
						"name":        "convention-alignment-extra",
						"description": "Verify commit subjects follow Conventional Commits; flag subjects over 72 chars or missing scope.",
					},
				},
				"docCurrencyItems": []map[string]any{
					{"check": ".awf/topics/parts/<domain>/<topic>/current-state.md - update when plan shifts current authority"},
					{"check": "docs/workflow.md - update when plan changes a workflow rule"},
					{"check": "AGENTS.md - update when plan changes chain, principles, or invariants"},
					{"check": "docs/decisions/INDEX.md - regenerate when plan flips an ADR status"},
				},
			},
		},
		"code-reviewer": {
			"prefix": "example",
			"vars":   map[string]any{},
			"data": map[string]any{
				"correctnessTraps": []map[string]any{
					{"description": "Check that error return paths use %w wrapping so callers can inspect the error chain."},
					{"description": "Flag nil pointer dereferences in struct methods where the receiver may be nil."},
				},
				"docCurrencyItems": []map[string]any{
					{"check": ".awf/topics/parts/<domain>/<topic>/current-state.md - update when the implementation shifts current authority"},
					{"check": "docs/decisions/INDEX.md - regenerate when ADR status flips to Implemented"},
				},
			},
		},
	}
	reviewerWants := []string{
		"every viable correct remediation would contradict or change a settled user-approved design or decision, or would require an unauthorized change to an active current-state claim",
		"competing clean options, severity, structural character, and the fact that a finding survived a prior correction",
		"cite the affected authority and name the deviation it would require",
		"is not an unauthorized deviation merely because its proposed future state differs from current state",
		"would make a new load-bearing choice material outside approved durable boundaries",
		"changing accepted semantics is a `user-decision` under the Consensus adherence rule below, while removing authority-free surplus is not",
		"Removing an unaccepted surplus commitment restores the accepted decision set and is an authority-preserving `reasoned` correction",
		// Pin the surviving classification bullets by their own text. The
		// bare tokens `mechanical` and `reasoned` also occur in the finding
		// schema, so asserting those alone lets a whole bullet be deleted
		// while the suite stays green.
		"- **mechanical**: the answer is unambiguous from existing rules, docs, or code",
		"- **reasoned**: a good answer can be reached by reading the relevant code or docs",
		"user-decision",
		"suggested_fix",
	}
	for _, name := range []string{"adr-reviewer", "plan-reviewer", "code-reviewer"} {
		outs := map[string]string{
			"populated": renderAgentGolden(t, name, reviewerData[name]),
			"empty": renderAgentGolden(t, name, map[string]any{
				"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{},
			}),
		}
		for variant, out := range outs {
			assertNoLeaks(t, out)
			// The claim predicates the ambiguity non-trigger on the shared
			// spine, whose leading "Ambiguity," reaches only these agent
			// bodies. A bare token is not enough here: code-reviewer names
			// ambiguity elsewhere, so pin the spine sentence's own opening.
			if !strings.Contains(out, "Ambiguity, competing clean options") {
				t.Errorf("%s/%s spine paragraph no longer opens the non-trigger list with ambiguity", variant, name)
			}
			for _, want := range reviewerWants {
				if !strings.Contains(out, want) {
					t.Errorf("%s/%s missing spine clause %q", variant, name, want)
				}
			}
			for _, reject := range retired {
				if strings.Contains(out, reject) {
					t.Errorf("%s/%s retains retired escalation phrase %q", variant, name, reject)
				}
			}
		}
	}
}

// invariant: rendering/workflow-skill-templates:semantic-owner-assurance-decomposition (TestSemanticOwnerAssuranceDecomposition)
func TestSemanticOwnerAssuranceDecomposition(t *testing.T) {
	partial, err := fs.ReadFile(templates.FS, "partials/semantic-owner-assurance.md")
	if err != nil {
		t.Fatal(err)
	}
	body := string(partial)
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			t.Errorf("semantic-owner partial must not interrupt consumer structure with heading %q", line)
		}
	}
	for _, want := range []string{
		"Before assigning or reviewing a broad implementation unit, the parent identifies its semantic owners",
		"separates independently verifiable owners into distinct implementation, settlement, and assurance units",
		"cross-owner composition is itself one coherent transaction or protected contract",
		"same underlying semantic concern or violated contract across separable owners",
		"not severity, reviewer lens, or remediation classification",
		"finite remaining scope",
		"ordinary bounded review",
		"originating transaction's sole verify pass",
		"without another reviewer dispatch",
		"terminal assurance covers composed integration effects and the complete range",
		"Unrelated blockers stay under implementation-autonomy routing and never widen the active outcome",
		"No file, line, commit, task, finding-count, or elapsed-time threshold",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("semantic-owner contract missing %q", want)
		}
	}

	consumers := []string{
		"agents/code-reviewer.md.tmpl",
		"agents/implementer.md.tmpl",
		"skills/executing-plans/SKILL.md.tmpl",
		"skills/subagent-driven-development/SKILL.md.tmpl",
		"skills/reviewing-impl/SKILL.md.tmpl",
		"docs/workflow.md.tmpl",
	}
	for _, path := range consumers {
		source, err := fs.ReadFile(templates.FS, path)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(string(source), "<!-- awf:include semantic-owner-assurance -->"); got != 1 {
			t.Errorf("%s semantic-owner include count = %d, want 1", path, got)
		}
	}

	spine, err := fs.ReadFile(templates.FS, "partials/review-spine-tail.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(spine), "Do not discard findings with different locations even if the root cause is the same") {
		t.Error("review spine no longer preserves distinct-location findings")
	}
}

func TestReviewingImplTemplate(t *testing.T) {
	out := renderSkillGolden(t, "reviewing-impl", map[string]any{"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "layout": testLayout()})
	if !strings.Contains(out, "Effort-free review creates no effort") {
		t.Fatal("review contract missing")
	}
}
