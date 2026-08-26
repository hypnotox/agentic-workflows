//go:build linux

package filesystem

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/filepublication"
	"golang.org/x/sys/unix"
)

func TestRetirementReservationRemovalFailureRestoresDestinationAndReportsResidue(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootPath, "destination", "payload"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(rootPath, "temporary", "concurrent"), 0o700); err != nil {
		t.Fatal(err)
	}
	h, err := Open(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	expected, err := h.root.Lstat("destination")
	if err != nil {
		t.Fatal(err)
	}

	consumed, err := exchangeExpected(h.root, "temporary", "destination", expected, true, true)
	var cleanup *filepublication.CommittedCleanupError
	if !consumed || !errors.As(err, &cleanup) {
		t.Fatalf("retirement failure = consumed %t, error %v; want structured committed residue", consumed, err)
	}
	if cleanup.DestinationPath != "destination" || cleanup.ResiduePath != "temporary" {
		t.Fatalf("retirement paths = destination %q, residue %q", cleanup.DestinationPath, cleanup.ResiduePath)
	}
	if _, err := os.Stat(filepath.Join(rootPath, "destination", "payload")); err != nil {
		t.Fatalf("observed destination was not restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(rootPath, "temporary", "concurrent")); err != nil {
		t.Fatalf("concurrent residue was not preserved: %v", err)
	}
}

func TestExpectedMutationNeverTouchesRelocatedParentThroughEscapingSymlink(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "replace", true: "remove"}[remove], func(t *testing.T) {
			container := t.TempDir()
			rootPath := filepath.Join(container, "root")
			outside := filepath.Join(container, "outside")
			if err := os.MkdirAll(filepath.Join(rootPath, "parent"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(outside, 0o755); err != nil {
				t.Fatal(err)
			}
			h, err := Open(rootPath)
			if err != nil {
				t.Fatal(err)
			}
			defer h.Close()
			if err := os.WriteFile(filepath.Join(rootPath, "parent", "destination"), []byte("observed"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(rootPath, "parent", "temporary"), []byte("replacement"), 0o640); err != nil {
				t.Fatal(err)
			}
			expected, err := h.root.Lstat("parent/destination")
			if err != nil {
				t.Fatal(err)
			}
			relocated := filepath.Join(outside, "parent")
			if err := os.Rename(filepath.Join(rootPath, "parent"), relocated); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(relocated, filepath.Join(rootPath, "parent")); err != nil {
				t.Skipf("directory symlink unsupported: %v", err)
			}

			watch, err := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
			if err != nil {
				t.Fatal(err)
			}
			defer unix.Close(watch)
			if _, err := unix.InotifyAddWatch(watch, relocated, unix.IN_MOVED_FROM|unix.IN_MOVED_TO|unix.IN_CREATE|unix.IN_DELETE); err != nil {
				t.Fatal(err)
			}

			consumed, err := exchangeExpected(h.root, "parent/temporary", "parent/destination", expected, remove, false)
			if err == nil || consumed {
				t.Fatalf("escaping-parent commit = consumed %v, error %v; want uncommitted refusal", consumed, err)
			}
			if got, err := os.ReadFile(filepath.Join(relocated, "destination")); err != nil || string(got) != "observed" {
				t.Fatalf("outside destination = %q, %v", got, err)
			}
			if got, err := os.ReadFile(filepath.Join(relocated, "temporary")); err != nil || string(got) != "replacement" {
				t.Fatalf("outside temporary = %q, %v", got, err)
			}
			var events [4096]byte
			if n, err := unix.Read(watch, events[:]); err == nil && n != 0 {
				t.Fatalf("expected mutation transiently changed relocated outside parent: %d inotify bytes", n)
			} else if err != nil && !errors.Is(err, unix.EAGAIN) {
				t.Fatal(err)
			}
		})
	}
}
