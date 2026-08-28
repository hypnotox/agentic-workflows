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
	if err != nil {                    // coverage-ignore: the system temporary root cannot be made unavailable safely or portably
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil { // coverage-ignore: a cleanup fault requires external interference after the test owns the directory
			t.Errorf("remove short temporary directory: %v", err)
		}
	})
	return root
}
