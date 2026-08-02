package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/project"
	"golang.org/x/mod/semver"
)

// runCheckRepo runs the repository-universe aggregate and owns its project-level notes.
func runCheckRepo(ctx context.Context, root string, stdout io.Writer) error {
	lockV, binV, ok, err := checkLockVsBinary(root)
	if err != nil {
		return err
	}
	if ok && semver.Compare(binV, lockV) > 0 {
		fmt.Fprintf(stdout, "note: awf %s is ahead of this project (rendered by %s); run awf render to re-pin\n", strings.TrimPrefix(binV, "v"), strings.TrimPrefix(lockV, "v"))
	}
	p, err := project.Open(ctx, root)
	if err != nil {
		return err
	}
	notes, err := p.AdvisoryNotes(ctx)
	if err != nil {
		return err
	}
	for _, n := range notes {
		fmt.Fprintf(stdout, "note: %s\n", n)
	}
	var first error
	if err := runCheckDrift(ctx, root, stdout); err != nil {
		first = err
	}
	if err := runCheckState(ctx, root, stdout); err != nil && first == nil {
		first = err
	}
	if err := runProseGate(ctx, root, stdout); err != nil && first == nil {
		first = err
	}
	if err := runMemoryGate(ctx, root, stdout); err != nil && first == nil {
		first = err
	}
	return first
}

func runCheckDrift(ctx context.Context, root string, stdout io.Writer) error {
	p, err := project.Open(ctx, root)
	if err != nil {
		return err
	}
	drift, err := p.Check(ctx)
	if err != nil {
		return err
	}
	for _, d := range drift {
		fmt.Fprintf(stdout, "  %-14s %s: %s\n", d.Kind, d.Path, d.Detail)
	}
	if len(drift) == 0 {
		fmt.Fprintln(stdout, "awf check repo drift: clean")
		return nil
	}
	return fmt.Errorf("awf check repo drift: %d drift(s)", len(drift))
}

func runCheckState(ctx context.Context, root string, stdout io.Writer) error {
	p, err := project.Open(ctx, root)
	if err != nil {
		return err
	}
	report, err := p.CheckCurrentState(ctx)
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
		fmt.Fprintln(stdout, "awf check repo state: clean")
		return nil
	}
	return fmt.Errorf("awf check repo state: %d current-state issue(s)", len(current))
}

func checkLockVsBinary(root string) (lockV, binV string, ok bool, err error) {
	return lockVsBinary(root)
}
