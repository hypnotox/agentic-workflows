package dashboardruntime

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func resetPathSeams(t *testing.T) {
	t.Helper()
	oldLstat, oldMkdirAll := pathLstat, pathMkdirAll
	t.Cleanup(func() { pathLstat, pathMkdirAll = oldLstat, oldMkdirAll })
}

func TestPrivatePathFaults(t *testing.T) {
	boom := errors.New("injected")
	base := t.TempDir()
	xdg, outside := filepath.Join(base, "xdg"), filepath.Join(base, "outside")
	if err := os.Mkdir(xdg, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := ensurePrivateCacheRoot(xdg, outside); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("escape error = %v", err)
	}
	t.Run("mkdir", func(t *testing.T) {
		resetPathSeams(t)
		pathLstat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
		pathMkdirAll = func(string, os.FileMode) error { return boom }
		if err := ensurePrivateDirectory("missing"); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("lstat", func(t *testing.T) {
		resetPathSeams(t)
		pathLstat = func(string) (os.FileInfo, error) { return nil, boom }
		if err := ensurePrivateDirectory("bad"); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("error = %v", err)
		}
	})
	public := t.TempDir()
	if err := os.Chmod(public, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ensurePrivateDirectory(public); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("public error = %v", err)
	}
	if err := ensurePrivateCacheRoot(public, filepath.Join(public, "awf")); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("public root error = %v", err)
	}
	if got := splitPath(filepath.Join(string(filepath.Separator), "one", "two")); len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Fatalf("splitPath = %v", got)
	}
}
