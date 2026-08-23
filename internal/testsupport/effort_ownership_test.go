package testsupport_test

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

const (
	effortopImport = "github.com/hypnotox/agentic-workflows/internal/effortop"
	effortImport   = "github.com/hypnotox/agentic-workflows/internal/effort"
	worktreeImport = "github.com/hypnotox/agentic-workflows/internal/worktree"
)

// TestEffortCommandUsesOneFocusedOperationPerLeaf protects the composition
// boundary. It associates calls with runEffort's actual switch leaves, rather
// than counting source text, so comments and dead helpers cannot satisfy a
// command route.
func TestEffortCommandUsesOneFocusedOperationPerLeaf(t *testing.T) {
	root := testsupport.RepoRoot(t)
	path := filepath.Join(root, "cmd", "awf", "effort.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyEffortCommandComposition(file); err != nil {
		t.Error(err)
	}

	operation, err := os.ReadFile(filepath.Join(root, "internal", "effortop", "operation.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{"internal/effort", "internal/worktree"} {
		if !strings.Contains(string(operation), owner) {
			t.Errorf("effortop no longer coordinates %s", owner)
		}
	}
}

func TestEffortCommandOwnershipASTProofRejectsNonRoutes(t *testing.T) {
	tests := []struct {
		name, source, want string
	}{
		{
			name: "comment and dead helper do not satisfy show",
			source: `package fixture
import e "github.com/hypnotox/agentic-workflows/internal/effortop"
func runEffort() { switch c.sub {
case "new": e.New()
case "list": e.List()
case "show": return /* e.Show() */
case "finish": e.Finish()
case "worktree": if c.add { e.AddWorktree() } else { e.RemoveWorktree() }
case "integrate": e.Integrate()
case "memory read": e.ReadMemory()
case "memory edit": e.EditMemory()
case "memory update": e.UpdateMemory()
case "activity attach": e.AttachActivity()
case "activity heartbeat": e.HeartbeatActivity()
case "activity detach": e.DetachActivity()
} }
func obsoleteHelper() { e.Show() }`,
			want: `leaf "show" focused operations = []`,
		},
		{
			name: "renamed direct owner import is rejected",
			source: `package fixture
import (
 e "github.com/hypnotox/agentic-workflows/internal/effortop"
 resident "github.com/hypnotox/agentic-workflows/internal/effort"
)
func runEffort() { switch "show" { case "show": e.Show(); resident.Show() } }`,
			want: "direct effort operation Show",
		},
		{
			name: "renamed composed owner receiver is rejected",
			source: `package fixture
import e "github.com/hypnotox/agentic-workflows/internal/effortop"
func runEffort() {
 resident := composed.service
 switch c.sub { case "show": e.Show(); resident.Show() }
}`,
			want: "direct effort operation Show",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), tt.name+".go", tt.source, 0)
			if err != nil {
				t.Fatal(err)
			}
			err = verifyEffortCommandComposition(file)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("proof error = %v, want %q", err, tt.want)
			}
		})
	}
}

type effortLeaf struct {
	name string
	body []ast.Stmt
}

var effortLeafOperations = map[string]string{
	"new":                "New",
	"list":               "List",
	"show":               "Show",
	"finish":             "Finish",
	"worktree add":       "AddWorktree",
	"worktree remove":    "RemoveWorktree",
	"integrate":          "Integrate",
	"memory read":        "ReadMemory",
	"memory edit":        "EditMemory",
	"memory update":      "UpdateMemory",
	"activity attach":    "AttachActivity",
	"activity heartbeat": "HeartbeatActivity",
	"activity detach":    "DetachActivity",
}

var directEffortOperations = map[string]bool{
	"New": true, "List": true, "Show": true, "Finish": true, "Memory": true,
	"UpdateMemory": true, "AttachActivity": true, "HeartbeatActivity": true, "DetachActivity": true,
}

var directWorktreeOperations = map[string]bool{
	"NewEffort": true, "Add": true, "Remove": true, "Integrate": true,
}

func verifyEffortCommandComposition(file *ast.File) error {
	imports, err := effortImportAliases(file)
	if err != nil {
		return err
	}
	if err := rejectDirectOwnerOperations(file, imports); err != nil {
		return err
	}
	fn := functionDeclarations(file)["runEffort"]
	if fn == nil || fn.Body == nil {
		return errors.New("runEffort is absent")
	}
	owners := effortOwnerReceivers(fn)
	if err := rejectOwnerReceiverOperations(fn, owners); err != nil {
		return err
	}
	leaves, err := resolvedEffortLeaves(fn)
	if err != nil {
		return err
	}
	if len(leaves) != len(effortLeafOperations) {
		return fmt.Errorf("runEffort operation leaves = %d, want %d", len(leaves), len(effortLeafOperations))
	}
	seen := map[string]bool{}
	for _, leaf := range leaves {
		want, known := effortLeafOperations[leaf.name]
		if !known || seen[leaf.name] {
			return fmt.Errorf("unexpected or duplicate runEffort leaf %q", leaf.name)
		}
		seen[leaf.name] = true
		operations, err := focusedCalls(leaf.body, imports)
		if err != nil {
			return fmt.Errorf("runEffort leaf %q: %w", leaf.name, err)
		}
		if len(operations) != 1 || operations[0] != want {
			return fmt.Errorf("runEffort leaf %q focused operations = %v, want exactly [%s]", leaf.name, operations, want)
		}
	}
	for leaf := range effortLeafOperations {
		if !seen[leaf] {
			return fmt.Errorf("runEffort leaf %q is absent", leaf)
		}
	}
	return nil
}

