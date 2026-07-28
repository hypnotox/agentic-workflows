package catalog

import (
	"errors"
	"fmt"
)

// ValidateWorkflowMappings verifies that every governed standard skill has a
// closed body kind. The router uses this metadata only to describe the fixed
// body it returns; it has no workflow-state authority.
func ValidateWorkflowMappings(cat *Catalog) error {
	if cat == nil {
		return errors.New("workflow catalog is nil")
	}
	for name, spec := range cat.Skills {
		if spec.Workflow == nil {
			return fmt.Errorf("skill %q has no workflow mapping", name)
		}
		if !validWorkflowKind(spec.Workflow.Kind) {
			return fmt.Errorf("skill %q has unknown workflow kind %q", name, spec.Workflow.Kind)
		}
	}
	return nil
}

// WorkflowMappingsForSkills returns enabled fixed-body kinds and rejects stale,
// duplicate, and unmapped names before a router can advertise them.
func WorkflowMappingsForSkills(cat *Catalog, enabled []string) (map[string]WorkflowMapping, error) {
	if cat == nil {
		return nil, errors.New("workflow catalog is nil")
	}
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
		if !validWorkflowKind(spec.Workflow.Kind) {
			return nil, fmt.Errorf("skill %q has unknown workflow kind %q", name, spec.Workflow.Kind)
		}
		result[name] = *spec.Workflow
	}
	return result, nil
}

func validWorkflowKind(value WorkflowKind) bool {
	return value == WorkflowChain || value == WorkflowTask || value == WorkflowSupport
}
