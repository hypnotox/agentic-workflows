package effort

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// invariant: tooling/effort-management:effort-record-authority
func TestAtomicReplacementDurabilityOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "record.json")
	writeEffortFile(t, path, "old")
	identity, err := lstatRegular(path)
	if err != nil {
		t.Fatal(err)
	}
	fs := &faultFileSystem{}
	if err := atomicReplaceFS(fs, path, []byte("published"), &identity); err != nil {
		t.Fatal(err)
	}
	want := []string{"create-temp", "stat-temp", "write-temp", "fsync-temp", "close-temp", "rename", "open-directory", "fsync-directory", "close-directory"}
	if !reflect.DeepEqual(fs.events, want) {
		t.Fatalf("durability order = %v, want %v", fs.events, want)
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != "published" {
		t.Fatalf("published bytes = %q, %v", raw, err)
	}
	assertNoEffortTemps(t, dir)
}

// invariant: tooling/effort-management:effort-record-authority
func TestAtomicReplacementFaultsPreserveOldOrPublishedBytesAndContext(t *testing.T) {
	for _, stage := range []string{"create-temp", "stat-temp", "write-temp", "short-write", "fsync-temp", "close-temp", "rename", "open-directory", "fsync-directory", "close-directory"} {
		t.Run(stage, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "record.json")
			writeEffortFile(t, path, "old")
			identity, err := lstatRegular(path)
			if err != nil {
				t.Fatal(err)
			}
			fs := &faultFileSystem{fail: stage}
			err = atomicReplaceFS(fs, path, []byte("published"), &identity)
			if err == nil || (stage != "short-write" && !errors.Is(err, errInjectedDurability)) || (stage == "short-write" && !errors.Is(err, io.ErrShortWrite)) {
				t.Fatalf("fault error = %v", err)
			}
			if !strings.Contains(err.Error(), path) && !strings.Contains(err.Error(), dir) {
				t.Fatalf("fault error lacks operation path: %v", err)
			}
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			want := "old"
			if stage == "open-directory" || stage == "fsync-directory" || stage == "close-directory" {
				want = "published"
			}
			if string(raw) != want {
				t.Fatalf("bytes after %s = %q, want %q", stage, raw, want)
			}
			assertNoEffortTemps(t, dir)
		})
	}
}

func TestAtomicReplacementReportsTemporaryCleanupFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "record.json")
	writeEffortFile(t, path, "old")
	identity, _ := lstatRegular(path)
	fs := &faultFileSystem{fail: "write-temp", failRemove: true}
	err := atomicReplaceFS(fs, path, []byte("new"), &identity)
	if !errors.Is(err, errInjectedDurability) || !errors.Is(err, errInjectedCleanup) || !strings.Contains(err.Error(), path) {
		t.Fatalf("cleanup failure error = %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, ".effort-*.tmp"))
	if len(matches) != 1 {
		t.Fatalf("failed cleanup temp count = %d, want 1", len(matches))
	}
}

func TestAtomicReplacementRejectsUnsafeTemporaryAndRacedDestinationIdentities(t *testing.T) {
	t.Run("temporary outside sibling directory", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "record.json")
		writeEffortFile(t, path, "old")
		identity, _ := lstatRegular(path)
		fs := &faultFileSystem{outside: t.TempDir()}
		if err := atomicReplaceFS(fs, path, []byte("new"), &identity); err == nil || !strings.Contains(err.Error(), "outside") {
			t.Fatalf("outside temp error = %v", err)
		}
		assertNoEffortTemps(t, fs.outside)
	})
	t.Run("temporary file type", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "record.json")
		writeEffortFile(t, path, "old")
		identity, _ := lstatRegular(path)
		fs := &faultFileSystem{unsafeTemp: true}
		if err := atomicReplaceFS(fs, path, []byte("new"), &identity); err == nil || !strings.Contains(err.Error(), "file-type") {
			t.Fatalf("unsafe temp error = %v", err)
		}
		assertNoEffortTemps(t, dir)
	})
	t.Run("temporary identity replacement", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "record.json")
		writeEffortFile(t, path, "old")
		identity, _ := lstatRegular(path)
		fs := &faultFileSystem{replaceTemp: true}
		if err := atomicReplaceFS(fs, path, []byte("new"), &identity); err == nil || !strings.Contains(err.Error(), "identity") {
			t.Fatalf("temp identity error = %v", err)
		}
		assertNoEffortTemps(t, dir)
	})
	t.Run("absent destination race", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "record.json")
		fs := &faultFileSystem{raceDestination: path}
		if err := atomicReplaceFS(fs, path, []byte("new"), nil); err == nil || !errors.Is(err, os.ErrExist) {
			t.Fatalf("destination race error = %v", err)
		}
		if raw, err := os.ReadFile(path); err != nil || string(raw) != "raced" {
			t.Fatalf("raced destination = %q, %v", raw, err)
		}
		assertNoEffortTemps(t, dir)
	})
}

