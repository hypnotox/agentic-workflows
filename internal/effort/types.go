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

// managedTopologyError preserves the legacy error message while carrying
// model-owned recovery actions, so callers never inspect the prose.
type managedTopologyError struct {
	message string
	actions []RecoveryAction
}

func (e *managedTopologyError) Error() string { return e.message }
func (e *managedTopologyError) Unwrap() error { return ErrManagedTopologyPresent }

// refusalError preserves established effort refusal prose and cause identity
// while carrying the semantic facts needed for ordinary CLI presentation.
type refusalError struct {
	message   string
	condition string
	state     string
	cause     string
	actions   []RecoveryAction
	err       error
}

func (e *refusalError) Error() string { return e.message }
func (e *refusalError) Unwrap() error { return e.err }

func refusal(message, condition, state, cause string, actions []RecoveryAction, err error) error {
	return &refusalError{message: message, condition: condition, state: state, cause: cause, actions: actions, err: err}
}

func managedTopologyRefusal(actions []RecoveryAction, format string, args ...any) error {
	return &managedTopologyError{message: fmt.Sprintf(format, args...), actions: actions}
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

// FinishResidentState identifies the resident namespace proven at return.
type FinishResidentState string

const (
	FinishStateActive   FinishResidentState = "active"
	FinishStateReserved FinishResidentState = "reserved"
	FinishStateArchived FinishResidentState = "archived"
)

// FinishResult reports each observable namespace and durability boundary.
type FinishResult struct {
	State                    FinishResidentState
	Reserved                 bool
	Archived                 bool
	DestinationSyncAvailable bool
	SourceSyncAvailable      bool
	DestinationSynced        bool
	SourceSynced             bool
	ArchivePath              string
}

// RollbackResult reports the narrow failed-creation deletion transition.
type RollbackResult struct {
	Reserved        bool
	Removed         bool
	ReservationPath string
	ResiduePath     string
}

// RecoveryAction is one independently executable ordered remedy for a failed
// effort operation. Its meaning remains model-owned until presentation maps it.
type RecoveryAction struct{ Text string }

// PartialFinishError preserves a failed restartable finish's observed state,
// mechanism cause, and site-specific recovery actions.
type PartialFinishError struct {
	Result  FinishResult
	Cause   error
	Actions []RecoveryAction
}

// Error preserves the failed mechanism's message for legacy error callers.
func (e *PartialFinishError) Error() string { return e.Cause.Error() }

// Unwrap exposes the failed mechanism for identity-aware callers.
func (e *PartialFinishError) Unwrap() error { return e.Cause }
