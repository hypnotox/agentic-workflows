package project

import (
	"os"
	"strings"
	"testing"
)

// invariant: tooling/cli:pitfall-scaffold (TestPitfallScaffoldPresentationBoundary)
func TestPitfallScaffoldPresentationBoundary(t *testing.T) {
	for _, name := range []string{"scaffold.go", "operations.go"} {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		for _, forbidden := range []string{"PitfallScaffoldDocument", "NewPitfall("} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s retains focused pitfall mutation or presentation symbol %q", name, forbidden)
			}
		}
	}
}
