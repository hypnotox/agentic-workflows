package filepublication

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// invariant: tooling/file-publication:exclusive-file-publication-single-home (TestExclusivePublicationHasOneReleasedPlatformHome)
func TestExclusivePublicationHasOneReleasedPlatformHome(t *testing.T) {
	var findings []string
	testsupport.WalkRepoSources(t, testsupport.RepoRoot(t), func(path string, body []byte) {
		findings = append(findings, exclusivePublicationFindings(path, body)...)
	})
	if len(findings) != 0 {
		t.Fatalf("released-platform no-replace publication exists outside internal/filepublication:\n\t%s", strings.Join(findings, "\n\t"))
	}
}

func TestExclusivePublicationDetectorRejectsSecondHomes(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
		body string
	}{
		{name: "Linux no-replace rename", path: "internal/other/linux.go", body: `package other
func publish() { unix.Renameat2(a, b, c, d, unix.RENAME_NOREPLACE) }`},
		{name: "Darwin exclusive rename", path: "internal/other/darwin.go", body: `package other
func publish() { unix.RenamexNp(a, b, unix.RENAME_EXCL) }`},
		{name: "Windows no-replace move", path: "internal/other/windows.go", body: `package other
func publish() { windows.MoveFileEx(a, b, windows.MOVEFILE_WRITE_THROUGH) }`},
		{name: "Windows no-replace move in effort", path: "internal/effort/publication_windows.go", body: `package effort
func publish() { windows.MoveFileEx(a, b, windows.MOVEFILE_WRITE_THROUGH) }`},
		{name: "effort creation branch", path: "internal/effort/publication_linux.go", body: `package effort
func publishAtomic(expected *identity) error { if expected == nil { return publishAtomic(a, b, nil) }; return nil }`},
		{name: "outward package dependency", path: "internal/filepublication/publication.go", body: `package filepublication
import "github.com/hypnotox/agentic-workflows/internal/effort"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := exclusivePublicationFindings(test.path, []byte(test.body)); len(got) == 0 {
				t.Fatal("duplicate implementation was not detected")
			}
		})
	}
}

func TestExclusivePublicationDetectorIgnoresTextAndGenericReplacementMove(t *testing.T) {
	body := []byte(`package effort
// windows.MoveFileEx(a, b, windows.MOVEFILE_WRITE_THROUGH)
const note = "RENAME_NOREPLACE RENAME_EXCL expected == nil"
func moveReplacement(flags uint32) { windows.MoveFileEx(a, b, flags) }
`)
	if got := exclusivePublicationFindings("internal/effort/publication_windows.go", body); len(got) != 0 {
		t.Fatalf("legitimate replacement support was rejected: %v", got)
	}
}

func exclusivePublicationFindings(path string, body []byte) []string {
	file, err := parser.ParseFile(token.NewFileSet(), path, body, 0)
	if err != nil {
		return []string{path + ": cannot parse production source: " + err.Error()}
	}
	var findings []string
	if strings.HasPrefix(path, "internal/filepublication/") {
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err == nil && importPath == "github.com/hypnotox/agentic-workflows/internal/effort" {
				findings = append(findings, path+": publication leaf depends outward on effort")
			}
		}
		return findings
	}

	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.CallExpr:
			selector, ok := value.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch selector.Sel.Name {
			case "Renameat2":
				if callUsesSelector(value, "RENAME_NOREPLACE") {
					findings = append(findings, path+": calls Renameat2 with RENAME_NOREPLACE")
				}
			case "RenamexNp":
				if callUsesSelector(value, "RENAME_EXCL") {
					findings = append(findings, path+": calls RenamexNp with RENAME_EXCL")
				}
			case "MoveFileEx":
				if callUsesSelector(value, "MOVEFILE_WRITE_THROUGH") && !callUsesSelector(value, "MOVEFILE_REPLACE_EXISTING") {
					findings = append(findings, path+": calls MoveFileEx as a no-replace publication")
				}
			}
		case *ast.IfStmt:
			if expectedNilCondition(value.Cond) && blockCalls(value.Body, "publishAtomic") {
				findings = append(findings, path+": retains an expected-absent effort publication branch")
			}
		}
		return true
	})
	return findings
}

func callUsesSelector(call *ast.CallExpr, name string) bool {
	for _, argument := range call.Args {
		selector, ok := argument.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == name {
			return true
		}
	}
	return false
}

func expectedNilCondition(expression ast.Expr) bool {
	binary, ok := expression.(*ast.BinaryExpr)
	if !ok || binary.Op != token.EQL {
		return false
	}
	left, leftOK := binary.X.(*ast.Ident)
	right, rightOK := binary.Y.(*ast.Ident)
	return leftOK && rightOK && ((left.Name == "expected" && right.Name == "nil") || (left.Name == "nil" && right.Name == "expected"))
}

func blockCalls(block *ast.BlockStmt, name string) bool {
	found := false
	ast.Inspect(block, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		identifier, ok := call.Fun.(*ast.Ident)
		if ok && identifier.Name == name {
			found = true
		}
		return !found
	})
	return found
}
