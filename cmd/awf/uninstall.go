package main

import (
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// runUninstall removes awf's generated footprint from a project: every rendered
// file recorded in the lock, the directories left empty by their removal, the
// now-stale lock file itself, and the core.hooksPath setting when it points at
// awf's rendered .githooks. It deliberately leaves the authored .awf/ config
// (config.yaml, sidecars, convention parts) in place.
// invariant: uninstall-removes-lock-tracked
func runUninstall(root string, stdout io.Writer) error {
	lockPath := filepath.Join(root, ".awf", "awf.lock")
	lock, err := manifest.Load(lockPath)
	if err != nil {
		return fmt.Errorf("no %s — nothing to uninstall", filepath.Join(".awf", "awf.lock"))
	}

	removed := 0
	dirs := map[string]bool{}
	for path := range lock.Files {
		abs := filepath.Join(root, path)
		if err := os.Remove(abs); err == nil {
			removed++
		}
		for d := filepath.Dir(abs); d != root; d = filepath.Dir(d) {
			dirs[d] = true
		}
	}
	// Remove now-empty directories deepest-first (a child path is always longer
	// than its parent, so descending-length order attempts children first).
	dirList := slices.Collect(maps.Keys(dirs))
	slices.SortFunc(dirList, func(a, b string) int { return len(b) - len(a) })
	for _, d := range dirList {
		_ = os.Remove(d) // removes only if now empty
	}

	if err := os.Remove(lockPath); err != nil { // coverage-ignore: lock was just loaded, so removal fails only on a permission fault root bypasses
		return fmt.Errorf("remove lock: %w", err)
	}
	fmt.Fprintf(stdout, "awf uninstall: removed %d generated file(s) and the lock\n", removed)
	unsetAwfHooks(root, stdout)
	fmt.Fprintln(stdout, "awf uninstall: left the .awf/ config in place (delete it to fully remove)")
	return nil
}

// unsetAwfHooks unsets core.hooksPath when it points at awf's rendered .githooks,
// leaving a foreign value (a non-awf hooks manager) untouched.
func unsetAwfHooks(root string, stdout io.Writer) {
	top := gitToplevel(root)
	if top == "" {
		return
	}
	hooksPath, _ := filepath.Rel(top, filepath.Join(root, ".githooks"))
	if gitConfigGet(top, "core.hooksPath") != hooksPath {
		return
	}
	cmd := exec.Command("git", "config", "--unset", "core.hooksPath")
	cmd.Dir = top
	if err := cmd.Run(); err != nil { // coverage-ignore: --unset of a key we just read back fails only on a permission fault root bypasses
		return
	}
	fmt.Fprintln(stdout, "awf uninstall: unset core.hooksPath")
}
