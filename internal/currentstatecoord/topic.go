package currentstatecoord

import (
	"context"
	"fmt"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// QueryTopic selects and validates one working authority universe before
// delegating topic query semantics to topic.
func QueryTopic(root string, repo *awfgit.Repo, ctx context.Context, selector string, opts topic.QueryOptions) (topic.QueryResult, error) {
	ws, err := workingCurrentState(root, repo, ctx)
	if err != nil {
		return topic.QueryResult{}, err
	}
	findings := currentstate.Check(ws.Loaded.ADRs, ws.Loaded.Topics.All())
	if len(findings) > 0 {
		messages := make([]string, len(findings))
		for i, finding := range findings {
			messages[i] = finding.Message
		}
		return topic.QueryResult{}, fmt.Errorf("current-state validation failed: %s", strings.Join(messages, "; "))
	}
	return topic.Query(ws.Loaded.Topics, ws.Loaded.Corpus, selector, opts, safelyMatchablePaths(ws.Tree))
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