func effortImportAliases(file *ast.File) (map[string]string, error) {
	aliases := map[string]string{}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, err
		}
		if path != effortopImport && path != effortImport && path != worktreeImport {
			continue
		}
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "." || name == "_" {
			return nil, fmt.Errorf("unsupported import alias %q for %s", name, path)
		}
		aliases[name] = path
	}
	return aliases, nil
}

func resolvedEffortLeaves(fn *ast.FuncDecl) ([]effortLeaf, error) {
	var route *ast.SwitchStmt
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.SwitchStmt:
			if route == nil && isEffortRouteSwitch(value) {
				route = value
			}
		}
		return true
	})
	if route == nil {
		return nil, errors.New("runEffort switch is absent")
	}
	var leaves []effortLeaf
	for _, statement := range route.Body.List {
		clause, ok := statement.(*ast.CaseClause)
		if !ok {
			return nil, errors.New("runEffort switch contains a non-case clause")
		}
		name, ok := stringCaseName(clause)
		if !ok {
			continue // The default refusal is not an operation leaf.
		}
		if name != "worktree" {
			leaves = append(leaves, effortLeaf{name: name, body: clause.Body})
			continue
		}
		branch := firstIf(clause.Body)
		if branch == nil || branch.Else == nil {
			return nil, errors.New("worktree route does not resolve add and remove branches")
		}
		leaves = append(leaves, effortLeaf{name: "worktree add", body: statementList(branch.Body)})
		leaves = append(leaves, effortLeaf{name: "worktree remove", body: statementList(branch.Else)})
	}
	return leaves, nil
}

func isEffortRouteSwitch(route *ast.SwitchStmt) bool {
	selector, ok := route.Tag.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "sub" {
		return false
	}
	receiver, ok := selector.X.(*ast.Ident)
	return ok && receiver.Name == "c"
}

func stringCaseName(clause *ast.CaseClause) (string, bool) {
	if len(clause.List) != 1 {
		return "", false
	}
	literal, ok := clause.List[0].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	name, err := strconv.Unquote(literal.Value)
	return name, err == nil
}

func firstIf(statements []ast.Stmt) *ast.IfStmt {
	for _, statement := range statements {
		if branch, ok := statement.(*ast.IfStmt); ok {
			return branch
		}
	}
	return nil
}

func statementList(statement ast.Stmt) []ast.Stmt {
	if block, ok := statement.(*ast.BlockStmt); ok {
		return block.List
	}
	return []ast.Stmt{statement}
}

func focusedCalls(statements []ast.Stmt, imports map[string]string) ([]string, error) {
	var calls []string
	var err error
	for _, statement := range statements {
		ast.Inspect(statement, func(node ast.Node) bool {
			if _, ok := node.(*ast.FuncLit); ok {
				return false
			}
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			receiver, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			switch imports[receiver.Name] {
			case effortopImport:
				calls = append(calls, selector.Sel.Name)
			case effortImport:
				if directEffortOperations[selector.Sel.Name] {
					err = fmt.Errorf("direct effort operation %s", selector.Sel.Name)
				}
			case worktreeImport:
				if directWorktreeOperations[selector.Sel.Name] {
					err = fmt.Errorf("direct worktree operation %s", selector.Sel.Name)
				}
			}
			return true
		})
		if err != nil {
			return nil, err
		}
	}
	return calls, nil
}

func effortOwnerReceivers(fn *ast.FuncDecl) map[string]string {
	owners := map[string]string{}
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || len(assignment.Lhs) != len(assignment.Rhs) {
			return true
		}
		for i, right := range assignment.Rhs {
			left, ok := assignment.Lhs[i].(*ast.Ident)
			if !ok {
				continue
			}
			if owner := effortOwnerExpression(right, owners); owner != "" {
				owners[left.Name] = owner
			}
		}
		return true
	})
	return owners
}

func effortOwnerExpression(expression ast.Expr, owners map[string]string) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return owners[value.Name]
	case *ast.SelectorExpr:
		switch value.Sel.Name {
		case "service":
			return effortImport
		case "manager":
			return worktreeImport
		}
	}
	return ""
}

func rejectOwnerReceiverOperations(fn *ast.FuncDecl, owners map[string]string) error {
	var found error
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok || found != nil {
			return found == nil
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch effortOwnerExpression(selector.X, owners) {
		case effortImport:
			if directEffortOperations[selector.Sel.Name] {
				found = fmt.Errorf("direct effort operation %s", selector.Sel.Name)
			}
		case worktreeImport:
			if directWorktreeOperations[selector.Sel.Name] {
				found = fmt.Errorf("direct worktree operation %s", selector.Sel.Name)
			}
		}
		return true
	})
	return found
}

func rejectDirectOwnerOperations(file *ast.File, imports map[string]string) error {
	var found error
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || found != nil {
			return found == nil
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		switch imports[receiver.Name] {
		case effortImport:
			if directEffortOperations[selector.Sel.Name] {
				found = fmt.Errorf("direct effort operation %s", selector.Sel.Name)
			}
		case worktreeImport:
			if directWorktreeOperations[selector.Sel.Name] {
				found = fmt.Errorf("direct worktree operation %s", selector.Sel.Name)
			}
		}
		return true
	})
	return found
}
