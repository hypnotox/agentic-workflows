package effort

import (
	"os"
	"testing"
)

// invariant: tooling/effort-management:effort-record-authority
func TestPartialAbsentDistinguishesExistingPath(t *testing.T) {
	path := t.TempDir()
	absent, err := partialAbsent(path)
	if err != nil || absent {
		t.Fatalf("existing path classified as absent: absent=%v err=%v", absent, err)
	}
	file := path + "/file"
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if absent, err := partialAbsent(file); err != nil || absent {
		t.Fatalf("existing file classified as absent: absent=%v err=%v", absent, err)
	}
	old := partialLstat
	partialLstat = func(string) (os.FileInfo, error) { return nil, os.ErrPermission }
	defer func() { partialLstat = old }()
	if absent, err := partialAbsent(file); err == nil || absent {
		t.Fatalf("stat error classified as absent: absent=%v err=%v", absent, err)
	}
}

// invariant: tooling/effort-management:effort-record-authority
func TestManagedDirectoryTruthRejectsInjectedForeignOwner(t *testing.T) {
	path := t.TempDir()
	old := residentOwner
	residentOwner = func(string, os.FileInfo) error { return os.ErrPermission }
	defer func() { residentOwner = old }()
	if present, err := managedDirectoryTruth(path); err == nil || present {
		t.Fatalf("foreign owner accepted: present=%v err=%v", present, err)
	}
}

// invariant: tooling/effort-management:effort-record-authority
func TestValidateCurrentOwnerAcceptsCurrentDirectory(t *testing.T) {
	path := t.TempDir()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCurrentOwner(path, info); err != nil {
		t.Fatal(err)
	}
}
