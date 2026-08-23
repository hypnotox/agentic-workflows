package project

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/plan"
)

// A first adoption accepts an existing intrinsically governed record without
// rewriting it or using its number to select a parser.
// invariant: adr-system/plan-artifacts:plan-v2-assignment-advisories (TestPlanV2AssignmentAdvisories)
func TestPlanV2AssignmentAdvisories(t *testing.T) {
	plans := []plan.Plan{{Filename: "2026-08-02-v2.md", Path: "docs/plans/2026-08-02-v2.md", Format: "plan-v2", Status: "Proposed", DoD: []plan.DoDItem{{Slug: "complete"}}, Phases: []plan.Phase{{Number: 1, Tasks: []plan.Task{{Number: 1, Fields: plan.TaskFields{}}}}}}}
	drift, notes := planArtifactReport(plans, adr.Corpus{})
	if len(drift) != 0 || len(notes) != 1 || !strings.Contains(notes[0], "no outcome assignment") {
		t.Fatalf("planArtifactReport = drift %#v, notes %#v", drift, notes)
	}
	// A Decision assignment in one Proposed plan cannot cover another plan.
	source := "---\nformat: current-state-v4\nstatus: Proposed\ndate: 2026-08-02\nslug: fixture\n---\n# ADR-fixture: Fixture\n\n## Context\n\nContext.\n\n## Decision\n\n1. `decision: first` First.\n\n## State changes\n\nNone.\n\n## Consequences\n\nNone.\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-08-02: Proposed\n"
	record, err := adr.ParseV4("fixture.md", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := adr.NewCorpus([]adr.ADR{record})
	if err != nil {
		t.Fatal(err)
	}
	assigned := plan.Plan{
		Filename: "assigned.md", Path: "docs/plans/assigned.md", Format: "plan-v2", Status: "Proposed",
		ADRs: []plan.ADRLink{{Slug: "fixture"}},
		Phases: []plan.Phase{{
			Number: 1,
			Tasks: []plan.Task{{
				Number: 1,
				Fields: plan.TaskFields{Applying: []plan.DecisionRef{{
					Authored: "fixture:first", ADR: "fixture", Selector: "first", Kind: "Applying",
				}}},
			}},
		}},
	}
	missing := assigned
	missing.Filename, missing.Path = "missing.md", "docs/plans/missing.md"
	missing.Phases = append([]plan.Phase(nil), assigned.Phases...)
	missing.Phases[0].Tasks = append([]plan.Task(nil), assigned.Phases[0].Tasks...)
	missing.Phases[0].Tasks[0].Fields.Applying = nil
	drift, notes = planArtifactReport([]plan.Plan{assigned, missing}, corpus)
	if len(drift) != 0 || !slices.Contains(notes, "missing.md Decision fixture:first has no Applying assignment") {
		t.Fatalf("independent assignments = drift %#v, notes %#v", drift, notes)
	}
}

func TestPlanArtifactReportFindsReferencesAndSortsNotes(t *testing.T) {
	p := plan.Plan{
		Filename: "2026-08-02-v2.md", Path: "docs/plans/2026-08-02-v2.md", Format: "plan-v2", Status: "Proposed",
		ADRs: []plan.ADRLink{{Slug: "missing"}}, DoD: []plan.DoDItem{{Slug: "one"}, {Slug: "two"}},
		Phases: []plan.Phase{
			{Number: 1, Tasks: []plan.Task{{Number: 1, Fields: plan.TaskFields{Applying: []plan.DecisionRef{{Authored: "missing:item", ADR: "missing", Selector: "item", Kind: "Applying"}}}}}},
			{Number: 2, Advances: []string{"one"}, Tasks: []plan.Task{{Number: 1, Fields: plan.TaskFields{Kind: plan.TaskSpike}}}},
		},
	}
	drift, notes := planArtifactReport([]plan.Plan{p}, adr.Corpus{})
	if len(drift) != 2 || !strings.Contains(drift[0].Detail, "ADR not found") || !strings.Contains(drift[1].Detail, "ADR not found") {
		t.Fatalf("hard findings = %#v", drift)
	}
	if !slices.IsSorted(notes) || len(notes) != 2 || !strings.Contains(strings.Join(notes, "\n"), "advanced but has no Completes") || !strings.Contains(strings.Join(notes, "\n"), "no outcome assignment") {
		t.Fatalf("notes = %#v", notes)
	}
}

func TestPlanContextHelpersRejectMissingReferencesAndSelectors(t *testing.T) {
	p := plan.Plan{Filename: "v2.md", Phases: []plan.Phase{{Number: 1, Tasks: []plan.Task{{Number: 1}}}}}
	if _, _, err := selectedRefs(p, "9"); err == nil {
		t.Fatal("missing selector accepted")
	}
	phase, task, err := selectedRefs(p, "1")
	if err != nil || phase.Number != 1 || task.Number != 0 {
		t.Fatalf("phase selected refs = %#v %#v %v", phase, task, err)
	}
	phase, task, err = selectedRefs(p, "1.1")
	if err != nil || phase.Number != 1 || task.Number != 1 {
		t.Fatalf("selected refs = %#v %#v %v", phase, task, err)
	}
	task.Fields.Applying = []plan.DecisionRef{{Authored: "missing:item", ADR: "missing", Selector: "item", Kind: "Applying"}}
	_, _, err = resolveSelectedPlanDecisions(p, adr.Corpus{}, phase, task)
	if err == nil || !strings.Contains(err.Error(), "ADR not found") {
		t.Fatalf("missing reference = %v", err)
	}
}

func TestPlanArtifactReportEnforcesDecisionReferenceContracts(t *testing.T) {
	source := "---\nformat: current-state-v4\nstatus: Proposed\ndate: 2026-08-02\nslug: fixture\n---\n# ADR-fixture: Fixture\n\n## Context\n\nContext.\n\n## Decision\n\n1. `decision: first` First.\n\n## State changes\n\nNone.\n\n## Consequences\n\nNone.\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-08-02: Proposed\n"
	record, err := adr.ParseV4("fixture.md", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := adr.NewCorpus([]adr.ADR{record})
	if err != nil {
		t.Fatal(err)
	}
	p := plan.Plan{
		Filename: "v2.md", Path: "docs/plans/v2.md", Format: "plan-v2", ADRs: []plan.ADRLink{{Slug: "other"}},
		Phases: []plan.Phase{{Number: 1, Tasks: []plan.Task{{Number: 1, Fields: plan.TaskFields{
			Applying: []plan.DecisionRef{{Authored: "fixture:first", ADR: "fixture", Selector: "first", Kind: "Applying"}},
		}}}}},
	}
	drift, _ := planArtifactReport([]plan.Plan{p}, corpus)
	if len(drift) != 2 || !strings.Contains(drift[0].Detail, "ADR not found") || !strings.Contains(drift[1].Detail, "Applying ADR is absent from adrs") {
		t.Fatalf("Applying membership drift = %#v", drift)
	}
	p.ADRs = []plan.ADRLink{{Slug: "fixture"}}
	p.Phases[0].Tasks[0].Fields.Applying = nil
	p.Phases[0].Tasks[0].Fields.Context = []plan.DecisionRef{{Authored: "fixture:missing", ADR: "fixture", Selector: "missing", Kind: "Context"}}
	drift, _ = planArtifactReport([]plan.Plan{p}, corpus)
	if len(drift) != 1 || !strings.Contains(drift[0].Detail, "Context requires frozen ADR") {
		t.Fatalf("context freeze drift = %#v", drift)
	}
}

// invariant: adr-system/plan-artifacts:plan-v2-decision-references (TestResolvePlanDecisionsUsesFrozenCorpusIdentityAndSelector)
func TestResolvePlanDecisionsUsesFrozenCorpusIdentityAndSelector(t *testing.T) {
	v4Source := "---\nformat: current-state-v4\nstatus: Proposed\ndate: 2026-08-02\nslug: fixture\n---\n# ADR-0001: Fixture\n\n## Context\n\nContext.\n\n## Decision\n\n1. `decision: first` First.\n\n2. `decision: zeta` Zeta.\n\n## State changes\n\nNone.\n\n## Consequences\n\nNone.\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-08-02: Proposed\n"
	v4, err := adr.ParseV4("0001-fixture.md", []byte(v4Source))
	if err != nil {
		t.Fatal(err)
	}
	v3Source := strings.Replace(v4Source, "current-state-v4", "current-state-v3", 1)
	v3Source = strings.Replace(v3Source, "1. `decision: first` First.\n\n2. `decision: zeta` Zeta.", "1. First.\n\n2. Zeta.", 1)
	v3, err := adr.ParseV3("0001-fixture.md", []byte(v3Source))
	if err != nil {
		t.Fatal(err)
	}

	resolve := func(record adr.ADR, link plan.ADRLink, ref plan.DecisionRef, context, selectorError bool) ([]plan.ResolvedDecision, error) {
		t.Helper()
		corpus, err := adr.NewCorpus([]adr.ADR{record})
		if err != nil {
			t.Fatal(err)
		}
		p := plan.Plan{Filename: "v2.md", Path: "docs/plans/v2.md", Format: "plan-v2", ADRs: []plan.ADRLink{link}, Phases: []plan.Phase{{Number: 1, Tasks: []plan.Task{{Number: 1, Fields: plan.TaskFields{Applying: []plan.DecisionRef{ref}}}}}}}
		if context {
			p.Phases[0].Tasks[0].Fields.Applying = nil
			p.Phases[0].Tasks[0].Fields.Context = []plan.DecisionRef{ref}
		}
		drift, _ := planArtifactReport([]plan.Plan{p}, corpus)
		switch {
		case context && record.IsContentAmendable():
			if len(drift) != 1 || !strings.Contains(drift[0].Detail, "Context requires frozen ADR") {
				t.Fatalf("amendable Context drift = %#v", drift)
			}
		case !selectorError && len(drift) != 0:
			t.Fatalf("planArtifactReport drift = %#v", drift)
		case selectorError && len(drift) != 1:
			t.Fatalf("selector planArtifactReport drift = %#v", drift)
		}
		phase, task := p.Phases[0], p.Phases[0].Tasks[0]
		applying, selectedContext, err := resolveSelectedPlanDecisions(p, corpus, phase, task)
		if context {
			return selectedContext, err
		}
		return applying, err
	}

	// Numbered links and retained-slug task references resolve the same V4 ADR,
	// and the reverse spelling produces the same projection record.
	applySlug := plan.DecisionRef{Authored: "fixture:first", ADR: "fixture", Selector: "first", Kind: "Applying"}
	applyNumber := plan.DecisionRef{Authored: "0001:first", ADR: "0001", Selector: "first", Kind: "Applying"}
	bySlug, err := resolve(v4, plan.ADRLink{Number: 1}, applySlug, false, false)
	if err != nil || len(bySlug) != 1 {
		t.Fatalf("number link / slug ref = %#v, %v", bySlug, err)
	}
	byNumber, err := resolve(v4, plan.ADRLink{Slug: "fixture"}, applyNumber, false, false)
	if err != nil || len(byNumber) != 1 || bySlug[0] != byNumber[0] {
		t.Fatalf("slug link / number ref = %#v, %v; want %#v", byNumber, err, bySlug)
	}

	// V4 decision IDs are stable while content is amendable, but Context needs a
	// frozen record. Copies model lifecycle states without invalidating fixtures.
	contextRef := plan.DecisionRef{Authored: "fixture:first", ADR: "fixture", Selector: "first", Kind: "Context"}
	if _, err := resolve(v4, plan.ADRLink{Slug: "fixture"}, contextRef, true, false); err == nil || !strings.Contains(err.Error(), "requires frozen ADR") {
		t.Fatalf("amendable V4 Context error = %v", err)
	}
	frozenV4 := v4
	frozenV4.Status = "Implemented"
	if resolved, err := resolve(frozenV4, plan.ADRLink{Slug: "fixture"}, contextRef, true, false); err != nil || len(resolved) != 1 {
		t.Fatalf("frozen V4 Context = %#v, %v", resolved, err)
	}
	frozenCorpus, err := adr.NewCorpus([]adr.ADR{frozenV4})
	if err != nil {
		t.Fatal(err)
	}
	contextTask := plan.Task{Number: 1, Fields: plan.TaskFields{Context: []plan.DecisionRef{contextRef}}}
	contextPhase := plan.Phase{Number: 1, Tasks: []plan.Task{contextTask}}
	contextPlan := plan.Plan{Filename: "v2.md", ADRs: []plan.ADRLink{{Slug: "fixture"}}, Phases: []plan.Phase{contextPhase}}
	applying, selectedContext, err := resolveSelectedPlanDecisions(contextPlan, frozenCorpus, contextPhase, contextTask)
	if err != nil || len(applying) != 0 || len(selectedContext) != 1 || selectedContext[0].Key != "0001:first" {
		t.Fatalf("selected frozen V4 Context = applying %#v, context %#v, err %v", applying, selectedContext, err)
	}
	amendableV4Corpus, err := adr.NewCorpus([]adr.ADR{v4})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := resolveSelectedPlanDecisions(contextPlan, amendableV4Corpus, contextPhase, contextTask); err == nil || !strings.Contains(err.Error(), "task 1.1 Context \"fixture:first\": requires frozen ADR") {
		t.Fatalf("selected amendable Context error = %v", err)
	}
	missingContextRef := plan.DecisionRef{Authored: "fixture:missing", ADR: "fixture", Selector: "missing", Kind: "Context"}
	missingContextTask := plan.Task{Number: 1, Fields: plan.TaskFields{Context: []plan.DecisionRef{missingContextRef}}}
	missingContextPhase := plan.Phase{Number: 1, Tasks: []plan.Task{missingContextTask}}
	_, _, err = resolveSelectedPlanDecisions(contextPlan, frozenCorpus, missingContextPhase, missingContextTask)
	if !errors.Is(err, adr.ErrDecisionSelectorUnknown) || !strings.Contains(err.Error(), "task 1.1 Context \"fixture:missing\"") {
		t.Fatalf("selected Context selector error = %v", err)
	}

	frozenV3 := v3
	frozenV3.Status = "Implemented"
	ordinal := plan.DecisionRef{Authored: "fixture:#1", ADR: "fixture", Selector: "#1", Kind: "Applying"}
	if resolved, err := resolve(frozenV3, plan.ADRLink{Slug: "fixture"}, ordinal, false, false); err != nil || len(resolved) != 1 {
		t.Fatalf("frozen pre-V4 ordinal = %#v, %v", resolved, err)
	}
	amendableV3, err := adr.NewCorpus([]adr.ADR{v3})
	if err != nil {
		t.Fatal(err)
	}
	amendablePlan := plan.Plan{Filename: "v2.md", Path: "docs/plans/v2.md", Format: "plan-v2", ADRs: []plan.ADRLink{{Slug: "fixture"}}, Phases: []plan.Phase{{Number: 1, Tasks: []plan.Task{{Number: 1, Fields: plan.TaskFields{Applying: []plan.DecisionRef{ordinal}}}}}}}
	if drift, _ := planArtifactReport([]plan.Plan{amendablePlan}, amendableV3); len(drift) != 1 || !strings.Contains(drift[0].Detail, adr.ErrDecisionSelectorAmendable.Error()) {
		t.Fatalf("amendable pre-V4 ordinal drift = %#v", drift)
	}
	if _, _, err := resolveSelectedPlanDecisions(amendablePlan, amendableV3, amendablePlan.Phases[0], amendablePlan.Phases[0].Tasks[0]); err == nil || !errors.Is(err, adr.ErrDecisionSelectorAmendable) {
		t.Fatalf("amendable pre-V4 ordinal error = %v", err)
	}

	assertSelector := func(selector string, cause error) {
		t.Helper()
		_, err := resolve(frozenV4, plan.ADRLink{Slug: "fixture"}, plan.DecisionRef{Authored: "fixture:" + selector, ADR: "fixture", Selector: selector, Kind: "Applying"}, false, true)
		var typed *adr.DecisionSelectorError
		if !errors.Is(err, cause) || !errors.As(err, &typed) || !slices.IsSorted(typed.Available) || strings.Join(typed.Available, ",") != "first,zeta" {
			t.Fatalf("selector %q error = %#v", selector, err)
		}
	}
	assertSelector("#1", adr.ErrDecisionSelectorIncompatible)
	assertSelector("missing", adr.ErrDecisionSelectorUnknown)
}

func TestPlanArtifactReportValidatesSelectorsAndAssignments(t *testing.T) {
	source := "---\nformat: current-state-v4\nstatus: Proposed\ndate: 2026-08-02\nslug: fixture\n---\n# ADR-fixture: Fixture\n\n## Context\n\nContext.\n\n## Decision\n\n1. `decision: first` First.\n\n2. `decision: second` Second.\n\n## State changes\n\nNone.\n\n## Consequences\n\nNone.\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-08-02: Proposed\n"
	record, err := adr.ParseV4("fixture.md", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	record.Status = "Implemented"
	corpus, err := adr.NewCorpus([]adr.ADR{record})
	if err != nil {
		t.Fatal(err)
	}
	p := plan.Plan{Filename: "v2.md", Path: "docs/plans/v2.md", Format: "plan-v2", Status: "Proposed", ADRs: []plan.ADRLink{{Slug: "fixture"}}, DoD: []plan.DoDItem{{Slug: "advanced"}, {Slug: "complete"}}, Phases: []plan.Phase{{Number: 1, Advances: []string{"advanced"}, Completes: []string{"complete"}, Tasks: []plan.Task{{Number: 1, Fields: plan.TaskFields{Applying: []plan.DecisionRef{{Authored: "fixture:first", ADR: "fixture", Selector: "first", Kind: "Applying"}}, Context: []plan.DecisionRef{{Authored: "fixture:missing", ADR: "fixture", Selector: "missing", Kind: "Context"}}}}, {Number: 2}}}}}
	drift, notes := planArtifactReport([]plan.Plan{{Format: "plan-v1"}, p}, corpus)
	if len(drift) != 1 || !strings.Contains(drift[0].Detail, "fixture:missing") || len(notes) != 3 || !strings.Contains(strings.Join(notes, "\n"), "task 1.2 has no Applying") || !strings.Contains(strings.Join(notes, "\n"), "fixture:second has no Applying") || !strings.Contains(strings.Join(notes, "\n"), "advanced but has no Completes") {
		t.Fatalf("plan artifact report = drift %#v, notes %#v", drift, notes)
	}
}
