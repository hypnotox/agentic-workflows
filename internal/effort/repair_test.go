package effort

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProtocol1AndMutableRepairShapesAreRejected(t *testing.T) {
	root := initEffortRepo(t)
	service, err := Open(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for name, raw := range map[string]string{
		"protocol1": `{"schemaVersion":1,"id":"018f47a0-7b3d-4c52-8f1a-123456789abc","slug":"legacy-shape","title":"Legacy shape","createdAt":"2026-07-29T12:00:00Z"}`,
		"mutable":   `{"schemaVersion":2,"id":"018f47a0-7b3d-4c52-8f1a-123456789abc","slug":"legacy-shape","title":"Legacy shape","createdAt":"2026-07-29T12:00:00Z","state":"active"}`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(root, ".awf", "efforts", "legacy-shape")
			if err := os.RemoveAll(dir); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			writeEffortFile(t, filepath.Join(dir, "memory.md"), "Effort: legacy-shape\n")
			writeEffortFile(t, filepath.Join(dir, "state.json"), raw)
			_, err := service.Show("legacy-shape")
			if err == nil || !strings.Contains(err.Error(), "preserve") {
				t.Fatalf("legacy state accepted: %v", err)
			}
		})
	}
}
