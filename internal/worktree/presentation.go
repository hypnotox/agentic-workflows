package worktree

import (
	"errors"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/effort"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// Diagnostic maps a typed refusal into the common readable diagnostic.
func (e *RefusalError) Diagnostic() (presentation.Diagnostic, error) {
	changed, err := topologyDiagnosticFields(e.Topology, e.ChangedTopology)
	if err != nil {
		return presentation.Diagnostic{}, err
	}
	changed, err = appendManagedFacts(changed, e.ManagedPath, e.ManagedBranch)
	if err != nil {
		return presentation.Diagnostic{}, err
	}
	if len(e.NextActions) == 0 {
		return presentation.Diagnostic{}, errors.New("worktree refusal has no modeled recovery actions")
	}
	steps := make([]presentation.Value, 0, len(e.NextActions))
	for _, action := range e.NextActions {
		step, err := presentation.Literal(action)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		steps = append(steps, step)
	}
	return presentation.Diagnostic{
		Condition: e.Condition, State: e.Category, Changed: changed,
		Cause: mechanismCauseText(e), Steps: steps,
	}, nil
}

// Diagnostic maps failed creation while retaining both mechanism identities.
func (e *CreationError) Diagnostic() (presentation.Diagnostic, error) {
	changed := make([]presentation.Field, 0, 6)
	for _, fact := range []struct {
		label string
		value bool
	}{{"effort resident", e.ChangedEffort}} {
		value, err := presentation.Prose(yesNo(fact.value))
		if err != nil { // coverage-ignore: yesNo always returns a nonempty prose value
			return presentation.Diagnostic{}, err
		}
		field, err := presentation.NewField(fact.label, value)
		if err != nil { // coverage-ignore: fixed grammar-valid changed-axis labels always validate
			return presentation.Diagnostic{}, err
		}
		changed = append(changed, field)
	}
	topology, err := topologyDiagnosticFields(e.Topology, e.ChangedTopology)
	if err != nil {
		return presentation.Diagnostic{}, err
	}
	changed = append(changed, topology...)
	changed, err = appendManagedFacts(changed, e.ManagedPath, e.ManagedBranch)
	if err != nil {
		return presentation.Diagnostic{}, err
	}
	steps := make([]presentation.Value, 0, len(e.Steps))
	for _, text := range e.Steps {
		step, err := presentation.Literal(text)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		steps = append(steps, step)
	}
	return presentation.Diagnostic{Condition: e.Condition, State: "operation", Changed: changed, Cause: mechanismCauseText(e), Steps: steps}, nil
}

func appendManagedFacts(fields []presentation.Field, path, branch string) ([]presentation.Field, error) {
	for _, fact := range []struct{ label, value string }{{"managed path", path}, {"managed branch", branch}} {
		if fact.value == "" {
			continue
		}
		value, err := presentation.Literal(fact.value)
		if err != nil {
			return nil, err
		}
		field, err := presentation.NewField(fact.label, value)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func topologyDiagnosticFields(topology TopologyEffects, legacy bool) ([]presentation.Field, error) {
	facts := []struct {
		label   string
		changed bool
	}{{"managed path", topology.ManagedPath}, {"git registration", topology.GitRegistration}, {"branch", topology.Branch}, {"receiving HEAD", topology.ReceivingHEAD}, {"topology uncertainty", topology.Uncertain}}
	fields := make([]presentation.Field, 0, len(facts))
	for _, fact := range facts {
		if !fact.changed {
			continue
		}
		value, err := presentation.Prose("yes")
		if err != nil {
			return nil, err
		}
		field, err := presentation.NewField(fact.label, value)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}
	if len(fields) == 0 {
		value, err := presentation.Prose(yesNo(legacy))
		if err != nil {
			return nil, err
		}
		field, err := presentation.NewField("managed topology", value)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}
	return fields, nil
}

// mechanismCauseText exposes only failed-call mechanisms, never a typed
// outcome's legacy Error envelope or its modeled recovery prose.
func mechanismCauseText(err error) string {
	causes := mechanismCauses(err)
	texts := make([]string, 0, len(causes))
	for _, cause := range causes {
		if cause != nil {
			texts = append(texts, cause.Error())
		}
	}
	return strings.Join(texts, " | ")
}

func mechanismCauses(err error) []error {
	if err == nil {
		return nil
	}
	var creation *CreationError
	if errors.As(err, &creation) {
		return append(mechanismCauses(creation.Cause), mechanismCauses(creation.RollbackCause)...)
	}
	var refusal *RefusalError
	if errors.As(err, &refusal) {
		return mechanismCauses(refusal.Err)
	}
	var partial *effort.PartialFinishError
	if errors.As(err, &partial) {
		return mechanismCauses(partial.Cause)
	}
	if errors.Is(err, effort.ErrManagedTopologyPresent) {
		return nil
	}
	return []error{err}
}

// Mutation maps managed-topology facts into the common readable mutation.
func (r Result) Mutation() (presentation.Mutation, error) {
	identity := []presentation.Field{}
	for _, fact := range []struct {
		label   string
		value   string
		literal bool
	}{{"worktree", r.Path, true}, {"branch", r.Branch, false}} {
		if fact.value == "" {
			continue
		}
		var value presentation.Value
		var err error
		if fact.literal {
			value, err = presentation.Literal(fact.value)
		} else {
			value, err = presentation.Prose(fact.value)
		}
		if err != nil {
			return presentation.Mutation{}, err
		}
		field, err := presentation.NewField(fact.label, value)
		if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
			return presentation.Mutation{}, err
		}
		identity = append(identity, field)
	}
	changes := []presentation.MutationChange{}
	if r.ChangedTopology {
		axes := []struct {
			label   string
			changed bool
		}{{"managed path", r.Topology.ManagedPath}, {"git registration", r.Topology.GitRegistration}, {"branch", r.Topology.Branch}, {"receiving HEAD", r.Topology.ReceivingHEAD}, {"topology uncertainty", r.Topology.Uncertain}}
		values := []presentation.Value{}
		for _, axis := range axes {
			if !axis.changed {
				continue
			}
			value, err := presentation.Prose(axis.label)
			if err != nil {
				return presentation.Mutation{}, err
			}
			values = append(values, value)
		}
		if len(values) == 0 {
			value, err := presentation.Prose("managed topology")
			if err != nil {
				return presentation.Mutation{}, err
			}
			values = append(values, value)
		}
		changes = append(changes, presentation.MutationChange{Label: "completed", Values: values})
	}
	next, err := presentation.Literal(r.NextAction)
	if err != nil {
		return presentation.Mutation{}, err
	}
	return presentation.Mutation{Status: r.Condition, Identity: identity, Changes: changes, NextActions: []presentation.Value{next}}, nil
}
