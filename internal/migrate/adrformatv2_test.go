package migrate

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func v2MigrationProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-one.md"), testsupport.ADR("Proposed", testsupport.WithTitle("0001: One")))
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0003-three.md"), testsupport.ADR("Proposed", testsupport.WithTitle("0003: Three")))
	lock := &manifest.Lock{AWFVersion: "0.19.0", SchemaVersion: 14, Files: map[string]manifest.Entry{}, ADRFormatV1From: 1, LegacyADRGaps: []int{}}
	if err := lock.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	return root
}

// invariant: config/migrations-and-locks:adr-v2-cutoff-atomic-immutable
func TestApplyADRFormatV2CutoffSavesAtomicAuthorityOnce(t *testing.T) {
	root := v2MigrationProject(t)
	path := config.LockPath(root)
	adrBefore := map[string][]byte{}
	for _, name := range []string{"0001-one.md", "0003-three.md"} {
		body, err := os.ReadFile(filepath.Join(root, "docs/decisions", name))
		if err != nil {
			t.Fatal(err)
		}
		adrBefore[name] = body
	}
	calls := 0
	var saved *manifest.Lock
	err := applyADRFormatV2CutoffWithSave(root, io.Discard, func(lock *manifest.Lock, gotPath string) error {
		calls++
		if gotPath != path {
			t.Errorf("save path = %q, want %q", gotPath, path)
		}
		copy := *lock
		saved = &copy
		return lock.Save(gotPath)
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("save calls = %d, want 1", calls)
	}
	if saved.SchemaVersion != 15 || saved.AWFVersion != "0.20.0" || saved.ADRFormatV1From != 1 || saved.ADRFormatV2From != 4 || len(saved.LegacyADRGaps) != 0 {
		t.Fatalf("saved authority = %#v", saved)
	}
	for name, want := range adrBefore {
		got, err := os.ReadFile(filepath.Join(root, "docs/decisions", name))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("migration changed ADR %s: err=%v", name, err)
		}
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyADRFormatV2CutoffWithSave(root, io.Discard, func(*manifest.Lock, string) error {
		t.Fatal("idempotent migration called saver")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("idempotent migration changed lock bytes")
	}
}

func TestApplyADRFormatV2CutoffInputFailuresDoNotSave(t *testing.T) {
	missing := t.TempDir()
	if err := applyADRFormatV2CutoffWithSave(missing, io.Discard, func(*manifest.Lock, string) error {
		t.Fatal("missing lock called saver")
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	corrupt := t.TempDir()
	testsupport.WriteFile(t, config.LockPath(corrupt), "{bad")
	if err := applyADRFormatV2CutoffWithSave(corrupt, io.Discard, func(*manifest.Lock, string) error { return nil }); err == nil {
		t.Fatal("corrupt lock accepted")
	}

	invalidConfig := v2MigrationProject(t)
	testsupport.WriteFile(t, config.ConfigPath(invalidConfig), "unknown: true\n")
	if err := applyADRFormatV2CutoffWithSave(invalidConfig, io.Discard, func(*manifest.Lock, string) error { return nil }); err == nil {
		t.Fatal("invalid config accepted")
	}

	invalidADR := v2MigrationProject(t)
	testsupport.WriteFile(t, filepath.Join(invalidADR, "docs/decisions/0004-bad.md"), "---\nstatus: [bad\n---\n")
	if err := applyADRFormatV2CutoffWithSave(invalidADR, io.Discard, func(*manifest.Lock, string) error { return nil }); err == nil {
		t.Fatal("invalid ADR accepted")
	}
}

func TestApplyADRFormatV2CutoffFailurePreservesLockBytes(t *testing.T) {
	root := v2MigrationProject(t)
	path := config.LockPath(root)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("injected save failure")
	if err := applyADRFormatV2CutoffWithSave(root, io.Discard, func(*manifest.Lock, string) error { return want }); !errors.Is(err, want) {
		t.Fatalf("error = %v, want injected failure", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("failing saver changed lock bytes")
	}
}
