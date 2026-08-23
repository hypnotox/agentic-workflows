package project

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestSyncPruneFailureKeepsLockEntry(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(testContext(t), root)
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	const retired = "obsolete/generated.md"
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	lock.Files[retired] = manifest.Entry{}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(retired))
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(path, "resident"), "keep\n")
	if _, _, _, err := syncReportProject(p); err == nil || !strings.Contains(err.Error(), "remove retired output") {
		t.Fatalf("prune error = %v", err)
	}
	lock, err = manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lock.Files[retired]; !ok {
		t.Fatal("failed prune disappeared from the saved lock")
	}
}

func TestSyncPruneReportSkipsAlreadyGoneFile(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(testContext(t), root)
	_ = syncProject(p)
	// Hand-delete the rendered file before the pruning sync: the report must
	// not claim a removal the prune did not perform.
	if err := os.Remove(filepath.Join(root, ".claude/skills/example-tdd/SKILL.md")); err != nil {
		t.Fatal(err)
	}
	noTDD := strings.Replace(sampleYAML, "  - tdd\n", "", 1)
	_ = os.WriteFile(configPath(root), []byte(noTDD), 0o644)
	p2, _ := Open(testContext(t), root)
	_, _, pruned, err := syncReportProject(p2)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(pruned, ".claude/skills/example-tdd/SKILL.md") {
		t.Errorf("already-gone file must not be reported pruned: %v", pruned)
	}
}

