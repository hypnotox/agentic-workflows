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

// Scaffold creates one topic's paired authored inputs atomically enough to
// remove every input and newly-created parent when a later step fails.
func Scaffold(root string, cfg *config.Config, domain, title string) (presentation.Document, error) {
	files, err := filesystem.Open(root)
	if err != nil {
		return presentation.Document{}, err
	}
	defer files.Close()
	return scaffoldConfined(files, cfg, domain, title)
}

// scaffoldConfined creates authored inputs through one selected-root handle.
// Every source is exclusive, and rollback removes only paths this invocation
// published. Parent directories are deliberately retained when another actor
// could have contributed to them, so rollback never claims ownership it lacks.
func scaffoldConfined(files *filesystem.Handle, cfg *config.Config, domain, title string) (presentation.Document, error) {
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
		return presentation.Document{}, err
	}
	created := make([]string, 0, len(planned))
	rollbackCreated := func(cause error) (presentation.Document, error) {
		var failures []error
		failures = append(failures, cause)
		for i := len(created) - 1; i >= 0; i-- {
			if err := files.Remove(created[i]); err != nil && !errors.Is(err, os.ErrNotExist) {
				failures = append(failures, fmt.Errorf("remove topic scaffold path %q: %w", created[i], err))
			}
		}
		return presentation.Document{}, errors.Join(failures...)
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
	return topic.CreatedDocument(planned)
}

// Create opens the selected project and performs one topic-scaffolding operation.
func Create(ctx context.Context, root, domain, title string, loader *project.Loader) (document presentation.Document, err error) {
	lease, err := loader.AcquireTrackedLease(ctx, root)
	if err != nil {
		return presentation.Document{}, err
	}
	defer func() { err = errors.Join(err, lease.Release()) }()
	_, cfg, err := loader.OpenForOperation(ctx, root)
	if err != nil {
		return presentation.Document{}, err
	}
	return Scaffold(root, cfg, domain, title)
}
