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

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// fullCatalogConfigForTarget builds the fixed-target, full-catalog eval config.
// The target argument remains at this test seam so callers can select which of
// the two unconditionally rendered outputs they inspect.
func fullCatalogConfigForTarget(_ *catalog.Catalog, _ string) string {
	return "prefix: " + evalPrefix + "\nintegrationBranch: main\nvars:\n  gateCmd: the project's gate\n"
}

// syncFullCatalog scaffolds the full-catalog fixture for focused Claude evals.
func syncFullCatalog(t *testing.T, cat *catalog.Catalog) string {
	return syncFullCatalogForTarget(t, cat, "claude")
}

func targetNamed(t *testing.T, targets []project.Target, name string) project.Target {
	t.Helper()
	for _, target := range targets {
		if target.Name == name {
			return target
		}
	}
	t.Fatalf("requested target %q not rendered in %#v", name, targets)
	return project.Target{}
}

func catalogDocPath(root, name string, entry catalog.DocEntry) string {
	if entry.AgentsDoc {
		return filepath.Join(root, "AGENTS.md")
	}
	path := entry.Path
	if path == "" {
		path = name + ".md"
	}
	return filepath.Join(root, "docs", filepath.FromSlash(path))
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
			if len(p.Targets) != 2 {
				t.Fatalf("targets = %d, want both built-in targets", len(p.Targets))
			}
			for _, target := range p.Targets {
				for _, s := range sortedKeys(cat.Skills) {
					path := filepath.Join(root, filepath.FromSlash(target.SkillPath(evalPrefix, s)))
					if _, err := os.Stat(path); err != nil {
						t.Errorf("%s catalog skill %q not rendered: %v", target.Name, s, err)
					}
				}
				for _, a := range sortedKeys(cat.Agents) {
					path := filepath.Join(root, filepath.FromSlash(target.AgentPath(a)))
					if _, err := os.Stat(path); err != nil {
						t.Errorf("%s catalog agent %q not rendered: %v", target.Name, a, err)
					}
				}
			}
			target := targetNamed(t, p.Targets, targetName)
			for name, entry := range cat.Docs {
				path := catalogDocPath(root, name, entry)
				if _, err := os.Stat(path); err != nil {
					t.Errorf("catalog doc %q not rendered at %s: %v", name, path, err)
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
			if drift, err := p.Check(testsupport.Context(t)); err != nil || len(drift) != 0 {
				t.Fatalf("check repaired missing output: drift=%v err=%v", drift, err)
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
