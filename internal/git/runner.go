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
// pinned to one already-validated repository root: every invocation selects
// that repository with -C and runs under the isolated Git environment, so no
// inherited repository selection, configuration, or credential control can
// redirect it.
type runner struct {
	root string
}

// newRunner pins a runner to root, which the caller has already validated.
func newRunner(root string) runner {
	return runner{root: root}
}

// run invokes native Git with argv against the pinned root and returns stdout.
// A non-zero exit becomes a *CommandError carrying the captured stderr; any
// other failure (a missing binary, a cancelled context) keeps its own cause.
func (r runner) run(ctx context.Context, argv ...string) ([]byte, error) {
	args := append([]string{"-C", r.root}, argv...)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = IsolatedGitEnvironment(os.Environ())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil, &CommandError{Args: args, ExitCode: exit.ExitCode(), Stderr: strings.TrimSpace(stderr.String()), Err: err}
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return stdout, nil
}
