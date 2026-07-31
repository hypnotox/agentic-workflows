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

func TestOpaqueErrorPreservesOnlyContextIdentity(t *testing.T) {
	if err := opaqueError(context.Canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled identity = %v", err)
	}
	if err := opaqueError(context.DeadlineExceeded); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline identity = %v", err)
	}
	if err := opaqueWrap("operation", context.Canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("wrapped canceled identity = %v", err)
	}
	if err := opaqueWrap("operation", context.DeadlineExceeded); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("wrapped deadline identity = %v", err)
	}
}

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
// invariant: tooling/git-access:isolated-deadlined-native
func TestRunnerRefusesDeadlineLessContext(t *testing.T) {
	_, err := newRunner(t.TempDir()).run(context.Background(), "status")
	if err == nil || !strings.Contains(err.Error(), "without a context deadline") {
		t.Fatalf("deadline-less runner error = %v", err)
	}
}

func TestRunnerExcludesLookupIgnoresHostileGitEnvironmentButHonorsHomeConfig(t *testing.T) {
	root := t.TempDir()
	initNativeRepoForRunner(t, root)
	home := t.TempDir()
	excludes := filepath.Join(home, "global-ignore")
	if err := os.WriteFile(excludes, []byte("ignored\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[core]\n\texcludesfile = "+excludes+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hostile := filepath.Join(t.TempDir(), "hostile-config")
	if err := os.WriteFile(hostile, []byte("[core]\n\texcludesfile = /hostile\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "foreign.git"))
	t.Setenv("GIT_WORK_TREE", t.TempDir())
	t.Setenv("GIT_INDEX_FILE", filepath.Join(t.TempDir(), "index"))
	t.Setenv("GIT_CONFIG_GLOBAL", hostile)
	got := newRunner(root).excludesFileArgs(testContext(t))
	want := []string{"-c", "core.excludesfile=" + excludes}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("ambient user excludes lookup = %#v, want %#v", got, want)
	}

	local := filepath.Join(t.TempDir(), "local-ignore")
	cmd := exec.CommandContext(testContext(t), "git", "-C", root, "config", "core.excludesFile", local)
	cmd.Env = ambientConfigEnvironment(os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("configure local excludes: %v: %s", err, out)
	}
	got = newRunner(root).excludesFileArgs(testContext(t))
	want = []string{"-c", "core.excludesfile=" + local}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("ambient local excludes lookup = %#v, want local precedence %#v", got, want)
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
		cmd.Env = isolatedGitEnvironment(os.Environ())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("configure empty excludes file: %v: %s", err, out)
		}
		if got := newRunner(root).excludesFileArgs(testContext(t)); got != nil {
			t.Fatalf("empty excludes file = %#v, want no override", got)
		}
	})
}

// TestCommandErrorNamesItsContextCause pins the rendered text for the two shapes
// where the exit status alone explains nothing: a killed process reports exit
// -1 with no stderr, so a timed-out or cancelled invocation would otherwise
// render as an unexplained "exit status -1" while its cause was matchable but
// invisible to the human reading the message.
func TestCommandErrorNamesItsContextCause(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want string
	}{
		{name: "deadline", err: context.DeadlineExceeded, want: "timed out"},
		{name: "cancel", err: context.Canceled, want: "canceled"},
		{name: "ordinary exit", err: nil, want: "exit status 1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			failure := &CommandError{Args: []string{"-C", "/repo", "status"}, ExitCode: 1, Err: test.err}
			if got := failure.Error(); !strings.Contains(got, test.want) {
				t.Fatalf("Error() = %q, want it to name %q", got, test.want)
			}
		})
	}
	var nilError *CommandError
	if got := nilError.Error(); got != "git command failed" {
		t.Fatalf("nil receiver Error() = %q", got)
	}
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
	if errors.As(err, &exit) {
		t.Fatal("CommandError leaked its exec cause")
	}

	var nilFailure *CommandError
	if nilFailure.Error() != "git command failed" || nilFailure.Unwrap() != nil {
		t.Fatal("nil *CommandError methods")
	}
}

// TestRunnerNonExitFailureIsOpaque proves a failure that never produced an
// exit status (here, no git binary at all) is not dressed up as a CommandError
// and does not leak the os/exec mechanism identity.
func TestRunnerNonExitFailureIsOpaque(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := newRunner(t.TempDir()).run(testContext(t), "version")

	var failure *CommandError
	if errors.As(err, &failure) {
		t.Fatalf("missing-binary failure reported as a command exit: %v", err)
	}
	if errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("missing-binary failure leaked its exec cause: %T %v", err, err)
	}
	if err == nil || !strings.Contains(err.Error(), "executable file not found") {
		t.Fatalf("missing-binary diagnostic = %v", err)
	}
}

// TestRunnerMidRunContextExpiryKeepsItsContextCause proves a deadline that
// expires while Git is already running does not flatten into the kill signal's
// exit status: the caller can match both the context and the seam CommandError,
// but never the exec mechanism error.
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
	if errors.As(err, &exit) {
		t.Fatal("mid-run expiry CommandError leaked its exec cause")
	}
}

// TestProbeSeparatesAnAnswerFromAFault pins the three outcomes the exit-code
// convention carries. The middle case is the whole point: exit 1 is Git saying
// "no", and reading it as a failure (or reading any other exit as "no") would
// turn a broken repository into a confident answer a destructive operation then
// acts on.
func TestProbeSeparatesAnAnswerFromAFault(t *testing.T) {
	root := t.TempDir()
	initNativeRepoForRunner(t, root)
	native := newRunner(root)

	if answer, err := native.probe(testContext(t), "check-ref-format", "refs/heads/awf/valid"); err != nil || !answer {
		t.Fatalf("exit-zero probe = %v, %v; want a true answer", answer, err)
	}
	if answer, err := native.probe(testContext(t), "check-ref-format", "refs/heads/awf/..invalid"); err != nil || answer {
		t.Fatalf("exit-one probe = %v, %v; want a false answer without an error", answer, err)
	}
	answer, err := native.probe(testContext(t), "rev-parse", "--verify", "no-such-revision")
	if answer {
		t.Fatal("a faulted probe answered true")
	}
	var failure *CommandError
	if !errors.As(err, &failure) || failure.ExitCode == 1 {
		t.Fatalf("faulted probe error = %v, want a *CommandError carrying a non-answer exit code", err)
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
	cmd.Env = isolatedGitEnvironment(os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v: %s", root, err, out)
	}
}
