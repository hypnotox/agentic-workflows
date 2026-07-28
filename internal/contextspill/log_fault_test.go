package contextspill

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLogOperationalFailures(t *testing.T) {
	if err := Log("", Notice{}, nil); err == nil {
		t.Fatal("empty root succeeded")
	}
	if err := Log(filepath.Join(t.TempDir(), "missing"), Notice{}, nil); err == nil {
		t.Fatal("missing root succeeded")
	}

	for _, test := range []struct {
		name string
		set  func()
		want string
	}{
		{name: "mkdir", set: func() { mkdirPath = func(string, os.FileMode) error { return errors.New("mkdir fault") } }, want: "mkdir fault"},
		{name: "open", set: func() { openFile = func(string, int, uint32) (int, error) { return -1, errors.New("open fault") } }, want: "open fault"},
		{name: "fstat", set: func() { fstatFile = func(int, *syscall.Stat_t) error { return errors.New("fstat fault") } }, want: "fstat fault"},
		{name: "nonregular", set: func() {
			fstatFile = func(fd int, stat *syscall.Stat_t) error {
				if err := syscall.Fstat(fd, stat); err != nil {
					return err
				}
				stat.Mode = syscall.S_IFDIR | 0o600
				return nil
			}
		}, want: "not a regular file"},
		{name: "file mode", set: func() {
			fstatFile = func(fd int, stat *syscall.Stat_t) error {
				if err := syscall.Fstat(fd, stat); err != nil {
					return err
				}
				stat.Mode = syscall.S_IFREG | 0o644
				return nil
			}
		}, want: "want 0600"},
		{name: "file owner", set: func() {
			fstatFile = func(fd int, stat *syscall.Stat_t) error {
				if err := syscall.Fstat(fd, stat); err != nil {
					return err
				}
				stat.Uid++
				return nil
			}
		}, want: "another user"},
		{name: "lock", set: func() { flockFile = func(int, int) error { return errors.New("lock fault") } }, want: "lock fault"},
		{name: "write", set: func() { writeFile = func(int, []byte) (int, error) { return 0, errors.New("write fault") } }, want: "write fault"},
		{name: "short write", set: func() { writeFile = func(int, []byte) (int, error) { return 0, nil } }, want: "short write"},
		{name: "sync", set: func() { fsyncFile = func(int) error { return errors.New("sync fault") } }, want: "sync fault"},
		{name: "unlock", set: func() {
			flockFile = func(fd int, operation int) error {
				if operation == syscall.LOCK_UN {
					return errors.New("unlock fault")
				}
				return syscall.Flock(fd, operation)
			}
		}, want: "unlock fault"},
		{name: "close", set: func() { closeFile = func(fd int) error { _ = syscall.Close(fd); return errors.New("close fault") } }, want: "close fault"},
		{name: "first error preserved", set: func() {
			writeFile = func(int, []byte) (int, error) { return 0, errors.New("primary write fault") }
			flockFile = func(fd int, operation int) error {
				if operation == syscall.LOCK_UN {
					return errors.New("later unlock fault")
				}
				return syscall.Flock(fd, operation)
			}
			closeFile = func(fd int) error { _ = syscall.Close(fd); return errors.New("later close fault") }
		}, want: "primary write fault"},
	} {
		t.Run(test.name, func(t *testing.T) {
			restoreLogOps(t)
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, ".awf"), 0o755); err != nil {
				t.Fatal(err)
			}
			test.set()
			err := Log(root, Notice{Bytes: 1}, []string{"x"})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Log() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestLogPathInspectionFailures(t *testing.T) {
	for name, prepare := range map[string]func(string) error{
		"missing awf": func(string) error { return nil },
		"awf file":    func(root string) error { return os.WriteFile(filepath.Join(root, ".awf"), nil, 0o600) },
		"local file": func(root string) error {
			if err := os.Mkdir(filepath.Join(root, ".awf"), 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(root, ".awf", "local"), nil, 0o600)
		},
	} {
		t.Run(name, func(t *testing.T) {
			restoreLogOps(t)
			root := t.TempDir()
			if err := prepare(root); err != nil {
				t.Fatal(err)
			}
			if err := Log(root, Notice{}, nil); err == nil {
				t.Fatal("unsafe path succeeded")
			}
		})
	}

	t.Run("directory owner", func(t *testing.T) {
		restoreLogOps(t)
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, ".awf"), 0o755); err != nil {
			t.Fatal(err)
		}
		original := lstatPath
		lstatPath = func(path string) (os.FileInfo, error) {
			info, err := original(path)
			if err != nil {
				return nil, err
			}
			return fakeFileInfo{mode: info.Mode(), sys: syscall.Stat_t{Uid: uint32(os.Getuid() + 1)}}, nil
		}
		if err := Log(root, Notice{}, nil); err == nil || !strings.Contains(err.Error(), "another user") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("ownership unavailable", func(t *testing.T) {
		restoreLogOps(t)
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, ".awf"), 0o755); err != nil {
			t.Fatal(err)
		}
		lstatPath = func(string) (os.FileInfo, error) { return fakeFileInfo{mode: os.ModeDir | 0o755, sys: nil}, nil }
		if err := Log(root, Notice{}, nil); err == nil || !strings.Contains(err.Error(), "ownership unavailable") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestWriteAllFDHandlesPartialWrites(t *testing.T) {
	restoreLogOps(t)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	original := writeFile
	writeFile = func(fd int, data []byte) (int, error) {
		if len(data) > 1 {
			data = data[:1]
		}
		return original(fd, data)
	}
	if err := Log(root, Notice{Bytes: 2}, []string{"x"}); err != nil {
		t.Fatal(err)
	}
}

func TestFileUIDShapes(t *testing.T) {
	cases := []fakeFileInfo{
		{sys: nil},
		{sys: (*syscall.Stat_t)(nil)},
		{sys: 1},
		{sys: struct{ Other uint32 }{}},
		{sys: struct{ Uid string }{Uid: "x"}},
	}
	for _, info := range cases {
		if _, ok := fileUID(info); ok {
			t.Fatalf("fileUID(%T) unexpectedly succeeded", info.sys)
		}
	}
	if uid, ok := fileUID(fakeFileInfo{sys: syscall.Stat_t{Uid: 42}}); !ok || uid != 42 {
		t.Fatalf("fileUID = %d, %v", uid, ok)
	}
}

func restoreLogOps(t *testing.T) {
	t.Helper()
	oldNow, oldLstat, oldMkdir := now, lstatPath, mkdirPath
	oldOpen, oldFstat, oldFlock := openFile, fstatFile, flockFile
	oldWrite, oldFsync, oldClose := writeFile, fsyncFile, closeFile
	t.Cleanup(func() {
		now, lstatPath, mkdirPath = oldNow, oldLstat, oldMkdir
		openFile, fstatFile, flockFile = oldOpen, oldFstat, oldFlock
		writeFile, fsyncFile, closeFile = oldWrite, oldFsync, oldClose
	})
	now = time.Now
	lstatPath = os.Lstat
	mkdirPath = os.Mkdir
	openFile = syscall.Open
	fstatFile = syscall.Fstat
	flockFile = syscall.Flock
	writeFile = syscall.Write
	fsyncFile = syscall.Fsync
	closeFile = syscall.Close
}

type fakeFileInfo struct {
	mode os.FileMode
	sys  any
}

func (f fakeFileInfo) Name() string       { return "fake" }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeFileInfo) Sys() any           { return f.sys }

var _ fs.FileInfo = fakeFileInfo{}
