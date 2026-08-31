package main

import (
	"context"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func runAudit(ctx context.Context, root, rangeArg string, stdout io.Writer) error {
	if rangeArg == "" {
		return &usageErr{"awf audit: a range is required: <base> (meaning <base>..HEAD) or <a>..<b>"}
	}
	base, head, err := awfgit.ParseRange(rangeArg, true)
	if err != nil {
		return &usageErr{"awf audit: " + err.Error()}
	}
	session, err := loadProjectSession(ctx, root)
	if err != nil {
		return err
	}
	outcome, err := audit.RunConfigured(ctx, root, session.Config(), base, head)
	if err != nil {
		return err
	}
	document, err := outcome.Report.Document()
	if err != nil {
		return err
	}
	if err := presentation.Render(stdout, document); err != nil {
		return err
	}
	if outcome.Failed {
		return &producedReportError{fmt.Errorf("awf audit: error-ranked findings over %d commit(s) in %s..%s", outcome.Commits, base, head)}
	}
	return nil
}
