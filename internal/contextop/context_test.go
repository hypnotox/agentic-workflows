package contextop

import (
	"context"
	"errors"
	"strings"
	"testing"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestRunRetainsSyntaxStaticAndStagedSelection(t *testing.T) {
	root := t.TempDir()
	static, err := Run(context.Background(), root, Input{Paths: []string{"x"}}, nil, nil, nil)
	if err != nil || !strings.Contains(string(static), "static: not inside an awf project") {
		t.Fatalf("static result = %q, %v", static, err)
	}
	_, err = Run(context.Background(), root, Input{Paths: []string{"x"}, Shows: []string{"bad"}}, nil, nil, nil)
	var usage *UsageError
	if !strings.Contains(err.Error(), "unknown context facet") || !asUsage(err, &usage) {
		t.Fatalf("syntax error = %v", err)
	}
	_, err = Run(context.Background(), root, Input{Paths: []string{"x"}, Staged: true}, nil, nil, func(context.Context, string) error { return nil })
	if err == nil {
		t.Fatal("staged selection accepted a non-repository")
	}
}

func TestWorkingStatePropagatesPreparationFailure(t *testing.T) {
	root := testsupport.RepoRoot(t)
	state, err := project.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := awfgit.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := workingState(ctx, state, repo); !errors.Is(err, context.Canceled) {
		t.Fatalf("working state error = %v, want cancellation", err)
	}
}

func asUsage(err error, target **UsageError) bool { return errors.As(err, target) }
