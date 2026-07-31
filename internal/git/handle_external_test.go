package git_test

import (
	"context"
	"testing"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func testContext(t *testing.T) context.Context { return testsupport.Context(t) }

func gitRepo(t *testing.T, root string) *awfgit.Repo {
	t.Helper()
	repo, _, err := awfgit.OpenContaining(root)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}
