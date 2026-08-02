// Package fsfixture provides the repository's kernel-backed controlled filesystem fault source as the distinct test source authorized by ADR-consumer-local-contracts-over-single-home-filesystem-access.
package fsfixture

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// ADR-consumer-local-contracts-over-single-home-filesystem-access permits this distinct test source because the standard-library-only testsupport leaf cannot import the production handle.

// Operation identifies a controllable filesystem operation.
type Operation string

const (
	// OperationWalk faults traversal before entry metadata is resolved.
	OperationWalk Operation = "walk"
	// OperationWalkInfo faults entry metadata resolution during traversal.
	OperationWalkInfo Operation = "walk-info"
	// OperationRead faults file reads.
	OperationRead Operation = "read"
	// OperationInfo faults final-symlink-following metadata reads.
	OperationInfo Operation = "info"
	// OperationLinkInfo faults final-symlink-preserving metadata reads.
	OperationLinkInfo Operation = "link-info"
)

// Fault configures one operation and path to return Err.
type Fault struct {
	Operation Operation
	Path      string
	Err       error
}

// Handle is a kernel-backed filesystem handle with controlled faults.
type Handle struct {
	root   *os.Root
	faults map[faultKey]error
}
type faultKey struct {
	operation Operation
	path      string
}

// Open opens root after validating controlled faults.
func Open(root string, faults ...Fault) (*Handle, error) {
	configured := make(map[faultKey]error, len(faults))
	for i, fault := range faults {
		if !known(fault.Operation) {
			return nil, fmt.Errorf("fsfixture: fault %d: unknown operation %q", i, fault.Operation)
		}
		if fault.Err == nil {
			return nil, fmt.Errorf("fsfixture: fault %d: nil error", i)
		}
		if err := validPath(fault.Path); err != nil {
			return nil, fmt.Errorf("fsfixture: fault %d: invalid path %q", i, fault.Path)
		}
		key := faultKey{fault.Operation, fault.Path}
		if _, ok := configured[key]; ok {
			return nil, fmt.Errorf("fsfixture: fault %d: duplicate %s fault for %q", i, fault.Operation, fault.Path)
		}
		configured[key] = fault.Err
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("fsfixture: open root %q: %w", root, err)
	}
	return &Handle{root: r, faults: configured}, nil
}

// Close closes the fixture root.
func (h *Handle) Close() error { return h.root.Close() }

// Walk visits entries and applies configured traversal faults.
func (h *Handle) Walk(subtree string, visit func(string, fs.FileInfo) (bool, error)) error {
	if err := validPath(subtree); err != nil {
		return h.wrap(OperationWalk, subtree, err)
	}
	if err := h.fault(OperationWalk, subtree); err != nil {
		return h.wrap(OperationWalk, subtree, err)
	}
	if err := h.fault(OperationWalkInfo, subtree); err != nil {
		return h.wrap(OperationWalkInfo, subtree, err)
	}
	rootInfo, err := h.root.Lstat(subtree)
	if err != nil {
		return h.wrap(OperationWalkInfo, subtree, err)
	}
	if rootInfo.Mode()&fs.ModeSymlink != 0 {
		_, err := visit(subtree, rootInfo)
		if err != nil {
			return h.wrap(OperationWalk, subtree, err)
		}
		return nil
	}
	return fs.WalkDir(h.root.FS(), subtree, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil { // coverage-ignore: permission controls are execution-identity-dependent; otherwise an underlying WalkDir error requires a concurrent filesystem change
			return h.wrap(OperationWalk, path, walkErr)
		}
		if err := h.fault(OperationWalk, path); err != nil {
			return h.wrap(OperationWalk, path, err)
		}
		if err := h.fault(OperationWalkInfo, path); err != nil {
			return h.wrap(OperationWalkInfo, path, err)
		}
		info, err := entry.Info()
		if err != nil { // coverage-ignore: permission controls are execution-identity-dependent; otherwise this requires a concurrent filesystem change
			return h.wrap(OperationWalkInfo, path, err)
		}
		descend, err := visit(path, info)
		if err != nil {
			return h.wrap(OperationWalk, path, err)
		}
		if info.IsDir() && !descend {
			return fs.SkipDir
		}
		return nil
	})
}

// Read reads path or returns its selected fault.
func (h *Handle) Read(path string) ([]byte, error) {
	if err := validPath(path); err != nil {
		return nil, h.wrap(OperationRead, path, err)
	}
	if err := h.fault(OperationRead, path); err != nil {
		return nil, h.wrap(OperationRead, path, err)
	}
	b, err := h.root.ReadFile(path)
	if err != nil {
		return nil, h.wrap(OperationRead, path, err)
	}
	return b, nil
}

// Info returns metadata following a final symlink or its selected fault.
func (h *Handle) Info(path string) (fs.FileInfo, error) {
	if err := validPath(path); err != nil {
		return nil, h.wrap(OperationInfo, path, err)
	}
	if err := h.fault(OperationInfo, path); err != nil {
		return nil, h.wrap(OperationInfo, path, err)
	}
	info, err := h.root.Stat(path)
	if err != nil {
		return nil, h.wrap(OperationInfo, path, err)
	}
	return info, nil
}

// LinkInfo returns metadata without following a final symlink or its selected fault.
func (h *Handle) LinkInfo(path string) (fs.FileInfo, error) {
	if err := validPath(path); err != nil {
		return nil, h.wrap(OperationLinkInfo, path, err)
	}
	if err := h.fault(OperationLinkInfo, path); err != nil {
		return nil, h.wrap(OperationLinkInfo, path, err)
	}
	info, err := h.root.Lstat(path)
	if err != nil {
		return nil, h.wrap(OperationLinkInfo, path, err)
	}
	return info, nil
}

func (h *Handle) fault(operation Operation, path string) error {
	return h.faults[faultKey{operation, path}]
}
func (h *Handle) wrap(operation Operation, path string, err error) error {
	return fmt.Errorf("fsfixture: %s %q: %w", operation, path, err)
}
func known(operation Operation) bool {
	switch operation {
	case OperationWalk, OperationWalkInfo, OperationRead, OperationInfo, OperationLinkInfo:
		return true
	}
	return false
}
func validPath(path string) error {
	if path == "." || fs.ValidPath(path) {
		return nil
	}
	return errors.New("invalid path")
}
