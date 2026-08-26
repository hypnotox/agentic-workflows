//go:build !windows

package filesystem

import (
	"fmt"
	"path/filepath"
)

// CanonicalRoot returns the symlink-resolved absolute identity used to
// serialize one existing physical root.
func CanonicalRoot(root string) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("make absolute: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve symbolic links: %w", err)
	}
	return resolved, nil
}
