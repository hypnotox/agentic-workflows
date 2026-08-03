package testsupport

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const (
	testHomePrefix   = "home-"
	staleTestHomeAge = 24 * time.Hour
)

type testTempFS struct {
	mkdir     func(string, fs.FileMode) error
	mkdirTemp func(string, string) (string, error)
	lstat     func(string) (fs.FileInfo, error)
	readDir   func(string) ([]fs.DirEntry, error)
	walkDir   func(string, fs.WalkDirFunc) error
	removeAll func(string) error
}

func osTestTempFS() testTempFS {
	return testTempFS{os.Mkdir, os.MkdirTemp, os.Lstat, os.ReadDir, filepath.WalkDir, os.RemoveAll}
}

type testTempManager struct {
	root     string
	now      func() time.Time
	fs       testTempFS
	validate func(string, fs.FileInfo) error
}

func newTestTempManager(root string, now func() time.Time, files testTempFS, validate func(string, fs.FileInfo) error) (*testTempManager, error) {
	if now == nil || validate == nil || files.mkdir == nil || files.mkdirTemp == nil || files.lstat == nil || files.readDir == nil || files.walkDir == nil || files.removeAll == nil {
		return nil, errors.New("test temp manager requires root, clock, filesystem, and validator")
	}
	return &testTempManager{root: root, now: now, fs: files, validate: validate}, nil
}

func productionTestTempManager() (*testTempManager, error) {
	root, err := testTempRoot()
	if err != nil { // coverage-ignore: Unix production root selection cannot fail; Windows only retains compile compatibility
		return nil, err
	}
	return newTestTempManager(root, time.Now, osTestTempFS(), validateTestTempPath)
}

func (m *testTempManager) ensureRoot() error {
	if !filepath.IsAbs(m.root) || filepath.Clean(m.root) != m.root || filepath.Dir(m.root) == m.root {
		return fmt.Errorf("unsafe test temp root %q", m.root)
	}
	if err := m.fs.mkdir(m.root, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("create test temp root %s: %w", m.root, err)
	}
	info, err := m.fs.lstat(m.root)
	if err != nil {
		return fmt.Errorf("inspect test temp root %s: %w", m.root, err)
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("unsafe test temp root %s", m.root)
	}
	if err := m.validate(m.root, info); err != nil {
		return fmt.Errorf("unsafe test temp root %s: %w", m.root, err)
	}
	return nil
}

func canonicalTestHome(name string) bool {
	if !strings.HasPrefix(name, testHomePrefix) || len(name) == len(testHomePrefix) {
		return false
	}
	for _, r := range name[len(testHomePrefix):] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (m *testTempManager) allocate() (string, error) {
	if err := m.ensureRoot(); err != nil {
		return "", err
	}
	path, err := m.fs.mkdirTemp(m.root, testHomePrefix)
	if err != nil {
		return "", fmt.Errorf("allocate test home: %w", err)
	}
	if filepath.Dir(path) != m.root || !canonicalTestHome(filepath.Base(path)) {
		return "", fmt.Errorf("allocate test home outside managed root: %s", path)
	}
	return path, nil
}

type testTempCleanupResult struct {
	homes int
	bytes int64
}

func (r testTempCleanupResult) String() string {
	return fmt.Sprintf("test temp cleanup: removed %d home(s), %d logical byte(s)\n", r.homes, r.bytes)
}

// CleanupMode selects which managed test homes cleanup considers.
type CleanupMode int

const (
	// CleanupStale removes only homes strictly older than 24 hours.
	CleanupStale CleanupMode = iota
	// CleanupAll removes every canonical managed home.
	CleanupAll
)

func (m *testTempManager) cleanup(mode CleanupMode) (testTempCleanupResult, error) {
	result, err, _ := m.cleanupWithScan(mode)
	return result, err
}

func (m *testTempManager) cleanupWithScan(mode CleanupMode) (testTempCleanupResult, error, bool) {
	if err := m.ensureRoot(); err != nil {
		return testTempCleanupResult{}, err, false
	}
	entries, err := m.fs.readDir(m.root)
	if err != nil {
		return testTempCleanupResult{}, fmt.Errorf("read test temp root %s: %w", m.root, err), false
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var result testTempCleanupResult
	var failures []error
	cutoff := m.now().Add(-staleTestHomeAge)
	for _, entry := range entries {
		if !canonicalTestHome(entry.Name()) {
			continue
		}
		path := filepath.Join(m.root, entry.Name())
		info, err := m.fs.lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("inspect test home %s: %w", path, err))
			continue
		}
		if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
			failures = append(failures, fmt.Errorf("unsafe test home %s", path))
			continue
		}
		if err := m.validate(path, info); err != nil {
			failures = append(failures, fmt.Errorf("unsafe test home %s: %w", path, err))
			continue
		}
		if mode == CleanupStale && !info.ModTime().Before(cutoff) {
			continue
		}
		bytes, err := m.logicalBytes(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("size test home %s: %w", path, err))
			continue
		}
		if err := m.fs.removeAll(path); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			failures = append(failures, fmt.Errorf("remove test home %s: %w", path, err))
			continue
		}
		result.homes++
		result.bytes += bytes
	}
	return result, errors.Join(failures...), true
}

