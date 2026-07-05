package project

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

// invariant: singleton-kind-single-source
func TestPlainSingletonsMatchCatalogSingletonKinds(t *testing.T) {
	var got []string
	for _, sg := range plainSingletons {
		got = append(got, sg.kind)
	}
	slices.Sort(got)

	var want []string
	for _, k := range catalog.SingletonKinds {
		if k == "agents-doc" {
			continue
		}
		want = append(want, k)
	}
	slices.Sort(want)

	if !slices.Equal(got, want) {
		t.Errorf("plainSingletons kinds = %v, want catalog.SingletonKinds minus agents-doc = %v", got, want)
	}
}

// invariant: singleton-kind-single-source
func TestCatalogSingletonsMatchSingletonKinds(t *testing.T) {
	cat := catalog.Standard
	got := slices.Sorted(maps.Keys(cat.Singletons))
	want := slices.Sorted(slices.Values(catalog.SingletonKinds))
	if !slices.Equal(got, want) {
		t.Errorf("cat.Singletons keys = %v, want catalog.SingletonKinds = %v (agents-doc included in both)", got, want)
	}
}

// invariant: mandatory-docs-not-in-docs-catalog
func TestCatalogDocsExcludeSingletonKinds(t *testing.T) {
	cat := catalog.Standard
	for name := range cat.Docs {
		if slices.Contains(catalog.SingletonKinds, name) {
			t.Errorf("cat.Docs contains %q, which is a singleton kind and must not be a toggleable doc", name)
		}
	}
}

// agentsDocContent renders the tree and returns AGENTS.md's content.
func agentsDocContent(t *testing.T, configYAML string) string {
	t.Helper()
	p, err := Open(scaffold(t, configYAML))
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Path == "AGENTS.md" {
			return f.Content
		}
	}
	t.Fatal("AGENTS.md not rendered")
	return ""
}

// With no commands data and no command vars, the Commands section renders a
// self-describing placeholder; identical command values render once.
func TestAgentsDocCommandsPlaceholderAndDedupe(t *testing.T) {
	empty := agentsDocContent(t, "prefix: example\nvars: {}\nskills: []\nagents: []\n")
	if !strings.Contains(empty, "<!-- No commands configured") {
		t.Errorf("empty Commands section missing the placeholder:\n%s", empty)
	}
	dup := agentsDocContent(t, "prefix: example\nvars:\n  testCmd: make test\n  gateCmd: make test\n  checkCmd: make check\nskills: []\nagents: []\n")
	if got := strings.Count(dup, "- `make test` — run the test suite"); got != 1 {
		t.Errorf("testCmd line rendered %d times, want 1:\n%s", got, dup)
	}
	if strings.Contains(dup, "— run the gate before committing") {
		t.Errorf("gateCmd identical to testCmd must not render its own Commands line:\n%s", dup)
	}
	if !strings.Contains(dup, "`make check` — check rendered files for drift") {
		t.Errorf("distinct checkCmd line missing:\n%s", dup)
	}
}
