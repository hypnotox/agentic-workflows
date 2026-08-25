package main

import (
	"context"
	"errors"
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
	_, cfg, _, err := openProjectOperation(ctx, root)
	if err != nil {
		return err
	}
	outcome, err := audit.RunConfigured(ctx, root, cfg, base, head)
	if err != nil {
		return presentAuditRefusal(err)
	}
	document, err := outcome.Report.Document()
	if err != nil { // coverage-ignore: the audit mapping has already validated every shape
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

func presentAuditRefusal(err error) error {
	var horizon *audit.HistoricalHorizonError
	if errors.As(err, &horizon) {
		return fmt.Errorf("%w: audit with a release supporting schemas %d through %d, or restore an in-horizon revision", err, horizon.Floor, horizon.Horizon)
	}
	var partial *audit.PartialHistoricalAuthorityError
	if errors.As(err, &partial) {
		return fmt.Errorf("%w: restore the complete .awf/config.yaml and .awf/awf.lock pair", err)
	}
	return err
}
