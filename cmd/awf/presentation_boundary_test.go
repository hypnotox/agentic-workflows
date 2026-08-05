package main

import (
	"bytes"
	"errors"
	"go/ast"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"golang.org/x/tools/go/packages"
)

func TestCommandOutputBoundary(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"awf", "bogus"}, &stdout, &stderr); code != 2 {
		t.Fatalf("unknown command exit = %d", code)
	}
	if stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("usage output streams stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

type typedDiagnosticError struct {
	diagnostic presentation.Diagnostic
	err        error
}

func (e typedDiagnosticError) Error() string { return "typed diagnostic" }
func (e typedDiagnosticError) Diagnostic() (presentation.Diagnostic, error) {
	return e.diagnostic, e.err
}

func TestDiagnosticOutcomeUsesTypedDiagnostic(t *testing.T) {
	value, err := presentation.Prose("retry")
	if err != nil {
		t.Fatal(err)
	}
	outcome := diagnosticOutcome(typedDiagnosticError{diagnostic: presentation.Diagnostic{Condition: "operation refused", State: "operation", Steps: []presentation.Value{value}}})
	if outcome.stream != commandStderr || outcome.exit != 1 || outcome.err == nil {
		t.Fatalf("typed outcome = %#v", outcome)
	}
	var stdout, stderr bytes.Buffer
	if code := writeOutcome(&stdout, &stderr, outcome); code != 1 || stdout.Len() != 0 {
		t.Fatalf("typed diagnostic streams: code=%d stdout=%q", code, stdout.String())
	}
	const want = "condition: operation refused\nstate: operation\n\ndiagnostic:\n  steps:\n    step 1: retry\n"
	if stderr.String() != want {
		t.Fatalf("typed diagnostic stderr = %q, want %q", stderr.String(), want)
	}
}

func TestTypedDiagnosticFallbackRendersOriginalFailureOnce(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{{"mapping", typedDiagnosticError{err: errors.New("mapping failed")}}, {"document", typedDiagnosticError{diagnostic: presentation.Diagnostic{Condition: " "}}}} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := writeOutcome(&stdout, &stderr, diagnosticOutcome(test.err)); code != 1 {
				t.Fatalf("writeOutcome exit = %d", code)
			}
			if stdout.Len() != 0 {
				t.Fatalf("writeOutcome stdout = %q", stdout.String())
			}
			failure := "mapping failed"
			if test.name == "document" {
				failure = "presentation value is empty"
			}
			if strings.Count(stderr.String(), failure) != 1 {
				t.Fatalf("writeOutcome stderr = %q, want %q exactly once", stderr.String(), failure)
			}
		})
	}
}

func TestWriteStatusValidatesAndPropagatesWrites(t *testing.T) {
	if err := writeStatus(io.Discard, " \n\t"); err == nil {
		t.Fatal("empty normalized status accepted")
	}
	if err := writeStatus(errorWriter{}, "ready"); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("write error = %v", err)
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestRendererFailureFallback(t *testing.T) {
	for _, test := range []struct {
		name  string
		cause error
		want  string
	}{{"ordinary", errors.New("render failed"), "awf: render failed\n"}, {"hostile whitespace", errors.New("\t render\n\u00a0failed \r\n"), "awf: render failed\n"}, {"empty", errors.New(" \t\n\u00a0"), "awf: renderer failed\n"}} {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer
			writeRendererFailure(&stderr, test.cause)
			if got := stderr.String(); got != test.want {
				t.Fatalf("fallback = %q, want %q", got, test.want)
			}
		})
	}
}

