package publisher

import "testing"

// invariant: rendering/adapter-outputs:generated-adapter-runtime-ownership (TestPiAdapterOutputsAreSubagentOnly)
func TestPiAdapterOutputsAreSubagentOnly(t *testing.T) {
	want := map[string]bool{".pi/extensions/awf-subagents/index.ts": true, ".pi/extensions/awf-subagents/model-routing.ts": true}
	for _, output := range piTarget.Outputs {
		if !want[output.Path] {
			t.Fatalf("unexpected Pi output %q", output.Path)
		}
		delete(want, output.Path)
	}
	if len(want) != 0 {
		t.Fatalf("missing outputs: %#v", want)
	}
}
