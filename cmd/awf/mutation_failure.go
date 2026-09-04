package main

import (
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

// mutationFailure is the command boundary's ordinary failed-mutation
// diagnostic. It reports only visible facts and retry guidance; it makes no
// rollback or recovery claim.
type mutationFailure struct {
	condition string
	cause     error
	touched   []string
	pending   []string
	rerun     string
}

func (e mutationFailure) Error() string { return e.cause.Error() }
func (e mutationFailure) Unwrap() error { return e.cause }

func (e mutationFailure) Diagnostic() (presentation.Diagnostic, error) {
	changed := make([]presentation.Field, 0, len(e.touched))
	for _, path := range e.touched {
		field, err := pathField("touched path", path)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		changed = append(changed, field)
	}
	planned := make([]presentation.Field, 0, len(e.pending))
	for _, path := range e.pending {
		field, err := pathField("pending path", path)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		planned = append(planned, field)
	}
	texts := []string{
		"run git status --short and git diff to inspect visible changes",
		"correct or restore the desired paths",
		"rerun " + e.rerun,
	}
	steps := make([]presentation.Value, len(texts))
	for i, text := range texts {
		value, err := presentation.Prose(text)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		steps[i] = value
	}
	return presentation.Diagnostic{Condition: e.condition, State: "operation", Planned: planned, Changed: changed, Cause: e.cause.Error(), Steps: steps}, nil
}

func pathField(label, path string) (presentation.Field, error) {
	value, err := presentation.Literal(path)
	if err != nil {
		return presentation.Field{}, err
	}
	return presentation.NewField(label, value)
}

func publisherPaths(result publisher.Result) []string {
	return result.Touched()
}
