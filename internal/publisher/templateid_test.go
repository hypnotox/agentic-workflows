package publisher

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/templates"
)

// TestLiveTemplateIDsResolve derives the complete live identity population from
// its existing owners and verifies every live entry resolves in the embedded FS.
func TestLiveTemplateIDsResolve(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	inputs := renderInputsForTest(p)
	ids := liveTemplateIDs(inputs)
	kinds := allKindDescriptors()
	for _, descriptor := range kinds {
		if descriptor.freeformDomain && !ids[descriptor.templateID(catalog.Standard, "")] {
			t.Errorf("kind-derived domain template %q is not live", descriptor.templateID(catalog.Standard, ""))
		}
	}
	for tid := range ids {
		if _, err := fs.ReadFile(templates.FS, tid); err != nil {
			t.Errorf("live template %q does not resolve: %v", tid, err)
		}
	}

	domainIndex := -1
	for i := range kinds {
		if kinds[i].freeformDomain {
			domainIndex = i
			break
		}
	}
	if domainIndex < 0 {
		t.Fatal("no freeform domain descriptor")
	}
	kinds[domainIndex].templateID = func(*catalog.Catalog, string) string { return "missing/domain.tmpl" }
	domainIDs := liveMarkdownTemplateIDsWithKinds(inputs, kinds)
	if _, ok := domainIDs["missing/domain.tmpl"]; !ok {
		t.Error("a missing kind-derived domain identity escaped the live population")
	}
	if _, err := fs.ReadFile(templates.FS, "missing/domain.tmpl"); err == nil {
		t.Error("missing kind-derived domain fixture unexpectedly resolves")
	}

	base := p
	selected := base.Catalog()
	missing := selected.Docs["architecture"]
	missing.TID = "missing/live-template.tmpl"
	selected.Docs["missing-live-fixture"] = missing
	lower := deriveSession(base, testConfig(p), NewFilesystemReader(p.Root()), selected, base.Targets())
	if _, err := New(lower, project.Version).Plan(); err == nil || !strings.Contains(err.Error(), "missing/live-template.tmpl") {
		t.Fatalf("missing live template error = %v", err)
	}
}

// slicesContainsSubstring reports whether any finding contains the substring.
func slicesContainsSubstring(findings []string, substring string) bool {
	for _, f := range findings {
		if strings.Contains(f, substring) {
			return true
		}
	}
	return false
}
