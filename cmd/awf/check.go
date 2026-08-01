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
	if err := runCheckRepo(ctx, root, stdout); err != nil {
		return err
	}
	_, _, repoErr := awfgit.OpenContaining(root)
	if errors.Is(repoErr, awfgit.ErrNotARepository) {
		fmt.Fprintln(stdout, "note: staged check universe unavailable outside a git repository")
		return nil
	}
	if repoErr != nil {
		return repoErr
	}
	return runCheckStaged(ctx, root, stdout)
}
