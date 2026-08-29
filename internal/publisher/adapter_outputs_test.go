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
	want := []string{
		".pi/extensions/awf-effort/client.ts",
		".pi/extensions/awf-effort/index.ts",
		".pi/extensions/awf-subagents/index.ts",
		".pi/extensions/awf-subagents/model-routing.ts",
	}
	var extensions []string
	for _, output := range prepared.Plan().Outputs() {
		if filepath.ToSlash(filepath.Dir(output.Path())) != ".pi/extensions/awf-effort" && filepath.ToSlash(filepath.Dir(output.Path())) != ".pi/extensions/awf-subagents" {
			continue
		}
		extensions = append(extensions, output.Path())
		domains, topics := topic.PathAuthority(prepared.Topics(), output.Path())
		if !slices.Contains(domains, "rendering") || !slices.Contains(topics, "rendering/adapter-outputs") {
			t.Errorf("extension %q ownership = domains %v topics %v", output.Path(), domains, topics)
		}
	}
	slices.Sort(extensions)
	if !slices.Equal(extensions, want) {
		t.Fatalf("generated extension outputs = %v, want %v", extensions, want)
	}
}
