package topic

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
)

// repoRootForOwnership walks upward from the test's working directory to the
// directory containing go.mod.
func repoRootForOwnership(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test's working directory")
		}
		dir = parent
	}
}

// productionPackageFiles enumerates every production package under internal/
// and cmd/ as one representative repo-relative Go file per package directory:
// a directory counts when it holds at least one non-test .go file, and
// internal/testsupport/testdata is excluded because its Go files are fixtures.
func productionPackageFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	for _, top := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if rel == "internal/testsupport/testdata" || strings.HasPrefix(rel, "internal/testsupport/testdata/") {
				return filepath.SkipDir
			}
			entries, err := os.ReadDir(path)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				name := entry.Name()
				if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
					continue
				}
				files = append(files, rel+"/"+name)
				break
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	slices.Sort(files)
	return files
}

// unownedPackages reports every representative file matched by no domain's
// paths selectors, collapsed to its package directory.
func unownedPackages(domainPaths map[string][]string, files []string) []string {
	var unowned []string
	for _, file := range files {
		owned := false
		for _, selectors := range domainPaths {
			if pathglob.MatchAny(selectors, file) {
				owned = true
				break
			}
		}
		if !owned {
			unowned = append(unowned, filepath.ToSlash(filepath.Dir(file)))
		}
	}
	slices.Sort(unowned)
	return slices.Compact(unowned)
}

// TestProductionPackagesAreDomainOwned proves the claim: every production
// package under internal/ and cmd/ is matched by at least one domain's paths,
// so a package omitted from domain ownership fails here rather than degrading
// silently to unowned (context coverage is advisory and exits zero over an
// unowned package).
// invariant: tooling/context-and-topic:production-packages-domain-owned (TestProductionPackagesAreDomainOwned)
func TestProductionPackagesAreDomainOwned(t *testing.T) {
	root := repoRootForOwnership(t)
	cfg, err := config.Load(filepath.Join(root, config.DirName))
	if err != nil {
		t.Fatal(err)
	}
	domainPaths := map[string][]string{}
	for _, d := range cfg.Domains {
		sc, err := cfg.Sidecar("domains", d)
		if err != nil {
			t.Fatal(err)
		}
		if len(sc.Paths) > 0 {
			domainPaths[d] = sc.Paths
		}
	}
	if len(domainPaths) == 0 {
		t.Fatal("no domain declares paths - the ownership universe is empty")
	}

	files := productionPackageFiles(t, root)
	if len(files) < 10 {
		t.Fatalf("production package enumeration found only %d packages - the walk is broken", len(files))
	}
	if unowned := unownedPackages(domainPaths, files); len(unowned) != 0 {
		t.Errorf("production packages owned by no domain (add them to a domain's paths):\n\t%s",
			strings.Join(unowned, "\n\t"))
	}

	// The detector must be able to fail: a package outside every selector is
	// reported, and one inside a selector is not.
	fabricated := unownedPackages(domainPaths, []string{
		"internal/definitely-unowned-fixture/fixture.go",
		"internal/project/project.go",
	})
	if !slices.Contains(fabricated, "internal/definitely-unowned-fixture") {
		t.Errorf("an unowned package escaped the detector: %v", fabricated)
	}
	if slices.Contains(fabricated, "internal/project") {
		t.Errorf("an owned package was reported unowned: %v", fabricated)
	}
}
