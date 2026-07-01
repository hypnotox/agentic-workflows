package adr_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestRenderDomainIndexFiltersGroupsAndAnnotates covers membership filtering by
// domain, status grouping (incl. within-status number sort and an extra status
// appended after statusOrder), superseded-successor annotation, and relative
// links.
func TestRenderDomainIndexFiltersGroupsAndAnnotates(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"0001-first.md":  testsupport.ADR("Implemented", testsupport.WithDomains("rendering"), testsupport.WithTitle("0001: First")),
		"0002-second.md": testsupport.ADR("Implemented", testsupport.WithDomains("rendering", "config"), testsupport.WithTitle("0002: Second")),
		"0003-third.md":  testsupport.ADR("Accepted", testsupport.WithDomains("config"), testsupport.WithTitle("0003: Third")),
		"0004-fourth.md": testsupport.ADR("Superseded", testsupport.WithDomains("rendering"), testsupport.WithSupersededBy("0002"), testsupport.WithTitle("0004: Fourth")),
		"0005-fifth.md":  testsupport.ADR("Draft", testsupport.WithDomains("rendering"), testsupport.WithTitle("0005: Fifth")),
	}
	for name, content := range files {
		testsupport.WriteFile(t, filepath.Join(dir, name), content)
	}

	got, err := adr.RenderDomainIndex(dir, "rendering")
	if err != nil {
		t.Fatalf("RenderDomainIndex: %v", err)
	}

	// invariant: domain-index-matches-domains — exactly the ADRs tagged with the
	// queried domain appear; one tagged only "config" must not.
	for _, want := range []string{"ADR-0001: First", "ADR-0002: Second", "ADR-0004: Fourth", "ADR-0005: Fifth"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in index:\n%s", want, got)
		}
	}
	if strings.Contains(got, "ADR-0003: Third") {
		t.Errorf("ADR-0003 (domain config only) must not appear in the rendering index:\n%s", got)
	}

	// Status grouping headings, with an extra status (Draft) after the known ones.
	for _, h := range []string{"### Implemented", "### Superseded", "### Draft"} {
		if !strings.Contains(got, h) {
			t.Errorf("expected heading %q:\n%s", h, got)
		}
	}
	// statusOrder places Implemented before Superseded; the extra Draft comes last.
	impl, sup, draft := strings.Index(got, "### Implemented"), strings.Index(got, "### Superseded"), strings.Index(got, "### Draft")
	if impl >= sup || sup >= draft {
		t.Errorf("status order wrong: Implemented(%d) Superseded(%d) Draft(%d)\n%s", impl, sup, draft, got)
	}

	// Within Implemented, 0001 precedes 0002 (number sort).
	if strings.Index(got, "ADR-0001: First") > strings.Index(got, "ADR-0002: Second") {
		t.Errorf("0001 should sort before 0002:\n%s", got)
	}

	// Superseded entry carries its successor; links are relative to docs/domains/.
	if !strings.Contains(got, "→ superseded by ADR-0002") {
		t.Errorf("expected superseded annotation:\n%s", got)
	}
	if !strings.Contains(got, "](../decisions/0004-fourth.md)") {
		t.Errorf("expected ../decisions/ link:\n%s", got)
	}
}

// TestRenderDomainIndexPlaceholder returns a placeholder (never empty) when no ADR
// is tagged with the queried domain.
func TestRenderDomainIndexPlaceholder(t *testing.T) {
	dir := t.TempDir()
	content := testsupport.ADR("Accepted", testsupport.WithDomains("config"), testsupport.WithTitle("0001: Only Config"))
	testsupport.WriteFile(t, filepath.Join(dir, "0001-only-config.md"), content)
	got, err := adr.RenderDomainIndex(dir, "rendering")
	if err != nil {
		t.Fatalf("RenderDomainIndex: %v", err)
	}
	if !strings.Contains(got, "No decisions recorded for this domain yet") {
		t.Errorf("expected placeholder, got: %q", got)
	}
}

// TestRenderDomainIndexParseError propagates a ParseDir error rather than
// producing output.
func TestRenderDomainIndexParseError(t *testing.T) {
	dir := t.TempDir()
	// Deliberately malformed frontmatter — ADR() only emits valid YAML, so this
	// negative-test input stays a raw literal.
	content := "---\nstatus: [unterminated\n---\n# ADR-0001: Broken\n"
	testsupport.WriteFile(t, filepath.Join(dir, "0001-broken.md"), content)
	got, err := adr.RenderDomainIndex(dir, "rendering")
	if err == nil {
		t.Fatal("expected error from malformed frontmatter, got nil")
	}
	if got != "" {
		t.Errorf("expected empty output on error, got: %q", got)
	}
}

// TestParseDomainsAndSupersededBy confirms the new frontmatter fields land on the
// parsed ADR.
func TestParseDomainsAndSupersededBy(t *testing.T) {
	dir := t.TempDir()
	content := testsupport.ADR("Superseded", testsupport.WithDomains("rendering", "config"), testsupport.WithSupersededBy("0009"), testsupport.WithTitle("0007: Example"))
	testsupport.WriteFile(t, filepath.Join(dir, "0007-example.md"), content)
	adrs, err := adr.ParseDir(dir)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	if len(adrs) != 1 {
		t.Fatalf("expected 1 ADR, got %d", len(adrs))
	}
	if got := strings.Join(adrs[0].Domains, ","); got != "rendering,config" {
		t.Errorf("Domains = %q, want rendering,config", got)
	}
	if adrs[0].SupersededBy != "0009" {
		t.Errorf("SupersededBy = %q, want 0009", adrs[0].SupersededBy)
	}
}
