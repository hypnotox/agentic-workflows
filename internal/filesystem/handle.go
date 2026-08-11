// Package filesystem owns root-confined production filesystem access.
package filesystem

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/filepublication"
)

// Handle provides root-confined filesystem operations.
type Handle struct {
	root *os.Root
}

// Open opens root as a root-confined filesystem handle.
func Open(root string) (*Handle, error) {
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("filesystem: open root %q: %w", root, err)
	}
	return &Handle{root: r}, nil
}

// Close closes the root-confined filesystem handle.
func (h *Handle) Close() error { return h.root.Close() }

// Walk visits subtree entries with metadata describing each entry itself.
func (h *Handle) Walk(subtree string, visit func(path string, info fs.FileInfo) (bool, error)) error {
	if err := validPath(subtree); err != nil {
		return fmt.Errorf("filesystem: walk %q: %w", subtree, err)
	}
	rootInfo, err := h.root.Lstat(subtree)
	if err != nil {
		return fmt.Errorf("filesystem: walk-info %q: %w", subtree, err)
	}
	if rootInfo.Mode()&fs.ModeSymlink != 0 {
		_, err := visit(subtree, rootInfo)
		if err != nil {
			return fmt.Errorf("filesystem: walk %q: %w", subtree, err)
		}
		return nil
	}
	err = fs.WalkDir(h.root.FS(), subtree, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil { // coverage-ignore: permission controls are execution-identity-dependent; otherwise an underlying WalkDir error requires a concurrent filesystem change
			return fmt.Errorf("filesystem: walk %q: %w", path, walkErr)
		}
		info, err := entry.Info()
		if err != nil { // coverage-ignore: permission controls are execution-identity-dependent; otherwise this requires a concurrent filesystem change
			return fmt.Errorf("filesystem: walk-info %q: %w", path, err)
		}
		descend, err := visit(path, info)
		if err != nil {
			return fmt.Errorf("filesystem: walk %q: %w", path, err)
		}
		if info.IsDir() && !descend {
			return fs.SkipDir
		}
		return nil
	})
	return err
}

// MkdirAll creates path and missing parents beneath the selected root.
func (h *Handle) MkdirAll(path string, mode fs.FileMode) error {
	if err := validPath(path); err != nil {
		return fmt.Errorf("filesystem: mkdir-all %q: %w", path, err)
	}
	if err := h.root.MkdirAll(path, mode); err != nil {
		return fmt.Errorf("filesystem: mkdir-all %q: %w", path, err)
	}
	return nil
}

// Publish atomically publishes one complete file without replacement beneath
// the selected root.
func (h *Handle) Publish(path string, contents []byte, mode fs.FileMode) error {
	if err := validPath(path); err != nil {
		return fmt.Errorf("filesystem: publish %q: %w", path, err)
	}
	if err := filepublication.PublishConfined(h.root, path, contents, mode); err != nil {
		return fmt.Errorf("filesystem: publish %q: %w", path, err)
	}
	return nil
}

// Read reads path beneath the selected root.
func (h *Handle) Read(path string) ([]byte, error) {
	if err := validPath(path); err != nil {
		return nil, fmt.Errorf("filesystem: read %q: %w", path, err)
	}
	b, err := h.root.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("filesystem: read %q: %w", path, err)
	}
	return b, nil
}

// Info returns metadata for path, following a final symbolic link.
func (h *Handle) Info(path string) (fs.FileInfo, error) {
	if err := validPath(path); err != nil {
		return nil, fmt.Errorf("filesystem: info %q: %w", path, err)
	}
	info, err := h.root.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("filesystem: info %q: %w", path, err)
	}
	return info, nil
}

// LinkInfo returns metadata for path without following a final symbolic link.
func (h *Handle) LinkInfo(path string) (fs.FileInfo, error) {
	if err := validPath(path); err != nil {
		return nil, fmt.Errorf("filesystem: link-info %q: %w", path, err)
	}
	info, err := h.root.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("filesystem: link-info %q: %w", path, err)
	}
	return info, nil
}

func validPath(path string) error {
	if path == "." || fs.ValidPath(path) {
		return nil
	}
	return errors.New("invalid path")
}
