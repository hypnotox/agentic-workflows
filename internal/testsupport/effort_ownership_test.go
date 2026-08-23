package testsupport_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestEffortCommandUsesOneFocusedOperationPerLeaf protects the composition
// boundary: each parsed leaf invokes its focused operation, and only effortop
// coordinates the resident and managed-topology owners.
func TestEffortCommandUsesOneFocusedOperationPerLeaf(t *testing.T) {
	root := testsupport.RepoRoot(t)
	command, err := os.ReadFile(filepath.Join(root, "cmd", "awf", "effort.go"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(command)
	for _, operation := range []string{
		"effortop.New(", "effortop.List(", "effortop.Show(", "effortop.Finish(",
		"effortop.AddWorktree(", "effortop.RemoveWorktree(", "effortop.Integrate(",
		"effortop.ReadMemory(", "effortop.EditMemory(", "effortop.UpdateMemory(",
		"effortop.AttachActivity(", "effortop.HeartbeatActivity(", "effortop.DetachActivity(",
	} {
		if got := strings.Count(source, operation); got != 1 {
			t.Errorf("command invocation sites for %s = %d, want 1", operation, got)
		}
	}
	for _, direct := range []string{
		"service.New(", "service.List(", "service.Show(", "service.Finish(",
		"service.Memory(", "service.UpdateMemory(", "manager.NewEffort(",
		"manager.Add(", "manager.Remove(", "manager.Integrate(",
	} {
		if strings.Contains(source, direct) {
			t.Errorf("command retains direct orchestration through %s", direct)
		}
	}
	operation, err := os.ReadFile(filepath.Join(root, "internal", "effortop", "operation.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{"internal/effort", "internal/worktree"} {
		if !strings.Contains(string(operation), owner) {
			t.Errorf("effortop no longer coordinates %s", owner)
		}
	}
}
