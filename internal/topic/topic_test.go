package topic

import (
	"strings"
	"testing"
)

func TestParseMetadataAccepted(t *testing.T) {
	root := "/repo/.awf/topics/metadata"
	id, m, err := ParseMetadata(root, root+"/rendering/output-plan.yaml", []byte("title: Output plan\nsummary: Deterministic outputs.\npaths: [\"internal/**\"]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if id.String() != "rendering/output-plan" || m.Title != "Output plan" || len(m.Paths) != 1 {
		t.Fatalf("got %#v %#v", id, m)
	}
	_, m, err = ParseMetadata(root, root+"/rendering/global.yaml", []byte("title: Global\nsummary: Everywhere.\napplies: global\n"))
	if err != nil || m.Applies != "global" {
		t.Fatalf("global: %#v %v", m, err)
	}
}
func TestParseMetadataRejected(t *testing.T) {
	root := ".awf/topics/metadata"
	canonical := root + "/a/b.yaml"
	cases := map[string]struct{ path, body string }{
		"outside root": {"bad.yaml", "title: X\nsummary: X\npaths: [x]\n"}, "nested path": {root + "/a/nested/b.yaml", "title: X\nsummary: X\npaths: [x]\n"}, "identity": {root + "/Bad/x.yaml", "title: X\nsummary: X\npaths: [x]\n"}, "yaml": {canonical, "title: [\n"}, "not mapping": {canonical, "- title\n"}, "duplicate field": {canonical, "title: X\ntitle: Y\nsummary: X\npaths: [x]\n"}, "paths not sequence": {canonical, "title: X\nsummary: X\npaths: {}\n"}, "unknown": {canonical, "title: X\nsummary: X\npaths: [x]\ndata: {}\n"}, "second valid document": {canonical, "title: X\nsummary: X\npaths: [x]\n---\ntitle: Y\nsummary: Y\npaths: [y]\n"}, "second unknown document": {canonical, "title: X\nsummary: X\npaths: [x]\n---\ndata: {}\n"}, "title": {canonical, "summary: X\npaths: [x]\n"}, "summary empty": {canonical, "title: X\npaths: [x]\n"}, "summary multiline": {canonical, "title: X\nsummary: |\n  x\n  y\npaths: [x]\n"}, "neither": {canonical, "title: X\nsummary: X\n"}, "both": {canonical, "title: X\nsummary: X\npaths: [x]\napplies: global\n"}, "applies": {canonical, "title: X\nsummary: X\napplies: local\n"}, "empty path": {canonical, "title: X\nsummary: X\npaths: [\"\"]\n"}, "duplicate": {canonical, "title: X\nsummary: X\npaths: [x, x]\n"}, "glob": {canonical, "title: X\nsummary: X\npaths: ['[']\n"}}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := ParseMetadata(root, tc.path, []byte(tc.body)); err == nil {
				t.Fatal("wanted error")
			}
		})
	}
}

func TestParseMetadataRejectsNonStringScalars(t *testing.T) {
	root := ".awf/topics/metadata"
	path := root + "/a/b.yaml"
	cases := map[string]string{
		"numeric title":   "title: 123\nsummary: X\npaths: [x]\n",
		"boolean summary": "title: X\nsummary: true\npaths: [x]\n",
		"null applies":    "title: X\nsummary: X\napplies: null\n",
		"numeric path":    "title: X\nsummary: X\npaths: [123]\n",
		"boolean path":    "title: X\nsummary: X\npaths: [true]\n",
		"null path":       "title: X\nsummary: X\npaths: [null]\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := ParseMetadata(root, path, []byte(body))
			if err == nil || !strings.Contains(err.Error(), "must be a string scalar") {
				t.Fatalf("ParseMetadata = %v", err)
			}
		})
	}
}
func TestParsePartAccepted(t *testing.T) {
	part := `<!-- awf:comment author note -->
Intro with {{ .notATemplate }}.

## Claims

### ` + "`rule: stable-order`" + `
Order is stable.
Origin: ADR-0001
Revised-by: ADR-0002, ADR-0003
References: other/topic:claim

### ` + "`invariant: tested`" + `
Tests prove this.
Origin: ADR-0001
Backing: test

### ` + "`invariant: explained`" + `
Review manually.
Origin: ADR-0001
Backing: unbacked
Verify: inspect output
`
	topic, err := ParsePart(TopicID{"rendering", "plan"}, "current-state.md", []byte(part))
	if err != nil {
		t.Fatal(err)
	}
	if topic.Intro == "" || !strings.Contains(topic.Part, "awf:comment") || len(topic.Claims) != 3 || topic.Claims[0].ID != "rendering/plan:stable-order" || topic.Claims[0].RevisedBy[1] != "0003" || topic.Claims[2].Verify != "inspect output" {
		t.Fatalf("%#v", topic)
	}
	empty, err := ParsePart(TopicID{"a", "b"}, "current-state.md", []byte("Intro\n\n## Claims\n"))
	if err != nil || len(empty.Claims) != 0 {
		t.Fatalf("empty: %#v %v", empty, err)
	}
}

