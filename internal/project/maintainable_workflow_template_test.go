package project

import (
	"io/fs"
	"sort"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// invariant: rendering/workflow-skill-templates:maintainable-code-review-lenses (TestMaintainableCodeReviewLenses)
func TestMaintainableCodeReviewLenses(t *testing.T) {
	outputs := map[string]string{
		"plan": renderAgentGolden(t, "plan-reviewer", map[string]any{
			"prefix": "example", "vars": map[string]any{}, "data": map[string]any{},
			"layout": map[string]any{"plansDir": "docs/plans"},
		}),
		"code": renderAgentGolden(t, "code-reviewer", map[string]any{
			"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "layout": testLayout(),
		}),
		"adr": renderAgentGolden(t, "adr-reviewer", map[string]any{
			"prefix": "example", "vars": map[string]any{}, "data": map[string]any{},
			"layout": map[string]any{"adrDir": "docs/decisions", "indexMd": "docs/decisions/INDEX.md"},
		}),
	}
	contracts := map[string][]string{
		"plan": {
			"model", "ownership", "representations", "translation boundaries", "dependency direction", "test seams",
			"where a protected property requires dependency ordering", "necessary enabling refactors precede dependent behavior", "bounded to the failure they prevent", "deterministically verifiable",
			"approved, deferred, or declined disposition", "needless indirection", "pattern mandates", "unapproved or unjustified abstraction, indirection, validation, test machinery, tooling, cleanup, or process", "Do not demand additions merely because more structure, testing, cleanup, or validation is imaginable",
		},
		"code": {
			"cohesion", "coupling", "dependency direction", "representation leakage", "duplicated policy", "testability",
			"needless indirection", "protected-contract conformance", "coherent transactions rather than literal planned phase grouping", "behavior bolted onto an unsuitable abstraction",
			"refactoring scope silently broadened", "unapproved or unjustified abstraction, indirection, validation, test machinery, tooling, cleanup, or process", "Do not demand additions merely because more structure, testing, cleanup, or validation is imaginable",
		},
		"adr": {
			"semantic model", "representation", "module/package boundary", "dependency direction", "ownership boundary",
			"comparable structural contract", "only when a Decision changes", "cohesion", "representation isolation",
			"enabling-refactor disposition", "testable seams", "justification for indirection", "skip this lens",
		},
	}
	for name, out := range outputs {
		wants := append([]string{"docs/maintainable-code-design.md", "Report-only"}, contracts[name]...)
		if name != "adr" {
			wants = append(wants,
				"one-home ownership", "obsolete-path", "dependency-direction", "representation-boundary", "residual-debt",
				"actionable findings digest", "semantic owner", "concrete maintainability risk",
				"`location` records the affected location", "`issue` names the semantic owner and concrete risk",
				"`suggested_fix` names the smallest clean remediation", "`classification` records remediation ownership",
				"severity remains informational", "pure aesthetic", "non-admissible",
			)
		} else if strings.Contains(out, "actionable findings digest") || strings.Contains(out, "review-maintainability-risk") {
			t.Errorf("ADR reviewer unexpectedly carries implementation-and-plan risk threshold:\n%s", out)
		}
		for _, want := range wants {
			if !strings.Contains(out, want) {
				t.Errorf("%s reviewer missing %q:\n%s", name, want, out)
			}
		}
		if forbidden := map[string]string{
			"plan": "necessary enabling refactors are ordered before dependent behavior",
			"code": "conformance to the settled design",
		}[name]; forbidden != "" && strings.Contains(out, forbidden) {
			t.Errorf("%s reviewer retains route-binding review rule %q", name, forbidden)
		}
		for _, line := range strings.Split(out, "\n") {
			directive := strings.TrimLeft(strings.TrimSpace(line), "-*#0123456789. )")
			lower := strings.ToLower(directive)
			for _, banned := range []string{"edit ", "fix ", "apply a fix", "commit ", "loop a re-review", "re-review "} {
				if strings.HasPrefix(lower, banned) {
					t.Errorf("%s reviewer must remain report-only, found directive %q:\n%s", name, directive, out)
				}
			}
		}
	}
}

// invariant: rendering/workflow-skill-templates:concrete-maintainability-review (TestConcreteMaintainabilityReview)
func TestConcreteMaintainabilityReview(t *testing.T) {
	const partialPath = "partials/review-maintainability-risk.md"
	partial, err := fs.ReadFile(templates.FS, partialPath)
	if err != nil {
		t.Fatalf("concrete maintainability shared home is absent: read %s: %v", partialPath, err)
	}
	body := string(partial)
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			t.Errorf("concrete maintainability partial must be heading-free, found %q", line)
		}
	}
	for _, want := range []string{
		"semantic owner", "affected location", "concrete maintainability risk", "smallest clean remediation", "classification",
		"`location`", "`issue`", "`suggested_fix`", "`classification`", "severity remains informational",
		"future divergence", "ambiguous ownership", "hidden parallel policy", "inappropriate dependency", "representation leakage", "wrong model", "unbounded debt", "reduced verification strength",
		"aesthetic", "non-admissible", "competing clean local remedies", "brainstorming", "ADR review", "AF-013", "one bounded verify pass",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("concrete maintainability shared home missing %q", want)
		}
	}
	consumers := []string{
		"agents/code-reviewer.md.tmpl", "agents/plan-reviewer.md.tmpl",
		"skills/reviewing-impl/SKILL.md.tmpl", "skills/reviewing-plan/SKILL.md.tmpl",
	}
	expectedConsumers := make(map[string]bool, len(consumers))
	for _, consumer := range consumers {
		expectedConsumers[consumer] = true
	}
	actualConsumers := map[string]int{}
	err = fs.WalkDir(templates.FS, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		raw, readErr := fs.ReadFile(templates.FS, path)
		if readErr != nil {
			return readErr
		}
		source := string(raw)
		if count := strings.Count(source, "<!-- awf:include review-maintainability-risk -->"); count > 0 {
			actualConsumers[path] = count
		}
		if path != partialPath && strings.Contains(source, "concrete maintainability risk") && strings.Contains(source, "smallest clean remediation") && strings.Contains(source, "non-admissible") {
			t.Errorf("%s duplicates the operative concrete-maintainability threshold", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for path, count := range actualConsumers {
		if !expectedConsumers[path] || count != 1 {
			t.Errorf("unexpected concrete-maintainability consumer %s with %d includes", path, count)
		}
	}
	for consumer := range expectedConsumers {
		if actualConsumers[consumer] != 1 {
			t.Errorf("%s has %d concrete-maintainability includes, want 1", consumer, actualConsumers[consumer])
		}
	}
	for name, data := range map[string]map[string]any{
		"configured": {"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}, "skills": map[string]bool{}, "targetSubagentTools": true},
		"empty":      {"prefix": "", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}, "skills": map[string]bool{}},
	} {
		for _, consumer := range consumers {
			var out string
			if strings.HasPrefix(consumer, "agents/") {
				out = renderAgentGolden(t, strings.TrimSuffix(strings.TrimPrefix(consumer, "agents/"), ".md.tmpl"), data)
			} else {
				out = renderSkillGolden(t, strings.Split(consumer, "/")[1], data)
			}
			assertNoLeaks(t, out)
			for _, want := range []string{"semantic owner", "concrete maintainability risk", "non-admissible"} {
				if !strings.Contains(out, want) {
					t.Errorf("%s/%s missing concrete-maintainability semantic %q", name, consumer, want)
				}
			}
		}
	}
}

// invariant: rendering/workflow-skill-templates:strongest-practical-durable-oracle (TestStrongestPracticalDurableOracleContract)
func TestStrongestPracticalDurableOracleContract(t *testing.T) {
	const partialPath = "partials/durable-oracle.md"
	partial, err := fs.ReadFile(templates.FS, partialPath)
	if err != nil {
		t.Fatalf("durable-oracle shared home is absent: read %s: %v", partialPath, err)
	}
	body := string(partial)
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			t.Errorf("durable-oracle partial must be heading-free, found %q", line)
		}
	}
	for _, want := range []string{
		"strongest practical durable oracle", "normal and preferred path",
		"observed failing for the right reason and then passing", "concrete reason",
		"preserve or improve verification strength", "strongest safe, reproducible alternative",
		"deterministic integration or reproduction harness", "contract or invariant test",
		"scripted, reproducible manual verification", "recorded inputs and expected result",
		"durable automation is unavailable", "strongest safe evidence that can be retained",
		"stress or invariant evidence", "safe fixture or dry-run evidence",
		"Never weaken expected behaviour", "Never weaken verification strength",
		"root cause rather than the symptom", "guidance, not a requirement to mechanically attempt",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("durable-oracle shared home missing %q", want)
		}
	}

	consumers := []string{
		"skills/bugfix/SKILL.md.tmpl", "skills/tdd/SKILL.md.tmpl",
		"skills/debugging/SKILL.md.tmpl", "skills/writing-plans/SKILL.md.tmpl",
		"agents/plan-reviewer.md.tmpl", "agents/code-reviewer.md.tmpl",
		"docs/testing.md.tmpl",
	}
	expected := make(map[string]bool, len(consumers))
	for _, consumer := range consumers {
		expected[consumer] = true
	}
	actual := map[string]int{}
	for _, root := range []string{"skills", "agents", "docs", "partials"} {
		err := fs.WalkDir(templates.FS, root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			raw, readErr := fs.ReadFile(templates.FS, path)
			if readErr != nil {
				return readErr
			}
			rawBody := string(raw)
			if count := strings.Count(rawBody, "<!-- awf:include durable-oracle -->"); count > 0 {
				actual[path] = count
			}
			if path != partialPath && !expected[path] && durableOracleParallelDoctrine(rawBody) {
				t.Errorf("%s contains a parallel durable-oracle policy home", path)
			}
			if path != partialPath && strings.Contains(rawBody, "Every behaviour-changing fix requires the strongest practical durable oracle") {
				t.Errorf("%s copies the canonical durable-oracle rule", path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, consumer := range consumers {
		if actual[consumer] != 1 {
			t.Errorf("%s has %d durable-oracle includes, want 1", consumer, actual[consumer])
		}
		raw, err := fs.ReadFile(templates.FS, consumer)
		if err != nil {
			t.Fatal(err)
		}
		rawBody := string(raw)
		for _, retired := range []string{
			"fixes ship with a regression test",
			"Isolate with a failing test, written first",
			"must fail for the right reason on the unfixed code",
			"every behaviour-changing change has a regression test",
			"**testing-discipline**: behaviour-changing tasks have regression tests",
		} {
			if strings.Contains(rawBody, retired) {
				t.Errorf("%s retains universal red-first rule %q", consumer, retired)
			}
		}
	}
	for path, count := range actual {
		if !expected[path] {
			t.Errorf("unexpected durable-oracle consumer %s with %d includes", path, count)
		}
	}
	for name, candidate := range map[string]string{
		"direct-copy": "The strongest practical durable oracle uses red then green as the preferred path; an alternative needs a concrete reason and must preserve verification strength.",
		"paraphrase":  "Use the most durable evidence: prefer observed red then green, require a concrete reason for any alternative, and keep verification at least as strong.",
	} {
		if !durableOracleParallelDoctrine(candidate) {
			t.Errorf("%s parallel durable-oracle doctrine escaped the single-home detector", name)
		}
	}
	if durableOracleParallelDoctrine("An ordinary fix uses its regression test; record an alternative harness when needed.") {
		t.Error("legitimate stage-local durable-oracle procedure was misclassified as a parallel policy home")
	}

	tddSource, err := fs.ReadFile(templates.FS, "skills/tdd/SKILL.md.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tddSource), "alternative-oracle path does not widen to features") {
		t.Error("TDD alternative path is not explicitly confined to fixes")
	}
	for _, reviewer := range []string{"agents/code-reviewer.md.tmpl", "agents/plan-reviewer.md.tmpl"} {
		raw, err := fs.ReadFile(templates.FS, reviewer)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), "non-fix behaviour-changing") {
			t.Errorf("%s does not preserve regression evidence for non-fix behavior changes", reviewer)
		}
	}

	for name, data := range map[string]map[string]any{
		"configured": {"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}, "skills": map[string]bool{}},
		"empty":      {"prefix": "", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}, "skills": map[string]bool{}},
	} {
		for _, consumer := range consumers {
			out := renderGolden(t, consumer, data)
			assertNoLeaks(t, out)
			for _, want := range []string{"strongest practical durable oracle", "concrete reason", "Never weaken expected behaviour"} {
				if !strings.Contains(out, want) {
					t.Errorf("%s/%s missing durable-oracle semantic %q", name, consumer, want)
				}
			}
		}
	}
}

func durableOracleParallelDoctrine(body string) bool {
	lower := strings.ToLower(body)
	strongest := (strings.Contains(lower, "strongest practical") && strings.Contains(lower, "durable oracle")) || strings.Contains(lower, "most durable evidence")
	preferredRedGreen := strings.Contains(lower, "red") && strings.Contains(lower, "green") && (strings.Contains(lower, "prefer") || strings.Contains(lower, "normal path"))
	concreteReason := strings.Contains(lower, "concrete reason")
	strength := strings.Contains(lower, "preserve verification strength") || strings.Contains(lower, "keep verification at least as strong")
	alternative := strings.Contains(lower, "alternative")
	return strongest && preferredRedGreen && concreteReason && strength && alternative
}

// invariant: rendering/workflow-skill-templates:clean-integration (TestCleanIntegrationContract)
func TestCleanIntegrationContract(t *testing.T) {
	const partialPath = "partials/clean-integration.md"
	partial, err := fs.ReadFile(templates.FS, partialPath)
	if err != nil {
		t.Fatalf("clean-integration shared home is absent: read %s: %v", partialPath, err)
	}
	body := string(partial)
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			t.Errorf("clean-integration partial must be heading-free, found %q", line)
		}
	}
	for _, want := range []string{
		"maintainable-code-design.md", "current and target owner", "narrowest clean integration point",
		"bounded enabling refactor", "obsolete or parallel path", "verification surfaces", "residual debt",
		"YAGNI", "unrelated cleanup", "test-shaped production design", "separate material decision",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("clean-integration shared home missing %q", want)
		}
	}

	// This explicit manifest is intentionally consumer-owned rather than inferred
	// from text: new consumers must make their ownership visible here.
	consumers := []string{
		"skills/brainstorming/SKILL.md.tmpl", "skills/writing-plans/SKILL.md.tmpl",
		"skills/executing-direct/SKILL.md.tmpl", "skills/executing-plans/SKILL.md.tmpl",
		"skills/subagent-driven-development/SKILL.md.tmpl", "skills/bugfix/SKILL.md.tmpl",
		"skills/tdd/SKILL.md.tmpl", "skills/reviewing-plan/SKILL.md.tmpl",
		"skills/reviewing-impl/SKILL.md.tmpl", "agents/implementer.md.tmpl",
		"agents/plan-reviewer.md.tmpl", "agents/code-reviewer.md.tmpl",
	}
	for _, consumer := range consumers {
		raw, err := fs.ReadFile(templates.FS, consumer)
		if err != nil {
			t.Fatal(err)
		}
		rawBody := string(raw)
		if got := strings.Count(rawBody, "<!-- awf:include clean-integration -->"); got != 1 {
			t.Errorf("%s has %d clean-integration includes, want 1", consumer, got)
		}
	}
	for _, root := range []string{"skills", "agents", "partials"} {
		err := fs.WalkDir(templates.FS, root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || path == partialPath {
				return nil
			}
			raw, readErr := fs.ReadFile(templates.FS, path)
			if readErr != nil {
				return readErr
			}
			if cleanIntegrationParallelDoctrine(string(raw)) {
				t.Errorf("%s contains a parallel clean-integration operative rule", path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for name, candidate := range map[string]string{
		"direct-copy": "determine the current and target owner, choose the narrowest clean integration point, remove the obsolete or parallel path, and state residual debt",
		"paraphrase":  "identify where behavior belongs now and where it should belong, choose a narrow entry point, remove the superseded route, and record remaining debt",
	} {
		if !cleanIntegrationParallelDoctrine(candidate) {
			t.Errorf("%s parallel doctrine escaped the single-home detector", name)
		}
	}
	if cleanIntegrationParallelDoctrine("Preserve the approved owner and report unrelated cleanup.") {
		t.Error("legitimate stage-local protocol was misclassified as parallel doctrine")
	}

	variants := map[string]map[string]any{
		"configured": {"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}, "skills": map[string]bool{"reviewing-impl": true}, "targetSubagentTools": true},
		"empty":      {"prefix": "", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}, "skills": map[string]bool{}},
	}
	for variant, data := range variants {
		for _, consumer := range consumers {
			var out string
			if strings.HasPrefix(consumer, "agents/") {
				out = renderAgentGolden(t, strings.TrimSuffix(strings.TrimPrefix(consumer, "agents/"), ".md.tmpl"), data)
			} else {
				out = renderSkillGolden(t, strings.Split(consumer, "/")[1], data)
			}
			assertNoLeaks(t, out)
			for _, want := range []string{"current and target owner", "residual debt"} {
				if !strings.Contains(out, want) {
					t.Errorf("%s/%s missing clean-integration semantic %q", variant, consumer, want)
				}
			}
		}
	}
}

func cleanIntegrationParallelDoctrine(body string) bool {
	lower := strings.ToLower(body)
	groups := []bool{
		(strings.Contains(lower, "current") && strings.Contains(lower, "target") && strings.Contains(lower, "owner")) || (strings.Contains(lower, "belongs now") && strings.Contains(lower, "should belong")),
		strings.Contains(lower, "clean integration point") || strings.Contains(lower, "narrow entry point"),
		strings.Contains(lower, "obsolete or parallel path") || strings.Contains(lower, "superseded route"),
		strings.Contains(lower, "residual debt") || strings.Contains(lower, "remaining debt"),
	}
	matched := 0
	for _, group := range groups {
		if group {
			matched++
		}
	}
	return matched >= 3
}

// invariant: rendering/workflow-skill-templates:authority-guided-implementation-autonomy (TestAuthorityGuidedImplementationAutonomy)
func TestAuthorityGuidedImplementationAutonomy(t *testing.T) {
	partial, err := fs.ReadFile(templates.FS, "partials/implementation-autonomy.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(partial), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			t.Errorf("shared autonomy partial must not interrupt consumer structure with heading %q", line)
		}
	}

	consumers := []string{"agents/implementer.md.tmpl", "skills/executing-direct/SKILL.md.tmpl", "skills/bugfix/SKILL.md.tmpl", "skills/tdd/SKILL.md.tmpl", "skills/executing-plans/SKILL.md.tmpl", "skills/subagent-driven-development/SKILL.md.tmpl", "skills/reviewing-impl/SKILL.md.tmpl"}
	variants := map[string]map[string]any{
		"configured": {
			"prefix": "example", "vars": map[string]any{"gateCmd": "./x gate"}, "layout": testLayout(),
			"data": catalog.Standard.Agents["plan-reviewer"].Data, "skills": map[string]bool{"reviewing-impl": true}, "targetSubagentTools": true,
		},
		"empty": {"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}, "skills": map[string]bool{}},
	}
	wants := []string{
		"Resolve implementation findings autonomously",
		"applicable ADRs, current-state claims, and repository authority",
		"approved outcome, material scope, settled durable boundaries, and required verification",
		"Diagnose a source contradiction, correctness or safety concern, review finding, blocker symptom, or failed check",
		"reasoned non-mechanical deviation that another owner or reviewer can rely on records its changed detail, rationale, governing authority, and verification",
		"The parent may add an omitted path",
		"necessary to complete the approved outcome",
		"An omitted path alone is not a reason to stop",
		"Do not replan the approved outcome, broaden material scope, overturn settled durable choices, weaken an oracle, or perform unrelated cleanup",
		"authorities conflict or must change",
		"approved outcome or material scope must change",
		"genuine unresolved design fork remains",
		"safe or correct completion inside the boundary is impossible",
		"required verification remains unreachable after reasonable diagnosis and remediation",
	}
	obsolete := []string{
		"overturn settled structural choices",
		"If a newly discovered need affects behavior, scope, structure, dependencies, patterns, checks, or testing strategy",
		"complete approval-requiring invalidating-source report",
	}
	for _, consumer := range consumers {
		raw, err := fs.ReadFile(templates.FS, consumer)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(string(raw), "<!-- awf:include implementation-autonomy -->"); got != 1 {
			t.Errorf("%s has %d autonomy includes, want 1", consumer, got)
		}
		for variant, data := range variants {
			var out string
			if strings.HasPrefix(consumer, "agents/") {
				out = renderAgentGolden(t, "implementer", data)
			} else {
				out = renderSkillGolden(t, strings.Split(consumer, "/")[1], data)
			}
			assertNoLeaks(t, out)
			for _, want := range wants {
				if !strings.Contains(out, want) {
					t.Errorf("%s/%s missing %q", variant, consumer, want)
				}
			}
			for _, reject := range obsolete {
				if strings.Contains(out, reject) {
					t.Errorf("%s/%s retains obsolete broad-stop rule %q", variant, consumer, reject)
				}
			}
			if consumer == "skills/executing-plans/SKILL.md.tmpl" {
				for _, want := range []string{"Amend the mutable plan", "only when another phase or reviewer can rely on that material instruction"} {
					if !strings.Contains(out, want) {
						t.Errorf("%s/%s missing inline reconciliation directive %q", variant, consumer, want)
					}
				}
			}
		}
	}
}

// The approval boundary is an unresolved material decision, not the act of
// mutating production code. This pins both halves: the new trigger and its
// enumerated material-decision cases must be present, and the retired
// artifact-class trigger must be gone. Asserting only the absence of the old
// wording would pass against a partial that says nothing at all.
//
// invariant: rendering/workflow-skill-templates:independent-workflow-escalation (TestProductionCodeOutlineApprovalProjection)
// invariant: rendering/workflow-skill-templates:mandatory-approval-boundaries (TestProductionCodeOutlineApprovalProjection)
func TestProductionCodeOutlineApprovalProjection(t *testing.T) {
	partial, err := fs.ReadFile(templates.FS, "partials/outline-approval.md")
	if err != nil {
		t.Fatal(err)
	}
	body := string(partial)
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			t.Errorf("outline approval partial must not interrupt consumer structure with heading %q", line)
		}
	}
	for _, want := range []string{
		"An approval boundary is triggered by an unresolved material decision, never by the act of mutating production code",
		"the requested outcome is materially ambiguous",
		"viable approaches carry meaningfully different durable consequences",
		"it is unsettled whether externally observable behaviour, compatibility, safety, or material scope should change",
		"repository authority contradicts the request",
		"a required verification oracle would have to be weakened",
		"an irreversible or destructive action is not already authorized",
		"the clean implementation exposes a separate load-bearing decision",
		"Routine implementation detail creates no approval boundary, whatever kind of file it touches",
		"retained conversation", "Decision-log evidence", "explicit request to execute a named plan", "Architecture summary",
		"brainstorming is the sole owner", "parent-supplied protected contract", "never recreate the approval interaction",
		"stops without mutation to report to its parent when that contract is absent or must change",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("outline approval partial missing %q", want)
		}
	}
	// The retired trigger made the artifact class the boundary. Its carve-out
	// list existed only to undo that, so both must be gone: the general rule
	// already leaves documentation, test, and generated-output work autonomous.
	for _, retired := range []string{
		"mechanical production refactors",
		"tests that prepare a production change",
		"Documentation-only work, test-only maintenance",
		"generated-output-only work, and non-code mechanical work remain autonomous",
	} {
		if strings.Contains(body, retired) {
			t.Errorf("outline approval partial retains retired universal-approval wording %q", retired)
		}
	}
	for _, consumer := range []string{
		"agents/implementer.md.tmpl", "skills/executing-direct/SKILL.md.tmpl", "skills/tdd/SKILL.md.tmpl",
		"skills/bugfix/SKILL.md.tmpl", "skills/executing-plans/SKILL.md.tmpl", "skills/subagent-driven-development/SKILL.md.tmpl",
		"skills/writing-plans/SKILL.md.tmpl", "skills/proposing-adr/SKILL.md.tmpl",
	} {
		raw, err := fs.ReadFile(templates.FS, consumer)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(string(raw), "<!-- awf:include outline-approval -->"); got != 1 {
			t.Errorf("%s has %d outline approval includes, want 1", consumer, got)
		}
	}
	// The approval-boundary claim's user-facing stop renders from the checkpoint
	// partial, not from the intake partial above. Without this the claim is
	// nominal: reverting the checkpoint trigger alone left the suite green.
	checkpoint, err := fs.ReadFile(templates.FS, "partials/checkpoint-approval.md")
	if err != nil {
		t.Fatal(err)
	}
	checkpointBody := string(checkpoint)
	for _, want := range []string{
		"Before ADR or plan authoring that would resolve an unresolved material decision",
		"Before beginning a change whose material decision is unresolved",
	} {
		if !strings.Contains(checkpointBody, want) {
			t.Errorf("checkpoint approval partial missing material-decision trigger %q", want)
		}
	}
	for _, retired := range []string{
		"Before ADR or plan authoring for a hand-authored production-code change",
		"Before beginning a hand-authored production-code change",
	} {
		if strings.Contains(checkpointBody, retired) {
			t.Errorf("checkpoint approval partial retains retired artifact-class trigger %q", retired)
		}
	}
	for templateID, direct := range map[string]string{
		"skills/bugfix/SKILL.md.tmpl": "For materially larger work, ask the user whether to",
		"skills/tdd/SKILL.md.tmpl":    "Escalate materially larger work by asking the user whether to",
	} {
		raw, err := fs.ReadFile(templates.FS, templateID)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), direct) {
			t.Errorf("%s retains direct larger-work user route %q", templateID, direct)
		}
	}
}

