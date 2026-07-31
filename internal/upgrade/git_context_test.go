package upgrade

import (
	"context"
	"testing"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func testContext(t *testing.T) context.Context { return testsupport.Context(t) }

func headHash(t *testing.T, root string) (string, error) {
	t.Helper()
	repo, err := awfgit.Open(root)
	if err != nil {
		return "", err
	}
	return repo.HeadHash(testContext(t))
}
