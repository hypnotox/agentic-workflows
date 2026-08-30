package topic

import (
	"strings"
	"testing"
)

func TestParseMetadataAccepted(t *testing.T) {
	id, m, err := ParseMetadata(".awf/topics/metadata", ".awf/topics/metadata/rendering/output-plan.yaml", []byte("title: Output plan\nsummary: Deterministic outputs.\npaths: [\"internal/**\"]\n"))
	if err != nil || id.String() != "rendering/output-plan" || m.Title != "Output plan" {
		t.Fatalf("got %#v %#v %v", id, m, err)
	}
}
func TestParseMetadataRejected(t *testing.T) {
	for _, body := range []string{"title: X\nsummary: X\n", "title: X\nsummary: X\npaths: ['[']\n", "title: [\n"} {
		if _, _, err := ParseMetadata(".awf/topics/metadata", ".awf/topics/metadata/a/b.yaml", []byte(body)); err == nil {
			t.Fatal("wanted error")
		}
	}
}
func TestParsePartRetainsCurrentStateFields(t *testing.T) {
	part := "Intro.\n\n## Claims\n\n### `rule: stable-order`\nOrder is stable.\nSummary: Stable ordering.\nReferences: other/topic:claim\n\n### `invariant: tested`\nTests prove this.\nBacking: test\n\n### `invariant: explained`\nReview manually.\nBacking: unbacked\nVerify: inspect output\n"
	topic, err := ParsePart(TopicID{"rendering", "plan"}, "current-state.md", []byte(part))
	if err != nil {
		t.Fatal(err)
	}
	if len(topic.Claims) != 3 || topic.Claims[0].Summary != "Stable ordering." || len(topic.Claims[0].References) != 1 || topic.Claims[1].Backing != TestBacking || topic.Claims[2].Verify != "inspect output" {
		t.Fatalf("%#v", topic)
	}
}
func TestParsePartRejectsRetiredProvenanceMetadata(t *testing.T) {
	for _, metadata := range []string{"Origin: ADR-0001\n", "Revised-by: ADR-0002\n"} {
		_, err := ParsePart(TopicID{"a", "b"}, "part", []byte("Intro.\n\n## Claims\n\n### `rule: x`\nProse.\n"+metadata))
		if err == nil || !strings.Contains(err.Error(), "reserved metadata") {
			t.Fatalf("metadata %q error = %v", metadata, err)
		}
	}
}

// invariant: invariants/topics-and-markers:invariants-duplicate-slug (TestParsePartRejected)
// invariant: invariants/topics-and-markers:unbacked-requires-verify-note (TestParsePartRejected)
func TestParsePartRejected(t *testing.T) {
	cases := []string{
		"Intro", "## Claims\n", "Intro.\n\n## Claims\n### `rule: x`\n",
		"Intro.\n\n## Claims\n### `rule: x`\nProse.\nBacking: test\n",
		"Intro.\n\n## Claims\n### `invariant: x`\nProse.\n",
		"Intro.\n\n## Claims\n### `invariant: x`\nProse.\nBacking: unbacked\n",
		"Intro.\n\n## Claims\n### `invariant: x`\nProse.\nBacking: test\nVerify: no\n",
		"Intro.\n\n## Claims\n### `rule: x`\nProse.\nReferences: a/b:c, a/b:c\n",
		"Intro.\n\n## Claims\n### `rule: x`\nFirst.\n\n### `rule: x`\nSecond.\n",
	}
	for _, body := range cases {
		if _, err := ParsePart(TopicID{"a", "b"}, "part", []byte(body)); err == nil {
			t.Fatalf("accepted %q", body)
		}
	}
}
func TestClaimSummaryMetadata(t *testing.T) {
	base := "Intro.\n\n## Claims\n\n### `rule: x`\nProse.\n"
	claim, err := ParsePart(TopicID{"a", "b"}, "part", []byte(base+"Summary: compact.\n"))
	if err != nil || claim.Claims[0].Summary != "compact." {
		t.Fatalf("%#v %v", claim, err)
	}
	for _, summary := range []string{"Summary: \n", "Summary: one\nSummary: two\n", "Summary: " + strings.Repeat("é", 161) + "\n"} {
		if _, err := ParsePart(TopicID{"a", "b"}, "part", []byte(base+summary)); err == nil {
			t.Fatal("accepted invalid summary")
		}
	}
}
