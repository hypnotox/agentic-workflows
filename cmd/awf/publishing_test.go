package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/contextinput"
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

func TestWorkingContextStatePropagatesSelectionFailure(t *testing.T) {
	root := ctxCmdFixture(t)
	state, _, _, err := openProjectOperation(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workingContextState(context.Background(), state, nil); err == nil {
		t.Fatal("working context accepted a missing repository")
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
		{"drift result", func(ctx context.Context, root string) error {
			_, err := stagedDriftResult(ctx, root)
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

func TestContextCompositionSelectsOneFreshTreeForPublisherAndCompletion(t *testing.T) {
	root := ctxCmdFixture(t)
	gitfixture.AddAll(t, gitfixture.At(root))
	topicPath := filepath.Join(root, ".awf", "topics", "metadata", "alpha", "one.yaml")
	writeTitle := func(title string) {
		t.Helper()
		testsupport.WriteFile(t, topicPath, "title: "+title+"\nsummary: The one topic.\npaths:\n  - internal/foo/**\n")
	}
	assertTitle := func(want string, state contextinput.Input) {
		t.Helper()
		got, ok := state.Loaded.Topics.ByTopicID("alpha/one")
		if !ok || got.Metadata.Title != want {
			t.Fatalf("Publisher/completion topic = %#v, want title %q", got, want)
		}
	}

	writeTitle("Working")
	state, _, repo, err := openProjectOperation(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	working, err := workingContextState(context.Background(), state, repo)
	if err != nil {
		t.Fatal(err)
	}
	assertTitle("Working", working)
	staged, err := stagedContextState(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	assertTitle("One", staged)

	writeTitle("Working Again")
	working, err = workingContextState(context.Background(), state, repo)
	if err != nil {
		t.Fatal(err)
	}
	assertTitle("Working Again", working)
	gitfixture.AddAll(t, gitfixture.At(root))
	writeTitle("Dirty Working Bytes")
	staged, err = stagedContextState(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	assertTitle("Working Again", staged)
}

func TestStagedContextStatePropagatesPreparationFailure(t *testing.T) {
	if _, err := stagedContextState(context.Background(), t.TempDir()); err == nil {
		t.Fatal("stagedContextState accepted a directory outside a repository")
	}
}