var errInjectedDurability = errors.New("injected durability failure")
var errInjectedCleanup = errors.New("injected cleanup failure")

type faultFileSystem struct {
	fail            string
	events          []string
	outside         string
	replaceTemp     bool
	raceDestination string
	unsafeTemp      bool
	failRemove      bool
}

func (f *faultFileSystem) hit(stage string) error {
	f.events = append(f.events, stage)
	if f.fail == stage {
		return errInjectedDurability
	}
	return nil
}

func (f *faultFileSystem) CreateTemp(dir, pattern string) (durableFile, error) {
	if err := f.hit("create-temp"); err != nil {
		return nil, err
	}
	if f.outside != "" {
		dir = f.outside
	}
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	return &faultDurableFile{File: file, fs: f, kind: "temp"}, nil
}
func (f *faultFileSystem) Rename(oldPath, newPath string) error {
	if err := f.hit("rename"); err != nil {
		return err
	}
	return os.Rename(oldPath, newPath)
}
func (f *faultFileSystem) Remove(path string) error {
	if f.failRemove {
		return errInjectedCleanup
	}
	return os.Remove(path)
}
func (f *faultFileSystem) OpenDirectory(path string) (durableFile, error) {
	if err := f.hit("open-directory"); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &faultDurableFile{File: file, fs: f, kind: "directory"}, nil
}

type faultDurableFile struct {
	*os.File
	fs     *faultFileSystem
	kind   string
	closed bool
}

func (f *faultDurableFile) Stat() (os.FileInfo, error) {
	if f.kind == "temp" {
		if err := f.fs.hit("stat-temp"); err != nil {
			return nil, err
		}
		if f.fs.unsafeTemp {
			return os.Stat(filepath.Dir(f.Name()))
		}
	}
	return f.File.Stat()
}

func (f *faultDurableFile) Write(raw []byte) (int, error) {
	if err := f.fs.hit("write-temp"); err != nil {
		return 0, err
	}
	if f.fs.fail == "short-write" {
		f.fs.events = append(f.fs.events, "short-write")
		count := len(raw) / 2
		_, _ = f.File.Write(raw[:count])
		return count, nil
	}
	return f.File.Write(raw)
}
func (f *faultDurableFile) Sync() error {
	stage := "fsync-" + f.kind
	if err := f.fs.hit(stage); err != nil {
		return err
	}
	return f.File.Sync()
}
func (f *faultDurableFile) Close() error {
	if f.closed {
		return f.File.Close()
	}
	f.closed = true
	stage := "close-" + f.kind
	if err := f.fs.hit(stage); err != nil {
		_ = f.File.Close()
		return err
	}
	if err := f.File.Close(); err != nil {
		return err
	}
	if f.kind == "temp" && f.fs.replaceTemp {
		replacement := f.Name() + ".replacement"
		if err := os.WriteFile(replacement, []byte("replacement"), 0o600); err != nil {
			return err
		}
		if err := os.Rename(replacement, f.Name()); err != nil {
			return err
		}
	}
	if f.kind == "temp" && f.fs.raceDestination != "" {
		if err := os.WriteFile(f.fs.raceDestination, []byte("raced"), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func assertNoEffortTemps(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".effort-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain: %v", matches)
	}
}
