package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CommandError reports a native Git invocation that exited non-zero. It carries
// the exact arguments, the exit code, and the captured stderr so a caller can
// match the failure with errors.As and report what Git itself said, without the
// caller ever touching os/exec.
type CommandError struct {
	Args     []string
	ExitCode int
	Stderr   string
	Err      error
}

func (e *CommandError) Error() string {
	if e == nil {
		return "git command failed"
	}
	message := "git " + strings.Join(e.Args, " ")
	message += fmt.Sprintf(": exit status %d", e.ExitCode)
	if e.Stderr != "" {
		message += ": " + e.Stderr
	}
	return message
}

func (e *CommandError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// runner is the package's only native-Git subprocess boundary. One runner is
// pinned to one repository root: every invocation selects that repository with
// -C and runs under the isolated Git environment, so no inherited repository
// selection, configuration, or credential control can redirect it. Validating
// root is the caller's obligation, not the runner's: ResolveControlRoots
// discharges it today, and the Phase 3 Open(root) handle closes the remaining
// construction sites.
type runner struct {
	root string
}

// newRunner pins a runner to root. The caller is responsible for validating
// root; see the runner type comment for which sites discharge that obligation.
func newRunner(root string) runner {
	return runner{root: root}
}

// run invokes native Git with argv against the pinned root and returns stdout.
// A context carrying no deadline is refused before anything is spawned: a git
// blocked on a stale index.lock or a credential prompt would otherwise hang awf
// indefinitely, including inside the pre-commit hook, so the deadline is a
// structural requirement of the seam rather than a per-call courtesy.
// A non-zero exit becomes a *CommandError carrying the captured stderr; any
// other failure (a missing binary, a context cancelled before launch) keeps its
// own cause. A context that expires or is cancelled mid-run reaches the caller
// as a *CommandError whose chain still carries the context cause, because
// exec's Wait reports only the kill signal's exit status.
func (r runner) run(ctx context.Context, argv ...string) ([]byte, error) {
	args := append([]string{"-C", r.root}, argv...)
	if !hasDeadline(ctx) {
		return nil, fmt.Errorf("git %s: refusing to run without a context deadline; the caller must bound this invocation", strings.Join(args, " "))
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = IsolatedGitEnvironment(os.Environ())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			cause := err
			if ctxErr := ctx.Err(); ctxErr != nil {
				cause = errors.Join(err, ctxErr)
			}
			return nil, &CommandError{Args: args, ExitCode: exit.ExitCode(), Stderr: strings.TrimSpace(stderr.String()), Err: cause}
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return stdout, nil
}

// hasDeadline reports whether ctx bounds how long a native Git invocation may
// take. It is the single test behind the runner's refusal.
func hasDeadline(ctx context.Context) bool {
	_, ok := ctx.Deadline()
	return ok
}

// excludesFileArgs replays the effective core.excludesFile as a per-invocation
// -c override. The isolated environment deliberately strips the user and system
// config, which also strips the ignore rules real Git would apply, and a
// working-tree oracle that silently narrows its ignore universe reports files
// the developer's own `git status` does not. So the effective value is read
// once from the ambient environment, before isolation, and replayed on the
// invocation; every other isolation property (repository selection,
// credentials, prompts) is untouched. An unset value, an unreadable config, or
// a context the runner would refuse anyway contributes no override, matching
// git's own treatment of a missing optional ignore source.
func (r runner) excludesFileArgs(ctx context.Context) []string {
	if !hasDeadline(ctx) {
		return nil
	}
	cmd := exec.CommandContext(ctx, "git", "-C", r.root, "config", "--get", "core.excludesFile")
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	value := strings.TrimSpace(string(out))
	if value == "" {
		return nil
	}
	return []string{"-c", "core.excludesfile=" + value}
}
