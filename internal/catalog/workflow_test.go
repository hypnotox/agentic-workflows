package catalog

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/templates"
)

func TestStandardWorkflowMappingsAreExactAndComplete(t *testing.T) {
	want := map[string]WorkflowMapping{
		"brainstorming":               {Kind: WorkflowChain, PhaseEffect: PhaseStart, Phase: "brainstorming"},
		"executing-direct":            {Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "implementation", RouteEffect: RouteSelectDirect, ImplementationMode: "inline-execution", RequiresPhases: []string{"brainstorming"}},
		"proposing-adr":               {Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "adr-authoring", RouteEffect: RouteSelectADR, RequiresPhases: []string{"brainstorming"}},
		"reviewing-adr":               {Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "adr-review", RequiresPhases: []string{"adr-authoring"}},
		"writing-plans":               {Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "planning", RouteEffect: RoutePromoteADRPlan, RequiresPhases: []string{"adr-review", "brainstorming"}},
		"reviewing-plan":              {Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "plan-review", RequiresPhases: []string{"planning"}},
		"reviewing-plan-resync":       {Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "adr-plan-resync", RequiresPhases: []string{"plan-review"}},
		"executing-plans":             {Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "implementation", ImplementationMode: "inline-execution", RequiresPhases: []string{"adr-plan-resync", "plan-review"}},
		"subagent-driven-development": {Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "implementation", ImplementationMode: "subagent-driven-development", RequiresPhases: []string{"adr-plan-resync", "plan-review"}},
		"reviewing-impl":              {Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "implementation-review", RequiresPhases: []string{"implementation"}},
		"retrospective":               {Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "retrospective", RouteEffect: RouteSelectInvestigationIfUnrouted, TerminalEffect: TerminalArmCompletion, RequiresPhases: []string{"implementation-review", "investigation"}},
		"bugfix":                      {Kind: WorkflowTask, PhaseEffect: PhaseStart, Phase: "brainstorming", RouteEffect: RouteSelectBugfix},
		"debugging":                   {Kind: WorkflowTask, PhaseEffect: PhaseStart, Phase: "investigation", Activity: "debugging"},
		"exploring":                   {Kind: WorkflowSupport, PhaseEffect: PhaseCurrent, Activity: "exploration"},
		"tdd":                         {Kind: WorkflowSupport, PhaseEffect: PhaseCurrent, Activity: "tdd", RequiresPhases: []string{"implementation"}},
		"adr-lifecycle":               {Kind: WorkflowSupport, PhaseEffect: PhaseCurrent, Activity: "adr-lifecycle"},
		"refactor-coupling-audit":     {Kind: WorkflowSupport, PhaseEffect: PhaseCurrent, Activity: "refactor-coupling-audit", RequiresPhases: []string{"brainstorming"}},
		"roadmap-graduation":          {Kind: WorkflowSupport, PhaseEffect: PhaseCurrent, Activity: "roadmap-graduation"},
	}
	got := make(map[string]WorkflowMapping, len(Standard.Skills))
	for name, spec := range Standard.Skills {
		if spec.Workflow != nil {
			got[name] = *spec.Workflow
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("standard workflow mappings differ\n got: %#v\nwant: %#v", got, want)
	}
	if err := ValidateWorkflowMappings(Standard); err != nil {
		t.Fatal(err)
	}
	if !containsRequirement(Standard.Skills["brainstorming"].RequiresSkills, "executing-direct") {
		t.Fatal("brainstorming does not require executing-direct")
	}
}

func TestExecutingDirectTemplateIsFixedAndTargetNeutral(t *testing.T) {
	body, err := fs.ReadFile(templates.FS, "skills/executing-direct/SKILL.md.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, required := range []string{"neither an ADR nor a plan", "applicable current-state authority", "project's required formatting and verification commands", "{{ .prefix }}-reviewing-impl"} {
		if !strings.Contains(text, required) {
			t.Errorf("executing-direct template lacks %q", required)
		}
	}
	for _, targetSpecific := range []string{"Pi", "Claude", "Cursor", ".pi/"} {
		if strings.Contains(text, targetSpecific) {
			t.Errorf("executing-direct template contains target-specific term %q", targetSpecific)
		}
	}
}

func TestStandardWorkflowMappingsCoverEveryRoute(t *testing.T) {
	covered := map[string]bool{}
	for _, spec := range Standard.Skills {
		switch spec.Workflow.RouteEffect {
		case RouteNone:
		case RouteSelectDirect:
			covered["direct"] = true
		case RouteSelectADR:
			covered["adr"] = true
		case RouteSelectPlan:
			covered["plan"] = true
		case RoutePromoteADRPlan:
			covered["plan"], covered["adr-plan"] = true, true
		case RouteSelectBugfix:
			covered["bugfix"] = true
		case RouteSelectInvestigationIfUnrouted:
			covered["investigation-only"] = true
		}
	}
	for _, route := range []string{"direct", "adr", "plan", "adr-plan", "bugfix", "investigation-only"} {
		if !covered[route] {
			t.Errorf("route %q is uncovered", route)
		}
	}
}

func TestWorkflowMappingsForSkillsRejectsDisabledAndStaleEntries(t *testing.T) {
	if err := ValidateWorkflowMappings(nil); err == nil {
		t.Fatal("nil workflow catalog unexpectedly passed validation")
	}
	if _, err := WorkflowMappingsForSkills(nil, nil); err == nil {
		t.Fatal("nil enabled workflow catalog unexpectedly passed validation")
	}
	got, err := WorkflowMappingsForSkills(Standard, []string{"brainstorming", "tdd"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["brainstorming"].Phase != "brainstorming" {
		t.Fatalf("enabled mappings = %#v", got)
	}
	if _, present := got["executing-direct"]; present {
		t.Fatal("disabled executing-direct leaked into enabled mappings")
	}
	for _, enabled := range [][]string{{"stale-skill"}, {"tdd", "tdd"}} {
		if _, err := WorkflowMappingsForSkills(Standard, enabled); err == nil {
			t.Fatalf("enabled skills %v unexpectedly passed", enabled)
		}
	}
	unmapped := &Catalog{Skills: map[string]SkillSpec{"subject": {}}}
	if _, err := WorkflowMappingsForSkills(unmapped, []string{"subject"}); err == nil {
		t.Fatal("enabled unmapped workflow unexpectedly passed")
	}
	invalid := *Standard.Skills["tdd"].Workflow
	invalid.Kind = "raw"
	if _, err := WorkflowMappingsForSkills(&Catalog{Skills: map[string]SkillSpec{"subject": {Workflow: &invalid}}}, []string{"subject"}); err == nil {
		t.Fatal("enabled invalid workflow unexpectedly passed")
	}
}

func TestWorkflowMappingValidationRejectsInvalidCases(t *testing.T) {
	tests := []struct {
		name string
		edit func(*WorkflowMapping)
		want string
	}{
		{"unknown kind", func(m *WorkflowMapping) { m.Kind = "raw" }, "unknown kind"},
		{"unknown phase effect", func(m *WorkflowMapping) { m.PhaseEffect = "raw" }, "unknown phase effect"},
		{"unknown route effect", func(m *WorkflowMapping) { m.RouteEffect = "raw" }, "unknown route effect"},
		{"unknown terminal effect", func(m *WorkflowMapping) { m.TerminalEffect = "raw" }, "unknown terminal effect"},
		{"unknown phase", func(m *WorkflowMapping) { m.Phase = "raw" }, "unknown phase"},
		{"unknown activity", func(m *WorkflowMapping) { m.Activity = "raw" }, "unknown activity"},
		{"unknown implementation mode", func(m *WorkflowMapping) { m.ImplementationMode = "raw" }, "unknown implementation mode"},
		{"unsorted predecessors", func(m *WorkflowMapping) { m.RequiresPhases = []string{"planning", "brainstorming"} }, "not sorted"},
		{"duplicate predecessors", func(m *WorkflowMapping) { m.RequiresPhases = []string{"brainstorming", "brainstorming"} }, "duplicate"},
		{"unknown predecessor", func(m *WorkflowMapping) { m.RequiresPhases = []string{"raw"} }, "unknown required phase"},
		{"impossible terminal mapping", func(m *WorkflowMapping) { m.TerminalEffect = TerminalArmCompletion }, "terminal effect requires"},
		{"invalid route combination", func(m *WorkflowMapping) { m.RouteEffect = RouteSelectBugfix }, "bugfix selection requires"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mapping := *Standard.Skills["executing-direct"].Workflow
			mapping.RequiresPhases = append([]string(nil), mapping.RequiresPhases...)
			tc.edit(&mapping)
			cat := &Catalog{Skills: map[string]SkillSpec{"subject": {Workflow: &mapping}}}
			err := ValidateWorkflowMappings(cat)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateWorkflowMappings() error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestWorkflowCombinationRejectsEveryImpossibleShape(t *testing.T) {
	tests := []struct {
		name    string
		mapping WorkflowMapping
		want    string
	}{
		{"none phase effect", WorkflowMapping{Kind: WorkflowChain, PhaseEffect: PhaseNone}, "phase effect cannot be none"},
		{"current carries phase", WorkflowMapping{Kind: WorkflowSupport, PhaseEffect: PhaseCurrent, Phase: "implementation", Activity: "tdd"}, "current-phase mapping may only"},
		{"current lacks activity", WorkflowMapping{Kind: WorkflowSupport, PhaseEffect: PhaseCurrent}, "current-phase mapping requires"},
		{"changing lacks phase", WorkflowMapping{Kind: WorkflowTask, PhaseEffect: PhaseStart}, "phase-changing mapping requires"},
		{"support starts", WorkflowMapping{Kind: WorkflowSupport, PhaseEffect: PhaseStart, Phase: "brainstorming"}, "support mapping must"},
		{"chain starts wrong phase", WorkflowMapping{Kind: WorkflowChain, PhaseEffect: PhaseStart, Phase: "planning"}, "chain start must"},
		{"chain transition lacks predecessor", WorkflowMapping{Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "planning"}, "chain transition requires"},
		{"task transitions", WorkflowMapping{Kind: WorkflowTask, PhaseEffect: PhaseTransition, Phase: "implementation", RequiresPhases: []string{"brainstorming"}}, "task mapping must"},
		{"start has predecessor", WorkflowMapping{Kind: WorkflowTask, PhaseEffect: PhaseStart, Phase: "brainstorming", RequiresPhases: []string{"investigation"}}, "phase start cannot"},
		{"activity outside current", WorkflowMapping{Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "implementation", Activity: "tdd", RequiresPhases: []string{"brainstorming"}}, "activity requires"},
		{"mode outside implementation", WorkflowMapping{Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "planning", ImplementationMode: "inline-execution", RequiresPhases: []string{"brainstorming"}}, "implementation mode requires"},
		{"route on start", WorkflowMapping{Kind: WorkflowTask, PhaseEffect: PhaseStart, Phase: "brainstorming", RouteEffect: RouteSelectDirect}, "route effect requires"},
		{"bad direct selection", WorkflowMapping{Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "implementation", RouteEffect: RouteSelectDirect, RequiresPhases: []string{"brainstorming"}}, "direct selection requires"},
		{"bad ADR selection", WorkflowMapping{Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "planning", RouteEffect: RouteSelectADR, RequiresPhases: []string{"brainstorming"}}, "ADR selection requires"},
		{"bad plan selection", WorkflowMapping{Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "implementation", RouteEffect: RouteSelectPlan, RequiresPhases: []string{"brainstorming"}}, "plan selection requires"},
		{"bad ADR-plan promotion", WorkflowMapping{Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "implementation", RouteEffect: RoutePromoteADRPlan, RequiresPhases: []string{"brainstorming"}}, "ADR-plan promotion requires"},
		{"bad bugfix selection", WorkflowMapping{Kind: WorkflowTask, PhaseEffect: PhaseStart, Phase: "investigation", RouteEffect: RouteSelectBugfix, Activity: "debugging"}, "bugfix selection requires"},
		{"bad investigation fallback", WorkflowMapping{Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "implementation", RouteEffect: RouteSelectInvestigationIfUnrouted, RequiresPhases: []string{"investigation"}}, "investigation fallback requires"},
		{"bad terminal target", WorkflowMapping{Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "implementation", TerminalEffect: TerminalArmCompletion, RequiresPhases: []string{"brainstorming"}}, "terminal effect requires"},
		{"completion lacks fallback", WorkflowMapping{Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "retrospective", TerminalEffect: TerminalArmCompletion, RequiresPhases: []string{"implementation-review"}}, "completion arming requires"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateWorkflowCombination(tc.mapping); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("validateWorkflowCombination() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestWorkflowMappingValidationCoversExplicitPlanSelection(t *testing.T) {
	skills := make(map[string]SkillSpec, len(Standard.Skills)+1)
	for name, spec := range Standard.Skills {
		skills[name] = spec
	}
	plan := WorkflowMapping{Kind: WorkflowChain, PhaseEffect: PhaseTransition, Phase: "planning", RouteEffect: RouteSelectPlan, RequiresPhases: []string{"brainstorming"}}
	skills["explicit-plan"] = SkillSpec{Workflow: &plan}
	if err := validateRouteCoverage(&Catalog{Skills: skills}); err != nil {
		t.Fatal(err)
	}
}

func TestWorkflowMappingValidationRejectsMissingMapping(t *testing.T) {
	if err := ValidateWorkflowMappings(&Catalog{Skills: map[string]SkillSpec{"missing": {}}}); err == nil {
		t.Fatal("missing workflow mapping unexpectedly passed")
	}
}

func TestWorkflowMappingValidationRejectsUncoveredRoute(t *testing.T) {
	cat := &Catalog{Skills: make(map[string]SkillSpec, len(Standard.Skills))}
	for name, spec := range Standard.Skills {
		mapping := *spec.Workflow
		mapping.RequiresPhases = append([]string(nil), mapping.RequiresPhases...)
		spec.Workflow = &mapping
		cat.Skills[name] = spec
	}
	direct := cat.Skills["executing-direct"]
	direct.Workflow.RouteEffect = RouteNone
	direct.Workflow.ImplementationMode = ""
	cat.Skills["executing-direct"] = direct
	if err := ValidateWorkflowMappings(cat); err == nil || !strings.Contains(err.Error(), "uncovered route \"direct\"") {
		t.Fatalf("ValidateWorkflowMappings() error = %v, want uncovered direct route", err)
	}
}

func containsRequirement(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
