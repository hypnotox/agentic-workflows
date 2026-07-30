package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The runner is the package's only subprocess boundary, so its contract is
// pinned here rather than through any one entrypoint that happens to use it.
// These cases are serial: they rewrite the process environment with t.Setenv.

// TestRunnerIgnoresInheritedRepositorySelection proves the isolation is
// unconditional: an ambient GIT_DIR, GIT_WORK_TREE, GIT_INDEX_FILE, or
// GIT_CONFIG_GLOBAL pointing anywhere hostile cannot redirect an invocation away
// from the root the runner was pinned to.
func TestRunnerIgnoresInheritedRepositorySelection(t *testing.T) {
	pinned := filepath.Join(t.TempDir(), "pinned checkout")
	initNativeRepoForRunner(t, pinned)
	foreign := filepath.Join(t.TempDir(), "foreign checkout")
	initNativeRepoForRunner(t, foreign)

	hostileGlobal := filepath.Join(t.TempDir(), "hostile-global-config")
	if err := os.WriteFile(hostileGlobal, []byte("[core]\n\tbare = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_DIR", filepath.Join(foreign, ".git"))
	t.Setenv("GIT_WORK_TREE", foreign)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(t.TempDir(), "hostile-index"))
	t.Setenv("GIT_CONFIG_GLOBAL", hostileGlobal)

	output, err := newRunner(pinned).run(testContext(t), "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatalf("isolated invocation under a hostile environment: %v", err)
	}
	got, err := filepath.EvalSymlinks(strings.TrimSpace(string(output)))
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(pinned)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("inherited selection redirected the runner to %q, want the pinned root %q", got, want)
	}
}

// TestRunnerFailureCarriesCommandIdentityAndStderr proves a non-zero exit
// reaches the caller as a matchable *CommandError carrying what Git said, so no
// consumer needs os/exec to learn why an invocation failed.
func TestRunnerRefusesDeadlineLessContext(t *testing.T) {
	_, err := newRunner(t.TempDir()).run(context.Background(), "status")
	if err == nil || !strings.Contains(err.Error(), "without a context deadline") {
		t.Fatalf("deadline-less runner error = %v", err)
	}
}

func TestRunnerExcludesFileArgsTreatsUnavailableAndEmptyConfigAsNoOverride(t *testing.T) {
	runner := newRunner(t.TempDir())
	if got := runner.excludesFileArgs(context.Background()); got != nil {
		t.Fatalf("deadline-less excludes lookup = %#v, want no override", got)
	}
	t.Run("unavailable git", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if got := runner.excludesFileArgs(testContext(t)); got != nil {
			t.Fatalf("unavailable config lookup = %#v, want no override", got)
		}
	})
	t.Run("empty value", func(t *testing.T) {
		root := t.TempDir()
		initNativeRepoForRunner(t, root)
		cmd := exec.CommandContext(testContext(t), "git", "-C", root, "config", "core.excludesFile", "")
		cmd.Env = IsolatedGitEnvironment(os.Environ())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("configure empty excludes file: %v: %s", err, out)
		}
		if got := newRunner(root).excludesFileArgs(testContext(t)); got != nil {
			t.Fatalf("empty excludes file = %#v, want no override", got)
		}
	})
}

func TestRunnerFailureCarriesCommandIdentityAndStderr(t *testing.T) {
	root := t.TempDir()
	_, err := newRunner(root).run(testContext(t), "rev-parse", "--show-toplevel")

	var failure *CommandError
	if !errors.As(err, &failure) {
		t.Fatalf("failing invocation error = %T %v, want *CommandError", err, err)
	}
	if failure.ExitCode <= 0 {
		t.Fatalf("exit code = %d, want a non-zero exit status", failure.ExitCode)
	}
	if failure.Stderr == "" {
		t.Fatal("failing invocation dropped Git's stderr")
	}
	if len(failure.Args) < 2 || failure.Args[0] != "-C" || failure.Args[1] != root {
		t.Fatalf("recorded args = %#v, want the pinned root selection first", failure.Args)
	}
	if !strings.Contains(failure.Error(), root) || !strings.Contains(failure.Error(), failure.Stderr) {
		t.Fatalf("error text %q dropped the root or the stderr", failure.Error())
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatal("CommandError did not unwrap to its exec cause")
	}

	var nilFailure *CommandError
	if nilFailure.Error() != "git command failed" || nilFailure.Unwrap() != nil {
		t.Fatal("nil *CommandError methods")
	}
}

// TestRunnerNonExitFailureKeepsItsOwnCause proves a failure that never produced
// an exit status (here, no git binary at all) is not dressed up as a
// CommandError: its own cause survives for errors.Is.
func TestRunnerNonExitFailureKeepsItsOwnCause(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := newRunner(t.TempDir()).run(testContext(t), "version")

	var failure *CommandError
	if errors.As(err, &failure) {
		t.Fatalf("missing-binary failure reported as a command exit: %v", err)
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("missing-binary failure lost its cause: %T %v", err, err)
	}
}

// TestRunnerMidRunContextExpiryKeepsItsContextCause proves a deadline that
// expires while Git is already running does not flatten into the kill signal's
// exit status: exec's Wait prefers the ExitError, so the runner rejoins the
// context cause and the caller can still match both the context and the command.
func TestRunnerMidRunContextExpiryKeepsItsContextCause(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake native Git fixture requires a POSIX script")
	}
	bin := t.TempDir()
	// exec replaces the shell, so the deadline kill reaches the sleeping process
	// itself rather than leaving a grandchild holding the captured stdout pipe.
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\nexec sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	root := t.TempDir()
	ctx, cancel := context.WithTimeout(testContext(t), 50*time.Millisecond)
	defer cancel()
	_, err := newRunner(root).run(ctx, "worktree", "list", "--porcelain")

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("mid-run expiry error = %T %v, want a chain carrying context.DeadlineExceeded", err, err)
	}
	var failure *CommandError
	if !errors.As(err, &failure) {
		t.Fatalf("mid-run expiry error = %T %v, want a matchable *CommandError", err, err)
	}
	if len(failure.Args) < 2 || failure.Args[0] != "-C" || failure.Args[1] != root {
		t.Fatalf("recorded args = %#v, want the pinned root selection first", failure.Args)
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatal("mid-run expiry CommandError dropped its exec cause")
	}
}

func initNativeRepoForRunner(t *testing.T, root string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("native Git fixture is exercised on POSIX")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(testContext(t), "git", "-C", root, "init", "--quiet")
	cmd.Env = IsolatedGitEnvironment(os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v: %s", root, err, out)
	}
}
