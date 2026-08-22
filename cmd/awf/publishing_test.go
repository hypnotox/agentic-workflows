package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func writeMalformedPitfall(t *testing.T, root string) {
	t.Helper()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "docs", "pitfalls", "bad.md"), "malformed source\n")
}

func requireMalformedPitfallError(t *testing.T, err error) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "bad.md") || !strings.Contains(err.Error(), "missing frontmatter") {
		t.Fatalf("publisher preparation error = %v, want malformed pitfall", err)
	}
}

func TestWorkingContextStatePropagatesPublisherPreparationFailure(t *testing.T) {
	root := ctxCmdFixture(t)
	state, _, repo, err := openProjectOperation(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	writeMalformedPitfall(t, root)

	_, err = workingContextState(context.Background(), state, repo)
	requireMalformedPitfallError(t, err)
}

func TestStagedPublisherPreparationFailuresPropagate(t *testing.T) {
	cases := []struct {
		name string
		run  func(context.Context, string) error
	}{
		{"drift", func(ctx context.Context, root string) error {
			_, err := stagedDrift(ctx, root)
			return err
		}},
		{"context state", func(ctx context.Context, root string) error {
			_, err := stagedContextState(ctx, root)
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := ctxCmdFixture(t)
			writeMalformedPitfall(t, root)
			gitfixture.AddAll(t, gitfixture.At(root))

			requireMalformedPitfallError(t, tc.run(context.Background(), root))
		})
	}
}

func TestStagedContextStatePropagatesPreparationFailure(t *testing.T) {
	if _, err := stagedContextState(context.Background(), t.TempDir()); err == nil {
		t.Fatal("stagedContextState accepted a directory outside a repository")
	}
}
