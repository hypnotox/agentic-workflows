package effort

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

// invariant: tooling/effort-management:effort-record-authority
func TestEffortLeafSafetyRefusesSymlinksWithoutTouchingTargets(t *testing.T) {
	for _, tc := range []struct {
		name string
		leaf func(string) string
		act  func(*Service) error
	}{
		{"record", func(root string) string { return filepath.Join(root, ".awf", "efforts", idA+".json") }, func(s *Service) error { _, err := s.Show(idA); return err }},
		{"memory", func(root string) string { return filepath.Join(root, ".awf", "memory", idA+".md") }, func(s *Service) error { _, _, err := s.Memory(idA); return err }},
		{"assignment", func(root string) string { return filepath.Join(root, ".awf", "assignments", "sessions.json") }, func(s *Service) error { _, err := s.List(); return err }},
		{"lock", func(root string) string { return filepath.Join(root, ".awf", "efforts", ".lock") }, func(s *Service) error { _, err := s.New("Unsafe lock", false); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := initEffortRepo(t)
			service := openEffortService(t, root, time.Now().UTC())
			if tc.name != "lock" {
				if _, err := service.New("Safety", false); err != nil {
					t.Fatal(err)
				}
			}
			leaf := tc.leaf(root)
			if err := os.MkdirAll(filepath.Dir(leaf), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(leaf); err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
			outside := filepath.Join(t.TempDir(), "outside")
			const sentinel = "outside-must-not-change\n"
			writeEffortFile(t, outside, sentinel)
			if err := os.Symlink(outside, leaf); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			err := tc.act(service)
			var hard *awfgit.HardSafetyError
			if !errors.As(err, &hard) || hard.Forceable() || hard.Category != "symlink" {
				t.Fatalf("symlink error = %T %v", err, err)
			}
			if got, readErr := os.ReadFile(outside); readErr != nil || string(got) != sentinel {
				t.Fatalf("outside target changed: %q, %v", got, readErr)
			}
		})
	}
}

func TestEffortReadsRevalidateResidentAncestorsBeforeLeafAccess(t *testing.T) {
	root := initEffortRepo(t)
	service := openEffortService(t, root, time.Now().UTC())
	if _, err := service.New("Ancestors", false); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(service.paths.assign, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(service.paths.assign); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	writeEffortFile(t, filepath.Join(outside, "sessions.json"), `{"schemaVersion":1,"sessions":{}}`)
	if err := os.Symlink(outside, service.paths.assign); err != nil {
		t.Skip(err)
	}
	if _, err := service.Show(idA); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("replaced assignment root error = %v", err)
	}
}

func TestEffortOperationsRejectReplacedResidentRoots(t *testing.T) {
	for _, operation := range []string{"effort-load", "effort-list", "memory", "worktrees"} {
		t.Run(operation, func(t *testing.T) {
			root := initEffortRepo(t)
			service := openEffortService(t, root, time.Now().UTC())
			if _, err := service.New("Root replacement", false); err != nil {
				t.Fatal(err)
			}
			var resident string
			switch operation {
			case "effort-load", "effort-list":
				resident = service.paths.efforts
			case "memory":
				resident = service.paths.memory
			case "worktrees":
				resident = service.paths.worktrees
			}
			if err := os.MkdirAll(resident, 0o700); err != nil {
				t.Fatal(err)
			}
			backup := resident + ".backup"
			if err := os.Rename(resident, backup); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = os.Remove(resident)
				_ = os.Rename(backup, resident)
			})
			outside := t.TempDir()
			if err := os.Symlink(outside, resident); err != nil {
				t.Skip(err)
			}
			var err error
			switch operation {
			case "effort-load":
				_, err = service.Show(idA)
			case "effort-list":
				_, err = service.List()
			case "memory":
				_, err = service.paths.memoryTruth(idA)
			case "worktrees":
				_, err = service.Repair(idA)
			}
			if err == nil || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("%s replaced-root error = %v", operation, err)
			}
		})
	}
}