// The doctrine has exactly one authored definition, and every other surface
// renders that source or points at it. The proof runs over RENDERED surfaces in
// both governance footprints rather than the template tree, because convention
// parts under .awf/ are rendered surfaces too and a template-only scan cannot
// see them. The agent guide is allowed to carry the thesis sentence inline for
// its self-contained style, so that assertion is derived from the definition
// itself and the two copies cannot drift apart.
//
// invariant: rendering/workflow-skill-templates:protected-contract-over-route (TestProtectedContractDoctrineSingleHome)
// invariant: rendering/workflow-skill-templates:closed-workflow-profiles (TestProtectedContractDoctrineSingleHome)
func TestProtectedContractDoctrineSingleHome(t *testing.T) {
	const source = "partials/protected-contract.md"
	partial, err := fs.ReadFile(templates.FS, source)
	if err != nil {
		t.Fatal(err)
	}
	body := string(partial)
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			t.Errorf("protected-contract partial must not interrupt consumer structure with heading %q", line)
		}
	}
	// Each clause carries distinct weight: the protected set including the four
	// protections the governing record's reassurance rests on, the route set, and
	// the per-constraint precedence rule that decides a mixed rule.
	definition := []string{
		"The workflow governs a change's protected contract, not its execution route",
		"the requested outcome, the explicitly settled durable choices, the material scope",
		"the required verification strength, the prohibited shortcuts, and every constraint an active project rule places on one of these",
		"which includes generated-source ownership, drift detection, and path and worktree confinement",
		"Everything else about how the change is carried out is the route",
		"phase and task boundaries, their order, local names, file and symbol inventories, helper allocation, execution mode",
		"Precedence is decided per constraint, not per rule",
		"one rule may be protected in its protected clauses and subordinate in its route clauses",
		"A route detail binds only when a settled decision states that it is load-bearing",
	}
	for _, want := range definition {
		if !strings.Contains(body, want) {
			t.Errorf("protected-contract partial missing %q", want)
		}
	}
	thesis := definition[0]
	// Clauses no surface but the doctrine's rendered home may carry. The thesis is
	// excluded: the agent guide carries it as its pointer text, checked below.
	exclusive := []string{
		"Precedence is decided per constraint, not per rule",
		"the required verification strength, the prohibited shortcuts, and every constraint an active project rule places on one of these",
	}
	workflowTemplate, err := fs.ReadFile(templates.FS, "docs/workflow.md.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(workflowTemplate), "<!-- awf:include protected-contract -->"); got != 1 {
		t.Errorf("workflow template has %d protected-contract includes, want 1", got)
	}

	// Both footprints carry the same doctrine; neither is a lighter standard.
	for _, profile := range []catalog.Profile{catalog.ProfileCore, catalog.ProfileFull} {
		t.Run(string(profile), func(t *testing.T) {
			root := scaffold(t, "prefix: example\nprofile: "+string(profile)+"\nintegrationBranch: main\n")
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			files, err := renderAll(p)
			if err != nil {
				t.Fatal(err)
			}
			rendered := map[string]string{}
			for _, f := range files {
				rendered[f.Path] = f.Content
			}
			workflowPath := layout(renderInputsForTest(p)).Singletons["workflowRef"]
			workflow, ok := rendered[workflowPath]
			if !ok {
				t.Fatalf("rendered workflow document %q absent", workflowPath)
			}
			want := definition
			if profile != catalog.ProfileCore {
				want = append(append([]string{}, definition...), "and current-state authority")
			}
			for _, clause := range want {
				if !strings.Contains(workflow, clause) {
					t.Errorf("rendered workflow document missing doctrine clause %q", clause)
				}
			}
			// Core has no current-state authority to protect, so the Full-only
			// protection must not leak into the Core footprint.
			if profile == catalog.ProfileCore && strings.Contains(workflow, "current-state authority") {
				t.Error("Core workflow document names Full-only current-state authority")
			}
			// No second rendered surface may carry a defining clause. The thesis is
			// additionally legal in the agent guide, which carries it under a link;
			// any third surface stating it is a standalone copy.
			for clause, allowed := range map[string]map[string]bool{
				exclusive[0]: {workflowPath: true},
				exclusive[1]: {workflowPath: true},
				thesis:       {workflowPath: true, "AGENTS.md": true},
			} {
				var carriers []string
				for path, content := range rendered {
					if !allowed[path] && strings.Contains(content, clause) {
						carriers = append(carriers, path)
					}
				}
				if len(carriers) != 0 {
					sort.Strings(carriers)
					t.Errorf("doctrine clause %q defined outside its rendered home in %v", clause, carriers)
				}
			}
			guide, ok := rendered["AGENTS.md"]
			if !ok {
				t.Fatal("rendered agent guide absent")
			}
			if !strings.Contains(guide, thesis) {
				t.Errorf("rendered agent guide does not carry the doctrine thesis %q", thesis)
			}
			// Carrying the thesis is only legal because the guide points at the
			// doctrine's home from the same paragraph. Scoping to that line matters:
			// the document map always links the workflow document, so a guide-wide
			// search is satisfied by a link that has nothing to do with the doctrine.
			var thesisLine string
			for _, line := range strings.Split(guide, "\n") {
				if strings.Contains(line, thesis) {
					thesisLine = line
					break
				}
			}
			if thesisLine == "" {
				t.Fatalf("rendered agent guide does not carry the doctrine thesis %q", thesis)
			}
			if !strings.Contains(thesisLine, "]("+workflowPath+")") {
				t.Errorf("agent guide states the doctrine without linking %q in the same paragraph", workflowPath)
			}
		})
	}
}

