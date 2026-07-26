package catalog

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/templates"
)

type protocol21WorkflowMapping struct {
	Kind                   WorkflowKind
	EntryPhase             string
	AllowEntryWithoutPhase bool
	EntryPredecessors      []string
	ContinuationPhases     []string
	Activity               string
	ImplementationMode     string
	RouteEffect            RouteEffect
	TerminalEffect         TerminalEffect
}

// invariant: tooling/workflow-telemetry:effort-lifecycle-and-routes
func TestProtocol21StandardWorkflowMappingTableIsExact(t *testing.T) {
	all := []string{"adr-authoring", "adr-plan-resync", "adr-review", "brainstorming", "implementation", "implementation-review", "investigation", "plan-review", "planning", "retrospective"}
	want := map[string]protocol21WorkflowMapping{
		"brainstorming":               {Kind: WorkflowChain, EntryPhase: "brainstorming", AllowEntryWithoutPhase: true, EntryPredecessors: []string{"investigation"}, ContinuationPhases: []string{"brainstorming"}},
		"bugfix":                      {Kind: WorkflowTask, EntryPhase: "brainstorming", AllowEntryWithoutPhase: true, EntryPredecessors: []string{}, ContinuationPhases: []string{"brainstorming"}, RouteEffect: RouteSelectBugfix},
		"debugging":                   {Kind: WorkflowTask, EntryPhase: "investigation", AllowEntryWithoutPhase: true, EntryPredecessors: []string{}, ContinuationPhases: append([]string{}, all...), Activity: "debugging"},
		"exploring":                   {Kind: WorkflowSupport, EntryPredecessors: []string{}, ContinuationPhases: append([]string{}, all...), Activity: "exploration"},
		"tdd":                         {Kind: WorkflowSupport, EntryPredecessors: []string{}, ContinuationPhases: []string{"implementation"}, Activity: "tdd"},
		"refactor-coupling-audit":     {Kind: WorkflowSupport, EntryPredecessors: []string{}, ContinuationPhases: []string{"brainstorming"}, Activity: "refactor-coupling-audit"},
		"adr-lifecycle":               {Kind: WorkflowSupport, EntryPredecessors: []string{}, ContinuationPhases: append([]string{}, all...), Activity: "adr-lifecycle"},
		"roadmap-graduation":          {Kind: WorkflowSupport, EntryPredecessors: []string{}, ContinuationPhases: append([]string{}, all...), Activity: "roadmap-graduation"},
		"proposing-adr":               {Kind: WorkflowChain, EntryPhase: "adr-authoring", EntryPredecessors: []string{"brainstorming"}, ContinuationPhases: []string{"adr-authoring"}, RouteEffect: RouteSelectADR},
		"writing-plans":               {Kind: WorkflowChain, EntryPhase: "planning", EntryPredecessors: []string{"adr-review", "brainstorming"}, ContinuationPhases: []string{"planning"}, RouteEffect: RoutePromoteADRPlan},
		"reviewing-adr":               {Kind: WorkflowChain, EntryPhase: "adr-review", EntryPredecessors: []string{"adr-authoring"}, ContinuationPhases: []string{"adr-review"}},
		"reviewing-plan":              {Kind: WorkflowChain, EntryPhase: "plan-review", EntryPredecessors: []string{"planning"}, ContinuationPhases: []string{"plan-review"}},
		"reviewing-plan-resync":       {Kind: WorkflowChain, EntryPhase: "adr-plan-resync", EntryPredecessors: []string{"plan-review"}, ContinuationPhases: []string{"adr-plan-resync"}},
		"executing-direct":            {Kind: WorkflowChain, EntryPhase: "implementation", EntryPredecessors: []string{"brainstorming"}, ContinuationPhases: []string{"implementation"}, ImplementationMode: "inline-execution", RouteEffect: RouteSelectDirect},
		"executing-plans":             {Kind: WorkflowChain, EntryPhase: "implementation", EntryPredecessors: []string{"adr-plan-resync", "plan-review"}, ContinuationPhases: []string{"implementation"}, ImplementationMode: "inline-execution"},
		"subagent-driven-development": {Kind: WorkflowChain, EntryPhase: "implementation", EntryPredecessors: []string{"adr-plan-resync", "plan-review"}, ContinuationPhases: []string{"implementation"}, ImplementationMode: "subagent-driven-development"},
		"reviewing-impl":              {Kind: WorkflowChain, EntryPhase: "implementation-review", EntryPredecessors: []string{"implementation"}, ContinuationPhases: []string{"implementation-review"}},
		"retrospective":               {Kind: WorkflowChain, EntryPhase: "retrospective", EntryPredecessors: []string{"implementation-review", "investigation"}, ContinuationPhases: []string{"retrospective"}, RouteEffect: RouteSelectInvestigationIfUnrouted, TerminalEffect: TerminalArmCompletion},
	}
	got := make(map[string]protocol21WorkflowMapping, len(Standard.Skills))
	for name, spec := range Standard.Skills {
		got[name] = protocol21MappingShape(t, spec.Workflow)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("protocol 2.1 workflow mappings differ\n got: %#v\nwant: %#v", got, want)
	}
}

