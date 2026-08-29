package publisher

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// invariant: rendering/adapter-outputs:generated-adapter-runtime-ownership (TestGeneratedAdapterRuntimeOwnership)
func TestGeneratedAdapterRuntimeOwnership(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	state, err := project.Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := New(state.OutputState(), testConfig(state), NewFilesystemReader(root), project.Version).Prepare()
	if err != nil {
		t.Fatal(err)
	}
	for _, extension := range []string{".pi/extensions/awf-subagents/index.ts", ".pi/extensions/awf-effort/index.ts"} {
		if !slices.Contains(prepared.Plan().Paths(), extension) {
			t.Fatalf("generated extension %q is absent from output plan", extension)
		}
		domains, topics := topic.PathAuthority(prepared.Topics(), extension)
		if !slices.Contains(domains, "rendering") || !slices.Contains(topics, "rendering/adapter-outputs") {
			t.Fatalf("extension ownership = domains %v topics %v", domains, topics)
		}
	}
}

// invariant: rendering/sync-and-drift:managed-output-attribution (TestManagedOutputAttribution)
func TestManagedOutputAttribution(t *testing.T) {
	// Output declarations are built before render and classify planned outputs.
	state, err := project.Open(testContext(t), filepath.Clean(filepath.Join("..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(state.OutputState(), testConfig(state), NewFilesystemReader(state.Root()), project.Version).Prepare(); err != nil {
		t.Fatal(err)
	}
}
