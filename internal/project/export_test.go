package project

// Sync renders and writes the project like SyncReport, discarding the backup
// report — a test-only convenience for the many in-package tests that only care
// whether the sync errors. Production uses SyncReport directly (ADR-0063).
func (p *Project) Sync() error {
	_, err := p.SyncReport()
	return err
}
