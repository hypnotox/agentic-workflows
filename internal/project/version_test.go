package project

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"golang.org/x/mod/semver"
)

// invariant: config/migrations-and-locks:schema-min-version
func TestVersionCoversCurrentSchema(t *testing.T) {
	min, ok := minVersionBySchema[migrate.Current()]
	if !ok {
		t.Fatalf("minVersionBySchema has no entry for schema generation %d; add one alongside the migration (ADR-0049 Decision 4)", migrate.Current())
	}
	if semver.Compare("v"+Version, "v"+min) < 0 {
		t.Errorf("project.Version %s is below the minimum %s for schema generation %d; bump the const (ADR-0049 Decision 4)", Version, min, migrate.Current())
	}
	if migrate.Current() != 19 {
		t.Errorf("migrate.Current() = %d, want 19", migrate.Current())
	}
	if minVersionBySchema[18] != "0.22.0" {
		t.Errorf("minVersionBySchema[18] = %q, want %q", minVersionBySchema[18], "0.22.0")
	}
	// Generation 19 (ADR-0159's rename migration) landed after 0.22.0 shipped, so
	// it raises the floor to the next release rather than reusing a released
	// version a published binary would not actually support.
	if minVersionBySchema[19] != "0.23.0" {
		t.Errorf("minVersionBySchema[19] = %q, want %q", minVersionBySchema[19], "0.23.0")
	}
	if Version != "0.23.0" {
		t.Errorf("Version = %q, want %q", Version, "0.23.0")
	}
	if !BridgeTrancheComplete {
		t.Error("bridge tranche must be complete now that Plans 1 and 2 have both landed")
	}
}
