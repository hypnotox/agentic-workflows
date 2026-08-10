//go:build !windows

package adr

import (
	"fmt"
	"path/filepath"
)

func canonicalDecisionsDirectory(dir string) (string, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil { // coverage-ignore: Abs fails only for an unavailable working directory
		return "", fmt.Errorf("make absolute: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve symbolic links: %w", err)
	}
	return resolved, nil
}
