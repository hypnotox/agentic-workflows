package contextop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
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

func contextPreparationAcceptedV1(t testing.TB, num, title, date, stateChanges string) string {
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

func contextPreparationFixture(t testing.TB) string {
	t.Helper()
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, root, contextPreparationYAML)
	lock := &manifest.Lock{
		AWFVersion: project.Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{"prior": {}},
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

func contextPreparationProject(t testing.TB, root string) (*project.ProjectState, *awfgit.Repo) {
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
	input, err := workingState(testsupport.Context(t), state, nil, nil)
	if err != nil {
		t.Fatalf("working context filesystem fallback: %v", err)
	}
	if _, found := input.Snapshot().Tree.Lookup("internal/foo/x.go"); !found {
		t.Fatal("working context filesystem fallback omitted project input")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := workingState(canceled, state, nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("working context filesystem cancellation error = %v", err)
	}
	if _, err := workingCompleteState(canceled, state, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("complete working context filesystem cancellation error = %v", err)
	}
}

func TestWorkingStateSkipsUnrelatedRenderValidation(t *testing.T) {
	root := contextPreparationFixture(t)
	// This source override is parsed only while rendering the generated agents
	// document; declaration projection merely records it as an input.
	testsupport.WriteAwfConfig(t, root, contextPreparationYAML+"render:\n  templateSourceRoot: templates\n")
	testsupport.WriteFile(t, filepath.Join(root, "templates", "agents-doc", "AGENTS.md.tmpl"), "{{ broken")
	state, repo := contextPreparationProject(t, root)
	prep, err := currentstatecoord.PrepareWorkingContext(state.OutputState(), repo, testsupport.Context(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := complete(prep); err == nil || !strings.Contains(err.Error(), "template") {
		t.Fatalf("complete Publisher preparation error = %v, want unrelated template render validation", err)
	}
	input, err := workingState(testsupport.Context(t), state, repo, nil)
	if err != nil {
		t.Fatalf("focused ordinary context rejected unrelated render validation: %v", err)
	}
	if got := contextq.New(input).ContextForOptions([]string{"internal/foo/x.go"}, contextq.ContextOptions{}); len(got.Requests) != 1 {
		t.Fatalf("focused ordinary context result = %#v", got)
	}
}

func TestFocusedWorkingStateMatchesCompleteContextForExactRequest(t *testing.T) {
	root := contextPreparationFixture(t)
	state, repo := contextPreparationProject(t, root)
	focusedState, err := workingState(testsupport.Context(t), state, repo, []string{"internal/foo/x.go"})
	if err != nil {
		t.Fatal(err)
	}
	completeState, err := workingCompleteState(testsupport.Context(t), state, repo)
	if err != nil {
		t.Fatal(err)
	}
	options := contextq.ContextOptions{Selection: contextq.SelectionExplicit}
	focusedText := contextq.RenderContextText(contextq.New(focusedState).ContextForOptions([]string{"internal/foo/x.go"}, options), "live state for this project", nil)
	completeText := contextq.RenderContextText(contextq.New(completeState).ContextForOptions([]string{"internal/foo/x.go"}, options), "live state for this project", nil)
	if focusedText != completeText {
		t.Fatalf("focused output differs from complete\nfocused:\n%s\ncomplete:\n%s", focusedText, completeText)
	}
	focused := focusedState.Snapshot()
	if focused.Inventory == nil {
		t.Fatal("focused state omitted live inventory")
	}
	if _, present := focused.Inventory.Lookup("README.md"); !present {
		t.Fatal("focused inventory omitted unread present payload")
	}
	if _, read := focused.Tree.Lookup("README.md"); read {
		t.Fatal("focused exact context read unrelated regular payload")
	}
	countBytes := func(input contextinput.Input) (int, int) {
		files, bytes := 0, 0
		for _, file := range input.Snapshot().Tree.List() {
			files++
			bytes += len(file.Bytes)
		}
		return files, bytes
	}
	focusedFiles, focusedBytes := countBytes(focusedState)
	completeFiles, completeBytes := countBytes(completeState)
	t.Logf("ordinary exact capture: focused files=%d bytes=%d; complete files=%d bytes=%d", focusedFiles, focusedBytes, completeFiles, completeBytes)
	if focusedFiles >= completeFiles || focusedBytes >= completeBytes {
		t.Fatalf("focused capture did not reduce selected payload: files=%d/%d bytes=%d/%d", focusedFiles, completeFiles, focusedBytes, completeBytes)
	}
}

// invariant: tooling/context-and-topic:context-query-boundary (TestFocusedWorkingStateSelectsRequiredBytesWithoutNestedOrUnrelatedPayload)
// invariant: tooling/context-and-topic:context-read-only (TestFocusedWorkingStateSelectsRequiredBytesWithoutNestedOrUnrelatedPayload)
func TestFocusedWorkingStateSelectsRequiredBytesWithoutNestedOrUnrelatedPayload(t *testing.T) {
	root := contextPreparationFixture(t)
	for path, body := range map[string]string{
		".awf/unrelated-payload.md": "unread awf payload\n",
		"docs/unrelated-payload.md": "unread docs payload\n",
		"nested/.awf/config.yaml":   "prefix: nested\nprofile: core\nintegrationBranch: main\n",
		"nested/internal/marker.go": "package nested\n// state: alpha/one:order\n",
	} {
		testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(path)), body)
	}
	state, repo := contextPreparationProject(t, root)
	focused, err := workingState(testsupport.Context(t), state, repo, []string{"internal/foo/x.go", "nested/internal/marker.go"})
	if err != nil {
		t.Fatal(err)
	}
	view := focused.Snapshot()
	for _, path := range []string{".awf/unrelated-payload.md", "docs/unrelated-payload.md", "nested/internal/marker.go"} {
		if _, read := view.Tree.Lookup(path); read {
			t.Errorf("focused context read unrelated or nested-adopter payload %s", path)
		}
	}
	for _, path := range []string{"internal/foo/x.go", "docs/decisions/0001-first.md", ".awf/topics/metadata/alpha/one.yaml"} {
		if _, read := view.Tree.Lookup(path); !read {
			t.Errorf("focused context omitted required bytes %s", path)
		}
	}
	for _, path := range []string{".awf/unrelated-payload.md", "docs/unrelated-payload.md", "nested/internal/marker.go"} {
		if _, present := view.Inventory.Lookup(path); !present {
			t.Errorf("focused context omitted complete inventory entry %s", path)
		}
	}
	complete, err := workingCompleteState(testsupport.Context(t), state, repo)
	if err != nil {
		t.Fatal(err)
	}
	options := contextq.ContextOptions{Selection: contextq.SelectionExplicit}
	paths := []string{"internal/foo/x.go", "nested/internal/marker.go"}
	got := contextq.RenderContextText(contextq.New(focused).ContextForOptions(paths, options), "live state for this project", nil)
	want := contextq.RenderContextText(contextq.New(complete).ContextForOptions(paths, options), "live state for this project", nil)
	if got != want {
		t.Fatalf("focused nested-adopter output differs from complete\nfocused:\n%s\ncomplete:\n%s", got, want)
	}
}

// invariant: tooling/context-and-topic:context-read-only (TestFocusedWorkingStateMatchesCompleteParityMatrix)
func TestFocusedWorkingStateMatchesCompleteParityMatrix(t *testing.T) {
	root := contextPreparationFixture(t)
	configBody := strings.Replace(contextPreparationYAML, `globs: ["internal/**"]`, `globs: ["**/*.go"]`, 1) + "contextIgnore:\n  - ignored/**\n"
	testsupport.WriteAwfConfig(t, root, configBody)
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "domains", "alpha.yaml"), "paths:\n  - internal/foo/**\n  - linked/**\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "topics", "metadata", "alpha", "one.yaml"), "title: One\nsummary: The one topic.\npaths:\n  - internal/foo/**\n  - linked/**\n")
	for path, body := range map[string]string{
		"docs/plans/2026-08-03-context.md": contextPreparationPlan,
		"docs/generated.md":                "generated payload\n",
		"ignored/x.go":                     "package ignored\n",
		"nested/.awf/config.yaml":          "prefix: nested\nprofile: core\nintegrationBranch: main\n",
		"nested/internal/marker.go":        "package nested\n// state: alpha/one:order\n",
		"linked/internal/marker.go":        "package linked\n// state: alpha/one:order\n",
	} {
		testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(path)), body)
	}
	if err := os.Symlink("internal/foo/x.go", filepath.Join(root, "source-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "linked", ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../.awf/config.yaml", filepath.Join(root, "linked", ".awf", "config.yaml")); err != nil {
		t.Fatal(err)
	}
	lock, err := manifest.Load(filepath.Join(root, ".awf", "awf.lock"))
	if err != nil {
		t.Fatal(err)
	}
	lock.Files["docs/generated.md"] = manifest.Entry{TemplateID: "fixture"}
	if err := lock.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	state, repo := contextPreparationProject(t, root)
	completeState, err := workingCompleteState(testsupport.Context(t), state, repo)
	if err != nil {
		t.Fatal(err)
	}
	allFacets := []contextq.ContextFacet{
		contextq.FacetRelationships,
		contextq.FacetInvariants,
		contextq.FacetAllRules,
		contextq.FacetEvidence,
		contextq.FacetSelectors,
		contextq.FacetReferences,
		contextq.FacetPending,
		contextq.FacetArtifacts,
	}
	cases := []struct {
		name   string
		paths  []string
		facets []contextq.ContextFacet
	}{
		{name: "exact marker", paths: []string{"internal/foo/x.go"}},
		{name: "directory", paths: []string{"internal/foo"}},
		{name: "root", paths: []string{"."}},
		{name: "mixed exact and directory", paths: []string{"internal/foo/x.go", "docs/decisions"}},
		{name: "exceptional paths", paths: []string{"missing", "../outside", "ignored/x.go", "nested/internal/marker.go", "linked/internal/marker.go", "source-link"}},
		{name: "ADR plan reference and generated artifact", paths: []string{"docs/decisions/0001-first.md", "docs/generated.md"}, facets: []contextq.ContextFacet{contextq.FacetReferences, contextq.FacetArtifacts}},
		{name: "all facets", paths: []string{"internal/foo/x.go", "docs/decisions/0002-later.md"}, facets: allFacets},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			focusedState, err := workingState(testsupport.Context(t), state, repo, tc.paths)
			if err != nil {
				t.Fatal(err)
			}
			options := contextq.ContextOptions{Selection: contextq.SelectionExplicit, Facets: tc.facets}
			got := contextq.RenderContextText(contextq.New(focusedState).ContextForOptions(tc.paths, options), "live state for this project", tc.facets)
			want := contextq.RenderContextText(contextq.New(completeState).ContextForOptions(tc.paths, options), "live state for this project", tc.facets)
			if got != want {
				t.Fatalf("focused output differs from complete\nfocused:\n%s\ncomplete:\n%s", got, want)
			}
		})
	}
}

// invariant: tooling/context-and-topic:context-read-only (TestFocusedWorkingStateSelectsOnlyRequestedManifestPayloads)
func TestFocusedWorkingStateSelectsOnlyRequestedManifestPayloads(t *testing.T) {
	root := contextPreparationFixture(t)
	testsupport.WriteAwfConfig(t, root, contextPreparationYAML+"contextIgnore:\n  - ignored/**\n")
	for path, body := range map[string]string{
		"payload.bin": "root payload\n", "assets/payload.bin": "directory payload\n", "ignored/payload.bin": "ignored payload\n",
		"manifest/exact.txt": "exact output\n", "manifest/dir/a.txt": "directory output\n", "manifest/root.txt": "root output\n",
	} {
		testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(path)), body)
	}
	lock, err := manifest.Load(filepath.Join(root, ".awf", "awf.lock"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"manifest/exact.txt", "manifest/dir/a.txt", "manifest/root.txt"} {
		lock.Files[path] = manifest.Entry{TemplateID: "fixture"}
	}
	if err := lock.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	state, repo := contextPreparationProject(t, root)
	for _, tc := range []struct {
		name, unread       string
		requests, selected []string
	}{
		{name: "exact unmanifested", requests: []string{"payload.bin"}, unread: "payload.bin"},
		{name: "directory unmanifested", requests: []string{"assets"}, unread: "assets/payload.bin"},
		{name: "ignored unmanifested", requests: []string{"ignored/payload.bin"}, unread: "ignored/payload.bin"},
		{name: "exact manifested", requests: []string{"manifest/exact.txt"}, selected: []string{"manifest/exact.txt"}},
		{name: "directory manifested", requests: []string{"manifest/dir"}, selected: []string{"manifest/dir/a.txt"}},
		{name: "root manifested", requests: []string{"."}, unread: "payload.bin", selected: []string{"manifest/exact.txt", "manifest/dir/a.txt", "manifest/root.txt"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			focused, err := workingState(testsupport.Context(t), state, repo, tc.requests)
			if err != nil {
				t.Fatal(err)
			}
			view := focused.Snapshot()
			if tc.unread != "" {
				if _, present := view.Inventory.Lookup(tc.unread); !present {
					t.Fatalf("inventory omitted %s", tc.unread)
				}
				if _, read := view.Tree.Lookup(tc.unread); read {
					t.Fatalf("focused context read classification-only payload %s", tc.unread)
				}
			}
			for _, path := range tc.selected {
				if _, read := view.Tree.Lookup(path); !read {
					t.Errorf("focused context omitted requested manifest payload %s", path)
				}
			}
		})
	}
}

func TestWorkingStatePropagatesPublisherPreparationFailure(t *testing.T) {
	root := contextPreparationFixture(t)
	state, repo := contextPreparationProject(t, root)
	writeMalformedPitfall(t, root)
	_, err := workingCompleteState(testsupport.Context(t), state, repo)
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
	working, err := workingState(testsupport.Context(t), state, repo, nil)
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
	working, err = workingState(testsupport.Context(t), state, repo, nil)
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
			working, err := workingState(testsupport.Context(t), state, repo, nil)
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

// invariant: tooling/context-and-topic:context-read-only (TestNonordinaryRoutesRetainCompletePublisherPreparation)
func TestNonordinaryRoutesRetainCompletePublisherPreparation(t *testing.T) {
	root := contextPreparationFixture(t)
	testsupport.WriteAwfConfig(t, root, contextPreparationYAML+"render:\n  templateSourceRoot: templates\n")
	testsupport.WriteFile(t, filepath.Join(root, "templates", "agents-doc", "AGENTS.md.tmpl"), "{{ broken")
	gitfixture.AddAll(t, gitfixture.At(root))
	state, repo := contextPreparationProject(t, root)
	load := func(context.Context, string) (*project.ProjectState, *config.Config, *awfgit.Repo, error) {
		return state, nil, repo, nil
	}
	gate := func(context.Context, string) error { return nil }
	for _, input := range []Input{
		{Paths: []string{"internal/foo/x.go"}, Staged: true},
		{Paths: []string{"internal/foo/x.go"}, Range: "base..head"},
		{Paths: []string{"internal/foo/x.go"}, Uncovered: true},
	} {
		if _, err := Run(testsupport.Context(t), root, input, load, gate, gate); err == nil || !strings.Contains(err.Error(), "template") {
			t.Fatalf("nonordinary input %#v error = %v, want complete render validation", input, err)
		}
	}
}

func TestFocusedContextRetainsRequiredAnswerValidation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{"authority", func(t *testing.T, root string) {
			testsupport.WriteFile(t, filepath.Join(root, ".awf", "topics", "metadata", "alpha", "one.yaml"), "title: [broken\n")
		}},
		{"marker", func(t *testing.T, root string) {
			testsupport.WriteFile(t, filepath.Join(root, "internal", "foo", "x.go"), "package foo\n// state: no-such-claim\n")
		}},
		{"plan link", func(t *testing.T, root string) {
			testsupport.WriteFile(t, filepath.Join(root, "docs", "plans", "2026-08-04-broken.md"), "---\nformat: plan-v2\ndate: 2026-08-04\nstatus: Proposed\n---\n# Plan: Broken\n")
		}},
		{"declaration", func(t *testing.T, root string) { writeMalformedPitfall(t, root) }},
		{"requested artifact", func(t *testing.T, root string) {
			testsupport.WriteFile(t, filepath.Join(root, ".awf", "awf.lock"), "not a lock\n")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := contextPreparationFixture(t)
			tc.mutate(t, root)
			state, repo := contextPreparationProject(t, root)
			if _, err := workingState(testsupport.Context(t), state, repo, nil); err == nil {
				t.Fatal("focused context accepted malformed input needed for its answer")
			}
		})
	}
}
