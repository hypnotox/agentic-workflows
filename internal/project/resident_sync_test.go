package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

type residentSnapshot struct {
	content []byte
	mode    os.FileMode
}

func snapshotResidentSyncFixture(t *testing.T, root string) map[string]residentSnapshot {
	t.Helper()
	files := make(map[string]residentSnapshot)
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = residentSnapshot{contents, info.Mode().Perm()}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func assertResidentSyncFixtureUnchanged(t *testing.T, root string, before map[string]residentSnapshot) {
	t.Helper()
	after := snapshotResidentSyncFixture(t, root)
	if len(after) != len(before) {
		t.Fatalf("fixture file count after sync = %d, want %d: %#v", len(after), len(before), after)
	}
	for path, want := range before {
		got, ok := after[path]
		if !ok || string(got.content) != string(want.content) || got.mode != want.mode {
			t.Fatalf("fixture file %s after sync = %#v, want byte/mode-identical %#v", path, got, want)
		}
	}
}

// invariant: rendering/singletons-and-payloads:resident-output-preservation (TestSyncPreservesOwnedResidentRoots)
func TestSyncPreservesOwnedResidentRoots(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"efforts", "worktrees", "effort-archive"} {
		path := filepath.Join(root, ".awf", name, "retained", "nested", "resident")
		testsupport.WriteFile(t, path, name)
		if err := os.Chmod(path, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	before := snapshotResidentSyncFixture(t, root)
	p, err = Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	assertResidentSyncFixtureUnchanged(t, root, before)
}
