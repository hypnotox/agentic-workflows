package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
)

// invariant: config/migrations-and-locks:schema-min-version (TestSchemaMinimumVersionAuthority)
func TestSchemaMinimumVersionAuthority(t *testing.T) {
	for schema, minimum := range minVersionBySchema {
		if err := ValidateSchemaMinimumVersion(schema, minimum); err != nil {
			t.Errorf("schema %d at minimum %s: %v", schema, minimum, err)
		}
	}
	// Derived from the registry rather than a literal generation: the loop above
	// compares each minimum against itself and so can never fail, and a
	// hard-coded generation stops naming the current one the moment a migration
	// registers. Pinning migrate.Current() keeps this assertion pointed at the
	// generation the claim is about, on every future bump.
	if got := minVersionBySchema[migrate.Current()]; got != Version {
		t.Fatalf("generation-%d minimum version = %q, want %s", migrate.Current(), got, Version)
	}
	if got := minVersionBySchema[38]; got != "0.31.0" {
		t.Fatalf("generation-38 minimum version = %q, want 0.31.0", got)
	}
	if got := minVersionBySchema[20]; got != "0.24.0" {
		t.Fatalf("generation-20 minimum version = %q, want 0.24.0", got)
	}
	if err := ValidateSchemaMinimumVersion(20, "0.23.0"); err == nil || !strings.Contains(err.Error(), "requires awf 0.24.0") {
		t.Fatalf("generation-20 older binary error = %v", err)
	}
	// A binary that predates the unified-effort-resident generation must refuse
	// a tree that has already advanced to it.
	if err := ValidateSchemaMinimumVersion(22, "0.25.0"); err == nil || !strings.Contains(err.Error(), "requires awf 0.26.0") {
		t.Fatalf("generation-22 older binary error = %v", err)
	}
	if err := ValidateSchemaMinimumVersion(migrate.Current()+1, Version); err == nil || !strings.Contains(err.Error(), "no minimum") {
		t.Fatalf("unmapped schema error = %v", err)
	}
}

// invariant: config/migrations-and-locks:archive-root-upgrade-boundary (TestArchiveRootUpgradeBoundary)
func TestArchiveRootUpgradeBoundary(t *testing.T) {
	root := scaffold(t, "prefix: archive\nintegrationBranch: main\n")
	lock := &manifest.Lock{AWFVersion: Version, SchemaVersion: migrate.Current() - 1, Files: map[string]manifest.Entry{}}
	if err := lock.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	if state, _, err := migrate.GateState(root); err != nil || state != "gate" {
		t.Fatalf("older generation gate = %q, %v; want gate", state, err)
	}
	if _, _, err := migrate.Upgrade(testContext(t), root); err != nil {
		t.Fatal(err)
	}
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, ".awf", "effort-archive", ".gitignore")
	want := "# " + bannerText + "\n*\n!.gitignore\n"
	if got, err := os.ReadFile(marker); err != nil || string(got) != want {
		t.Fatalf("upgraded marker = %q, %v; want %q", got, err, want)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != want {
		t.Fatalf("repaired marker = %q, %v; want %q", got, err, want)
	}
}
