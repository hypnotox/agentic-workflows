package docs

import (
	"regexp"
	"testing"
)

func TestGuidesReferenceAvailableCLIPages(t *testing.T) {
	// Guides and generated skill bodies navigate through the CLI rather than
	// relying on an adopter checkout containing AWF's internal/docs directory.
	route := regexp.MustCompile("`(?:\\./awf |awf )?docs ([a-z]+)`")
	for name := range pageFiles {
		body, ok := Page(name)
		if !ok || len(body) == 0 {
			t.Fatalf("empty page %q", name)
		}
		for _, match := range route.FindAllSubmatch(body, -1) {
			if _, ok := Page(string(match[1])); !ok {
				t.Errorf("%s references unavailable CLI page %s", name, match[1])
			}
		}
	}
	if _, ok := Page("unknown"); ok {
		t.Fatal("unknown page was accepted")
	}
}
