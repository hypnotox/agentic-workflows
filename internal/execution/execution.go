// Package execution selects closed operation steps, prepares their requirement closure once, and executes prepared actions in deterministic order.
package execution

import (
	"context"
	"fmt"
)

// RequirementID identifies a caller-declared prerequisite capability; the repository-check consumer uses it for config, Project, report, and index capabilities.
type RequirementID string

// StepID identifies a caller-declared operation step; the repository-check consumer uses it for drift, state, prose, and memory.
type StepID string

// Requirement declares a capability, its prerequisite capabilities, and its optional preparation; repository checks declare their operation-local capability preparation with it.
type Requirement struct {
	ID           RequirementID
	Dependencies []RequirementID
	Prepare      func(context.Context) error
}

// Step declares a selectable operation and its requirements after foundations are ready; repository checks use it for their selectable checks.
type Step struct {
	ID           StepID
	Requirements func(context.Context) ([]RequirementID, error)
}

// Action is a prepared operation action; repository checks bind typed output-producing closures as actions.
type Action func(context.Context) error

// BoundAction associates a selected step with its prepared action; the repository-check binder returns these associations.
type BoundAction struct {
	Step StepID
	Run  Action
}

// Binder creates actions for the selected steps after their requirements are prepared; repository checks use it to freeze prepared typed inputs into closures.
type Binder func([]StepID) ([]BoundAction, error)

// System is a closed caller-declared set of requirements, steps, foundations, and action binding; repository checks compose one per operation.
type System struct {
	Requirements []Requirement
	Steps        []Step
	Foundations  []RequirementID
	Bind         Binder
}

// FailurePolicy controls whether execution stops after an action failure; repository-check children stop and its aggregate continues.
type FailurePolicy uint8

const (
	// StopOnFailure leaves later actions unattempted after the first action failure.
	StopOnFailure FailurePolicy = iota
	// ContinueOnFailure attempts every selected action despite action failures.
	ContinueOnFailure
)

// Outcome records the identity and result of one attempted action; the repository-check adapter uses it for first-error routing.
type Outcome struct {
	Step StepID
	Err  error
}

// Prepared is a fully prepared and bound execution ready to run; the repository-check adapter runs it after capability readiness.
type Prepared struct {
	steps   []StepID
	actions []BoundAction
}

type definitionErrorKind string

const (
	definitionDuplicateRequirement definitionErrorKind = "duplicate requirement"
	definitionDuplicateStep        definitionErrorKind = "duplicate step"
	definitionUnknownFoundation    definitionErrorKind = "unknown foundation"
	definitionUnknownDependency    definitionErrorKind = "unknown dependency"
	definitionDependencyCycle      definitionErrorKind = "dependency cycle"
	definitionUnknownRequestedStep definitionErrorKind = "unknown requested step"
	definitionUnknownResolved      definitionErrorKind = "unknown resolved requirement"
	definitionInvalidBinding       definitionErrorKind = "invalid binding"
)

// definitionError describes an invalid closed definition without exposing a caller protocol.
type definitionError struct {
	kind                  definitionErrorKind
	step                  StepID
	requirement           RequirementID
	referencedRequirement RequirementID
	referencedStep        StepID
}

func (e *definitionError) Error() string {
	switch e.kind {
	case definitionDuplicateRequirement:
		return fmt.Sprintf("duplicate requirement %q", e.requirement)
	case definitionDuplicateStep:
		return fmt.Sprintf("duplicate step %q", e.step)
	case definitionUnknownFoundation:
		return fmt.Sprintf("unknown foundation requirement %q", e.referencedRequirement)
	case definitionUnknownDependency:
		return fmt.Sprintf("requirement %q depends on unknown requirement %q", e.requirement, e.referencedRequirement)
	case definitionDependencyCycle:
		return fmt.Sprintf("requirement dependency cycle at %q", e.requirement)
	case definitionUnknownRequestedStep:
		return fmt.Sprintf("unknown requested step %q", e.referencedStep)
	case definitionUnknownResolved:
		return fmt.Sprintf("step %q resolved unknown requirement %q", e.step, e.referencedRequirement)
	case definitionInvalidBinding:
		return fmt.Sprintf("invalid binding for selected step %q and bound step %q", e.step, e.referencedStep)
	default:
		return "invalid execution definition"
	}
}

