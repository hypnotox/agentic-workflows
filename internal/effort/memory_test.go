package effort

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEffortMemoryTruthAndIdempotence(t *testing.T) {
	root := initEffortRepo(t)
	service := openEffortService(t, root, time.Now().UTC())
	if _, err := service.New("Memory", false); err != nil {
		t.Fatal(err)
	}
	if present, err := service.paths.memoryTruth(idA); err != nil || present {
		t.Fatalf("absent memory = %v, %v", present, err)
	}
	path, _, err := service.Memory(idA)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Memory(idA); err != nil {
		t.Fatalf("owned memory is not idempotent: %v", err)
	}
	writeEffortFile(t, path, "Effort: other\n")
	if _, err := service.paths.memoryTruth(idA); err == nil {
		t.Fatal("foreign memory accepted as truth")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := service.paths.memoryTruth(idA); err == nil || !strings.Contains(err.Error(), "file-type") {
		t.Fatalf("directory memory = %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	writeEffortFile(t, outside, "Effort: "+idA+"\n")
	if err := os.Symlink(outside, path); err == nil {
		if _, err := service.paths.memoryTruth(idA); err == nil {
			t.Fatal("symlink memory accepted")
		}
	}
}
