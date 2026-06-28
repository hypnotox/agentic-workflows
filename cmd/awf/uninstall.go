package main

import (
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

// runUninstall removes awf's generated footprint from a project (delegated to
// project.Uninstall: lock-tracked files, the dirs they leave empty, and the lock)
// and unsets core.hooksPath when it points at awf's rendered .githooks. It
// deliberately leaves the authored .awf/ config (config.yaml, sidecars,
// convention parts) in place.
func runUninstall(root string, stdout io.Writer) error {
	removed, err := project.Uninstall(root)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "awf uninstall: removed %d generated file(s) and the lock\n", removed)
	unsetAwfHooks(root, stdout)
	fmt.Fprintln(stdout, "awf uninstall: left the .awf/ config in place (delete it to fully remove)")
	return nil
}

// unsetAwfHooks unsets the repo-local core.hooksPath when it points at awf's
// rendered .githooks, leaving a foreign value (a non-awf hooks manager) — or a
// user's global default — untouched.
func unsetAwfHooks(root string, stdout io.Writer) {
	repo, top, ok := openWorktree(root)
	if !ok {
		return
	}
	if localHooksPath(repo) != awfHooksRel(top, root) {
		return
	}
	if err := writeLocalHooksPath(repo, ""); err != nil { // coverage-ignore: SetConfig writes .git/config; fails only on a permission fault root bypasses
		return
	}
	fmt.Fprintln(stdout, "awf uninstall: unset core.hooksPath")
}