func TestEffortLeafSafetyRefusesHardLinkedLockOutsideResidentRoot(t *testing.T) {
	root := initEffortRepo(t)
	service := openEffortService(t, root, time.Now().UTC())
	if err := os.MkdirAll(service.paths.efforts, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside-lock-inode")
	writeEffortFile(t, outside, "outside\n")
	lock := filepath.Join(service.paths.efforts, ".lock")
	if err := os.Link(outside, lock); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	if _, err := service.New("Hard link", false); err == nil || !strings.Contains(err.Error(), "links, want 1") {
		t.Fatalf("hard-linked lock error = %v", err)
	}
	if raw, err := os.ReadFile(outside); err != nil || string(raw) != "outside\n" {
		t.Fatalf("outside hard-link target changed: %q, %v", raw, err)
	}
}

// invariant: tooling/effort-management:effort-record-authority
func TestEffortLeafSafetyRefusesTypesPermissionsAndForeignOwnership(t *testing.T) {
	root := initEffortRepo(t)
	service := openEffortService(t, root, time.Now().UTC())
	if _, err := service.New("Types", false); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(root, ".awf", "efforts", idA+".json")
	if err := os.Remove(record); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(record, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Show(idA); err == nil || !strings.Contains(err.Error(), "file-type") {
		t.Fatalf("directory record error = %v", err)
	}
	if err := os.Remove(record); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(filepath.Join(root, ".awf", "efforts"), 0o775); err != nil {
		t.Fatal(err)
	}
	if _, err := service.New("Bad resident mode", false); err == nil || !strings.Contains(err.Error(), "resident-permissions") {
		t.Fatalf("unsafe resident mode error = %v", err)
	}
	if err := os.Chmod(filepath.Join(root, ".awf", "efforts"), 0o700); err != nil {
		t.Fatal(err)
	}

	lock := filepath.Join(root, ".awf", "efforts", ".lock")
	writeEffortFile(t, lock, "")
	if err := os.Chmod(lock, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.New("Bad lock", false); err == nil || !strings.Contains(err.Error(), "unsafe-lock") {
		t.Fatalf("unsafe lock mode error = %v", err)
	}

	if runtime.GOOS == "windows" || os.Geteuid() != 0 {
		return
	}
	if err := os.Chmod(lock, 0o600); err != nil {
		t.Fatal(err)
	}
	foreignUID := 1
	if foreignUID == os.Geteuid() {
		foreignUID = 2
	}
	if err := os.Chown(lock, foreignUID, -1); err != nil {
		t.Skipf("foreign ownership unavailable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chown(lock, os.Geteuid(), -1) })
	if _, err := lstatRegular(lock); err == nil || !strings.Contains(err.Error(), "foreign-owner") {
		t.Fatalf("foreign leaf validation error = %v", err)
	}
	if _, err := service.New("Foreign lock", false); err == nil || !strings.Contains(err.Error(), "foreign-owner") {
		t.Fatalf("foreign lock error = %v", err)
	}
}

func TestEffortListingRefusesUnsafeDirectoryFIFOAndUnreadableRecord(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(string) error
	}{
		{"directory", func(path string) error { return os.Mkdir(path, 0o700) }},
		{"fifo", func(path string) error { return syscall.Mkfifo(path, 0o600) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := initEffortRepo(t)
			service := openEffortService(t, root, time.Now().UTC())
			if err := os.MkdirAll(service.paths.efforts, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := tc.setup(service.paths.record(idA)); err != nil {
				t.Skipf("fixture unavailable: %v", err)
			}
			if _, err := service.List(); err == nil || !strings.Contains(err.Error(), "file-type") {
				t.Fatalf("unsafe listing error = %v", err)
			}
		})
	}
	if os.Geteuid() == 0 {
		return
	}
	root := initEffortRepo(t)
	service := openEffortService(t, root, time.Now().UTC())
	if _, err := service.New("Unreadable", false); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(service.paths.record(idA), 0); err != nil {
		t.Skipf("permission fixture unavailable: %v", err)
	}
	if _, err := service.Show(idA); err == nil || !strings.Contains(err.Error(), service.paths.record(idA)) {
		t.Fatalf("unreadable record error = %v", err)
	}
}

// invariant: tooling/effort-management:effort-record-authority
func TestEffortIdentityReplacementIsRefusedAndPreserved(t *testing.T) {
	root := initEffortRepo(t)
	service := openEffortService(t, root, time.Now().UTC())
	if _, err := service.New("Original", false); err != nil {
		t.Fatal(err)
	}
	path := service.paths.record(idA)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fs := &replaceOnCloseFS{path: path}
	service.store.fs = fs
	if _, err := service.Rename(idA, "Replacement attempt"); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("replacement race error = %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != fs.replacement {
		t.Fatalf("resident replacement changed: %q, %v", got, err)
	}
	if string(original) == fs.replacement {
		t.Fatal("fixture replacement did not change identity")
	}
	if matches, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".effort-*.tmp")); len(matches) != 0 {
		t.Fatalf("temporary files remain: %v", matches)
	}
}

type replaceOnCloseFS struct {
	path        string
	replacement string
}

func (f *replaceOnCloseFS) CreateTemp(dir, pattern string) (durableFile, error) {
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	return &replaceOnCloseFile{File: file, owner: f}, nil
}
func (f *replaceOnCloseFS) Publish(tempPath, path string, expected *fileIdentity) error {
	return publishAtomic(tempPath, path, expected)
}
func (f *replaceOnCloseFS) Remove(path string) error                       { return os.Remove(path) }
func (f *replaceOnCloseFS) OpenDirectory(path string) (durableFile, error) { return os.Open(path) }

type replaceOnCloseFile struct {
	*os.File
	owner *replaceOnCloseFS
}

func (f *replaceOnCloseFile) Close() error {
	if err := f.File.Close(); err != nil {
		return err
	}
	f.owner.replacement = `{"schemaVersion":1,"replacement":true}`
	replacement := f.owner.path + ".replacement"
	if err := os.WriteFile(replacement, []byte(f.owner.replacement), 0o600); err != nil {
		return err
	}
	return os.Rename(replacement, f.owner.path)
}
