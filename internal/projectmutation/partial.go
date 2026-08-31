package projectmutation

import "errors"

// Partial retains an operation-owned outcome, cause, and ordered recovery
// guidance without taking ownership of their policy.
type Partial[T any] struct {
	Outcome  T
	Cause    error
	Recovery []string
}

// NewPartial constructs shared typed partial-effect plumbing for a focused
// operation's outcome and recovery policy.
func NewPartial[T any](outcome T, cause error, recovery []string) Partial[T] {
	return Partial[T]{Outcome: outcome, Cause: cause, Recovery: append([]string(nil), recovery...)}
}

func (e *Partial[T]) Error() string { return e.Cause.Error() }
func (e *Partial[T]) Unwrap() error { return e.Cause }

// JoinCause retains a later cleanup or release failure in the same typed
// partial result.
func (e *Partial[T]) JoinCause(err error) { e.Cause = errors.Join(e.Cause, err) }

type causeJoiner interface{ JoinCause(error) }

// Promote converts a plain post-commit failure into the focused operation's
// typed partial result. Existing partial results remain authoritative.
func Promote[T any](outcome T, err error, phase Phase, committed func(T) bool, makePartial func(T, error, Phase) error) error {
	if err == nil || !committed(outcome) {
		return err
	}
	var partial causeJoiner
	if errors.As(err, &partial) {
		return err
	}
	return makePartial(outcome, err, phase)
}

// Finish joins a release result after the focused operation has classified its
// own failure. Committed outcomes become typed partials, while an existing
// partial retains both causes.
func Finish[T any](outcome T, operationErr, releaseErr error, committed func(T) bool, makePartial func(T, error, Phase) error) error {
	if releaseErr == nil {
		return operationErr
	}
	var partial causeJoiner
	if errors.As(operationErr, &partial) {
		partial.JoinCause(releaseErr)
		return operationErr
	}
	joined := errors.Join(operationErr, releaseErr)
	if committed(outcome) {
		return makePartial(outcome, joined, PhaseRelease)
	}
	return joined
}
