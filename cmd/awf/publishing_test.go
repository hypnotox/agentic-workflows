package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/contextinput"
	"github.com/hypnotox/agentic-workflows/internal/contextq"
	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
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

func TestWorkingContextStateUsesFilesystemWithoutRepository(t *testing.T) {
	root := ctxCmdFixture(t)
	state, _, _, err := openProjectOperation(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	input, err := workingContextState(context.Background(), state, nil)
	if err != nil {
		t.Fatalf("working context filesystem fallback: %v", err)
	}
	if _, found := input.Snapshot().Tree.Lookup("internal/foo/x.go"); !found {
		t.Fatal("working context filesystem fallback omitted project input")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := workingContextState(canceled, state, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("working context filesystem cancellation error = %v", err)
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

// invariant: tooling/context-and-topic:context-query-boundary (TestContextCompositionSelectsOneFreshTreeForPublisherAndCompletion)
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
		got, ok := state.Snapshot().Loaded.Topics.ByTopicID("alpha/one")
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

type countingContextReader struct {
	outputplan.TreeReader
	calls int
	reads map[string]int
}

func (r *countingContextReader) ReadFile(path string) ([]byte, bool, error) {
	r.calls++
	r.reads[path]++
	return r.TreeReader.ReadFile(path)
}

func (r *countingContextReader) Paths(prefix string) ([]string, error) {
	r.calls++
	return r.TreeReader.Paths(prefix)
}

// invariant: tooling/context-and-topic:context-query-boundary (TestContextCompletionReusesPublisherParsedSemantics)
func TestContextCompletionReusesPublisherParsedSemantics(t *testing.T) {
	root := ctxCmdFixture(t)
	testsupport.WriteFile(t, filepath.Join(root, "docs", "plans", "2026-08-03-context.md"), readCommandV2Plan)
	state, _, repo, err := openProjectOperation(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	gitfixture.AddAll(t, gitfixture.At(root))

	operations := []struct {
		name    string
		prepare func() (*currentstatecoord.ContextPreparation, error)
	}{
		{"working", func() (*currentstatecoord.ContextPreparation, error) {
			return currentstatecoord.PrepareWorkingContext(state.OutputState(), repo, context.Background())
		}},
		{"staged", func() (*currentstatecoord.ContextPreparation, error) {
			return currentstatecoord.PrepareStagedContext(context.Background(), root)
		}},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			prep, err := operation.prepare()
			if err != nil {
				t.Fatal(err)
			}
			reader := &countingContextReader{TreeReader: prep.Reader, reads: map[string]int{}}
			prep.Reader = reader
			prepared, err := preparePublisher(preparedPublisher(prep))
			if err != nil {
				t.Fatal(err)
			}
			beforeCompletion := reader.calls
			input := currentstatecoord.CompleteContext(prep, prepared.ADRs(), prepared.Topics(), prepared.Plans(), prepared.Plan().Declarations())
			result := contextq.New(input).ContextForOptions([]string{"internal/foo/x.go"}, contextq.ContextOptions{})
			if len(result.Requests) != 1 {
				t.Fatalf("completed context requests = %#v", result.Requests)
			}
			if reader.calls != beforeCompletion {
				t.Fatalf("completion reparsed Publisher semantics: reads %d -> %d", beforeCompletion, reader.calls)
			}
			for _, path := range []string{"docs/decisions/0001-first.md", ".awf/topics/metadata/alpha/one.yaml", ".awf/topics/parts/alpha/one/current-state.md", "docs/plans/2026-08-03-context.md"} {
				if reader.reads[path] == 0 {
					t.Errorf("Publisher preparation did not traverse representative semantic input %s", path)
				}
			}
		})
	}
}

func TestContextPreparationRespectsProfileAuthority(t *testing.T) {
	for _, profile := range []struct {
		name, config string
		full         bool
	}{
		{name: "core", config: "prefix: example\nprofile: core\nintegrationBranch: main\n"},
		{name: "full", config: strings.Replace(ctxCmdYAML, "integrationBranch: main\n", "profile: full\nintegrationBranch: main\n", 1), full: true},
	} {
		t.Run(profile.name, func(t *testing.T) {
			root := ctxCmdFixture(t)
			testsupport.WriteAwfConfig(t, root, profile.config)
			testsupport.WriteFile(t, filepath.Join(root, "docs", "plans", "2026-08-03-context.md"), readCommandV2Plan)
			gitfixture.AddAll(t, gitfixture.At(root))
			state, _, repo, err := openProjectOperation(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			working, err := workingContextState(context.Background(), state, repo)
			if err != nil {
				t.Fatal(err)
			}
			staged, err := stagedContextState(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			for name, input := range map[string]contextinput.Input{"working": working, "staged": staged} {
				view := input.Snapshot()
				for authority, present := range map[string]bool{
					"ADRs":   len(view.Loaded.ADRs) > 0,
					"topics": len(view.Loaded.Topics.All()) > 0,
					"plans":  len(view.PlanState.Plans) > 0,
				} {
					if present != profile.full {
						t.Errorf("%s %s %s authority present = %v, want %v", profile.name, name, authority, present, profile.full)
					}
				}
			}
		})
	}
}

var _ outputplan.TreeReader = (*countingContextReader)(nil)
