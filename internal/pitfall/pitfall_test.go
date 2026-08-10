package pitfall

import (
	"strings"
	"testing"
)

func source(path, front, body string) SourceFile {
	return SourceFile{Path: path, Regular: true, Bytes: []byte("---\n" + front + "---\n" + body)}
}

// invariant: rendering/doc-outputs:pitfall-corpus-validated (TestCorpusContract)
func TestCorpusContract(t *testing.T) {
	valid := source(SourceDir+"/alpha.md", "title: Alpha\ndomains: [rendering]\ntags: [proof]\nrelated: [1]\n", "body\n")
	corpus, err := Load([]SourceFile{valid})
	if err != nil || corpus.Len() != 1 {
		t.Fatalf("load = %#v, %v", corpus.All(), err)
	}
	entry := corpus.All()[0]
	if entry.Title != "Alpha" || entry.SourcePath != valid.Path || entry.Body != "body\n" {
		t.Fatalf("entry = %#v", entry)
	}

	bad := []struct {
		name string
		file SourceFile
		want string
	}{
		{"nested", source(SourceDir+"/nested/a.md", "title: A\n", "body\n"), "direct child"},
		{"outside", source("elsewhere/a.md", "title: A\n", "body\n"), "direct child"},
		{"mode", SourceFile{Path: SourceDir + "/a.md", Bytes: valid.Bytes}, "regular"},
		{"extension", source(SourceDir+"/a.txt", "title: A\n", "body\n"), ".md"},
		{"reserved", source(SourceDir+"/index.md", "title: A\n", "body\n"), "reserved"},
		{"slug", source(SourceDir+"/Bad_slug.md", "title: A\n", "body\n"), "invalid"},
		{"frontmatter", SourceFile{Path: SourceDir + "/a.md", Regular: true, Bytes: []byte("body")}, "missing frontmatter"},
		{"unknown", source(SourceDir+"/a.md", "title: A\nunknown: x\n", "body\n"), "field unknown"},
		{"missing-title", source(SourceDir+"/a.md", "domains: [x]\n", "body\n"), "required title"},
		{"empty-title", source(SourceDir+"/a.md", "title: '  '\n", "body\n"), "title is empty"},
		{"newline-title", source(SourceDir+"/a.md", "title: |\n  A\n  B\n", "body\n"), "CR or LF"},
		{"empty-body", source(SourceDir+"/a.md", "title: A\n", " \n"), "body is empty"},
		{"empty-domain", source(SourceDir+"/a.md", "title: A\ndomains: ['']\n", "body\n"), "nonempty"},
		{"duplicate-domain", source(SourceDir+"/a.md", "title: A\ndomains: [x, x]\n", "body\n"), "duplicate domains"},
		{"bad-tag", source(SourceDir+"/a.md", "title: A\ntags: [' x ']\n", "body\n"), "tags entries"},
		{"bad-related", source(SourceDir+"/a.md", "title: A\nrelated: [0]\n", "body\n"), "positive"},
		{"duplicate-related", source(SourceDir+"/a.md", "title: A\nrelated: [1, 1]\n", "body\n"), "duplicate related"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Load([]SourceFile{tc.file}); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	if _, err := Load([]SourceFile{valid, valid}); err == nil || !strings.Contains(err.Error(), "duplicate source") {
		t.Fatal(err)
	}
	dup := source(SourceDir+"/beta.md", "title: \"  aLpHa\\t\"\n", "body\n")
	if _, err := Load([]SourceFile{valid, dup}); err == nil || !strings.Contains(err.Error(), "duplicates") {
		t.Fatal(err)
	}
	if !EqualTitle("Kelvin\u2003X", "kelvin X") {
		t.Fatal("simple fold/whitespace equivalence failed")
	}
}

func TestAllocationSerializationEscapingAndLinks(t *testing.T) {
	used := map[string]bool{"hello-world": true, "hello-world-2": true, "hello-world-4": true}
	if got, err := AllocateSlug(" Hello, WORLD! ", used); err != nil || got != "hello-world-3" {
		t.Fatalf("slug = %q, %v", got, err)
	}
	for _, title := range []string{"日本語", "index"} {
		if _, err := AllocateSlug(title, map[string]bool{}); err == nil {
			t.Fatalf("%q allocated", title)
		}
	}
	e := Entry{Slug: "a", SourcePath: SourceDir + "/a.md", Title: "A [x] | `y` \\ z", Domains: []string{"d"}, Tags: []string{"t"}, Related: []int{2}, Body: "body"}
	serialized, err := Serialize(e)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(SourceFile{Path: e.SourcePath, Regular: true, Bytes: serialized})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Title != e.Title || parsed.Body != "body\n" {
		t.Fatalf("round trip = %#v", parsed)
	}
	if _, err := Serialize(Entry{Title: "A"}); err == nil {
		t.Fatal("empty body serialized")
	}
	for _, escaped := range []string{EscapeHeading(e.Title), EscapeLinkLabel(e.Title), EscapeTableCell(e.Title)} {
		if !strings.Contains(escaped, `\[x\]`) || !strings.Contains(escaped, `\|`) || !strings.Contains(escaped, `\\`) {
			t.Fatalf("escaped = %q", escaped)
		}
	}
	e.Body = "[inline](rel.md) ![image](img/a.png)\n<other.md>\n[id]: refs/a.md\n[external](https://example.com) [root](/x) [frag](#x) ` [code](no.md) `\n```md\n[fenced](no.md)\n```\n"
	links := RelativeLinks(e)
	if len(links) != 4 {
		t.Fatalf("links = %#v", links)
	}
	for _, l := range links {
		if l.Source != e.SourcePath || strings.Contains(l.Destination, "no.md") {
			t.Fatalf("link = %#v", l)
		}
	}
}
