package presentation

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

type badNode struct{}

func (badNode) presentationNode() {}

func TestReportDocumentRejectsConstructibleZeroFields(t *testing.T) {
	for _, report := range []Report{{Context: []Field{{}}}, {Summary: []Field{{}}}} {
		if _, err := report.Document(); err == nil {
			t.Fatal("report accepted a constructible zero Field")
		}
	}
}

// invariant: code-design/presentation-ownership:closed-presentation-tree (TestPresentationTreeContract)
func TestPresentationTreeContract(t *testing.T) {
	assertPresentationSourceContract(t)
	v := func(s string) value {
		got, err := Prose(s)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	f, _ := NewField("field", v("value"))
	f.presentationNode()
	mustReject := func(name string, construct func() error) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			if err := construct(); err == nil {
				t.Fatal("invalid grammar transition accepted")
			}
		})
	}
	r1, _ := NewRecord(v(`left|right`), v(`slash\\`))
	r2, _ := NewRecord(v("two"), v("three"))
	group, _ := NewRecordGroup("records", []string{"first", "second"}, r1, r2)
	group.presentationNode()
	list, _ := NewList("items", v("one"), v("two"))
	list.presentationNode()
	steps, _ := NewSteps("steps", v("first"), v("second"))
	steps.presentationNode()
	inner, _ := NewSection("inner", f, list, group, steps)
	inner.presentationNode()
	r1.presentationNode()
	middle, _ := NewSection("middle", inner)
	outer, _ := NewSection("outer", middle)
	doc, err := NewDocument(f, outer)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Render(&out, doc); err != nil {
		t.Fatal(err)
	}
	const wantGolden = "field: value\n\nouter:\n  middle:\n    inner:\n      field: value\n      items:\n        one\n        two\n      records:\n        left\\|right | slash\\\\\\\\\n        two | three\n      steps:\n        step 1: first\n        step 2: second\n"
	if out.String() != wantGolden {
		t.Fatalf("presentation grammar:\n--- got ---\n%s--- want ---\n%s", out.String(), wantGolden)
	}

	// Document admits only ordered root Fields then Sections. Section admits
	// every non-Record node, including another Section, and rejects the lone
	// leaf node that has no label. These are the complete node-child matrix;
	// the remaining constructors below prove each leaf's scalar arity.
	t.Run("admitted node matrix", func(t *testing.T) {
		if _, err := NewDocument(f, outer); err != nil {
			t.Fatalf("document Field/Section matrix: %v", err)
		}
		if _, err := NewSection("matrix", f, inner, list, group, steps); err != nil {
			t.Fatalf("section Field/Section/List/RecordGroup/Steps matrix: %v", err)
		}
	})
	mustReject("document rejects list", func() error { _, err := NewDocument(list); return err })
	mustReject("document rejects record group", func() error { _, err := NewDocument(group); return err })
	mustReject("document rejects record", func() error { _, err := NewDocument(r1); return err })
	mustReject("document rejects steps", func() error { _, err := NewDocument(steps); return err })
	mustReject("section rejects record", func() error { _, err := NewSection("matrix", r1); return err })
	mustReject("section rejects unknown node", func() error { _, err := NewSection("matrix", badNode{}); return err })
	mustReject("list requires scalar leaf", func() error { _, err := NewList("items", value{}); return err })
	mustReject("steps requires scalar leaf", func() error { _, err := NewSteps("steps", value{}); return err })
	mustReject("record requires scalar fields", func() error { _, err := NewRecord(value{}); return err })
	mustReject("record group requires matching record arity", func() error { _, err := NewRecordGroup("records", []string{"one"}, r1); return err })
	for _, want := range []string{"field: value\n\nouter:", "items:\n        one", `left\|right | slash\\\\`} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Errorf("output missing %q: %s", want, out.String())
		}
	}
	for _, err := range []error{
		func() error { _, e := NewRecord(); return e }(),
		func() error { _, e := NewSection("Bad", f); return e }(),
		func() error { _, e := NewList("Bad", v("one")); return e }(),
		func() error { _, e := NewRecordGroup("Bad", []string{"one", "two"}, r1); return e }(),
		func() error { _, e := NewRecord(value{}); return e }(),
		func() error { _, e := NewDocument(badNode{}); return e }(),
		func() error { _, e := NewDocument(nil); return e }(),
		func() error { _, e := NewSection("section", nil); return e }(),
		func() error { _, e := NewSection("section", badNode{}); return e }(),
		func() error { _, e := NewDocument(Section{label: "section", nodes: []Node{badNode{}}}); return e }(),
		func() error { _, e := NewDocument(Field{label: "field", value: value{}}, outer); return e }(),
		func() error { _, e := NewSection("section", Field{label: "field", value: value{}}); return e }(),
		func() error { _, e := NewList("items", value{}); return e }(),
		func() error { _, e := NewRecordGroup("records", []string{"bad label!"}, r1); return e }(),
		func() error {
			_, e := NewRecordGroup("records", []string{"one"}, Record{values: []value{{}}})
			return e
		}(),
		func() error { _, e := NewSection("section", List{label: "items", values: nil}); return e }(),
		func() error {
			_, e := NewSection("section", RecordGroup{label: "records", schema: nil, records: nil})
			return e
		}(),
		func() error { _, e := NewSection("section", List{label: "items", values: []value{{}}}); return e }(),
		func() error {
			_, e := NewSection("section", RecordGroup{label: "records", schema: []string{"one"}, records: []Record{{values: []value{{}}}}})
			return e
		}(),
		func() error { _, e := NewRecordGroup("records", nil, r1); return e }(),
		func() error { _, e := NewRecordGroup("records", []string{"one"}, r1); return e }(),
		func() error { _, e := NewList("items"); return e }(),
		func() error { _, e := NewSection("section"); return e }(),
		func() error { _, e := NewDocument(outer, f); return e }(),
	} {
		if err == nil {
			t.Error("invalid shape accepted")
		}
	}
	tooDeep, _ := NewSection("one", f)
	for _, label := range []string{"two", "three"} {
		tooDeep, _ = NewSection(label, tooDeep)
	}
	if _, err := NewSection("four", tooDeep); err == nil {
		t.Error("fourth section level accepted")
	}
}

