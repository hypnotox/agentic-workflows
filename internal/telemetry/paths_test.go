package telemetry

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestTelemetryPathsRejectTraversalAndUnsafeTypes(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", ".", "..", "a/b", `a\b`} {
		if err := validatePathIdentifier("effortId", value); err == nil {
			t.Fatalf("unsafe identifier %q accepted", value)
		}
	}
	root := t.TempDir()
	private := filepath.Join(root, "private")
	if err := os.Mkdir(private, ownerOnlyMode); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(private, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := inspectConfined(private, file, false); err != nil {
		t.Fatalf("private regular file refused: %v", err)
	}
	if err := inspectConfined(private, filepath.Join(root, "outside"), false); err == nil {
		t.Fatal("escaping path accepted")
	}
	link := filepath.Join(private, "link")
	if err := os.Symlink(file, link); err == nil {
		if err := inspectConfined(private, link, false); err == nil {
			t.Fatal("symlink accepted")
		}
	}
	if err := inspectConfined(private, private, true); err != nil {
		t.Fatalf("root itself refused: %v", err)
	}
	if err := inspectPath(filepath.Join(private, "missing"), false); err == nil {
		t.Fatal("missing path accepted")
	}
	if err := inspectPath(file, true); err == nil {
		t.Fatal("file accepted as directory")
	}
	if err := os.Chmod(private, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := inspectConfined(private, file, false); runtime.GOOS != "windows" && err == nil {
		t.Fatal("unsafe confinement root accepted")
	}
}

func TestLeaseOperationGuardRejectsSymlinkOrReparse(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "guard")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := lockLeaseOperations(t.Context(), link); err == nil {
		t.Fatal("symlinked lease operation guard accepted")
	}
}

func TestNewLedgerAnchorsConfinementBeforeCreatingMetrics(t *testing.T) {
	t.Parallel()
	if _, err := NewLedger("relative-project"); err == nil {
		t.Fatal("relative project root accepted")
	}
	root := t.TempDir()
	if _, err := NewLedger(root); err == nil {
		t.Fatal("project without .awf anchor accepted")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".awf")); err == nil {
		if _, err := NewLedger(root); err == nil {
			t.Fatal("symlinked .awf anchor accepted")
		}
		if _, err := os.Lstat(filepath.Join(outside, "metrics")); !os.IsNotExist(err) {
			t.Fatalf("metrics created through unsafe anchor: %v", err)
		}
	}

	root = t.TempDir()
	awf := filepath.Join(root, ".awf")
	metrics := filepath.Join(awf, "metrics")
	if err := os.MkdirAll(metrics, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(metrics, "efforts")); err == nil {
		if _, err := NewLedger(root); err == nil {
			t.Fatal("existing unsafe telemetry component accepted")
		}
		if _, err := os.Lstat(filepath.Join(metrics, "leases")); !os.IsNotExist(err) {
			t.Fatalf("later telemetry component created before validation completed: %v", err)
		}
	}
}

func TestTelemetryPathsPermissionsAndOwnershipCapabilities(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "metrics")
	if err := os.MkdirAll(root, ownerOnlyMode); err != nil {
		t.Fatal(err)
	}
	if err := inspectPath(root, true); err != nil {
		t.Fatalf("private directory rejected: %v", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(root, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := inspectPath(root, true); err == nil {
			t.Fatal("non-private mode accepted")
		}
	}
	if _, ok := fileUID(fakeFileInfo{sys: nil}); ok {
		t.Fatal("missing ownership capability reported as supported")
	}
	if _, ok := fileUID(fakeFileInfo{sys: struct{ Other uint32 }{}}); ok {
		t.Fatal("foreign ownership shape reported as supported")
	}
	if _, ok := fileUID(fakeFileInfo{sys: (*struct{ Uid uint32 })(nil)}); ok {
		t.Fatal("nil ownership pointer accepted")
	}
	if _, ok := fileUID(fakeFileInfo{sys: new(int)}); ok {
		t.Fatal("non-struct ownership pointer accepted")
	}
	if _, ok := fileUID(fakeFileInfo{sys: &struct{ Uid int }{Uid: 42}}); ok {
		t.Fatal("signed ownership field accepted")
	}
	if uid, ok := fileUID(fakeFileInfo{sys: &struct{ Uid uint32 }{Uid: 42}}); !ok || uid != 42 {
		t.Fatalf("supported ownership shape not read: %d %v", uid, ok)
	}
	if _, err := currentUID(); runtime.GOOS != "windows" && err != nil {
		t.Fatalf("current UID unavailable on POSIX: %v", err)
	}
	first, err := currentOwnerIdentity()
	if err != nil || first == "" {
		t.Fatalf("owner identity unavailable: %q %v", first, err)
	}
	second, err := currentOwnerIdentity()
	if err != nil || second != first {
		t.Fatalf("owner identity is unstable: %q %q %v", first, second, err)
	}
}

type fakeFileInfo struct{ sys any }

func (fakeFileInfo) Name() string       { return "fake" }
func (fakeFileInfo) Size() int64        { return 0 }
func (fakeFileInfo) Mode() fs.FileMode  { return 0 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any         { return f.sys }