// CleanTestTemps cleans production managed test homes and writes their summary.
func CleanTestTemps(mode CleanupMode, output io.Writer) error {
	return cleanTestTemps(mode, output, productionTestTempManager)
}

func cleanTestTemps(mode CleanupMode, output io.Writer, managerFactory func() (*testTempManager, error)) error {
	if mode != CleanupStale && mode != CleanupAll {
		return fmt.Errorf("unknown test temp cleanup mode %d", mode)
	}
	manager, err := managerFactory()
	if err != nil {
		return err
	}
	result, cleanupErr, scanned := manager.cleanupWithScan(mode)
	if !scanned {
		return cleanupErr
	}
	if _, err := io.WriteString(output, result.String()); err != nil {
		return errors.Join(cleanupErr, fmt.Errorf("write test temp cleanup summary: %w", err))
	}
	return cleanupErr
}

func (m *testTempManager) logicalBytes(path string) (int64, error) {
	var total int64
	err := m.fs.walkDir(path, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil
		}
		if size := info.Size(); size > 0 {
			total += size
		}
		return nil
	})
	return total, err
}

func preserveDefaultGOPATH(
	lookupEnv func(string) (string, bool),
	userHomeDir func() (string, error),
	setEnv func(string, string) error,
) error {
	if goPath, ok := lookupEnv("GOPATH"); ok && goPath != "" {
		return nil
	}
	home, err := userHomeDir()
	if err != nil {
		return fmt.Errorf("resolve default GOPATH home: %w", err)
	}
	if err := setEnv("GOPATH", filepath.Join(home, "go")); err != nil {
		return fmt.Errorf("preserve default GOPATH: %w", err)
	}
	return nil
}

func RunIsolated(m *testing.M) int {
	if err := preserveDefaultGOPATH(os.LookupEnv, os.UserHomeDir, os.Setenv); err != nil { // coverage-ignore: helper fault paths are injected above; production environment access cannot be faulted safely
		panic(err)
	}
	manager, err := productionTestTempManager()
	if err != nil { // coverage-ignore: Unix production root selection cannot fail; Windows compiles but does not run tests
		panic(err)
	}
	return runIsolated(m.Run, func(home string) error { return os.Setenv("HOME", home) }, manager, os.Stderr)
}

func runIsolated(run func() int, setHome func(string) error, manager *testTempManager, stderr io.Writer) int {
	if err := manager.ensureRoot(); err != nil {
		panic(err)
	}
	if _, err := manager.cleanup(CleanupStale); err != nil {
		if _, writeErr := fmt.Fprintf(stderr, "testsupport: stale test-home cleanup: %v\n", err); writeErr != nil {
			panic(fmt.Errorf("write stale test-home cleanup warning: %w", writeErr))
		}
	}
	home, err := manager.allocate()
	if err != nil {
		panic(err)
	}
	if err := setHome(home); err != nil {
		panic(err)
	}
	code := run()
	if err := manager.fs.removeAll(home); err != nil {
		_, writeErr := fmt.Fprintf(stderr, "testsupport: remove current test home %s: %v\n", home, err)
		if code == 0 {
			return 1
		}
		// Preserve the suite's nonzero result; stderr has no fallback diagnostic channel.
		_ = writeErr
	}
	return code
}
