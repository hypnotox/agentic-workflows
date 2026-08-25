package contextop

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/contextinput"
	"github.com/hypnotox/agentic-workflows/internal/contextq"
	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

const contextPreparationYAML = `prefix: example
integrationBranch: main
vars:
  gateCmd: make gate
domains:
  - alpha
  - core
currentState:
  sources:
    - globs: ["internal/**"]
      marker: "//"
  testGlobs: ["internal/**/*_test.go"]
`

const contextPreparationPlan = `---
format: plan-v2
date: 2026-08-03
adrs: [fixture, context, third]
status: Proposed
---
# Plan: Read command V2

## Goal

Project the exact selected work without leaking unselected decisions.

## Architecture summary

Keep plan parsing and decision resolution at their existing boundaries.

## Phase 1: Project

**Execution mode: inline.**

Advances: ["advanced"]
Completes: ["completed"]

### Task 1.1: Read selected
Applying: ["fixture:first"]
Context: ["context:second"]

Keep this exact task body.

### Task 1.2: Keep unselected
Applying: ["context:second"]
Context: ["third:third"]

Do not leak this task or its decision into task 1.1.

### Phase close

Run the exact gate.

` + "```commit\nfeat(plans): project task context\n```" + `

## Definition of done

- ` + "`dod: advanced`" + ` Advance the exact projection.
- ` + "`dod: completed`" + ` Complete the exact projection.

## Notes

Keep the exact note.
`

func contextPreparationAcceptedV1(t *testing.T, num, title, date, stateChanges string) string {
	t.Helper()
	doc := func(status, history string) string {
		return "---\nformat: current-state-v1\nstatus: " + status + "\ndate: " + date + "\n---\n" +
			"# ADR-" + num + ": " + title + "\n\n" +
			"## Context\n\nBackground prose.\n\n" +
			"## Decision\n\n1. The decision.\n\n" +
			"## State changes\n\n" + stateChanges + "\n\n" +
			"## Consequences\n\nConsequence prose.\n\n" +
			"## Alternatives Considered\n\nNone considered.\n\n" +
			"## Status history\n\n" + history + "\n"
	}
	scaffold, err := adr.ParseV1(num+"-x.md", []byte(doc("Proposed", "- "+date+": Proposed")))
	if err != nil {
		t.Fatalf("parse fixture ADR: %v", err)
	}
	return doc("Accepted", "- "+date+": Proposed\n- "+date+": Accepted; content-sha256: "+adr.ContentDigest(scaffold.Sections))
}

