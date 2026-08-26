package publisher

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filepublication"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/projectstate"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

type replacementFaultFilesystem struct {
	syncFilesystem
	path string
	err  error
}

type committedReplacementFaultFilesystem struct {
	syncFilesystem
	path, residue string
	cause         error
}

func (f committedReplacementFaultFilesystem) ReplaceExpected(path string, expected fs.FileInfo, contents []byte, mode fs.FileMode) error {
	if err := f.syncFilesystem.ReplaceExpected(path, expected, contents, mode); err != nil {
		return err
	}
	if path == f.path {
		return &filepublication.CommittedCleanupError{DestinationPath: path, ResiduePath: f.residue, Cause: f.cause}
	}
	return nil
}

func (f replacementFaultFilesystem) Replace(path string, contents []byte, mode fs.FileMode) error {
	if path == f.path {
		return f.err
	}
	return f.syncFilesystem.Replace(path, contents, mode)
}
func (f replacementFaultFilesystem) ReplaceExpected(path string, expected fs.FileInfo, contents []byte, mode fs.FileMode) error {
	if path == f.path {
		return f.err
	}
	return f.syncFilesystem.ReplaceExpected(path, expected, contents, mode)
}

type publicationFaultFilesystem struct {
	syncFilesystem
	err   error
	calls *int
}

func (f publicationFaultFilesystem) Publish(string, []byte, fs.FileMode) error {
	*f.calls++
	return f.err
}

type removalFaultFilesystem struct {
	syncFilesystem
	path string
	err  error
}

type committedRemovalFaultFilesystem struct {
	syncFilesystem
	path, residue string
	cause         error
}

func (f committedRemovalFaultFilesystem) RemoveExpected(path string, expected fs.FileInfo) error {
	if err := f.syncFilesystem.RemoveExpected(path, expected); err != nil {
		return err
	}
	if path == f.path {
		return &filepublication.CommittedCleanupError{DestinationPath: path, ResiduePath: f.residue, Cause: f.cause}
	}
	return nil
}

func (f removalFaultFilesystem) Remove(path string) error {
	if path == f.path {
		return f.err
	}
	return f.syncFilesystem.Remove(path)
}
func (f removalFaultFilesystem) RemoveExpected(path string, expected fs.FileInfo) error {
	if path == f.path {
		return f.err
	}
	return f.syncFilesystem.RemoveExpected(path, expected)
}

type readFaultFilesystem struct {
	syncFilesystem
	err error
}

func (f readFaultFilesystem) Read(string) ([]byte, error) { return nil, f.err }

type readWithModeFaultFilesystem struct {
	syncFilesystem
	path string
	err  error
}

func (f readWithModeFaultFilesystem) ReadWithMode(path string) ([]byte, fs.FileMode, error) {
	if path == f.path {
		return nil, 0, f.err
	}
	return f.syncFilesystem.ReadWithMode(path)
}

type linkInfoFaultFilesystem struct {
	syncFilesystem
	path string
	err  error
}

func (f linkInfoFaultFilesystem) LinkInfo(path string) (fs.FileInfo, error) {
	if f.path == "" || f.path == path {
		return nil, f.err
	}
	return f.syncFilesystem.LinkInfo(path)
}

type chmodFaultFilesystem struct {
	syncFilesystem
	err error
}

func (f chmodFaultFilesystem) Chmod(string, fs.FileMode) error { return f.err }

type recordedChmodFilesystem struct {
	syncFilesystem
	path  string
	calls *int
}

func (f recordedChmodFilesystem) Chmod(path string, mode fs.FileMode) error {
	if path == f.path {
		*f.calls++
	}
	return f.syncFilesystem.Chmod(path, mode)
}

type recordedFilesystem struct {
	syncFilesystem
	replaces *[]string
}

func (f recordedFilesystem) Replace(path string, contents []byte, mode fs.FileMode) error {
	*f.replaces = append(*f.replaces, path)
	return f.syncFilesystem.Replace(path, contents, mode)
}
func (f recordedFilesystem) ReplaceExpected(path string, expected fs.FileInfo, contents []byte, mode fs.FileMode) error {
	*f.replaces = append(*f.replaces, path)
	return f.syncFilesystem.ReplaceExpected(path, expected, contents, mode)
}

type collisionFilesystem struct {
	syncFilesystem
	root     string
	competed bool
}

func (f *collisionFilesystem) Publish(path string, contents []byte, mode fs.FileMode) error {
	if !f.competed {
		f.competed = true
		if err := os.WriteFile(filepath.Join(f.root, path), []byte("concurrent winner"), 0o600); err != nil {
			return err
		}
	}
	return f.syncFilesystem.Publish(path, contents, mode)
}

type blockingPublishFilesystem struct {
	syncFilesystem
	ready   chan<- struct{}
	release <-chan struct{}
	calls   int
	mu      sync.Mutex
}

