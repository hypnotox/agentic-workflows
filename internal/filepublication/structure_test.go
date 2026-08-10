package filepublication

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

const modulePath = "github.com/hypnotox/agentic-workflows"

// invariant: tooling/file-publication:exclusive-file-publication-single-home (TestExclusivePublicationHasOneReleasedPlatformHome)
func TestExclusivePublicationHasOneReleasedPlatformHome(t *testing.T) {
	var findings []string
	testsupport.WalkRepoSources(t, testsupport.RepoRoot(t), func(sourcePath string, body []byte) {
		findings = append(findings, exclusivePublicationFindings(sourcePath, body)...)
	})
	if len(findings) != 0 {
		t.Fatalf("released-platform no-replace publication exists outside internal/filepublication:\n\t%s", strings.Join(findings, "\n\t"))
	}
}

func TestExclusivePublicationDetectorRejectsSecondHomes(t *testing.T) {
	for _, test := range []struct {
		name       string
		sourcePath string
		body       string
	}{
		{name: "Linux no-replace rename", sourcePath: "internal/other/linux.go", body: `package other
import u "golang.org/x/sys/unix"
func publish() { u.Renameat2(a, b, c, d, u.RENAME_NOREPLACE) }`},
		{name: "Darwin aliased exclusive rename", sourcePath: "internal/other/darwin.go", body: `package other
import "golang.org/x/sys/unix"
const exclusive = unix.RENAME_EXCL
func publish() { unix.RenamexNp(a, b, exclusive) }`},
		{name: "Windows zero flags", sourcePath: "internal/other/windows.go", body: `package other
import "golang.org/x/sys/windows"
func publish() { windows.MoveFileEx(a, b, 0) }`},
		{name: "Windows composed no-replace flags", sourcePath: "internal/other/windows.go", body: `package other
import w "golang.org/x/sys/windows"
const exclusive = w.MOVEFILE_WRITE_THROUGH | 2
func publish() { w.MoveFileEx(a, b, exclusive) }`},
		{name: "Windows no-replace move in effort", sourcePath: "internal/effort/publication_windows.go", body: `package effort
import "golang.org/x/sys/windows"
func publish() { windows.MoveFileEx(a, b, windows.MOVEFILE_WRITE_THROUGH) }`},
		{name: "typed nil effort creation branch", sourcePath: "internal/effort/publication_linux.go", body: `package effort
type identity struct{}
func publishAtomic(expected *identity) error { if expected == (*identity)(nil) { return publishAtomic(a, b, nil) }; return nil }`},
		{name: "outward effort dependency", sourcePath: "internal/filepublication/publication.go", body: `package filepublication
import "github.com/hypnotox/agentic-workflows/internal/effort"`},
		{name: "other outward internal dependency", sourcePath: "internal/filepublication/publication.go", body: `package filepublication
import f "github.com/hypnotox/agentic-workflows/internal/filesystem"
var _ = f.Handle{}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := exclusivePublicationFindings(test.sourcePath, []byte(test.body)); len(got) == 0 {
				t.Fatal("duplicate implementation was not detected")
			}
		})
	}
}

func TestExclusivePublicationDetectorAcceptsReplacementAndUnrelatedText(t *testing.T) {
	for _, test := range []struct {
		name       string
		sourcePath string
		body       string
	}{
		{name: "comments strings and unrelated selector", sourcePath: "internal/other/notes.go", body: `package other
// windows.MoveFileEx(a, b, windows.MOVEFILE_WRITE_THROUGH)
const note = "RENAME_NOREPLACE RENAME_EXCL expected == nil"
type localAPI struct{}
func (localAPI) MoveFileEx(any, any, uint32) error { return nil }
func replace(api localAPI) { api.MoveFileEx(a, b, 0) }`},
		{name: "generic effort replacement move", sourcePath: "internal/effort/publication_windows.go", body: `package effort
import "golang.org/x/sys/windows"
func moveReplacement(flags uint32) { windows.MoveFileEx(a, b, flags) }`},
		{name: "explicit Windows replacement", sourcePath: "internal/other/windows.go", body: `package other
import w "golang.org/x/sys/windows"
const replace = w.MOVEFILE_REPLACE_EXISTING | w.MOVEFILE_WRITE_THROUGH
func publish() { w.MoveFileEx(a, b, replace) }`},
		{name: "Unix replacement operations", sourcePath: "internal/other/unix.go", body: `package other
import "golang.org/x/sys/unix"
func replace() { unix.Renameat2(a, b, c, d, unix.RENAME_EXCHANGE); unix.RenamexNp(a, b, unix.RENAME_SWAP) }`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := exclusivePublicationFindings(test.sourcePath, []byte(test.body)); len(got) != 0 {
				t.Fatalf("legitimate replacement support was rejected: %v", got)
			}
		})
	}
}

func exclusivePublicationFindings(sourcePath string, body []byte) []string {
	file, err := parser.ParseFile(token.NewFileSet(), sourcePath, body, 0)
	if err != nil {
		return []string{sourcePath + ": cannot parse production source: " + err.Error()}
	}
	imports := importAliases(file)
	constants := constantExpressions(file)
	var findings []string
	if strings.HasPrefix(sourcePath, "internal/filepublication/") {
		for _, importPath := range imports {
			if strings.HasPrefix(importPath, modulePath+"/internal/") {
				findings = append(findings, sourcePath+": publication leaf depends outward on "+importPath)
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
			qualifier, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			importPath := imports[qualifier.Name]
			switch {
			case importPath == "golang.org/x/sys/unix" && selector.Sel.Name == "Renameat2" && callFlagReferences(value, constants, qualifier.Name, "RENAME_NOREPLACE"):
				findings = append(findings, sourcePath+": calls unix.Renameat2 with RENAME_NOREPLACE")
			case importPath == "golang.org/x/sys/unix" && selector.Sel.Name == "RenamexNp" && callFlagReferences(value, constants, qualifier.Name, "RENAME_EXCL"):
				findings = append(findings, sourcePath+": calls unix.RenamexNp with RENAME_EXCL")
			case importPath == "golang.org/x/sys/windows" && selector.Sel.Name == "MoveFileEx" && windowsMoveIsNoReplace(sourcePath, value, constants, qualifier.Name):
				findings = append(findings, sourcePath+": calls windows.MoveFileEx without MOVEFILE_REPLACE_EXISTING")
			}
		case *ast.IfStmt:
			if expectedNilCondition(value.Cond) && blockCalls(value.Body, "publishAtomic") {
				findings = append(findings, sourcePath+": retains an expected-absent effort publication branch")
			}
		}
		return true
	})
	return findings
}

func importAliases(file *ast.File) map[string]string {
	imports := make(map[string]string, len(file.Imports))
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil || (spec.Name != nil && (spec.Name.Name == "." || spec.Name.Name == "_")) {
			continue
		}
		name := path.Base(importPath)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = importPath
	}
	return imports
}

func constantExpressions(file *ast.File) map[string]ast.Expr {
	constants := map[string]ast.Expr{}
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.CONST {
			continue
		}
		for _, spec := range general.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, name := range value.Names {
				if index < len(value.Values) {
					constants[name.Name] = value.Values[index]
				}
			}
		}
	}
	return constants
}

func callFlagReferences(call *ast.CallExpr, constants map[string]ast.Expr, qualifier, flag string) bool {
	if len(call.Args) == 0 {
		return false
	}
	return expressionReferencesSelector(call.Args[len(call.Args)-1], constants, qualifier, flag, map[string]bool{})
}

func expressionReferencesSelector(expression ast.Expr, constants map[string]ast.Expr, qualifier, flag string, visiting map[string]bool) bool {
	found := false
	ast.Inspect(expression, func(node ast.Node) bool {
		if found {
			return false
		}
		switch value := node.(type) {
		case *ast.SelectorExpr:
			identifier, ok := value.X.(*ast.Ident)
			if ok && identifier.Name == qualifier && value.Sel.Name == flag {
				found = true
				return false
			}
		case *ast.Ident:
			constant, ok := constants[value.Name]
			if ok && !visiting[value.Name] {
				visiting[value.Name] = true
				found = expressionReferencesSelector(constant, constants, qualifier, flag, visiting)
				delete(visiting, value.Name)
			}
		}
		return true
	})
	return found
}

func windowsMoveIsNoReplace(sourcePath string, call *ast.CallExpr, constants map[string]ast.Expr, qualifier string) bool {
	if len(call.Args) < 3 {
		return true
	}
	flags := call.Args[2]
	if sourcePath == "internal/effort/publication_windows.go" {
		if identifier, ok := flags.(*ast.Ident); ok && identifier.Name == "flags" {
			return false
		}
	}
	return !expressionReferencesSelector(flags, constants, qualifier, "MOVEFILE_REPLACE_EXISTING", map[string]bool{})
}

func expectedNilCondition(expression ast.Expr) bool {
	for {
		parenthesized, ok := expression.(*ast.ParenExpr)
		if !ok {
			break
		}
		expression = parenthesized.X
	}
	binary, ok := expression.(*ast.BinaryExpr)
	if !ok || binary.Op != token.EQL {
		return false
	}
	return (expressionIsIdentifier(binary.X, "expected") && expressionIsNil(binary.Y)) || (expressionIsNil(binary.X) && expressionIsIdentifier(binary.Y, "expected"))
}

func expressionIsIdentifier(expression ast.Expr, name string) bool {
	for {
		parenthesized, ok := expression.(*ast.ParenExpr)
		if !ok {
			break
		}
		expression = parenthesized.X
	}
	identifier, ok := expression.(*ast.Ident)
	return ok && identifier.Name == name
}

func expressionIsNil(expression ast.Expr) bool {
	for {
		parenthesized, ok := expression.(*ast.ParenExpr)
		if !ok {
			break
		}
		expression = parenthesized.X
	}
	if identifier, ok := expression.(*ast.Ident); ok {
		return identifier.Name == "nil"
	}
	conversion, ok := expression.(*ast.CallExpr)
	return ok && len(conversion.Args) == 1 && expressionIsNil(conversion.Args[0])
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
