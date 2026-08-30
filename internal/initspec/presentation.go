package initspec

import "github.com/hypnotox/agentic-workflows/internal/presentation"

// Outcome is the complete ordinary result of one successful initialization.
// It combines config adoption, sync mutation, advisories, and ordered next
// actions without owning any mutation behavior.
type Outcome struct {
	Status         string
	ConfigPath     string
	ExistingConfig bool
	IgnoredAnswers bool
	Sync           presentation.Mutation
	Advisories     []string
	NextActions    []string
}

// Document maps a successful init outcome into one complete Mutation.
func (o Outcome) Document() (presentation.Document, error) {
	configPath, err := presentation.Literal(o.ConfigPath)
	if err != nil {
		return presentation.Document{}, err
	}
	configAction := "scaffolded"
	if o.ExistingConfig {
		configAction = "kept and re-rendered"
	}
	action, err := presentation.Prose(configAction)
	if err != nil {
		return presentation.Document{}, err
	}
	pathField, err := presentation.NewField("config", configPath)
	if err != nil {
		return presentation.Document{}, err
	}
	actionField, err := presentation.NewField("config action", action)
	if err != nil {
		return presentation.Document{}, err
	}
	identity := append([]presentation.Field{pathField, actionField}, o.Sync.Identity...)
	changes := append([]presentation.MutationChange(nil), o.Sync.Changes...)
	notes := append([]presentation.Value(nil), o.Sync.Notes...)
	if o.IgnoredAnswers {
		value, valueErr := presentation.Prose("--set/--answers values were ignored; edit .awf/config.yaml instead")
		if valueErr != nil {
			return presentation.Document{}, valueErr
		}
		notes = append(notes, value)
	}
	advisories, err := proseValues(o.Advisories)
	if err != nil {
		return presentation.Document{}, err
	}
	notes = append(notes, advisories...)
	nextActions := append([]presentation.Value(nil), o.Sync.NextActions...)
	actions, err := proseValues(o.NextActions)
	if err != nil {
		return presentation.Document{}, err
	}
	nextActions = append(nextActions, actions...)
	status := o.Status
	if status == "" {
		status = "initialization completed"
	}
	return (presentation.Mutation{Status: status, Identity: identity, Changes: changes, Notes: notes, NextActions: nextActions}).Document()
}

func proseValues(values []string) ([]presentation.Value, error) {
	result := make([]presentation.Value, len(values))
	for i, text := range values {
		value, err := presentation.Prose(text)
		if err != nil {
			return nil, err
		}
		result[i] = value
	}
	return result, nil
}
