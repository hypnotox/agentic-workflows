package project

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// invariant: rendering/doc-outputs:pitfall-output-complete (TestPitfallDogfoodSourceOutputParity)
func TestPitfallDogfoodSourceOutputParity(t *testing.T) {
	root := filepath.Clean("../..")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := p.loadPitfallCorpus()
	if err != nil {
		t.Fatal(err)
	}
	var source []string
	for _, e := range corpus.All() {
		source = append(source, e.Slug)
	}
	slices.Sort(source)
	matches, err := filepath.Glob(filepath.Join(root, "docs", "pitfalls", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	output := make([]string, 0, len(matches))
	for _, match := range matches {
		output = append(output, strings.TrimSuffix(filepath.Base(match), ".md"))
	}
	slices.Sort(output)
	indexBytes, err := os.ReadFile(filepath.Join(root, "docs", "pitfalls.md"))
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`\(pitfalls/([a-z0-9-]+)\.md\)`)
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(string(indexBytes), -1) {
		seen[m[1]] = true
	}
	index := make([]string, 0, len(seen))
	for slug := range seen {
		index = append(index, slug)
	}
	slices.Sort(index)
	if !slices.Equal(source, output) || !slices.Equal(source, index) {
		t.Fatalf("pitfall parity mismatch\nsource-only=%v\noutput-only=%v\nindex-only=%v", difference(source, output), difference(output, source), difference(source, index))
	}
}

func difference(a, b []string) []string {
	set := map[string]bool{}
	for _, value := range b {
		set[value] = true
	}
	var out []string
	for _, value := range a {
		if !set[value] {
			out = append(out, value)
		}
	}
	return out
}

// invariant: code-design/single-home:pitfall-model-single-home (TestPitfallModelSingleHome)
func TestPitfallModelSingleHome(t *testing.T) {
	root := filepath.Clean("../..")
	var declarations []string
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, de os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if de.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(b)
		for _, forbidden := range []string{"type pitfallEntry struct", "func AllocateSlug(", "func EqualTitle(", "func Serialize("} {
			if strings.Contains(text, forbidden) {
				declarations = append(declarations, filepath.ToSlash(path)+":"+forbidden)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range declarations {
		if !strings.Contains(declaration, "/internal/pitfall/") {
			t.Fatalf("pitfall semantic declaration outside internal/pitfall: %s", declaration)
		}
	}
	if len(declarations) < 3 {
		t.Fatalf("semantic ownership scan found too few declarations: %v", declarations)
	}
}