func assertPresentationSourceContract(t *testing.T) {
	t.Helper()
	files, fileSet := presentationSourceFiles(t, "")
	wantNodes := []string{"Field", "List", "Record", "RecordGroup", "Section", "Steps", "nodeMarker"}
	wantBoundary := []string{"NewDocument", "NewSection", "Prompt", "Render", "validateDocument", "writeDocument", "writeNode"}
	if err := presentationContract(fileSet, files, wantNodes, wantBoundary); err != nil {
		t.Fatal(err)
	}

	// This fixture is deliberately name-opaque. The exact contract must reject
	// a promoted marker method, an opaque consumer, and both package-level and
	// non-marker-method attempts to hide behind the marker method's name.
	files, fileSet = presentationSourceFiles(t, `
package presentation

type escapedNode struct{ *nodeMarker }
type alternateConsumer struct{}

func FormatPresentation(document Document) {}
func presentationNode(document Document) {}
func (alternateConsumer) presentationNode(document Document) {}
`)
	err := presentationContract(fileSet, files, wantNodes, wantBoundary)
	if err == nil {
		t.Fatal("rogue presentation types satisfied the exact contract")
	}
	for _, want := range []string{"unexpected node implementations: escapedNode", "unexpected boundary functions:", "FormatPresentation", "presentationNode"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("contract error %q missing %q", err, want)
		}
	}

	// Keep the result-bearing same-name method isolated: if marker detection
	// stops checking result arity, no other rogue declaration can mask the gap.
	files, fileSet = presentationSourceFiles(t, `
package presentation

func (Document) presentationNode() bool { return false }
`)
	err = presentationContract(fileSet, files, wantNodes, wantBoundary)
	if err == nil || !strings.Contains(err.Error(), "unexpected boundary functions: presentationNode") {
		t.Fatalf("result-bearing marker lookalike escaped exact contract: %v", err)
	}
}

func presentationSourceFiles(t *testing.T, fixture string) ([]*ast.File, *token.FileSet) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate presentation source")
	}
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(file), "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	var files []*ast.File
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, parsed)
	}
	if fixture != "" {
		parsed, err := parser.ParseFile(fileSet, "contract_fixture.go", fixture, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, parsed)
	}
	return files, fileSet
}

