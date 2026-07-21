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
)

const testsupportImport = "github.com/hypnotox/agentic-workflows/internal/testsupport"

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
// invariant: tooling/quality-gates:testsupport-zero-internal-deps
// invariant: tooling/test-infrastructure:test-support-leaf-boundary
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
