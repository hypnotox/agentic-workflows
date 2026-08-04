// Package effort owns repository-local immutable effort residents and their memory.
package effort

import (
	"errors"
	"fmt"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// ErrManagedTopologyPresent classifies the restartable-finish refusal that
// managed Git topology for the slug still exists. Callers branch on
// errors.Is; the prose stays the user-facing protocol.
var ErrManagedTopologyPresent = errors.New("managed topology present")

// managedTopologyError carries one refusal message unchanged while
// classifying it, so callers never inspect the prose.
type managedTopologyError struct{ message string }

func (e *managedTopologyError) Error() string { return e.message }
func (e *managedTopologyError) Unwrap() error { return ErrManagedTopologyPresent }

// Diagnostic maps the restartable finish refusal from its typed class rather
// than exposing the legacy semicolon-delimited message at the command boundary.
func (e *managedTopologyError) Diagnostic() (presentation.Diagnostic, error) {
	unchanged, err := presentation.Prose("no")
	if err != nil { // coverage-ignore: fixed nonempty diagnostic fact
		return presentation.Diagnostic{}, err
	}
	resident, err := presentation.NewField("active resident", unchanged)
	if err != nil { // coverage-ignore: fixed grammar-valid diagnostic label and validated value
		return presentation.Diagnostic{}, err
	}
	present, err := presentation.Prose("yes")
	if err != nil { // coverage-ignore: fixed nonempty diagnostic fact
		return presentation.Diagnostic{}, err
	}
	topology, err := presentation.NewField("managed topology", present)
	if err != nil { // coverage-ignore: fixed grammar-valid diagnostic label and validated value
		return presentation.Diagnostic{}, err
	}
	step, err := presentation.Prose("remove the managed worktree, then retry finish")
	if err != nil { // coverage-ignore: fixed nonempty recovery step
		return presentation.Diagnostic{}, err
	}
	return presentation.Diagnostic{Condition: "managed topology remains", State: "finish", Changed: []presentation.Field{resident, topology}, Steps: []presentation.Value{step}}, nil
}

func managedTopologyRefusal(format string, args ...any) error {
	return &managedTopologyError{message: fmt.Sprintf(format, args...)}
}

const SchemaVersion = 2

// NewInput carries the caller-selected immutable identity and independent
// descriptive title for one effort creation.
type NewInput struct {
	Slug  string
	Title string
}

// Record is the public protocol-2 effort view. SchemaVersion belongs to the
// containing reply, while static state carries it directly.
type Record struct {
	SchemaVersion int       `json:"-"`
	ID            string    `json:"id"`
	Slug          string    `json:"slug"`
	Title         string    `json:"title"`
	CreatedAt     time.Time `json:"createdAt"`
	MemoryPath    string    `json:"memoryPath"`
}

// FinishResult reports the restartable deletion mutations separately.
type FinishResult struct {
	Renamed bool
	Cleaned bool
}

// PartialFinishError preserves a failed restartable finish's state without
// requiring presentation to inspect its historical error text.
type PartialFinishError struct {
	Result FinishResult
	Cause  error
	Slug   string
}

func (e *PartialFinishError) Error() string { return e.Cause.Error() }
func (e *PartialFinishError) Unwrap() error { return e.Cause }

func (e *PartialFinishError) Diagnostic() (presentation.Diagnostic, error) {
	changed := make([]presentation.Field, 0, 2)
	for _, fact := range []struct {
		label string
		value bool
	}{{"active resident", e.Result.Renamed}, {"finishing cleanup", e.Result.Cleaned}} {
		value, err := presentation.Prose(yesNo(fact.value))
		if err != nil { // coverage-ignore: yesNo always supplies a nonempty fact value
			return presentation.Diagnostic{}, err
		}
		field, err := presentation.NewField(fact.label, value)
		if err != nil { // coverage-ignore: fixed grammar-valid diagnostic label and validated value
			return presentation.Diagnostic{}, err
		}
		changed = append(changed, field)
	}
	step, err := presentation.Prose("retry `awf effort finish " + e.Slug + "`")
	if err != nil { // coverage-ignore: fixed nonempty recovery grammar always has a command prefix
		return presentation.Diagnostic{}, err
	}
	return presentation.Diagnostic{
		Condition: "effort finish was interrupted", State: "finish", Changed: changed,
		Cause: e.Cause.Error(), Steps: []presentation.Value{step},
	}, nil
}
