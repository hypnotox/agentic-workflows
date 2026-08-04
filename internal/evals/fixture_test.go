package evals

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
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
	b.WriteString("integrationBranch: main\n")
	b.WriteString("targets:\n  - " + target + "\n")
	writeList(&b, "skills", sortedKeys(cat.Skills))
	writeList(&b, "agents", sortedKeys(cat.Agents))
	// Only toggleable docs go in the docs: enable array; Mandatory singletons
	// render unconditionally and must not be listed (ADR-0061).
	writeList(&b, "docs", catalog.NonMandatoryDocNames(cat))
	return b.String()
}

// syncFullCatalog scaffolds the Claude full-catalog fixture for focused evals.
func syncFullCatalog(t *testing.T, cat *catalog.Catalog) string {
	return syncFullCatalogForTarget(t, cat, "claude")
}

// syncFullCatalogForTarget scaffolds a temp project with the full-catalog
// config and initializes it. It reuses the exported testsupport primitives
// rather than internal/project's package-private scaffold helper (ADR-0053
// Decision item 5).
func syncFullCatalogForTarget(t *testing.T, cat *catalog.Catalog, target string) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fullCatalogConfigForTarget(cat, target))
	p, err := project.Open(testsupport.Context(t), root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, _, _, err := p.InitializeReport(testsupport.Context(t), project.InitAuthority{InitializedWithVersion: project.Version}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	return root
}

// skillPath returns the rendered Claude SKILL.md path for existing focused evals.
func skillPath(root, name string) string {
	return filepath.Join(root, ".claude", "skills", evalPrefix+"-"+name, "SKILL.md")
}

// agentPath returns the rendered Claude agent path for existing focused evals.
func agentPath(root, name string) string {
	return filepath.Join(root, ".claude", "agents", name+".md")
}

// TestFullCatalogCoverage proves the full-catalog fixture actually renders an
// artifact for every catalog skill and agent, so no chain artifact is silently
// dropped (e.g. by a requiresDoc gate). This is the guard that keeps the eval
// suite exhaustive as the catalog grows.
//
// invariant: tooling/evaluations:evals-full-catalog-coverage (TestFullCatalogCoverage)
func TestFullCatalogCoverage(t *testing.T) {
	cat := loadCatalog(t)
	for _, targetName := range []string{"claude", "pi"} {
		t.Run(targetName, func(t *testing.T) {
			root := syncFullCatalogForTarget(t, cat, targetName)
			p, err := project.Open(testsupport.Context(t), root)
			if err != nil {
				t.Fatalf("open initialized project: %v", err)
			}
			if len(p.Targets) != 1 {
				t.Fatalf("targets = %d, want one", len(p.Targets))
			}
			target := p.Targets[0]
			for _, s := range sortedKeys(cat.Skills) {
				path := filepath.Join(root, filepath.FromSlash(target.SkillPath(evalPrefix, s)))
				if _, err := os.Stat(path); err != nil {
					t.Errorf("catalog skill %q not rendered: %v", s, err)
				}
			}
			for _, a := range sortedKeys(cat.Agents) {
				path := filepath.Join(root, filepath.FromSlash(target.AgentPath(a)))
				if _, err := os.Stat(path); err != nil {
					t.Errorf("catalog agent %q not rendered: %v", a, err)
				}
			}
			if drift, err := p.Check(testsupport.Context(t)); err != nil || len(drift) != 0 {
				t.Fatalf("initial check: drift=%v err=%v", drift, err)
			}

			missing := target.SkillPath(evalPrefix, sortedKeys(cat.Skills)[0])
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(missing))); err != nil {
				t.Fatalf("remove %s: %v", missing, err)
			}
			drift, err := p.Check(testsupport.Context(t))
			if err != nil {
				t.Fatalf("check missing output: %v", err)
			}
			if !hasDrift(drift, missing, "missing") {
				t.Errorf("missing output drift = %v, want %q missing", drift, missing)
			}
			if _, _, _, err := p.SyncReport(testsupport.Context(t)); err != nil {
				t.Fatalf("repair missing output: %v", err)
			}

			if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("tampered\n"), 0o644); err != nil {
				t.Fatalf("tamper AGENTS.md: %v", err)
			}
			drift, err = p.Check(testsupport.Context(t))
			if err != nil {
				t.Fatalf("check stale output: %v", err)
			}
			if !hasDrift(drift, "AGENTS.md", "hand-edited") {
				t.Errorf("edited output drift = %v, want AGENTS.md hand-edited", drift)
			}
			if _, _, _, err := p.SyncReport(testsupport.Context(t)); err != nil {
				t.Fatalf("repair stale output: %v", err)
			}
			if drift, err := p.Check(testsupport.Context(t)); err != nil || len(drift) != 0 {
				t.Fatalf("final check: drift=%v err=%v", drift, err)
			}

			if err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(filepath.Dir(target.SkillDir))), func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if strings.Contains(string(raw), "<no value>") {
					t.Errorf("%s contains unresolved-value token", path)
				}
				return nil
			}); err != nil {
				t.Fatalf("walk target tree: %v", err)
			}
		})
	}
}

func hasDrift(drift []manifest.Drift, path, kind string) bool {
	for _, item := range drift {
		if item.Path == path && item.Kind == kind {
			return true
		}
	}
	return false
}