func (f *blockingPublishFilesystem) Publish(path string, contents []byte, mode fs.FileMode) error {
	f.mu.Lock()
	f.calls++
	call := f.calls
	f.mu.Unlock()
	if call <= 2 {
		f.ready <- struct{}{}
		<-f.release
	}
	return f.syncFilesystem.Publish(path, contents, mode)
}

type swapBeforePublishFilesystem struct {
	syncFilesystem
	root, outside string
	swapped       bool
}

func (f *swapBeforePublishFilesystem) Publish(path string, contents []byte, mode fs.FileMode) error {
	if !f.swapped {
		dir := filepath.Dir(filepath.Join(f.root, filepath.FromSlash(path)))
		if err := os.Rename(dir, dir+"-saved"); err != nil {
			return err
		}
		if err := os.Symlink(f.outside, dir); err != nil {
			return err
		}
		f.swapped = true
	}
	return f.syncFilesystem.Publish(path, contents, mode)
}

type swapAfterPruneFilesystem struct {
	syncFilesystem
	root, outside string
	calls         []string
}

func (f *swapAfterPruneFilesystem) Remove(path string) error {
	f.calls = append(f.calls, path)
	return f.syncFilesystem.Remove(path)
}
func (f *swapAfterPruneFilesystem) RemoveExpected(path string, expected fs.FileInfo) error {
	f.calls = append(f.calls, path)
	err := f.syncFilesystem.RemoveExpected(path, expected)
	if path == "cleanup/child/file" && err == nil {
		dir := filepath.Join(f.root, "cleanup")
		if e := os.Rename(dir, dir+"-saved"); e != nil {
			return e
		}
		if e := os.Symlink(f.outside, dir); e != nil {
			return e
		}
	}
	return err
}

type swapBeforeLockReplaceFilesystem struct {
	syncFilesystem
	root, outside string
	swapped       bool
}

func (f *swapBeforeLockReplaceFilesystem) Replace(path string, contents []byte, mode fs.FileMode) error {
	return f.ReplaceExpected(path, nil, contents, mode)
}
func (f *swapBeforeLockReplaceFilesystem) ReplaceExpected(path string, expected fs.FileInfo, contents []byte, mode fs.FileMode) error {
	if path == ".awf/awf.lock" && !f.swapped {
		if err := os.Rename(filepath.Join(f.root, ".awf"), filepath.Join(f.root, "saved-awf")); err != nil {
			return err
		}
		if err := os.Symlink(f.outside, filepath.Join(f.root, ".awf")); err != nil {
			return err
		}
		f.swapped = true
	}
	return f.syncFilesystem.ReplaceExpected(path, expected, contents, mode)
}

func testSyncPlan(t *testing.T, state *ProjectState) (renderInputs, *outputplan.Plan) {
	t.Helper()
	inputs := renderInputsForTest(state)
	plan, err := testPublisher(inputs).Plan()
	if err != nil {
		t.Fatal(err)
	}
	return inputs, &plan
}
func syncWithFilesystems(t *testing.T, state *ProjectState, filesystems syncFilesystems) ([]Backup, []Change, []string, error) {
	t.Helper()
	inputs, plan := testSyncPlan(t, state)
	backups, changes, pruned, _, err := syncReportWithPlan(inputs, nil, filesystems, plan)
	return backups, changes, pruned, err
}
func assertPerm(t *testing.T, path string, want fs.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != want {
		t.Fatalf("mode %s = %v, %v; want %v", path, info, err, want)
	}
}

func TestBackupFileConfinedReturnsSourceInspectionError(t *testing.T) {
	root := scaffold(t, sampleYAML)
	state, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(state))
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	if _, err := backupFileConfined("missing", filesystems.tracked); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backup missing source error = %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := backupFileConfined("directory", filesystems.tracked); err == nil {
		t.Fatal("backup accepted directory source")
	}
}

// invariant: rendering/sync-and-drift:sync-backs-up-foreign (TestBackupFileRetriesOnlyPublicationCollision)
func TestBackupFileRetriesOnlyPublicationCollision(t *testing.T) {
	root := scaffold(t, sampleYAML)
	const source = "rescue source"
	testsupport.WriteFile(t, filepath.Join(root, "artifact"), source)
	sourceInfo, err := os.Stat(filepath.Join(root, "artifact"))
	if err != nil {
		t.Fatal(err)
	}
	state, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(state))
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	if _, err := backupFileConfined("artifact", &collisionFilesystem{syncFilesystem: filesystems.tracked, root: root}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, want string }{{"artifact.awf-bak", "concurrent winner"}, {"artifact.awf-bak.1", source}} {
		got, err := os.ReadFile(filepath.Join(root, tc.name))
		if err != nil || string(got) != tc.want {
			t.Fatalf("%s = %q, %v", tc.name, got, err)
		}
	}
	info, err := os.Stat(filepath.Join(root, "artifact.awf-bak.1"))
	if err != nil || info.Mode().Perm() != sourceInfo.Mode().Perm() {
		t.Fatalf("rescue mode = %v, %v", info, err)
	}
}

