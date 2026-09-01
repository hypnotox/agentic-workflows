package audit

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// Outcome is the configured audit result for command rendering and exit mapping.
type Outcome struct {
	Report  presentation.Report
	Failed  bool
	Commits int
}

// RunHistorical builds the fixed-policy audit input for one repository and
// returns its audit-owned report for command rendering.
func RunHistorical(ctx context.Context, root, base, head string) (Outcome, error) {
	generated := map[string]bool{}
	lock, _, err := manifest.LoadOptional(config.LockPath(root))
	if err != nil {
		return Outcome{}, err
	}
	if lock != nil {
		for path := range lock.Files {
			generated[path] = true
		}
	}
	findings, commits, err := Run(ctx, root, base, head, Inputs{
		Settings:       historicalSettings(),
		GeneratedPaths: generated,
		DocsDir:        config.DocsDir,
	})
	if err != nil {
		return Outcome{}, err
	}
	report, failed, err := reportOutcome(findings, commits, base, head)
	if err != nil {
		return Outcome{}, err
	}
	return Outcome{Report: report, Failed: failed, Commits: commits}, nil
}
