package effort

import (
	"os"
	"testing"
	"time"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

// testWiring composes the production-shaped dependency set for the checkout at
// root, so a test states only the dependency it deliberately replaces and every
// other one behaves exactly as it does in the composed command.
func testWiring(t *testing.T, root string) (awfgit.ControlRoots, Dependencies) {
	t.Helper()
	roots, err := awfgit.ResolveControlRoots(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := awfgit.Open(roots.InvokingRoot)
	if err != nil {
		t.Fatal(err)
	}
	return roots, Dependencies{
		Clock:                 time.Now,
		UUID:                  RandomUUIDv4,
		Worktrees:             repo.WorktreeList,
		BranchExists:          repo.BranchExists,
		ValidateRef:           repo.ValidateRefName,
		RemoveTree:            os.RemoveAll,
		ExpectedArchiveMarker: func() ([]byte, error) { return []byte(testArchiveMarker), nil },
	}
}

// openTestService composes a service over root with deps applied on top of the
// production-shaped wiring.
func openTestService(t *testing.T, root string, apply func(*Dependencies)) *Service {
	t.Helper()
	roots, deps := testWiring(t, root)
	if apply != nil {
		apply(&deps)
	}
	service, err := Open(roots, deps)
	if err != nil {
		t.Fatal(err)
	}
	return service
}
