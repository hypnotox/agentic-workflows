package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/project"
	"golang.org/x/mod/semver"
)

// runCheckStaged runs the staged transition universe. The commit child is direct-only.
func runCheckStaged(ctx context.Context, root string, stdout io.Writer) error {
	if err := writeStagedAheadNote(ctx, root, stdout); err != nil {
		return err
	}
	err := errors.Join(
		writeStagedState(ctx, root, stdout, false),
		writeStagedDrift(ctx, root, stdout, false),
	)
	if err == nil {
		fmt.Fprintln(stdout, "awf check staged: clean")
	}
	return err
}

func runCheckStagedState(ctx context.Context, root string, stdout io.Writer) error {
	if err := writeStagedAheadNote(ctx, root, stdout); err != nil {
		return err
	}
	return writeStagedState(ctx, root, stdout, true)
}

func runCheckStagedDrift(ctx context.Context, root string, stdout io.Writer) error {
	if err := writeStagedAheadNote(ctx, root, stdout); err != nil {
		return err
	}
	return writeStagedDrift(ctx, root, stdout, true)
}

func writeStagedAheadNote(ctx context.Context, root string, stdout io.Writer) error {
	lock, err := stagedLock(ctx, root)
	if err != nil {
		return err
	}
	lockV, binV, ok := lockVsBinaryLock(lock)
	if ok && semver.Compare(binV, lockV) > 0 {
		fmt.Fprintf(stdout, "note: awf %s is ahead of this project (rendered by %s); run awf render to re-pin\n", strings.TrimPrefix(binV, "v"), strings.TrimPrefix(lockV, "v"))
	}
	return nil
}

func writeStagedState(ctx context.Context, root string, stdout io.Writer, printClean bool) error {
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
		if printClean {
			fmt.Fprintln(stdout, "awf check staged state: clean")
		}
		return nil
	}
	return fmt.Errorf("awf check staged state: %d current-state issue(s)", len(current))
}

func writeStagedDrift(ctx context.Context, root string, stdout io.Writer, printClean bool) error {
	drift, err := project.CheckStagedDriftRoot(ctx, root)
	if err != nil {
		return err
	}
	for _, d := range drift {
		fmt.Fprintf(stdout, "  %-14s %s: %s\n", d.Kind, d.Path, d.Detail)
	}
	if len(drift) == 0 {
		if printClean {
			fmt.Fprintln(stdout, "awf check staged drift: clean")
		}
		return nil
	}
	return fmt.Errorf("awf check staged drift: %d drift(s)", len(drift))
}
