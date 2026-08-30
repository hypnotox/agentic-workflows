package effort

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestEffortResidentsRefuseForeignOwnedBytesUnprivileged proves the refusal
// itself through the injectable owner seam, so an ordinary gate run covers the
// diagnosis that the real-fixture test below can only reach under privilege.
func TestEffortResidentsRefuseForeignOwnedBytesUnprivileged(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(deps *Dependencies) {
		deps.UUID = func() (string, error) { return testIDA, nil }
	})
	if _, err := service.New(testContext(t), NewInput{Slug: "seam-owner", Title: "Seam owner"}); err != nil {
		t.Fatal(err)
	}
	resident := filepath.Join(root, ".awf", "efforts", "seam-owner")
	before, err := os.ReadFile(filepath.Join(resident, "memory.md"))
	if err != nil {
		t.Fatal(err)
	}

	original := residentOwner
	t.Cleanup(func() { residentOwner = original })
	residentOwner = func(path string, _ os.FileInfo, _ *os.File) error {
		return safety("foreign-owner", path, nil)
	}
	if _, err := service.Show("seam-owner"); err == nil || !strings.Contains(err.Error(), "foreign-owner") {
		t.Fatalf("foreign owner refusal = %v", err)
	}
	if _, err := service.List(); err == nil || !strings.Contains(err.Error(), "foreign-owner") {
		t.Fatalf("enumeration foreign owner refusal = %v", err)
	}

	residentOwner = original
	after, err := os.ReadFile(filepath.Join(resident, "memory.md"))
	if err != nil || string(after) != string(before) {
		t.Fatalf("refusal changed resident bytes: %v", err)
	}
}

// TestEffortResidentsRefuseForeignOwnedBytes exercises the no-follow owner check
// against a resident owned by another user. Creating that fixture requires
// privilege, so the body runs only under a privileged test process; the
// production branch it covers carried the retired coverage exclusion marker.
func TestEffortResidentsRefuseForeignOwnedBytes(t *testing.T) {
	if runtime.GOOS == "windows" || testCurrentEUID() != 0 {
		t.Skip("foreign ownership requires a privileged non-Windows test process")
	}
	root := initEffortRepo(t)
	service := openTestService(t, root, func(deps *Dependencies) {
		deps.UUID = func() (string, error) { return testIDA, nil }
	})
	if _, err := service.New(testContext(t), NewInput{Slug: "foreign-owner", Title: "Foreign owner"}); err != nil {
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
		service := openTestService(t, root, nil)
		if _, err := service.List(); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("symlink error = %v", err)
		}
		if _, err := os.Stat(outside); err != nil {
			t.Fatalf("outside target changed: %v", err)
		}
	})
	t.Run("hard-linked memory", func(t *testing.T) {
		root := initEffortRepo(t)
		service := openTestService(t, root, func(deps *Dependencies) {
			deps.UUID = func() (string, error) { return testIDA, nil }
		})
		if _, err := service.New(testContext(t), NewInput{Slug: "hard-link", Title: "Hard link"}); err != nil {
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
