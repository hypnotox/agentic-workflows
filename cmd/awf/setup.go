package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// runSetup activates the project's git hooks by pointing core.hooksPath at the
// rendered .githooks directory. It is idempotent and safe to re-run. It errors if
// .githooks is absent (run `awf sync` first); if not inside a git repository it
// warns and is a no-op so `awf init` chaining never breaks. The hooks path is
// resolved relative to the repository top level (via go-git), so setup works when
// run from a subdirectory. If the repository's local core.hooksPath is already set
// to a different value (e.g. a husky/lefthook setup in this repo), it refuses
// unless forceHooks is set, rather than silently hijacking it. Only the repo-local
// value is consulted — a user's global core.hooksPath default is not a per-repo
// setup to guard, and awf's local value overrides it for this repo anyway.
// invariant: setup-guards-hookspath
func runSetup(root string, forceHooks bool, stdout, stderr io.Writer) error {
	if _, err := os.Stat(filepath.Join(root, ".githooks")); os.IsNotExist(err) {
		return fmt.Errorf("no .githooks/ in %s — run `awf sync` first", root)
	} else if err != nil { // coverage-ignore: Stat returns a non-NotExist error only on a permission fault that root bypasses
		return err
	}
	repo, top, ok := openWorktree(root)
	if !ok {
		fmt.Fprintln(stderr, "awf setup: not a git repository — skipping hook activation")
		return nil
	}
	hooksPath := awfHooksRel(top, root)
	existing := localHooksPath(repo)
	if existing != "" && existing != hooksPath && !forceHooks {
		return fmt.Errorf("core.hooksPath is already set to %q — awf would override it; "+
			"re-run with --force-hooks to let awf manage hooks, or leave it and skip `awf setup`", existing)
	}
	if err := writeLocalHooksPath(repo, hooksPath); err != nil { // coverage-ignore: SetConfig writes .git/config; fails only on a permission fault that root bypasses
		return fmt.Errorf("set core.hooksPath: %w", err)
	}
	if existing != "" && existing != hooksPath {
		fmt.Fprintf(stdout, "awf setup: replaced existing core.hooksPath %q\n", existing)
	}
	fmt.Fprintf(stdout, "awf setup: git hooks activated (core.hooksPath=%s)\n", hooksPath)
	return nil
}

// awfHooksRel returns awf's .githooks directory expressed relative to the git top
// level. root's symlinks are resolved first so the path matches go-git's
// real-path toplevel (e.g. a /tmp→/private/tmp checkout on macOS); otherwise it is
// the same .githooks the normal repo-root case yields.
func awfHooksRel(top, root string) string {
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	rel, _ := filepath.Rel(top, filepath.Join(root, ".githooks"))
	return rel
}
