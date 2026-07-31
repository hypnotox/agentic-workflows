package project

import "testing"

// TestIndexCurrentStatePropagatesSnapshotFailure pins that the staged load
// surfaces an index-snapshot failure rather than degrading to an empty
// universe. StagedContextState refuses earlier, at openRootProject, so this is
// the only route that reaches the snapshot failure directly.
func TestIndexCurrentStatePropagatesSnapshotFailure(t *testing.T) {
	t.Parallel()
	if _, err := (&Project{Root: t.TempDir()}).indexCurrentState(testContext(t)); err == nil {
		t.Fatal("index current state accepted a non-repository")
	}
}