// invariant: rendering/workflow-skill-templates:mandatory-approval-boundaries (TestMaintainableCodeStageCoverage)
// invariant: rendering/workflow-skill-templates:maintainable-code-stage-coverage (TestMaintainableCodeStageCoverage)
func TestMaintainableCodeStageCoverage(t *testing.T) {
	allSkills := map[string]bool{
		"debugging": true, "reviewing-impl": true, "tdd": true,
	}
	type stageContract struct {
		wants   []string
		rejects []string
	}
	patternToolbox := []string{
		"Patterns are a non-exhaustive glossary",
		"Strategy can select among genuinely varying policies",
		"Adapter can isolate an incompatible representation",
		"Facade can present a focused entry point",
		"Value objects can protect a value",
		"Repositories can isolate storage concerns",
		"Ports-and-adapters can keep policy independent",
	}
	cases := map[string]stageContract{
		"brainstorming": {wants: []string{
			"docs/maintainable-code-design.md", "semantic model and ownership", "representation boundaries", "dependency direction", "test seams", "preparatory-refactor decision", "proportionate simplicity contract", "scope and exclusions", "structural approach and dependencies", "patterns or abstractions", "checks and testing strategy", "few sentences rather than a fixed checklist", "final approved design becomes the implementation boundary",
		}},
		"proposing-adr": {wants: []string{
			"docs/maintainable-code-design.md", "settled model and ownership, boundaries, dependency direction, constraints, and enabling work", "here and in Decision", "do not replace them with a pattern name",
		}},
		"refactor-coupling-audit": {wants: []string{
			"docs/maintainable-code-design.md", "duplication, coupling, representation leakage, or a workaround", "bounded enabling-refactor or larger-work result", "perform it first, include it in the current effort, defer it in a durable project-owned record, or decline it with the trade-off stated", "ADR scope", "Context section before", "Decision is drafted", "Scope shrink rule", "defer X",
		}},
		"writing-plans": {wants: []string{
			"docs/maintainable-code-design.md", "settled model and ownership, boundaries, dependency direction, representation translations, refactor decision, prohibited shortcuts, and validation", "ordered executable tasks", "self-contained", "no prior conversation context", "sequencing, coordination, or resumability materially helps", "records and operationalizes approved choices", "rather than inventing speculative structure, checks, or work",
		}},
		"tdd": {wants: []string{
			"docs/maintainable-code-design.md", "bounded enabling refactor", "duplication, coupling, representation leakage, or a workaround", "Route materially larger work through the active workflow's design discussion", "perform it first, include it in the current effort, defer it in a durable project-owned record, or decline it with the trade-off stated", "smallest behavior-proving, model-supporting seam", "force representation leakage or needless indirection", "confirm it fails for the right reason", "minimal root-cause change", "Ground tests, checks, seams, and harness work only in changed behavior, a demonstrated regression, an existing documented contract, or a clearly applicable project invariant", "reject speculative test or policy machinery", "Resolve implementation findings autonomously", "applicable ADRs, current-state claims, and repository authority", "approved outcome, material scope, settled durable boundaries", "Stop and report through the active workflow only",
		}},
		"executing-plans": {wants: []string{
			"docs/maintainable-code-design.md", "preserve the plan's settled durable choices", "bounded enabling refactor", "reassess if grounded source contradicts them", "stop rather than bolt correctness onto the wrong abstraction", "Each brief explicitly identifies the parent-supplied approved boundary", "Resolve implementation findings autonomously", "applicable ADRs, current-state claims, and repository authority", "approved outcome, material scope, settled durable boundaries", "Stop and report through the active workflow only",
		}},
		"executing-direct": {wants: []string{
			"docs/maintainable-code-design.md", "assess bounded enabling refactoring before editing", "preserve settled boundaries", "no independent need for brainstorming", "material choice or clarification", "Re-evaluate planning", "only when that independent need fires", "Resolve implementation findings autonomously", "applicable ADRs, current-state claims, and repository authority", "approved outcome, material scope, settled durable boundaries", "Stop and report through the active workflow only",
		}},
		"subagent-driven-development": {wants: []string{
			"docs/maintainable-code-design.md", "preserve the plan's settled durable choices", "bounded enabling refactor", "reassess them if grounded source contradicts them", "stop and escalate rather than accept a bolt-on workaround", "Sequential dispatch only, never parallel", "complete phase", "explicitly identify the parent-supplied approved boundary", "commit-disabled implementation child", "Resolve implementation findings autonomously", "applicable ADRs, current-state claims, and repository authority", "approved outcome, material scope, settled durable boundaries", "Stop and report through the active workflow only",
		}},
		"bugfix": {wants: []string{
			"docs/maintainable-code-design.md", "unsuitable model or boundary", "bounded enabling work that prevents a workaround", "For materially larger work, route the disposition through the active workflow's design discussion", "perform it first, include it in the current effort, defer it in a durable project-owned record, or decline it with the trade-off stated", "root-cause fix, not the symptom", "one concern per commit", "Resolve implementation findings autonomously", "applicable ADRs, current-state claims, and repository authority", "approved outcome, material scope, settled durable boundaries", "Stop and report through the active workflow only",
		}},
		"reviewing-plan": {wants: []string{
			"docs/maintainable-code-design.md", "current and target owner", "residual debt", "report-only judge", "full mode",
		}},
		"reviewing-impl": {wants: []string{
			"docs/maintainable-code-design.md", "current and target owner", "residual debt", "independent assurance", "report-only findings",
		}},
	}
	for skill, contract := range cases {
		t.Run(skill, func(t *testing.T) {
			out := renderSkillGolden(t, skill, map[string]any{
				"prefix": "example", "vars": map[string]any{}, "data": map[string]any{},
				"layout": testLayout(), "skills": allSkills, "targetSubagentTools": true,
			})
			assertNoLeaks(t, out)
			for _, want := range contract.wants {
				if !strings.Contains(out, want) {
					t.Errorf("%s output missing %q:\n%s", skill, want, out)
				}
			}
			for _, reject := range append(patternToolbox, contract.rejects...) {
				if strings.Contains(out, reject) {
					t.Errorf("%s output must not duplicate guide pattern-toolbox or later-stage handoff prose %q:\n%s", skill, reject, out)
				}
			}
		})
	}
}

