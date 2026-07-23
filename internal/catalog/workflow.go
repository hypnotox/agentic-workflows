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
	if !validPhaseEffect(mapping.PhaseEffect) {
		return workflowValueError(name, "phase effect", mapping.PhaseEffect)
	}
	if !validRouteEffect(mapping.RouteEffect) {
		return workflowValueError(name, "route effect", mapping.RouteEffect)
	}
	if !validTerminalEffect(mapping.TerminalEffect) {
		return workflowValueError(name, "terminal effect", mapping.TerminalEffect)
	}
	if mapping.Phase != "" && !slices.Contains(vocabularies["phases"], mapping.Phase) {
		return workflowValueError(name, "phase", mapping.Phase)
	}
	if mapping.Activity != "" && !slices.Contains(vocabularies["activities"], mapping.Activity) {
		return workflowValueError(name, "activity", mapping.Activity)
	}
	if mapping.ImplementationMode != "" && !slices.Contains(vocabularies["activities"], mapping.ImplementationMode) {
		return workflowValueError(name, "implementation mode", mapping.ImplementationMode)
	}
	if !slices.IsSorted(mapping.RequiresPhases) {
		return fmt.Errorf("skill %q workflow required phases are not sorted", name)
	}
	for i, phase := range mapping.RequiresPhases {
		if !slices.Contains(vocabularies["phases"], phase) {
			return workflowValueError(name, "required phase", phase)
		}
		if i > 0 && mapping.RequiresPhases[i-1] == phase {
			return fmt.Errorf("skill %q workflow required phases contain duplicate %q", name, phase)
		}
	}
	if err := validateWorkflowCombination(mapping); err != nil {
		return fmt.Errorf("skill %q workflow: %w", name, err)
	}
	return nil
}

func workflowValueError(name, field string, value any) error {
	return fmt.Errorf("skill %q workflow has unknown %s %q", name, field, value)
}

func validWorkflowKind(value WorkflowKind) bool {
	return value == WorkflowChain || value == WorkflowTask || value == WorkflowSupport
}

func validPhaseEffect(value PhaseEffect) bool {
	return value == PhaseNone || value == PhaseStart || value == PhaseTransition || value == PhaseCurrent
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
	if mapping.PhaseEffect == PhaseNone {
		return errors.New("phase effect cannot be none")
	}
	if mapping.PhaseEffect == PhaseCurrent {
		if mapping.Phase != "" || mapping.ImplementationMode != "" || mapping.RouteEffect != RouteNone || mapping.TerminalEffect != TerminalNone {
			return errors.New("current-phase mapping may only record an activity and required phases")
		}
		if mapping.Activity == "" {
			return errors.New("current-phase mapping requires an activity")
		}
	}
	if mapping.PhaseEffect != PhaseCurrent && mapping.Phase == "" {
		return errors.New("phase-changing mapping requires a phase")
	}
	if mapping.Kind == WorkflowSupport && mapping.PhaseEffect != PhaseCurrent {
		return errors.New("support mapping must use the current phase")
	}
	if mapping.Kind == WorkflowChain && mapping.PhaseEffect == PhaseStart && mapping.Phase != "brainstorming" {
		return errors.New("chain start must enter brainstorming")
	}
	if mapping.Kind == WorkflowChain && mapping.PhaseEffect == PhaseTransition && len(mapping.RequiresPhases) == 0 {
		return errors.New("chain transition requires a predecessor phase")
	}
	if mapping.Kind == WorkflowTask && mapping.PhaseEffect != PhaseStart {
		return errors.New("task mapping must start a phase")
	}
	if mapping.PhaseEffect == PhaseStart && len(mapping.RequiresPhases) != 0 {
		return errors.New("phase start cannot require predecessor phases")
	}
	if mapping.Activity != "" && mapping.PhaseEffect != PhaseCurrent && (mapping.Kind != WorkflowTask || mapping.PhaseEffect != PhaseStart || mapping.Phase != "investigation" || mapping.Activity != "debugging") {
		return errors.New("activity requires the current phase except debugging investigation start")
	}
	if mapping.ImplementationMode != "" && (mapping.PhaseEffect != PhaseTransition || mapping.Phase != "implementation") {
		return errors.New("implementation mode requires an implementation transition")
	}
	if mapping.RouteEffect != RouteNone && mapping.PhaseEffect != PhaseTransition && mapping.RouteEffect != RouteSelectBugfix {
		return errors.New("route effect requires a transition except bugfix selection")
	}
	switch mapping.RouteEffect {
	case RouteNone:
	case RouteSelectDirect:
		if mapping.Kind != WorkflowChain || mapping.Phase != "implementation" || mapping.ImplementationMode != "inline-execution" {
			return errors.New("direct selection requires an inline implementation chain transition")
		}
	case RouteSelectADR:
		if mapping.Kind != WorkflowChain || mapping.Phase != "adr-authoring" {
			return errors.New("ADR selection requires an ADR-authoring chain transition")
		}
	case RouteSelectPlan:
		if mapping.Kind != WorkflowChain || mapping.Phase != "planning" {
			return errors.New("plan selection requires a planning chain transition")
		}
	case RoutePromoteADRPlan:
		if mapping.Kind != WorkflowChain || mapping.Phase != "planning" {
			return errors.New("ADR-plan promotion requires a planning chain transition")
		}
	case RouteSelectBugfix:
		if mapping.Kind != WorkflowTask || mapping.PhaseEffect != PhaseStart || mapping.Phase != "brainstorming" {
			return errors.New("bugfix selection requires a brainstorming task start")
		}
	case RouteSelectInvestigationIfUnrouted:
		if mapping.Kind != WorkflowChain || mapping.Phase != "retrospective" {
			return errors.New("investigation fallback requires a retrospective chain transition")
		}
	}
	if mapping.TerminalEffect != TerminalNone && (mapping.Kind != WorkflowChain || mapping.PhaseEffect != PhaseTransition || mapping.Phase != "retrospective") {
		return errors.New("terminal effect requires a retrospective chain transition")
	}
	if mapping.TerminalEffect == TerminalArmCompletion && mapping.RouteEffect != RouteSelectInvestigationIfUnrouted {
		return errors.New("completion arming requires investigation fallback routing")
	}
	return nil
}