func protocol21MappingShape(t *testing.T, mapping *WorkflowMapping) protocol21WorkflowMapping {
	t.Helper()
	if mapping == nil {
		t.Fatal("workflow mapping is nil")
	}
	value := reflect.ValueOf(mapping).Elem()
	field := func(name string) reflect.Value {
		got := value.FieldByName(name)
		if !got.IsValid() {
			t.Fatalf("WorkflowMapping lacks protocol 2.1 field %s", name)
		}
		return got
	}
	if field("EntryPredecessors").IsNil() || field("ContinuationPhases").IsNil() {
		t.Fatal("protocol 2.1 workflow phase slices must be nonnil")
	}
	return protocol21WorkflowMapping{
		Kind: field("Kind").Interface().(WorkflowKind), EntryPhase: field("EntryPhase").String(), AllowEntryWithoutPhase: field("AllowEntryWithoutPhase").Bool(),
		EntryPredecessors: append([]string{}, field("EntryPredecessors").Interface().([]string)...), ContinuationPhases: append([]string{}, field("ContinuationPhases").Interface().([]string)...),
		Activity: field("Activity").String(), ImplementationMode: field("ImplementationMode").String(), RouteEffect: field("RouteEffect").Interface().(RouteEffect), TerminalEffect: field("TerminalEffect").Interface().(TerminalEffect),
	}
}

