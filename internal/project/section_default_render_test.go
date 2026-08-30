package project

import (
	"strings"
	"testing"
)

// End-to-end coverage for the section-default splice. The feature's two halves
// meet only at a shared constant - token→sentinel in placeholders.go,
// sentinel→splice in internal/render - so a part file carrying
// {{=awf:sectionDefault}} is driven through the full Open→RenderAll pipeline
// here. A renderTarget call-order regression would pass both unit halves but
// fail this.
// invariant: rendering/render-engine:section-default-splice (TestSectionDefaultPartRendersEndToEnd)
func TestSectionDefaultPartRendersEndToEnd(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nvars: {}\n", map[string]string{
		"parts/working-with-awf/commands.md": "Preamble before the default.\n\n{{=awf:sectionDefault}}\n\nAppendix after the default.\n",
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderAll(p)
	if err != nil {
		t.Fatal(err)
	}
	doc := renderedByPath(t, files, "docs/working-with-awf.md")
	pre := strings.Index(doc, "Preamble before the default.")
	def := strings.Index(doc, "`awf init`")
	app := strings.Index(doc, "Appendix after the default.")
	if pre < 0 || def < 0 || app < 0 || pre > def || def > app {
		t.Fatalf("default not spliced between the part fragments: pre=%d def=%d app=%d\n%s", pre, def, app, doc)
	}
	if strings.Contains(doc, "\x00") {
		t.Fatalf("sentinel bytes leaked into rendered output:\n%s", doc)
	}
	if strings.Contains(doc, "sectionDefault") {
		t.Fatalf("placeholder token survived rendering:\n%s", doc)
	}
}

// A part re-injecting a stub section's default must fail the same full
// pipeline with the ADR-0072 hard error, not render an authoring prompt.
// invariant: rendering/render-engine:section-default-stub-error (TestSectionDefaultStubPartFailsRender)
func TestSectionDefaultStubPartFailsRender(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nvars: {}\n", map[string]string{
		"parts/agents-doc/identity.md": "{{=awf:sectionDefault}}\n",
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := renderAll(p); err == nil || !strings.Contains(err.Error(), "re-injects a stub default") {
		t.Fatalf("expected the stub re-injection hard error, got: %v", err)
	}
}
