package project

import (
	"context"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// Sync renders and writes the project like SyncReport, discarding the backup,
// change, and prune reports - a test-only convenience for the many in-package
// tests that only care whether the sync errors. Production uses SyncReport
// directly (ADR-0063).
func (p *Project) Sync() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	_, found, err := manifest.LoadOptional(lockPath(p))
	if err != nil {
		return err
	}
	if !found {
		_, _, _, err = p.InitializeReport(ctx, InitAuthority{InitializedWithVersion: Version})
		return err
	}
	_, _, _, err = p.SyncReport(ctx)
	return err
}

// RenderAll renders only plan write nodes in deterministic path order - a
// test-only convenience for the many in-package tests that assert over the
// rendered set. Production operations derive their own state once and enter
// through outputPlan, so no production caller remains (ADR-0063, ADR-0180).
func (p *Project) RenderAll() ([]RenderedFile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	op, err := p.OutputPlan(ctx)
	if err != nil {
		return nil, err
	}
	return op.writeFiles(), nil
}
