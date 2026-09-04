package publisher

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

type mutationRecorder struct {
	syncFilesystem
	operations []string
	failPath   string
	fail       error
}

func (f *mutationRecorder) MkdirAll(path string, mode fs.FileMode) error {
	f.operations = append(f.operations, "mkdir "+path)
	return f.syncFilesystem.MkdirAll(path, mode)
}
func (f *mutationRecorder) Chmod(path string, mode fs.FileMode) error {
	f.operations = append(f.operations, "chmod "+path)
	return f.syncFilesystem.Chmod(path, mode)
}
func (f *mutationRecorder) ReplaceExpected(path string, expected *filesystem.ExpectedIdentity, contents []byte, mode fs.FileMode) error {
	f.operations = append(f.operations, "replace "+path)
	if path == f.failPath {
		return f.fail
	}
	return f.syncFilesystem.ReplaceExpected(path, expected, contents, mode)
}
func (f *mutationRecorder) ReplaceExpectedRegularFile(path string, expected *filesystem.ExpectedIdentity, before []byte, beforeMode fs.FileMode, contents []byte, mode fs.FileMode) error {
	f.operations = append(f.operations, "replace "+path)
	if path == f.failPath {
		_ = expected.Release()
		return f.fail
	}
	return f.syncFilesystem.ReplaceExpectedRegularFile(path, expected, before, beforeMode, contents, mode)
}
func (f *mutationRecorder) RemoveExpected(path string, expected *filesystem.ExpectedIdentity) error {
	f.operations = append(f.operations, "remove "+path)
	return f.syncFilesystem.RemoveExpected(path, expected)
}
func (f *mutationRecorder) RemoveExpectedRegularFile(path string, expected *filesystem.ExpectedIdentity, contents []byte, mode fs.FileMode) error {
	f.operations = append(f.operations, "remove "+path)
	return f.syncFilesystem.RemoveExpectedRegularFile(path, expected, contents, mode)
}

type competingCreateFilesystem struct {
	syncFilesystem
	root, path string
	competed   bool
}

func (f *competingCreateFilesystem) ReplaceExpected(path string, expected *filesystem.ExpectedIdentity, contents []byte, mode fs.FileMode) error {
	if path == f.path && expected == nil && !f.competed {
		f.competed = true
		if err := os.WriteFile(filepath.Join(f.root, filepath.FromSlash(path)), []byte("concurrent winner\n"), 0o600); err != nil {
			return err
		}
	}
	return f.syncFilesystem.ReplaceExpected(path, expected, contents, mode)
}

func testSyncPlan(t *testing.T, state *project.Session) (renderInputs, *outputplan.Plan) {
	t.Helper()
	inputs := renderInputsForTest(state)
	plan, err := testPublisher(inputs).Plan()
	if err != nil {
		t.Fatal(err)
	}
	return inputs, &plan
}

func syncWithFilesystems(t *testing.T, state *project.Session, filesystems syncFilesystems) ([]Change, []string, error) {
	t.Helper()
	inputs, plan := testSyncPlan(t, state)
	return syncReportWithPlan(inputs, nil, filesystems, plan, migrate.Current(), true, nil)
}

func openTestFilesystems(t *testing.T, state *project.Session) (syncFilesystems, func()) {
	t.Helper()
	filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(state))
	if err != nil {
		t.Fatal(err)
	}
	return filesystems, closeAll
}

func addLockedOutput(t *testing.T, root, name, contents string, mode fs.FileMode) {
	t.Helper()
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	lock.Files[name] = manifest.Entry{OutputHash: manifest.Hash([]byte(contents)), Mode: uint32(mode.Perm())}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(name)), contents)
	if err := os.Chmod(filepath.Join(root, filepath.FromSlash(name)), mode); err != nil {
		t.Fatal(err)
	}
}

func TestInitializeReportsOutputDirectoriesAndLockAsTouched(t *testing.T) {
	root := scaffold(t, sampleYAML)
	state, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := testPublisher(renderInputsForTest(state)).Initialize(InitAuthority{InitializedWithVersion: Version})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{".awf/awf.lock", ".claude", "AGENTS.md"} {
		if !slices.Contains(result.Touched(), path) {
			t.Fatalf("touched paths %v omit %s", result.Touched(), path)
		}
	}
}

