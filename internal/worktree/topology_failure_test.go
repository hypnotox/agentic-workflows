package worktree

import (
	"context"
	"errors"
	"strings"
	"testing"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

func TestSettleAddFailureRetainsRegisteredMutation(t *testing.T) {
	id, path := "effort-id", "/managed/effort-id"
	m := &Manager{ctx: context.Background(), roots: awfgit.ControlRoots{InvokingRoot: "/primary"}, run: func(context.Context, string, ...string) ([]byte, error) {
		return []byte("worktree " + path + "\x00HEAD abc\x00branch refs/heads/" + branch(id) + "\x00\x00"), nil
	}}
	err := m.settleAddFailure(id, path, errors.New("add failed"))
	if err == nil || !strings.Contains(err.Error(), "partial Git mutation") {
		t.Fatalf("settlement = %v", err)
	}
}
