package snapshot

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/git"
)

// RangePair captures the before and after Trees for the transition into the
// commit rev resolves to: after is that commit's tree and before is its
// first-parent tree, or an empty Tree for a root commit. Merges follow the
// first parent only, so a transition committed on a branch and merged is still
// observed at the merge. Both are committed universes; neither reads working
// bytes.
func RangePair(repoRoot, rev string) (before, after *Tree, err error) {
	beforeBlobs, afterBlobs, err := git.RangeBlobs(repoRoot, rev)
	if err != nil {
		return nil, nil, fmt.Errorf("snapshot range: %w", err)
	}
	if before, err = treeFromBlobs(beforeBlobs); err != nil { // coverage-ignore: git tree blobs carry unique, safe paths and representable modes, so NewTree cannot reject them
		return nil, nil, err
	}
	if after, err = treeFromBlobs(afterBlobs); err != nil { // coverage-ignore: git tree blobs carry unique, safe paths and representable modes, so NewTree cannot reject them
		return nil, nil, err
	}
	return before, after, nil
}
