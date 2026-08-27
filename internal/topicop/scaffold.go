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
	"github.com/hypnotox/agentic-workflows/internal/project"
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
			return presentation.Document{}, created, &PartialScaffoldError{Created: append([]string(nil), created...), Removed: removed, Remaining: remaining, Cause: errors.Join(append([]error{cause}, failures...)...), Recovery: []string{"remove only the listed remaining topic paths, then retry"}}
		}
		return presentation.Document{}, created, cause
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

// CreateLeased opens the selected project and performs one topic-scaffolding operation under a caller-held tracked transaction.
func CreateLeased(ctx context.Context, root, domain, title string, loader *project.Loader, lease *filesystem.Lease) (document presentation.Document, err error) {
	if lease == nil || !lease.CoversTracked(root) {
		return presentation.Document{}, errors.New("topic operation requires a covering tracked lease")
	}
	files, err := filesystem.Open(root)
	if err != nil {
		return presentation.Document{}, err
	}
	var created []string
	defer func() { err = finishScaffoldClose(created, err, files.Close()) }()
	matches, err := files.RootMatches(root)
	if err != nil {
		return presentation.Document{}, err
	}
	if !matches {
		return presentation.Document{}, filesystem.ErrIdentityChanged
	}
	_, cfg, configIdentity, err := loader.OpenForMutation(ctx, root, files)
	if err != nil {
		return presentation.Document{}, err
	}
	defer configIdentity.Release() //nolint:errcheck // descriptor cleanup cannot change the filesystem mutation outcome
	matches, err = files.RootMatches(root)
	if err != nil {
		return presentation.Document{}, err
	}
	if !matches {
		return presentation.Document{}, filesystem.ErrIdentityChanged
	}
	document, created, err = scaffoldConfined(files, cfg, domain, title)
	return document, err
}
