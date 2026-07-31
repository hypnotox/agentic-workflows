// Package effort owns repository-local immutable effort residents and their memory.
package effort

import (
	"errors"
	"fmt"
	"time"
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

func managedTopologyRefusal(format string, args ...any) error {
	return &managedTopologyError{message: fmt.Sprintf(format, args...)}
}

const SchemaVersion = 2

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
