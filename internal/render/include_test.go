package render

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func partialsFS(files map[string]string) fstest.MapFS {
	m := fstest.MapFS{}
	for name, body := range files {
		m["partials/"+name+".md"] = &fstest.MapFile{Data: []byte(body)}
	}
	return m
}

func expandIncludes(src string, partialFS fs.FS) (string, error) {
	expanded, err := ExpandIncludesSource(src, "", partialFS)
	if err != nil {
		return "", err
	}
	return expanded.AuthoredText(), nil
}

func TestExpandIncludesSplices(t *testing.T) {
	src := "intro\n\n<!-- awf:include spine -->\n\ntail\n"
	out, err := expandIncludes(src, partialsFS(map[string]string{"spine": "BODY\n"}))
	if err != nil {
		t.Fatal(err)
	}
	// invariant: rendering/render-engine:include-splice (TestExpandIncludesSplices)
	if out != "intro\n\nBODY\n\ntail\n" {
		t.Fatalf("bad splice:\n%q", out)
	}
}

func TestExpandIncludesNoDirectivePassesThrough(t *testing.T) {
	src := "no directives here\n"
	out, err := expandIncludes(src, partialsFS(nil))
	if err != nil {
		t.Fatal(err)
	}
	if out != src {
		t.Fatalf("expected passthrough, got %q", out)
	}
}

func TestExpandIncludesSourceRetainsOrderedTransitions(t *testing.T) {
	src := "before\n<!-- awf:include spine -->\nafter\n"
	expanded, err := ExpandIncludesSource(src, "templates/guide.md.tmpl", partialsFS(map[string]string{"spine": "PART\n"}))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := expanded.AuthoredText(), "before\nPART\nafter\n"; got != want {
		t.Fatalf("flattened source = %q, want %q", got, want)
	}
	want := []SourceSpan{
		{Source: "templates/guide.md.tmpl", Text: "before\n"},
		{Source: "partials/spine.md", Text: "PART\n"},
		{Source: "templates/guide.md.tmpl", Text: "after\n"},
	}
	if !reflect.DeepEqual(expanded.Spans, want) {
		t.Fatalf("source spans = %#v, want %#v", expanded.Spans, want)
	}
}

func TestExpandIncludesMultiple(t *testing.T) {
	src := "<!-- awf:include a -->\nmid\n<!-- awf:include b -->\n"
	out, err := expandIncludes(src, partialsFS(map[string]string{"a": "AAA\n", "b": "BBB\n"}))
	if err != nil {
		t.Fatal(err)
	}
	if out != "AAA\nmid\nBBB\n" {
		t.Fatalf("bad multi splice:\n%q", out)
	}
}

func TestExpandIncludesMissingPartialFails(t *testing.T) {
	_, err := expandIncludes("<!-- awf:include nope -->\n", partialsFS(nil))
	// invariant: rendering/render-engine:include-missing-fails (TestExpandIncludesMissingPartialFails)
	if err == nil || !strings.Contains(err.Error(), "unknown partial") {
		t.Fatalf("expected unknown-partial error, got %v", err)
	}
}

func TestExpandIncludesNestedFails(t *testing.T) {
	_, err := expandIncludes("<!-- awf:include a -->\n",
		partialsFS(map[string]string{"a": "x\n<!-- awf:include b -->\n"}))
	// invariant: rendering/render-engine:include-no-nested (TestExpandIncludesNestedFails)
	if err == nil || !strings.Contains(err.Error(), "nested include") {
		t.Fatalf("expected nested-include error, got %v", err)
	}
}

func TestExpandIncludesSectionMarkerFails(t *testing.T) {
	_, err := expandIncludes("<!-- awf:include a -->\n",
		partialsFS(map[string]string{"a": "<!-- awf:section x -->\nbody\n<!-- awf:end -->\n"}))
	// invariant: rendering/render-engine:include-no-sections (TestExpandIncludesSectionMarkerFails)
	if err == nil || !strings.Contains(err.Error(), "section marker") {
		t.Fatalf("expected section-marker error, got %v", err)
	}
}