func TestSyncPreflightsEveryDesiredOutputBeforeMutation(t *testing.T) {
	root, state := initializedSampleProject(t)
	agents := filepath.Join(root, "AGENTS.md")
	if err := os.Chmod(agents, 0o600); err != nil {
		t.Fatal(err)
	}
	beforeLock := mustRead(t, lockFile(root))
	bridge := filepath.Join(root, "CLAUDE.md")
	if err := os.Remove(bridge); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(bridge, 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := testPublisher(renderInputsForTest(state)).SyncLeased(testContext(t), nil)
	if err == nil || len(result.Changes()) != 0 || len(result.Pruned()) != 0 {
		t.Fatalf("sync result = %#v, error = %v", result, err)
	}
	if info, statErr := os.Stat(agents); statErr != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("earlier desired output mutated before refusal: %v, %v", info, statErr)
	}
	if got := mustRead(t, lockFile(root)); !bytes.Equal(got, beforeLock) {
		t.Fatal("lock changed after desired-output preflight refusal")
	}
}

func TestPreflightSyncLeasedDoesNotApplyValidatedMutations(t *testing.T) {
	root, state := initializedSampleProject(t)
	agents := filepath.Join(root, "AGENTS.md")
	if err := os.Chmod(agents, 0o600); err != nil {
		t.Fatal(err)
	}
	beforeLock := mustRead(t, lockFile(root))
	if err := testPublisher(renderInputsForTest(state)).PreflightSyncLeased(testContext(t), nil); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(agents); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("preflight mutated output: %v, %v", info, err)
	}
	if got := mustRead(t, lockFile(root)); !bytes.Equal(got, beforeLock) {
		t.Fatal("preflight changed lock")
	}
}

func TestSyncPreflightsEveryRetirementBeforeMutation(t *testing.T) {
	root, state := initializedSampleProject(t)
	agents := filepath.Join(root, "AGENTS.md")
	if err := os.Chmod(agents, 0o600); err != nil {
		t.Fatal(err)
	}
	addLockedOutput(t, root, "retired/output.md", "locked\n", 0o644)
	testsupport.WriteFile(t, filepath.Join(root, "retired/output.md"), "diverged\n")
	beforeLock := mustRead(t, lockFile(root))
	result, err := testPublisher(renderInputsForTest(state)).SyncLeased(testContext(t), nil)
	if err == nil || !strings.Contains(err.Error(), "retired/output.md") || len(result.Changes()) != 0 {
		t.Fatalf("sync result = %#v, error = %v", result, err)
	}
	if info, statErr := os.Stat(agents); statErr != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("desired output mutated before retirement refusal: %v, %v", info, statErr)
	}
	if got := mustRead(t, lockFile(root)); !bytes.Equal(got, beforeLock) {
		t.Fatal("lock changed after retirement preflight refusal")
	}
}

func TestInitializeRefusesDifferingForeignOutputWithoutClobber(t *testing.T) {
	root := scaffold(t, sampleYAML)
	state, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "AGENTS.md")
	testsupport.WriteFile(t, output, "foreign\n")
	result, err := testPublisher(renderInputsForTest(state)).Initialize(InitAuthority{InitializedWithVersion: Version})
	if err == nil || !strings.Contains(err.Error(), "AGENTS.md") || len(result.Changes()) != 0 {
		t.Fatalf("initialize result = %#v, error = %v", result, err)
	}
	if got := mustRead(t, output); string(got) != "foreign\n" {
		t.Fatalf("foreign output changed: %q", got)
	}
	if _, err := os.Stat(output + ".awf-bak"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected safety copy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("later output mutated before refusal: %v", err)
	}
}

func TestInitializeAdoptsExactResidentFile(t *testing.T) {
	root := scaffold(t, sampleYAML)
	state, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	inputs, plan := testSyncPlan(t, state)
	var content string
	for _, output := range plan.Outputs() {
		if output.Path() == "AGENTS.md" {
			content = output.Content()
		}
	}
	if content == "" {
		t.Fatal("AGENTS.md absent from plan")
	}
	testsupport.WriteFile(t, filepath.Join(root, "AGENTS.md"), content)
	result, err := testPublisher(inputs).Initialize(InitAuthority{InitializedWithVersion: Version})
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(result.Changes(), func(change Change) bool { return change.Path == "AGENTS.md" }) {
		t.Fatalf("adopted output reported changed: %#v", result.Changes())
	}
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lock.Files["AGENTS.md"]; !ok {
		t.Fatal("adopted output absent from lock")
	}
}

