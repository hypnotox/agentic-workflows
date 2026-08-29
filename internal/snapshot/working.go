package snapshot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/git"
)

// WorkingTree captures the repository's working universe as an immutable Tree:
// every tracked-and-present or nonignored-untracked file, with executable and
// symlink modes preserved. Symlinks are not followed; their target is retained
// as inert bytes. Deleted, ignored, and nested-repository paths are excluded
// by the handle's WorkingPaths. It is the complete selected filesystem
// universe; generated-output and other consumer-specific eligibility filters
// are applied by downstream consumers, not here.
func WorkingTree(ctx context.Context, repo *git.Repo) (*Tree, error) {
	paths, err := repo.WorkingPaths(ctx)
	if err != nil {
		return nil, fmt.Errorf("snapshot working: %w", err)
	}
	root := repo.Root()
	var files []File
	for _, p := range paths {
		full := filepath.Join(root, filepath.FromSlash(p))
		info, statErr := os.Lstat(full)
		if statErr != nil { // coverage-ignore: git just enumerated this path; only a concurrent filesystem mutation can make Lstat fail
			return nil, fmt.Errorf("snapshot working stat %s: %w", p, statErr)
		}
		mode := Regular
		var data []byte
		if info.Mode()&os.ModeSymlink != 0 {
			target, readErr := os.Readlink(full)
			if readErr != nil { // coverage-ignore: Lstat just identified this link; only a concurrent mutation can fail Readlink
				return nil, fmt.Errorf("snapshot working readlink %s: %w", p, readErr)
			}
			mode, data = Symlink, []byte(target)
		} else {
			if !info.Mode().IsRegular() {
				continue
			}
			var readErr error
			data, readErr = os.ReadFile(full)
			if readErr != nil {
				return nil, fmt.Errorf("snapshot working read %s: %w", p, readErr)
			}
			if info.Mode().Perm()&0o111 != 0 {
				mode = Executable
			}
		}
		files = append(files, File{Path: p, Mode: mode, Bytes: data})
	}
	return NewTree(files)
}
