package project

import (
	"strings"
	"testing"
)

// End-to-end coverage for the ADR-0072 splice. The feature's two halves meet
// only at a shared constant — token→sentinel in placeholders.go, sentinel→splice
// in internal/render — so a part file carrying {{=awf:sectionDefault}} is driven
// through the full Open→RenderAll pipeline here: a renderTarget call-order
// regression (substitution after Assemble, or the stub check hoisted before
// planSections) would pass both unit halves but fail this.
// invariant: section-default-splice
func TestSectionDefaultPartRendersEndToEnd(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nvars: {}\nskills: []\nagents: []\n", map[string]string{
		"parts/adr-readme/naming.md": "Preamble before the default.\n\n{{=awf:sectionDefault}}\n\nAppendix after the default.\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	readme := renderedByPath(t, files, "docs/decisions/README.md")
	pre := strings.Index(readme, "Preamble before the default.")
	def := strings.Index(readme, "NNNN-kebab-title.md") // stable phrase from the naming default
	app := strings.Index(readme, "Appendix after the default.")
	if pre < 0 || def < 0 || app < 0 || pre > def || def > app {
		t.Fatalf("default not spliced between the part fragments: pre=%d def=%d app=%d\n%s", pre, def, app, readme)
	}
	if strings.Contains(readme, "\x00") {
		t.Fatalf("sentinel bytes leaked into rendered output:\n%s", readme)
	}
	if strings.Contains(readme, "sectionDefault") {
		t.Fatalf("placeholder token survived rendering:\n%s", readme)
	}
}

// A part re-injecting a stub section's default must fail the same full
// pipeline with the ADR-0072 hard error, not render an authoring prompt.
// invariant: section-default-stub-error
func TestSectionDefaultStubPartFailsRender(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nvars: {}\nskills: []\nagents: []\n", map[string]string{
		"parts/agents-doc/identity.md": "{{=awf:sectionDefault}}\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.RenderAll(); err == nil || !strings.Contains(err.Error(), "re-injects a stub default") {
		t.Fatalf("expected the stub re-injection hard error, got: %v", err)
	}
}
