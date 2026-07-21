package snapshot

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/git"
)

// CommitTree captures the committed tree that rev resolves to as an immutable
// Tree. It reads only committed content, never the working tree, so a commit or
// HEAD universe is reproducible regardless of local edits. Ordinary and
// executable, and symlink files are included with their mode preserved;
// symlink bytes are inert targets and gitlinks are skipped.
func CommitTree(repoRoot, rev string) (*Tree, error) {
	blobs, err := git.CommitBlobs(repoRoot, rev)
	if err != nil {
		return nil, fmt.Errorf("snapshot commit: %w", err)
	}
	return treeFromBlobs(blobs)
}