// invariant: rendering/workflow-skill-templates:maintainable-code-subagent-contract (TestMaintainableCodeSubagentContract)
func TestMaintainableCodeSubagentContract(t *testing.T) {
	renderSection := func(t *testing.T, templateID, section string, data map[string]any) string {
		t.Helper()
		src, err := fs.ReadFile(templates.FS, templateID)
		if err != nil {
			t.Fatalf("read %s: %v", templateID, err)
		}
		expanded, err := render.ExpandIncludes(string(src), templates.FS)
		if err != nil {
			t.Fatalf("expand %s: %v", templateID, err)
		}
		for _, segment := range parseSections(expanded) {
			if segment.IsSection && segment.Name == section {
				out, err := render.Execute(segment.Text, data, nil, "test")
				if err != nil {
					t.Fatalf("render %s section %s: %v", templateID, section, err)
				}
				assertNoLeaks(t, out)
				return out
			}
		}
		t.Fatalf("%s has no %s section", templateID, section)
		return ""
	}
	const subagentTemplate = "skills/subagent-driven-development/SKILL.md.tmpl"
	const scopedContext = "semantic boundary and ownership, external/internal representations and their translation point, allowed dependency direction, preparatory-refactor decision, prohibited bolt-on shortcuts, validation expectations"
	data := map[string]any{"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "layout": testLayout()}

	context := renderSection(t, subagentTemplate, "procedure-extract-context", data)
	if !strings.Contains(context, scopedContext) {
		t.Errorf("scoped context is not closed to the six task-relevant categories:\n%s", context)
	}

	for _, tc := range []struct {
		name, dispatch, review, reportOnly string
		data                               map[string]any
		wants                              []string
	}{
		{
			name: "Pi", data: map[string]any{"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "layout": testLayout(), "targetSubagentTools": true},
			dispatch: "known clean and green baseline", review: "Review is report-only and phase-level", reportOnly: "parent-owned",
			wants: []string{"commit-disabled implementation child", "complete phase", "explicitly identify the parent-supplied approved boundary", "parent independently inventories the checkout"},
		},
		{
			name: "generic", data: data,
			dispatch: "known clean and green baseline", review: "Review is report-only and phase-level", reportOnly: "parent-owned",
			wants: []string{"commit-disabled implementation child", "complete phase", "explicitly identify the parent-supplied approved boundary", "parent independently inventories the checkout"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dispatch := renderSection(t, subagentTemplate, "dispatch-conventions", tc.data)
			if !strings.Contains(dispatch, tc.dispatch) {
				t.Errorf("dispatch branch missing its instruction %q:\n%s", tc.dispatch, dispatch)
			}
			for _, want := range tc.wants {
				if !strings.Contains(dispatch, want) {
					t.Errorf("dispatch branch missing %q:\n%s", want, dispatch)
				}
			}
			review := renderSection(t, subagentTemplate, "per-task-review", tc.data)
			if !strings.Contains(review, tc.review) {
				t.Errorf("review branch missing its instruction %q:\n%s", tc.review, review)
			}
			if !strings.Contains(review, tc.reportOnly) {
				t.Errorf("review branch lost report-only clause %q:\n%s", tc.reportOnly, review)
			}
		})
	}

	implementer := renderAgentGolden(t, "implementer", data)
	for _, want := range []string{
		"Resolve implementation findings autonomously",
		"applicable ADRs, current-state claims, and repository authority",
		"reasoned non-mechanical deviation that another owner or reviewer can rely on records its changed detail, rationale, governing authority, and verification",
		"The parent may add an omitted path",
		"An omitted path alone is not a reason to stop",
		"implementation child reports the necessity",
		"never modifies an unassigned path",
		"Do not replan the approved outcome, broaden material scope",
		"or perform unrelated cleanup",
		"`deviations: none` or each changed detail, rationale, governing authority",
		"and verification, plus deliberately out-of-scope work",
		"current and target owner",
		"residual debt",
	} {
		if !strings.Contains(implementer, want) {
			t.Errorf("implementer contract missing authority-preserving deviation clause %q:\n%s", want, implementer)
		}
	}

	inline := renderSection(t, "skills/executing-plans/SKILL.md.tmpl", "procedure-per-task", data)
	const inlineContext = "Iterate phases, not tasks"
	if !strings.Contains(inline, inlineContext) {
		t.Errorf("inline current-task context is not closed to the six categories:\n%s", inline)
	}
	if !strings.Contains(inline, "Each brief explicitly identifies the parent-supplied approved boundary") {
		t.Errorf("inline helper dispatch omits the parent-supplied approved boundary:\n%s", inline)
	}
	for _, want := range []string{"current and target owner", "residual debt"} {
		if !strings.Contains(inline, want) {
			t.Errorf("inline helper dispatch omits clean-integration semantic %q:\n%s", want, inline)
		}
	}
}

// invariant: rendering/workflow-skill-templates:implementer-context-grounding (TestCodeGraphNavigationGuidance)
// invariant: tooling/authority-queries:codegraph-navigation-boundary (TestCodeGraphNavigationGuidance)
func TestCodeGraphNavigationGuidance(t *testing.T) {
	for name := range catalog.Standard.Skills {
		templateID := "skills/" + name + "/SKILL.md.tmpl"
		source, err := fs.ReadFile(templates.FS, templateID)
		if err != nil {
			t.Fatalf("read %s: %v", templateID, err)
		}
		expanded, err := render.ExpandIncludes(string(source), templates.FS)
		if err != nil {
			t.Fatalf("expand %s: %v", templateID, err)
		}
		for _, forbidden := range []string{"awf context", "./awf topic ", "AWF_CONTEXT_SPILL_V1", "context-spill", "--show", "--full"} {
			if strings.Contains(expanded, forbidden) {
				t.Errorf("%s retains retired navigation %q", templateID, forbidden)
			}
		}
	}
	for _, templateID := range []string{"partials/orientation-ladder.md", "partials/context-orientation.md"} {
		body, err := fs.ReadFile(templates.FS, templateID)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"CodeGraph", "exact-known-file", "genuinely trivial", "./awf resolve topic", "./awf read topic"} {
			if !strings.Contains(string(body), want) {
				t.Errorf("%s missing %q", templateID, want)
			}
		}
	}
	for _, templateID := range []string{"skills/reviewing-adr/SKILL.md.tmpl", "skills/adr-lifecycle/SKILL.md.tmpl"} {
		body, err := fs.ReadFile(templates.FS, templateID)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "./awf read adr") {
			t.Errorf("%s does not use focused ADR reads", templateID)
		}
		if templateID == "skills/reviewing-adr/SKILL.md.tmpl" && !strings.Contains(string(body), "./awf read topic") {
			t.Errorf("%s does not use focused topic reads", templateID)
		}
	}
}

