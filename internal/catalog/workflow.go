package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/telemetry"
)

// ValidateWorkflowMappings verifies that the catalog is a closed, protocol-
// compatible lifecycle mapping. Every governed skill must carry one mapping.
func ValidateWorkflowMappings(cat *Catalog) error {
	if cat == nil {
		return errors.New("workflow catalog is nil")
	}
	vocabularies := protocolVocabularies()
	for name, spec := range cat.Skills {
		if spec.Workflow == nil {
			return fmt.Errorf("skill %q has no workflow mapping", name)
		}
		if err := validateWorkflowMapping(name, *spec.Workflow, vocabularies); err != nil {
			return err
		}
	}
	return validateRouteCoverage(cat)
}

// WorkflowMappingsForSkills returns only enabled mappings and rejects stale,
// duplicate, or unmapped skill names before a router can advertise them.
func WorkflowMappingsForSkills(cat *Catalog, enabled []string) (map[string]WorkflowMapping, error) {
	if cat == nil {
		return nil, errors.New("workflow catalog is nil")
	}
	vocabularies := protocolVocabularies()
	result := make(map[string]WorkflowMapping, len(enabled))
	for _, name := range enabled {
		if _, duplicate := result[name]; duplicate {
			return nil, fmt.Errorf("enabled workflow skill %q is duplicated", name)
		}
		spec, ok := cat.Skills[name]
		if !ok {
			return nil, fmt.Errorf("enabled workflow skill %q is stale", name)
		}
		if spec.Workflow == nil {
			return nil, fmt.Errorf("enabled skill %q has no workflow mapping", name)
		}
		if err := validateWorkflowMapping(name, *spec.Workflow, vocabularies); err != nil {
			return nil, err
		}
		result[name] = *spec.Workflow
	}
	return result, nil
}

func protocolVocabularies() map[string][]string {
	var contract struct {
		Vocabularies map[string][]string `json:"vocabularies"`
	}
	if err := json.Unmarshal(telemetry.DescriptorBytes(), &contract); err != nil { // coverage-ignore: embedded protocol descriptor is validated as JSON by telemetry initialization
		panic(fmt.Sprintf("decode telemetry protocol vocabularies: %v", err))
	}
	return contract.Vocabularies
}

func validateWorkflowMapping(name string, mapping WorkflowMapping, vocabularies map[string][]string) error {
	if !validWorkflowKind(mapping.Kind) {
		return workflowValueError(name, "kind", mapping.Kind)
	}
	if !validRouteEffect(mapping.RouteEffect) {
		return workflowValueError(name, "route effect", mapping.RouteEffect)
	}
	if !validTerminalEffect(mapping.TerminalEffect) {
		return workflowValueError(name, "terminal effect", mapping.TerminalEffect)
	}
	if mapping.EntryPhase != "" && !slices.Contains(vocabularies["phases"], mapping.EntryPhase) {
		return workflowValueError(name, "entry phase", mapping.EntryPhase)
	}
	if mapping.Activity != "" && !slices.Contains(vocabularies["activities"], mapping.Activity) {
		return workflowValueError(name, "activity", mapping.Activity)
	}
	if mapping.ImplementationMode != "" && !slices.Contains(vocabularies["activities"], mapping.ImplementationMode) {
		return workflowValueError(name, "implementation mode", mapping.ImplementationMode)
	}
	if err := validateWorkflowPhases(name, "entry predecessor", mapping.EntryPredecessors, vocabularies["phases"]); err != nil {
		return err
	}
	if err := validateWorkflowPhases(name, "continuation phase", mapping.ContinuationPhases, vocabularies["phases"]); err != nil {
		return err
	}
	if err := validateWorkflowCombination(mapping); err != nil {
		return fmt.Errorf("skill %q workflow: %w", name, err)
	}
	return nil
}

func validateWorkflowPhases(name, field string, phases, vocabulary []string) error {
	if phases == nil {
		return fmt.Errorf("skill %q workflow %ss are missing", name, field)
	}
	if !slices.IsSorted(phases) {
		return fmt.Errorf("skill %q workflow %ss are not sorted", name, field)
	}
	for i, phase := range phases {
		if !slices.Contains(vocabulary, phase) {
			return workflowValueError(name, field, phase)
		}
		if i > 0 && phases[i-1] == phase {
			return fmt.Errorf("skill %q workflow %ss contain duplicate %q", name, field, phase)
		}
	}
	return nil
}

func workflowValueError(name, field string, value any) error {
	return fmt.Errorf("skill %q workflow has unknown %s %q", name, field, value)
}

func validWorkflowKind(value WorkflowKind) bool {
	return value == WorkflowChain || value == WorkflowTask || value == WorkflowSupport
}

