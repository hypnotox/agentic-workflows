package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runSetup activates the project's git hooks by pointing core.hooksPath at the
// rendered .githooks directory. It is idempotent and safe to re-run. It errors if
// .githooks is absent (run `awf sync` first); if not inside a git repository it
// warns and is a no-op so `awf init` chaining never breaks. The hooks path is
// resolved relative to the repository top level (via git rev-parse), so setup
// works when run from a subdirectory. If core.hooksPath is already set to a
// different value (e.g. a husky/lefthook setup), it refuses unless forceHooks is
// set, rather than silently hijacking it.
// invariant: setup-guards-hookspath
func runSetup(root string, forceHooks bool, stdout, stderr io.Writer) error {
	if _, err := os.Stat(filepath.Join(root, ".githooks")); os.IsNotExist(err) {
		return fmt.Errorf("no .githooks/ in %s — run `awf sync` first", root)
	} else if err != nil { // coverage-ignore: Stat returns a non-NotExist error only on a permission fault that root bypasses
		return err
	}
	top := gitToplevel(root)
	if top == "" {
		fmt.Fprintln(stderr, "awf setup: not a git repository — skipping hook activation")
		return nil
	}
	hooksPath, err := filepath.Rel(top, filepath.Join(root, ".githooks"))
	if err != nil { // coverage-ignore: root is inside top (rev-parse succeeded), so Rel cannot fail
		return err
	}
	existing := gitConfigGet(top, "core.hooksPath")
	if existing != "" && existing != hooksPath && !forceHooks {
		return fmt.Errorf("core.hooksPath is already set to %q — awf would override it; "+
			"re-run with --force-hooks to let awf manage hooks, or leave it and skip `awf setup`", existing)
	}
	cmd := exec.Command("git", "config", "core.hooksPath", hooksPath)
	cmd.Dir = top
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil { // coverage-ignore: git config write fails only on a permission fault that root bypasses
		return fmt.Errorf("git config core.hooksPath: %w", err)
	}
	if existing != "" && existing != hooksPath {
		fmt.Fprintf(stdout, "awf setup: replaced existing core.hooksPath %q\n", existing)
	}
	fmt.Fprintf(stdout, "awf setup: git hooks activated (core.hooksPath=%s)\n", hooksPath)
	return nil
}

// gitToplevel returns the absolute path of the git work-tree root containing dir,
// or "" if dir is not inside a git repository.
func gitToplevel(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitConfigGet returns the trimmed value of a git config key resolved from dir,
// or "" if it is unset (git exits non-zero with empty output for an absent key).
func gitConfigGet(dir, key string) string {
	cmd := exec.Command("git", "config", "--get", key)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