func TestSyncFilesystemsRouteUnchangedPaths(t *testing.T) {
	tracked, residentTree := &readFaultFilesystem{}, &readFaultFilesystem{}
	filesystems := syncFilesystems{tracked: tracked, resident: residentTree}
	for _, tc := range []struct {
		path string
		want syncFilesystem
	}{{"AGENTS.md", tracked}, {".awf/efforts/.gitignore", residentTree}} {
		got, path := filesystems.output(tc.path)
		if got != tc.want || path != tc.path {
			t.Fatalf("output(%q) = %T, %q", tc.path, got, path)
		}
	}
}
func TestOpenSyncFilesystemsComposesDistinctRootsBeforeMutation(t *testing.T) {
	root := scaffold(t, sampleYAML)
	state, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	base := state.OutputState()
	inputs := newRenderInputs(projectstate.NewDerivedWithFacts(base.Root(), resident.NewRoots(root, t.TempDir()), base.Nested(), config.Facts{}, base.Catalog(), base.CompleteCatalog(), base.Targets()), testConfig(state), NewFilesystemReader(root), Version)
	filesystems, closeAll, err := openSyncFilesystems(inputs)
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	if filesystems.tracked == filesystems.resident {
		t.Fatal("distinct roots reused one handle")
	}
	inputs.state = projectstate.NewDerivedWithFacts(base.Root(), resident.NewRoots(root, filepath.Join(root, "missing")), base.Nested(), config.Facts{}, base.Catalog(), base.CompleteCatalog(), base.Targets())
	if _, _, err := openSyncFilesystems(inputs); err == nil {
		t.Fatal("missing resident root opened")
	}
	inputs.state = projectstate.NewDerivedWithFacts(base.Root(), resident.NewRoots(filepath.Join(root, "missing-tracked"), root), base.Nested(), config.Facts{}, base.Catalog(), base.CompleteCatalog(), base.Targets())
	if _, _, err := openSyncFilesystems(inputs); err == nil {
		t.Fatal("missing tracked root opened")
	}
}

func TestSyncFilesystemFailuresPreserveErrorIdentity(t *testing.T) {
	for _, tc := range []struct {
		name string
		wrap func(syncFilesystems, error) syncFilesystems
	}{{"lock read", func(s syncFilesystems, e error) syncFilesystems {
		s.tracked = readFaultFilesystem{s.tracked, e}
		return s
	}}, {"output link info", func(s syncFilesystems, e error) syncFilesystems {
		s.tracked = linkInfoFaultFilesystem{syncFilesystem: s.tracked, err: e}
		return s
	}}, {"resident marker chmod", func(s syncFilesystems, e error) syncFilesystems {
		s.resident = chmodFaultFilesystem{s.resident, e}
		return s
	}}} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffold(t, sampleYAML)
			state, _ := Open(testContext(t), root)
			if err := syncProject(state); err != nil {
				t.Fatal(err)
			}
			filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(state))
			if err != nil {
				t.Fatal(err)
			}
			defer closeAll()
			failure := errors.New(tc.name)
			if _, _, _, err = syncWithFilesystems(t, state, tc.wrap(filesystems, failure)); !errors.Is(err, failure) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

// invariant: rendering/sync-and-drift:local-doc-prune-preserved (TestLocalDocPruneUnreadableSourcePreservesRecoveryAndLock)
func TestLocalDocPruneUnreadableSourcePreservesRecoveryAndLock(t *testing.T) {
	localDocPruneFault(t, "unreadable", func(f syncFilesystem, e error) syncFilesystem {
		return readWithModeFaultFilesystem{syncFilesystem: f, path: "docs/runbooks/incident.md", err: e}
	}, false)
}

// invariant: rendering/sync-and-drift:local-doc-prune-preserved (TestLocalDocPruneFaultsKeepRecoveryAndLock)
func TestLocalDocPruneFaultsKeepRecoveryAndLock(t *testing.T) {
	for _, tc := range []struct {
		name   string
		wrap   func(syncFilesystem, error) syncFilesystem
		backup bool
	}{{"backup publication", func(f syncFilesystem, e error) syncFilesystem { return publicationFaultFilesystem{f, e, new(int)} }, false}, {"inspection", func(f syncFilesystem, e error) syncFilesystem {
		return linkInfoFaultFilesystem{syncFilesystem: f, path: "docs/runbooks/incident.md", err: e}
	}, false}, {"removal after backup", func(f syncFilesystem, e error) syncFilesystem {
		return removalFaultFilesystem{f, "docs/runbooks/incident.md", e}
	}, true}} {
		t.Run(tc.name, func(t *testing.T) { localDocPruneFault(t, tc.name, tc.wrap, tc.backup) })
	}
}
func localDocPruneFault(t *testing.T, name string, wrap func(syncFilesystem, error) syncFilesystem, wantBackup bool) {
	t.Helper()
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nlocalDocs:\n  - name: runbooks/incident\n    title: Incident\n    description: Handle incidents.\n")
	state, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(state); err != nil {
		t.Fatal(err)
	}
	const local = "docs/runbooks/incident.md"
	output := filepath.Join(root, local)
	before, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	testConfig(state).LocalDocs = nil
	filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(state))
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	failure := errors.New(name)
	filesystems.tracked = wrap(filesystems.tracked, failure)
	backups, _, pruned, err := syncWithFilesystems(t, state, filesystems)
	if !errors.Is(err, failure) || len(pruned) != 0 || (!wantBackup && len(backups) != 0) {
		t.Fatalf("sync = backups %v, pruned %v, error %v", backups, pruned, err)
	}
	if got, e := os.ReadFile(output); e != nil || !bytes.Equal(got, before) {
		t.Fatalf("source = %q, %v", got, e)
	}
	if got, e := os.ReadFile(lockFile(root)); e != nil || !bytes.Contains(got, []byte(local)) {
		t.Fatalf("lock = %q, %v", got, e)
	}
	if wantBackup {
		if got, e := os.ReadFile(output + ".awf-bak"); e != nil || !bytes.Equal(got, before) {
			t.Fatalf("recovery = %q, %v", got, e)
		}
	} else if _, e := os.Stat(output + ".awf-bak"); !os.IsNotExist(e) {
		t.Fatalf("unexpected backup: %v", e)
	}
}

