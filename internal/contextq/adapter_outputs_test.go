package contextq

import (
	"path/filepath"
	"slices"
	"testing"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

// TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion reads this
// repository's own tree. It lives here rather than beside the output plan
// because it asserts the context projection's classification, ownership, and
// directory-expansion vocabulary, which is private to this package (ADR-0195
// item 8); its proof markers are valid anywhere inside currentState.testGlobs.

// invariant: rendering/adapter-outputs:generated-adapter-runtime-ownership (TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion)
func TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion(t *testing.T) {
	p, err := project.Open(testContext(t), filepath.Clean(filepath.Join("..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	repo, _, err := awfgit.OpenContaining(p.Root())
	if err != nil {
		t.Fatal(err)
	}
	state, err := project.BuildContextState(p, repo, testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	q := New(state)
	for _, extension := range []string{".pi/extensions/awf-subagents/index.ts", ".pi/extensions/awf-effort/index.ts"} {
		result := q.ContextForOptions([]string{extension}, ContextOptions{Selection: SelectionExplicit})
		if len(result.Requests) != 1 || result.Requests[0].Exact == nil || result.Requests[0].Exact.Context.Classification != pathGeneratedOutput {
			t.Fatalf("extension classification = %#v", result.Requests)
		}
		path := result.Requests[0].Exact.Context
		if !slices.ContainsFunc(path.Domains, func(domain domainRef) bool { return domain.Name == "rendering" }) || !slices.ContainsFunc(path.Topics, func(topic contextPathTopic) bool { return topic.ID == "rendering/adapter-outputs" }) {
			t.Fatalf("extension ownership = domains %#v topics %#v", path.Domains, path.Topics)
		}
	}
	expanded := q.ContextForOptions([]string{".pi/extensions"}, ContextOptions{Selection: SelectionExplicit})
	if len(expanded.Requests) != 1 || expanded.Requests[0].Kind != requestDirectoryEmpty || expanded.Requests[0].Directory == nil || expanded.Requests[0].Directory.Included != 0 {
		t.Fatalf("generated extension entered whole-tree expansion: %#v", expanded)
	}
}
