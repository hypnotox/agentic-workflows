package testsupport

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestTreeSeedRefusesInvalidSourcesAndEmptySeed(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := CaptureTree(missing); err == nil || !strings.Contains(err.Error(), "capture tree seed root") {
		t.Fatalf("missing source error = %v", err)
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CaptureTree(file); err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("file source error = %v", err)
	}
	if err := (TreeSeed{}).Clone(filepath.Join(t.TempDir(), "clone")); err == nil || err.Error() != "clone tree seed: empty seed" {
		t.Fatalf("empty seed error = %v", err)
	}
}

func TestTreeSeedRejectsMalformedAndUnsafeArchives(t *testing.T) {
	archive := func(t *testing.T, header tar.Header) []byte {
		t.Helper()
		var body bytes.Buffer
		writer := tar.NewWriter(&body)
		if err := writer.WriteHeader(&header); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		return body.Bytes()
	}
	for _, tc := range []struct {
		name, want string
		body       []byte
	}{
		{name: "malformed", want: "clone tree seed", body: []byte("not a tar archive")},
		{name: "escaping path", want: "invalid archived path", body: archive(t, tar.Header{Name: "../escape", Typeflag: tar.TypeReg, Mode: 0o644})},
		{name: "unsupported entry", want: "unsupported entry", body: archive(t, tar.Header{Name: "pipe", Typeflag: tar.TypeFifo, Mode: 0o644})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seed := TreeSeed{archive: tc.body, rootMode: 0o755, rootModified: time.Unix(1, 0)}
			if err := seed.Clone(filepath.Join(t.TempDir(), "clone")); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Clone() error = %v, want %q", err, tc.want)
			}
		})
	}
}