// invariant: rendering/sync-and-drift:sync-backs-up-foreign (TestBackupFileConfinedPropagatesPublicationFailure)
func TestBackupFileConfinedPropagatesPublicationFailure(t *testing.T) {
	root := scaffold(t, sampleYAML)
	state, _ := Open(testContext(t), root)
	filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(state))
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	failure := errors.New("publication failed")
	calls := 0
	_, err = backupFileConfined(".awf/config.yaml", publicationFaultFilesystem{filesystems.tracked, failure, &calls})
	if !errors.Is(err, failure) || calls != 1 {
		t.Fatalf("backup error=%v calls=%d", err, calls)
	}
}

// invariant: rendering/sync-and-drift:sync-mutations-root-confined (TestSyncReportsCommittedBackupAndCleanupResidue)
// invariant: rendering/sync-and-drift:sync-backs-up-foreign (TestSyncReportsCommittedBackupAndCleanupResidue)
func TestSyncReportsCommittedBackupAndCleanupResidue(t *testing.T) {
	root := scaffold(t, sampleYAML)
	testsupport.WriteFile(t, filepath.Join(root, "AGENTS.md"), "foreign\n")
	state, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(state))
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	failure := errors.New("cleanup failed")
	committed := &filepublication.CommittedCleanupError{DestinationPath: "AGENTS.md.awf-bak", ResiduePath: ".filepublication-residue.tmp", Cause: failure}
	calls := 0
	filesystems.tracked = publicationFaultFilesystem{syncFilesystem: filesystems.tracked, err: committed, calls: &calls}
	inputs, plan := testSyncPlan(t, state)
	seed := &InitAuthority{InitializedWithVersion: Version}
	backups, _, _, effects, err := syncReportWithPlan(inputs, seed, filesystems, plan)
	if !errors.Is(err, failure) || calls != 1 {
		t.Fatalf("sync error = %v, calls = %d", err, calls)
	}
	if len(backups) != 1 || backups[0].Bak != "AGENTS.md.awf-bak" {
		t.Fatalf("backups = %#v", backups)
	}
	for _, want := range []Effect{{Kind: "backup-created", Path: "AGENTS.md.awf-bak", Recovery: "retain while recovering the render"}, {Kind: "publication-residue", Path: ".filepublication-residue.tmp", Recovery: "remove this temporary residue, then rerun awf render"}} {
		if !slices.Contains(effects, want) {
			t.Fatalf("effects = %#v, want %#v", effects, want)
		}
	}
}