func TestWriteOutcomeRendererFailureIsSingleDiagnostic(t *testing.T) {
	value, _ := presentation.Prose("ready")
	field, _ := presentation.NewField("condition", value)
	document, _ := presentation.NewDocument(field)
	var stdout, stderr bytes.Buffer
	if got := writeOutcomeWithRenderer(&stdout, &stderr, commandOutcome{document: document}, func(_ io.Writer, _ presentation.Document) error { return errors.New("render failed") }); got != 1 {
		t.Fatalf("exit = %d", got)
	}
	if stdout.Len() != 0 || stderr.String() != "awf: render failed\n" {
		t.Fatalf("streams stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

// TestOrdinaryCommandOutputUsesPresentation scans the command boundary and its
// result-model owners. Ordinary output must lower through presentation; only
// the five named successful payload/protocol functions may write bytes directly.
func TestOrdinaryCommandOutputUsesPresentation(t *testing.T) {
	if findings := ordinaryOutputFindings(t, nil); len(findings) != 0 {
		t.Fatalf("ordinary output bypasses:\n%s", strings.Join(findings, "\n"))
	}
	for _, fixture := range []string{"negative-direct-write.go", "negative-builder.go", "negative-format.go", "negative-markdown.go", "negative-renderer.go"} {
		if findings := fixtureFindings(t, fixture); len(findings) == 0 {
			t.Errorf("%s produced no finding", fixture)
		}
	}
	for _, fixture := range []string{"positive-read-plan.go", "positive-changelog.go", "positive-activity.go", "positive-init.go", "positive-context-delivery.go"} {
		if findings := fixtureFindings(t, fixture); len(findings) != 0 {
			t.Errorf("%s findings: %v", fixture, findings)
		}
	}
}

// TestExplicitOutputBypasses proves the successful bypass set is closed and
// separately proves that the renderer fallback is reachable only after failure.
func TestExplicitOutputBypasses(t *testing.T) {
	if findings := ordinaryOutputFindings(t, nil); len(findings) != 0 {
		t.Fatalf("unexpected bypass: %s", strings.Join(findings, "; "))
	}
	if findings := rendererFailureFindings(t, nil); len(findings) != 0 {
		t.Fatalf("renderer fallback reachability: %s", strings.Join(findings, "; "))
	}
	if findings := fixtureRendererFindings(t, "negative-renderer-fallback.go"); len(findings) == 0 {
		t.Fatal("negative renderer fallback fixture produced no finding")
	}
	if findings := fixtureRendererFindings(t, "positive-renderer-fallback.go"); len(findings) != 0 {
		t.Fatalf("positive renderer fallback findings: %v", findings)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

const modulePath = "github.com/hypnotox/agentic-workflows"

var successfulBypasses = map[string]bool{
	modulePath + "/cmd/awf.runReadPlan":                 true,
	modulePath + "/cmd/awf.writeChangelogPayload":       true,
	modulePath + "/cmd/awf.writeEffortActivityProtocol": true,
	modulePath + "/cmd/awf.writeInitDescriptorProtocol": true,
	modulePath + "/internal/contextdelivery.Deliver":    true,
}

// These exact helpers write authored files or spill-file storage rather than a
// user-visible presentation. They are serialization sinks, not output bypasses.
var nonPresentationWrites = map[string]bool{
	modulePath + "/cmd/awf.writeAndCloseTopicFile":     true,
	modulePath + "/internal/contextdelivery.writeFull": true,
}

func loadPresentationPackages(t *testing.T, overlay map[string][]byte) []*packages.Package {
	t.Helper()
	patterns := []string{
		"./cmd/awf", "./internal/audit", "./internal/clispec", "./internal/commitpolicy",
		"./internal/contextdelivery", "./internal/contextq", "./internal/effort", "./internal/initspec",
		"./internal/memorycite", "./internal/project", "./internal/prosegate", "./internal/topic",
		"./internal/upgrade", "./internal/worktree",
	}
	pkgs, err := packages.Load(&packages.Config{Dir: repoRoot(t), Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax, Overlay: overlay}, patterns...)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		if len(pkg.Errors) != 0 {
			t.Fatal(pkg.Errors[0])
		}
	}
	return pkgs
}

func ordinaryOutputFindings(t *testing.T, overlay map[string][]byte) []string {
	t.Helper()
	var findings []string
	declaredBypasses := map[string]bool{}
	for _, pkg := range loadPresentationPackages(t, overlay) {
		for _, file := range pkg.Syntax {
			if !presentationOwnerFile(pkg, file) {
				continue
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				identity := functionIdentity(pkg, fn)
				if successfulBypasses[identity] {
					declaredBypasses[identity] = true
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					switch node := n.(type) {
					case *ast.CallExpr:
						if ordinaryWriteCall(node) && !successfulBypasses[identity] && !nonPresentationWrites[identity] && identity != modulePath+"/cmd/awf.writeRendererFailure" {
							findings = append(findings, presentationSite(pkg, node)+" direct output in "+identity)
						}
						if forbiddenPresentationConstruction(node) {
							findings = append(findings, presentationSite(pkg, node)+" ad hoc presentation construction in "+identity)
						}
					case *ast.BasicLit:
						if !successfulBypasses[identity] && forbiddenPresentationLiteral(node) {
							findings = append(findings, presentationSite(pkg, node)+" raw Markdown presentation in "+identity)
						}
					}
					return true
				})
			}
		}
	}
	if overlay == nil {
		for identity := range successfulBypasses {
			if !declaredBypasses[identity] {
				findings = append(findings, "missing successful bypass symbol "+identity)
			}
		}
	}
	sort.Strings(findings)
	return findings
}

func presentationOwnerFile(pkg *packages.Package, file *ast.File) bool {
	if pkg.PkgPath == modulePath+"/cmd/awf" || pkg.PkgPath == modulePath+"/internal/contextdelivery" {
		return true
	}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err == nil && path == modulePath+"/internal/presentation" {
			return true
		}
	}
	return false
}

func functionIdentity(pkg *packages.Package, fn *ast.FuncDecl) string {
	return pkg.PkgPath + "." + fn.Name.Name
}

func ordinaryWriteCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if id, ok := sel.X.(*ast.Ident); ok && (id.Name == "fmt" || id.Name == "io") {
		return strings.HasPrefix(sel.Sel.Name, "Fprint") || strings.HasPrefix(sel.Sel.Name, "Print") || sel.Sel.Name == "WriteString" || sel.Sel.Name == "Copy"
	}
	return sel.Sel.Name == "Write" || sel.Sel.Name == "WriteString" || sel.Sel.Name == "WriteByte"
}

func forbiddenPresentationConstruction(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok || id.Name != "fmt" || (sel.Sel.Name != "Sprintf" && sel.Sel.Name != "Fprintf" && sel.Sel.Name != "Printf") || len(call.Args) == 0 {
		return false
	}
	format, ok := call.Args[0].(*ast.BasicLit)
	if sel.Sel.Name == "Fprintf" && len(call.Args) > 1 {
		format, ok = call.Args[1].(*ast.BasicLit)
	}
	if !ok {
		return false
	}
	text, err := strconv.Unquote(format.Value)
	return err == nil && (strings.Contains(text, "%-") || strings.Contains(text, "% "))
}

func forbiddenPresentationLiteral(literal *ast.BasicLit) bool {
	if literal.Kind.String() != "STRING" {
		return false
	}
	text, err := strconv.Unquote(literal.Value)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "|---") || strings.HasPrefix(trimmed, "| ---") {
			return true
		}
	}
	return false
}

