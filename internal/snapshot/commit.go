package snapshot

import (
	"context"
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/git"
)

// CommitTree captures the committed tree that rev resolves to as an immutable
// Tree. It reads only committed content, never the working tree, so a commit or
// HEAD universe is reproducible regardless of local edits. Ordinary and
// executable, and symlink files are included with their mode preserved;
// symlink bytes are inert targets and gitlinks are skipped.
func CommitTree(ctx context.Context, repo *git.Repo, rev string) (*Tree, error) {
	blobs, err := repo.CommitBlobs(ctx, rev)
	if err != nil {
		return nil, fmt.Errorf("snapshot commit: %w", err)
	}
	return treeFromBlobs(blobs)
}

// CommitTrees captures revisions in caller order.
func CommitTrees(ctx context.Context, repo *git.Repo, revs []string) ([]*Tree, error) {
	trees := make([]*Tree, len(revs))
	for i, rev := range revs {
		tree, err := CommitTree(ctx, repo, rev)
		if err != nil {
			return nil, err
		}
		trees[i] = tree
	}
	return trees, nil
}
