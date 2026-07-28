package catalog

import (
	"errors"
	"fmt"
)

// ValidateWorkflowProfiles verifies complete skill selection metadata. Profile
// neighbors are advisory: they must exist, be distinct, and cannot self-reference.
func ValidateWorkflowProfiles(cat *Catalog) error {
	if cat == nil {
		return errors.New("workflow catalog is nil")
	}
	for name, spec := range cat.Skills {
		p := spec.Profile
		if !validWorkflowKind(p.Kind) {
			return fmt.Errorf("skill %q has unknown workflow kind %q", name, p.Kind)
		}
		if p.Purpose == "" || p.Trigger == "" {
			return fmt.Errorf("skill %q has incomplete workflow profile", name)
		}
		for _, neighbors := range [][]string{p.UsuallyFollows, p.CommonFollowUps} {
			seen := map[string]bool{}
			for _, neighbor := range neighbors {
				if neighbor == name {
					return fmt.Errorf("skill %q names itself as a workflow neighbor", name)
				}
				if _, ok := cat.Skills[neighbor]; !ok {
					return fmt.Errorf("skill %q names unknown workflow neighbor %q", name, neighbor)
				}
				if seen[neighbor] {
					return fmt.Errorf("skill %q duplicates workflow neighbor %q", name, neighbor)
				}
				seen[neighbor] = true
			}
		}
	}
	return nil
}

func validWorkflowKind(value WorkflowKind) bool {
	return value == WorkflowChain || value == WorkflowTask || value == WorkflowSupport
}
