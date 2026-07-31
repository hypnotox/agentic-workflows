package git

import (
	"context"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func statusRepo(t *testing.T, root string) *Repo {
	t.Helper()
	repo, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func testContext(t *testing.T) context.Context { return testsupport.Context(t) }