// invariant: rendering/sync-and-drift:sync-mutations-root-confined (TestSyncReportsEveryCommittedPublicationCleanup)
func TestSyncReportsEveryCommittedPublicationCleanup(t *testing.T) {
	const residue = ".awf-publication-residue"
	failure := errors.New("committed cleanup failed")
	assertEffects := func(t *testing.T, effects []Effect, wants ...Effect) {
		t.Helper()
		for _, want := range wants {
			if !slices.Contains(effects, want) {
				t.Fatalf("effects = %#v, want %#v", effects, want)
			}
		}
	}

	t.Run("output replacement", func(t *testing.T) {
		root := scaffold(t, sampleYAML)
		state, _ := Open(testContext(t), root)
		if err := syncProject(state); err != nil {
			t.Fatal(err)
		}
		filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(state))
		if err != nil {
			t.Fatal(err)
		}
		defer closeAll()
		filesystems.tracked = committedReplacementFaultFilesystem{syncFilesystem: filesystems.tracked, path: "AGENTS.md", residue: residue, cause: failure}
		inputs, plan := testSyncPlan(t, state)
		_, _, _, effects, err := syncReportWithPlan(inputs, nil, filesystems, plan)
		if !errors.Is(err, failure) {
			t.Fatalf("sync error = %v", err)
		}
		assertEffects(t, effects,
			Effect{Kind: "output-replaced", Path: "AGENTS.md", Recovery: "rerun awf render to complete authority publication"},
			Effect{Kind: "publication-residue", Path: residue, Recovery: "remove this temporary residue, then rerun awf render"},
		)
	})

	t.Run("prune", func(t *testing.T) {
		root := scaffold(t, sampleYAML)
		state, _ := Open(testContext(t), root)
		if err := syncProject(state); err != nil {
			t.Fatal(err)
		}
		lock, err := manifest.Load(lockFile(root))
		if err != nil {
			t.Fatal(err)
		}
		const retired = "retired/output"
		lock.Files[retired] = manifest.Entry{}
		if err := lock.Save(lockFile(root)); err != nil {
			t.Fatal(err)
		}
		testsupport.WriteFile(t, filepath.Join(root, retired), "retired\n")
		filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(state))
		if err != nil {
			t.Fatal(err)
		}
		defer closeAll()
		filesystems.tracked = committedRemovalFaultFilesystem{syncFilesystem: filesystems.tracked, path: retired, residue: residue, cause: failure}
		inputs, plan := testSyncPlan(t, state)
		_, _, pruned, effects, err := syncReportWithPlan(inputs, nil, filesystems, plan)
		if !errors.Is(err, failure) || !slices.Contains(pruned, retired) {
			t.Fatalf("sync error = %v, pruned = %v", err, pruned)
		}
		assertEffects(t, effects,
			Effect{Kind: "output-removed", Path: retired, Recovery: "rerun awf render to complete pruning and lock publication"},
			Effect{Kind: "publication-residue", Path: residue, Recovery: "remove this temporary residue, then rerun awf render"},
		)
	})

	t.Run("empty directory cleanup", func(t *testing.T) {
		root := scaffold(t, sampleYAML)
		state, _ := Open(testContext(t), root)
		if err := syncProject(state); err != nil {
			t.Fatal(err)
		}
		lock, err := manifest.Load(lockFile(root))
		if err != nil {
			t.Fatal(err)
		}
		const retired, cleanup = "cleanup/child/output", "cleanup/child"
		lock.Files[retired] = manifest.Entry{}
		if err := lock.Save(lockFile(root)); err != nil {
			t.Fatal(err)
		}
		testsupport.WriteFile(t, filepath.Join(root, retired), "retired\n")
		filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(state))
		if err != nil {
			t.Fatal(err)
		}
		defer closeAll()
		filesystems.tracked = committedRemovalFaultFilesystem{syncFilesystem: filesystems.tracked, path: cleanup, residue: residue, cause: failure}
		inputs, plan := testSyncPlan(t, state)
		_, _, _, effects, err := syncReportWithPlan(inputs, nil, filesystems, plan)
		if !errors.Is(err, failure) {
			t.Fatalf("sync error = %v", err)
		}
		assertEffects(t, effects,
			Effect{Kind: "empty-directory-removed", Path: cleanup, Recovery: "rerun awf render"},
			Effect{Kind: "publication-residue", Path: residue, Recovery: "remove this temporary residue, then rerun awf render"},
		)
	})

	t.Run("final lock replacement", func(t *testing.T) {
		root := scaffold(t, sampleYAML)
		state, _ := Open(testContext(t), root)
		if err := syncProject(state); err != nil {
			t.Fatal(err)
		}
		filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(state))
		if err != nil {
			t.Fatal(err)
		}
		defer closeAll()
		filesystems.tracked = committedReplacementFaultFilesystem{syncFilesystem: filesystems.tracked, path: ".awf/awf.lock", residue: residue, cause: failure}
		inputs, plan := testSyncPlan(t, state)
		_, _, _, effects, err := syncReportWithPlan(inputs, nil, filesystems, plan)
		if !errors.Is(err, failure) {
			t.Fatalf("sync error = %v", err)
		}
		assertEffects(t, effects,
			Effect{Kind: "lock-replaced", Path: ".awf/awf.lock", Recovery: "rerun awf render to verify and complete publication"},
			Effect{Kind: "publication-residue", Path: residue, Recovery: "remove this temporary residue, then rerun awf render"},
		)
	})
}

