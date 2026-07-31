package migrate

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func v3MigrationProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-one.md"), testsupport.ADR("Proposed", testsupport.WithTitle("0001: One")))
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0003-three.md"), testsupport.ADR("Proposed", testsupport.WithTitle("0003: Three")))
	lock := &manifest.Lock{AWFVersion: "0.30.0", SchemaVersion: 27, Files: map[string]manifest.Entry{}, ADRFormatV1From: 1, ADRFormatV2From: 2, LegacyADRGaps: []int{}}
	if err := lock.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	return root
}

// invariant: config/migrations-and-locks:adr-v2-cutoff-atomic-immutable
func TestApplyADRFormatV3CutoffSavesAtomicAuthorityOnce(t *testing.T) {
	root := v3MigrationProject(t)
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
	var out bytes.Buffer
	var saved *manifest.Lock
	err := applyADRFormatV3CutoffWithSave(root, &out, func(lock *manifest.Lock, gotPath string) error {
		calls++
		if gotPath != path {
			t.Errorf("save path = %q, want %q", gotPath, path)
		}
		copied := *lock
		saved = &copied
		return lock.Save(gotPath)
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("save calls = %d, want 1", calls)
	}
	if saved.SchemaVersion != adrFormatV3Generation || saved.AWFVersion != "0.30.0" || saved.ADRFormatV1From != 1 || saved.ADRFormatV2From != 2 || saved.ADRFormatV3From != 4 {
		t.Fatalf("saved authority = %#v", saved)
	}
	if out.String() != "adr-format-v3-cutoff: sealed ADR V3 cutoff at 4\n" {
		t.Fatalf("output = %q", out.String())
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
	if err := applyADRFormatV3CutoffWithSave(root, io.Discard, func(*manifest.Lock, string) error {
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

// A tree without permanent authority gains only the schema stamp: there is no
// corpus boundary to seal until the current-state cutover promotes one.
func TestApplyADRFormatV3CutoffStampsNonPermanentAuthority(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\n")
	lock := &manifest.Lock{AWFVersion: "0.30.0", SchemaVersion: 27, Files: map[string]manifest.Entry{}}
	if err := lock.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := applyADRFormatV3Cutoff(root, &out); err != nil {
		t.Fatal(err)
	}
	got, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != adrFormatV3Generation || got.ADRFormatV3From != 0 || out.Len() != 0 {
		t.Fatalf("pre-tracking authority = %#v output=%q", got, out.String())
	}
}

func TestApplyADRFormatV3CutoffInputFailuresDoNotSave(t *testing.T) {
	missing := t.TempDir()
	if err := applyADRFormatV3CutoffWithSave(missing, io.Discard, func(*manifest.Lock, string) error {
		t.Fatal("missing lock called saver")
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	corrupt := t.TempDir()
	testsupport.WriteFile(t, config.LockPath(corrupt), "{bad")
	if err := applyADRFormatV3CutoffWithSave(corrupt, io.Discard, func(*manifest.Lock, string) error { return nil }); err == nil {
		t.Fatal("corrupt lock accepted")
	}

	invalidConfig := v3MigrationProject(t)
	testsupport.WriteFile(t, config.ConfigPath(invalidConfig), "unknown: true\n")
	if err := applyADRFormatV3CutoffWithSave(invalidConfig, io.Discard, func(*manifest.Lock, string) error { return nil }); err == nil {
		t.Fatal("invalid config accepted")
	}

	// A stray file in the decisions directory is now a corpus error, so the
	// cutoff computation refuses rather than sealing against a partial corpus.
	strayFile := v3MigrationProject(t)
	testsupport.WriteFile(t, filepath.Join(strayFile, "docs/decisions/notes.md"), "# Notes\n")
	err := applyADRFormatV3CutoffWithSave(strayFile, io.Discard, func(*manifest.Lock, string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "compute ADR V3 cutoff") {
		t.Fatalf("stray decisions file = %v", err)
	}
}

func TestApplyADRFormatV3CutoffFailurePreservesLockBytes(t *testing.T) {
	root := v3MigrationProject(t)
	path := config.LockPath(root)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("injected save failure")
	if err := applyADRFormatV3CutoffWithSave(root, io.Discard, func(*manifest.Lock, string) error { return want }); !errors.Is(err, want) {
		t.Fatalf("error = %v, want injected failure", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("failing saver changed lock bytes")
	}
}
