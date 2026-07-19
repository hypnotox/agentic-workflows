package topic

import (
	"strings"
	"testing"
)

func TestParseMetadataAccepted(t *testing.T) {
	id, m, err := ParseMetadata("/repo/.awf/topics/metadata/rendering/output-plan.yaml", []byte("title: Output plan\nsummary: Deterministic outputs.\npaths: [\"internal/**\"]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if id.String() != "rendering/output-plan" || m.Title != "Output plan" || len(m.Paths) != 1 {
		t.Fatalf("got %#v %#v", id, m)
	}
	_, m, err = ParseMetadata(".awf/topics/metadata/rendering/global.yaml", []byte("title: Global\nsummary: Everywhere.\napplies: global\n"))
	if err != nil || m.Applies != "global" {
		t.Fatalf("global: %#v %v", m, err)
	}
}
func TestParseMetadataRejected(t *testing.T) {
	cases := map[string]struct{ path, body string }{
		"path": {"bad.yaml", "title: X\nsummary: X\npaths: [x]\n"}, "identity": {".awf/topics/metadata/Bad/x.yaml", "title: X\nsummary: X\npaths: [x]\n"}, "yaml": {".awf/topics/metadata/a/b.yaml", "title: [\n"}, "unknown": {".awf/topics/metadata/a/b.yaml", "title: X\nsummary: X\npaths: [x]\ndata: {}\n"}, "title": {".awf/topics/metadata/a/b.yaml", "summary: X\npaths: [x]\n"}, "summary empty": {".awf/topics/metadata/a/b.yaml", "title: X\npaths: [x]\n"}, "summary multiline": {".awf/topics/metadata/a/b.yaml", "title: X\nsummary: |\n  x\n  y\npaths: [x]\n"}, "neither": {".awf/topics/metadata/a/b.yaml", "title: X\nsummary: X\n"}, "both": {".awf/topics/metadata/a/b.yaml", "title: X\nsummary: X\npaths: [x]\napplies: global\n"}, "applies": {".awf/topics/metadata/a/b.yaml", "title: X\nsummary: X\napplies: local\n"}, "empty path": {".awf/topics/metadata/a/b.yaml", "title: X\nsummary: X\npaths: [\"\"]\n"}, "duplicate": {".awf/topics/metadata/a/b.yaml", "title: X\nsummary: X\npaths: [x, x]\n"}, "glob": {".awf/topics/metadata/a/b.yaml", "title: X\nsummary: X\npaths: ['[']\n"}}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := ParseMetadata(tc.path, []byte(tc.body)); err == nil {
				t.Fatal("wanted error")
			}
		})
	}
}
func TestParsePartAccepted(t *testing.T) {
	part := `Intro with {{ .notATemplate }}.

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
	if topic.Intro == "" || len(topic.Claims) != 3 || topic.Claims[0].ID != "rendering/plan:stable-order" || topic.Claims[0].RevisedBy[1] != "0003" || topic.Claims[2].Verify != "inspect output" {
		t.Fatalf("%#v", topic)
	}
	empty, err := ParsePart(TopicID{"a", "b"}, "current-state.md", []byte("Intro\n\n## Claims\n"))
	if err != nil || len(empty.Claims) != 0 {
		t.Fatalf("empty: %#v %v", empty, err)
	}
}
func TestParsePartRejected(t *testing.T) {
	head := "### `rule: x`\nProse.\nOrigin: ADR-0001\n"
	cases := map[string]string{
		"missing intro": "## Claims\n",
		"missing":       "Intro", "duplicate claims section": "## Claims\n\n## Claims\n", "later section": "## Claims\n\n## Later\n", "content": "## Claims\nnot heading\n", "bad heading": "## Claims\n### rule: x\n", "nested heading": "## Claims\n### `rule: x`\ntext\n# nested\nOrigin: ADR-0001\n", "duplicate slug": "## Claims\n" + head + "\n" + head,
		"empty prose": "## Claims\n### `rule: x`\nOrigin: ADR-0001\n", "reserved in prose": "## Claims\n### `rule: x`\nOrigin ADR-0001\nOrigin: ADR-0001\n", "missing origin": "## Claims\n### `rule: x`\nProse\n", "bad origin": "## Claims\n### `rule: x`\nProse\nOrigin: 1\n", "empty origin": "## Claims\n### `rule: x`\nProse\nOrigin: \n", "bad revised": "## Claims\n### `rule: x`\nProse\nOrigin: ADR-0001\nRevised-by: nope\n", "dup revised": "## Claims\n### `rule: x`\nProse\nOrigin: ADR-0001\nRevised-by: ADR-0002, ADR-0002\n", "bad ref": "## Claims\n### `rule: x`\nProse\nOrigin: ADR-0001\nReferences: nope\n", "dup ref": "## Claims\n### `rule: x`\nProse\nOrigin: ADR-0001\nReferences: a/b:c, a/b:c\n", "rule backing": "## Claims\n### `rule: x`\nProse\nOrigin: ADR-0001\nBacking: test\n", "invariant missing": "## Claims\n### `invariant: x`\nProse\nOrigin: ADR-0001\n", "bad backing": "## Claims\n### `invariant: x`\nProse\nOrigin: ADR-0001\nBacking: magic\n", "test verify": "## Claims\n### `invariant: x`\nProse\nOrigin: ADR-0001\nBacking: test\nVerify: no\n", "unbacked no verify": "## Claims\n### `invariant: x`\nProse\nOrigin: ADR-0001\nBacking: unbacked\n", "after verify": "## Claims\n### `invariant: x`\nProse\nOrigin: ADR-0001\nBacking: unbacked\nVerify: yes\nVerify: twice\n", "out of order": "## Claims\n### `rule: x`\nProse\nReferences: a/b:c\nOrigin: ADR-0001\n"}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if name != "missing intro" && name != "missing" {
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
