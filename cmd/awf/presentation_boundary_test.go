package main

import (
	"bytes"
	"errors"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
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
	var stderr bytes.Buffer
	if got := writeOutcome(errorWriter{}, &stderr, commandOutcome{document: document}); got != 1 {
		t.Fatalf("exit = %d", got)
	}
	if stderr.String() != "awf: write presentation: write failed\n" {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

// TestOrdinaryCommandOutputUsesPresentation scans the command boundary and its
// result-model owners. Ordinary output must lower through presentation; only
// the five named successful payload/protocol functions may write bytes directly.
func TestOrdinaryCommandOutputUsesPresentation(t *testing.T) {
	if findings := ordinaryOutputFindings(t, nil); len(findings) != 0 {
		t.Fatalf("ordinary output bypasses:\n%s", strings.Join(findings, "\n"))
	}
	negativeFixtures := map[string]string{
		"negative-direct-write.go": "direct output",
		"negative-builder.go":      "direct output",
		"negative-format.go":       "ad hoc presentation construction",
		"negative-markdown.go":     "raw Markdown presentation",
		"negative-renderer.go":     "direct output",
		"negative-resident.go":     "direct output",
		"negative-alias.go":        "direct output",
	}
	for fixture, want := range negativeFixtures {
		findings := fixtureFindings(t, fixture)
		if len(findings) != 1 || !strings.Contains(findings[0], want) {
			t.Errorf("%s findings = %v, want exactly one %q finding", fixture, findings, want)
		}
	}
	for _, fixture := range []string{"positive-read-plan.go", "positive-changelog.go", "positive-activity.go", "positive-init.go", "positive-context-delivery.go", "positive-shadow.go"} {
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
	if findings := fixtureRendererFindings(t, "positive-renderer-fallback.go"); len(findings) != 0 {
		t.Fatalf("positive renderer fallback findings: %v", findings)
	}
	for _, fixture := range []string{"negative-renderer-fallback.go", "negative-nonpresentation-renderer.go"} {
		findings := fixtureRendererFindings(t, fixture)
		if len(findings) != 1 || !strings.Contains(findings[0], "not dominated by renderer failure") {
			t.Fatalf("%s findings = %v", fixture, findings)
		}
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
	modulePath + "/cmd/awf.writeAndCloseTopicFile":          true,
	modulePath + "/internal/contextdelivery.writeFull":      true,
	modulePath + "/internal/contextq.contextGroupKey":       true,
	modulePath + "/internal/effort.replaceResidentExpected": true,
	modulePath + "/internal/effort.replaceMemory":           true,
	modulePath + "/internal/effort.publishNew":              true,
	modulePath + "/internal/project.encodeMarkdownAgent":    true,
	modulePath + "/internal/project.glossaryRows":           true,
	modulePath + "/internal/project.pitfallsMarkdown":       true,
	modulePath + "/internal/project.commitScopeTable":       true,
	modulePath + "/internal/upgrade.treeDigest":             true,
}

// These owners create authored payload bytes rather than user-facing text.
// Their literals are not presentation syntax and must remain distinguishable
// from a raw CLI presentation literal.
var nonPresentationLiteralOwners = map[string]bool{
	modulePath + "/internal/topic.ScaffoldFiles":      true,
	modulePath + "/internal/topic.ParsePart":          true,
	modulePath + "/internal/effort.memoryBody":        true,
	modulePath + "/internal/project.pitfallsMarkdown": true,
	modulePath + "/internal/project.commitScopeTable": true,
}

func loadPresentationPackages(t *testing.T, overlay map[string][]byte) []*packages.Package {
	t.Helper()
	patterns := []string{
		"./cmd/awf", "./internal/audit", "./internal/clispec", "./internal/commitpolicy",
		"./internal/contextdelivery", "./internal/contextq", "./internal/effort", "./internal/initspec",
		"./internal/memorycite", "./internal/project", "./internal/prosegate", "./internal/resident",
		"./internal/topic", "./internal/upgrade", "./internal/worktree",
	}
	pkgs, err := packages.Load(&packages.Config{Dir: repoRoot(t), Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo, Overlay: overlay}, patterns...)
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
	return ordinaryOutputFindingsInPackages(loadPresentationPackages(t, overlay), overlay == nil)
}

func ordinaryOutputFindingsInPackages(pkgs []*packages.Package, requireBypasses bool) []string {
	var findings []string
	declaredBypasses := map[string]bool{}
	for _, pkg := range pkgs {
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
						if ordinaryWriteCall(pkg, node) && !successfulBypasses[identity] && !nonPresentationWrites[identity] && identity != modulePath+"/cmd/awf.writeRendererFailure" {
							findings = append(findings, presentationSite(pkg, node)+" direct output in "+identity)
						}
						if forbiddenPresentationConstruction(node) {
							findings = append(findings, presentationSite(pkg, node)+" ad hoc presentation construction in "+identity)
						}
					case *ast.BasicLit:
						if !successfulBypasses[identity] && !nonPresentationLiteralOwners[identity] && forbiddenPresentationLiteral(node) {
							findings = append(findings, presentationSite(pkg, node)+" raw Markdown presentation in "+identity)
						}
					}
					return true
				})
			}
		}
	}
	if requireBypasses {
		for identity := range successfulBypasses {
			if !declaredBypasses[identity] {
				findings = append(findings, "missing successful bypass symbol "+identity)
			}
		}
	}
	sort.Strings(findings)
	return findings
}

func presentationOwnerFile(_ *packages.Package, _ *ast.File) bool {
	// The declared owner-package set is the boundary. Do not make scanning
	// conditional on an import: a file that has lost presentation usage is the
	// precise regression this proof must find.
	return true
}