func TestSyncCommittedEffectOrderingIsStable(t *testing.T) {
	root := scaffold(t, sampleYAML)
	state, _ := Open(testContext(t), root)
	if err := syncProject(state); err != nil {
		t.Fatal(err)
	}
	retired := []string{"b/one", "c/deep/three", "a/two"}
	want := []Effect{
		{Kind: "output-removed", Path: "a/two", Recovery: "rerun awf render to complete pruning and lock publication"},
		{Kind: "output-removed", Path: "b/one", Recovery: "rerun awf render to complete pruning and lock publication"},
		{Kind: "output-removed", Path: "c/deep/three", Recovery: "rerun awf render to complete pruning and lock publication"},
		{Kind: "empty-directory-removed", Path: "c/deep", Recovery: "rerun awf render"},
		{Kind: "empty-directory-removed", Path: "a", Recovery: "rerun awf render"},
		{Kind: "empty-directory-removed", Path: "b", Recovery: "rerun awf render"},
		{Kind: "empty-directory-removed", Path: "c", Recovery: "rerun awf render"},
	}
	for iteration := range 8 {
		lock, err := manifest.Load(lockFile(root))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range retired {
			lock.Files[path] = manifest.Entry{}
			testsupport.WriteFile(t, filepath.Join(root, path), "retired\n")
		}
		if err := lock.Save(lockFile(root)); err != nil {
			t.Fatal(err)
		}
		filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(state))
		if err != nil {
			t.Fatal(err)
		}
		inputs, plan := testSyncPlan(t, state)
		_, _, _, effects, syncErr := syncReportWithPlan(inputs, nil, filesystems, plan)
		closeAll()
		if syncErr != nil {
			t.Fatal(syncErr)
		}
		got := make([]Effect, 0, len(want))
		for _, effect := range effects {
			if effect.Kind == "output-removed" || effect.Kind == "empty-directory-removed" {
				got = append(got, effect)
			}
		}
		if !slices.Equal(got, want) {
			t.Fatalf("iteration %d effects = %#v, want %#v", iteration, got, want)
		}
	}
}

func TestSyncReportDoesNotReportOutputWhenReplacementFails(t *testing.T) { syncFailedOutput(t, true) }
func TestSyncReportDoesNotReportOutputWhenWriteFails(t *testing.T)       { syncFailedOutput(t, false) }
func syncFailedOutput(t *testing.T, corruptHash bool) {
	t.Helper()
	root := scaffold(t, sampleYAML)
	state, _ := Open(testContext(t), root)
	if err := syncProject(state); err != nil {
		t.Fatal(err)
	}
	if corruptHash {
		lock, err := manifest.Load(lockFile(root))
		if err != nil {
			t.Fatal(err)
		}
		e := lock.Files["AGENTS.md"]
		e.OutputHash = "different"
		lock.Files["AGENTS.md"] = e
		if err := lock.Save(lockFile(root)); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(root, "AGENTS.md")
	testsupport.WriteFile(t, output, "hand edit\n")
	filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(state))
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	failure := errors.New("replacement failed")
	filesystems.tracked = replacementFaultFilesystem{filesystems.tracked, "AGENTS.md", failure}
	_, changes, _, err := syncWithFilesystems(t, state, filesystems)
	if !errors.Is(err, failure) || len(changes) != 0 {
		t.Fatalf("changes=%v err=%v", changes, err)
	}
	got, e := os.ReadFile(output)
	if e != nil || string(got) != "hand edit\n" {
		t.Fatalf("output=%q %v", got, e)
	}
}

func TestSyncRefusesResidentModeMutationAfterObservationFailure(t *testing.T) {
	root := scaffold(t, sampleYAML)
	state, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(state); err != nil {
		t.Fatal(err)
	}
	filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(state))
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	failure := errors.New("resident mode observation failed")
	chmodCalls := 0
	recorded := recordedChmodFilesystem{syncFilesystem: filesystems.resident, path: ".awf/efforts", calls: &chmodCalls}
	filesystems.resident = linkInfoFaultFilesystem{syncFilesystem: recorded, path: ".awf/efforts", err: failure}
	inputs, plan := testSyncPlan(t, state)
	_, _, _, _, err = syncReportWithPlan(inputs, nil, filesystems, plan)
	if !errors.Is(err, failure) || chmodCalls != 0 {
		t.Fatalf("sync error = %v, chmod calls = %d", err, chmodCalls)
	}
}

