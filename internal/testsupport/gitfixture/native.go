package gitfixture

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The native lane runs the git binary directly, for the repository states
// go-git cannot express: a registered worktree, an orphan branch, an
// in-progress merge, and a non-default object format. Every invocation runs
// under nativeEnvironment so a fixture never reads or writes the developer's
// git configuration.

// InitNativeAt creates a git repository at root with the default object format,
// creating root when it does not exist yet.
func InitNativeAt(t *testing.T, root string) Fixture {
	t.Helper()
	return InitNativeObjectFormat(t, root, "")
}

// InitNativeObjectFormat creates a git repository at root with the named object
// format, skipping the test when the installed Git cannot provide it. An empty
// or "sha1" format uses the installed default. The repository carries the
// fixture identity locally, so operations the code under test performs can
// commit without a global configuration.
func InitNativeObjectFormat(t *testing.T, root, format string) Fixture {
	t.Helper()
	explicit := format != "" && format != "sha1"
	args := []string{"init"}
	if explicit {
		args = append(args, "--object-format="+format)
	}
	// init runs without -C because root need not exist yet.
	output, err := runGit("", append(args, root)...)
	if err != nil && explicit { // coverage-ignore: reached only where the installed Git lacks the requested object format
		t.Skipf("installed Git lacks %s repositories: %v: %s", format, err, output)
	}
	if err != nil { // coverage-ignore: init into a writable fixture directory fails only on a permission fault a test cannot trigger
		t.Fatalf("git init: %v: %s", err, output)
	}
	f := Fixture{root: root}
	nativeConfig(t, f, "user.name", authorName)
	nativeConfig(t, f, "user.email", authorEmail)
	return f
}

// nativeConfig sets a repository-local configuration value.
func nativeConfig(t *testing.T, f Fixture, key, value string) {
	t.Helper()
	mustNative(t, f, "config", key, value)
}

// NativeAdd stages the named paths.
func NativeAdd(t *testing.T, f Fixture, paths ...string) {
	t.Helper()
	mustNative(t, f, append([]string{"add", "--"}, paths...)...)
}

// NativeAddAllExcept stages every change below the repository root except the
// given pathspecs, so a fixture can commit its own files while leaving a
// managed worktree root untouched.
func NativeAddAllExcept(t *testing.T, f Fixture, exclude ...string) {
	t.Helper()
	args := []string{"add", "-A", "--", "."}
	for _, path := range exclude {
		args = append(args, ":(exclude)"+path)
	}
	mustNative(t, f, args...)
}

// NativeCommit commits the staged tree with the fixture identity.
func NativeCommit(t *testing.T, f Fixture, msg string) {
	t.Helper()
	mustNative(t, f, "-c", "user.name="+authorName, "-c", "user.email="+authorEmail, "commit", "-m", msg)
}

// NativeRevParse resolves a revision to its hex object id.
func NativeRevParse(t *testing.T, f Fixture, rev string) string {
	t.Helper()
	return mustNative(t, f, "rev-parse", rev)
}

// NativeGitPath resolves a path inside the repository's git directory, such as
// the MERGE_HEAD marker of an in-progress merge.
func NativeGitPath(t *testing.T, f Fixture, name string) string {
	t.Helper()
	return mustNative(t, f, "rev-parse", "--git-path", name)
}

// NativeRevisionExists reports whether a revision resolves, covering both a
// branch reference and a pseudo-reference such as MERGE_HEAD. Only exit 1 is an
// answer: --verify --quiet reserves it for "does not resolve" and every other
// nonzero exit is a fault, so reading them all as absent would turn a broken
// fixture into a confident negative and pass a must-be-absent assertion for the
// wrong reason.
func NativeRevisionExists(t *testing.T, f Fixture, rev string) bool {
	t.Helper()
	output, err := runNative(f, "rev-parse", "--verify", "--quiet", rev)
	if err == nil {
		return true
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false
	}
	// coverage-ignore: rev-parse --verify --quiet answers a resolvable revision with 0 and an unresolvable one with 1; any other exit needs a broken fixture or a missing git binary, which reds the suite by other means
	t.Fatalf("git rev-parse --verify --quiet %s: %v\n%s", rev, err, output)
	return false
}

