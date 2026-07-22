package telemetry

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenLedgerValidationAndOptionalDirectories(t *testing.T) {
	if _, err := OpenLedger("relative"); err == nil {
		t.Fatal("relative project root opened")
	}
	if _, err := OpenLedger(t.TempDir()); err == nil || !strings.Contains(err.Error(), "project anchor") {
		t.Fatalf("missing project anchor error = %v", err)
	}

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".awf", "metrics"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenLedger(root); err != nil {
		t.Fatalf("missing optional telemetry directories: %v", err)
	}

	invalidRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(invalidRoot, ".awf"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(invalidRoot, ".awf", "metrics"), []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenLedger(invalidRoot); err == nil || !strings.Contains(err.Error(), "open telemetry storage") {
		t.Fatalf("invalid storage root error = %v", err)
	}
}

func TestOpenLedgerOptionalDirectoryFaults(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{
		filepath.Join(root, ".awf", "metrics", "efforts"),
		filepath.Join(root, ".awf", "metrics", "tombstones"),
	} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	ledger, err := newLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	originalLstat := ledger.ops.lstat
	ledger.ops.lstat = func(path string) (os.FileInfo, error) {
		if path == ledger.paths.efforts {
			return nil, errors.New("injected lstat failure")
		}
		return originalLstat(path)
	}
	if _, err := openLedger(ledger); err == nil {
		t.Fatal("optional directory lstat failure ignored")
	}

	ledger, err = newLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	originalInspect := ledger.ops.inspect
	ledger.ops.inspect = func(base, path string, directory bool) error {
		if path == ledger.paths.tombstones {
			return errors.New("injected inspect failure")
		}
		return originalInspect(base, path, directory)
	}
	if _, err := openLedger(ledger); err == nil {
		t.Fatal("optional directory inspection failure ignored")
	}
}

func TestReadAllEffortsDirectoryFaults(t *testing.T) {
	ledger := newRetentionLedger(t)
	ledger.ops.readDir = func(string) ([]os.DirEntry, error) { return nil, os.ErrNotExist }
	reads, err := ledger.ReadAllEfforts()
	if err != nil || len(reads) != 0 {
		t.Fatalf("missing efforts directory = %#v, %v", reads, err)
	}

	ledger = newRetentionLedger(t)
	ledger.ops.readDir = func(string) ([]os.DirEntry, error) { return nil, errors.New("injected read failure") }
	if _, err := ledger.ReadAllEfforts(); err == nil || !strings.Contains(err.Error(), "read telemetry efforts") {
		t.Fatalf("efforts directory read failure = %v", err)
	}

	ledger = newRetentionLedger(t)
	if err := os.Mkdir(filepath.Join(ledger.paths.efforts, "effort"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.ReadAllEfforts(); err == nil || !strings.Contains(err.Error(), "read telemetry effort "+"effort") {
		t.Fatalf("effort read failure = %v", err)
	}
}
