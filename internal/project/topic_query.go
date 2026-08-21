package project

import (
	"context"
	"fmt"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// QueryTopic assembles one read-only topic or claim projection from one
// intrinsically routed working snapshot. Active state and operation history therefore
// cannot come from different worktree universes.
func queryTopic(root string, repo *awfgit.Repo, ctx context.Context, selector string, opts topic.QueryOptions) (topic.QueryResult, error) {
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

// safelyMatchablePaths returns every scannable snapshot path: the universe a
// selector may be matched against. Symlinks and deletions are excluded because
// matching a selector against them would attribute authority to a path that
// carries no content.
func safelyMatchablePaths(tree *snapshot.Tree) []string {
	out := []string{}
	for _, f := range tree.List() {
		if f.Scannable() {
			out = append(out, f.Path)
		}
	}
	return out
}