// invariant: tooling/workflow-telemetry:effort-lifecycle-and-routes
func TestProtocol21WorkflowValidationRejectsEntryAndContinuationDefects(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value any
		want  string
	}{
		{"missing entry phase", "EntryPhase", "", "entry phase"},
		{"incompatible entry phase", "EntryPhase", "planning", "entry phase"},
		{"missing entry predecessors", "EntryPredecessors", []string(nil), "entry predecessor"},
		{"unordered entry predecessors", "EntryPredecessors", []string{"planning", "brainstorming"}, "not sorted"},
		{"duplicate entry predecessor", "EntryPredecessors", []string{"brainstorming", "brainstorming"}, "duplicate"},
		{"unknown entry predecessor", "EntryPredecessors", []string{"raw"}, "unknown entry predecessor"},
		{"missing continuation phases", "ContinuationPhases", []string(nil), "continuation"},
		{"unordered continuation phases", "ContinuationPhases", []string{"planning", "brainstorming"}, "not sorted"},
		{"duplicate continuation phase", "ContinuationPhases", []string{"implementation", "implementation"}, "duplicate"},
		{"unknown continuation phase", "ContinuationPhases", []string{"raw"}, "unknown continuation"},
		{"incompatible continuation phase", "ContinuationPhases", []string{"planning"}, "continuation"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mapping := *Standard.Skills["executing-direct"].Workflow
			value := reflect.ValueOf(&mapping).Elem().FieldByName(tc.field)
			if !value.IsValid() {
				t.Fatalf("WorkflowMapping lacks protocol 2.1 field %s", tc.field)
			}
			value.Set(reflect.ValueOf(tc.value))
			err := ValidateWorkflowMappings(&Catalog{Skills: map[string]SkillSpec{"subject": {Workflow: &mapping}}})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateWorkflowMappings() error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestStandardWorkflowMappingsAreExactAndComplete(t *testing.T) {
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
	if len(got) != 2 || got["brainstorming"].EntryPhase != "brainstorming" {
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
		{"unknown route effect", func(m *WorkflowMapping) { m.RouteEffect = "raw" }, "unknown route effect"},
		{"unknown terminal effect", func(m *WorkflowMapping) { m.TerminalEffect = "raw" }, "unknown terminal effect"},
		{"unknown entry phase", func(m *WorkflowMapping) { m.EntryPhase = "raw" }, "unknown entry phase"},
		{"unknown activity", func(m *WorkflowMapping) { m.Activity = "raw" }, "unknown activity"},
		{"unknown implementation mode", func(m *WorkflowMapping) { m.ImplementationMode = "raw" }, "unknown implementation mode"},
		{"impossible terminal mapping", func(m *WorkflowMapping) { m.TerminalEffect = TerminalArmCompletion }, "terminal effect requires"},
		{"invalid route combination", func(m *WorkflowMapping) { m.RouteEffect = RouteSelectBugfix }, "bugfix selection requires"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mapping := *Standard.Skills["executing-direct"].Workflow
			tc.edit(&mapping)
			cat := &Catalog{Skills: map[string]SkillSpec{"subject": {Workflow: &mapping}}}
			err := ValidateWorkflowMappings(cat)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateWorkflowMappings() error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestWorkflowCombinationRejectsImpossibleShapes(t *testing.T) {
	validContinuation := []string{"brainstorming"}
	tests := []struct {
		name    string
		mapping WorkflowMapping
		want    string
	}{
		{"empty continuation", WorkflowMapping{Kind: WorkflowSupport, EntryPredecessors: []string{}, ContinuationPhases: []string{}, Activity: "tdd"}, "cannot be empty"},
		{"support enters", WorkflowMapping{Kind: WorkflowSupport, EntryPhase: "brainstorming", EntryPredecessors: []string{}, ContinuationPhases: validContinuation, Activity: "tdd"}, "cannot enter"},
		{"support lacks activity", WorkflowMapping{Kind: WorkflowSupport, EntryPredecessors: []string{}, ContinuationPhases: validContinuation}, "requires an activity"},
		{"support carries effect", WorkflowMapping{Kind: WorkflowSupport, EntryPredecessors: []string{}, ContinuationPhases: validContinuation, Activity: "tdd", RouteEffect: RouteSelectPlan}, "may only continue"},
		{"task lacks entry", WorkflowMapping{Kind: WorkflowTask, EntryPredecessors: []string{}, ContinuationPhases: validContinuation}, "requires an entry phase"},
		{"task cannot enter without phase", WorkflowMapping{Kind: WorkflowTask, EntryPhase: "brainstorming", EntryPredecessors: []string{}, ContinuationPhases: validContinuation}, "must allow entry"},
		{"task continuation excludes entry", WorkflowMapping{Kind: WorkflowTask, EntryPhase: "investigation", AllowEntryWithoutPhase: true, EntryPredecessors: []string{}, ContinuationPhases: validContinuation}, "must include"},
		{"chain lacks entry", WorkflowMapping{Kind: WorkflowChain, EntryPredecessors: []string{"brainstorming"}, ContinuationPhases: validContinuation}, "requires an entry phase"},
		{"chain lacks predecessor", WorkflowMapping{Kind: WorkflowChain, EntryPhase: "brainstorming", EntryPredecessors: []string{}, ContinuationPhases: validContinuation}, "requires a predecessor"},
		{"chain continues elsewhere", WorkflowMapping{Kind: WorkflowChain, EntryPhase: "planning", EntryPredecessors: []string{"brainstorming"}, ContinuationPhases: validContinuation}, "incompatible"},
		{"bad activity", WorkflowMapping{Kind: WorkflowChain, EntryPhase: "brainstorming", EntryPredecessors: []string{"investigation"}, ContinuationPhases: validContinuation, Activity: "tdd"}, "activity requires"},
		{"bad mode", WorkflowMapping{Kind: WorkflowChain, EntryPhase: "planning", EntryPredecessors: []string{"brainstorming"}, ContinuationPhases: []string{"planning"}, ImplementationMode: "inline-execution"}, "implementation mode requires"},
		{"bad direct", WorkflowMapping{Kind: WorkflowChain, EntryPhase: "implementation", EntryPredecessors: []string{"brainstorming"}, ContinuationPhases: []string{"implementation"}, RouteEffect: RouteSelectDirect}, "direct selection requires"},
		{"bad ADR", WorkflowMapping{Kind: WorkflowChain, EntryPhase: "planning", EntryPredecessors: []string{"brainstorming"}, ContinuationPhases: []string{"planning"}, RouteEffect: RouteSelectADR}, "ADR selection requires"},
		{"bad plan", WorkflowMapping{Kind: WorkflowChain, EntryPhase: "implementation", EntryPredecessors: []string{"brainstorming"}, ContinuationPhases: []string{"implementation"}, RouteEffect: RouteSelectPlan}, "plan selection requires"},
		{"bad promotion", WorkflowMapping{Kind: WorkflowChain, EntryPhase: "implementation", EntryPredecessors: []string{"brainstorming"}, ContinuationPhases: []string{"implementation"}, RouteEffect: RoutePromoteADRPlan}, "ADR-plan promotion requires"},
		{"bad bugfix", WorkflowMapping{Kind: WorkflowTask, EntryPhase: "investigation", AllowEntryWithoutPhase: true, EntryPredecessors: []string{}, ContinuationPhases: []string{"investigation"}, Activity: "debugging", RouteEffect: RouteSelectBugfix}, "bugfix selection requires"},
		{"bad fallback", WorkflowMapping{Kind: WorkflowChain, EntryPhase: "implementation", EntryPredecessors: []string{"investigation"}, ContinuationPhases: []string{"implementation"}, RouteEffect: RouteSelectInvestigationIfUnrouted}, "investigation fallback requires"},
		{"bad terminal", WorkflowMapping{Kind: WorkflowChain, EntryPhase: "implementation", EntryPredecessors: []string{"brainstorming"}, ContinuationPhases: []string{"implementation"}, TerminalEffect: TerminalArmCompletion}, "terminal effect requires"},
		{"completion lacks fallback", WorkflowMapping{Kind: WorkflowChain, EntryPhase: "retrospective", EntryPredecessors: []string{"investigation"}, ContinuationPhases: []string{"retrospective"}, TerminalEffect: TerminalArmCompletion}, "completion arming requires"},
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
	plan := WorkflowMapping{Kind: WorkflowChain, EntryPhase: "planning", EntryPredecessors: []string{"brainstorming"}, ContinuationPhases: []string{"planning"}, RouteEffect: RouteSelectPlan}
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
		mapping.EntryPredecessors = append([]string{}, mapping.EntryPredecessors...)
		mapping.ContinuationPhases = append([]string{}, mapping.ContinuationPhases...)
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
