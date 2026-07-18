package main

import (
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

func runAudit(root, rangeArg string, stdout io.Writer) error {
	// The range is required and has no default (ADR-0127 Decision 2): an audit
	// that silently reports over nothing is worse than one that refuses.
	if rangeArg == "" {
		return &usageErr{"awf audit: a range is required: <base> (meaning <base>..HEAD) or <a>..<b>"}
	}
	base, head, err := awfgit.ParseRange(rangeArg, true)
	if err != nil {
		return &usageErr{"awf audit: " + err.Error()}
	}
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	findings, err := p.Audit(base, head)
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
		fmt.Fprintf(stdout, "  %-7s %-22s %s: %s\n", f.Severity, f.Rule, loc, f.Detail)
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