// Prepare validates a closed system, prepares its selected requirement closure, and binds selected actions for the repository-check adapter or another caller.
func Prepare(ctx context.Context, system System, requested []StepID) (*Prepared, error) {
	requirements, steps, err := validateSystem(system)
	if err != nil {
		return nil, err
	}
	selected, err := selectSteps(system.Steps, steps, requested)
	if err != nil {
		return nil, err
	}

	preparedRequirements := make(map[RequirementID]bool, len(requirements))
	foundations := closure(system.Foundations, requirements)
	if err := prepareRequirements(ctx, foundations, system.Requirements, requirements, preparedRequirements); err != nil {
		return nil, err
	}

	resolved := make([]RequirementID, 0)
	for _, step := range selected {
		if step.Requirements == nil {
			continue
		}
		ids, resolveErr := step.Requirements(ctx)
		if resolveErr != nil {
			return nil, fmt.Errorf("resolve requirements for step %q: %w", step.ID, resolveErr)
		}
		for _, id := range ids {
			if _, ok := requirements[id]; !ok {
				return nil, &definitionError{kind: definitionUnknownResolved, step: step.ID, referencedRequirement: id}
			}
			resolved = append(resolved, id)
		}
	}
	if err := prepareRequirements(ctx, closure(resolved, requirements), system.Requirements, requirements, preparedRequirements); err != nil {
		return nil, err
	}
	if system.Bind == nil {
		return nil, &definitionError{kind: definitionInvalidBinding}
	}
	selectedIDs := stepIDs(selected)
	actions, bindErr := system.Bind(selectedIDs)
	if bindErr != nil {
		return nil, fmt.Errorf("bind selected actions: %w", bindErr)
	}
	if err := validateBinding(selectedIDs, actions); err != nil {
		return nil, err
	}
	return &Prepared{steps: selectedIDs, actions: actions}, nil
}

// Run executes bound actions under policy and returns outcomes only for attempted actions; the repository-check adapter maps those outcomes to command behavior.
func (p *Prepared) Run(ctx context.Context, policy FailurePolicy) ([]Outcome, error) {
	if policy != StopOnFailure && policy != ContinueOnFailure {
		return nil, fmt.Errorf("unsupported failure policy %d", policy)
	}
	outcomes := make([]Outcome, 0, len(p.actions))
	for _, action := range p.actions {
		if err := ctx.Err(); err != nil {
			return outcomes, err
		}
		err := action.Run(ctx)
		if err != nil {
			err = fmt.Errorf("execute step %q: %w", action.Step, err)
		}
		outcomes = append(outcomes, Outcome{Step: action.Step, Err: err})
		if cancelErr := ctx.Err(); cancelErr != nil {
			return outcomes, cancelErr
		}
		if err != nil && policy == StopOnFailure {
			return outcomes, nil
		}
	}
	return outcomes, nil
}

func validateSystem(system System) (map[RequirementID]Requirement, map[StepID]Step, error) {
	requirements := make(map[RequirementID]Requirement, len(system.Requirements))
	for _, requirement := range system.Requirements {
		if _, exists := requirements[requirement.ID]; exists {
			return nil, nil, &definitionError{kind: definitionDuplicateRequirement, requirement: requirement.ID}
		}
		requirements[requirement.ID] = requirement
	}
	steps := make(map[StepID]Step, len(system.Steps))
	for _, step := range system.Steps {
		if _, exists := steps[step.ID]; exists {
			return nil, nil, &definitionError{kind: definitionDuplicateStep, step: step.ID}
		}
		steps[step.ID] = step
	}
	for _, foundation := range system.Foundations {
		if _, exists := requirements[foundation]; !exists {
			return nil, nil, &definitionError{kind: definitionUnknownFoundation, referencedRequirement: foundation}
		}
	}
	for _, requirement := range system.Requirements {
		for _, dependency := range requirement.Dependencies {
			if _, exists := requirements[dependency]; !exists {
				return nil, nil, &definitionError{kind: definitionUnknownDependency, requirement: requirement.ID, referencedRequirement: dependency}
			}
		}
	}
	if ordered, ok := topological(system.Requirements, requirementIDs(system.Requirements)); !ok {
		return nil, nil, &definitionError{kind: definitionDependencyCycle, requirement: firstUnordered(system.Requirements, ordered)}
	}
	return requirements, steps, nil
}

