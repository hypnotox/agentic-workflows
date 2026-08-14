package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestListDocumentRendersInventory(t *testing.T) {
	p := &Project{Cfg: &config.Config{Domains: []string{"tooling"}}, view: catalog.NewView(catalog.Standard)}
	for _, test := range []struct {
		kind, entry string
	}{{"domain", "tooling"}, {"target", "claude"}} {
		document, err := p.ListDocument(test.kind)
		if err != nil {
			t.Fatal(err)
		}
		var out strings.Builder
		if err := presentation.Render(&out, document); err != nil {
			t.Fatal(err)
		}
		if got := out.String(); !strings.Contains(got, "status: artifact inventory") || !strings.Contains(got, test.entry) {
			t.Fatalf("%s document = %q", test.kind, got)
		}
	}
}

func TestListPresentationRejectsInvalidEntries(t *testing.T) {
	if _, err := listCategory("items", []string{"bad\nitem"}); err == nil {
		t.Fatal("invalid list entry accepted")
	}
	p := &Project{Cfg: &config.Config{}, view: catalog.NewView(catalog.Standard)}
	if _, err := p.ListDocument("bogus"); err == nil {
		t.Fatal("unknown inventory kind accepted")
	}
}
