package project

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"golang.org/x/mod/semver"
)

// invariant: schema-min-version
func TestVersionCoversCurrentSchema(t *testing.T) {
	min, ok := minVersionBySchema[migrate.Current()]
	if !ok {
		t.Fatalf("minVersionBySchema has no entry for schema generation %d; add one alongside the migration (ADR-0049 Decision 4)", migrate.Current())
	}
	if semver.Compare("v"+Version, "v"+min) < 0 {
		t.Errorf("project.Version %s is below the minimum %s for schema generation %d; bump the const (ADR-0049 Decision 4)", Version, min, migrate.Current())
	}
	if migrate.Current() != 13 {
		t.Errorf("migrate.Current() = %d, want 13", migrate.Current())
	}
	if minVersionBySchema[13] != "0.17.0" {
		t.Errorf("minVersionBySchema[13] = %q, want %q", minVersionBySchema[13], "0.17.0")
	}
	if Version != "0.17.0" {
		t.Errorf("Version = %q, want %q", Version, "0.17.0")
	}
}
