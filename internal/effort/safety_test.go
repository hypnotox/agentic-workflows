package effort

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestEffortResidentsRefuseForeignOwnedBytes exercises the no-follow owner check
// against a resident owned by another user. Creating that fixture requires
// privilege, so the body runs only under a privileged test process; the
// production branch it covers carries a matching coverage-ignore.
func TestEffortResidentsRefuseForeignOwnedBytes(t *testing.T) {
	if runtime.GOOS == "windows" || testCurrentEUID() != 0 {
		t.Skip("foreign ownership requires a privileged non-Windows test process")
	}
	root := initEffortRepo(t)
	service, err := Open(context.Background(), root, Options{UUID: func() (string, error) { return testIDA, nil }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.New("Foreign owner"); err != nil {
		t.Fatal(err)
	}
	resident := filepath.Join(root, ".awf", "efforts", "foreign-owner")
	foreignUID := 1
	if foreignUID == testCurrentEUID() {
		foreignUID = 2
	}
	if err := testChown(resident, foreignUID); err != nil {
		t.Skipf("foreign ownership unavailable: %v", err)
	}
	t.Cleanup(func() { _ = testChown(resident, testCurrentEUID()) })
	if _, err := service.Show("foreign-owner"); err == nil || !strings.Contains(err.Error(), "foreign-owner") {
		t.Fatalf("foreign owner error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(resident, "memory.md")); err != nil {
		t.Fatalf("foreign resident bytes changed: %v", err)
	}
}

func TestEffortResidentsRejectSymlinksAndHardLinksWithoutDeletingTargets(t *testing.T) {
	t.Run("symlink directory", func(t *testing.T) {
		root := initEffortRepo(t)
		outside := t.TempDir()
		link := filepath.Join(root, ".awf", "efforts", "linked-effort")
		if err := os.Symlink(outside, link); err != nil {
			t.Fatal(err)
		}
		service, err := Open(context.Background(), root, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.List(); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("symlink error = %v", err)
		}
		if _, err := os.Stat(outside); err != nil {
			t.Fatalf("outside target changed: %v", err)
		}
	})
	t.Run("hard-linked memory", func(t *testing.T) {
		root := initEffortRepo(t)
		service, err := Open(context.Background(), root, Options{UUID: func() (string, error) { return testIDA, nil }})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.New("Hard link"); err != nil {
			t.Fatal(err)
		}
		memory := filepath.Join(root, ".awf", "efforts", "hard-link", "memory.md")
		outside := filepath.Join(t.TempDir(), "outside.md")
		if err := os.Link(memory, outside); err != nil {
			t.Skipf("hard links unavailable: %v", err)
		}
		if _, err := service.Show("hard-link"); err == nil || !strings.Contains(err.Error(), "hard-link") {
			t.Fatalf("hard-link error = %v", err)
		}
		if _, err := os.Stat(outside); err != nil {
			t.Fatalf("outside link changed: %v", err)
		}
	})
}