// invariant: rendering/workflow-skill-templates:authority-guided-implementation-autonomy (TestConditionalVerifyPass)
// invariant: rendering/workflow-skill-templates:authority-guided-review-remediation (TestConditionalVerifyPass)
// TestConditionalVerifyPass pins ADR-0197 item 3: the reviewing skills
// dispatch the verify pass only for reasoned or user-decision fixes, a
// solely-mechanical round records the skip, and a fix-free round dispatches
// nothing.
func TestConditionalVerifyPass(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"layout": testLayout(),
		"data":   map[string]any{},
	}
	for _, name := range []string{"reviewing-adr", "reviewing-plan"} {
		t.Run(name, func(t *testing.T) {
			out := renderSkillGolden(t, name, data)
			assertOrderedPhrases(t, out,
				"**Verify pass.**",
				"applied no fixes dispatches no verify pass",
				"all classified `mechanical` skips it",
				"recording the skip and its ground",
				"classified `reasoned` or was applied under a `user-decision` ruling",
			)
		})
	}

	implementationReview := renderSkillGolden(t, "reviewing-impl", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}, "skills": map[string]bool{"effort-workflow": true}, "targetSubagentTools": true,
	})
	for _, want := range []string{"locally obvious, low-risk, directly verified", "Uncertainty resolves toward review", "Effort-free review creates no effort", "returns to `example-effort-workflow`"} {
		if !strings.Contains(implementationReview, want) {
			t.Errorf("implementation review missing %q", want)
		}
	}
}
