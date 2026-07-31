package snapshot_test

import (
	"context"
	"testing"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func testContext(t *testing.T) context.Context { return testsupport.Context(t) }

func snapshotRepo(t *testing.T, root string) *awfgit.Repo {
	t.Helper()
	repo, err := awfgit.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}
