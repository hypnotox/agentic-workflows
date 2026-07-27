// Package worktree manages native-Git effort worktrees.
package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

type Runner func(context.Context, string, ...string) ([]byte, error)

func nativeRunner(ctx context.Context, root string, args ...string) ([]byte, error) {
	fixed := append([]string{"-C", root}, args...)
	cmd := exec.CommandContext(ctx, "git", fixed...)
	cmd.Env = awfgit.IsolatedGitEnvironment(os.Environ())
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil, fmt.Errorf("git %s: %w: %s", strings.Join(fixed, " "), err, strings.TrimSpace(string(exit.Stderr)))
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(fixed, " "), err)
	}
	return out, nil
}

func resolve(ctx context.Context, run Runner, root, revision string) (string, error) {
	out, err := run(ctx, root, "rev-parse", "--verify", revision+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("resolve base %q: %w", revision, err)
	}
	value := strings.TrimSuffix(string(out), "\n")
	if len(value) != 40 && len(value) != 64 {
		return "", fmt.Errorf("resolve base %q: invalid object ID", revision)
	}
	return value, nil
}

var ownedResidentUntracked = regexp.MustCompile(`^\?\? \.awf/(?:efforts/(?:\.lock|\.[0-9a-f-]{36}\.(?:worktree|integration|removal)\.partial|[0-9a-f-]{36}\.json)|worktrees/[0-9a-f-]{36}/?)$`)

func status(ctx context.Context, run Runner, root string) error {
	out, err := run(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return err
	}
	for _, entry := range strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		if entry == "" {
			continue
		}
		// These are the closed, repository-owned resident leaves created by this
		// manager. Everything else below .awf is ordinary untracked content.
		if ownedResidentUntracked.MatchString(entry) {
			continue
		}
		return &RefusalError{Category: "cleanliness", Risk: "checkout has tracked, untracked, or staged changes", Forceable: true}
	}
	return nil
}
func ancestor(ctx context.Context, run Runner, root, older, newer string) (bool, error) {
	_, err := run(ctx, root, "merge-base", "--is-ancestor", older, newer)
	if err == nil {
		return true, nil
	}
	// Git documents exit status 1 as the ordinary "not an ancestor" result.
	// Propagate every other runner fault rather than treating a failed probe as
	// topology evidence.
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}
