package telemetry

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePathsAndFileTypeInspection(t *testing.T) {
	if _, err := resolvePaths(context.Background(), t.TempDir()); err == nil {
		t.Fatal("resolvePaths accepted a non-repository")
	}
	root := telemetryRepo(t)
	unsafeRoot := telemetryRepo(t)
	if err := os.MkdirAll(filepath.Join(unsafeRoot, ".awf"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(unsafeRoot, ".awf", "metrics")); err != nil {
		t.Fatal(err)
	}
	if _, err := resolvePaths(t.Context(), unsafeRoot); err == nil {
		t.Fatal("resolvePaths accepted a symlinked metrics root")
	}
	resolved, err := resolvePaths(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.root != filepath.Join(root, ".awf", "metrics") || resolved.sessions != filepath.Join(resolved.root, "sessions") || resolved.efforts != filepath.Join(resolved.root, "efforts") {
		t.Fatalf("resolved paths = %#v", resolved)
	}
	dir := t.TempDir()
	regular := filepath.Join(dir, "regular")
	if err := os.WriteFile(regular, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if info, err := inspectRegular(regular); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("inspect regular = %v, %v", info, err)
	}
	if _, err := inspectRegular(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("inspectRegular accepted missing file")
	}
	if _, err := inspectRegular(dir); err == nil {
		t.Fatal("inspectRegular accepted directory")
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(regular, link); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectRegular(link); err == nil {
		t.Fatal("inspectRegular accepted symlink")
	}
	if info, err := inspectDirectory(dir); err != nil || !info.IsDir() {
		t.Fatalf("inspect directory = %v, %v", info, err)
	}
	if _, err := inspectDirectory(filepath.Join(dir, "missing-dir")); err == nil {
		t.Fatal("inspectDirectory accepted missing path")
	}
	if _, err := inspectDirectory(regular); err == nil {
		t.Fatal("inspectDirectory accepted regular file")
	}
	if _, err := inspectDirectory(link); err == nil {
		t.Fatal("inspectDirectory accepted symlink")
	}
}