func TestUnchangedSyncSkipsOutputsAndLock(t *testing.T) {
	_, state := initializedSampleProject(t)
	filesystems, closeAll := openTestFilesystems(t, state)
	defer closeAll()
	recorder := &mutationRecorder{syncFilesystem: filesystems.tracked}
	filesystems.tracked = recorder
	changes, pruned, err := syncWithFilesystems(t, state, filesystems)
	if err != nil || len(changes) != 0 || len(pruned) != 0 {
		t.Fatalf("unchanged sync = changes %v, pruned %v, error %v", changes, pruned, err)
	}
	for _, operation := range recorder.operations {
		if strings.HasPrefix(operation, "replace ") {
			t.Fatalf("unchanged sync replaced a file: %v", recorder.operations)
		}
	}
}

func TestSyncRemovesOnlyExactRetirementWithoutSafetyCopy(t *testing.T) {
	root, state := initializedSampleProject(t)
	const retired = "retired/deep/output.md"
	addLockedOutput(t, root, retired, "locked\n", 0o640)
	result, err := testPublisher(renderInputsForTest(state)).SyncLeased(testContext(t), nil)
	if err != nil || !slices.Contains(result.Pruned(), retired) {
		t.Fatalf("sync result = %#v, error = %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(root, retired)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("retired output remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, retired+".awf-bak")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected safety copy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "retired")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("empty ancestors remain: %v", err)
	}
}

func TestSyncRefusesRetirementModeDivergence(t *testing.T) {
	root, state := initializedSampleProject(t)
	const retired = "retired/output.md"
	addLockedOutput(t, root, retired, "locked\n", 0o644)
	if err := os.Chmod(filepath.Join(root, retired), 0o600); err != nil {
		t.Fatal(err)
	}
	beforeLock := mustRead(t, lockFile(root))
	result, err := testPublisher(renderInputsForTest(state)).SyncLeased(testContext(t), nil)
	if err == nil || len(result.Pruned()) != 0 {
		t.Fatalf("sync result = %#v, error = %v", result, err)
	}
	if _, statErr := os.Stat(filepath.Join(root, retired)); statErr != nil {
		t.Fatal("diverged retired output was removed")
	}
	if got := mustRead(t, lockFile(root)); !bytes.Equal(got, beforeLock) {
		t.Fatal("lock changed after retirement refusal")
	}
}

func TestSyncRefusesFinalSymlinksBeforeMutation(t *testing.T) {
	root, state := initializedSampleProject(t)
	agents := filepath.Join(root, "AGENTS.md")
	if err := os.Remove(agents); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("outside", agents); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	beforeLock := mustRead(t, lockFile(root))
	result, err := testPublisher(renderInputsForTest(state)).SyncLeased(testContext(t), nil)
	if err == nil || !strings.Contains(err.Error(), "AGENTS.md") || len(result.Changes()) != 0 {
		t.Fatalf("sync result = %#v, error = %v", result, err)
	}
	if info, statErr := os.Lstat(agents); statErr != nil || info.Mode()&fs.ModeSymlink == 0 {
		t.Fatalf("final symlink changed: %v, %v", info, statErr)
	}
	if got := mustRead(t, lockFile(root)); !bytes.Equal(got, beforeLock) {
		t.Fatal("lock changed after symlink refusal")
	}
}

func TestSyncWritesLockLast(t *testing.T) {
	root, state := initializedSampleProject(t)
	addLockedOutput(t, root, "retired/output.md", "retired\n", 0o644)
	if err := os.Chmod(filepath.Join(root, "AGENTS.md"), 0o600); err != nil {
		t.Fatal(err)
	}
	filesystems, closeAll := openTestFilesystems(t, state)
	defer closeAll()
	recorder := &mutationRecorder{syncFilesystem: filesystems.tracked}
	filesystems.tracked = recorder
	_, _, err := syncWithFilesystems(t, state, filesystems)
	if err != nil {
		t.Fatal(err)
	}
	if len(recorder.operations) == 0 || recorder.operations[len(recorder.operations)-1] != "replace .awf/awf.lock" {
		t.Fatalf("mutation order = %v", recorder.operations)
	}
}

