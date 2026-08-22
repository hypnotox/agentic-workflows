package testsupport_test

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

const testsupportImport = "github.com/hypnotox/agentic-workflows/internal/testsupport"

func productionTestsupportImportViolations(path string, source any) ([]string, error) {
	slashPath := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "./")
	parts := strings.Split(slashPath, "/")
	for i, part := range parts[:len(parts)-1] {
		if part == ".git" || part == "testdata" || part == "vendor" || part == "node_modules" {
			return nil, nil
		}
		if part == ".awf" && i+1 < len(parts) {
			for _, root := range resident.RootNames() {
				if parts[i+1] == root {
					return nil, nil
				}
			}
		}
	}
	if strings.HasSuffix(slashPath, "_test.go") ||
		slashPath == "internal/testsupport" || strings.HasPrefix(slashPath, "internal/testsupport/") {
		return nil, nil
	}

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, path, source, parser.ImportsOnly)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	violations := []string{}
	for _, imp := range astFile.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("%s: unquote import %s: %w", path, imp.Path.Value, err)
		}
		if p == testsupportImport || strings.HasPrefix(p, testsupportImport+"/") {
			violations = append(violations, fmt.Sprintf("%s imports test support %q", slashPath, p))
		}
	}
	return violations, nil
}

const repositoryModule = "github.com/hypnotox/agentic-workflows"

var foundationalMechanismRoots = []string{
	"internal/filepublication",
	"internal/filesystem",
	"internal/git",
	"internal/render",
	"internal/snapshot",
	"internal/projectstate",
}

var higherLayerImports = []string{
	repositoryModule + "/internal/adr",
	repositoryModule + "/internal/currentstate",
	repositoryModule + "/internal/plan",
	repositoryModule + "/internal/project",
	repositoryModule + "/internal/publisher",
	repositoryModule + "/internal/topic",
}

func importsPackage(path, target string) bool {
	return path == target || strings.HasPrefix(path, target+"/")
}

func repositoryLayerImportViolations(path string, source any) ([]string, error) {
	slashPath := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "./")
	dir := filepath.ToSlash(filepath.Dir(slashPath))
	mechanism := false
	for _, root := range foundationalMechanismRoots {
		if dir == root || strings.HasPrefix(dir, root+"/") {
			mechanism = true
			break
		}
	}
	project := dir == "internal/project" || strings.HasPrefix(dir, "internal/project/")
	if !mechanism && !project {
		return nil, nil
	}

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, path, source, parser.ImportsOnly)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	violations := []string{}
	for _, imp := range astFile.Imports {
		importPath, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("%s: unquote import %s: %w", path, imp.Path.Value, err)
		}
		if project && importsPackage(importPath, repositoryModule+"/internal/contextq") {
			violations = append(violations, fmt.Sprintf("%s reverses the existing contextq-to-project dependency with import %q", slashPath, importPath))
		}
		if !mechanism {
			continue
		}
		for _, forbidden := range higherLayerImports {
			if importsPackage(importPath, forbidden) {
				violations = append(violations, fmt.Sprintf("%s imports higher-layer package %q", slashPath, importPath))
				break
			}
		}
	}
	return violations, nil
}

func dependencyViolations(path string, source any) ([]string, error) {
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, path, source, parser.ImportsOnly)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	allowGoGit := strings.HasPrefix(filepath.ToSlash(path), "gitfixture/")
	violations := []string{}
	for _, imp := range astFile.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("%s: unquote import %s: %w", path, imp.Path.Value, err)
		}
		ownSubpackage := p == testsupportImport || strings.HasPrefix(p, testsupportImport+"/")
		standardLibrary := !strings.Contains(strings.Split(p, "/")[0], ".")
		goGitException := allowGoGit && strings.HasPrefix(p, "github.com/go-git/go-git/")
		if !standardLibrary && !ownSubpackage && !goGitException {
			violations = append(violations, fmt.Sprintf("%s imports third-party or repository package %q", path, p))
		}
	}
	return violations, nil
}

// TestZeroInternalDeps enforces mechanically that internal/testsupport and its
// subpackages depend only on the standard library and their own subpackages.
// gitfixture alone may import the scoped go-git module needed by Git fixtures.
// invariant: tooling/quality-gates:testsupport-zero-internal-deps (TestZeroInternalDeps)
// invariant: tooling/test-infrastructure:test-support-leaf-boundary (TestZeroInternalDeps)
func TestZeroInternalDeps(t *testing.T) {
	seen := 0
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == "testdata" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		seen++
		violations, err := dependencyViolations(path, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, violation := range violations {
			t.Error(violation)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen < 2 {
		t.Fatalf("inspected only %d non-test source file(s); expected at least testsupport.go and gitfixture/gitfixture.go - did they move?", seen)
	}
}

// invariant: tooling/test-infrastructure:production-never-imports-test-support (TestProductionNeverImportsTestSupport)
func TestProductionNeverImportsTestSupport(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		source     string
		violations int
		wantErr    bool
	}{
		{name: "root testsupport", path: "cmd/tool/main.go", source: "package main\nimport \"github.com/hypnotox/agentic-workflows/internal/testsupport\"", violations: 1},
		{name: "testsupport subpackage", path: "internal/tool/tool.go", source: "package tool\nimport \"github.com/hypnotox/agentic-workflows/internal/testsupport/fsfixture\"", violations: 1},
		{name: "nested fixture", path: "fixtures/nested-adopter/internal/tool/tool.go", source: "package tool\nimport \"github.com/hypnotox/agentic-workflows/internal/testsupport\"", violations: 1},
		{name: "primary archive resident", path: ".awf/effort-archive/id-slug/adversarial.go", source: "not go"},
		{name: "nested archive resident", path: "fixtures/nested-adopter/.awf/effort-archive/id-slug/adversarial.go", source: "not go"},
		{name: "standard library", path: "internal/tool/tool.go", source: "package tool\nimport \"strings\""},
		{name: "other repository package", path: "internal/tool/tool.go", source: "package tool\nimport \"github.com/hypnotox/agentic-workflows/internal/config\""},
		{name: "test file", path: "internal/tool/tool_test.go", source: "package tool\nimport \"github.com/hypnotox/agentic-workflows/internal/testsupport\""},
		{name: "testdata fixture", path: "internal/tool/testdata/fixture.go", source: "package fixture\nimport \"github.com/hypnotox/agentic-workflows/internal/testsupport\""},
		{name: "parse error", path: "internal/tool/tool.go", source: "not go", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := productionTestsupportImportViolations(tt.path, tt.source)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(got) != tt.violations {
				t.Fatalf("violations = %v, want %d", got, tt.violations)
			}
		})
	}

	root := testsupport.RepoRoot(t)
	testsupport.WalkRepoSources(t, root, func(path string, source []byte) {
		violations, err := productionTestsupportImportViolations(path, source)
		if err != nil {
			t.Fatal(err)
		}
		for _, violation := range violations {
			t.Error(violation)
		}
	})
}

