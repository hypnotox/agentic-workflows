package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func snapshotUpgradeFixture(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == filepath.Join(root, ".git") && info.IsDir() {
			return filepath.SkipDir
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

func assertUpgradeFixtureUnchanged(t *testing.T, root string, before map[string][]byte) {
	t.Helper()
	after := snapshotUpgradeFixture(t, root)
	if len(after) != len(before) {
		t.Fatalf("fixture file count after refusal = %d, want %d", len(after), len(before))
	}
	for path, want := range before {
		if got, ok := after[path]; !ok || !bytes.Equal(got, want) {
			t.Fatalf("fixture file %s changed after refusal", path)
		}
	}
}

func preparePublicSyncLaterFailure(t *testing.T, root string) {
	t.Helper()
	if err := os.Chmod(filepath.Join(root, "AGENTS.md"), 0o600); err != nil {
		t.Fatal(err)
	}
	unreadableOutput := filepath.Join(root, "CLAUDE.md")
	if err := os.Remove(unreadableOutput); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(unreadableOutput, 0o755); err != nil {
		t.Fatal(err)
	}
}

// invariant: code-design/outcome-modeling:actionable-outcome-protocol (TestPublicSyncFailureReportsVisiblePathsWithoutRecoveryClaims)
func TestPublicSyncFailureReportsVisiblePathsWithoutRecoveryClaims(t *testing.T) {
	root := scaffoldProject(t)
	preparePublicSyncLaterFailure(t, root)
	var stdout, stderr bytes.Buffer
	if code := runAt(t, root, []string{"awf", "render"}, &stdout, &stderr); code == 0 {
		t.Fatal("render accepted later output failure")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want no false success", stdout.String())
	}
	got := stderr.String()
	for _, want := range []string{"git status --short", "git diff", "rerun awf render"} {
		if !strings.Contains(got, want) {
			t.Errorf("diagnostic missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"recovery", "rollback", "partially committed"} {
		if strings.Contains(strings.ToLower(got), unwanted) {
			t.Errorf("diagnostic contains obsolete %q:\n%s", unwanted, got)
		}
	}
}

// invariant: config/migrations-and-locks:upgrade-gate (TestUpgradeMigrationMapsDirectResult)
func TestUpgradeMigrationMapsDirectResult(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: ex\nintegrationBranch: main\nskills: [brainstorming]\nagents: [grounding-checker]\n")
	if err := (&manifest.Lock{AWFVersion: "0.19.0", SchemaVersion: 36, Files: map[string]manifest.Entry{}}).Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	result, err := upgradeMigration(testContext(t), root)
	if !errors.Is(err, manifest.ErrUnsupportedLiveSource) {
		t.Fatalf("error = %v, want below-floor refusal", err)
	}
	if len(result.Touched) != 0 {
		t.Fatalf("touched = %v, want none", result.Touched)
	}
}

func TestUpgradeLegacyJournalRefusesOnlyUpgrade(t *testing.T) {
	root := scaffoldProject(t)
	marker := filepath.Join(root, ".awf", "current-state-upgrade.journal")
	if err := os.WriteFile(marker, []byte("legacy residue\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := runAt(t, root, []string{"awf", "upgrade"}, &stdout, &stderr); code == 0 {
		t.Fatal("upgrade accepted legacy journal")
	}
	if got := stderr.String(); !strings.Contains(got, "legacy upgrade journal") || !strings.Contains(got, "last journal-capable awf binary or Git") || strings.Contains(got, "--recover") {
		t.Fatalf("upgrade diagnostic = %q", got)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runAt(t, root, []string{"awf", "render"}, &stdout, &stderr); code != 0 {
		t.Fatalf("unrelated render globally blocked: code=%d stderr=%q", code, stderr.String())
	}
}

func TestRemovedUpgradeRecoverFlagIsUnknown(t *testing.T) {
	root := scaffoldProject(t)
	var stdout, stderr bytes.Buffer
	if code := runAt(t, root, []string{"awf", "upgrade", "--recover"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "unknown flag") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

// invariant: tooling/upgrade-runtime:upgrade-partial-progress-visible (TestRunUpgradeAcquiresLeaseBeforeAuthority)
func TestRunUpgradeAcquiresLeaseBeforeAuthority(t *testing.T) {
	root := scaffoldProject(t)
	lease, err := filesystem.AcquireProjectLease(testContext(t), root, awfgit.ProjectResidentRoot(testContext(t), root))
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release() //nolint:errcheck
	if err := os.Remove(config.ConfigPath(root)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	err = runUpgrade(ctx, root, io.Discard)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want lease refusal before partial-authority inspection", err)
	}
}

// invariant: config/migrations-and-locks:lock-atomic-save (TestUpgradeMigrationLeavesOldLockForOneFinalPublisherWrite)
func TestUpgradeMigrationLeavesOldLockForOneFinalPublisherWrite(t *testing.T) {
	root := scaffoldProject(t)
	lockPath := config.LockPath(root)
	lock, err := manifest.Load(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	oldSchema := migrate.Current() - 1
	if oldSchema < migrate.LiveSchemaFloor {
		t.Skip("no supported prior schema")
	}
	lock.SchemaVersion = oldSchema
	if err := lock.Save(lockPath); err != nil {
		t.Fatal(err)
	}
	result, err := upgradeMigration(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Applied) == 0 {
		t.Fatal("expected one supported migration")
	}
	stillOld, err := manifest.Load(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if stillOld.SchemaVersion != oldSchema {
		t.Fatalf("migration wrote intermediate schema lock %d, want old %d", stillOld.SchemaVersion, oldSchema)
	}
	lease, err := filesystem.AcquireProjectLease(testContext(t), root, awfgit.ProjectResidentRoot(testContext(t), root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := upgradeSyncMutationLeased(testContext(t), root, lease); err != nil {
		_ = lease.Release()
		t.Fatal(err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	final, err := manifest.Load(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if final.SchemaVersion != migrate.Current() {
		t.Fatalf("final schema = %d, want %d", final.SchemaVersion, migrate.Current())
	}
}

// invariant: tooling/cli:upgrade-always-syncs (TestRunUpgradeAlreadyCurrentStillSyncs)
func TestRunUpgradeAlreadyCurrentStillSyncs(t *testing.T) {
	root := scaffoldProject(t)
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runUpgrade(testContext(t), root, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "config schema already current") || !strings.Contains(out.String(), "AGENTS.md") {
		t.Fatalf("upgrade output = %q", out.String())
	}
	lock, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if lock.SchemaVersion != migrate.Current() {
		t.Fatalf("schema = %d, want %d", lock.SchemaVersion, migrate.Current())
	}
}

func TestRunUpgradeRefusesPartialAuthority(t *testing.T) {
	root := scaffoldProject(t)
	if err := os.Remove(config.ConfigPath(root)); err != nil {
		t.Fatal(err)
	}
	if err := runUpgrade(testContext(t), root, io.Discard); err == nil {
		t.Fatal("partial authority accepted")
	}
}