func functionIdentity(pkg *packages.Package, fn *ast.FuncDecl) string {
	if object, ok := pkg.TypesInfo.Defs[fn.Name]; ok {
		return objectIdentity(object)
	}
	return pkg.PkgPath + "." + fn.Name.Name
}

func objectIdentity(object types.Object) string {
	if object == nil || object.Pkg() == nil {
		return ""
	}
	return object.Pkg().Path() + "." + object.Name()
}

func calledObject(pkg *packages.Package, call *ast.CallExpr) types.Object {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return pkg.TypesInfo.Uses[fun]
	case *ast.SelectorExpr:
		return pkg.TypesInfo.Uses[fun.Sel]
	}
	return nil
}

func ordinaryWriteCall(pkg *packages.Package, call *ast.CallExpr) bool {
	identity := objectIdentity(calledObject(pkg, call))
	switch identity {
	case "fmt.Fprint", "fmt.Fprintf", "fmt.Fprintln", "fmt.Print", "fmt.Printf", "fmt.Println", "io.WriteString", "io.Copy":
		return true
	}
	// A resolved Write-like selector catches aliases and receivers without
	// mistaking an ident named Write for a writer method.
	if _, ok := call.Fun.(*ast.SelectorExpr); !ok {
		return false
	}
	object := calledObject(pkg, call)
	if function, ok := object.(*types.Func); ok {
		return function.Name() == "Write" || function.Name() == "WriteString" || function.Name() == "WriteByte"
	}
	return false
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
	return ordinaryOutputFindingsInPackages([]*packages.Package{loadBoundaryFixture(t, name, false)}, false)
}

func loadBoundaryFixture(t *testing.T, name string, renderer bool) *packages.Package {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "presentation-boundary", name))
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, name, data, 0)
	if err != nil {
		t.Fatal(err)
	}
	files := []*ast.File{file}
	if renderer {
		stub, parseErr := parser.ParseFile(fileSet, "renderer_stub.go", "package main\nimport \"io\"\nfunc writeRendererFailure(io.Writer, error) {}\n", 0)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		files = append(files, stub)
	}
	pkgPath := modulePath + "/cmd/awf"
	if file.Name.Name != "main" {
		pkgPath = modulePath + "/internal/" + file.Name.Name
	}
	info := &types.Info{Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}, Types: map[ast.Expr]types.TypeAndValue{}, Selections: map[*ast.SelectorExpr]*types.Selection{}}
	checked, err := (&types.Config{Importer: boundaryFixtureImporter{fallback: importer.Default()}}).Check(pkgPath, fileSet, files, info)
	if err != nil {
		t.Fatalf("type-check fixture %s: %v", name, err)
	}
	return &packages.Package{Name: file.Name.Name, PkgPath: pkgPath, Fset: fileSet, Syntax: files, Types: checked, TypesInfo: info}
}

type boundaryFixtureImporter struct {
	fallback types.Importer
}

func (i boundaryFixtureImporter) Import(path string) (*types.Package, error) {
	if path != modulePath+"/internal/presentation" {
		return i.fallback.Import(path)
	}
	ioPackage, err := i.fallback.Import("io")
	if err != nil {
		return nil, err
	}
	pkg := types.NewPackage(path, "presentation")
	document := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "Document", nil), types.NewStruct(nil, nil), nil)
	pkg.Scope().Insert(document.Obj())
	writer := ioPackage.Scope().Lookup("Writer").Type()
	params := types.NewTuple(types.NewParam(token.NoPos, pkg, "w", writer), types.NewParam(token.NoPos, pkg, "document", document))
	results := types.NewTuple(types.NewParam(token.NoPos, pkg, "err", types.Universe.Lookup("error").Type()))
	pkg.Scope().Insert(types.NewFunc(token.NoPos, pkg, "Render", types.NewSignatureType(nil, nil, nil, params, results, false)))
	pkg.MarkComplete()
	return pkg, nil
}

func rendererFailureFindings(t *testing.T, overlay map[string][]byte) []string {
	t.Helper()
	return rendererFailureFindingsInPackages(loadPresentationPackages(t, overlay), overlay == nil)
}

func rendererFailureFindingsInPackages(pkgs []*packages.Package, requireProductionCall bool) []string {
	var findings []string
	productionCalls := 0
	for _, pkg := range pkgs {
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
					if !ok || objectIdentity(calledObject(pkg, call)) != modulePath+"/cmd/awf.writeRendererFailure" {
						return true
					}
					productionCalls++
					if !rendererFailureDominates(pkg, fn, call) {
						findings = append(findings, presentationSite(pkg, call)+" writeRendererFailure is not dominated by renderer failure in "+identity)
					}
					return true
				})
			}
		}
	}
	if requireProductionCall && productionCalls != 1 {
		findings = append(findings, "writeRendererFailure production call count = "+strconv.Itoa(productionCalls)+", want 1")
	}
	sort.Strings(findings)
	return findings
}

func rendererFailureDominates(pkg *packages.Package, fn *ast.FuncDecl, target *ast.CallExpr) bool {
	if functionIdentity(pkg, fn) != modulePath+"/cmd/awf.writeOutcome" || len(target.Args) < 2 {
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
		if ok && renderOK && conditionOK && leftOK && rightOK && bound.Name == cause.Name && left.Name == bound.Name && right.Name == "nil" && condition.Op.String() == "!=" && objectIdentity(calledObject(pkg, renderCall)) == modulePath+"/internal/presentation.Render" {
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
	return rendererFailureFindingsInPackages([]*packages.Package{loadBoundaryFixture(t, name, true)}, false)
}
