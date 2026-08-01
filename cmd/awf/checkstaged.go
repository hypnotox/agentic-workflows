package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/project"
	"golang.org/x/mod/semver"
)

// runCheckStaged runs the staged transition universe. The commit child is direct-only.
func runCheckStaged(ctx context.Context, root string, stdout io.Writer) error {
	lock, err := stagedLock(ctx, root)
	if err != nil {
		return err
	}
	lockV, binV, ok := lockVsBinaryLock(lock)
	if ok && semver.Compare(binV, lockV) > 0 {
		fmt.Fprintf(stdout, "note: awf %s is ahead of this project (rendered by %s); run awf render to re-pin\n", strings.TrimPrefix(binV, "v"), strings.TrimPrefix(lockV, "v"))
	}
	report, err := project.CheckStagedRoot(ctx, root)
	if err != nil {
		return err
	}
	for _, n := range report.Notes() {
		fmt.Fprintf(stdout, "note: %s\n", n)
	}
	current := report.Findings()
	for _, f := range current {
		fmt.Fprintf(stdout, "  %-14s %s\n", "current-state", f)
	}
	if len(current) == 0 {
		fmt.Fprintln(stdout, "awf check staged: clean")
		return nil
	}
	return fmt.Errorf("awf check staged: %d current-state issue(s)", len(current))
}