func validRouteEffect(value RouteEffect) bool {
	switch value {
	case RouteNone, RouteSelectDirect, RouteSelectADR, RouteSelectPlan, RouteSelectBugfix,
		RouteSelectInvestigationIfUnrouted, RoutePromoteADRPlan:
		return true
	default:
		return false
	}
}

func validTerminalEffect(value TerminalEffect) bool {
	return value == TerminalNone || value == TerminalArmCompletion
}

func validateRouteCoverage(cat *Catalog) error {
	covered := map[string]bool{}
	for _, spec := range cat.Skills {
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
			return fmt.Errorf("workflow catalog has uncovered route %q", route)
		}
	}
	return nil
}

func validateWorkflowCombination(mapping WorkflowMapping) error {
	if len(mapping.ContinuationPhases) == 0 {
		return errors.New("continuation phases cannot be empty")
	}
	switch mapping.Kind {
	case WorkflowSupport:
		if mapping.EntryPhase != "" || mapping.AllowEntryWithoutPhase || len(mapping.EntryPredecessors) != 0 {
			return errors.New("support mapping cannot enter a phase")
		}
		if mapping.Activity == "" {
			return errors.New("support mapping requires an activity")
		}
		if mapping.ImplementationMode != "" || mapping.RouteEffect != RouteNone || mapping.TerminalEffect != TerminalNone {
			return errors.New("support mapping may only continue with an activity")
		}
	case WorkflowTask:
		if mapping.EntryPhase == "" {
			return errors.New("task mapping requires an entry phase")
		}
		if !mapping.AllowEntryWithoutPhase || len(mapping.EntryPredecessors) != 0 {
			return errors.New("task mapping must allow entry without a phase and have no entry predecessors")
		}
		if !slices.Contains(mapping.ContinuationPhases, mapping.EntryPhase) {
			return errors.New("task continuation phases must include its entry phase")
		}
	case WorkflowChain:
		if mapping.EntryPhase == "" {
			return errors.New("chain mapping requires an entry phase")
		}
		if len(mapping.EntryPredecessors) == 0 {
			return errors.New("chain entry requires a predecessor phase")
		}
		if len(mapping.ContinuationPhases) != 1 || mapping.ContinuationPhases[0] != mapping.EntryPhase {
			return errors.New("chain entry phase is incompatible with continuation phases")
		}
	}
	if mapping.Activity != "" && mapping.Kind != WorkflowSupport && (mapping.Kind != WorkflowTask || mapping.EntryPhase != "investigation" || mapping.Activity != "debugging") {
		return errors.New("activity requires support continuation except debugging investigation entry")
	}
	if mapping.ImplementationMode != "" && (mapping.Kind != WorkflowChain || mapping.EntryPhase != "implementation") {
		return errors.New("implementation mode requires an implementation chain entry")
	}
	switch mapping.RouteEffect {
	case RouteNone:
	case RouteSelectDirect:
		if mapping.Kind != WorkflowChain || mapping.EntryPhase != "implementation" || mapping.ImplementationMode != "inline-execution" {
			return errors.New("direct selection requires an inline implementation chain entry")
		}
	case RouteSelectADR:
		if mapping.Kind != WorkflowChain || mapping.EntryPhase != "adr-authoring" {
			return errors.New("ADR selection requires an ADR-authoring chain entry")
		}
	case RouteSelectPlan:
		if mapping.Kind != WorkflowChain || mapping.EntryPhase != "planning" {
			return errors.New("plan selection requires a planning chain entry")
		}
	case RoutePromoteADRPlan:
		if mapping.Kind != WorkflowChain || mapping.EntryPhase != "planning" {
			return errors.New("ADR-plan promotion requires a planning chain entry")
		}
	case RouteSelectBugfix:
		if mapping.Kind != WorkflowTask || mapping.EntryPhase != "brainstorming" {
			return errors.New("bugfix selection requires a brainstorming task entry")
		}
	case RouteSelectInvestigationIfUnrouted:
		if mapping.Kind != WorkflowChain || mapping.EntryPhase != "retrospective" {
			return errors.New("investigation fallback requires a retrospective chain entry")
		}
	}
	if mapping.TerminalEffect != TerminalNone && (mapping.Kind != WorkflowChain || mapping.EntryPhase != "retrospective") {
		return errors.New("terminal effect requires a retrospective chain entry")
	}
	if mapping.TerminalEffect == TerminalArmCompletion && mapping.RouteEffect != RouteSelectInvestigationIfUnrouted {
		return errors.New("completion arming requires investigation fallback routing")
	}
	return nil
}
