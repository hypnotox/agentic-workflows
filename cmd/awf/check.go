package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/project"
	"golang.org/x/mod/semver"
)

func runCheck(root string, stdout io.Writer) error {
	lockV, binV, ok, err := lockVsBinary(root)
	if err != nil { // coverage-ignore: the driver pre-gates check (Gated) so a corrupt lock hard-errors before runCheck (ADR-0076), and no direct caller passes one; the branch stays so the ahead-note never silently swallows a lock error
		return err
	}
	if ok && semver.Compare(binV, lockV) > 0 {
		fmt.Fprintf(stdout, "note: awf %s is ahead of this project (rendered by %s); run awf sync to re-pin\n",
			strings.TrimPrefix(binV, "v"), strings.TrimPrefix(lockV, "v"))
	}
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	notes, err := p.AdvisoryNotes()
	if err != nil {
		return err
	}
	// Advisories are printed before drift and never feed the failure count —
	// unauthored stub content cannot fail a gated command (ADR-0070).
	// invariant: stub-advisory-nonfailing
	for _, n := range notes {
		fmt.Fprintf(stdout, "note: %s\n", n)
	}
	drift, err := p.Check()
	if err != nil {
		return err
	}
	findings, err := p.CheckInvariants()
	if err != nil {
		return err
	}
	for _, d := range drift {
		fmt.Fprintf(stdout, "  %-14s %s — %s\n", d.Kind, d.Path, d.Detail)
	}
	for _, f := range findings {
		fmt.Fprintf(stdout, "  %-14s %s\n", "invariant", f.Line())
	}
	if len(drift) == 0 && len(findings) == 0 {
		fmt.Fprintln(stdout, "awf check: clean")
		return nil
	}
	return fmt.Errorf("awf check: %d drift(s), %d invariant issue(s)", len(drift), len(findings))
}
