package contextspill

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func testLogOperationalFailures(t *testing.T) {
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
		{name: "open root", set: func() { openAt = func(int, string, int, uint32) (int, error) { return -1, errors.New("open fault") } }, want: "open fault"},
		{name: "open awf", set: func() {
			original := openAt
			openAt = func(fd int, path string, flags int, mode uint32) (int, error) {
				if path == ".awf" {
					return -1, errors.New("awf fault")
				}
				return original(fd, path, flags, mode)
			}
		}, want: "awf fault"},
		{name: "directory kind", set: func() {
			fstatFile = func(fd int, stat *unix.Stat_t) error {
				if err := unix.Fstat(fd, stat); err != nil {
					return err
				}
				stat.Mode = unix.S_IFREG | 0o755
				return nil
			}
		}, want: "not a directory"},
		{name: "mkdir", set: func() { mkdirAt = func(int, string, uint32) error { return errors.New("mkdir fault") } }, want: "mkdir fault"},
		{name: "open log", set: func() {
			original := openAt
			openAt = func(fd int, path string, flags int, mode uint32) (int, error) {
				if path == "context-spills.log" {
					return -1, errors.New("log fault")
				}
				return original(fd, path, flags, mode)
			}
		}, want: "log fault"},
		{name: "fstat", set: func() {
			calls := 0
			fstatFile = func(fd int, stat *unix.Stat_t) error {
				calls++
				if calls == 3 {
					return errors.New("fstat fault")
				}
				return unix.Fstat(fd, stat)
			}
		}, want: "fstat fault"},
		{name: "nonregular", set: func() { alterLogStat(func(stat *unix.Stat_t) { stat.Mode = unix.S_IFDIR | 0o600 }) }, want: "not a regular file"},
		{name: "file mode", set: func() { alterLogStat(func(stat *unix.Stat_t) { stat.Mode = unix.S_IFREG | 0o644 }) }, want: "want 0600"},
		{name: "file owner", set: func() { alterLogStat(func(stat *unix.Stat_t) { stat.Uid++ }) }, want: "another user"},
		{name: "lock", set: func() { flockFile = func(int, int) error { return errors.New("lock fault") } }, want: "lock fault"},
		{name: "write", set: func() { writeFile = func(int, []byte) (int, error) { return 0, errors.New("write fault") } }, want: "write fault"},
		{name: "short write", set: func() { writeFile = func(int, []byte) (int, error) { return 0, nil } }, want: "short write"},
		{name: "sync", set: func() { fsyncFile = func(int) error { return errors.New("sync fault") } }, want: "sync fault"},
		{name: "unlock", set: func() {
			flockFile = func(fd int, operation int) error {
				if operation == unix.LOCK_UN {
					return errors.New("unlock fault")
				}
				return unix.Flock(fd, operation)
			}
		}, want: "unlock fault"},
		{name: "close", set: func() { closeFile = func(fd int) error { _ = unix.Close(fd); return errors.New("close fault") } }, want: "close fault"},
		{name: "first error preserved", set: func() {
			writeFile = func(int, []byte) (int, error) { return 0, errors.New("primary write fault") }
			flockFile = func(fd int, operation int) error {
				if operation == unix.LOCK_UN {
					return errors.New("later unlock fault")
				}
				return unix.Flock(fd, operation)
			}
			closeFile = func(fd int) error { _ = unix.Close(fd); return errors.New("later close fault") }
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

func testLogDirectoryValidationAndDescriptorAnchoring(t *testing.T) {
	for name, prepare := range map[string]func(string) error{
		"missing awf": func(string) error { return nil },
		"awf file":    func(root string) error { return os.WriteFile(filepath.Join(root, ".awf"), nil, 0o600) },
		"local file": func(root string) error {
			if err := os.Mkdir(filepath.Join(root, ".awf"), 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(root, ".awf", "local"), nil, 0o600)
		},
		"local mode": func(root string) error { return os.MkdirAll(filepath.Join(root, ".awf", "local"), 0o755) },
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
		fstatFile = func(fd int, stat *unix.Stat_t) error {
			if err := unix.Fstat(fd, stat); err != nil {
				return err
			}
			stat.Uid++
			return nil
		}
		if err := Log(root, Notice{}, nil); err == nil || !strings.Contains(err.Error(), "another user") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("substituted pathname cannot redirect log", func(t *testing.T) {
		restoreLogOps(t)
		root := t.TempDir()
		awf := filepath.Join(root, ".awf")
		local := filepath.Join(awf, "local")
		if err := os.MkdirAll(local, 0o700); err != nil {
			t.Fatal(err)
		}
		original := openAt
		swapped := false
		openAt = func(fd int, path string, flags int, mode uint32) (int, error) {
			if path == "context-spills.log" && !swapped {
				swapped = true
				if err := os.Rename(awf, awf+".anchored"); err != nil {
					return -1, err
				}
				if err := os.MkdirAll(filepath.Join(awf, "local"), 0o700); err != nil {
					return -1, err
				}
			}
			return original(fd, path, flags, mode)
		}
		if err := Log(root, Notice{Bytes: 7}, []string{"x"}); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(awf, "local", "context-spills.log")); !os.IsNotExist(err) {
			t.Fatalf("replacement received log: %v", err)
		}
		if _, err := os.Stat(filepath.Join(awf+".anchored", "local", "context-spills.log")); err != nil {
			t.Fatalf("anchored log missing: %v", err)
		}
	})
}

func testHasSafeLogRejectsForeignOwnerAndMissingPaths(t *testing.T) {
	restoreLogOps(t)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if nonempty, err := HasSafeLog(root); err != nil || nonempty {
		t.Fatalf("missing local = %v, %v", nonempty, err)
	}
	local := filepath.Join(root, ".awf", "local")
	if err := os.Mkdir(local, 0o700); err != nil {
		t.Fatal(err)
	}
	if nonempty, err := HasSafeLog(root); err != nil || nonempty {
		t.Fatalf("missing log = %v, %v", nonempty, err)
	}
	if err := os.WriteFile(filepath.Join(local, "context-spills.log"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if nonempty, err := HasSafeLog(root); err != nil || !nonempty {
		t.Fatalf("nonempty log = %v, %v", nonempty, err)
	}
	alterLogStat(func(stat *unix.Stat_t) { stat.Uid++ })
	if _, err := HasSafeLog(root); err == nil || !strings.Contains(err.Error(), "another user") {
		t.Fatalf("foreign owner error = %v", err)
	}
}

func testHasSafeLogOperationalErrors(t *testing.T) {
	for _, test := range []struct {
		name string
		set  func()
	}{
		{name: "root", set: func() { openAt = func(int, string, int, uint32) (int, error) { return -1, errors.New("root") } }},
		{name: "awf", set: func() {
			original := openAt
			openAt = func(fd int, path string, flags int, mode uint32) (int, error) {
				if path == ".awf" {
					return -1, errors.New("awf")
				}
				return original(fd, path, flags, mode)
			}
		}},
		{name: "local", set: func() {
			original := openAt
			openAt = func(fd int, path string, flags int, mode uint32) (int, error) {
				if path == "local" {
					return -1, errors.New("local")
				}
				return original(fd, path, flags, mode)
			}
		}},
		{name: "log open", set: func() {
			original := openAt
			openAt = func(fd int, path string, flags int, mode uint32) (int, error) {
				if path == "context-spills.log" {
					return -1, errors.New("log")
				}
				return original(fd, path, flags, mode)
			}
		}},
		{name: "log fstat", set: func() {
			calls := 0
			fstatFile = func(fd int, stat *unix.Stat_t) error {
				calls++
				if calls == 3 {
					return errors.New("fstat")
				}
				return unix.Fstat(fd, stat)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			restoreLogOps(t)
			root := t.TempDir()
			local := filepath.Join(root, ".awf", "local")
			if err := os.MkdirAll(local, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(local, "context-spills.log"), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
			test.set()
			if _, err := HasSafeLog(root); err == nil {
				t.Fatal("expected inspection error")
			}
		})
	}
}

func testWriteAllFDHandlesPartialWrites(t *testing.T) {
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

func alterLogStat(alter func(*unix.Stat_t)) {
	calls := 0
	fstatFile = func(fd int, stat *unix.Stat_t) error {
		calls++
		if err := unix.Fstat(fd, stat); err != nil {
			return err
		}
		if calls == 3 {
			alter(stat)
		}
		return nil
	}
}

func restoreLogOps(t *testing.T) {
	t.Helper()
	oldNow, oldOpen, oldMkdir := now, openAt, mkdirAt
	oldFstat, oldFlock, oldWrite := fstatFile, flockFile, writeFile
	oldFsync, oldClose := fsyncFile, closeFile
	t.Cleanup(func() {
		now, openAt, mkdirAt = oldNow, oldOpen, oldMkdir
		fstatFile, flockFile, writeFile = oldFstat, oldFlock, oldWrite
		fsyncFile, closeFile = oldFsync, oldClose
	})
	now = time.Now
	openAt = unix.Openat
	mkdirAt = unix.Mkdirat
	fstatFile = unix.Fstat
	flockFile = unix.Flock
	writeFile = unix.Write
	fsyncFile = unix.Fsync
	closeFile = unix.Close
}
