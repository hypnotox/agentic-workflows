//go:build !windows

package adr

import "github.com/hypnotox/agentic-workflows/internal/filesystem"

func canonicalDecisionsDirectory(dir string) (string, error) {
	return filesystem.CanonicalRoot(dir)
}