func TestSyncReturnsVisiblePartialResultAndRerunConverges(t *testing.T) {
	root, state := initializedSampleProject(t)
	inputs, plan := testSyncPlan(t, state)
	var selected []string
	desiredModes := map[string]fs.FileMode{}
	for _, output := range plan.Outputs() {
		if resident.IsResidentPath(output.Path()) {
			continue
		}
		if info, err := os.Stat(filepath.Join(root, output.Path())); err == nil && info.Mode().IsRegular() {
			selected = append(selected, output.Path())
			desiredModes[output.Path()] = 0o644
			if strings.HasPrefix(output.Content(), "#!") {
				desiredModes[output.Path()] = 0o755
			}
		}
		if len(selected) == 2 {
			break
		}
	}
	if len(selected) != 2 {
		t.Fatalf("need two tracked outputs, got %v", selected)
	}
	for _, name := range selected {
		if err := os.Chmod(filepath.Join(root, name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	beforeLock := mustRead(t, lockFile(root))
	filesystems, closeAll := openTestFilesystems(t, state)
	failure := errors.New("injected replacement failure")
	recorder := &mutationRecorder{syncFilesystem: filesystems.tracked, failPath: selected[1], fail: failure}
	filesystems.tracked = recorder
	changes, pruned, err := syncReportWithPlan(inputs, nil, filesystems, plan, migrate.Current(), true, nil)
	closeAll()
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), selected[1]) || len(pruned) != 0 {
		t.Fatalf("partial sync = changes %v, pruned %v, error %v", changes, pruned, err)
	}
	if !slices.ContainsFunc(changes, func(change Change) bool { return change.Path == selected[0] }) || slices.ContainsFunc(changes, func(change Change) bool { return change.Path == selected[1] }) {
		t.Fatalf("partial changes = %v", changes)
	}
	if info, statErr := os.Stat(filepath.Join(root, selected[0])); statErr != nil || info.Mode().Perm() != desiredModes[selected[0]] {
		t.Fatalf("successful first output not visible: %v, %v", info, statErr)
	}
	if info, statErr := os.Stat(filepath.Join(root, selected[1])); statErr != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("failed output changed: %v, %v", info, statErr)
	}
	if got := mustRead(t, lockFile(root)); !bytes.Equal(got, beforeLock) {
		t.Fatal("lock advanced after partial sync")
	}
	result, err := testPublisher(renderInputsForTest(state)).SyncLeased(testContext(t), nil)
	if err != nil || !slices.ContainsFunc(result.Changes(), func(change Change) bool { return change.Path == selected[1] }) {
		t.Fatalf("rerun result = %#v, error = %v", result, err)
	}
	filesystems, closeAll = openTestFilesystems(t, state)
	defer closeAll()
	recorder = &mutationRecorder{syncFilesystem: filesystems.tracked}
	filesystems.tracked = recorder
	changes, pruned, err = syncWithFilesystems(t, state, filesystems)
	if err != nil || len(changes) != 0 || len(pruned) != 0 {
		t.Fatalf("converged rerun = changes %v, pruned %v, error %v", changes, pruned, err)
	}
	for _, operation := range recorder.operations {
		if strings.HasPrefix(operation, "replace ") {
			t.Fatalf("converged rerun replaced a file: %v", recorder.operations)
		}
	}
}

func TestInitializeCreationIsExclusive(t *testing.T) {
	root := scaffold(t, sampleYAML)
	state, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	inputs, plan := testSyncPlan(t, state)
	first := "AGENTS.md"
	if !slices.ContainsFunc(plan.Outputs(), func(output outputplan.Output) bool { return output.Path() == first }) {
		t.Fatal("AGENTS.md absent from plan")
	}
	filesystems, closeAll := openTestFilesystems(t, state)
	defer closeAll()
	filesystems.tracked = &competingCreateFilesystem{syncFilesystem: filesystems.tracked, root: root, path: first}
	changes, pruned, err := syncReportWithPlan(inputs, &InitAuthority{InitializedWithVersion: Version}, filesystems, plan, migrate.Current(), true, nil)
	if err == nil || !strings.Contains(err.Error(), first) || len(pruned) != 0 {
		t.Fatalf("exclusive creation = changes %v, pruned %v, error %v", changes, pruned, err)
	}
	if got := mustRead(t, filepath.Join(root, first)); string(got) != "concurrent winner\n" {
		t.Fatalf("concurrent file was clobbered: %q", got)
	}
}
