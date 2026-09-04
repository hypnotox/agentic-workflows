package application

import (
	"errors"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/effort"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

type presentedError struct {
	cause         error
	mapDiagnostic func() (presentation.Diagnostic, error)
}

func (e *presentedError) Error() string { return e.cause.Error() }
func (e *presentedError) Unwrap() error { return e.cause }
func (e *presentedError) Diagnostic() (presentation.Diagnostic, error) {
	return e.mapDiagnostic()
}

func presentError(err error) error {
	// Prefer the outer application outcome before inspecting its nested worktree mechanism cause.
	var creation *CreationError
	if errors.As(err, &creation) {
		return &presentedError{cause: err, mapDiagnostic: func() (presentation.Diagnostic, error) {
			return creationDiagnostic(creation)
		}}
	}
	var refusal *worktree.RefusalError
	if errors.As(err, &refusal) {
		return &presentedError{cause: err, mapDiagnostic: func() (presentation.Diagnostic, error) {
			return worktreeDiagnostic(refusal)
		}}
	}
	if info, ok := effort.DiagnosticFor(err); ok {
		return &presentedError{cause: err, mapDiagnostic: func() (presentation.Diagnostic, error) {
			return effortDiagnostic(info)
		}}
	}
	return err
}

func effortDiagnostic(info effort.DiagnosticInfo) (presentation.Diagnostic, error) {
	changed := make([]presentation.Field, 0, len(info.Changed))
	for _, fact := range info.Changed {
		value, err := diagnosticValue(fact.Value, fact.Literal)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		field, err := presentation.NewField(fact.Label, value)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		changed = append(changed, field)
	}
	steps, err := recoverySteps(info.Actions)
	if err != nil {
		return presentation.Diagnostic{}, err
	}
	return presentation.Diagnostic{Condition: info.Condition, State: info.State, Cause: info.Cause, Changed: changed, Steps: steps}, nil
}

func diagnosticValue(text string, literal bool) (presentation.Value, error) {
	if literal {
		return presentation.Literal(text)
	}
	return presentation.Prose(text)
}

func recoverySteps(actions []effort.RecoveryAction) ([]presentation.Value, error) {
	steps := make([]presentation.Value, 0, len(actions))
	for _, action := range actions {
		step, err := presentation.Literal(action.Text)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, nil
}

func detailDocument(record effort.Record) (presentation.Document, error) {
	fields, err := effortFields(record, "slug")
	if err != nil {
		return presentation.Document{}, err
	}
	detail := presentation.Detail{Fields: fields}
	return detail.Document()
}

func effortFields(record effort.Record, slugLabel string) ([]presentation.Field, error) {
	fields := make([]presentation.Field, 0, 3)
	for _, fact := range []struct {
		label   string
		value   string
		literal bool
	}{{slugLabel, record.Slug, false}, {"title", record.Title, false}, {"memory", record.MemoryPath, true}} {
		value, err := diagnosticValue(fact.value, fact.literal)
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

func listDocument(records []effort.Record) (presentation.Document, error) {
	if len(records) == 0 {
		value, err := presentation.Prose("none")
		if err != nil {
			return presentation.Document{}, err
		}
		field, err := presentation.NewField("efforts", value)
		if err != nil {
			return presentation.Document{}, err
		}
		return presentation.NewDocument(field)
	}
	values := make([]presentation.Value, 0, len(records))
	for _, record := range records {
		value, err := presentation.Prose(record.Slug + ": " + record.Title)
		if err != nil {
			return presentation.Document{}, err
		}
		values = append(values, value)
	}
	list, err := presentation.NewList("efforts", values...)
	if err != nil {
		return presentation.Document{}, err
	}
	section, err := presentation.NewSection("effort list", list)
	if err != nil {
		return presentation.Document{}, err
	}
	return presentation.NewDocument(section)
}

func newDocument(record effort.Record, result worktree.Result) (presentation.Document, error) {
	mutation, err := worktreeMutation(result)
	if err != nil {
		return presentation.Document{}, err
	}
	identity, err := effortFields(record, "effort")
	if err != nil {
		return presentation.Document{}, err
	}
	mutation.Identity = append(identity, mutation.Identity...)
	return mutation.Document()
}

func finishDocument(result effort.FinishResult, slug string) (presentation.Document, error) {
	effortValue, err := presentation.Prose(slug)
	if err != nil {
		return presentation.Document{}, err
	}
	effortField, err := presentation.NewField("effort", effortValue)
	if err != nil {
		return presentation.Document{}, err
	}
	archiveValue, err := presentation.Literal(result.ArchivePath)
	if err != nil {
		return presentation.Document{}, err
	}
	archiveField, err := presentation.NewField("archive", archiveValue)
	if err != nil {
		return presentation.Document{}, err
	}
	archived, err := presentation.Prose("archived resident")
	if err != nil {
		return presentation.Document{}, err
	}
	next, err := presentation.Prose("continue without this finished effort; delete the local archive manually when it is no longer useful")
	if err != nil {
		return presentation.Document{}, err
	}
	return (presentation.Mutation{
		Status:      "archived",
		Identity:    []presentation.Field{effortField, archiveField},
		Changes:     []presentation.MutationChange{{Label: "completed", Values: []presentation.Value{archived}}},
		NextActions: []presentation.Value{next},
	}).Document()
}

func worktreeDocument(result worktree.Result, err error) (presentation.Document, error) {
	if err != nil {
		return presentation.Document{}, err
	}
	mutation, err := worktreeMutation(result)
	if err != nil {
		return presentation.Document{}, err
	}
	return mutation.Document()
}

func worktreeMutation(result worktree.Result) (presentation.Mutation, error) {
	identity := []presentation.Field{}
	for _, fact := range []struct {
		label   string
		value   string
		literal bool
	}{{"worktree", result.Path, true}, {"branch", result.Branch, false}} {
		if fact.value == "" {
			continue
		}
		value, err := diagnosticValue(fact.value, fact.literal)
		if err != nil {
			return presentation.Mutation{}, err
		}
		field, err := presentation.NewField(fact.label, value)
		if err != nil {
			return presentation.Mutation{}, err
		}
		identity = append(identity, field)
	}
	changes := []presentation.MutationChange{}
	if result.ChangedTopology {
		values := []presentation.Value{}
		for _, axis := range topologyAxes(result.Topology) {
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
	next, err := presentation.Literal(result.NextAction)
	if err != nil {
		return presentation.Mutation{}, err
	}
	return presentation.Mutation{Status: result.Condition, Identity: identity, Changes: changes, NextActions: []presentation.Value{next}}, nil
}

type topologyAxis struct {
	label   string
	changed bool
}

func topologyAxes(topology worktree.TopologyEffects) []topologyAxis {
	return []topologyAxis{
		{"managed path", topology.ManagedPath},
		{"git registration", topology.GitRegistration},
		{"branch", topology.Branch},
		{"receiving HEAD", topology.ReceivingHEAD},
	}
}

func worktreeDiagnostic(refusal *worktree.RefusalError) (presentation.Diagnostic, error) {
	changed, err := topologyDiagnosticFields(refusal.Topology, refusal.ChangedTopology)
	if err != nil {
		return presentation.Diagnostic{}, err
	}
	changed, err = appendManagedFacts(changed, refusal.ManagedPath, refusal.ManagedBranch)
	if err != nil {
		return presentation.Diagnostic{}, err
	}
	if len(refusal.NextActions) == 0 {
		return presentation.Diagnostic{}, errors.New("worktree refusal has no modeled recovery actions")
	}
	steps := make([]presentation.Value, 0, len(refusal.NextActions))
	for _, action := range refusal.NextActions {
		step, err := presentation.Literal(action)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		steps = append(steps, step)
	}
	return presentation.Diagnostic{Condition: refusal.Condition, State: refusal.Category, Changed: changed, Cause: mechanismCauseText(refusal), Steps: steps}, nil
}

func creationDiagnostic(creation *CreationError) (presentation.Diagnostic, error) {
	value, err := presentation.Prose(yesNo(creation.ChangedEffort))
	if err != nil {
		return presentation.Diagnostic{}, err
	}
	field, err := presentation.NewField("effort resident", value)
	if err != nil {
		return presentation.Diagnostic{}, err
	}
	changed := []presentation.Field{field}
	topology, err := topologyDiagnosticFields(creation.Topology, creation.ChangedTopology)
	if err != nil {
		return presentation.Diagnostic{}, err
	}
	changed = append(changed, topology...)
	changed, err = appendManagedFacts(changed, creation.ManagedPath, creation.ManagedBranch)
	if err != nil {
		return presentation.Diagnostic{}, err
	}
	steps := make([]presentation.Value, 0, len(creation.Steps))
	for _, text := range creation.Steps {
		step, err := presentation.Literal(text)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		steps = append(steps, step)
	}
	return presentation.Diagnostic{Condition: creation.Condition, State: "operation", Changed: changed, Cause: mechanismCauseText(creation), Steps: steps}, nil
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

func topologyDiagnosticFields(topology worktree.TopologyEffects, legacy bool) ([]presentation.Field, error) {
	fields := make([]presentation.Field, 0, 5)
	for _, fact := range topologyAxes(topology) {
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
		return mechanismCauses(creation.Cause)
	}
	var refusal *worktree.RefusalError
	if errors.As(err, &refusal) {
		return mechanismCauses(refusal.Err)
	}
	if errors.Is(err, effort.ErrManagedTopologyPresent) {
		return nil
	}
	return []error{err}
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
