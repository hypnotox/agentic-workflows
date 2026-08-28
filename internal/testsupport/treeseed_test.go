package testsupport

import (
	"os"
	"path/filepath"
	"testing"
)

// invariant: tooling/test-infrastructure:immutable-fixture-seeds (TestTreeSeedPreservesModesLinksAndCloneIsolation)
func TestTreeSeedPreservesModesLinksAndCloneIsolation(t *testing.T) {
	source := t.TempDir()
	if err := os.Chmod(source, 0o710); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(source, "nested"), 0o750); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(source, "nested", "tool")
	if err := os.WriteFile(executable, []byte("original\n"), 0o751); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("nested", "tool"), filepath.Join(source, "tool-link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	seed, err := CaptureTree(source)
	if err != nil {
		t.Fatal(err)
	}
	digest := seed.Digest()
	first := filepath.Join(t.TempDir(), "first")
	second := filepath.Join(t.TempDir(), "second")
	if err := seed.Clone(first); err != nil {
		t.Fatal(err)
	}
	if err := seed.Clone(second); err != nil {
		t.Fatal(err)
	}
	if seed.Digest() != digest {
		t.Fatal("immutable seed digest changed after cloning")
	}
	rootInfo, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	if got := rootInfo.Mode().Perm(); got != 0o710 {
		t.Fatalf("root mode = %o, want 710", got)
	}
	info, err := os.Stat(filepath.Join(first, "nested", "tool"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o751 {
		t.Fatalf("executable mode = %o, want 751", got)
	}
	link, err := os.Readlink(filepath.Join(first, "tool-link"))
	if err != nil {
		t.Fatal(err)
	}
	if link != filepath.Join("nested", "tool") {
		t.Fatalf("link = %q", link)
	}
	if err := os.WriteFile(filepath.Join(first, "nested", "tool"), []byte("mutated\n"), 0o751); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(second, "nested", "tool"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "original\n" {
		t.Fatalf("second clone changed through first: %q", body)
	}
}

func TestTreeSeedRefusesExistingDestination(t *testing.T) {
	seed, err := CaptureTree(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := seed.Clone(t.TempDir()); err == nil {
		t.Fatal("expected existing destination refusal")
	}
}
