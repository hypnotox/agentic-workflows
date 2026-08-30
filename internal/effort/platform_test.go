package effort

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/filepublication"
)

func testPlatformPublicationContract(t *testing.T) {
	t.Helper()
	t.Run("creation refuses raced destination", func(t *testing.T) {
		dir := t.TempDir()
		destination := filepath.Join(dir, "record.json")
		writeEffortFile(t, destination, "unexpected")
		if err := filepublication.Publish(destination, []byte("new"), 0o600); err == nil {
			t.Fatal("creation replaced an existing destination")
		}
		if raw, err := os.ReadFile(destination); err != nil || string(raw) != "unexpected" {
			t.Fatalf("raced creation destination = %q, %v", raw, err)
		}
	})
}

// invariant: tooling/file-publication:exclusive-file-publication-single-home (TestPlatformPublicationContract)
func TestPlatformPublicationContract(t *testing.T) {
	testPlatformPublicationContract(t)
}
