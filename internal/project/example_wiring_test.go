package project

import (
	"os"
	"strings"
	"testing"
)

// ADR-0090: the committed example adopter is kept deterministic through ./x —
// sync re-renders it from source; check drift-, invariant-, note-, and
// test-gates it. The example is its own Go module so the enclosing ./...
// sweeps never see it; this test pins the wiring so it cannot be silently
// dropped.
//
// invariant: example-adopter-checked
// invariant: example-zero-notes
// invariant: example-module-isolated
func TestExampleAdopterWiring(t *testing.T) {
	raw, err := os.ReadFile("../../x")
	if err != nil {
		t.Fatalf("read x: %v", err)
	}
	script := string(raw)
	for _, want := range []string{
		`(cd examples/sundial && "$bindir/awf" sync)`,
		`out="$(cd examples/sundial && "$bindir/awf" check)"`,
		`grep -q '^note: '`,
		`(cd examples/sundial && "$bindir/awf" invariants)`,
		`(cd examples/sundial && go test ./...)`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("x lost the example-adopter step %q (ADR-0090)", want)
		}
	}
	if _, err := os.Stat("../../examples/sundial/go.mod"); err != nil {
		t.Errorf("examples/sundial must stay its own Go module (ADR-0090): %v", err)
	}
}
