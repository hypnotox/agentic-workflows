package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

// runCheck runs both check universes. Outside a Git repository the repo universe
// still applies, while the staged universe is unavailable.
func runCheck(ctx context.Context, root string, stdout io.Writer) error {
	repoCheckErr := runCheckRepo(ctx, root, stdout)
	_, _, repoErr := awfgit.OpenContaining(root)
	if errors.Is(repoErr, awfgit.ErrNotARepository) {
		fmt.Fprintln(stdout, "note: staged check universe unavailable outside a git repository")
		return repoCheckErr
	}
	if repoErr != nil {
		return errors.Join(repoCheckErr, repoErr)
	}
	return errors.Join(repoCheckErr, runCheckStaged(ctx, root, stdout))
}
