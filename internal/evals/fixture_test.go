package evals

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// evalPrefix is the skill-name prefix the golden-task fixture renders under.
// Rendered skill dirs are ".claude/skills/<evalPrefix>-<name>/SKILL.md"; agents
// are unprefixed at ".claude/agents/<name>.md".
const evalPrefix = "example"

// loadCatalog loads the embedded catalog or fails the test.
func loadCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat := catalog.Standard
	return cat
}

// sortedKeys returns m's keys in deterministic order.
func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// writeList appends a "key:\n  - v\n" YAML block to b.
func writeList(b *strings.Builder, key string, vals []string) {
	b.WriteString(key + ":\n")
	for _, v := range vals {
		b.WriteString("  - " + v + "\n")
	}
}

// fullCatalogConfig builds a .awf/config.yaml enabling every catalog skill,
// agent, and doc - the deliberate inverse of the curated awf init default
// (ADR-0022) - so the rendered set exercises every workflow-chain seam. The
// enabled set is derived from the catalog (never hand-listed) so it cannot
// silently rot as the catalog grows (ADR-0053).
func fullCatalogConfigForTarget(cat *catalog.Catalog, target string) string {
	var b strings.Builder
	b.WriteString("prefix: " + evalPrefix + "\n")
	b.WriteString("targets:\n  - " + target + "\n")
	writeList(&b, "skills", sortedKeys(cat.Skills))
	writeList(&b, "agents", sortedKeys(cat.Agents))
	// Only toggleable docs go in the docs: enable array; Mandatory singletons
	// render unconditionally and must not be listed (ADR-0061).
	writeList(&b, "docs", catalog.NonMandatoryDocNames(cat))
	return b.String()
}

// syncFullCatalog scaffolds a temp project with the full-catalog config, runs a
// real Project.SyncReport, and returns the project root. It reuses the exported
// testsupport primitives rather than internal/project's package-private
// scaffold helper (ADR-0053 Decision item 5).
func syncFullCatalog(t *testing.T, cat *catalog.Catalog) string {
	return syncFullCatalogForTarget(t, cat, "claude")
}

func syncFullCatalogForTarget(t *testing.T, cat *catalog.Catalog, target string) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fullCatalogConfigForTarget(cat, target))
	p, err := project.Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, _, _, err := p.SyncReport(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	return root
}

// skillPath returns the rendered claude-target SKILL.md path for a skill name.
func skillPath(root, name string) string {
	return filepath.Join(root, ".claude", "skills", evalPrefix+"-"+name, "SKILL.md")
}

// agentPath returns the rendered claude-target agent path for an agent name.
func agentPath(root, name string) string {
	return filepath.Join(root, ".claude", "agents", name+".md")
}

// TestFullCatalogCoverage proves the full-catalog fixture actually renders an
// artifact for every catalog skill and agent, so no chain artifact is silently
// dropped (e.g. by a requiresDoc gate). This is the guard that keeps the eval
// suite exhaustive as the catalog grows.
//
// invariant: tooling/evaluations:evals-full-catalog-coverage
func TestFullCatalogCoverage(t *testing.T) {
	cat := loadCatalog(t)
	root := syncFullCatalog(t, cat)
	for _, s := range sortedKeys(cat.Skills) {
		if _, err := os.Stat(skillPath(root, s)); err != nil {
			t.Errorf("catalog skill %q not rendered: %v", s, err)
		}
	}
	for _, a := range sortedKeys(cat.Agents) {
		if _, err := os.Stat(agentPath(root, a)); err != nil {
			t.Errorf("catalog agent %q not rendered: %v", a, err)
		}
	}
}
