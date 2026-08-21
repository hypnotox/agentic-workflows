package main

import (
	"context"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

func runAudit(ctx context.Context, root, rangeArg string, stdout io.Writer) error {
	if rangeArg == "" {
		return &usageErr{"awf audit: a range is required: <base> (meaning <base>..HEAD) or <a>..<b>"}
	}
	base, head, err := awfgit.ParseRange(rangeArg, true)
	if err != nil {
		return &usageErr{"awf audit: " + err.Error()}
	}
	_, cfg, _, err := openProjectOperation(ctx, root)
	if err != nil {
		return err
	}
	findings, commits, err := project.Audit(root, cfg, ctx, base, head)
	if err != nil {
		return err
	}
	report, err := audit.Report(findings, commits, base, head)
	if err != nil { // coverage-ignore: audit owns fixed grammar-valid semantic fields
		return err
	}
	document, err := report.Document()
	if err != nil { // coverage-ignore: the audit mapping has already validated every shape
		return err
	}
	if err := presentation.Render(stdout, document); err != nil {
		return err
	}
	if report.Status == "failed" {
		return &producedReportError{fmt.Errorf("awf audit: error-ranked findings over %d commit(s) in %s..%s", commits, base, head)}
	}
	return nil
}