// invariant: invariants/topics-and-markers:invariants-duplicate-slug
// invariant: invariants/topics-and-markers:unbacked-requires-verify-note
func TestParsePartRejected(t *testing.T) {
	head := "### `rule: x`\nProse.\nOrigin: ADR-0001\n"
	cases := map[string]string{
		"missing intro": "## Claims\n",
		"missing":       "Intro", "duplicate claims section": "## Claims\n\n## Claims\n", "later section": "## Claims\n\n## Later\n", "content": "## Claims\nnot heading\n", "bad heading": "## Claims\n### rule: x\n", "nested heading": "## Claims\n### `rule: x`\ntext\n# nested\nOrigin: ADR-0001\n", "duplicate slug": "## Claims\n" + head + "\n" + head,
		"malformed authoring comment": "<!-- awf:comment no close\n## Claims\n", "comment-only intro": "<!-- awf:comment explanation -->\n## Claims\n", "empty prose": "## Claims\n### `rule: x`\nOrigin: ADR-0001\n", "comment-only prose": "## Claims\n### `rule: x`\n<!-- awf:comment claim -->\nOrigin: ADR-0001\n", "reserved in prose": "## Claims\n### `rule: x`\nOrigin ADR-0001\nOrigin: ADR-0001\n", "missing origin": "## Claims\n### `rule: x`\nProse\n", "bad origin": "## Claims\n### `rule: x`\nProse\nOrigin: 1\n", "empty origin": "## Claims\n### `rule: x`\nProse\nOrigin: \n", "bad revised": "## Claims\n### `rule: x`\nProse\nOrigin: ADR-0001\nRevised-by: nope\n", "dup revised": "## Claims\n### `rule: x`\nProse\nOrigin: ADR-0001\nRevised-by: ADR-0002, ADR-0002\n", "bad ref": "## Claims\n### `rule: x`\nProse\nOrigin: ADR-0001\nReferences: nope\n", "dup ref": "## Claims\n### `rule: x`\nProse\nOrigin: ADR-0001\nReferences: a/b:c, a/b:c\n", "rule backing": "## Claims\n### `rule: x`\nProse\nOrigin: ADR-0001\nBacking: test\n", "invariant missing": "## Claims\n### `invariant: x`\nProse\nOrigin: ADR-0001\n", "bad backing": "## Claims\n### `invariant: x`\nProse\nOrigin: ADR-0001\nBacking: magic\n", "test verify": "## Claims\n### `invariant: x`\nProse\nOrigin: ADR-0001\nBacking: test\nVerify: no\n", "unbacked no verify": "## Claims\n### `invariant: x`\nProse\nOrigin: ADR-0001\nBacking: unbacked\n", "after verify": "## Claims\n### `invariant: x`\nProse\nOrigin: ADR-0001\nBacking: unbacked\nVerify: yes\nVerify: twice\n", "out of order": "## Claims\n### `rule: x`\nProse\nReferences: a/b:c\nOrigin: ADR-0001\n"}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if name != "missing intro" && name != "malformed authoring comment" && name != "comment-only intro" && name != "missing" {
				body = "Intro.\n\n" + body
			}
			_, err := ParsePart(TopicID{"a", "b"}, "part", []byte(body))
			if err == nil {
				t.Fatal("wanted error")
			}
			if strings.TrimSpace(err.Error()) == "" {
				t.Fatal("empty diagnostic")
			}
		})
	}
}
