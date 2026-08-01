package testsupport

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func safeTestTempManager(t *testing.T, root string, now time.Time) *testTempManager {
	t.Helper()
	m, err := newTestTempManager(root, func() time.Time { return now }, osTestTempFS(), func(_ string, info fs.FileInfo) error {
		if info.Mode().Perm()&0o077 != 0 {
			return errors.New("open")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func mkdirHome(t *testing.T, root, name string, mod time.Time) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTestTempManagerConstruction(t *testing.T) {
	files := osTestTempFS()
	if _, err := newTestTempManager("", time.Now, files, func(string, fs.FileInfo) error { return nil }); err != nil {
		t.Fatalf("empty root must be validated by ensureRoot: %v", err)
	}
	if _, err := newTestTempManager(t.TempDir(), nil, files, func(string, fs.FileInfo) error { return nil }); err == nil {
		t.Fatal("nil clock accepted")
	}
	if _, err := newTestTempManager(t.TempDir(), time.Now, files, nil); err == nil {
		t.Fatal("nil validator accepted")
	}
	for _, tc := range []struct {
		name  string
		clear func(*testTempFS)
	}{
		{"mkdir", func(files *testTempFS) { files.mkdir = nil }},
		{"mkdirTemp", func(files *testTempFS) { files.mkdirTemp = nil }},
		{"lstat", func(files *testTempFS) { files.lstat = nil }},
		{"readDir", func(files *testTempFS) { files.readDir = nil }},
		{"walkDir", func(files *testTempFS) { files.walkDir = nil }},
		{"removeAll", func(files *testTempFS) { files.removeAll = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := osTestTempFS()
			tc.clear(&files)
			if _, err := newTestTempManager(t.TempDir(), time.Now, files, func(string, fs.FileInfo) error { return nil }); err == nil {
				t.Fatal("nil filesystem operation accepted")
			}
		})
	}
}

func TestTestTempEnsureRootRejectsUnsafeRoots(t *testing.T) {
	base := t.TempDir()
	now := time.Now()
	for _, tc := range []struct {
		name, root string
		setup      func(string)
	}{
		{"empty", "", nil}, {"relative", "relative", nil}, {"unclean", base + string(filepath.Separator) + "child" + string(filepath.Separator) + ".." + string(filepath.Separator) + "root", nil},
		{"filesystem root", string(filepath.Separator), nil},
		{"file", filepath.Join(base, "file"), func(p string) { _ = os.WriteFile(p, nil, 0o600) }},
		{"symlink", filepath.Join(base, "link"), func(p string) { _ = os.Symlink(base, p) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(tc.root)
			}
			m := safeTestTempManager(t, tc.root, now)
			if err := m.ensureRoot(); err == nil {
				t.Fatal("unsafe root accepted")
			}
		})
	}
	root := filepath.Join(base, "open")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := safeTestTempManager(t, root, now).ensureRoot(); err == nil {
		t.Fatal("group-accessible root accepted")
	}
	foreign := safeTestTempManager(t, filepath.Join(base, "foreign"), now)
	foreign.validate = func(string, fs.FileInfo) error { return errors.New("foreign owner") }
	if err := foreign.ensureRoot(); err == nil {
		t.Fatal("foreign root accepted")
	}
}

func TestTestTempEnsureRootRefusesSymlinkBeforeValidation(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "link")
	if err := os.Symlink(base, root); err != nil {
		t.Fatal(err)
	}
	m, err := newTestTempManager(root, time.Now, osTestTempFS(), func(string, fs.FileInfo) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ensureRoot(); err == nil {
		t.Fatal("root symlink reached validator")
	}
}

func TestTestTempEnsureRootAcceptsExistingAndCreateRace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	m := safeTestTempManager(t, root, time.Now())
	if err := m.ensureRoot(); err != nil {
		t.Fatal(err)
	}
	files := osTestTempFS()
	files.mkdir = func(string, fs.FileMode) error { return fs.ErrExist }
	m, err := newTestTempManager(root, time.Now, files, func(string, fs.FileInfo) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ensureRoot(); err != nil {
		t.Fatalf("ErrExist race: %v", err)
	}
}

func TestCanonicalTestHomeRejectsNonDecimalSuffix(t *testing.T) {
	for _, name := range []string{"home-", "home-abc", "home-12a", "home-12-"} {
		if canonicalTestHome(name) {
			t.Fatalf("noncanonical name accepted: %q", name)
		}
	}
	if !canonicalTestHome("home-123") {
		t.Fatal("decimal name rejected")
	}
}

func TestTestTempAllocateDirectCanonicalHome(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	m := safeTestTempManager(t, root, time.Now())
	home, err := m.allocate()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(home) != root || !canonicalTestHome(filepath.Base(home)) {
		t.Fatalf("home = %q", home)
	}
	files := osTestTempFS()
	files.mkdirTemp = func(string, string) (string, error) { return filepath.Join(t.TempDir(), "home-1"), nil }
	bad, err := newTestTempManager(root, time.Now, files, func(string, fs.FileInfo) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bad.allocate(); err == nil {
		t.Fatal("escaped allocation accepted")
	}
}

func TestCleanupStaleBoundaryAndPreservation(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	root := filepath.Join(t.TempDir(), "root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	equal := mkdirHome(t, root, "home-1", now.Add(-24*time.Hour))
	newer := mkdirHome(t, root, "home-2", now.Add(-24*time.Hour+time.Nanosecond))
	old := mkdirHome(t, root, "home-3", now.Add(-24*time.Hour-time.Nanosecond))
	for _, name := range []string{"home-", "home-abc", "other"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(old, filepath.Join(root, "home-4")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "other", "home-9"), 0o700); err != nil {
		t.Fatal(err)
	}
	result, err := safeTestTempManager(t, root, now).cleanup(CleanupStale)
	if err == nil {
		t.Fatal("unsafe canonical symlink must be reported")
	}
	if result.homes != 1 {
		t.Fatalf("removed %d homes, want 1", result.homes)
	}
	for _, path := range []string{equal, newer, filepath.Join(root, "home-"), filepath.Join(root, "home-abc"), filepath.Join(root, "home-4"), filepath.Join(root, "other", "home-9")} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("preserved %s: %v", path, err)
		}
	}
	if _, err := os.Lstat(old); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("old home remains: %v", err)
	}
}

func TestCleanupRejectsCanonicalRegularFile(t *testing.T) {
	now := time.Now()
	root := filepath.Join(t.TempDir(), "root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "home-5")
	if err := os.WriteFile(path, []byte("not a home directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, now.Add(-48*time.Hour), now.Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	result, err := safeTestTempManager(t, root, now).cleanup(CleanupStale)
	if err == nil || !strings.Contains(err.Error(), path) || result.homes != 0 {
		t.Fatalf("cleanup result=%+v err=%v", result, err)
	}
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("canonical regular file was not preserved: %v", err)
	}
}

func TestCleanupAllPreservesNoncanonicalEntries(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	canonical := mkdirHome(t, root, "home-1", time.Now())
	for _, name := range []string{"home-", "home-abc", "other"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	unrelatedFile := filepath.Join(root, "unrelated-file")
	if err := os.WriteFile(unrelatedFile, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	unrelatedLink := filepath.Join(root, "unrelated-link")
	if err := os.Symlink(canonical, unrelatedLink); err != nil {
		t.Fatal(err)
	}

	result, err := safeTestTempManager(t, root, time.Now()).cleanup(CleanupAll)
	if err != nil || result.homes != 1 {
		t.Fatalf("cleanup result=%+v err=%v", result, err)
	}
	for _, path := range []string{
		filepath.Join(root, "home-"),
		filepath.Join(root, "home-abc"),
		filepath.Join(root, "other"),
		unrelatedFile,
		unrelatedLink,
	} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("noncanonical entry %s was not preserved: %v", path, err)
		}
	}
}

func TestCleanupAllAccountsOnlySuccessfulRegularFiles(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	a := mkdirHome(t, root, "home-3", time.Now())
	b := mkdirHome(t, root, "home-1", time.Now())
	c := mkdirHome(t, root, "home-2", time.Now())
	if err := os.WriteFile(filepath.Join(b, "blocked"), []byte("xx"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a, "a"), []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(a, "dir"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a", filepath.Join(a, "link")); err != nil {
		t.Fatal(err)
	}
	files := osTestTempFS()
	files.readDir = func(path string) ([]fs.DirEntry, error) {
		entries, err := os.ReadDir(path)
		for left, right := 0, len(entries)-1; left < right; left, right = left+1, right-1 {
			entries[left], entries[right] = entries[right], entries[left]
		}
		return entries, err
	}
	var inspected []string
	files.lstat = func(path string) (fs.FileInfo, error) {
		if path != root {
			inspected = append(inspected, filepath.Base(path))
		}
		return os.Lstat(path)
	}
	var removed []string
	blockedB := errors.New("blocked home-1")
	blockedC := errors.New("blocked home-2")
	files.removeAll = func(path string) error {
		removed = append(removed, filepath.Base(path))
		switch path {
		case b:
			return blockedB
		case c:
			return blockedC
		default:
			return os.RemoveAll(path)
		}
	}
	m, err := newTestTempManager(root, time.Now, files, func(string, fs.FileInfo) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	result, err := m.cleanup(CleanupAll)
	if err == nil || !errors.Is(err, blockedB) || !errors.Is(err, blockedC) {
		t.Fatalf("failure identities = %v", err)
	}
	if first, second := strings.Index(err.Error(), b), strings.Index(err.Error(), c); first < 0 || second <= first {
		t.Fatalf("failure order = %v", err)
	}
	if strings.Join(inspected, ",") != "home-1,home-2,home-3" {
		t.Fatalf("inspection order %q", inspected)
	}
	if strings.Join(removed, ",") != "home-1,home-2,home-3" {
		t.Fatalf("removal order %q", removed)
	}
	if result.homes != 1 || result.bytes != 3 {
		t.Fatalf("result = %+v, want one home and 3 bytes", result)
	}
	if strings.Contains(err.Error(), a) {
		t.Fatalf("removed path retained: %v", err)
	}
	if got := result.String(); got != "test temp cleanup: removed 1 home(s), 3 logical byte(s)\n" {
		t.Fatalf("render = %q", got)
	}
	if got := (testTempCleanupResult{}).String(); got != "test temp cleanup: removed 0 home(s), 0 logical byte(s)\n" {
		t.Fatalf("zero render = %q", got)
	}
}

func TestCleanupConcurrentDisappearanceAndRootFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	mkdirHome(t, root, "home-1", time.Now().Add(-48*time.Hour))
	files := osTestTempFS()
	files.lstat = func(path string) (fs.FileInfo, error) {
		if filepath.Base(path) == "home-1" {
			return nil, fmtErrNotExist()
		}
		return os.Lstat(path)
	}
	m, err := newTestTempManager(root, time.Now, files, func(string, fs.FileInfo) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	result, err := m.cleanup(CleanupAll)
	if err != nil || result.homes != 0 {
		t.Fatalf("concurrent result=%+v err=%v", result, err)
	}
	files = osTestTempFS()
	touched := false
	files.readDir = func(string) ([]fs.DirEntry, error) { touched = true; return nil, errors.New("read") }
	m, _ = newTestTempManager("relative", time.Now, files, func(string, fs.FileInfo) error { return nil })
	if _, err := m.cleanup(CleanupAll); err == nil || touched {
		t.Fatal("invalid root read children")
	}
}

func fmtErrNotExist() error { return &os.PathError{Op: "lstat", Path: "gone", Err: fs.ErrNotExist} }

type testTempDirEntry struct {
	name    string
	mode    fs.FileMode
	info    fs.FileInfo
	infoErr error
}

func (e testTempDirEntry) Name() string               { return e.name }
func (e testTempDirEntry) IsDir() bool                { return e.mode.IsDir() }
func (e testTempDirEntry) Type() fs.FileMode          { return e.mode }
func (e testTempDirEntry) Info() (fs.FileInfo, error) { return e.info, e.infoErr }

func TestTestTempManagerFaultAndConcurrencyPaths(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	files := osTestTempFS()
	files.mkdir = func(string, fs.FileMode) error { return errors.New("mkdir") }
	m, _ := newTestTempManager(root, time.Now, files, func(string, fs.FileInfo) error { return nil })
	if err := m.ensureRoot(); err == nil {
		t.Fatal("mkdir failure accepted")
	}

	files = osTestTempFS()
	files.mkdir = func(string, fs.FileMode) error { return fs.ErrExist }
	files.lstat = func(string) (fs.FileInfo, error) { return nil, errors.New("lstat") }
	m, _ = newTestTempManager(root, time.Now, files, func(string, fs.FileInfo) error { return nil })
	if err := m.ensureRoot(); err == nil {
		t.Fatal("root lstat failure accepted")
	}
	if _, err := m.allocate(); err == nil {
		t.Fatal("allocation continued after unsafe root")
	}

	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	mkdirHome(t, root, "home-1", time.Now().Add(-48*time.Hour))
	files = osTestTempFS()
	files.readDir = func(string) ([]fs.DirEntry, error) { return nil, errors.New("read") }
	m, _ = newTestTempManager(root, time.Now, files, func(string, fs.FileInfo) error { return nil })
	if _, err := m.cleanup(CleanupAll); err == nil {
		t.Fatal("root read failure accepted")
	}

	files = osTestTempFS()
	files.lstat = func(path string) (fs.FileInfo, error) {
		if filepath.Base(path) == "home-1" {
			return nil, errors.New("candidate lstat")
		}
		return os.Lstat(path)
	}
	m, _ = newTestTempManager(root, time.Now, files, func(string, fs.FileInfo) error { return nil })
	if _, err := m.cleanup(CleanupAll); err == nil {
		t.Fatal("candidate lstat failure accepted")
	}

	files = osTestTempFS()
	files.walkDir = func(string, fs.WalkDirFunc) error { return fmtErrNotExist() }
	m, _ = newTestTempManager(root, time.Now, files, func(string, fs.FileInfo) error { return nil })
	if result, err := m.cleanup(CleanupAll); err != nil || result.homes != 0 {
		t.Fatalf("walk disappearance = %+v, %v", result, err)
	}

	files = osTestTempFS()
	files.walkDir = func(string, fs.WalkDirFunc) error { return errors.New("walk") }
	m, _ = newTestTempManager(root, time.Now, files, func(string, fs.FileInfo) error { return nil })
	if _, err := m.cleanup(CleanupAll); err == nil {
		t.Fatal("walk failure accepted")
	}

	files = osTestTempFS()
	files.removeAll = func(string) error { return fmtErrNotExist() }
	m, _ = newTestTempManager(root, time.Now, files, func(string, fs.FileInfo) error { return nil })
	if result, err := m.cleanup(CleanupAll); err != nil || result.homes != 0 {
		t.Fatalf("remove disappearance = %+v, %v", result, err)
	}
}

func TestTestTempLogicalBytesClassifiesUnknownEntryTypesByInfo(t *testing.T) {
	base := t.TempDir()
	directoryInfo, err := os.Stat(base)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(base, "target")
	if err := os.WriteFile(target, []byte("target bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	linkInfo, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}

	m := safeTestTempManager(t, base, time.Now())
	files := m.fs
	files.walkDir = func(_ string, walk fs.WalkDirFunc) error {
		for _, entry := range []fs.DirEntry{
			testTempDirEntry{name: "unknown-directory", info: directoryInfo},
			testTempDirEntry{name: "unknown-symlink", info: linkInfo},
		} {
			if err := walk(entry.Name(), entry, nil); err != nil {
				return err
			}
		}
		return nil
	}
	m.fs = files
	if bytes, err := m.logicalBytes("ignored"); err != nil || bytes != 0 {
		t.Fatalf("logical bytes = %d, %v; want 0", bytes, err)
	}
}

func TestTestTempLogicalBytesWalkFaults(t *testing.T) {
	m := safeTestTempManager(t, t.TempDir(), time.Now())
	files := m.fs
	files.walkDir = func(_ string, walk fs.WalkDirFunc) error { return walk("broken", nil, errors.New("walk entry")) }
	m.fs = files
	if _, err := m.logicalBytes("ignored"); err == nil {
		t.Fatal("walk entry failure accepted")
	}
	files = m.fs
	files.walkDir = func(_ string, walk fs.WalkDirFunc) error {
		return walk("regular", testTempDirEntry{name: "regular", mode: 0, infoErr: errors.New("info")}, nil)
	}
	m.fs = files
	if _, err := m.logicalBytes("ignored"); err == nil {
		t.Fatal("regular info failure accepted")
	}
}

func TestRunIsolatedOrderingWarningsAndFailures(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	mkdirHome(t, root, "home-1", time.Now().Add(-48*time.Hour))
	m := safeTestTempManager(t, root, time.Now())
	m.validate = func(path string, info fs.FileInfo) error {
		if filepath.Base(path) == "home-1" {
			return errors.New("unsafe")
		}
		return nil
	}
	var stderr strings.Builder
	var sequence []string
	files := m.fs
	files.mkdir = func(path string, mode fs.FileMode) error {
		sequence = append(sequence, "root")
		return os.Mkdir(path, mode)
	}
	files.readDir = func(path string) ([]fs.DirEntry, error) {
		sequence = append(sequence, "cleanup")
		return os.ReadDir(path)
	}
	mkdirTemp := files.mkdirTemp
	files.mkdirTemp = func(path, pattern string) (string, error) {
		sequence = append(sequence, "allocate")
		return mkdirTemp(path, pattern)
	}
	files.removeAll = func(path string) error {
		sequence = append(sequence, "remove")
		return os.RemoveAll(path)
	}
	m.fs = files
	if got := runIsolated(func() int { sequence = append(sequence, "run"); return 0 }, func(string) error { sequence = append(sequence, "home"); return nil }, m, &stderr); got != 0 {
		t.Fatalf("code = %d", got)
	}
	if strings.Join(sequence, ",") != "root,root,cleanup,root,allocate,home,run,remove" || !strings.Contains(stderr.String(), "stale test-home cleanup") {
		t.Fatalf("sequence=%q stderr=%q", sequence, stderr.String())
	}

	for _, tc := range []struct {
		name    string
		manager func() *testTempManager
		setHome func(string) error
	}{
		{"root", func() *testTempManager { return safeTestTempManager(t, "relative", time.Now()) }, func(string) error { return nil }},
		{"allocation", func() *testTempManager {
			x := safeTestTempManager(t, filepath.Join(t.TempDir(), "root"), time.Now())
			f := x.fs
			f.mkdirTemp = func(string, string) (string, error) { return "", errors.New("allocate") }
			x.fs = f
			return x
		}, func(string) error { return nil }},
		{"HOME", func() *testTempManager { return safeTestTempManager(t, filepath.Join(t.TempDir(), "root"), time.Now()) }, func(string) error { return errors.New("HOME") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ran := false
			defer func() {
				if recover() == nil {
					t.Fatal("expected panic")
				}
				if ran {
					t.Fatal("suite ran after pre-run failure")
				}
			}()
			runIsolated(func() int { ran = true; return 0 }, tc.setHome, tc.manager(), io.Discard)
		})
	}
}

func TestCleanTestTempsRejectsUnknownModeBeforeFactory(t *testing.T) {
	called := false
	var output strings.Builder
	err := cleanTestTemps(CleanupMode(99), &output, func() (*testTempManager, error) {
		called = true
		return nil, errors.New("must not run")
	})
	if err == nil || called || output.Len() != 0 {
		t.Fatalf("err=%v called=%t output=%q", err, called, output.String())
	}
}

func TestCleanTestTempsSelectsModesAndRendersSuccessfulScans(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name string
		mode CleanupMode
		age  time.Duration
	}{
		{"stale", CleanupStale, -48 * time.Hour},
		{"all", CleanupAll, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "root")
			if err := os.Mkdir(root, 0o700); err != nil {
				t.Fatal(err)
			}
			mkdirHome(t, root, "home-1", now.Add(tc.age))
			var output strings.Builder
			err := cleanTestTemps(tc.mode, &output, func() (*testTempManager, error) {
				return safeTestTempManager(t, root, now), nil
			})
			if err != nil || output.String() != "test temp cleanup: removed 1 home(s), 0 logical byte(s)\n" {
				t.Fatalf("err=%v output=%q", err, output.String())
			}
		})
	}
}

func TestCleanTestTempsRendersZeroAndPartialScans(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	if err := cleanTestTemps(CleanupAll, &output, func() (*testTempManager, error) {
		return safeTestTempManager(t, root, time.Now()), nil
	}); err != nil || output.String() != "test temp cleanup: removed 0 home(s), 0 logical byte(s)\n" {
		t.Fatalf("zero scan err=%v output=%q", err, output.String())
	}

	mkdirHome(t, root, "home-1", time.Now())
	blocked := errors.New("blocked")
	output.Reset()
	err := cleanTestTemps(CleanupAll, &output, func() (*testTempManager, error) {
		m := safeTestTempManager(t, root, time.Now())
		files := m.fs
		files.removeAll = func(string) error { return blocked }
		m.fs = files
		return m, nil
	})
	if !errors.Is(err, blocked) || output.String() != "test temp cleanup: removed 0 home(s), 0 logical byte(s)\n" {
		t.Fatalf("partial err=%v output=%q", err, output.String())
	}
}

func TestCleanTestTempsRootFailureDoesNotRender(t *testing.T) {
	var output strings.Builder
	err := cleanTestTemps(CleanupAll, &output, func() (*testTempManager, error) {
		return safeTestTempManager(t, "relative", time.Now()), nil
	})
	if err == nil || output.Len() != 0 {
		t.Fatalf("err=%v output=%q", err, output.String())
	}
}

func TestCleanTestTempsFactoryFailureDoesNotRender(t *testing.T) {
	var output strings.Builder
	err := cleanTestTemps(CleanupAll, &output, func() (*testTempManager, error) { return nil, fs.ErrPermission })
	if !errors.Is(err, fs.ErrPermission) || output.Len() != 0 {
		t.Fatalf("err=%v output=%q", err, output.String())
	}
}

func TestCleanTestTempsUsesProductionFactory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("production test temp cleanup is supported on Linux and macOS")
	}
	t.Setenv("TMPDIR", t.TempDir())
	var output strings.Builder
	if err := CleanTestTemps(CleanupStale, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output.String(), "test temp cleanup: removed ") {
		t.Fatalf("cleanup summary = %q", output.String())
	}
}

func TestRunIsolatedCleanupExitMapping(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	m := safeTestTempManager(t, root, time.Now())
	files := m.fs
	files.removeAll = func(string) error { return errors.New("remove") }
	m.fs = files
	var stderr strings.Builder
	if got := runIsolated(func() int { return 0 }, func(string) error { return nil }, m, &stderr); got != 1 {
		t.Fatalf("code = %d", got)
	}
	if !strings.Contains(stderr.String(), "remove current test home") {
		t.Fatalf("warning = %q", stderr.String())
	}
	m = safeTestTempManager(t, filepath.Join(t.TempDir(), "root"), time.Now())
	files = m.fs
	files.removeAll = func(string) error { return errors.New("remove") }
	m.fs = files
	if got := runIsolated(func() int { return 7 }, func(string) error { return nil }, m, io.Discard); got != 7 {
		t.Fatalf("nonzero code = %d", got)
	}
}