func selectSteps(declared []Step, known map[StepID]Step, requested []StepID) ([]Step, error) {
	wanted := make(map[StepID]bool, len(requested))
	for _, id := range requested {
		if _, exists := known[id]; !exists {
			return nil, &definitionError{kind: definitionUnknownRequestedStep, referencedStep: id}
		}
		wanted[id] = true
	}
	selected := make([]Step, 0, len(wanted))
	for _, step := range declared {
		if wanted[step.ID] {
			selected = append(selected, step)
		}
	}
	return selected, nil
}

func closure(initial []RequirementID, requirements map[RequirementID]Requirement) []RequirementID {
	needed := make(map[RequirementID]bool)
	var visit func(RequirementID)
	visit = func(id RequirementID) {
		if needed[id] {
			return
		}
		needed[id] = true
		for _, dependency := range requirements[id].Dependencies {
			visit(dependency)
		}
	}
	for _, id := range initial {
		visit(id)
	}
	ids := make([]RequirementID, 0, len(needed))
	for id := range needed {
		ids = append(ids, id)
	}
	return ids
}

func prepareRequirements(ctx context.Context, ids []RequirementID, declared []Requirement, requirements map[RequirementID]Requirement, prepared map[RequirementID]bool) error {
	ordered, _ := topological(declared, ids)
	for _, id := range ordered {
		if prepared[id] {
			continue
		}
		prepared[id] = true
		if prepare := requirements[id].Prepare; prepare != nil {
			if err := prepare(ctx); err != nil {
				return fmt.Errorf("prepare requirement %q: %w", id, err)
			}
		}
	}
	return nil
}

func topological(declared []Requirement, ids []RequirementID) ([]RequirementID, bool) {
	wanted := make(map[RequirementID]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	indegree := make(map[RequirementID]int, len(wanted))
	for _, requirement := range declared {
		if !wanted[requirement.ID] {
			continue
		}
		for _, dependency := range requirement.Dependencies {
			if wanted[dependency] {
				indegree[requirement.ID]++
			}
		}
	}
	ordered := make([]RequirementID, 0, len(wanted))
	used := make(map[RequirementID]bool, len(wanted))
	for len(ordered) < len(wanted) {
		var next RequirementID
		found := false
		for _, requirement := range declared {
			if wanted[requirement.ID] && !used[requirement.ID] && indegree[requirement.ID] == 0 {
				next, found = requirement.ID, true
				break
			}
		}
		if !found {
			return ordered, false
		}
		used[next] = true
		ordered = append(ordered, next)
		for _, requirement := range declared {
			if !wanted[requirement.ID] || used[requirement.ID] {
				continue
			}
			for _, dependency := range requirement.Dependencies {
				if dependency == next {
					indegree[requirement.ID]--
				}
			}
		}
	}
	return ordered, true
}

func requirementIDs(requirements []Requirement) []RequirementID {
	ids := make([]RequirementID, len(requirements))
	for i, requirement := range requirements {
		ids[i] = requirement.ID
	}
	return ids
}
func stepIDs(steps []Step) []StepID {
	ids := make([]StepID, len(steps))
	for i, step := range steps {
		ids[i] = step.ID
	}
	return ids
}
func firstUnordered(requirements []Requirement, ordered []RequirementID) RequirementID {
	seen := make(map[RequirementID]bool, len(ordered))
	for _, id := range ordered {
		seen[id] = true
	}
	for _, requirement := range requirements {
		if !seen[requirement.ID] {
			return requirement.ID
		}
	}
	return ""
}

func validateBinding(selected []StepID, actions []BoundAction) error {
	if len(actions) != len(selected) {
		return &definitionError{kind: definitionInvalidBinding}
	}
	for i, action := range actions {
		if action.Step != selected[i] || action.Run == nil {
			return &definitionError{kind: definitionInvalidBinding, step: selected[i], referencedStep: action.Step}
		}
	}
	return nil
}
