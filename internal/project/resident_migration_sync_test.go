package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

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

	if _, _, err := migrate.Upgrade(testContext(t), root); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"metrics", "assignments"} {
		if _, err := os.Lstat(filepath.Join(root, ".awf", name)); !os.IsNotExist(err) {
			t.Fatalf("obsolete %s root remains after migration: %v", name, err)
		}
	}
	for _, name := range []string{"efforts", "worktrees", "effort-archive"} {
		path := filepath.Join(root, ".awf", name, "retained", "nested", "resident.go")
		if got, err := os.ReadFile(path); err != nil || string(got) != name {
			t.Fatalf("retained %s resident changed: %q, %v", name, got, err)
		}
	}
	// The standalone memory root is reset, not migrated: generation 22 stops
	// owning it, so the whole root goes with the journaled schema advance.
	if _, err := os.Lstat(filepath.Join(root, ".awf", "memory")); !os.IsNotExist(err) {
		t.Fatalf("standalone memory root survived the schema-22 reset: %v", err)
	}

	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	if _, err := renderAll(p); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"metrics", "assignments"} {
		if _, err := os.Lstat(filepath.Join(root, ".awf", name)); !os.IsNotExist(err) {
			t.Fatalf("obsolete %s root was recreated by sync/render: %v", name, err)
		}
	}
	for _, name := range []string{"efforts", "worktrees", "effort-archive"} {
		path := filepath.Join(root, ".awf", name, "retained", "nested", "resident.go")
		if got, err := os.ReadFile(path); err != nil || string(got) != name {
			t.Fatalf("retained %s resident changed after sync/render: %q, %v", name, got, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(root, ".awf", "memory")); !os.IsNotExist(err) {
		t.Fatalf("sync/render recreated the standalone memory root: %v", err)
	}
}

func TestRetiredPlanResyncDuplicateSelectionsUpgradeAndSync(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {gateCmd: test-gate}\nskills: [reviewing-plan-resync, reviewing-plan-resync]\nagents: []\ntargets: [claude]\n")
	lock := &manifest.Lock{AWFVersion: "0.31.0", SchemaVersion: 37, Files: map[string]manifest.Entry{}, InitializedWithVersion: "0.31.0"}
	if err := lock.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := migrate.Upgrade(testContext(t), root); err != nil {
		t.Fatal(err)
	}
	configBytes, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(configBytes), "reviewing-plan-resync") {
		t.Fatalf("retired duplicate survived upgrade:\n%s", configBytes)
	}
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	applied, changes, err := migrate.Upgrade(testContext(t), root)
	if err != nil || len(applied) != 0 || len(changes) != 0 {
		t.Fatalf("second upgrade = %v, %v, %v", applied, changes, err)
	}
}