func presentationSite(pkg *packages.Package, node ast.Node) string {
	p := pkg.Fset.Position(node.Pos())
	return filepath.ToSlash(p.Filename) + ":" + strconv.Itoa(p.Line)
}

func fixtureFindings(t *testing.T, name string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "presentation-boundary", name))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(repoRoot(t), "cmd", "awf")
	if name == "positive-context-delivery.go" {
		dir = filepath.Join(repoRoot(t), "internal", "contextdelivery")
	}
	return ordinaryOutputFindings(t, map[string][]byte{filepath.Join(dir, "zz_presentation_fixture.go"): data})
}

func rendererFailureFindings(t *testing.T, overlay map[string][]byte) []string {
	t.Helper()
	var findings []string
	productionCalls := 0
	for _, pkg := range loadPresentationPackages(t, overlay) {
		if pkg.PkgPath != modulePath+"/cmd/awf" {
			continue
		}
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				identity := functionIdentity(pkg, fn)
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok || calledName(call) != "writeRendererFailure" {
						return true
					}
					if identity == modulePath+"/cmd/awf.writeOutcomeWithRenderer" {
						productionCalls++
					}
					if !rendererFailureDominates(fn, call) {
						findings = append(findings, presentationSite(pkg, call)+" writeRendererFailure is not dominated by renderer failure in "+identity)
					}
					return true
				})
			}
		}
	}
	if overlay == nil && productionCalls != 1 {
		findings = append(findings, "writeRendererFailure production call count = "+strconv.Itoa(productionCalls)+", want 1")
	}
	sort.Strings(findings)
	return findings
}

func calledName(call *ast.CallExpr) string {
	if id, ok := call.Fun.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func rendererFailureDominates(fn *ast.FuncDecl, target *ast.CallExpr) bool {
	identityAllowed := fn.Name.Name == "writeOutcomeWithRenderer" || fn.Name.Name == "fixtureRendererFailure"
	if !identityAllowed || len(target.Args) < 2 {
		return false
	}
	cause, ok := target.Args[len(target.Args)-1].(*ast.Ident)
	if !ok {
		return false
	}
	valid := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		stmt, ok := n.(*ast.IfStmt)
		if !ok || !containsNode(stmt.Body, target) {
			return true
		}
		assignment, ok := stmt.Init.(*ast.AssignStmt)
		if !ok || assignment.Tok.String() != ":=" || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
			return true
		}
		bound, ok := assignment.Lhs[0].(*ast.Ident)
		renderCall, renderOK := assignment.Rhs[0].(*ast.CallExpr)
		condition, conditionOK := stmt.Cond.(*ast.BinaryExpr)
		left, leftOK := condition.X.(*ast.Ident)
		right, rightOK := condition.Y.(*ast.Ident)
		if ok && renderOK && conditionOK && leftOK && rightOK && bound.Name == cause.Name && left.Name == bound.Name && right.Name == "nil" && condition.Op.String() == "!=" && calledName(renderCall) == "render" {
			valid = true
		}
		return true
	})
	return valid
}

func containsNode(root, target ast.Node) bool {
	found := false
	ast.Inspect(root, func(n ast.Node) bool {
		if n == target {
			found = true
			return false
		}
		return !found
	})
	return found
}

func fixtureRendererFindings(t *testing.T, name string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "presentation-boundary", name))
	if err != nil {
		t.Fatal(err)
	}
	return rendererFailureFindings(t, map[string][]byte{filepath.Join(repoRoot(t), "cmd", "awf", "zz_renderer_fixture.go"): data})
}
