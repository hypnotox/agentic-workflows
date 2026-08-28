package testsupport_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

type parsedTestFile struct {
	path string
	file *ast.File
	fset *token.FileSet
}

func exportedTestName(name string) bool {
	if !strings.HasPrefix(name, "Test") || !token.IsExported(name) {
		return false
	}
	suffix := strings.TrimPrefix(name, "Test")
	if suffix == "" {
		return true
	}
	first, _ := utf8.DecodeRuneInString(suffix)
	return !unicode.IsLower(first)
}

func directExportedTestCalls(sources map[string][]byte) ([]string, error) {
	var files []parsedTestFile
	declared := make(map[string]map[string]struct{})
	for path, source := range sources {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, source, 0)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		parsed := parsedTestFile{path: path, file: file, fset: fset}
		files = append(files, parsed)
		key := filepath.ToSlash(filepath.Dir(path)) + "\x00" + file.Name.Name
		if declared[key] == nil {
			declared[key] = make(map[string]struct{})
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Recv == nil && exportedTestName(function.Name.Name) {
				declared[key][function.Name.Name] = struct{}{}
			}
		}
	}

	var findings []string
	for _, parsed := range files {
		key := filepath.ToSlash(filepath.Dir(parsed.path)) + "\x00" + parsed.file.Name.Name
		for _, declaration := range parsed.file.Decls {
			caller, ok := declaration.(*ast.FuncDecl)
			if !ok || caller.Recv != nil || caller.Body == nil || !exportedTestName(caller.Name.Name) {
				continue
			}
			ast.Inspect(caller.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				callee, ok := call.Fun.(*ast.Ident)
				if !ok {
					return true
				}
				if _, ok := declared[key][callee.Name]; ok {
					position := parsed.fset.Position(call.Pos())
					findings = append(findings, fmt.Sprintf("%s:%d: %s calls %s", parsed.path, position.Line, caller.Name.Name, callee.Name))
				}
				return true
			})
		}
	}
	sort.Strings(findings)
	return findings, nil
}

func TestExportedTestsDoNotInvokeExportedTests(t *testing.T) {
	fixture := map[string][]byte{
		"pkg/example_test.go": []byte("package pkg\nimport \"testing\"\nfunc TestOwner(t *testing.T) {}\nfunc TestDuplicate(t *testing.T) { TestOwner(t) }\n"),
	}
	fixtureFindings, err := directExportedTestCalls(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtureFindings) != 1 || !strings.Contains(fixtureFindings[0], "TestDuplicate calls TestOwner") {
		t.Fatalf("fixture findings = %v, want direct exported-test call", fixtureFindings)
	}

	root := filepath.Join("..", "..")
	sources := make(map[string][]byte)
	testsupport.WalkRepoFiles(t, root, func(path string) bool {
		return strings.HasSuffix(path, "_test.go")
	}, func(path string, body []byte) {
		sources[path] = body
	})
	findings, err := directExportedTestCalls(sources)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("exported tests invoke other exported tests:\n%s", strings.Join(findings, "\n"))
	}
}