func TestSyncReportOpensDistinctResidentRootBeforeTrackedMutation(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := initializeReportProject(p, InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	agents := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(agents, []byte("hand edit\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(agents, 0o640); err != nil {
		t.Fatal(err)
	}
	beforeLock, err := os.ReadFile(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	missingResident := t.TempDir()
	if err := os.Remove(missingResident); err != nil {
		t.Fatal(err)
	}
	p, err = Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	roots := p.roots()
	p = setTestRoots(p, resident.NewRoots(roots.Tracked, missingResident))
	if _, _, _, err := syncReportProject(p); err == nil {
		t.Fatal("sync accepted missing distinct resident root")
	}
	if got, err := os.ReadFile(agents); err != nil || string(got) != "hand edit\n" {
		t.Fatalf("tracked output changed before resident open refusal = %q, %v", got, err)
	}
	assertPerm(t, agents, 0o640)
	if got, err := os.ReadFile(lockFile(root)); err != nil || !reflect.DeepEqual(got, beforeLock) {
		t.Fatalf("lock changed before resident open refusal = %q, %v", got, err)
	}
}

func TestSyncReportReportsContentAndModeOnce(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(testContext(t), root)
	if _, _, _, err := initializeReportProject(p, InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	agents := filepath.Join(root, "AGENTS.md")
	if err := os.Chmod(agents, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agents, []byte("hand edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ = Open(testContext(t), root)
	_, changes, _, err := syncReportProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Change{{Path: "AGENTS.md", Cause: "internal"}}; !reflect.DeepEqual(changes, want) {
		t.Fatalf("changes = %v, want one record for both corrections: %v", changes, want)
	}
	assertPerm(t, agents, 0o644)
}

// invariant: rendering/sync-and-drift:sync-backs-up-foreign (TestSyncReportForeignFinalSymlinkPolicy)
func TestSyncReportForeignFinalSymlinkPolicy(t *testing.T) {
	for _, tc := range []struct {
		name    string
		target  func(t *testing.T, root string) string
		wantErr bool
	}{
		{"in root", func(t *testing.T, root string) string {
			path := filepath.Join(root, "foreign")
			if err := os.WriteFile(path, []byte("foreign bytes\n"), 0o640); err != nil {
				t.Fatal(err)
			}
			return "foreign"
		}, false},
		{"escaping", func(t *testing.T, _ string) string {
			path := filepath.Join(t.TempDir(), "foreign")
			if err := os.WriteFile(path, []byte("outside\n"), 0o640); err != nil {
				t.Fatal(err)
			}
			return path
		}, true},
		{"broken", func(*testing.T, string) string { return "missing" }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffold(t, sampleYAML)
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, _, err := initializeReportProject(p, InitAuthority{InitializedWithVersion: Version}); err != nil {
				t.Fatal(err)
			}
			lock, err := manifest.Load(lockPath(p.Root()))
			if err != nil {
				t.Fatal(err)
			}
			delete(lock.Files, "AGENTS.md")
			if err := lock.Save(lockPath(p.Root())); err != nil {
				t.Fatal(err)
			}
			agents := filepath.Join(root, "AGENTS.md")
			if err := os.Remove(agents); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(tc.target(t, root), agents); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			backups, _, _, err := syncReportProject(p)
			if tc.wantErr {
				if err == nil {
					t.Fatal("foreign unsafe symlink was replaced")
				}
				if _, statErr := os.Lstat(agents); statErr != nil {
					t.Fatalf("unsafe link changed: %v", statErr)
				}
				if _, statErr := os.Stat(agents + ".awf-bak"); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("unsafe link backup = %v", statErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(backups) == 0 || backups[0].Bak != "AGENTS.md.awf-bak" {
				t.Fatalf("backups = %v", backups)
			}
			contents, readErr := os.ReadFile(agents + ".awf-bak")
			info, statErr := os.Stat(agents + ".awf-bak")
			if readErr != nil || statErr != nil || string(contents) != "foreign bytes\n" || info.Mode().Perm() != 0o640 {
				t.Fatalf("backup = %q, %v, %v", contents, info, errors.Join(readErr, statErr))
			}
			info, statErr = os.Lstat(agents)
			if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
				t.Fatalf("foreign link not replaced: %v, %v", info, statErr)
			}
		})
	}
}

func TestSyncReportReplacesManagedFinalSymlinkWithoutTargetAccess(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(testContext(t), root)
	if _, _, _, err := initializeReportProject(p, InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	agents := filepath.Join(root, "AGENTS.md")
	if err := os.Remove(agents); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing-target", agents); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	p, _ = Open(testContext(t), root)
	if _, _, _, err := syncReportProject(p); err != nil {
		t.Fatalf("SyncReport: %v", err)
	}
	info, err := os.Lstat(agents)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("managed symlink = %v, %v", info, err)
	}
}

// TestSyncReportClassifiesChangedOutput stages every provenance cause by
// authoring the prior lock directly - the classification compares the old
// entry against the fresh render, so a tweaked stored hash simulates the
// corresponding real change (an upstream template edit, a config edit, a
// non-hashed input) without mutating the embedded templates.
func TestSyncReportClassifiesChangedOutput(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(testContext(t), root)
	_, changes, pruned, err := initializeReportProject(p, InitAuthority{InitializedWithVersion: Version})
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 || len(pruned) != 0 {
		t.Errorf("first sync has no baseline and must report no changes or prunes, got changes %v, pruned %v", changes, pruned)
	}
	lock, err := manifest.Load(lockPath(p.Root()))
	if err != nil {
		t.Fatal(err)
	}
	mutate := func(path string, f func(e *manifest.Entry)) {
		t.Helper()
		e, ok := lock.Files[path]
		if !ok {
			t.Fatalf("no lock entry for %s; have %v", path, slices.Sorted(maps.Keys(lock.Files)))
		}
		f(&e)
		lock.Files[path] = e
	}
	// Output moved + template hash moved → upstream churn.
	mutate("AGENTS.md", func(e *manifest.Entry) { e.OutputHash = "x"; e.TemplateHash = "x" })
	// Output moved + config hash moved → the project's own inputs.
	mutate(".claude/skills/example-tdd/SKILL.md", func(e *manifest.Entry) { e.OutputHash = "x"; e.ConfigHash = "x" })
	// Both hashes moved.
	mutate("CLAUDE.md", func(e *manifest.Entry) { e.OutputHash = "x"; e.TemplateHash = "x"; e.ConfigHash = "x" })
	// Output moved, real hashes unmoved → a non-hashed input.
	mutate(".awf/efforts/.gitignore", func(e *manifest.Entry) { e.OutputHash = "x" })
	// Output moved on a generated index (no hashes by design) → regenerated.
	mutate("docs/decisions/INDEX.md", func(e *manifest.Entry) { e.OutputHash = "x" })
	// No prior entry → added.
	delete(lock.Files, "docs/workflow.md")
	if err := lock.Save(lockPath(p.Root())); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"AGENTS.md", ".claude/skills/example-tdd/SKILL.md", "CLAUDE.md", ".awf/efforts/.gitignore", "docs/decisions/INDEX.md", "docs/workflow.md"} {
		if err := os.WriteFile(filepath.Join(root, path), []byte("stale output\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p2, _ := Open(testContext(t), root)
	_, changes, _, err = syncReportProject(p2)
	if err != nil {
		t.Fatal(err)
	}
	want := []Change{
		{Path: ".awf/efforts/.gitignore", Cause: "internal"},
		{Path: ".claude/skills/example-tdd/SKILL.md", Cause: "config"},
		{Path: "AGENTS.md", Cause: "template"},
		{Path: "CLAUDE.md", Cause: "template+config"},
		{Path: "docs/decisions/INDEX.md", Cause: "regenerated"},
		{Path: "docs/workflow.md", Cause: "added"},
	}
	if !slices.Equal(changes, want) {
		t.Errorf("changes = %v\nwant %v (path-sorted; untouched files silent)", changes, want)
	}
}

// invariant: rendering/singletons-and-payloads:adr-system-singletons-rendered (TestSyncReportBacksUpForeignIndexNotManaged)
// invariant: rendering/singletons-and-payloads:plain-singleton-via-renderkind (TestSyncReportBacksUpForeignIndexNotManaged)
// invariant: rendering/doc-outputs:working-with-awf-mandatory (TestSyncReportBacksUpForeignIndexNotManaged)
func TestSyncReportBacksUpForeignIndexNotManaged(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	lay := layout(renderInputsForTest(p))
	// Plant a foreign ADR index with hand content before the first sync (no lock yet),
	// so its path is absent from the prior lock and therefore foreign.
	foreign := filepath.Join(root, lay.IndexMd)
	if err := os.MkdirAll(filepath.Dir(foreign), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(foreign, []byte("hand index\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	backups, _, _, err := initializeReportProject(p, InitAuthority{InitializedWithVersion: Version})
	if err != nil {
		t.Fatalf("InitializeReport: %v", err)
	}
	var got *Backup
	for i := range backups {
		if backups[i].Path == lay.IndexMd {
			got = &backups[i]
		}
	}
	// invariant: rendering/sync-and-drift:sync-backs-up-foreign (TestSyncReportBacksUpForeignIndexNotManaged)
	if got == nil {
		t.Fatalf("foreign INDEX.md not backed up; backups=%#v", backups)
	}
	if !got.Index {
		t.Errorf("INDEX.md backup must be flagged Index=true")
	}
	if b, _ := os.ReadFile(filepath.Join(root, got.Bak)); string(b) != "hand index\n" {
		t.Errorf("backup = %q, want original hand content", b)
	}
	// A path recorded in the prior lock is awf-managed: a second sync backs up
	// nothing and prunes nothing.
	again, _, pruned, err := syncReportProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Errorf("re-sync of awf-managed output must not back up, got %#v", again)
	}
	if len(pruned) != 0 {
		t.Errorf("re-sync of awf-managed output must not prune, got %v", pruned)
	}
}

// invariant: rendering/sync-and-drift:sync-mutations-root-confined (TestSyncMutationsStayWithinSelectedRoots)
// invariant: rendering/sync-and-drift:sync-backs-up-foreign (TestSyncMutationsStayWithinSelectedRoots)
func TestSyncMutationsStayWithinSelectedRoots(t *testing.T) {
	root := scaffold(t, sampleYAML)
	residentRoot := t.TempDir()
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	roots := p.roots()
	p = setTestRoots(p, resident.NewRoots(roots.Tracked, residentRoot))
	if _, _, _, err := initializeReportProject(p, InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ root, path string }{
		{root, "AGENTS.md"},
		{residentRoot, ".awf/efforts/.gitignore"},
	} {
		if _, err := os.Stat(filepath.Join(tc.root, tc.path)); err != nil {
			t.Fatalf("selected-root output %s missing: %v", tc.path, err)
		}
	}

	// A foreign ordinary output keeps its bytes, mode, and report through the
	// tracked handle before replacement.
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	delete(lock.Files, "AGENTS.md")
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("foreign\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "AGENTS.md"), 0o640); err != nil {
		t.Fatal(err)
	}
	p, _ = Open(testContext(t), root)
	roots = p.roots()
	roots.Resident = residentRoot
	p = setTestRoots(p, roots)
	backups, _, _, err := syncReportProject(p)
	if err != nil || !slices.Contains(backups, Backup{Path: "AGENTS.md", Bak: "AGENTS.md.awf-bak"}) {
		t.Fatalf("foreign backup = %v, error = %v", backups, err)
	}
	backup := filepath.Join(root, "AGENTS.md.awf-bak")
	if got, err := os.ReadFile(backup); err != nil || string(got) != "foreign\n" {
		t.Fatalf("backup bytes = %q, %v", got, err)
	}
	assertPerm(t, backup, 0o640)

	// A foreign resident output keeps the same lock-relative report while its
	// backup, replacement bytes, and final mode stay under the resident root.
	lock, err = manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	const residentOutput = ".awf/efforts/.gitignore"
	delete(lock.Files, residentOutput)
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	residentFile := filepath.Join(residentRoot, filepath.FromSlash(residentOutput))
	if err := os.WriteFile(residentFile, []byte("resident foreign\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(residentFile, 0o640); err != nil {
		t.Fatal(err)
	}
	p, _ = Open(testContext(t), root)
	roots = p.roots()
	p = setTestRoots(p, resident.NewRoots(roots.Tracked, residentRoot))
	backups, _, _, err = syncReportProject(p)
	wantResidentBackup := Backup{Path: residentOutput, Bak: residentOutput + ".awf-bak"}
	if err != nil || !slices.Contains(backups, wantResidentBackup) {
		t.Fatalf("resident backup = %v, error = %v", backups, err)
	}
	residentBackup := residentFile + ".awf-bak"
	residentBackupBytes, residentBackupErr := os.ReadFile(residentBackup)
	residentOutputBytes, residentOutputErr := os.ReadFile(residentFile)
	if residentBackupErr != nil || residentOutputErr != nil || string(residentBackupBytes) != "resident foreign\n" || string(residentOutputBytes) == "resident foreign\n" {
		t.Fatalf("resident publication = backup %q output %q errors %v", residentBackupBytes, residentOutputBytes, errors.Join(residentBackupErr, residentOutputErr))
	}
	assertPerm(t, residentBackup, 0o640)
	assertPerm(t, residentFile, 0o644)

	// A managed final symlink is pruned as its entry, not by touching its target.
	lock, err = manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	const retired = "obsolete/managed"
	lock.Files[retired] = manifest.Entry{}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "obsolete"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, filepath.FromSlash(retired))); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	p, _ = Open(testContext(t), root)
	roots = p.roots()
	roots.Resident = residentRoot
	p = setTestRoots(p, roots)
	_, _, pruned, err := syncReportProject(p)
	if err != nil || !slices.Contains(pruned, retired) {
		t.Fatalf("managed symlink prune = %v, %v", pruned, err)
	}
	if got, err := os.ReadFile(outside); err != nil || string(got) != "outside\n" {
		t.Fatalf("symlink target changed = %q, %v", got, err)
	}

	// An escaping prune parent refuses at the converted removal path, preserves
	// outside bytes and mode, and leaves the old lock entry for retry.
	lock, err = manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	const escapingPrune = "escape-prune/victim"
	lock.Files[escapingPrune] = manifest.Entry{}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	beforePruneLock, err := os.ReadFile(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	outsidePrune := t.TempDir()
	outsideVictim := filepath.Join(outsidePrune, "victim")
	if err := os.WriteFile(outsideVictim, []byte("outside prune\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsidePrune, filepath.Join(root, "escape-prune")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	p, _ = Open(testContext(t), root)
	roots = p.roots()
	roots.Resident = residentRoot
	p = setTestRoots(p, roots)
	if _, _, _, err := syncReportProject(p); err == nil {
		t.Fatal("sync accepted escaping prune parent")
	}
	if got, err := os.ReadFile(outsideVictim); err != nil || string(got) != "outside prune\n" {
		t.Fatalf("outside prune target changed = %q, %v", got, err)
	}
	assertPerm(t, outsideVictim, 0o600)
	if got, err := os.ReadFile(lockFile(root)); err != nil || !reflect.DeepEqual(got, beforePruneLock) {
		t.Fatalf("lock advanced after failed prune = %q, %v", got, err)
	}
	if err := os.Remove(filepath.Join(root, "escape-prune")); err != nil {
		t.Fatal(err)
	}
	lock, err = manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	delete(lock.Files, escapingPrune)
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}

	// An escaping output parent refuses before replacement or lock
	// advance, preserving outside bytes and modes.
	beforeLock, err := os.ReadFile(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	outsideRoot := t.TempDir()
	sentinel := filepath.Join(outsideRoot, "sentinel")
	if err := os.WriteFile(sentinel, []byte("outside bytes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "docs"), filepath.Join(root, "saved-docs")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideRoot, filepath.Join(root, "docs")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	p, _ = Open(testContext(t), root)
	roots = p.roots()
	roots.Resident = residentRoot
	p = setTestRoots(p, roots)
	if _, _, _, err := syncReportProject(p); err == nil {
		t.Fatal("sync accepted escaping output parent")
	}
	if got, err := os.ReadFile(sentinel); err != nil || string(got) != "outside bytes\n" {
		t.Fatalf("outside output parent changed = %q, %v", got, err)
	}
	assertPerm(t, sentinel, 0o600)
	if got, err := os.ReadFile(lockFile(root)); err != nil || !reflect.DeepEqual(got, beforeLock) {
		t.Fatalf("lock advanced after incomplete output mutation = %q, %v", got, err)
	}
}

func TestSyncLockRefusesEscapingParent(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := initializeReportProject(p, InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Rename(filepath.Join(root, ".awf"), filepath.Join(root, "saved-awf")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".awf")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	// The selected handle refuses the symlinked lock parent before it can load
	// or advance authority; outside receives neither lock bytes nor a mode change.
	if _, _, _, err := syncReportProject(p); err == nil {
		t.Fatal("sync accepted escaping lock parent")
	}
	if _, err := os.Stat(filepath.Join(outside, "awf.lock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outside lock = %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "saved-awf", "awf.lock")); err != nil || !reflect.DeepEqual(got, before) {
		t.Fatalf("saved lock advanced = %q, %v", got, err)
	}
}
