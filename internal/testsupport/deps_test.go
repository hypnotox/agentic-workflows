package testsupport_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestZeroInternalDeps enforces mechanically that internal/testsupport (and
// every subpackage under it, including gitfixture) stays a leaf package: no
// non-test .go file may import any
// github.com/hypnotox/agentic-workflows/internal/* package, so this package
// stays safely importable from any package's tests without risking an import
// cycle. gitfixture/ is the sole exception permitted to import go-git. The
// walk recurses the whole tree so a future deeper subpackage cannot escape the
// check, and the seen-count guard fails the test rather than passing vacuously
// if the source files are ever renamed or relocated out from under it.
// invariant: tooling/quality-gates:testsupport-zero-internal-deps
// invariant: tooling/test-infrastructure:test-support-leaf-boundary
func TestZeroInternalDeps(t *testing.T) {
	seen := 0
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		seen++
		fset := token.NewFileSet()
		astFile, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			t.Fatalf("parse %s: %v", path, perr)
		}
		allowGoGit := strings.HasPrefix(path, "gitfixture"+string(filepath.Separator))
		for _, imp := range astFile.Imports {
			p, uerr := strconv.Unquote(imp.Path.Value)
			if uerr != nil {
				t.Fatalf("%s: unquote import %s: %v", path, imp.Path.Value, uerr)
			}
			if strings.HasPrefix(p, "github.com/hypnotox/agentic-workflows/internal/") {
				t.Errorf("%s imports internal package %q - internal/testsupport must stay a leaf package (ADR-0044)", path, p)
			}
			if !allowGoGit && strings.HasPrefix(p, "github.com/go-git/") {
				t.Errorf("%s imports go-git package %q - only gitfixture/ may depend on go-git", path, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Guard against a vacuous pass: testsupport.go and gitfixture/gitfixture.go
	// must both be present, so the check can never silently inspect nothing.
	if seen < 2 {
		t.Fatalf("inspected only %d non-test source file(s); expected at least testsupport.go and gitfixture/gitfixture.go - did they move?", seen)
	}
}
