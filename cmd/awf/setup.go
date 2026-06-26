package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// runSetup activates the project's git hooks by pointing core.hooksPath at
// .githooks. It is idempotent and safe to re-run. It errors if .githooks is
// absent (the user must run `awf sync` first); if the directory is not inside
// a git repository it warns and is a no-op so `awf init` chaining never breaks.
func runSetup(root string, stdout, stderr io.Writer) error {
	if _, err := os.Stat(filepath.Join(root, ".githooks")); os.IsNotExist(err) {
		return fmt.Errorf("no .githooks/ in %s — run `awf sync` first", root)
	} else if err != nil { // coverage-ignore: Stat returns a non-NotExist error only on a permission fault that root bypasses
		return err
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); os.IsNotExist(err) {
		fmt.Fprintln(stderr, "awf setup: not a git repository — skipping hook activation")
		return nil
	}
	cmd := exec.Command("git", "config", "core.hooksPath", ".githooks")
	cmd.Dir = root
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git config core.hooksPath: %w", err)
	}
	fmt.Fprintln(stdout, "awf setup: git hooks activated (core.hooksPath=.githooks)")
	return nil
}