// invariant: rendering/sync-and-drift:sync-mutations-root-confined (TestSyncReportsResidentDirectoryModeCorrection)
func TestSyncReportsResidentDirectoryModeCorrection(t *testing.T) {
	root := scaffold(t, sampleYAML)
	state, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(state); err != nil {
		t.Fatal(err)
	}
	residentDir := filepath.Join(root, ".awf", "efforts")
	if err := os.Chmod(residentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := testPublisher(renderInputsForTest(state)).Sync()
	if err != nil {
		t.Fatal(err)
	}
	want := Effect{Kind: "mode-corrected", Path: ".awf/efforts", Recovery: "rerun awf render"}
	if !slices.Contains(result.Effects(), want) {
		t.Fatalf("effects = %#v, want %#v", result.Effects(), want)
	}
	assertPerm(t, residentDir, 0o700)
}

// invariant: rendering/sync-and-drift:sync-mutations-root-confined (TestPublisherSyncRetainsCommittedPartialResultOnLaterFilesystemFailure)
// TestPublisherSyncRetainsCommittedPartialResultOnLaterFilesystemFailure uses
// only the public Publisher.Sync boundary. It makes the first planned output
// need a mode correction, then turns the later bridge output into a directory
// that cannot be atomically replaced. Returning Result{} from Publisher.sync
// when its terminal error is non-nil would lose the asserted committed change.
func TestPublisherSyncRetainsCommittedPartialResultOnLaterFilesystemFailure(t *testing.T) {
	root := scaffold(t, sampleYAML)
	state, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(state); err != nil {
		t.Fatal(err)
	}
	agents := filepath.Join(root, "AGENTS.md")
	if err := os.Chmod(agents, 0o600); err != nil {
		t.Fatal(err)
	}
	bridge := filepath.Join(root, "CLAUDE.md")
	if err := os.Remove(bridge); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(bridge, 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := testPublisher(renderInputsForTest(state)).Sync()
	if err == nil {
		t.Fatal("Sync accepted an unreplaceable later output")
	}
	var partial *PartialError
	if !errors.As(err, &partial) || !slices.Equal(partial.Result.Effects(), result.Effects()) {
		t.Fatalf("error = %v, want typed partial outcome matching returned result", err)
	}
	if len(result.Effects()) == 0 || !slices.ContainsFunc(result.Effects(), func(effect Effect) bool { return effect.Path == "AGENTS.md" && effect.Recovery != "" }) {
		t.Fatalf("effects = %#v, want stable AGENTS.md effect and recovery", result.Effects())
	}
	if got := result.Changes(); len(got) != 1 || got[0] != (Change{Path: "AGENTS.md", Cause: "internal"}) {
		t.Fatalf("changes=%v, want committed AGENTS.md mode correction", got)
	}
	assertPerm(t, agents, 0o644)
}

// invariant: rendering/sync-and-drift:sync-mutations-root-confined (TestSyncBackupPublicationRefusesParentSwap)
// invariant: rendering/sync-and-drift:sync-backs-up-foreign (TestSyncBackupPublicationRefusesParentSwap)
// invariant: rendering/sync-and-drift:local-doc-prune-preserved (TestSyncBackupPublicationRefusesParentSwap)
func TestSyncBackupPublicationRefusesParentSwap(t *testing.T) {
	root := scaffold(t, sampleYAML)
	state, _ := Open(testContext(t), root)
	testsupport.WriteFile(t, filepath.Join(root, "collision/source"), "source bytes\n")
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "sentinel")
	testsupport.WriteFile(t, sentinel, "outside\n")
	if err := os.Chmod(sentinel, 0o600); err != nil {
		t.Fatal(err)
	}
	filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(state))
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	if _, err := backupFileConfined("collision/source", &swapBeforePublishFilesystem{filesystems.tracked, root, outside, false}); err == nil {
		t.Fatal("backup accepted swap")
	}
	if got, e := os.ReadFile(filepath.Join(root, "collision-saved/source")); e != nil || string(got) != "source bytes\n" {
		t.Fatalf("source=%q %v", got, e)
	}
	if _, e := os.Stat(filepath.Join(outside, "source.awf-bak")); !errors.Is(e, os.ErrNotExist) {
		t.Fatalf("outside backup=%v", e)
	}
	if got, err := os.ReadFile(sentinel); err != nil || string(got) != "outside\n" {
		t.Fatalf("outside sentinel = %q, %v", got, err)
	}
	assertPerm(t, sentinel, 0o600)
}

// invariant: rendering/sync-and-drift:sync-mutations-root-confined (TestSyncAncestorCleanupRefusesParentSwap)
func TestSyncAncestorCleanupRefusesParentSwap(t *testing.T) {
	root := scaffold(t, sampleYAML)
	state, _ := Open(testContext(t), root)
	if err := syncProject(state); err != nil {
		t.Fatal(err)
	}
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	const retired = "cleanup/child/file"
	lock.Files[retired] = manifest.Entry{}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, retired), "retired\n")
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "sentinel")
	testsupport.WriteFile(t, sentinel, "outside cleanup\n")
	if err := os.Chmod(sentinel, 0o600); err != nil {
		t.Fatal(err)
	}
	filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(state))
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	swapping := &swapAfterPruneFilesystem{syncFilesystem: filesystems.tracked, root: root, outside: outside}
	filesystems.tracked = swapping
	_, _, pruned, err := syncWithFilesystems(t, state, filesystems)
	if err == nil || !slices.Contains(pruned, retired) {
		t.Fatalf("pruned=%v err=%v, want committed prune and cleanup refusal", pruned, err)
	}
	if !slices.Equal(swapping.calls, []string{retired}) {
		t.Fatalf("calls=%v, want cleanup refusal immediately after parent swap", swapping.calls)
	}
	if got, err := os.ReadFile(sentinel); err != nil || string(got) != "outside cleanup\n" {
		t.Fatalf("outside sentinel = %q, %v", got, err)
	}
	assertPerm(t, sentinel, 0o600)
	updated, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := updated.Files[retired]; !ok {
		t.Fatal("cleanup refusal advanced the old lock")
	}
}

