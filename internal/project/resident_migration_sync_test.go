package project

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func snapshotResidentMigrationFixture(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = contents
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func assertResidentMigrationFixtureUnchanged(t *testing.T, root string, before map[string][]byte) {
	t.Helper()
	after := snapshotResidentMigrationFixture(t, root)
	if len(after) != len(before) {
		t.Fatalf("fixture file count after refusal = %d, want %d: %#v", len(after), len(before), after)
	}
	for path, want := range before {
		if got, ok := after[path]; !ok || !bytes.Equal(got, want) {
			t.Fatalf("fixture file %s after refusal = %q, want byte-identical %q", path, got, want)
		}
	}
}

// Generation 21 removes the obsolete workflow roots and generation 22 resets
// the standalone memory root, while the three roots awf still owns keep every
// dynamic descendant through migration, sync, and render alike.
//
// invariant: rendering/singletons-and-payloads:resident-output-preservation (TestResidentMigrationsPreserveOwnedRootsThroughProjectSync)
func TestResidentMigrationsPreserveOwnedRootsThroughProjectSync(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte("prefix: example\nprofile: full\nintegrationBranch: main\nvars: {gateCmd: test-gate}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock := &manifest.Lock{AWFVersion: "0.24.0", SchemaVersion: 20, Files: map[string]manifest.Entry{}, InitializedWithVersion: "0.24.0"}
	if err := lock.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"metrics", "assignments"} {
		testsupport.WriteFile(t, filepath.Join(root, ".awf", name, "obsolete", "resident"), name)
	}
	for _, name := range []string{"efforts", "memory", "worktrees", "effort-archive"} {
		testsupport.WriteFile(t, filepath.Join(root, ".awf", name, "retained", "nested", "resident.go"), name)
	}

	before := snapshotResidentMigrationFixture(t, root)
	if _, _, err := migrate.Upgrade(testContext(t), root); !errors.Is(err, manifest.ErrUnsupportedLiveSource) {
		t.Fatalf("legacy upgrade = %v, want below-floor refusal", err)
	}
	assertResidentMigrationFixtureUnchanged(t, root, before)
}

func TestRetiredPlanResyncRefusesBelowFloorWithoutMutation(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {gateCmd: test-gate}\nskills: [reviewing-plan-resync, reviewing-plan-resync]\nagents: []\ntargets: [claude]\n")
	lock := &manifest.Lock{AWFVersion: "0.31.0", SchemaVersion: 37, Files: map[string]manifest.Entry{}, InitializedWithVersion: "0.31.0"}
	if err := lock.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	before := snapshotResidentMigrationFixture(t, root)
	if _, _, err := migrate.Upgrade(testContext(t), root); !errors.Is(err, manifest.ErrUnsupportedLiveSource) {
		t.Fatalf("legacy upgrade = %v, want below-floor refusal", err)
	}
	assertResidentMigrationFixtureUnchanged(t, root, before)
}