// TestRepositoryLayerDirection protects the cheap import edges named by the broader direction invariant.
func TestRepositoryLayerDirection(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		source     string
		violations int
		wantErr    bool
	}{
		{name: "mechanism to project", path: "internal/render/render.go", source: "package render\nimport \"github.com/hypnotox/agentic-workflows/internal/project\"", violations: 1},
		{name: "mechanism to domain", path: "internal/filepublication/publication.go", source: "package filepublication\nimport \"github.com/hypnotox/agentic-workflows/internal/adr\"", violations: 1},
		{name: "snapshot to git", path: "internal/snapshot/tree.go", source: "package snapshot\nimport \"github.com/hypnotox/agentic-workflows/internal/git\""},
		{name: "filesystem to publication", path: "internal/filesystem/handle.go", source: "package filesystem\nimport \"github.com/hypnotox/agentic-workflows/internal/filepublication\""},
		{name: "project to context query", path: "internal/project/project.go", source: "package project\nimport \"github.com/hypnotox/agentic-workflows/internal/contextq\"", violations: 1},
		{name: "lower state to project", path: "internal/projectstate/state.go", source: "package projectstate\nimport \"github.com/hypnotox/agentic-workflows/internal/project\"", violations: 1},
		{name: "lower state to publisher", path: "internal/projectstate/state.go", source: "package projectstate\nimport \"github.com/hypnotox/agentic-workflows/internal/publisher\"", violations: 1},
		{name: "unrelated context query consumer", path: "internal/tool/tool.go", source: "package tool\nimport \"github.com/hypnotox/agentic-workflows/internal/contextq\""},
		{name: "malformed protected source", path: "internal/git/repo.go", source: "not go", wantErr: true},
		{name: "malformed unrelated source", path: "internal/tool/tool.go", source: "not go"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repositoryLayerImportViolations(tt.path, tt.source)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(got) != tt.violations {
				t.Fatalf("violations = %v, want %d", got, tt.violations)
			}
		})
	}

	root := testsupport.RepoRoot(t)
	testsupport.WalkRepoSources(t, root, func(path string, source []byte) {
		violations, err := repositoryLayerImportViolations(path, source)
		if err != nil {
			t.Fatal(err)
		}
		for _, violation := range violations {
			t.Error(violation)
		}
	})
}

// TestCurrentStateCoordinatorDirection keeps application coordination above its
// domain, state, snapshot, Git, publication, and aggregation consumers.
func TestCurrentStateCoordinatorDirection(t *testing.T) {
	root := testsupport.RepoRoot(t)
	lowerOwners := map[string]bool{
		"internal/adr": true, "internal/currentstate": true, "internal/git": true,
		"internal/projectstate": true, "internal/publisher": true,
		"internal/repositorycheck": true, "internal/snapshot": true,
	}
	testsupport.WalkRepoSources(t, root, func(relative string, content []byte) {
		directory := filepath.ToSlash(filepath.Dir(relative))
		if lowerOwners[directory] {
			assertImportsExclude(t, relative, content, "internal/currentstatecoord")
		}
		if directory == "internal/currentstatecoord" {
			assertImportsExclude(t, relative, content, "cmd/awf")
			assertImportsExclude(t, relative, content, "internal/contextq")
		}
	})
}

func TestDependencyProofRejectsThirdPartyImportFixture(t *testing.T) {
	violations, err := dependencyViolations("testdata/thirdparty.go", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 1 || !strings.Contains(violations[0], "example.com/thirdparty") {
		t.Fatalf("violations = %v", violations)
	}
	allowed := `package fixture
import (
 "testing"
 "github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)`
	if got, err := dependencyViolations("nested/allowed.go", allowed); err != nil || len(got) != 0 {
		t.Fatalf("own subpackage violations=%v err=%v", got, err)
	}
	goGit := `package fixture
import "github.com/go-git/go-git/v5"
`
	if got, err := dependencyViolations("gitfixture/allowed.go", goGit); err != nil || len(got) != 0 {
		t.Fatalf("gitfixture violations=%v err=%v", got, err)
	}
	if got, err := dependencyViolations("other/not-allowed.go", goGit); err != nil || len(got) != 1 {
		t.Fatalf("unscoped go-git violations=%v err=%v", got, err)
	}
	if _, err := dependencyViolations("bad.go", "not go"); err == nil {
		t.Fatal("malformed fixture parsed")
	}
}
