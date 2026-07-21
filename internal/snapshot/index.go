package snapshot

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/git"
)

// IndexTree captures the repository's stage-0 index as an immutable Tree.
// Ordinary, executable, and symlink files are included with mode preserved;
// symlink bytes are inert targets and gitlinks are skipped; an unmerged or
// unreadable index is rejected. Path selection and ordering come from
// git.IndexBlobs, which opens the repository through git.OpenRepo and so
// resolves a linked worktree's index like any other command.
func IndexTree(repoRoot string) (*Tree, error) {
	blobs, err := git.IndexBlobs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("snapshot index: %w", err)
	}
	return treeFromBlobs(blobs)
}

// treeFromBlobs converts git regular-file blobs into an immutable Tree,
// mapping the executable bit onto the Tree's Mode.
func treeFromBlobs(blobs []git.IndexBlob) (*Tree, error) {
	files := make([]File, len(blobs))
	for i, b := range blobs {
		mode := Regular
		switch b.Mode {
		case git.BlobRegular:
			mode = Regular
		case git.BlobExecutable:
			mode = Executable
		case git.BlobSymlink:
			mode = Symlink
		}
		files[i] = File{Path: b.Path, Mode: mode, Bytes: b.Bytes}
	}
	return NewTree(files)
}
