package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/project"
	"golang.org/x/mod/semver"
)

func runCheck(root string, staged bool, stdout io.Writer) error {
	lockV, binV, ok, err := checkLockVsBinary(root, staged)
	if err != nil { // coverage-ignore: the driver pre-gates check (Gated) so a corrupt lock hard-errors before runCheck (ADR-0076), and no direct caller passes one; the branch stays so the ahead-note never silently swallows a lock error
		return err
	}
	if ok && semver.Compare(binV, lockV) > 0 {
		fmt.Fprintf(stdout, "note: awf %s is ahead of this project (rendered by %s); run awf sync to re-pin\n",
			strings.TrimPrefix(binV, "v"), strings.TrimPrefix(lockV, "v"))
	}
	if staged {
		return runCheckStaged(root, stdout)
	}
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	notes, err := p.AdvisoryNotes()
	if err != nil {
		return err
	}
	// Advisories are printed before drift and never feed the failure count -
	// unauthored stub content cannot fail a gated command (ADR-0070).
	for _, n := range notes {
		fmt.Fprintf(stdout, "note: %s\n", n)
	}
	drift, err := p.Check()
	if err != nil {
		return err
	}
	report, err := p.CheckCurrentState()
	if err != nil {
		return err
	}
	// Coverage/fan-out warnings ride the same non-failing note: channel; only
	// error-severity coverage and the static handshake findings fail (ADR-0134).
	for _, n := range report.Notes() {
		fmt.Fprintf(stdout, "note: %s\n", n)
	}
	for _, d := range drift {
		fmt.Fprintf(stdout, "  %-14s %s: %s\n", d.Kind, d.Path, d.Detail)
	}
	current := report.Findings()
	for _, f := range current {
		fmt.Fprintf(stdout, "  %-14s %s\n", "current-state", f)
	}
	if len(drift) == 0 && len(current) == 0 {
		fmt.Fprintln(stdout, "awf check: clean")
		return nil
	}
	return fmt.Errorf("awf check: %d drift(s), %d current-state issue(s)", len(drift), len(current))
}

func checkLockVsBinary(root string, staged bool) (lockV, binV string, ok bool, err error) {
	if !staged {
		return lockVsBinary(root)
	}
	lock, err := stagedLock(root)
	if err != nil {
		return "", "", false, err
	}
	lockV, binV, ok = lockVsBinaryLock(lock)
	return lockV, binV, ok, nil
}

// runCheckStaged validates the staged HEAD-to-index current-state transition and
// the index coverage (ADR-0135). It skips the working-tree drift oracle: a
// pre-commit hook validates the exact slice about to land, not the working tree.
func runCheckStaged(root string, stdout io.Writer) error {
	report, err := project.CheckStagedRoot(root)
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
		fmt.Fprintln(stdout, "awf check --staged: clean")
		return nil
	}
	return fmt.Errorf("awf check --staged: %d current-state issue(s)", len(current))
}
