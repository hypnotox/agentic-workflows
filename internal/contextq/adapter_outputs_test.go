package contextq

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

// TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion reads this
// repository's own tree. It lives here rather than beside the output plan
// because it asserts the context projection's classification, ownership, and
// directory-expansion vocabulary, which is private to this package (ADR-0195
// item 8); its proof markers are valid anywhere inside currentState.testGlobs.

// invariant: rendering/adapter-outputs:generated-adapter-runtime-ownership (TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion)
// invariant: rendering/pi-runtime:pi-tools-integration-boundary (TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion)
// invariant: rendering/pi-runtime:pi-real-runtime-smoke (TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion)
// invariant: rendering/pi-runtime:pi-minimum-runtime (TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion)
// invariant: rendering/pi-runtime:pi-implementation-state-boundary (TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion)
// invariant: rendering/pi-workflows:pi-extension-editor-quiet-strip (TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion)
// invariant: rendering/pi-workflows:pi-implementation-batch-exclusivity (TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion)
// invariant: rendering/pi-workflows:pi-subagent-model-preferences (TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion)
// invariant: rendering/pi-workflows:pi-subagent-model-routing (TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion)
// invariant: rendering/pi-workflows:pi-implement-role-artifact (TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion)
// invariant: rendering/pi-workflows:pi-role-contract-loader (TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion)
// invariant: rendering/project-output-plan:multi-target-render (TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion)
// invariant: rendering/workflow-skill-templates:bounded-exploration-reporting (TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion)
// invariant: rendering/pi-workflows:pi-dedicated-grounding-dispatch (TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion)
// invariant: rendering/workflow-skill-templates:cross-runtime-exploration-dispatch (TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion)
// invariant: rendering/pi-workflows:pi-subagent-model-wizard (TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion)
// invariant: rendering/workflow-skill-templates:memory-checkpoint-chain-coverage (TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion)
// invariant: rendering/pi-workflows:pi-structured-exploration-contract (TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion)
func TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion(t *testing.T) {
	p, err := project.Open(testContext(t), filepath.Clean(filepath.Join("..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	state, err := p.ContextState(testContext(t))
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
