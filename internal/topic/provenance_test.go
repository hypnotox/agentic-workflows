package topic_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// writePart writes one authored claim part under the topics tree. Every case
// here needs exactly one in-scope part, so its topic is fixed.
func writePart(t *testing.T, root, body string) string {
	t.Helper()
	path := filepath.Join(root, ".awf", "topics", "parts", "d", "t", "current-state.md")
	testsupport.WriteFile(t, path, body)
	return path
}

// TestSubstituteProvenanceRewritesOnlyProvenanceLines covers the substitution
// half of numbering over one part holding every case at once: an Origin naming
// a numbered slug, an Origin naming a slug this run does not number, a
// Revised-by list whose substituted entry belongs numerically before an entry
// already in it, a Revised-by list nothing in the run touches, and a prose line
// mentioning the same slug outside the provenance grammar.
func TestSubstituteProvenanceRewritesOnlyProvenanceLines(t *testing.T) {
	root := t.TempDir()
	body := "Intro.\n\n## Claims\n\n" +
		"### `rule: origin-renamed`\nADR-alpha decided this, per ADR-alpha.\nOrigin: ADR-alpha\n\n" +
		"### `rule: origin-kept`\nProse.\nOrigin: ADR-untouched\n\n" +
		"### `rule: revised-resorted`\nProse.\nOrigin: ADR-0001\nRevised-by: ADR-0400, ADR-alpha\n\n" +
		"### `rule: revised-kept`\nProse.\nOrigin: ADR-0001\nRevised-by: ADR-0002, ADR-untouched\n"
	path := writePart(t, root, body)

	if err := topic.SubstituteProvenance(root, map[string]string{"alpha": "0200"}); err != nil {
		t.Fatalf("SubstituteProvenance: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "Intro.\n\n## Claims\n\n" +
		"### `rule: origin-renamed`\nADR-alpha decided this, per ADR-alpha.\nOrigin: ADR-0200\n\n" +
		"### `rule: origin-kept`\nProse.\nOrigin: ADR-untouched\n\n" +
		"### `rule: revised-resorted`\nProse.\nOrigin: ADR-0001\nRevised-by: ADR-0200, ADR-0400\n\n" +
		"### `rule: revised-kept`\nProse.\nOrigin: ADR-0001\nRevised-by: ADR-0002, ADR-untouched\n"
	if string(got) != want {
		t.Errorf("substitution wrong:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestSubstituteProvenanceDeduplicatesTouchedList proves the canonical form is
// duplicate-free as well as ascending: a list naming both a numbered record and
// the slug that took that same number collapses to one entry.
func TestSubstituteProvenanceDeduplicatesTouchedList(t *testing.T) {
	root := t.TempDir()
	path := writePart(t, root,
		"Intro.\n\n## Claims\n\n### `rule: x`\nProse.\nOrigin: ADR-0001\nRevised-by: ADR-0200, ADR-alpha\n")

	if err := topic.SubstituteProvenance(root, map[string]string{"alpha": "0200"}); err != nil {
		t.Fatalf("SubstituteProvenance: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "Intro.\n\n## Claims\n\n### `rule: x`\nProse.\nOrigin: ADR-0001\nRevised-by: ADR-0200\n"
	if string(got) != want {
		t.Errorf("duplicate not collapsed:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestSubstituteProvenanceLeavesUnrelatedInputsAlone covers the three ways the
// substitution declines to act: an empty rename set writes nothing at all, a
// non-part file under the parts tree is skipped, and a provenance line whose
// value does not parse is left for the corpus loader to diagnose rather than
// rewritten into a different malformation.
func TestSubstituteProvenanceLeavesUnrelatedInputsAlone(t *testing.T) {
	root := t.TempDir()
	part := writePart(t, root,
		"Intro.\n\n## Claims\n\n### `rule: x`\nProse.\nOrigin: not-an-adr-ref\nRevised-by: ADR-alpha, oops\n")
	stray := filepath.Join(root, ".awf", "topics", "parts", "d", "t", "notes.md")
	testsupport.WriteFile(t, stray, "Origin: ADR-alpha\n")
	original, err := os.ReadFile(part)
	if err != nil {
		t.Fatal(err)
	}

	if err := topic.SubstituteProvenance(root, nil); err != nil {
		t.Fatalf("empty rename set: %v", err)
	}
	if err := topic.SubstituteProvenance(root, map[string]string{"alpha": "0200"}); err != nil {
		t.Fatalf("SubstituteProvenance: %v", err)
	}
	after, err := os.ReadFile(part)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Errorf("unparseable provenance was rewritten:\ngot:\n%s\nwant:\n%s", after, original)
	}
	notes, err := os.ReadFile(stray)
	if err != nil {
		t.Fatal(err)
	}
	if string(notes) != "Origin: ADR-alpha\n" {
		t.Errorf("a non-part file under the parts tree must not be rewritten: %s", notes)
	}
}

// TestSubstituteProvenanceWalksOnlyTheTopicsPartsTree pins the second scoping
// rule. The basename filter alone does not confine the effect: this repo really
// does carry current-state.md outside the topics parts tree - every domain part
// is one, and a nested adopter tree holds a whole second topics tree - so a walk
// rooted anywhere higher would rewrite files numbering has no business touching.
// The domain part here is the shape that would be hit first.
func TestSubstituteProvenanceWalksOnlyTheTopicsPartsTree(t *testing.T) {
	root := t.TempDir()
	part := writePart(t, root,
		"Intro.\n\n## Claims\n\n### `rule: x`\nProse.\nOrigin: ADR-alpha\n")
	outside := map[string]string{
		filepath.Join(root, ".awf", "domains", "parts", "d", "current-state.md"):                      "Narrative.\nOrigin: ADR-alpha\n",
		filepath.Join(root, "examples", "n", ".awf", "topics", "parts", "d", "t", "current-state.md"): "Nested.\nOrigin: ADR-alpha\n",
	}
	for path, body := range outside {
		testsupport.WriteFile(t, path, body)
	}

	if err := topic.SubstituteProvenance(root, map[string]string{"alpha": "0200"}); err != nil {
		t.Fatalf("SubstituteProvenance: %v", err)
	}

	got, err := os.ReadFile(part)
	if err != nil {
		t.Fatal(err)
	}
	if want := "Intro.\n\n## Claims\n\n### `rule: x`\nProse.\nOrigin: ADR-0200\n"; string(got) != want {
		t.Errorf("the in-scope part was not substituted:\ngot:\n%s\nwant:\n%s", got, want)
	}
	for path, body := range outside {
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(after) != body {
			t.Errorf("%s is outside the topics parts tree and must not be rewritten:\ngot:\n%s\nwant:\n%s", path, after, body)
		}
	}
}
