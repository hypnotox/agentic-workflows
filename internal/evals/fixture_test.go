package evals

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// evalProjectPrefix supplies project identity to the golden-task fixture. The
// fixed AWF skill names are independent of it.
const evalProjectPrefix = "example"

var (
	evalSeedMu sync.Mutex
	evalSeeds  = make(map[string]testsupport.TreeSeed)
)

func loadEvalSession(ctx context.Context, root string) (*project.Session, error) {
	repo, _, err := awfgit.OpenContaining(root)
	if err != nil {
		if !errors.Is(err, awfgit.ErrNotARepository) {
			return nil, err
		}
		return project.NewLoaderWithoutRepository(config.Load, catalog.Standard, awfgit.ProjectResidentRoot).Load(ctx, root)
	}
	return project.NewLoader(config.Load, catalog.Standard, awfgit.ProjectResidentRoot, repo).Load(ctx, root)
}

func syncEvalProject(t *testing.T, p *project.Session) error {
	t.Helper()
	_, err := publisher.New(p, project.Version).SyncLeased(context.Background(), nil)
	return err
}

func checkProject(p *project.Session, ctx context.Context) ([]manifest.Drift, error) {
	cfg, err := config.Load(config.RootDir(p.Root()))
	if err != nil {
		return nil, err
	}
	operation := publisher.New(p, project.Version)
	plan, err := operation.Plan()
	if err != nil {
		return nil, err
	}
	pitfalls, _ := operation.Pitfalls()
	generated, _ := operation.GeneratedOutput()
	glossary, _ := operation.Glossary()
	report, err := project.BuildCheckReport(p, cfg, nil, ctx, plan, pitfalls, generated, glossary)
	if err != nil {
		return nil, err
	}
	var drift []manifest.Drift
	for _, finding := range report.DirectResult.Findings() {
		drift = append(drift, manifest.Drift{Kind: finding.Evidence.Kind, Path: finding.Evidence.Path, Detail: finding.Evidence.Detail})
	}
	for _, information := range report.DirectResult.Information() {
		drift = append(drift, manifest.Drift{Kind: information.Evidence.Kind, Path: information.Evidence.Path, Detail: information.Evidence.Detail})
	}
	return drift, nil
}

