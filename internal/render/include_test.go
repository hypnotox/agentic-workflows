package render

import (
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

func TestExpandIncludesSplices(t *testing.T) {
	src := "intro\n\n<!-- awf:include spine -->\n\ntail\n"
	out, err := ExpandIncludes(src, partialsFS(map[string]string{"spine": "BODY\n"}))
	if err != nil {
		t.Fatal(err)
	}
	// invariant: include-splice
	if out != "intro\n\nBODY\n\ntail\n" {
		t.Fatalf("bad splice:\n%q", out)
	}
}

func TestExpandIncludesNoDirectivePassesThrough(t *testing.T) {
	src := "no directives here\n"
	out, err := ExpandIncludes(src, partialsFS(nil))
	if err != nil {
		t.Fatal(err)
	}
	if out != src {
		t.Fatalf("expected passthrough, got %q", out)
	}
}

func TestExpandIncludesMultiple(t *testing.T) {
	src := "<!-- awf:include a -->\nmid\n<!-- awf:include b -->\n"
	out, err := ExpandIncludes(src, partialsFS(map[string]string{"a": "AAA\n", "b": "BBB\n"}))
	if err != nil {
		t.Fatal(err)
	}
	if out != "AAA\nmid\nBBB\n" {
		t.Fatalf("bad multi splice:\n%q", out)
	}
}

func TestExpandIncludesMissingPartialFails(t *testing.T) {
	_, err := ExpandIncludes("<!-- awf:include nope -->\n", partialsFS(nil))
	// invariant: include-missing-fails
	if err == nil || !strings.Contains(err.Error(), "unknown partial") {
		t.Fatalf("expected unknown-partial error, got %v", err)
	}
}

func TestExpandIncludesNestedFails(t *testing.T) {
	_, err := ExpandIncludes("<!-- awf:include a -->\n",
		partialsFS(map[string]string{"a": "x\n<!-- awf:include b -->\n"}))
	// invariant: include-no-nested
	if err == nil || !strings.Contains(err.Error(), "nested include") {
		t.Fatalf("expected nested-include error, got %v", err)
	}
}

func TestExpandIncludesSectionMarkerFails(t *testing.T) {
	_, err := ExpandIncludes("<!-- awf:include a -->\n",
		partialsFS(map[string]string{"a": "<!-- awf:section x -->\nbody\n<!-- awf:end -->\n"}))
	// invariant: include-no-sections
	if err == nil || !strings.Contains(err.Error(), "section marker") {
		t.Fatalf("expected section-marker error, got %v", err)
	}
}
