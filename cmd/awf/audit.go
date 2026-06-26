package main

import (
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

func runAudit(root, base string, stdout io.Writer) error {
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	findings, err := p.Audit(base)
	if err != nil {
		return err
	}
	errs := 0
	for _, f := range findings {
		if f.Severity == audit.Error {
			errs++
		}
		loc := f.Commit
		if loc == "" {
			loc = "branch"
		}
		fmt.Fprintf(stdout, "  %-7s %-22s %s — %s\n", f.Severity, f.Rule, loc, f.Detail)
	}
	if len(findings) == 0 {
		fmt.Fprintln(stdout, "awf audit: clean")
		return nil
	}
	warns := len(findings) - errs
	if errs == 0 {
		fmt.Fprintf(stdout, "awf audit: %d warning(s), 0 errors\n", warns)
		return nil // warnings never set non-zero exit
	}
	return fmt.Errorf("awf audit: %d error(s), %d warning(s)", errs, warns)
}
