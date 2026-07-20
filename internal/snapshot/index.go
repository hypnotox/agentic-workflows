package snapshot

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/git"
)

// IndexTree captures the repository's stage-0 index as an immutable Tree.
// Ordinary and executable files are included with their mode preserved;
// symlinks and gitlinks have no regular content and are skipped; an unmerged or
// unreadable index is rejected. Path selection and ordering come from
// git.IndexBlobs, which opens the repository through git.OpenRepo and so
// resolves a linked worktree's index like any other command.
func IndexTree(repoRoot string) (*Tree, error) {
	blobs, err := git.IndexBlobs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("snapshot index: %w", err)
	}
	files := make([]File, len(blobs))
	for i, b := range blobs {
		mode := Regular
		if b.Executable {
			mode = Executable
		}
		files[i] = File{Path: b.Path, Mode: mode, Bytes: b.Bytes}
	}
	return NewTree(files)
}