// invariant: config/migrations-and-locks:lock-atomic-save (TestSyncLockSaveRefusesParentSwap)
func TestSyncLockSaveRefusesParentSwap(t *testing.T) {
	root := scaffold(t, sampleYAML)
	state, _ := Open(testContext(t), root)
	if err := syncProject(state); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(state))
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	filesystems.tracked = &swapBeforeLockReplaceFilesystem{filesystems.tracked, root, outside, false}
	if _, _, _, err := syncWithFilesystems(t, state, filesystems); err == nil {
		t.Fatal("sync accepted swap")
	}
	if _, e := os.Stat(filepath.Join(outside, "awf.lock")); !errors.Is(e, os.ErrNotExist) {
		t.Fatalf("outside lock=%v", e)
	}
	if got, e := os.ReadFile(filepath.Join(root, "saved-awf/awf.lock")); e != nil || !bytes.Equal(got, before) {
		t.Fatalf("saved lock=%q %v", got, e)
	}
}

func TestConcurrentBackupsPublishCompleteCopies(t *testing.T) {
	root := scaffold(t, sampleYAML)
	const source = "complete backup source\n"
	testsupport.WriteFile(t, filepath.Join(root, "x"), source)
	sourceInfo, err := os.Stat(filepath.Join(root, "x"))
	if err != nil {
		t.Fatal(err)
	}
	state, _ := Open(testContext(t), root)
	filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(state))
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	filesystem := &blockingPublishFilesystem{syncFilesystem: filesystems.tracked, ready: ready, release: release}
	results := make(chan error, 2)
	for range 2 {
		go func() { _, err := backupFileConfined("x", filesystem); results <- err }()
	}
	<-ready
	<-ready
	close(release)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"x.awf-bak", "x.awf-bak.1"} {
		got, e := os.ReadFile(filepath.Join(root, name))
		if e != nil || string(got) != source {
			t.Fatalf("backup %s=%q %v", name, got, e)
		}
		info, e := os.Stat(filepath.Join(root, name))
		if e != nil || info.Mode().Perm() != sourceInfo.Mode().Perm() {
			t.Fatalf("mode %s=%v %v", name, info, e)
		}
	}
}

func TestSyncPrunesBeforeWritingReplacementLock(t *testing.T) {
	root := scaffold(t, sampleYAML)
	state, _ := Open(testContext(t), root)
	if err := syncProject(state); err != nil {
		t.Fatal(err)
	}
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	lock.Files["retired/output.md"] = manifest.Entry{}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, "retired/output.md"), "obsolete\n")
	filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(state))
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	var replaces []string
	filesystems.tracked = recordedFilesystem{filesystems.tracked, &replaces}
	_, _, pruned, err := syncWithFilesystems(t, state, filesystems)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(pruned, "retired/output.md") || len(replaces) == 0 || replaces[len(replaces)-1] != ".awf/awf.lock" {
		t.Fatalf("pruned=%v sequence=%v", pruned, replaces)
	}
}

func TestSyncRefusesInvalidPermanentLockBeforeMutation(t *testing.T) {
	for _, tc := range []struct {
		name string
		lock string
	}{
		{name: "empty inventory", lock: `{"awfVersion":"0.40.0","schemaVersion":46,"files":{}}`},
		{name: "misspelled inventory", lock: `{"awfVersion":"0.40.0","schemaVersion":46,"fiels":{"x":{}}}`},
		{name: "duplicate inventory entry", lock: `{"awfVersion":"0.40.0","schemaVersion":46,"files":{"x":{},"x":{}}}`},
		{name: "non-local inventory entry", lock: `{"awfVersion":"0.40.0","schemaVersion":46,"files":{"../escape":{},"AGENTS.md":{}}}`},
		{name: "unknown entry field", lock: `{"awfVersion":"0.40.0","schemaVersion":46,"files":{"x":{"typo":true}}}`},
		{name: "trailing document", lock: `{"awfVersion":"0.40.0","schemaVersion":46,"files":{"x":{}}} {}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffold(t, sampleYAML)
			state, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(root, "AGENTS.md")
			testsupport.WriteFile(t, output, "foreign bytes\n")
			testsupport.WriteFile(t, lockFile(root), tc.lock)
			filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(state))
			if err != nil {
				t.Fatal(err)
			}
			defer closeAll()
			backups, changes, pruned, err := syncWithFilesystems(t, state, filesystems)
			if err == nil || len(backups) != 0 || len(changes) != 0 || len(pruned) != 0 {
				t.Fatalf("invalid lock sync = backups=%v changes=%v pruned=%v err=%v", backups, changes, pruned, err)
			}
			if got, readErr := os.ReadFile(output); readErr != nil || string(got) != "foreign bytes\n" {
				t.Fatalf("output after refusal = %q, %v", got, readErr)
			}
			if got, readErr := os.ReadFile(lockFile(root)); readErr != nil || string(got) != tc.lock {
				t.Fatalf("lock after refusal = %q, %v", got, readErr)
			}
			if _, statErr := os.Stat(output + ".awf-bak"); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("backup after refusal = %v", statErr)
			}
		})
	}
}
