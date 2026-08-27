package filesystem

import (
	"fmt"
	"io/fs"
)

// SupportedTreeEntry reports whether entry is a directory or regular file.
// A zero type is ambiguous, so it resolves the complete mode before admitting
// the entry to a readable project-tree inventory.
func SupportedTreeEntry(entry fs.DirEntry) (bool, error) {
	if entry.Type() != 0 {
		return entry.IsDir() || entry.Type().IsRegular(), nil
	}
	info, err := entry.Info()
	if err != nil {
		return false, fmt.Errorf("filesystem: inspect tree entry %q: %w", entry.Name(), err)
	}
	return info.IsDir() || info.Mode().IsRegular(), nil
}
