package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func writeMalformedConfigReferencePart(t *testing.T, root string) {
	t.Helper()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "parts", "config-reference", "intro.md"), "<!-- awf:comment\n")
}

func requireMalformedConfigReferencePartError(t *testing.T, err error) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "config-reference/intro.md") || !strings.Contains(err.Error(), "malformed awf:comment") {
		t.Fatalf("publisher preparation error = %v, want malformed config-reference part", err)
	}
}

func TestWorkingContextStatePropagatesPublisherPreparationFailure(t *testing.T) {
	root := ctxCmdFixture(t)
	state, _, repo, err := openProjectOperation(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	writeMalformedConfigReferencePart(t, root)

	_, err = workingContextState(context.Background(), state, repo)
	requireMalformedConfigReferencePartError(t, err)
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
			writeMalformedConfigReferencePart(t, root)
			gitfixture.AddAll(t, gitfixture.At(root))

			requireMalformedConfigReferencePartError(t, tc.run(context.Background(), root))
		})
	}
}

func TestStagedContextStatePropagatesPreparationFailure(t *testing.T) {
	if _, err := stagedContextState(context.Background(), t.TempDir()); err == nil {
		t.Fatal("stagedContextState accepted a directory outside a repository")
	}
}
