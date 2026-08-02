package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

type planNoteSink map[string]struct{}

func (s planNoteSink) write(stdout io.Writer, notes []string) {
	for _, note := range notes {
		if _, exists := s[note]; exists {
			continue
		}
		s[note] = struct{}{}
		fmt.Fprintf(stdout, "note: %s\n", note)
	}
}

// runCheck runs both check universes. Outside a Git repository the repo universe
// still applies, while the staged universe is unavailable.
func runCheck(ctx context.Context, root string, stdout io.Writer) error {
	planNotes := planNoteSink{}
	repoCheckErr := runCheckRepoWithPlanNotes(ctx, root, stdout, planNotes)
	_, _, repoErr := awfgit.OpenContaining(root)
	if errors.Is(repoErr, awfgit.ErrNotARepository) {
		fmt.Fprintln(stdout, "note: staged check universe unavailable outside a git repository")
		return repoCheckErr
	}
	if repoErr != nil {
		return errors.Join(repoCheckErr, repoErr)
	}
	return errors.Join(repoCheckErr, runCheckStagedWithPlanNotes(ctx, root, stdout, planNotes))
}
