package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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

	output, err := newRunner(pinned).run(t.Context(), "rev-parse", "--show-toplevel")
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
func TestRunnerFailureCarriesCommandIdentityAndStderr(t *testing.T) {
	root := t.TempDir()
	_, err := newRunner(root).run(t.Context(), "rev-parse", "--show-toplevel")

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
	_, err := newRunner(t.TempDir()).run(t.Context(), "version")

	var failure *CommandError
	if errors.As(err, &failure) {
		t.Fatalf("missing-binary failure reported as a command exit: %v", err)
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("missing-binary failure lost its cause: %T %v", err, err)
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
	cmd := exec.CommandContext(t.Context(), "git", "-C", root, "init", "--quiet")
	cmd.Env = IsolatedGitEnvironment(os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v: %s", root, err, out)
	}
}
