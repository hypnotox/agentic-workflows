package main

import (
	"context"
	"errors"
	"io"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

type planNoteSink map[string]struct{}

type checkDependencies struct {
	openContaining func(string) (*awfgit.Repo, string, error)
	collectRepo    func(context.Context, string, planNoteSink) (checkCollection, error)
	collectStaged  func(context.Context, string, planNoteSink) (checkCollection, error)
}

func productionCheckDependencies() checkDependencies {
	return checkDependencies{
		openContaining: awfgit.OpenContaining,
		collectRepo:    collectCheckRepoWithPlanNotes,
		collectStaged:  collectCheckStaged,
	}
}

// runCheck runs both check universes. Outside a Git repository the repo universe
// still applies, while the staged universe is unavailable.
func runCheck(ctx context.Context, root string, stdout io.Writer) error {
	return runCheckWith(ctx, root, stdout, productionCheckDependencies())
}

func runCheckWith(ctx context.Context, root string, stdout io.Writer, dependencies checkDependencies) error {
	planNotes := planNoteSink{}
	repo, repoErr := dependencies.collectRepo(ctx, root, planNotes)
	if repoErr != nil {
		repo.operational = append(repo.operational, repoErr)
	}
	_, _, gitErr := dependencies.openContaining(root)
	if errors.Is(gitErr, awfgit.ErrNotARepository) {
		repo.information = append(repo.information, "staged check universe unavailable outside a git repository")
		return renderCheckCollection(stdout, repo)
	}
	if gitErr != nil {
		return errors.Join(append(repo.operational, gitErr)...)
	}
	staged, stagedErr := dependencies.collectStaged(ctx, root, planNotes)
	repo = repo.append(staged)
	if stagedErr != nil {
		repo.operational = append(repo.operational, stagedErr)
	}
	return renderCheckCollection(stdout, repo)
}