func mustEvalConfig(t *testing.T, state *project.Session) *config.Config {
	t.Helper()
	cfg, err := config.Load(config.RootDir(state.Root()))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func read(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

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
	return "prefix: " + evalProjectPrefix + "\nintegrationBranch: main\nvars:\n  gateCmd: the project's gate\n"
}

// cloneFullCatalog gives a test an isolated copy of the full-catalog Claude seed.
func cloneFullCatalog(t *testing.T, cat *catalog.Catalog) string {
	return cloneFullCatalogForTarget(t, cat, "claude")
}

func targetNamed(t *testing.T, targets []artifactregistry.Target, name string) artifactregistry.Target {
	t.Helper()
	for _, target := range targets {
		if target.Name == name {
			return target
		}
	}
	t.Fatalf("requested target %q not rendered in %#v", name, targets)
	return artifactregistry.Target{}
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

// fullCatalogSeedForTarget constructs one immutable full-catalog seed for each
// target identity. The product renders both fixed targets, but target-specific
// evals inspect only the requested tree.
func fullCatalogSeedForTarget(t *testing.T, cat *catalog.Catalog, target string) testsupport.TreeSeed {
	t.Helper()
	evalSeedMu.Lock()
	defer evalSeedMu.Unlock()
	if seed, ok := evalSeeds[target]; ok {
		return seed
	}
	root := filepath.Join(t.TempDir(), "seed")
	testsupport.WriteAwfConfig(t, root, fullCatalogConfigForTarget(cat, target))
	p, err := loadEvalSession(testsupport.Context(t), root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, err := publisher.New(p, project.Version).Initialize(publisher.InitAuthority{InitializedWithVersion: project.Version}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	seed, err := testsupport.CaptureTree(root)
	if err != nil {
		t.Fatalf("capture %s full-catalog seed: %v", target, err)
	}
	evalSeeds[target] = seed
	return seed
}

// cloneFullCatalogForTarget gives each consumer an isolated fixture. Tests that
// mutate generated output therefore change only their explicit clone.
func cloneFullCatalogForTarget(t *testing.T, cat *catalog.Catalog, target string) string {
	t.Helper()
	seed := fullCatalogSeedForTarget(t, cat, target)
	root := filepath.Join(t.TempDir(), "project")
	if err := seed.Clone(root); err != nil {
		t.Fatalf("clone %s full-catalog seed: %v", target, err)
	}
	return root
}

// skillPath returns the rendered Claude SKILL.md path for existing focused evals.
func skillPath(root, name string) string {
	return filepath.Join(root, ".claude", "skills", name, "SKILL.md")
}

// targetSkillPath returns a rendered skill path in the selected fixed target.
func targetSkillPath(root, target, name string) string {
	return filepath.Join(root, "."+target, "skills", name, "SKILL.md")
}

// syncStandardFootprint provides the historical profile-eval call sites with
// the one standard footprint now rendered for every project.
func syncStandardFootprint(t *testing.T, _ string) string {
	t.Helper()
	return cloneFullCatalog(t, loadCatalog(t))
}

// TestFullCatalogCoverage proves the full-catalog fixture renders every AWF
// skill and document, so no catalog artifact is silently dropped. This guard
// keeps the eval suite exhaustive as the catalog grows.
//
// invariant: tooling/evaluations:evals-full-catalog-coverage (TestFullCatalogCoverage)
// invariant: tooling/test-infrastructure:immutable-fixture-seeds (TestEvalFullCatalogSeedClonesAreIsolated)
func TestEvalFullCatalogSeedClonesAreIsolated(t *testing.T) {
	cat := loadCatalog(t)
	first := cloneFullCatalogForTarget(t, cat, "claude")
	seed := fullCatalogSeedForTarget(t, cat, "claude")
	digest := seed.Digest()
	second := cloneFullCatalogForTarget(t, cat, "claude")
	if err := os.WriteFile(filepath.Join(first, "AGENTS.md"), []byte("mutated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(second, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) == "mutated\n" {
		t.Fatal("second evaluation clone changed through first")
	}
	if fullCatalogSeedForTarget(t, cat, "claude").Digest() != digest {
		t.Fatal("evaluation seed digest changed after clone mutation")
	}
}

func TestFullCatalogCoverage(t *testing.T) {
	cat := loadCatalog(t)
	for _, targetName := range []string{"claude", "pi"} {
		t.Run(targetName, func(t *testing.T) {
			root := cloneFullCatalogForTarget(t, cat, targetName)
			p, err := loadEvalSession(testsupport.Context(t), root)
			if err != nil {
				t.Fatalf("load initialized project: %v", err)
			}
			if len(p.Targets()) != 2 {
				t.Fatalf("targets = %d, want both built-in targets", len(p.Targets()))
			}
			target := targetNamed(t, p.Targets(), targetName)
			for _, s := range sortedKeys(cat.Skills) {
				path := filepath.Join(root, filepath.FromSlash(target.SkillPath(s)))
				if _, err := os.Stat(path); err != nil {
					t.Errorf("%s AWF skill %q not rendered: %v", target.Name, s, err)
				}
			}
			for name, entry := range cat.Docs {
				path := catalogDocPath(root, name, entry)
				if _, err := os.Stat(path); err != nil {
					t.Errorf("catalog doc %q not rendered at %s: %v", name, path, err)
				}
			}
			if drift, err := checkProject(p, testsupport.Context(t)); err != nil || len(drift) != 0 {
				t.Fatalf("initial check: drift=%v err=%v", drift, err)
			}

			missing := target.SkillPath(sortedKeys(cat.Skills)[0])
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(missing))); err != nil {
				t.Fatalf("remove %s: %v", missing, err)
			}
			drift, err := checkProject(p, testsupport.Context(t))
			if err != nil {
				t.Fatalf("check missing output: %v", err)
			}
			if !hasDrift(drift, missing, "missing") {
				t.Errorf("missing output drift = %v, want %q missing", drift, missing)
			}
			if err := syncEvalProject(t, p); err != nil {
				t.Fatalf("repair missing output: %v", err)
			}
			if drift, err := checkProject(p, testsupport.Context(t)); err != nil || len(drift) != 0 {
				t.Fatalf("check repaired missing output: drift=%v err=%v", drift, err)
			}

			if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("tampered\n"), 0o644); err != nil {
				t.Fatalf("tamper AGENTS.md: %v", err)
			}
			drift, err = checkProject(p, testsupport.Context(t))
			if err != nil {
				t.Fatalf("check stale output: %v", err)
			}
			if !hasDrift(drift, "AGENTS.md", "hand-edited") {
				t.Errorf("edited output drift = %v, want AGENTS.md hand-edited", drift)
			}
			if err := syncEvalProject(t, p); err != nil {
				t.Fatalf("repair stale output: %v", err)
			}
			if drift, err := checkProject(p, testsupport.Context(t)); err != nil || len(drift) != 0 {
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
