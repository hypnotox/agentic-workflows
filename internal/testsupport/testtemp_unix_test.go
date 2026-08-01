//go:build linux || darwin

package testsupport

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTestTempUnixProductionRoot(t *testing.T) {
	root, err := testTempRoot()
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("awf-test-homes-%d", os.Geteuid())
	if filepath.Base(root) != want {
		t.Fatalf("root basename = %q, want %q", filepath.Base(root), want)
	}
	m, err := productionTestTempManager()
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ensureRoot(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(root)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("root permissions = %o", info.Mode().Perm())
	}
}

type unavailableStatFileInfo struct{ os.FileInfo }

func (unavailableStatFileInfo) Sys() any { return nil }

func TestTestTempUnixValidatorRejectsSymlinkAndUnavailableOwnership(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(root, "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateTestTempPath(link, info); err == nil {
		t.Fatal("symlink accepted")
	}
	info, err = os.Lstat(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateTestTempPath(root, unavailableStatFileInfo{info}); err == nil {
		t.Fatal("unavailable ownership accepted")
	}
}

func TestTestTempUnixRejectsOpenRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	m, err := newTestTempManager(root, time.Now, osTestTempFS(), validateTestTempPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ensureRoot(); err == nil {
		t.Fatal("chmod-created unsafe root accepted")
	}
}
