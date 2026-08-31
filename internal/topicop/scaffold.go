package topicop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/projectmutation"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// PartialScaffoldError retains rollback facts when a topic creation cannot
// remove every path it created. Cause identity remains available to callers.
type PartialScaffoldError struct {
	Created, Removed, Remaining []string
	Cause                       error
	Recovery                    []string
}

func (e *PartialScaffoldError) Error() string { return e.Cause.Error() }
func (e *PartialScaffoldError) Unwrap() error { return e.Cause }
func (e *PartialScaffoldError) JoinCause(err error) {
	e.Cause = errors.Join(e.Cause, err)
}
func (e *PartialScaffoldError) Document() (presentation.Document, error) {
	changes := []presentation.MutationChange{}
	for _, group := range []struct {
		label string
		paths []string
	}{{"created paths", e.Created}, {"removed paths", e.Removed}, {"remaining paths", e.Remaining}} {
		if len(group.paths) == 0 {
			continue
		}
		values := []presentation.Value{}
		for _, path := range group.paths {
			value, err := presentation.Literal(path)
			if err != nil {
				return presentation.Document{}, err
			}
			values = append(values, value)
		}
		changes = append(changes, presentation.MutationChange{Label: group.label, Values: values})
	}
	recovery := e.Recovery
	if len(recovery) == 0 {
		recovery = []string{"remove only the listed remaining topic paths, then retry"}
	}
	next := make([]presentation.Value, 0, len(recovery))
	for _, action := range recovery {
		value, err := presentation.Prose(action)
		if err != nil {
			return presentation.Document{}, err
		}
		next = append(next, value)
	}
	return (presentation.Mutation{Status: "topic scaffold partially committed", Changes: changes, NextActions: next}).Document()
}

// scaffoldConfined creates authored inputs through one selected-root handle.
// Every source is exclusive, and rollback removes only paths this invocation
// published. Parent directories are deliberately retained when another actor
// could have contributed to them, so rollback never claims ownership it lacks.
func scaffoldConfined(files *filesystem.Handle, cfg *config.Config, domain, title string) (presentation.Document, []string, error) {
	planned, err := topic.ScaffoldFilesWithExists(cfg, domain, title, func(path string) (bool, error) {
		_, err := files.LinkInfo(path)
		if err == nil {
			return true, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	})
	if err != nil {
		return presentation.Document{}, nil, err
	}
	created := make([]string, 0, len(planned))
	rollbackCreated := func(cause error) (presentation.Document, []string, error) {
		removed, remaining := []string{}, []string{}
		var failures []error
		for i := len(created) - 1; i >= 0; i-- {
			if err := files.Remove(created[i]); err != nil && !errors.Is(err, os.ErrNotExist) {
				failures = append(failures, fmt.Errorf("remove topic scaffold path %q: %w", created[i], err))
				remaining = append(remaining, created[i])
			} else {
				removed = append(removed, created[i])
			}
		}
		if len(failures) != 0 {
			return presentation.Document{}, nil, &PartialScaffoldError{Created: append([]string(nil), created...), Removed: removed, Remaining: remaining, Cause: errors.Join(append([]error{cause}, failures...)...), Recovery: []string{"remove only the listed remaining topic paths, then retry"}}
		}
		return presentation.Document{}, nil, cause
	}
	for _, file := range planned {
		rel := filepath.ToSlash(file.Path)
		if err := files.MkdirAll(filepath.ToSlash(filepath.Dir(rel)), 0o755); err != nil {
			return rollbackCreated(fmt.Errorf("create parent for topic scaffold path %q: %w", rel, err))
		}
		if err := files.Publish(rel, file.Content, 0o644); err != nil {
			return rollbackCreated(fmt.Errorf("create topic scaffold path %q exclusively: %w", rel, err))
		}
		created = append(created, rel)
	}
	document, err := topic.CreatedDocument(planned)
	return document, created, err
}

func finishScaffoldClose(created []string, operationErr, closeErr error) error {
	if closeErr == nil {
		return operationErr
	}
	if operationErr != nil || len(created) == 0 {
		return errors.Join(operationErr, closeErr)
	}
	paths := append([]string(nil), created...)
	return &PartialScaffoldError{
		Created:   paths,
		Remaining: append([]string(nil), paths...),
		Cause:     fmt.Errorf("close selected root after topic scaffold: %w", closeErr),
		Recovery: []string{
			"inspect the listed created topic paths",
			"do not retry scaffolding until the listed paths are reconciled",
		},
	}
}

// Outcome retains the semantic document and every authored path created by one
// topic scaffold.
type Outcome struct {
	Document presentation.Document
	Created  []string
}

func committed(outcome Outcome) bool { return len(outcome.Created) != 0 }

// Finish retains created paths when presentation cleanup or the caller-held
// lease release fails after the focused mutation returns.
func Finish(outcome Outcome, operationErr, releaseErr error) error {
	makePartial := func(outcome Outcome, cause error, phase projectmutation.Phase) error {
		recovery := []string{"inspect the listed created topic paths", "do not retry scaffolding until the post-commit fault is repaired"}
		if phase == projectmutation.PhaseRelease {
			recovery[1] = "do not retry scaffolding until the lease-release fault is repaired"
		}
		paths := append([]string(nil), outcome.Created...)
		return &PartialScaffoldError{Created: paths, Remaining: append([]string(nil), paths...), Cause: cause, Recovery: recovery}
	}
	operationErr = projectmutation.Promote(outcome, operationErr, projectmutation.PhaseCleanup, committed, makePartial)
	return projectmutation.Finish(outcome, operationErr, releaseErr, committed, makePartial)
}

// Create opens the selected project and performs one topic-scaffolding operation under the caller's tracked transaction.
func Create(_ context.Context, domain, title string, tx *projectmutation.Transaction) (outcome Outcome, err error) {
	if tx == nil || tx.Scope() != projectmutation.TrackedScope {
		return Outcome{}, errors.New("topic operation requires a covering tracked lease")
	}
	files, err := tx.Open()
	if err != nil {
		return Outcome{}, err
	}
	defer func() { err = finishScaffoldClose(outcome.Created, err, files.Close()) }()
	matches, err := files.RootMatches(tx.Root())
	if err != nil {
		return Outcome{}, err
	}
	if !matches {
		return Outcome{}, filesystem.ErrIdentityChanged
	}
	session, configIdentity, err := tx.LoadForMutation(files)
	if err != nil {
		return Outcome{}, err
	}
	cfg := session.Config()
	defer configIdentity.Release() //nolint:errcheck // descriptor cleanup cannot change the filesystem mutation outcome
	matches, err = files.RootMatches(tx.Root())
	if err != nil {
		return Outcome{}, err
	}
	if !matches {
		return Outcome{}, filesystem.ErrIdentityChanged
	}
	outcome.Document, outcome.Created, err = scaffoldConfined(files, cfg, domain, title)
	return outcome, err
}
