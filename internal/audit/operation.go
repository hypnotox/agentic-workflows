package audit

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

// Outcome is the configured audit result for command rendering and exit mapping.
type Outcome struct {
	Report  presentation.Report
	Failed  bool
	Commits int
}

// RunConfigured builds the configured audit input for one repository and
// returns its audit-owned report for command rendering.
func RunConfigured(ctx context.Context, root string, cfg *config.Config, base, head string) (Outcome, error) {
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
	docsDir := config.DocsDir
	findings, commits, err := Run(ctx, root, base, head, Inputs{
		Settings:       Resolve(config.AuditScopes(cfg.Audit)),
		GeneratedPaths: generated,
		ADRDir:         docsDir + "/decisions",
		DocsDir:        docsDir,
		IndexMd:        docsDir + "/decisions/INDEX.md",
		PlansDir:       docsDir + "/plans",
	})
	if err != nil {
		return Outcome{}, err
	}
	report, err := Report(findings, commits, base, head)
	if err != nil { // coverage-ignore: Run has accepted the grammar-valid range passed from the CLI parser
		return Outcome{}, err
	}
	failed := false
	for _, finding := range findings {
		if finding.Severity == severity.Error {
			failed = true
			break
		}
	}
	return Outcome{Report: report, Failed: failed, Commits: commits}, nil
}
