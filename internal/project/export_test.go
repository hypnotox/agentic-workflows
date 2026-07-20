package project

import "github.com/hypnotox/agentic-workflows/internal/manifest"

// Sync renders and writes the project like SyncReport, discarding the backup,
// change, and prune reports - a test-only convenience for the many in-package
// tests that only care whether the sync errors. Production uses SyncReport
// directly (ADR-0063).
func (p *Project) Sync() error {
	_, found, err := manifest.LoadOptional(p.lockPath())
	if err != nil {
		return err
	}
	if !found {
		_, _, _, err = p.InitializeReport(InitAuthority{InitializedWithVersion: Version})
		return err
	}
	_, _, _, err = p.SyncReport()
	return err
}
