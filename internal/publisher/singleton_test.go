package publisher

import (
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

// invariant: rendering/doc-outputs:working-with-awf-mandatory (TestWorkingWithAwfMandatorySingleton)
func TestWorkingWithAwfMandatorySingleton(t *testing.T) {
	const kind = "working-with-awf"
	if !slices.Contains(catalog.SingletonKindsFor(catalog.Standard), kind) {
		t.Fatalf("catalog singleton kinds omit %q", kind)
	}
	for _, singleton := range plainSingletons(catalog.Standard) {
		if singleton.kind == kind {
			return
		}
	}
	t.Fatalf("plain singleton set omits %q", kind)
}
