package testsupport

import (
	"os"
	"testing"
)

// ShortTempDir returns a cleanup-owned temporary directory without embedding
// the test name, keeping Unix socket fixtures below Darwin's path limit.
func ShortTempDir(t testing.TB) string {
	t.Helper()
	root, err := os.MkdirTemp("", "t") //nolint:usetesting // t.TempDir embeds the test name and can exceed Darwin's Unix-socket path limit.
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove short temporary directory: %v", err)
		}
	})
	return root
}
