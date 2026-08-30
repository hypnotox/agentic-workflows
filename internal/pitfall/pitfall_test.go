package pitfall

import (
	"strings"
	"testing"
)

func source(path, front, body string) SourceFile {
	return SourceFile{Path: path, Regular: true, Bytes: []byte("---\n" + front + "---\n" + body)}
}

// invariant: rendering/doc-outputs:pitfall-corpus-validated (TestCorpusContract)
// invariant: config/configuration:no-active-tag-system (TestCorpusContract)
func TestCorpusContract(t *testing.T) {
	valid := source(SourceDir+"/alpha.md", "title: Alpha\ndomains: [rendering]\n", "body\n")
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
		{"traversal-shaped", source(SourceDir+"/nested/../entry.md", "title: A\n", "body\n"), "canonical slash-relative"},
		{"repeated-separator", source(SourceDir+"//entry.md", "title: A\n", "body\n"), "canonical slash-relative"},
		{"dot-segment", source(SourceDir+"/./entry.md", "title: A\n", "body\n"), "canonical slash-relative"},
		{"backslash", source(`.awf\docs\pitfalls\entry.md`, "title: A\n", "body\n"), "canonical slash-relative"},
		{"absolute", source("/"+SourceDir+"/entry.md", "title: A\n", "body\n"), "canonical slash-relative"},
		{"outside", source("elsewhere/a.md", "title: A\n", "body\n"), "direct child"},
		{"mode", SourceFile{Path: SourceDir + "/a.md", Bytes: valid.Bytes}, "regular"},
		{"extension", source(SourceDir+"/a.txt", "title: A\n", "body\n"), ".md"},
		{"reserved", source(SourceDir+"/index.md", "title: A\n", "body\n"), "reserved"},
		{"slug", source(SourceDir+"/Bad_slug.md", "title: A\n", "body\n"), "invalid"},
		{"frontmatter", SourceFile{Path: SourceDir + "/a.md", Regular: true, Bytes: []byte("body")}, "missing frontmatter"},
		{"metadata-not-mapping", source(SourceDir+"/a.md", "- title: A\n", "body\n"), "must be a mapping"},
		{"title-sequence", source(SourceDir+"/a.md", "title: [A]\n", "body\n"), "string scalar"},
		{"title-number", source(SourceDir+"/a.md", "title: 12\n", "body\n"), "string scalar"},
		{"title-bool", source(SourceDir+"/a.md", "title: true\n", "body\n"), "string scalar"},
		{"unknown", source(SourceDir+"/a.md", "title: A\nunknown: x\n", "body\n"), "field unknown"},
		{"retired-tags", source(SourceDir+"/a.md", "title: A\ntags: [legacy]\n", "body\n"), "field tags"},
		{"duplicate-key", source(SourceDir+"/a.md", "title: A\ntitle: B\n", "body\n"), "duplicate pitfall metadata key"},
		{"missing-title", source(SourceDir+"/a.md", "domains: [x]\n", "body\n"), "required title"},
		{"empty-title", source(SourceDir+"/a.md", "title: '  '\n", "body\n"), "title is empty"},
		{"newline-title", source(SourceDir+"/a.md", "title: |\n  A\n  B\n", "body\n"), "CR or LF"},
		{"empty-body", source(SourceDir+"/a.md", "title: A\n", " \n"), "body is empty"},
		{"empty-domain", source(SourceDir+"/a.md", "title: A\ndomains: ['']\n", "body\n"), "nonempty"},
		{"empty-domains-list", source(SourceDir+"/a.md", "title: A\ndomains: []\n", "body\n"), "nonempty list"},
		{"null-domains", source(SourceDir+"/a.md", "title: A\ndomains: null\n", "body\n"), "nonempty list"},
		{"scalar-domains", source(SourceDir+"/a.md", "title: A\ndomains: x\n", "body\n"), "nonempty list"},
		{"mapping-domains", source(SourceDir+"/a.md", "title: A\ndomains: {x: y}\n", "body\n"), "nonempty list"},
		{"numeric-domain", source(SourceDir+"/a.md", "title: A\ndomains: [12]\n", "body\n"), "string scalars"},
		{"bool-domain", source(SourceDir+"/a.md", "title: A\ndomains: [true]\n", "body\n"), "string scalars"},
		{"duplicate-domain", source(SourceDir+"/a.md", "title: A\ndomains: [x, x]\n", "body\n"), "duplicate domains"},
		{"retired-related", source(SourceDir+"/a.md", "title: A\nrelated: [1]\n", "body\n"), "field related"},
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

func TestScaffoldUsesCorpusIdentityAndCanonicalSource(t *testing.T) {
	corpus := New([]Entry{
		{Slug: "durable-hazard", SourcePath: SourceDir + "/durable-hazard.md", Title: "Other"},
		{Slug: "durable-hazard-2", SourcePath: SourceDir + "/durable-hazard-2.md", Title: "Another"},
		{Slug: "durable-hazard-4", SourcePath: SourceDir + "/durable-hazard-4.md", Title: "Fourth"},
	})
	entry, source, err := corpus.Scaffold("  Durable, HAZARD!  ")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Slug != "durable-hazard-3" || entry.SourcePath != SourceDir+"/durable-hazard-3.md" || entry.Title != "Durable, HAZARD!" {
		t.Fatalf("entry = %#v", entry)
	}
	const want = "---\ntitle: Durable, HAZARD!\n---\nDescribe the durable hazard, its consequence, and the safer practice.\n"
	if string(source) != want {
		t.Fatalf("source = %q, want %q", source, want)
	}
	for _, tc := range []struct{ title, want string }{
		{"  ", "empty"},
		{"index", "reserved"},
		{"日本語", "no ASCII"},
		{"line\nbreak", "CR or LF"},
	} {
		if _, _, err := corpus.Scaffold(tc.title); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("Scaffold(%q) error = %v, want %q", tc.title, err, tc.want)
		}
	}
	duplicateCorpus := New([]Entry{{Slug: "existing", SourcePath: SourceDir + "/existing.md", Title: "Kelvin   Hazard"}})
	if _, _, err := duplicateCorpus.Scaffold("  kelvin\tHAZARD "); err == nil || !strings.Contains(err.Error(), "duplicates") {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestAllocationSerializationEscaping(t *testing.T) {
	used := map[string]bool{"hello-world": true, "hello-world-2": true, "hello-world-4": true}
	if got, err := AllocateSlug(" Hello, WORLD! ", used); err != nil || got != "hello-world-3" {
		t.Fatalf("slug = %q, %v", got, err)
	}
	for _, title := range []string{"日本語", "index"} {
		if _, err := AllocateSlug(title, map[string]bool{}); err == nil {
			t.Fatalf("%q allocated", title)
		}
	}
	e := Entry{Slug: "a", SourcePath: SourceDir + "/a.md", Title: "A [x] | `y` \\ z", Domains: []string{"d"}, Body: "body"}
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
	punctuation := "# [domain] | `code` \\ path\r\nnext"
	for _, escaped := range []string{EscapeHeading(punctuation), EscapeLinkLabel(punctuation), EscapeTableCell(punctuation)} {
		for _, want := range []string{`\#`, `\[domain\]`, `\|`, `\` + "`code\\`", `\\`, "&#13;", "&#10;"} {
			if !strings.Contains(escaped, want) {
				t.Fatalf("metadata escape %q missing %q", escaped, want)
			}
		}
		if strings.ContainsAny(escaped, "\r\n") {
			t.Fatalf("metadata escape retained structural line break: %q", escaped)
		}
	}

}
