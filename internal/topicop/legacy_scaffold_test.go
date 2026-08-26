package topicop

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

type scaffoldWriteCloser interface {
	io.Writer
	Close() error
}

type scaffoldDependencies struct {
	mkdirAll func(string, os.FileMode) error
	readDir  func(string) ([]os.DirEntry, error)
	stat     func(string) (os.FileInfo, error)
	openFile func(string, int, os.FileMode) (scaffoldWriteCloser, error)
	remove   func(string) error
}

func productionScaffoldDependencies() scaffoldDependencies {
	return scaffoldDependencies{os.MkdirAll, os.ReadDir, os.Stat, func(path string, flag int, mode os.FileMode) (scaffoldWriteCloser, error) {
		return os.OpenFile(path, flag, mode)
	}, os.Remove}
}

func scaffoldWith(root string, cfg *config.Config, domain, title string, dependencies scaffoldDependencies) (presentation.Document, error) {
	files, err := topic.ScaffoldFilesWithExists(cfg, domain, title, func(relative string) (bool, error) {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative)))
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
	createdFiles := make([]string, 0, len(files))
	var createdDirs []string
	for _, file := range files {
		path := filepath.Join(root, filepath.FromSlash(file.Path))
		dirs, err := createParents(filepath.Dir(path), dependencies)
		createdDirs = append(createdDirs, dirs...)
		if err != nil {
			return presentation.Document{}, rollback(fmt.Errorf("create parent for topic scaffold path %q: %w", filepath.ToSlash(path), err), createdFiles, createdDirs, dependencies)
		}
		writer, err := dependencies.openFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return presentation.Document{}, rollback(fmt.Errorf("create topic scaffold path %q exclusively: %w", filepath.ToSlash(path), err), createdFiles, createdDirs, dependencies)
		}
		createdFiles = append(createdFiles, path)
		if err := writeAndClose(path, writer, file.Content); err != nil {
			return presentation.Document{}, rollback(err, createdFiles, createdDirs, dependencies)
		}
	}
	return topic.CreatedDocument(files)
}

func writeAndClose(path string, writer scaffoldWriteCloser, content []byte) error {
	_, writeErr := io.Copy(writer, bytes.NewReader(content))
	closeErr := writer.Close()
	var failures []error
	if writeErr != nil {
		failures = append(failures, fmt.Errorf("write topic scaffold path %q: %w", filepath.ToSlash(path), writeErr))
	}
	if closeErr != nil {
		failures = append(failures, fmt.Errorf("close topic scaffold path %q: %w", filepath.ToSlash(path), closeErr))
	}
	return errors.Join(failures...)
}

func createParents(parent string, dependencies scaffoldDependencies) ([]string, error) {
	var missing []string
	for current := filepath.Clean(parent); ; current = filepath.Dir(current) {
		info, err := dependencies.stat(current)
		if err == nil {
			if !info.IsDir() {
				return nil, fmt.Errorf("parent path %q is not a directory", filepath.ToSlash(current))
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("inspect topic scaffold parent %q: %w", filepath.ToSlash(current), err)
		}
		missing = append(missing, current)
		next := filepath.Dir(current)
		if next == current {
			break
		}
	}
	slices.Reverse(missing)
	mkdirErr := dependencies.mkdirAll(parent, 0o755)
	created := make([]string, 0, len(missing))
	for _, path := range missing {
		if info, err := dependencies.stat(path); err == nil && info.IsDir() {
			created = append(created, path)
		}
	}
	return created, mkdirErr
}

func rollback(primary error, createdFiles, createdDirs []string, dependencies scaffoldDependencies) error {
	failures := []error{primary}
	for i := len(createdFiles) - 1; i >= 0; i-- {
		if err := dependencies.remove(createdFiles[i]); err != nil && !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, fmt.Errorf("remove created topic scaffold path %q: %w", filepath.ToSlash(createdFiles[i]), err))
		}
	}
	dirs := slices.Clone(createdDirs)
	slices.SortStableFunc(dirs, func(a, b string) int {
		return strings.Count(b, string(filepath.Separator)) - strings.Count(a, string(filepath.Separator))
	})
	for _, dir := range dirs {
		entries, err := dependencies.readDir(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("inspect created topic scaffold directory %q: %w", filepath.ToSlash(dir), err))
			continue
		}
		if len(entries) == 0 {
			if err := dependencies.remove(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
				failures = append(failures, fmt.Errorf("remove created topic scaffold directory %q: %w", filepath.ToSlash(dir), err))
			}
		}
	}
	return errors.Join(failures...)
}
