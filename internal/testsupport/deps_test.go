package testsupport_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestZeroInternalDeps enforces mechanically that internal/testsupport (and
// its gitfixture subpackage) stays a leaf package: no non-test .go file may
// import any github.com/hypnotox/agentic-workflows/internal/* package, so
// this package stays safely importable from any package's tests without
// risking an import cycle. gitfixture/ is the sole exception permitted to
// import go-git.
// invariant: testsupport-zero-internal-deps
func TestZeroInternalDeps(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	sub, err := filepath.Glob(filepath.Join("gitfixture", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, sub...)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		astFile, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		allowGoGit := strings.HasPrefix(f, "gitfixture"+string(filepath.Separator))
		for _, imp := range astFile.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("%s: unquote import %s: %v", f, imp.Path.Value, err)
			}
			if strings.HasPrefix(path, "github.com/hypnotox/agentic-workflows/internal/") {
				t.Errorf("%s imports internal package %q — internal/testsupport must stay a leaf package (ADR-0044)", f, path)
			}
			if !allowGoGit && strings.HasPrefix(path, "github.com/go-git/") {
				t.Errorf("%s imports go-git package %q — only gitfixture/ may depend on go-git", f, path)
			}
		}
	}
}