func contextPreparationFixture(t *testing.T) string {
	t.Helper()
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, root, contextPreparationYAML)
	lock := &manifest.Lock{
		AWFVersion: project.Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{},
	}
	if err := lock.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	for rel, body := range map[string]string{
		".awf/domains/alpha.yaml":                      "paths:\n  - internal/foo/**\n",
		".awf/domains/core.yaml":                       "paths: []\n",
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: The one topic.\npaths:\n  - internal/foo/**\n",
		".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n### `rule: order`\nOrder is deterministic.\nOrigin: ADR-0001\n\n### `invariant: tested`\nTests protect output.\nOrigin: ADR-0001\nBacking: test\n\n### `invariant: stable`\nOutput is stable.\nOrigin: ADR-0001\nBacking: unbacked\nVerify: by hand.\n",
		".awf/topics/metadata/core/g.yaml":             "title: Global\nsummary: Global rules.\napplies: global\n",
		".awf/topics/parts/core/g/current-state.md":    "Intro.\n\n## Claims\n\n### `rule: everywhere`\nApplies everywhere.\nOrigin: ADR-0001\n",
		"docs/decisions/0001-first.md":                 testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTitle("0001: First"), testsupport.WithBody("## Context\nx\n## Consequences\nc\n")),
		"docs/decisions/0002-later.md":                 contextPreparationAcceptedV1(t, "0002", "Later", "2026-07-20", "- add `alpha/one:pending-rule`"),
		"internal/foo/x.go":                            "package foo\n// state: alpha/one:order\n",
		"internal/foo/y.go":                            "package foo\n",
		"internal/foo/y_test.go":                       "package foo\n// invariant: alpha/one:tested (TestTested)\nfunc TestTested() {}\n",
	} {
		testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	return root
}

func contextPreparationProject(t *testing.T, root string) (*project.ProjectState, *awfgit.Repo) {
	t.Helper()
	state, err := project.Open(testsupport.Context(t), root)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := awfgit.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return state, repo
}

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

func TestWorkingStateUsesFilesystemWithoutRepository(t *testing.T) {
	root := contextPreparationFixture(t)
	state, _ := contextPreparationProject(t, root)
	input, err := workingState(testsupport.Context(t), state, nil)
	if err != nil {
		t.Fatalf("working context filesystem fallback: %v", err)
	}
	if _, found := input.Snapshot().Tree.Lookup("internal/foo/x.go"); !found {
		t.Fatal("working context filesystem fallback omitted project input")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := workingState(canceled, state, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("working context filesystem cancellation error = %v", err)
	}
}

func TestWorkingStatePropagatesPublisherPreparationFailure(t *testing.T) {
	root := contextPreparationFixture(t)
	state, repo := contextPreparationProject(t, root)
	writeMalformedPitfall(t, root)
	_, err := workingState(testsupport.Context(t), state, repo)
	requireMalformedPitfallError(t, err)
}

func TestStagedStatePublisherPreparationFailurePropagates(t *testing.T) {
	root := contextPreparationFixture(t)
	writeMalformedPitfall(t, root)
	gitfixture.AddAll(t, gitfixture.At(root))
	_, err := stagedState(testsupport.Context(t), root)
	requireMalformedPitfallError(t, err)
}

func TestContextCompositionSelectsOneFreshTreeForPublisherAndCompletion(t *testing.T) {
	root := contextPreparationFixture(t)
	gitfixture.AddAll(t, gitfixture.At(root))
	topicPath := filepath.Join(root, ".awf", "topics", "metadata", "alpha", "one.yaml")
	writeTitle := func(title string) {
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
	state, repo := contextPreparationProject(t, root)
	working, err := workingState(testsupport.Context(t), state, repo)
	if err != nil {
		t.Fatal(err)
	}
	assertTitle("Working", working)
	staged, err := stagedState(testsupport.Context(t), root)
	if err != nil {
		t.Fatal(err)
	}
	assertTitle("One", staged)
	writeTitle("Working Again")
	working, err = workingState(testsupport.Context(t), state, repo)
	if err != nil {
		t.Fatal(err)
	}
	assertTitle("Working Again", working)
	gitfixture.AddAll(t, gitfixture.At(root))
	writeTitle("Dirty Working Bytes")
	staged, err = stagedState(testsupport.Context(t), root)
	if err != nil {
		t.Fatal(err)
	}
	assertTitle("Working Again", staged)
}

func TestStagedStatePropagatesPreparationFailure(t *testing.T) {
	if _, err := stagedState(testsupport.Context(t), t.TempDir()); err == nil {
		t.Fatal("stagedState accepted a directory outside a repository")
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

func TestContextCompletionReusesPublisherParsedSemantics(t *testing.T) {
	root := contextPreparationFixture(t)
	testsupport.WriteFile(t, filepath.Join(root, "docs", "plans", "2026-08-03-context.md"), contextPreparationPlan)
	state, repo := contextPreparationProject(t, root)
	gitfixture.AddAll(t, gitfixture.At(root))
	operations := []struct {
		name    string
		prepare func() (*currentstatecoord.ContextPreparation, error)
	}{
		{"working", func() (*currentstatecoord.ContextPreparation, error) {
			return currentstatecoord.PrepareWorkingContext(state.OutputState(), repo, testsupport.Context(t))
		}},
		{"staged", func() (*currentstatecoord.ContextPreparation, error) {
			return currentstatecoord.PrepareStagedContext(testsupport.Context(t), root)
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
			input, err := complete(prep)
			if err != nil {
				t.Fatal(err)
			}
			beforeCompletion := reader.calls
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
		{name: "full", config: strings.Replace(contextPreparationYAML, "integrationBranch: main\n", "profile: full\nintegrationBranch: main\n", 1), full: true},
	} {
		t.Run(profile.name, func(t *testing.T) {
			root := contextPreparationFixture(t)
			testsupport.WriteAwfConfig(t, root, profile.config)
			testsupport.WriteFile(t, filepath.Join(root, "docs", "plans", "2026-08-03-context.md"), contextPreparationPlan)
			gitfixture.AddAll(t, gitfixture.At(root))
			state, repo := contextPreparationProject(t, root)
			working, err := workingState(testsupport.Context(t), state, repo)
			if err != nil {
				t.Fatal(err)
			}
			staged, err := stagedState(testsupport.Context(t), root)
			if err != nil {
				t.Fatal(err)
			}
			for name, input := range map[string]contextinput.Input{"working": working, "staged": staged} {
				view := input.Snapshot()
				for authority, present := range map[string]bool{"ADRs": len(view.Loaded.ADRs) > 0, "topics": len(view.Loaded.Topics.All()) > 0, "plans": len(view.PlanState.Plans) > 0} {
					if present != profile.full {
						t.Errorf("%s %s %s authority present = %v, want %v", profile.name, name, authority, present, profile.full)
					}
				}
			}
		})
	}
}

var _ outputplan.TreeReader = (*countingContextReader)(nil)
