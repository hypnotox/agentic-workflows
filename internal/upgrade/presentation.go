package upgrade

import "github.com/hypnotox/agentic-workflows/internal/presentation"

const journalLabel = "journal"

// CompletedMutation maps a successful final upgrade into its terminal
// presentation result.
func (o Outcome) CompletedMutation() (presentation.Mutation, error) {
	return o.mutation("upgrade completed")
}

// RecoveredMutation maps a successful journal recovery into its terminal
// presentation result.
func (o Outcome) RecoveredMutation() (presentation.Mutation, error) {
	return o.mutation("upgrade recovered")
}

func (o Outcome) mutation(status string) (presentation.Mutation, error) {
	values, err := journalValues(o.Evidence)
	if err != nil { // coverage-ignore: evidence formatting always includes the fixed nonempty separator
		return presentation.Mutation{}, err
	}
	mutation := presentation.Mutation{Status: status}
	if len(values) > 0 {
		mutation.Changes = []presentation.MutationChange{{Label: journalLabel, Values: values}}
	}
	return mutation, nil
}

// FailureDiagnostic maps a failed journal operation into an actionable
// diagnostic using only axes still changed at return.
func (o Outcome) FailureDiagnostic(condition string, cause error) (presentation.Diagnostic, error) {
	changed, err := journalFields(o.terminalChanged())
	if err != nil { // coverage-ignore: terminal evidence formatting always includes the fixed nonempty separator
		return presentation.Diagnostic{}, err
	}
	steps, err := upgradeRemedies(len(changed) > 0)
	if err != nil { // coverage-ignore: closed remedy literals are presentation-valid
		return presentation.Diagnostic{}, err
	}
	return presentation.Diagnostic{
		Condition: condition,
		State:     "operation",
		Changed:   changed,
		Cause:     cause.Error(),
		Steps:     steps,
	}, nil
}

func (o Outcome) terminalChanged() []Evidence {
	// Nil preserves source compatibility for callers that construct the initial
	// Outcome shape directly. Transaction owners always provide Changed,
	// including an explicit empty set after a complete rollback.
	if o.Changed == nil {
		return o.Evidence
	}
	return o.Changed
}

func journalValues(evidence []Evidence) ([]presentation.Value, error) {
	values := make([]presentation.Value, 0, len(evidence))
	for _, fact := range evidence {
		value, err := presentation.Prose(fact.Action + ": " + fact.Path)
		if err != nil { // coverage-ignore: the fixed separator makes every evidence fact nonempty
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func journalFields(evidence []Evidence) ([]presentation.Field, error) {
	values, err := journalValues(evidence)
	if err != nil { // coverage-ignore: evidence formatting always includes the fixed nonempty separator
		return nil, err
	}
	fields := make([]presentation.Field, 0, len(values))
	for _, value := range values {
		field, err := presentation.NewField(journalLabel, value)
		if err != nil { // coverage-ignore: journalLabel is fixed grammar-valid and value is validated
			return nil, err
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func upgradeRemedies(changed bool) ([]presentation.Value, error) {
	texts := []string{"correct the reported cause and retry"}
	if changed {
		texts = []string{
			"run awf upgrade --recover if an upgrade journal exists",
			"inspect the listed changed axes",
			"restore the project from version control if recovery cannot complete",
		}
	}
	steps := make([]presentation.Value, 0, len(texts))
	for _, text := range texts {
		value, err := presentation.Prose(text)
		if err != nil { // coverage-ignore: closed remedy literals are presentation-valid
			return nil, err
		}
		steps = append(steps, value)
	}
	return steps, nil
}
