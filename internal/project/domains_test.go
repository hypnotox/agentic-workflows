package project

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/templates"
)

const domainCfg = "prefix: example\nskills: []\nagents: []\ndomains: [rendering]\n"

func writeADR(t *testing.T, root, name, body string) {
	t.Helper()
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", name), body)
}

func readDomainDoc(t *testing.T, root, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "docs", "domains", name+".md"))
	if err != nil {
		t.Fatalf("read domain doc %s: %v", name, err)
	}
	return string(b)
}

func TestDomainDocRendersIndexAndNarrative(t *testing.T) {
	root := scaffoldFiles(t, domainCfg, map[string]string{
		"domains/parts/rendering/current-state.md": "## Current state\n\nThe render engine is stable.\n",
	})
	writeADR(t, root, "0001-engine.md", testsupport.ADR("Implemented", testsupport.WithDomains("rendering"), testsupport.WithTitle("0001: Engine")))
	writeADR(t, root, "0002-layout.md", testsupport.ADR("Accepted", testsupport.WithDomains("rendering"), testsupport.WithTitle("0002: Layout")))
	writeADR(t, root, "0003-config.md", testsupport.ADR("Accepted", testsupport.WithDomains("config"), testsupport.WithTitle("0003: Config")))

	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	out := readDomainDoc(t, root, "rendering")
	if strings.Contains(out, "<no value>") {
		t.Errorf("domain doc leaked <no value>:\n%s", out)
	}
	if !strings.Contains(out, "The render engine is stable.") {
		t.Errorf("expected the current-state convention part:\n%s", out)
	}
	if !strings.Contains(out, "## Decisions") {
		t.Errorf("expected the forced Decisions heading:\n%s", out)
	}
	for _, want := range []string{"ADR-0001: Engine", "ADR-0002: Layout"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in the index:\n%s", want, out)
		}
	}
	if strings.Contains(out, "ADR-0003: Config") {
		t.Errorf("ADR-0003 (config domain) must not appear in the rendering doc:\n%s", out)
	}
}

// invariant: domain-doc-regenerated
func TestDomainDocStaleOnAdrRetag(t *testing.T) {
	root := scaffoldFiles(t, domainCfg, nil)
	writeADR(t, root, "0001-engine.md", testsupport.ADR("Implemented", testsupport.WithDomains("rendering"), testsupport.WithTitle("0001: Engine"), testsupport.WithBody("## Decision\n\n1. x.\n")))
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if drift, _ := p.Check(); len(drift) != 0 {
		t.Fatalf("expected clean after sync, got: %#v", drift)
	}
	// Retag a NEW ADR into the rendering domain without re-syncing.
	writeADR(t, root, "0002-new.md", testsupport.ADR("Accepted", testsupport.WithDomains("rendering"), testsupport.WithTitle("0002: New"), testsupport.WithBody("## Decision\n\n1. x.\n")))
	drift, err := p.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !hasDrift(drift, "docs/domains/rendering.md", "stale") {
		t.Errorf("expected rendering.md stale after ADR retag, got: %#v", drift)
	}
}

func TestDomainDocMissingWhenDeleted(t *testing.T) {
	root := scaffoldFiles(t, domainCfg, nil)
	writeADR(t, root, "0001-engine.md", testsupport.ADR("Implemented", testsupport.WithDomains("rendering"), testsupport.WithTitle("0001: Engine")))
	p, _ := Open(root)
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "docs", "domains", "rendering.md")); err != nil {
		t.Fatal(err)
	}
	drift, _ := p.Check()
	if !hasDrift(drift, "docs/domains/rendering.md", "missing") {
		t.Errorf("expected rendering.md missing after delete, got: %#v", drift)
	}
}

func TestDomainDocOrphanedWhenDomainRemoved(t *testing.T) {
	root := scaffoldFiles(t, domainCfg, nil)
	writeADR(t, root, "0001-engine.md", testsupport.ADR("Implemented", testsupport.WithDomains("rendering"), testsupport.WithTitle("0001: Engine")))
	p, _ := Open(root)
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	// Drop the domain from config; the lock still carries the rendered doc.
	if err := os.WriteFile(configPath(root), []byte("prefix: example\nskills: []\nagents: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p2, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	drift, _ := p2.Check()
	if !hasDrift(drift, "docs/domains/rendering.md", "orphaned") {
		t.Errorf("expected rendering.md orphaned after domain removal, got: %#v", drift)
	}
}

// TestGenerateDomainDocsPropagatesIndexError exercises generateDomainDocs's
// RenderDomainIndex error arm directly. Through Sync/Check this arm is unreachable
// (generateActiveMD parses the same decisions dir and fails first, covered by
// TestSyncFailsOnMalformedADR), so the method is called directly here.
func TestGenerateDomainDocsPropagatesIndexError(t *testing.T) {
	root := scaffoldFiles(t, domainCfg, nil)
	writeADR(t, root, "0001-broken.md", "---\nstatus: [unterminated\n---\n# ADR-0001: Broken\n")
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := p.generateDomainDocs(); err == nil {
		t.Error("expected generateDomainDocs to propagate the RenderDomainIndex parse error")
	}
}

func TestDomainPartOrphan(t *testing.T) {
	root := scaffoldFiles(t, domainCfg, map[string]string{
		// A part for the forced-body index section - deliberately not declared.
		"domains/parts/rendering/decisions.md": "shadow\n",
		// A part for an undeclared section.
		"domains/parts/rendering/bogus.md": "nope\n",
		// A part dir for a domain not in the enable list.
		"domains/parts/other/current-state.md": "stray\n",
	})
	p, _ := Open(root)
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	for _, want := range []string{
		filepath.Join(".awf", "domains", "parts", "rendering", "decisions.md"),
		filepath.Join(".awf", "domains", "parts", "rendering", "bogus.md"),
		filepath.Join(".awf", "domains", "parts", "other"),
	} {
		if !hasDrift(drift, want, "orphaned") {
			t.Errorf("expected orphaned drift for %s, got: %#v", want, drift)
		}
	}
}

// invariant: docs-section-parity (domain template)
func TestDomainDocSectionParity(t *testing.T) {
	cat := catalog.Standard
	src, err := fs.ReadFile(templates.FS, "domains/domain.md.tmpl")
	if err != nil {
		t.Fatalf("read domain template: %v", err)
	}
	var markers []string
	for _, s := range render.ParseSections(string(src)) {
		if s.IsSection {
			markers = append(markers, s.Name)
		}
	}
	if got := strings.Join(markers, ","); got != strings.Join(cat.DomainDoc.Sections, ",") {
		t.Errorf("domain template markers %v != catalog domainDoc sections %v", markers, cat.DomainDoc.Sections)
	}
	if got := fmt.Sprint(markers); got != "[current-state]" {
		t.Errorf("expected exactly [current-state] (Decisions is forced body, not a section), got %s", got)
	}
}

func hasDrift(drift []manifest.Drift, path, kind string) bool {
	for _, d := range drift {
		if d.Path == path && d.Kind == kind {
			return true
		}
	}
	return false
}