func presentationContract(fileSet *token.FileSet, files []*ast.File, wantNodes, wantBoundary []string) error {
	info := &types.Info{Defs: make(map[*ast.Ident]types.Object)}
	config := types.Config{Importer: importer.Default()}
	pkg, err := config.Check("github.com/hypnotox/agentic-workflows/internal/presentation", fileSet, files, info)
	if err != nil {
		return err
	}
	document := pkg.Scope().Lookup("Document").Type()
	nodeType := pkg.Scope().Lookup("Node").Type()
	node, ok := nodeType.Underlying().(*types.Interface)
	if !ok {
		return errors.New("Node is not an interface")
	}
	node.Complete()
	var nodeTypes, boundary []string
	for _, name := range pkg.Scope().Names() {
		object, ok := pkg.Scope().Lookup(name).(*types.TypeName)
		if !ok || object.IsAlias() {
			continue
		}
		named, ok := object.Type().(*types.Named)
		if !ok {
			continue
		}
		if _, isInterface := named.Underlying().(*types.Interface); !isInterface && (types.Implements(named, node) || types.Implements(types.NewPointer(named), node)) {
			nodeTypes = append(nodeTypes, object.Name())
		}
	}
	for _, source := range files {
		for _, declaration := range source.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			object, ok := info.Defs[function.Name].(*types.Func)
			if !ok {
				return fmt.Errorf("resolve function %s", function.Name.Name)
			}
			signature := object.Type().(*types.Signature)
			if !isPresentationNodeMarker(function, signature) && signatureConsumesPresentation(signature, document, nodeType, node) {
				boundary = append(boundary, object.Name())
			}
		}
	}
	sort.Strings(nodeTypes)
	sort.Strings(boundary)
	var differences []string
	if unexpected := difference(nodeTypes, wantNodes); len(unexpected) > 0 {
		differences = append(differences, "unexpected node implementations: "+strings.Join(unexpected, ", "))
	}
	if missing := difference(wantNodes, nodeTypes); len(missing) > 0 {
		differences = append(differences, "missing node implementations: "+strings.Join(missing, ", "))
	}
	if unexpected := difference(boundary, wantBoundary); len(unexpected) > 0 {
		differences = append(differences, "unexpected boundary functions: "+strings.Join(unexpected, ", "))
	}
	if missing := difference(wantBoundary, boundary); len(missing) > 0 {
		differences = append(differences, "missing boundary functions: "+strings.Join(missing, ", "))
	}
	if len(differences) > 0 {
		return errors.New(strings.Join(differences, "; "))
	}
	return nil
}

func isPresentationNodeMarker(function *ast.FuncDecl, signature *types.Signature) bool {
	return function.Name.Name == "presentationNode" && signature.Recv() != nil && signature.Params().Len() == 0 && signature.Results().Len() == 0
}

func difference(got, want []string) []string {
	var result []string
	for _, value := range got {
		if !containsString(want, value) {
			result = append(result, value)
		}
	}
	return result
}

func signatureConsumesPresentation(signature *types.Signature, document, nodeType types.Type, node *types.Interface) bool {
	if receiver := signature.Recv(); receiver != nil && (presentationOperand(receiver.Type(), document, nodeType) || types.Implements(receiver.Type(), node)) {
		return true
	}
	for i := range signature.Params().Len() {
		if presentationOperand(signature.Params().At(i).Type(), document, nodeType) {
			return true
		}
	}
	return false
}

// presentationOperand recognizes the boundary's generic Document and Node
// operands, not concrete node internals such as []Field used by validators.
func presentationOperand(typ, document, nodeType types.Type) bool {
	if types.Identical(typ, document) || types.Identical(typ, nodeType) {
		return true
	}
	switch typ := typ.(type) {
	case *types.Pointer:
		return presentationOperand(typ.Elem(), document, nodeType)
	case *types.Slice:
		return presentationOperand(typ.Elem(), document, nodeType)
	case *types.Array:
		return presentationOperand(typ.Elem(), document, nodeType)
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