// NativeBranch creates a branch at HEAD.
func NativeBranch(t *testing.T, f Fixture, name string) {
	t.Helper()
	mustNative(t, f, "branch", name)
}

// NativeBranchForce points a branch at target, creating or moving it.
func NativeBranchForce(t *testing.T, f Fixture, name, target string) {
	t.Helper()
	mustNative(t, f, "branch", "-f", name, target)
}

// NativeCheckout checks out an existing reference.
func NativeCheckout(t *testing.T, f Fixture, ref string) {
	t.Helper()
	mustNative(t, f, "checkout", ref)
}

// NativeCheckoutOrphan starts an orphan branch, the parentless history go-git
// cannot check out.
func NativeCheckoutOrphan(t *testing.T, f Fixture, name string) {
	t.Helper()
	mustNative(t, f, "checkout", "--orphan", name)
}

// NativeRemoveAll removes every tracked path from the index and worktree,
// emptying an orphan branch before it takes its own content.
func NativeRemoveAll(t *testing.T, f Fixture) {
	t.Helper()
	mustNative(t, f, "rm", "-rf", ".")
}

// NativeMergeAbort abandons an in-progress merge.
func NativeMergeAbort(t *testing.T, f Fixture) {
	t.Helper()
	mustNative(t, f, "merge", "--abort")
}

// NativeWriteTree writes the current index out as a tree and reports its hash,
// the cheapest fingerprint of an index a test must prove unchanged.
func NativeWriteTree(t *testing.T, f Fixture) string {
	t.Helper()
	return mustNative(t, f, "write-tree")
}

// NativeWorktreeAdd registers a linked worktree at path on a new branch.
func NativeWorktreeAdd(t *testing.T, f Fixture, path, branch string) {
	t.Helper()
	mustNative(t, f, "worktree", "add", "-b", branch, path)
}

// NativeWorktreeAddDetached registers a linked worktree at path with a detached
// HEAD at rev.
func NativeWorktreeAddDetached(t *testing.T, f Fixture, path, rev string) {
	t.Helper()
	mustNative(t, f, "worktree", "add", "--detach", path, rev)
}

// NativeWorktreeRemove unregisters the linked worktree at path.
func NativeWorktreeRemove(t *testing.T, f Fixture, path string) {
	t.Helper()
	mustNative(t, f, "worktree", "remove", path)
}

// mustNative runs an invocation the caller asserts succeeds and returns its
// trimmed output.
func mustNative(t *testing.T, f Fixture, args ...string) string {
	t.Helper()
	output, err := runNative(f, args...)
	if err != nil { // coverage-ignore: an invocation the caller asserts succeeds fails only on a broken fixture, which reds the suite by other means
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return output
}

// runNative runs git pinned to the fixture root, returning trimmed combined
// output.
func runNative(f Fixture, args ...string) (string, error) {
	return runGit(f.root, args...)
}

// runGit runs git under the isolated environment, pinned to root when one is
// given, and returns trimmed combined output.
func runGit(root string, args ...string) (string, error) {
	if root != "" {
		args = append([]string{"-C", root}, args...)
	}
	command := exec.Command("git", args...)
	command.Env = nativeEnvironment(os.Environ())
	output, err := command.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

// nativeEnvironment strips every inherited git control variable and pins the
// settings that keep a fixture invocation off the developer's machine: no
// global or system configuration, no terminal or credential prompt. It repeats
// internal/git's construction rather than importing it, because testsupport
// carries zero internal dependencies
// (tooling/quality-gates:testsupport-zero-internal-deps).
func nativeEnvironment(inherited []string) []string {
	filtered := make([]string, 0, len(inherited)+6)
	for _, entry := range inherited {
		key, _, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(key)
		if strings.HasPrefix(upper, "GIT_") || upper == "GCM_INTERACTIVE" || upper == "SSH_ASKPASS" {
			continue
		}
		filtered = append(filtered, entry)
	}
	return append(filtered,
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=true",
		"SSH_ASKPASS=true",
		"GCM_INTERACTIVE=Never",
	)
}
