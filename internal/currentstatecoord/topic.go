package currentstatecoord

import (
	"context"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// QueryTopic selects one working topic-authority universe before delegating
// topic query semantics to topic. Historical decisions are not current-state
// authority and are therefore not loaded or validated here.
func QueryTopic(root string, repo *awfgit.Repo, ctx context.Context, selector string, opts topic.QueryOptions) (topic.QueryResult, error) {
	ws, err := workingCurrentState(root, repo, ctx)
	if err != nil {
		return topic.QueryResult{}, err
	}
	return topic.Query(ws.Loaded.Topics, selector, opts, safelyMatchablePaths(ws.Tree))
}

func safelyMatchablePaths(tree *snapshot.Tree) []string {
	out := []string{}
	for _, f := range tree.List() {
		if f.Scannable() {
			out = append(out, f.Path)
		}
	}
	return out
}
