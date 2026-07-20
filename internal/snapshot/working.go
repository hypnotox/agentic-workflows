package snapshot

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/git"
)

// WorkingTree captures the repository's working universe as an immutable Tree:
// every tracked-and-present or nonignored-untracked regular file, with the
// executable bit preserved. Symlinks are not followed and contribute no
// content; deleted, ignored, and nested-repository paths are already excluded
// by git.WorkingPaths. It is the complete selected filesystem universe;
// generated, contextIgnore, and other eligibility filters are applied by
// downstream consumers, not here.
func WorkingTree(repoRoot string) (*Tree, error) {
	paths, err := git.WorkingPaths(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("snapshot working: %w", err)
	}
	var files []File
	for _, p := range paths {
		full := filepath.Join(repoRoot, filepath.FromSlash(p))
		info, statErr := os.Lstat(full)
		if statErr != nil { // coverage-ignore: git just enumerated this path; only a concurrent filesystem mutation can make Lstat fail
			return nil, fmt.Errorf("snapshot working stat %s: %w", p, statErr)
		}
		if !info.Mode().IsRegular() {
			continue // symlinks and directories have no snapshot content
		}
		data, readErr := os.ReadFile(full)
		if readErr != nil {
			return nil, fmt.Errorf("snapshot working read %s: %w", p, readErr)
		}
		mode := Regular
		if info.Mode().Perm()&0o111 != 0 {
			mode = Executable
		}
		files = append(files, File{Path: p, Mode: mode, Bytes: data})
	}
	return NewTree(files)
}
