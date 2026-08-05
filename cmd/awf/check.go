package main

import (
	"context"
	"errors"
	"io"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

type planNoteSink map[string]struct{}

var (
	checkOpenContaining = awfgit.OpenContaining
	checkCollectStaged  = collectCheckStaged
)

// runCheck runs both check universes. Outside a Git repository the repo universe
// still applies, while the staged universe is unavailable.
func runCheck(ctx context.Context, root string, stdout io.Writer) error {
	planNotes := planNoteSink{}
	repo, repoErr := collectCheckRepoWithPlanNotes(ctx, root, planNotes)
	if repoErr != nil {
		return repoErr
	}
	_, _, gitErr := checkOpenContaining(root)
	if errors.Is(gitErr, awfgit.ErrNotARepository) {
		repo.notes = append(repo.notes, "staged check universe unavailable outside a git repository")
		return renderCheckCollection(stdout, repo)
	}
	if gitErr != nil {
		return gitErr
	}
	staged, stagedErr := checkCollectStaged(ctx, root, planNotes)
	if stagedErr != nil {
		return stagedErr
	}
	return renderCheckCollection(stdout, repo.append(staged))
}
