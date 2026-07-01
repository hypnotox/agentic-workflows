package project

import (
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/templates"
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

// invariant: mandatory-docs-not-in-docs-catalog
func TestCatalogDocsExcludeSingletonKinds(t *testing.T) {
	cat, err := catalog.Load(templates.FS)
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}
	for name := range cat.Docs {
		if slices.Contains(catalog.SingletonKinds, name) {
			t.Errorf("cat.Docs contains %q, which is a singleton kind and must not be a toggleable doc", name)
		}
	}
}
